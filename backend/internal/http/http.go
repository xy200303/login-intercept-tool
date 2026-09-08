package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"fenx/backend/internal/auth"
	"fenx/backend/internal/config"
	"fenx/backend/internal/models"
	"fenx/backend/internal/source"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type API struct {
	cfg   config.Config
	db    *gorm.DB
	jwt   *auth.Manager
	locks sync.Map
}

func New(cfg config.Config, database *gorm.DB, manager *auth.Manager) *API {
	return &API{cfg: cfg, db: database, jwt: manager}
}
func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", a.health)
	mux.HandleFunc("POST /api/v1/auth/login", a.login)
	mux.HandleFunc("GET /api/v1/me", a.requireAuth(a.me))
	mux.HandleFunc("GET /api/v1/agents", a.requireRole("super_admin", "operator", "agent")(a.agents))
	mux.HandleFunc("POST /api/v1/agents", a.requireRole("super_admin", "operator")(a.createAgent))
	mux.HandleFunc("GET /api/v1/users", a.requireRole("super_admin")(a.users))
	mux.HandleFunc("POST /api/v1/users", a.requireRole("super_admin")(a.createUser))
	mux.HandleFunc("GET /api/v1/monitor-tasks", a.requireAuth(a.tasks))
	mux.HandleFunc("POST /api/v1/monitor-tasks", a.requireAuth(a.createTask))
	mux.HandleFunc("POST /api/v1/monitor-tasks/{id}/run", a.requireAuth(a.runTask))
	mux.HandleFunc("GET /api/v1/sync-runs", a.requireAuth(a.syncRuns))
	mux.HandleFunc("GET /api/v1/matches", a.requireAuth(a.matches))
	mux.HandleFunc("GET /api/v1/kkud/snapshots", a.requireRole("super_admin", "operator", "agent")(a.snapshots))
	mux.HandleFunc("POST /api/v1/connections/test", a.requireAuth(a.testConnections))
	mux.HandleFunc("POST /api/v1/decision", a.decision)
	mux.HandleFunc("POST /api/v1/agents/{id}/detect", a.requireRole("super_admin", "agent")(a.detectNow))
	mux.HandleFunc("GET /api/v1/conflicts", a.requireAuth(a.conflicts))
	mux.HandleFunc("GET /api/v1/conflicts/{id}", a.requireAuth(a.conflictDetail))
	mux.HandleFunc("POST /api/v1/conflicts/{id}/preview", a.requireRole("super_admin", "agent")(a.conflictPreview))
	mux.HandleFunc("POST /api/v1/conflicts/{id}/actions", a.requireRole("super_admin")(a.conflictActions))
	mux.HandleFunc("POST /api/v1/actions/{id}/undo-disable", a.requireRole("super_admin")(a.undoDisable))
	mux.HandleFunc("POST /api/v1/actions/{id}/retry", a.requireRole("super_admin")(a.retryAction))
	mux.HandleFunc("PATCH /api/v1/kkud/users/{source_id}/vip", a.requireRole("super_admin")(a.updateKKUDVIP))
	mux.HandleFunc("GET /api/v1/fenx/users", a.requireRole("super_admin")(a.fenxUsers))
	mux.HandleFunc("PATCH /api/v1/fenx/users/{uid}", a.requireRole("super_admin")(a.updateFenxUser))
	mux.HandleFunc("POST /api/v1/fenx/users/{uid}/disable", a.requireRole("super_admin")(a.disableFenxUser))
	mux.HandleFunc("POST /api/v1/fenx/users/{uid}/enable", a.requireRole("super_admin")(a.enableFenxUser))
	mux.HandleFunc("DELETE /api/v1/fenx/users/{uid}", a.requireRole("super_admin")(a.deleteFenxUser))
	mux.HandleFunc("GET /api/v1/settings/external-db", a.requireRole("super_admin")(a.getExternalDBSettings))
	mux.HandleFunc("PUT /api/v1/settings/external-db", a.requireRole("super_admin")(a.putExternalDBSettings))
	mux.HandleFunc("GET /api/v1/audit-events", a.requireRole("super_admin")(a.auditEvents))
	mux.HandleFunc("POST /api/v1/auth/change-password", a.requireAuth(a.changePassword))
	mux.HandleFunc("GET /api/v1/allowlist", a.requireRole("super_admin")(a.allowlist))
	mux.HandleFunc("POST /api/v1/allowlist", a.requireRole("super_admin")(a.createAllowlist))
	mux.HandleFunc("DELETE /api/v1/allowlist/{id}", a.requireRole("super_admin")(a.deleteAllowlist))
	return mux
}

// testConnections 连接测试：带 body 测表单中未保存的值（password 空则用已保存的），
// 不带 body 测已保存配置（sys_config 优先，环境变量兜底）。
func (a *API) testConnections(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	var in struct {
		KKUD externalDBConfig `json:"kkud"`
		Fenx externalDBConfig `json:"fenx"`
	}
	hasBody := false
	if r.Body != nil {
		var raw map[string]json.RawMessage
		if json.NewDecoder(r.Body).Decode(&raw) == nil && len(raw) > 0 {
			hasBody = true
			body, _ := json.Marshal(raw)
			_ = json.Unmarshal(body, &in)
		}
	}
	resolve := func(name string, form externalDBConfig) (source.MySQLSource, error) {
		// 表单值（任一字段非空视为表单配置）；password 为空用已保存的
		if hasBody && (strings.TrimSpace(form.Host) != "" || strings.TrimSpace(form.Name) != "" || strings.TrimSpace(form.User) != "") {
			if strings.TrimSpace(form.Password) == "" {
				saved, _ := a.loadExternalConfig(name)
				form.Password = saved.Password
			}
			if !form.complete() {
				return source.MySQLSource{}, fmt.Errorf("host、name、user 不能为空")
			}
			display := name
			if name == "fenx" {
				display = "fenx_site"
			}
			return source.MySQLSource{Name: display, DSN: form.dsn()}, nil
		}
		return a.externalSource(name)
	}
	result := map[string]any{}
	test := func(key, name string, form externalDBConfig) {
		src, err := resolve(name, form)
		if err != nil {
			result[key] = map[string]any{"ok": false, "error": err.Error()}
			return
		}
		if err := src.Test(ctx); err != nil {
			result[key] = map[string]any{"ok": false, "error": "连接失败，请检查地址、账号与网络"}
			return
		}
		result[key] = map[string]any{"ok": true}
	}
	test("kkud", "kkud", in.KKUD)
	test("fenx_site", "fenx", in.Fenx)
	write(w, 200, result)
}
func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	status := "ok"
	if a.db == nil {
		status = "degraded"
	}
	write(w, 200, map[string]any{"status": status})
}
func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var in struct{ Username, Password string }
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		write(w, 401, map[string]string{"message": "用户名或密码错误"})
		return
	}
	userID, role := uint(1), "super_admin"
	var agentID *uint
	valid := in.Username == a.cfg.AdminUsername && in.Password == a.cfg.AdminPassword
	if !valid && a.db != nil {
		var user models.PlatformUser
		if err := a.db.Where("username = ? AND status = ?", in.Username, "active").First(&user).Error; err == nil && bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(in.Password)) == nil {
			valid, userID, role, agentID = true, user.ID, user.Role, user.AgentID
		}
	}
	if !valid {
		write(w, 401, map[string]string{"message": "用户名或密码错误"})
		return
	}
	token, err := a.jwt.Issue(userID, in.Username, role)
	if err != nil {
		write(w, 500, map[string]string{"message": "token error"})
		return
	}
	write(w, 200, map[string]any{"access_token": token, "user": map[string]any{"id": userID, "username": in.Username, "role": role, "agent_id": agentID}})
}
func (a *API) me(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(claimsKey{}).(*auth.Claims)
	var agentID *uint
	if a.db != nil {
		var user models.PlatformUser
		if err := a.db.Select("id, agent_id").First(&user, claims.UserID).Error; err == nil {
			agentID = user.AgentID
		}
	}
	write(w, 200, map[string]any{"uid": claims.UserID, "username": claims.Username, "role": claims.Role, "agent_id": agentID, "expires_at": claims.ExpiresAt, "issued_at": claims.IssuedAt})
}
func (a *API) agents(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		write(w, 200, []models.Agent{})
		return
	}
	query := a.db.Order("id desc")
	if scoped := a.scopedAgentID(claimsOf(r)); scoped != nil {
		query = query.Where("id = ?", *scoped)
	}
	var rows []models.Agent
	query.Find(&rows)
	write(w, 200, rows)
}
func (a *API) createAgent(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		write(w, 503, map[string]string{"message": "数据库不可用"})
		return
	}
	var row models.Agent
	if json.NewDecoder(r.Body).Decode(&row) != nil || strings.TrimSpace(row.DisplayName) == "" || len(parseSourceValues(row.SourceValues)) == 0 {
		write(w, 400, map[string]string{"message": "代理名称和匹配值不能为空"})
		return
	}
	row.Enabled = true
	a.db.Create(&row)
	write(w, 201, row)
}

func (a *API) users(w http.ResponseWriter, _ *http.Request) {
	if a.db == nil {
		write(w, 200, []models.PlatformUser{})
		return
	}
	var rows []models.PlatformUser
	a.db.Select("id, username, role, agent_id, status, last_login_ip, created_at").Order("id desc").Find(&rows)
	write(w, 200, rows)
}

func (a *API) createUser(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		write(w, 503, map[string]string{"message": "数据库不可用"})
		return
	}
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
		AgentID  *uint  `json:"agent_id"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil || strings.TrimSpace(input.Username) == "" || len(input.Password) < 8 {
		write(w, 400, map[string]string{"message": "用户名不能为空且密码至少 8 位"})
		return
	}
	if input.Role != "agent" && input.Role != "operator" {
		write(w, 400, map[string]string{"message": "角色无效"})
		return
	}
	if input.Role == "agent" && input.AgentID == nil {
		write(w, 400, map[string]string{"message": "代理账号必须绑定代理"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		write(w, 500, map[string]string{"message": "密码处理失败"})
		return
	}
	user := models.PlatformUser{Username: strings.TrimSpace(input.Username), PasswordHash: string(hash), Role: input.Role, AgentID: input.AgentID, Status: "active"}
	if err := a.db.Create(&user).Error; err != nil {
		write(w, 409, map[string]string{"message": "用户名已存在或创建失败"})
		return
	}
	write(w, 201, map[string]any{"id": user.ID, "username": user.Username, "role": user.Role, "agent_id": user.AgentID, "status": user.Status})
}
func (a *API) tasks(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		write(w, 200, []models.MonitorTask{})
		return
	}
	query := a.db.Order("id desc")
	if scoped := a.scopedAgentID(claimsOf(r)); scoped != nil {
		query = query.Where("agent_id = ?", *scoped)
	}
	var rows []models.MonitorTask
	query.Find(&rows)
	write(w, 200, rows)
}

func (a *API) createTask(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		write(w, 503, map[string]string{"message": "数据库不可用"})
		return
	}
	var input models.MonitorTask
	if json.NewDecoder(r.Body).Decode(&input) != nil || input.AgentID == 0 {
		write(w, 400, map[string]string{"message": "代理不能为空"})
		return
	}
	if scoped := a.scopedAgentID(claimsOf(r)); scoped != nil {
		input.AgentID = *scoped
	}
	if input.IntervalSec < 300 {
		input.IntervalSec = 1800
	}
	var agent models.Agent
	if err := a.db.First(&agent, input.AgentID).Error; err != nil {
		write(w, 400, map[string]string{"message": "代理不存在"})
		return
	}
	input.Enabled = true
	if err := a.db.Create(&input).Error; err != nil {
		write(w, 500, map[string]string{"message": "任务创建失败"})
		return
	}
	write(w, 201, input)
}

func (a *API) runTask(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		write(w, 503, map[string]string{"message": "数据库不可用"})
		return
	}
	taskID, err := parseID(r.PathValue("id"))
	if err != nil || taskID == 0 {
		write(w, 400, map[string]string{"message": "任务编号无效"})
		return
	}
	if scoped := a.scopedAgentID(claimsOf(r)); scoped != nil {
		var task models.MonitorTask
		if err := a.db.Select("id, agent_id").First(&task, taskID).Error; err != nil {
			write(w, 404, map[string]string{"message": "任务不存在"})
			return
		}
		if task.AgentID != *scoped {
			write(w, 403, map[string]string{"message": "无权运行其他代理的任务"})
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	run, err := a.RunTaskNow(ctx, taskID)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "任务不存在" || err.Error() == "代理不存在" {
			status = http.StatusNotFound
		} else if err.Error() == "任务正在运行" {
			status = http.StatusConflict
		}
		write(w, status, map[string]string{"message": err.Error()})
		return
	}
	write(w, 200, run)
}

func (a *API) RunTaskNow(ctx context.Context, taskID uint) (models.SyncRun, error) {
	var run models.SyncRun
	if a.db == nil {
		return run, fmt.Errorf("数据库不可用")
	}
	lockValue, _ := a.locks.LoadOrStore(taskID, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	if !lock.TryLock() {
		return run, fmt.Errorf("任务正在运行")
	}
	defer lock.Unlock()
	var task models.MonitorTask
	if err := a.db.First(&task, taskID).Error; err != nil {
		return run, fmt.Errorf("任务不存在")
	}
	var agent models.Agent
	if err := a.db.First(&agent, task.AgentID).Error; err != nil {
		return run, fmt.Errorf("代理不存在")
	}
	values := parseSourceValues(agent.SourceValues)
	run = models.SyncRun{TaskID: task.ID, Status: "running", StartedAt: time.Now().UTC()}
	if err := a.db.Create(&run).Error; err != nil {
		return run, fmt.Errorf("无法创建运行记录")
	}
	var rows []map[string]any
	kkudSource, sourceErr := a.externalSource("kkud")
	queryErr := sourceErr
	if sourceErr == nil {
		rows, queryErr = kkudSource.QueryRows(ctx, source.KKUDTable, "daili", values, 5000)
	}
	run.RowsRead = len(rows)
	if queryErr != nil {
		run.Status = "failed"
		run.Error = queryErr.Error()
	} else {
		run.Status = "succeeded"
		fenxSource, fenxErr := a.externalSource("fenx")
		for _, row := range rows {
			snapshot := snapshotFromRow(task.ID, row)
			var existing models.KKUDSnapshot
			findErr := a.db.Where("task_id = ? AND source_pk = ?", snapshot.TaskID, snapshot.SourcePK).First(&existing).Error
			if findErr == nil {
				snapshot.ID = existing.ID
				if err := a.db.Save(&snapshot).Error; err != nil {
					run.Status = "partial"
					run.Error = appendError(run.Error, err.Error())
					continue
				}
			} else if findErr == gorm.ErrRecordNotFound {
				if err := a.db.Create(&snapshot).Error; err != nil {
					run.Status = "partial"
					run.Error = appendError(run.Error, err.Error())
					continue
				}
			} else {
				run.Status = "partial"
				run.Error = appendError(run.Error, findErr.Error())
				continue
			}
			run.RowsSaved++
			if fenxErr == nil {
				if err := a.saveMatches(ctx, fenxSource, run.ID, snapshot.ID, snapshot); err != nil {
					run.Status = "partial"
					run.Error = appendError(run.Error, err.Error())
				}
			}
		}
	}
	finished := time.Now().UTC()
	run.FinishedAt = &finished
	if err := a.db.Save(&run).Error; err != nil {
		return run, fmt.Errorf("运行记录保存失败")
	}
	// 采集+关联完成后自动执行 IP 冲突检测（§6.4）
	if agent.Enabled {
		detectCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		if _, detectErr := a.DetectConflicts(detectCtx, agent.ID); detectErr != nil {
			run.Error = appendError(run.Error, "冲突检测失败: "+detectErr.Error())
			a.db.Save(&run)
		}
		cancel()
	}
	return run, nil
}

func (a *API) saveMatches(ctx context.Context, fenx source.MySQLSource, runID, snapshotID uint, snapshot models.KKUDSnapshot) error {
	rows, err := fenx.QueryIdentityRows(ctx, source.FenxUsersTable, snapshot.NormalizedMobile, snapshot.NormalizedQQ, snapshot.NormalizedEmail, 20)
	if err != nil {
		return err
	}
	type candidate struct {
		userID  string
		matched []string
	}
	candidates := make([]candidate, 0, len(rows))
	for _, row := range rows {
		userID := lookupRow(row, "id", "uid", "userid", "user_id")
		if userID == "" {
			continue
		}
		matched := make([]string, 0, 3)
		if snapshot.NormalizedMobile != "" && normalizeMobile(lookupRow(row, "mobile", "phone", "tel", "telephone")) == snapshot.NormalizedMobile {
			matched = append(matched, "mobile")
		}
		if snapshot.NormalizedQQ != "" && normalizeQQ(lookupRow(row, "qq", "qq_num")) == snapshot.NormalizedQQ {
			matched = append(matched, "qq")
		}
		if snapshot.NormalizedEmail != "" && normalizeEmail(lookupRow(row, "email", "mail")) == snapshot.NormalizedEmail {
			matched = append(matched, "email")
		}
		if len(matched) == 0 {
			continue
		}
		candidates = append(candidates, candidate{userID: userID, matched: matched})
	}
	// 同一 kkud 用户命中多个 fenx 账号时标记 ambiguous，不进入自动治理（§6.3）
	ambiguous := len(candidates) > 1
	for _, item := range candidates {
		confidence := 0.55
		state := "review"
		for _, field := range item.matched {
			if field == "mobile" {
				confidence = 0.95
				state = "matched"
				break
			}
		}
		if ambiguous {
			state = "ambiguous"
		}
		match := models.UserMatch{RunID: runID, KKUDSnapshotID: snapshotID, FenxUserID: item.userID, Confidence: confidence, MatchedFields: strings.Join(item.matched, ","), State: state}
		if err := a.db.Create(&match).Error; err != nil {
			return err
		}
	}
	return nil
}

func lookupRow(row map[string]any, names ...string) string {
	for key, value := range row {
		for _, name := range names {
			if strings.EqualFold(key, name) {
				return strings.TrimSpace(fmt.Sprint(value))
			}
		}
	}
	return ""
}

func (a *API) StartScheduler(ctx context.Context) {
	if a.db == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				var tasks []models.MonitorTask
				a.db.Where("enabled = ? AND (next_run_at IS NULL OR next_run_at <= ?)", true, now.UTC()).Find(&tasks)
				for _, task := range tasks {
					runCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
					_, _ = a.RunTaskNow(runCtx, task.ID)
					cancel()
					next := now.UTC().Add(time.Duration(task.IntervalSec) * time.Second)
					a.db.Model(&task).Update("next_run_at", next)
				}
			}
		}
	}()
}

func (a *API) syncRuns(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		write(w, 200, []models.SyncRun{})
		return
	}
	query := a.db.Model(&models.SyncRun{})
	if scoped := a.scopedAgentID(claimsOf(r)); scoped != nil {
		query = query.Joins("JOIN monitor_tasks ON monitor_tasks.id = sync_runs.task_id").Where("monitor_tasks.agent_id = ?", *scoped)
	}
	var rows []models.SyncRun
	query.Order("sync_runs.id desc").Limit(100).Find(&rows)
	write(w, 200, rows)
}

func (a *API) matches(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		write(w, 200, []models.UserMatch{})
		return
	}
	query := a.db.Model(&models.UserMatch{})
	if scoped := a.scopedAgentID(claimsOf(r)); scoped != nil {
		query = query.
			Joins("JOIN sync_runs ON sync_runs.id = user_matches.run_id").
			Joins("JOIN monitor_tasks ON monitor_tasks.id = sync_runs.task_id").
			Where("monitor_tasks.agent_id = ?", *scoped)
	}
	var rows []models.UserMatch
	query.Order("user_matches.id desc").Limit(200).Find(&rows)
	write(w, 200, rows)
}

func (a *API) snapshots(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		write(w, 200, []models.KKUDSnapshot{})
		return
	}
	query := a.db.Model(&models.KKUDSnapshot{})
	if mobile := strings.TrimSpace(r.URL.Query().Get("mobile")); mobile != "" {
		query = query.Where("mobile = ?", mobile)
	}
	if scoped := a.scopedAgentID(claimsOf(r)); scoped != nil {
		// 代理角色强制按本代理 source_values 过滤，忽略传入的 agent_value
		var agent models.Agent
		if err := a.db.Select("id, source_values").First(&agent, *scoped).Error; err != nil {
			write(w, 404, map[string]string{"message": "代理不存在"})
			return
		}
		values := parseSourceValues(agent.SourceValues)
		if len(values) == 0 {
			write(w, 200, []models.KKUDSnapshot{})
			return
		}
		query = query.Where("agent_value IN ?", values)
	} else if agentValue := strings.TrimSpace(r.URL.Query().Get("agent_value")); agentValue != "" {
		query = query.Where("agent_value = ?", agentValue)
	}
	limit := 200
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := parseID(raw); err == nil && parsed <= 500 {
			limit = int(parsed)
		}
	}
	var rows []models.KKUDSnapshot
	query.Order("captured_at desc").Limit(limit).Find(&rows)
	write(w, 200, rows)
}

func parseID(value string) (uint, error) {
	var id uint64
	if value == "" {
		return 0, fmt.Errorf("empty id")
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("invalid id")
		}
		id = id*10 + uint64(r-'0')
	}
	if id == 0 {
		return 0, fmt.Errorf("invalid id")
	}
	return uint(id), nil
}

func parseSourceValues(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var values []string
	if strings.HasPrefix(raw, "[") && json.Unmarshal([]byte(raw), &values) == nil {
		return cleanValues(values)
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' || r == ';' })
	return cleanValues(parts)
}

func cleanValues(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func snapshotFromRow(taskID uint, row map[string]any) models.KKUDSnapshot {
	data, _ := source.MarshalRow(row)
	hash := sha256.Sum256(data)
	sourcePK := lookupRow(row, "id", "uid", "userid", "user_id")
	if sourcePK == "" {
		sourcePK = fmt.Sprintf("row-%x", hash[:12])
	}
	mobile := lookupRow(row, "mobile", "phone", "tel", "telephone")
	qq := lookupRow(row, "qq", "qq_num")
	email := lookupRow(row, "email", "mail")
	return models.KKUDSnapshot{
		TaskID: taskID, SourcePK: sourcePK, AgentValue: lookupRow(row, "daili", "代理", "agent"),
		Mobile: mobile, QQ: qq, Email: email,
		NormalizedMobile: normalizeMobile(mobile), NormalizedQQ: normalizeQQ(qq), NormalizedEmail: normalizeEmail(email),
		DataJSON: string(data), DataHash: fmt.Sprintf("%x", hash[:]), CapturedAt: time.Now().UTC(),
	}
}

func appendError(current, next string) string {
	if current == "" {
		return next
	}
	return current + "; " + next
}
func (a *API) decision(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 32*1024))
	if err != nil || !a.validDecisionSignature(r, body) {
		write(w, 401, map[string]string{"decision": "review", "reason": "invalid_signature"})
		return
	}
	var in struct {
		AgentID   uint   `json:"agent_id"`
		AccountID string `json:"account_id"`
		IP        string `json:"ip"`
		Action    string `json:"action"`
	}
	if json.Unmarshal(body, &in) != nil || in.AccountID == "" || in.Action == "" {
		write(w, 400, map[string]string{"decision": "review"})
		return
	}
	if a.db == nil {
		// 存储不可用时不能静默放行，由 PHP 侧 fail-open 处理
		write(w, 503, map[string]string{"decision": "service_unavailable", "reason": "claim_storage_unavailable"})
		return
	}
	ip, valid, internal := normalizeIP(in.IP)
	if !valid || internal {
		a.writeAudit(nil, "decision.allow", fmt.Sprintf("agent:%d account:%s", in.AgentID, in.AccountID), "internal_or_empty_ip action="+in.Action)
		write(w, 200, map[string]any{"decision": "allow", "reason": "internal_or_empty_ip"})
		return
	}
	var claims []models.IPClaim
	a.db.Where("agent_id = ? AND ip = ?", in.AgentID, ip).Find(&claims)
	for _, claim := range claims {
		if claim.AccountID != in.AccountID {
			write(w, 200, map[string]any{"decision": "deny_duplicate_ip", "reason": "ip_claimed_by_another_account"})
			return
		}
	}
	claim := models.IPClaim{AgentID: in.AgentID, IP: ip, Action: in.Action, AccountID: in.AccountID}
	if err := a.db.Create(&claim).Error; err != nil {
		var existing models.IPClaim
		if a.db.Where("agent_id = ? AND ip = ?", in.AgentID, ip).First(&existing).Error == nil && existing.AccountID != in.AccountID {
			write(w, 200, map[string]any{"decision": "deny_duplicate_ip", "reason": "ip_claimed_by_another_account"})
			return
		}
		if existing.AccountID == in.AccountID {
			write(w, 200, map[string]any{"decision": "allow", "reason": "ip_already_claimed_by_same_account"})
			return
		}
		write(w, 503, map[string]string{"decision": "review", "reason": "claim_storage_unavailable"})
		return
	}
	write(w, 200, map[string]any{"decision": "allow", "reason": "initial_policy", "agent_id": in.AgentID, "account_id": in.AccountID, "action": in.Action})
}

func (a *API) validDecisionSignature(r *http.Request, body []byte) bool {
	supplied := r.Header.Get("X-Fenx-Signature")
	mac := hmac.New(sha256.New, []byte(a.cfg.DecisionSecret))
	_, _ = mac.Write(body)
	want := fmt.Sprintf("sha256=%x", mac.Sum(nil))
	return supplied != "" && hmac.Equal([]byte(supplied), []byte(want))
}
func (a *API) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		claims, err := a.jwt.Parse(token)
		if err != nil {
			write(w, 401, map[string]string{"message": "未登录或登录已过期"})
			return
		}
		next(w, r.WithContext(contextWithClaims(r.Context(), claims)))
	}
}

func (a *API) requireRole(roles ...string) func(http.HandlerFunc) http.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}
	return func(next http.HandlerFunc) http.HandlerFunc {
		return a.requireAuth(func(w http.ResponseWriter, r *http.Request) {
			claims, _ := r.Context().Value(claimsKey{}).(*auth.Claims)
			if claims == nil {
				write(w, http.StatusUnauthorized, map[string]string{"message": "未登录"})
				return
			}
			if _, ok := allowed[claims.Role]; !ok {
				write(w, http.StatusForbidden, map[string]string{"message": "无权执行此操作"})
				return
			}
			next(w, r)
		})
	}
}

type claimsKey struct{}

func contextWithClaims(ctx context.Context, claims *auth.Claims) context.Context {
	return context.WithValue(ctx, claimsKey{}, claims)
}
func write(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}
