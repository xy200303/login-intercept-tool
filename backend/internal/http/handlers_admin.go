package httpapi

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"fenx/backend/internal/models"
	"fenx/backend/internal/source"
	"golang.org/x/crypto/bcrypt"
)

func maskMobile(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) >= 7 {
		return raw[:3] + "****" + raw[len(raw)-4:]
	}
	if raw == "" {
		return ""
	}
	return "****"
}

// maskEmail 本地部分保留前 2 位，如 24****@qq.com。
func maskEmail(raw string) string {
	raw = strings.TrimSpace(raw)
	at := strings.Index(raw, "@")
	if at < 0 {
		return "****"
	}
	local := raw[:at]
	if len(local) > 2 {
		local = local[:2]
	}
	return local + "****" + raw[at:]
}

// fenxSensitiveFields 为任何 API 响应中一律不出现的外部库列（密码哈希、银行卡、证件等）。
var fenxSensitiveFields = map[string]bool{
	"password": true, "bankaccount": true, "idcard": true, "bankname": true,
	"bankbranch": true, "deduction": true, "memo": true, "messageid": true, "activateid": true,
}

// sanitizeFenxRow 就地剔除敏感列并对 email/mobile 脱敏，用于一切响应体。
func sanitizeFenxRow(row map[string]any) {
	for key, value := range row {
		lower := strings.ToLower(key)
		if fenxSensitiveFields[lower] || strings.Contains(lower, "password") {
			delete(row, key)
			continue
		}
		switch lower {
		case "mobile", "tel", "phone":
			row[key] = maskMobile(fmt.Sprint(value))
		case "email", "mail":
			row[key] = maskEmail(fmt.Sprint(value))
		}
	}
}

// sanitizeJobForResponse 返回脱敏副本：归档的 before/after 快照保留在库中，
// 但响应体里剔除敏感列。
func sanitizeJobForResponse(job models.ActionJob) models.ActionJob {
	scrub := func(raw *string) *string {
		if raw == nil || *raw == "" {
			return nil
		}
		var row map[string]any
		if json.Unmarshal([]byte(*raw), &row) != nil {
			return raw
		}
		sanitizeFenxRow(row)
		data, err := json.Marshal(row)
		if err != nil {
			return nil
		}
		return jsonbPtr(string(data))
	}
	job.BeforeJSON = scrub(job.BeforeJSON)
	job.AfterJSON = scrub(job.AfterJSON)
	return job
}

// updateKKUDVIP 开通/取消 kkud 用户 VIP（仅超管；写死表/列常量，vip='1' 开通、” 取消）。
func (a *API) updateKKUDVIP(w http.ResponseWriter, r *http.Request) {
	sourceID := strings.TrimSpace(r.PathValue("source_id"))
	var in struct {
		VIP *bool `json:"vip"`
	}
	if sourceID == "" || json.NewDecoder(r.Body).Decode(&in) != nil || in.VIP == nil {
		write(w, 400, map[string]string{"message": "参数无效，需要 {vip: true|false}"})
		return
	}
	kkud, srcErr := a.externalSource("kkud")
	if srcErr != nil {
		write(w, 503, map[string]string{"message": srcErr.Error()})
		return
	}
	columns, err := kkud.TableColumnsCached(r.Context(), source.KKUDTable)
	if err != nil {
		write(w, 502, map[string]string{"message": "外部库不可用"})
		return
	}
	pkColumn := pickColumn(columns, "id", "uid", "userid", "user_id")
	vipColumn := pickColumn(columns, source.KKUDVIPColumn)
	if pkColumn == "" || vipColumn == "" {
		write(w, 500, map[string]string{"message": "kkud 表缺少主键或 VIP 列"})
		return
	}
	value := ""
	if *in.VIP {
		value = "1"
	}
	affected, err := kkud.Exec(r.Context(), "UPDATE `"+source.KKUDTable+"` SET `"+vipColumn+"` = ? WHERE `"+pkColumn+"` = ?", value, sourceID)
	if err != nil {
		write(w, 502, map[string]string{"message": "VIP 更新失败"})
		return
	}
	if affected == 0 {
		write(w, 404, map[string]string{"message": "kkud 用户不存在"})
		return
	}
	claims := claimsOf(r)
	a.writeAudit(&claims.UserID, "kkud.update_vip", "kkud_user:"+sourceID, fmt.Sprintf("vip=%v affected=%d", *in.VIP, affected))
	write(w, 200, map[string]any{"source_id": sourceID, "vip": *in.VIP, "updated": affected})
}

// fenxUsers 搜索 fenx 账号（仅超管；列白名单查询，手机号/邮箱脱敏返回，§12 PII 最小化）。
func (a *API) fenxUsers(w http.ResponseWriter, r *http.Request) {
	fenx, srcErr := a.externalSource("fenx")
	if srcErr != nil {
		write(w, 503, map[string]string{"message": srcErr.Error()})
		return
	}
	pkColumn, err := fenxPKColumn(r.Context(), fenx, source.FenxUsersTable)
	if err != nil {
		write(w, 502, map[string]string{"message": "外部库不可用"})
		return
	}
	columns, err := fenx.TableColumnsCached(r.Context(), source.FenxUsersTable)
	if err != nil {
		write(w, 502, map[string]string{"message": "外部库不可用"})
		return
	}
	// 输出列白名单：按实际存在列探测，缺列跳过
	selects := []string{"`" + pkColumn + "` AS `uid`"}
	for _, name := range []string{"username", "email", "qq", "mobile", "regip", "loginip", "regtime", "logintime", "loginnum", "status", "levelid", "money"} {
		if column := pickColumn(columns, name); column != "" && !strings.EqualFold(column, pkColumn) {
			selects = append(selects, "`"+column+"` AS `"+name+"`")
		}
	}
	conditions := []string{"1=1"}
	args := []any{}
	if uid := strings.TrimSpace(r.URL.Query().Get("uid")); uid != "" {
		if _, err := parseID(uid); err != nil {
			write(w, 400, map[string]string{"message": "uid 无效"})
			return
		}
		conditions = append(conditions, "`"+pkColumn+"` = ?")
		args = append(args, uid)
	}
	if username := strings.TrimSpace(r.URL.Query().Get("username")); username != "" {
		if column := pickColumn(columns, "username", "user", "name"); column != "" {
			conditions = append(conditions, "`"+column+"` LIKE ?")
			args = append(args, "%"+username+"%")
		}
	}
	if mobile := strings.TrimSpace(r.URL.Query().Get("mobile")); mobile != "" {
		conditions = append(conditions, "`mobile` = ?")
		args = append(args, mobile)
	}
	limit, offset := pageParams(r, 50, 200)
	args = append(args, limit, offset)
	listQuery := "SELECT " + strings.Join(selects, ", ") + " FROM `" + source.FenxUsersTable + "` WHERE " + strings.Join(conditions, " AND ") + " ORDER BY `" + pkColumn + "` DESC LIMIT ? OFFSET ?"
	rows, err := fenx.QueryMaps(r.Context(), listQuery, args...)
	if err != nil {
		// schema 可能变更：失效元数据缓存后重试一次
		source.InvalidateTableColumns(fenx.DSN, source.FenxUsersTable)
		rows, err = fenx.QueryMaps(r.Context(), listQuery, args...)
	}
	if err != nil {
		write(w, 502, map[string]string{"message": "查询失败"})
		return
	}
	for _, row := range rows {
		if mobile, ok := row["mobile"]; ok {
			row["mobile"] = maskMobile(fmt.Sprint(mobile))
		}
		if email, ok := row["email"]; ok {
			row["email"] = maskEmail(fmt.Sprint(email))
		}
	}
	write(w, 200, map[string]any{"items": rows, "limit": limit, "offset": offset})
}

// fenxUserEditableFields PATCH 可编辑字段白名单（zyads_users 实测列范围；
// uid/regtime/regip/logintime/loginnum/password 不走 PATCH）。
var fenxUserEditableFields = map[string]bool{
	"username": true, "email": true, "qq": true, "tel": true, "mobile": true,
	"idcard": true, "levelid": true, "type": true, "accountname": true,
	"bankbranch": true, "bankname": true, "bankaccount": true, "contact": true,
	"recommend": true, "status": true, "groupid": true, "insite": true,
	"memo": true, "zlink": true, "serviceid": true, "money": true, "integral": true,
}

// updateFenxUser 编辑 fenx 账号白名单字段（仅超管；列先探测存在性，全参数化，写审计）。
func (a *API) updateFenxUser(w http.ResponseWriter, r *http.Request) {
	uid := strings.TrimSpace(r.PathValue("uid"))
	if _, err := parseID(uid); err != nil {
		write(w, 400, map[string]string{"message": "uid 无效"})
		return
	}
	var patch map[string]any
	if json.NewDecoder(r.Body).Decode(&patch) != nil {
		write(w, 400, map[string]string{"message": "参数无效"})
		return
	}
	fenx, srcErr := a.externalSource("fenx")
	if srcErr != nil {
		write(w, 503, map[string]string{"message": srcErr.Error()})
		return
	}
	pkColumn, err := fenxPKColumn(r.Context(), fenx, source.FenxUsersTable)
	if err != nil {
		write(w, 502, map[string]string{"message": "外部库不可用"})
		return
	}
	columns, err := fenx.TableColumnsCached(r.Context(), source.FenxUsersTable)
	if err != nil {
		write(w, 502, map[string]string{"message": "外部库不可用"})
		return
	}
	existing := map[string]bool{}
	for _, column := range columns {
		existing[strings.ToLower(column)] = true
	}
	assignments := []string{}
	args := []any{}
	fields := []string{}
	for field, value := range patch {
		if !fenxUserEditableFields[field] || !existing[field] {
			continue
		}
		assignments = append(assignments, "`"+field+"` = ?")
		args = append(args, value)
		fields = append(fields, field)
	}
	if len(assignments) == 0 {
		write(w, 400, map[string]string{"message": "没有可更新的字段"})
		return
	}
	before := a.fenxBeforeSnapshot(r.Context(), []string{uid})
	args = append(args, uid)
	affected, err := fenx.Exec(r.Context(), "UPDATE `"+source.FenxUsersTable+"` SET "+strings.Join(assignments, ", ")+" WHERE `"+pkColumn+"` = ?", args...)
	if err != nil {
		write(w, 502, map[string]string{"message": "更新失败"})
		return
	}
	if affected == 0 {
		if _, exists := before[uid]; !exists {
			write(w, 404, map[string]string{"message": "用户不存在"})
			return
		}
	}
	after := a.fenxBeforeSnapshot(r.Context(), []string{uid})
	claims := claimsOf(r)
	detail, _ := json.Marshal(map[string]any{"fields": fields, "before_status": fmt.Sprint(before[uid]["status"]), "after_status": fmt.Sprint(after[uid]["status"])})
	a.writeAudit(&claims.UserID, "fenx.update_user", "fenx_user:"+uid, string(detail))
	write(w, 200, map[string]any{"uid": uid, "updated": affected, "fields": fields})
}

// resetFenxUserPassword 重置 fenx 用户密码：md5(明文 + 'zyiis')（PHP 源码确认的哈希方式）。
func (a *API) resetFenxUserPassword(w http.ResponseWriter, r *http.Request) {
	uid := strings.TrimSpace(r.PathValue("uid"))
	if _, err := parseID(uid); err != nil {
		write(w, 400, map[string]string{"message": "uid 无效"})
		return
	}
	var in struct {
		Password string `json:"password"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || len(in.Password) < 6 {
		write(w, 400, map[string]string{"message": "密码至少 6 位"})
		return
	}
	fenx, srcErr := a.externalSource("fenx")
	if srcErr != nil {
		write(w, 503, map[string]string{"message": srcErr.Error()})
		return
	}
	pkColumn, err := fenxPKColumn(r.Context(), fenx, source.FenxUsersTable)
	if err != nil {
		write(w, 502, map[string]string{"message": "外部库不可用"})
		return
	}
	hash := fmt.Sprintf("%x", md5.Sum([]byte(in.Password+"zyiis")))
	affected, err := fenx.Exec(r.Context(), "UPDATE `"+source.FenxUsersTable+"` SET `password` = ? WHERE `"+pkColumn+"` = ?", hash, uid)
	if err != nil {
		write(w, 502, map[string]string{"message": "密码重置失败"})
		return
	}
	if affected == 0 {
		write(w, 404, map[string]string{"message": "用户不存在"})
		return
	}
	claims := claimsOf(r)
	a.writeAudit(&claims.UserID, "fenx.reset_password", "fenx_user:"+uid, "")
	write(w, 200, map[string]any{"uid": uid, "reset": true})
}

// fenxUserMeta 枚举/等级元数据（登录即可；levels 实时查 zyads_level，30 分钟缓存，
// 表不存在时降级为空数组）。
func (a *API) fenxUserMeta(w http.ResponseWriter, _ *http.Request) {
	meta := map[string]any{
		"statuses": []map[string]any{
			{"value": 0, "label": "待审核"}, {"value": 1, "label": "待激活"},
			{"value": 2, "label": "正常"}, {"value": 4, "label": "禁用"},
		},
		"types": []map[string]any{
			{"value": 1, "label": "普通用户"}, {"value": 2, "label": "流量主"},
		},
		"groups": []map[string]any{
			{"value": 0, "label": "默认组"}, {"value": 1, "label": "组1"},
		},
		"levels": a.fenxLevels(),
	}
	write(w, 200, meta)
}

type fenxLevel struct {
	LevelID   string `json:"levelid"`
	LevelName string `json:"levelname"`
}

var fenxLevelsCache = struct {
	sync.RWMutex
	dsn     string
	levels  []fenxLevel
	expires time.Time
}{}

// fenxLevels 从 zyads_level 读取等级下拉数据（30 分钟缓存，按 fenx DSN 为 key）。
func (a *API) fenxLevels() []fenxLevel {
	empty := []fenxLevel{}
	fenx, srcErr := a.externalSource("fenx")
	if srcErr != nil {
		return empty
	}
	fenxLevelsCache.RLock()
	cached := fenxLevelsCache.dsn == fenx.DSN && time.Now().Before(fenxLevelsCache.expires)
	levels := fenxLevelsCache.levels
	fenxLevelsCache.RUnlock()
	if cached {
		return levels
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := fenx.TableColumnsCached(ctx, source.FenxLevelTable); err != nil {
		return empty // 表不存在或不可读：降级空数组
	}
	rows, err := fenx.QueryMaps(ctx, "SELECT `levelid`, `levelname` FROM `"+source.FenxLevelTable+"` ORDER BY `levelid`")
	if err != nil {
		source.InvalidateTableColumns(fenx.DSN, source.FenxLevelTable)
		return empty
	}
	levels = make([]fenxLevel, 0, len(rows))
	for _, row := range rows {
		levels = append(levels, fenxLevel{
			LevelID:   strings.TrimSpace(fmt.Sprint(row["levelid"])),
			LevelName: strings.TrimSpace(fmt.Sprint(row["levelname"])),
		})
	}
	fenxLevelsCache.Lock()
	fenxLevelsCache.dsn = fenx.DSN
	fenxLevelsCache.levels = levels
	fenxLevelsCache.expires = time.Now().Add(30 * time.Minute)
	fenxLevelsCache.Unlock()
	return levels
}

// setFenxUserStatus 禁用（status=4）/启用（status=2）fenx 账号的公共实现（仅超管；写审计）。
func (a *API) setFenxUserStatus(w http.ResponseWriter, r *http.Request, status int, action string) {
	uid := strings.TrimSpace(r.PathValue("uid"))
	if _, err := parseID(uid); err != nil {
		write(w, 400, map[string]string{"message": "uid 无效"})
		return
	}
	fenx, srcErr := a.externalSource("fenx")
	if srcErr != nil {
		write(w, 503, map[string]string{"message": srcErr.Error()})
		return
	}
	pkColumn, err := fenxPKColumn(r.Context(), fenx, source.FenxUsersTable)
	if err != nil {
		write(w, 502, map[string]string{"message": "外部库不可用"})
		return
	}
	before := a.fenxBeforeSnapshot(r.Context(), []string{uid})
	row, exists := before[uid]
	if !exists {
		write(w, 404, map[string]string{"message": "用户不存在"})
		return
	}
	if _, err := fenx.Exec(r.Context(), "UPDATE `"+source.FenxUsersTable+"` SET `status` = ? WHERE `"+pkColumn+"` = ?", status, uid); err != nil {
		write(w, 502, map[string]string{"message": "状态更新失败"})
		return
	}
	claims := claimsOf(r)
	a.writeAudit(&claims.UserID, "fenx."+action, "fenx_user:"+uid, fmt.Sprintf("before_status=%s after_status=%d", fmt.Sprint(row["status"]), status))
	write(w, 200, map[string]any{"uid": uid, "status": status, "before_status": fmt.Sprint(row["status"])})
}

func (a *API) disableFenxUser(w http.ResponseWriter, r *http.Request) {
	a.setFenxUserStatus(w, r, source.FenxDisabledStatus, "disable_user")
}

func (a *API) enableFenxUser(w http.ResponseWriter, r *http.Request) {
	a.setFenxUserStatus(w, r, source.FenxNormalStatus, "enable_user")
}

// deleteFenxUser 单账号事务删除 users 行（仅超管；先归档 before_json，不碰 log_login）。
func (a *API) deleteFenxUser(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		write(w, 503, map[string]string{"message": "数据库不可用"})
		return
	}
	uid := strings.TrimSpace(r.PathValue("uid"))
	if _, err := parseID(uid); err != nil {
		write(w, 400, map[string]string{"message": "uid 无效"})
		return
	}
	fenx, srcErr := a.externalSource("fenx")
	if srcErr != nil {
		write(w, 503, map[string]string{"message": srcErr.Error()})
		return
	}
	pkColumn, err := fenxPKColumn(r.Context(), fenx, source.FenxUsersTable)
	if err != nil {
		write(w, 502, map[string]string{"message": "外部库不可用"})
		return
	}
	before := a.fenxBeforeSnapshot(r.Context(), []string{uid})
	row, exists := before[uid]
	if !exists {
		write(w, 404, map[string]string{"message": "用户不存在"})
		return
	}
	claims := claimsOf(r)
	beforeJSON, _ := json.Marshal(row)
	job := models.ActionJob{
		ConflictID: 0, Action: "delete", Status: "pending", TargetUID: uid,
		BeforeJSON: jsonbPtr(string(beforeJSON)), IdempotencyKey: fmt.Sprintf("admin-delete:%s:%d", uid, time.Now().UTC().UnixNano()),
		OperatorID: &claims.UserID, CreatedAt: time.Now().UTC(),
	}
	if err := a.db.Create(&job).Error; err != nil {
		write(w, 500, map[string]string{"message": "归档失败，未执行删除"})
		return
	}
	err = fenx.WithTx(r.Context(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(r.Context(), "DELETE FROM `"+source.FenxUsersTable+"` WHERE `"+pkColumn+"` = ?", uid)
		return err
	})
	if err != nil {
		job.Status = "failed"
		job.Error = "删除失败"
		a.db.Save(&job)
		write(w, 502, map[string]string{"message": "删除失败"})
		return
	}
	executed := time.Now().UTC()
	job.ExecutedAt = &executed
	job.Status = "succeeded"
	a.db.Save(&job)
	a.writeAudit(&claims.UserID, "fenx.delete_user", "fenx_user:"+uid, fmt.Sprintf("action_job=%d archived=%d_bytes", job.ID, len(beforeJSON)))
	write(w, 200, map[string]any{"uid": uid, "deleted": true, "action_job_id": job.ID})
}

// auditEvents 审计日志（仅超管；按时间倒序分页）。
func (a *API) auditEvents(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		write(w, 503, map[string]string{"message": "数据库不可用"})
		return
	}
	var total int64
	a.db.Model(&models.AuditEvent{}).Count(&total)
	limit, offset := pageParams(r, 50, 200)
	var rows []models.AuditEvent
	a.db.Order("id desc").Limit(limit).Offset(offset).Find(&rows)
	write(w, 200, map[string]any{"items": rows, "total": total, "limit": limit, "offset": offset})
}

// changePassword 登录用户修改自己的密码（bcrypt 校验旧密码）。
func (a *API) changePassword(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		write(w, 503, map[string]string{"message": "数据库不可用"})
		return
	}
	var in struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || len(in.NewPassword) < 8 {
		write(w, 400, map[string]string{"message": "新密码至少 8 位"})
		return
	}
	claims := claimsOf(r)
	var user models.PlatformUser
	if err := a.db.First(&user, claims.UserID).Error; err != nil {
		write(w, 400, map[string]string{"message": "该账号由环境变量管理，请通过环境变量修改密码"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(in.OldPassword)) != nil {
		write(w, 403, map[string]string{"message": "旧密码错误"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		write(w, 500, map[string]string{"message": "密码处理失败"})
		return
	}
	if err := a.db.Model(&user).Update("password_hash", string(hash)).Error; err != nil {
		write(w, 500, map[string]string{"message": "密码更新失败"})
		return
	}
	// 改密后撤销全部 refresh token，强制重新登录
	a.revokeUserRefreshTokens(user.ID)
	a.writeAudit(&claims.UserID, "auth.change_password", fmt.Sprintf("platform_user:%d", user.ID), "")
	write(w, 200, map[string]any{"changed": true})
}

// allowlist 白名单列表（仅超管）。
func (a *API) allowlist(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		write(w, 503, map[string]string{"message": "数据库不可用"})
		return
	}
	query := a.db.Model(&models.Allowlist{})
	if raw := strings.TrimSpace(r.URL.Query().Get("agent_id")); raw != "" {
		if id, err := parseID(raw); err == nil {
			query = query.Where("agent_id = ?", id)
		}
	}
	limit, offset := pageParams(r, 50, 200)
	var rows []models.Allowlist
	query.Order("id desc").Limit(limit).Offset(offset).Find(&rows)
	write(w, 200, map[string]any{"items": rows, "limit": limit, "offset": offset})
}

// createAllowlist 新增白名单（仅超管；expires_at 必填）。
func (a *API) createAllowlist(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		write(w, 503, map[string]string{"message": "数据库不可用"})
		return
	}
	var in struct {
		AgentID   uint   `json:"agent_id"`
		FenxUID   string `json:"fenx_uid"`
		IP        string `json:"ip"`
		Reason    string `json:"reason"`
		ExpiresAt string `json:"expires_at"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || in.AgentID == 0 || (strings.TrimSpace(in.FenxUID) == "" && strings.TrimSpace(in.IP) == "") {
		write(w, 400, map[string]string{"message": "代理与账号或 IP 不能为空"})
		return
	}
	expiresAt, err := time.Parse(time.RFC3339, strings.TrimSpace(in.ExpiresAt))
	if err != nil {
		write(w, 400, map[string]string{"message": "expires_at 必填且需为 RFC3339 时间"})
		return
	}
	if !expiresAt.After(time.Now()) {
		write(w, 400, map[string]string{"message": "expires_at 必须是将来的时间"})
		return
	}
	entry := models.Allowlist{
		AgentID: in.AgentID, FenxUID: strings.TrimSpace(in.FenxUID), IP: strings.TrimSpace(in.IP),
		Reason: strings.TrimSpace(in.Reason), ExpiresAt: &expiresAt, CreatedAt: time.Now().UTC(),
	}
	if err := a.db.Create(&entry).Error; err != nil {
		write(w, 500, map[string]string{"message": "白名单创建失败"})
		return
	}
	claims := claimsOf(r)
	a.writeAudit(&claims.UserID, "allowlist.create", fmt.Sprintf("allowlist:%d", entry.ID), fmt.Sprintf("agent_id=%d fenx_uid=%s ip=%s expires_at=%s", entry.AgentID, entry.FenxUID, entry.IP, in.ExpiresAt))
	write(w, 201, entry)
}

// deleteAllowlist 删除白名单（仅超管）。
func (a *API) deleteAllowlist(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		write(w, 503, map[string]string{"message": "数据库不可用"})
		return
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		write(w, 400, map[string]string{"message": "白名单编号无效"})
		return
	}
	var entry models.Allowlist
	if err := a.db.First(&entry, id).Error; err != nil {
		write(w, 404, map[string]string{"message": "白名单不存在"})
		return
	}
	if err := a.db.Delete(&entry).Error; err != nil {
		write(w, 500, map[string]string{"message": "白名单删除失败"})
		return
	}
	claims := claimsOf(r)
	a.writeAudit(&claims.UserID, "allowlist.delete", fmt.Sprintf("allowlist:%d", id), fmt.Sprintf("agent_id=%d fenx_uid=%s ip=%s", entry.AgentID, entry.FenxUID, entry.IP))
	write(w, 200, map[string]any{"deleted": true})
}
