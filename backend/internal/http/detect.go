package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"fenx/backend/internal/models"
	"fenx/backend/internal/source"
	"gorm.io/gorm"
)

func normalizeMobile(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var b strings.Builder
	for i, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if r == '+' && i == 0 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func normalizeQQ(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func normalizeEmail(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// normalizeIP 压缩 IP 表示；ok=false 表示空或解析失败。
func normalizeIP(raw string) (normalized string, ok bool, internal bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "unknown") {
		return "", false, false
	}
	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return raw, false, false
	}
	internal = addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsUnspecified()
	return addr.String(), true, internal
}

type conflictEvidence struct {
	Source string `json:"source"` // register | login_current | login_log
	IP     string `json:"ip"`
	Time   string `json:"time,omitempty"`
}

type detectAccount struct {
	UID        string
	Username   string
	RegIP      string
	LoginIP    string
	RegTime    string
	Status     string
	MatchState string
	Evidence   []conflictEvidence
}

type DetectSummary struct {
	AgentID     uint   `json:"agent_id"`
	Accounts    int    `json:"accounts"`
	IPsChecked  int    `json:"ips_checked"`
	Conflicts   int    `json:"conflicts"`
	Created     int    `json:"created"`
	Updated     int    `json:"updated"`
	Reopened    int    `json:"reopened"`
	LogLoginErr string `json:"log_login_error,omitempty"`
}

// DetectConflicts 对单个代理执行 IP 冲突检测（设计方案 §6.4），幂等 upsert。
func (a *API) DetectConflicts(ctx context.Context, agentID uint) (DetectSummary, error) {
	summary := DetectSummary{AgentID: agentID}
	if a.db == nil {
		return summary, fmt.Errorf("数据库不可用")
	}
	var agent models.Agent
	if err := a.db.First(&agent, agentID).Error; err != nil {
		return summary, fmt.Errorf("代理不存在")
	}
	if strings.TrimSpace(a.cfg.FenxDSN) == "" {
		return summary, fmt.Errorf("fenx 数据源未配置")
	}
	fenx := source.MySQLSource{Name: "fenx_site", DSN: a.cfg.FenxDSN}

	// 1. 该代理经采集关联到的全部 fenx 账号及最佳匹配置信状态
	type matchRow struct {
		FenxUserID string
		State      string
	}
	var matchRows []matchRow
	if err := a.db.Table("user_matches AS m").
		Select("m.fenx_user_id, m.state").
		Joins("JOIN kkud_snapshots s ON m.kkud_snapshot_id = s.id").
		Joins("JOIN monitor_tasks t ON s.task_id = t.id").
		Where("t.agent_id = ?", agentID).
		Scan(&matchRows).Error; err != nil {
		return summary, fmt.Errorf("读取关联结果失败")
	}
	stateRank := map[string]int{"matched": 3, "ambiguous": 2, "review": 1}
	bestState := map[string]string{}
	for _, row := range matchRows {
		uid := strings.TrimSpace(row.FenxUserID)
		if uid == "" {
			continue
		}
		if stateRank[row.State] > stateRank[bestState[uid]] {
			bestState[uid] = row.State
		}
	}
	summary.Accounts = len(bestState)
	if len(bestState) == 0 {
		return summary, nil
	}

	// 2. 读取 fenx 账号注册/当前登录 IP
	if !source.ValidIdentifier(a.cfg.FenxUsersTable) || !source.ValidIdentifier(a.cfg.FenxLoginLogTable) {
		return summary, fmt.Errorf("fenx 表名配置无效")
	}
	pkColumn, usernameColumn, err := fenxKeyColumns(ctx, fenx, a.cfg.FenxUsersTable)
	if err != nil {
		return summary, err
	}
	uids := make([]string, 0, len(bestState))
	for uid := range bestState {
		if _, convErr := strconv.ParseUint(uid, 10, 64); convErr == nil {
			uids = append(uids, uid)
		}
	}
	if len(uids) == 0 {
		return summary, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(uids)), ",")
	args := make([]any, len(uids))
	for i, uid := range uids {
		args[i] = uid
	}
	userRows, err := fenx.QueryMaps(ctx, "SELECT `"+pkColumn+"` AS uid, `"+usernameColumn+"` AS username, `regip`, `loginip`, `regtime`, `logintime`, `status` FROM `"+a.cfg.FenxUsersTable+"` WHERE `"+pkColumn+"` IN ("+placeholders+")", args...)
	if err != nil {
		return summary, fmt.Errorf("读取 fenx 用户失败")
	}
	accounts := map[string]*detectAccount{}
	usernameToUID := map[string]string{}
	for _, row := range userRows {
		uid := strings.TrimSpace(fmt.Sprint(row["uid"]))
		if uid == "" {
			continue
		}
		account := &detectAccount{
			UID:        uid,
			Username:   strings.TrimSpace(fmt.Sprint(row["username"])),
			RegIP:      strings.TrimSpace(fmt.Sprint(row["regip"])),
			LoginIP:    strings.TrimSpace(fmt.Sprint(row["loginip"])),
			RegTime:    strings.TrimSpace(fmt.Sprint(row["regtime"])),
			Status:     strings.TrimSpace(fmt.Sprint(row["status"])),
			MatchState: bestState[uid],
		}
		if ip, ok, _ := normalizeIP(account.RegIP); ok {
			account.Evidence = append(account.Evidence, conflictEvidence{Source: "register", IP: ip, Time: account.RegTime})
		}
		if ip, ok, _ := normalizeIP(account.LoginIP); ok {
			account.Evidence = append(account.Evidence, conflictEvidence{Source: "login_current", IP: ip, Time: strings.TrimSpace(fmt.Sprint(row["logintime"]))})
		}
		accounts[uid] = account
		if account.Username != "" {
			usernameToUID[account.Username] = uid
		}
	}

	// 3. 读取登录日志成功记录（status=1）：zyads_log_login 无 uid 列，按 username 关联；
	//    同时兼容按 uid 关联的表结构（列名动态探测）
	refs := make([]string, 0, len(accounts))
	for _, account := range accounts {
		if account.Username != "" {
			refs = append(refs, account.Username)
		} else {
			refs = append(refs, account.UID)
		}
	}
	logEvidence, logErr := successfulLoginIPs(ctx, fenx, a.cfg.FenxLoginLogTable, refs)
	if logErr != nil {
		summary.LogLoginErr = "登录日志读取失败"
	} else {
		for ref, items := range logEvidence {
			uid := ref
			if mapped, ok := usernameToUID[ref]; ok {
				uid = mapped
			}
			if account, ok := accounts[uid]; ok {
				account.Evidence = append(account.Evidence, items...)
			}
		}
	}

	// 4. 白名单（未过期）
	var allowlist []models.Allowlist
	now := time.Now().UTC()
	a.db.Where("agent_id = ? AND (expires_at IS NULL OR expires_at > ?)", agentID, now).Find(&allowlist)
	allowed := map[string]bool{}
	for _, entry := range allowlist {
		allowed[entry.FenxUID+"|"+entry.IP] = true
	}

	// 5. 按 IP 分组（同账号自身 regip==loginip 只算一个账号，不产生冲突）
	byIP := map[string]map[string]*detectAccount{}
	for _, account := range accounts {
		seen := map[string]bool{}
		for _, ev := range account.Evidence {
			if seen[ev.IP] {
				continue
			}
			seen[ev.IP] = true
			if byIP[ev.IP] == nil {
				byIP[ev.IP] = map[string]*detectAccount{}
			}
			byIP[ev.IP][account.UID] = account
		}
	}
	summary.IPsChecked = len(byIP)

	for ip, members := range byIP {
		for uid := range members {
			if allowed[uid+"|"+ip] {
				delete(members, uid)
			}
		}
		if len(members) < 2 {
			continue
		}
		if err := a.upsertConflict(agentID, ip, members, now, &summary); err != nil {
			return summary, err
		}
	}
	return summary, nil
}

func (a *API) upsertConflict(agentID uint, ip string, members map[string]*detectAccount, now time.Time, summary *DetectSummary) error {
	conflictType := "register"
	hasRegister, hasLogin := false, false
	for _, account := range members {
		for _, ev := range account.Evidence {
			if ev.IP != ip {
				continue
			}
			if ev.Source == "register" {
				hasRegister = true
			} else {
				hasLogin = true
			}
		}
	}
	switch {
	case hasRegister && hasLogin:
		conflictType = "mixed"
	case hasLogin:
		conflictType = "login"
	}
	risk := "medium"
	_, valid, internal := normalizeIP(ip)
	if !valid || internal {
		risk = "low"
	} else if conflictType == "mixed" {
		risk = "high"
	}
	winner := pickWinner(members)

	var conflict models.IPConflict
	err := a.db.Where("agent_id = ? AND ip = ? AND conflict_type = ?", agentID, ip, conflictType).First(&conflict).Error
	switch {
	case err == nil:
		updates := map[string]any{"risk": risk, "member_count": len(members), "last_seen_at": now, "winner_uid": winner}
		if conflict.Status == "resolved" {
			updates["status"] = "open"
			updates["resolved_at"] = nil
			summary.Reopened++
		} else {
			summary.Updated++
		}
		if err := a.db.Model(&conflict).Updates(updates).Error; err != nil {
			return fmt.Errorf("更新冲突失败")
		}
	case err == gorm.ErrRecordNotFound:
		conflict = models.IPConflict{
			AgentID: agentID, IP: ip, ConflictType: conflictType, Risk: risk, Status: "open",
			WinnerUID: winner, MemberCount: len(members), FirstSeenAt: now, LastSeenAt: now,
		}
		if err := a.db.Create(&conflict).Error; err != nil {
			return fmt.Errorf("创建冲突失败")
		}
		summary.Created++
	default:
		return fmt.Errorf("查询冲突失败")
	}
	summary.Conflicts++

	if err := a.db.Where("conflict_id = ?", conflict.ID).Delete(&models.ConflictMember{}).Error; err != nil {
		return fmt.Errorf("清理冲突成员失败")
	}
	for _, account := range members {
		evidence := make([]conflictEvidence, 0, len(account.Evidence))
		for _, ev := range account.Evidence {
			if ev.IP == ip {
				evidence = append(evidence, ev)
			}
		}
		evidenceJSON, _ := json.Marshal(evidence)
		member := models.ConflictMember{
			ConflictID:   conflict.ID,
			FenxUID:      account.UID,
			Username:     account.Username,
			RegIP:        account.RegIP,
			LoginIP:      account.LoginIP,
			MatchState:   account.MatchState,
			EvidenceJSON: string(evidenceJSON),
			IsWinner:     account.UID == winner,
		}
		if err := a.db.Create(&member).Error; err != nil {
			return fmt.Errorf("写入冲突成员失败")
		}
	}
	return nil
}

// pickWinner 优先最早注册且 status 正常（status=2）的账号；白名单/VIP 字段预留。
func pickWinner(members map[string]*detectAccount) string {
	best := ""
	bestOK := false
	for uid, account := range members {
		normal := account.Status == "2"
		candidate := account.RegTime
		if best == "" || (normal && !bestOK) || (normal == bestOK && regTimeLess(candidate, regTimeOf(members, best))) {
			best, bestOK = uid, normal
		}
	}
	return best
}

func regTimeOf(members map[string]*detectAccount, uid string) string {
	return members[uid].RegTime
}

// regTimeLess 比较注册时间，支持 unix 时间戳与常见日期串，空值视为最晚。
func regTimeLess(a, b string) bool {
	ta, oka := parseRegTime(a)
	tb, okb := parseRegTime(b)
	if oka && okb {
		return ta.Before(tb)
	}
	if oka != okb {
		return oka
	}
	return a < b
}

func parseRegTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "0" {
		return time.Time{}, false
	}
	if ts, err := strconv.ParseInt(raw, 10, 64); err == nil && ts > 0 {
		return time.Unix(ts, 0).UTC(), true
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05 -0700 MST", "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// fenxKeyColumns 探测 fenx 用户表的主键列与用户名列。zyads_users 主键为 uid，
// 候选顺序 uid 优先，避免误选同名的非主键 id 列。
func fenxKeyColumns(ctx context.Context, fenx source.MySQLSource, table string) (pkColumn, usernameColumn string, err error) {
	columns, err := fenx.TableColumns(ctx, table)
	if err != nil {
		return "", "", fmt.Errorf("读取 fenx 用户表结构失败")
	}
	pkColumn = pickColumn(columns, "uid", "id", "userid", "user_id")
	usernameColumn = pickColumn(columns, "username", "user", "name")
	if pkColumn == "" {
		return "", "", fmt.Errorf("fenx 用户表缺少主键列")
	}
	if usernameColumn == "" {
		return "", "", fmt.Errorf("fenx 用户表缺少用户名列")
	}
	return pkColumn, usernameColumn, nil
}

// fenxPKColumn 只需主键列时的便捷包装。
func fenxPKColumn(ctx context.Context, fenx source.MySQLSource, table string) (string, error) {
	pkColumn, _, err := fenxKeyColumns(ctx, fenx, table)
	return pkColumn, err
}

func pickColumn(columns []string, names ...string) string {
	for _, name := range names {
		for _, column := range columns {
			if strings.EqualFold(column, name) {
				return column
			}
		}
	}
	return ""
}

// successfulLoginIPs 读取登录日志中 status=1 的记录。refs 为用户引用值集合
// （username 或 uid，取决于表中存在的引用列）；返回按引用值分组的证据。
func successfulLoginIPs(ctx context.Context, fenx source.MySQLSource, table string, refs []string) (map[string][]conflictEvidence, error) {
	columns, err := fenx.TableColumns(ctx, table)
	if err != nil {
		return nil, err
	}
	refColumn := pickColumn(columns, "username", "uid", "userid", "user_id")
	ipColumn := pickColumn(columns, "ip", "loginip", "login_ip")
	timeColumn := pickColumn(columns, "time", "logintime", "login_time", "addtime", "created_at")
	statusColumn := pickColumn(columns, "status")
	if refColumn == "" || ipColumn == "" {
		return nil, fmt.Errorf("登录日志缺少用户引用/ip 列")
	}
	selectTime := "NULL"
	if timeColumn != "" {
		selectTime = "`" + timeColumn + "`"
	}
	whereStatus := ""
	if statusColumn != "" {
		whereStatus = "`" + statusColumn + "` = 1 AND "
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(refs)), ",")
	args := make([]any, len(refs))
	for i, ref := range refs {
		args[i] = ref
	}
	query := "SELECT `" + refColumn + "` AS ref, `" + ipColumn + "` AS ip, " + selectTime + " AS log_time FROM `" + table + "` WHERE " + whereStatus + "`" + refColumn + "` IN (" + placeholders + ")"
	rows, err := fenx.QueryMaps(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	result := map[string][]conflictEvidence{}
	for _, row := range rows {
		ref := strings.TrimSpace(fmt.Sprint(row["ref"]))
		ip, ok, _ := normalizeIP(fmt.Sprint(row["ip"]))
		if ref == "" || !ok {
			continue
		}
		logTime := ""
		if row["log_time"] != nil {
			logTime = strings.TrimSpace(fmt.Sprint(row["log_time"]))
		}
		result[ref] = append(result[ref], conflictEvidence{Source: "login_log", IP: ip, Time: logTime})
	}
	return result, nil
}
