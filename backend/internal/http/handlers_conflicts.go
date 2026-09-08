package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"fenx/backend/internal/auth"
	"fenx/backend/internal/models"
	"fenx/backend/internal/source"
)

type agentPolicy struct {
	Action string `json:"action"` // alert | disable | delete
}

func parsePolicy(raw string) agentPolicy {
	policy := agentPolicy{Action: "alert"}
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &policy)
	}
	if policy.Action != "disable" && policy.Action != "delete" {
		policy.Action = "alert"
	}
	return policy
}

func claimsOf(r *http.Request) *auth.Claims {
	claims, _ := r.Context().Value(claimsKey{}).(*auth.Claims)
	return claims
}

// scopedAgentID 返回代理角色被限制查看的 agent_id；超管/运营返回 nil（不限制）。
func (a *API) scopedAgentID(claims *auth.Claims) *uint {
	if claims == nil || claims.Role != "agent" || a.db == nil {
		return nil
	}
	var user models.PlatformUser
	if err := a.db.Select("id, agent_id").First(&user, claims.UserID).Error; err != nil || user.AgentID == nil {
		empty := uint(0)
		return &empty
	}
	return user.AgentID
}

func (a *API) writeAudit(actorID *uint, action, target, detail string) {
	if a.db == nil {
		return
	}
	_ = a.db.Create(&models.AuditEvent{ActorID: actorID, Action: action, Target: target, Detail: detail, CreatedAt: time.Now().UTC()}).Error
}

func pageParams(r *http.Request, defaultLimit, maxLimit int) (limit, offset int) {
	limit = defaultLimit
	if parsed, err := parseID(r.URL.Query().Get("limit")); err == nil && int(parsed) <= maxLimit {
		limit = int(parsed)
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	return limit, offset
}

func (a *API) conflicts(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		write(w, 503, map[string]string{"message": "数据库不可用"})
		return
	}
	query := a.db.Model(&models.IPConflict{})
	if scoped := a.scopedAgentID(claimsOf(r)); scoped != nil {
		query = query.Where("agent_id = ?", *scoped)
	} else if raw := strings.TrimSpace(r.URL.Query().Get("agent_id")); raw != "" {
		if id, err := parseID(raw); err == nil {
			query = query.Where("agent_id = ?", id)
		}
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" {
		query = query.Where("status = ?", status)
	}
	if risk := strings.TrimSpace(r.URL.Query().Get("risk")); risk != "" {
		query = query.Where("risk = ?", risk)
	}
	if ip := strings.TrimSpace(r.URL.Query().Get("ip")); ip != "" {
		query = query.Where("ip = ?", ip)
	}
	var total int64
	query.Count(&total)
	limit, offset := pageParams(r, 50, 200)
	var rows []models.IPConflict
	query.Order("last_seen_at desc").Limit(limit).Offset(offset).Find(&rows)
	write(w, 200, map[string]any{"items": rows, "total": total, "limit": limit, "offset": offset})
}

func (a *API) conflictDetail(w http.ResponseWriter, r *http.Request) {
	conflict, ok := a.loadConflictScoped(w, r)
	if !ok {
		return
	}
	var members []models.ConflictMember
	a.db.Where("conflict_id = ?", conflict.ID).Order("is_winner desc, fenx_uid").Find(&members)
	write(w, 200, map[string]any{"conflict": conflict, "members": members})
}

func (a *API) loadConflictScoped(w http.ResponseWriter, r *http.Request) (models.IPConflict, bool) {
	var conflict models.IPConflict
	if a.db == nil {
		write(w, 503, map[string]string{"message": "数据库不可用"})
		return conflict, false
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		write(w, 400, map[string]string{"message": "冲突编号无效"})
		return conflict, false
	}
	if err := a.db.First(&conflict, id).Error; err != nil {
		write(w, 404, map[string]string{"message": "冲突不存在"})
		return conflict, false
	}
	if scoped := a.scopedAgentID(claimsOf(r)); scoped != nil && conflict.AgentID != *scoped {
		write(w, 403, map[string]string{"message": "无权查看该冲突"})
		return conflict, false
	}
	return conflict, true
}

// detectNow 手动触发单个代理的冲突检测（超管或该代理本人）。
func (a *API) detectNow(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		write(w, 503, map[string]string{"message": "数据库不可用"})
		return
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		write(w, 400, map[string]string{"message": "代理编号无效"})
		return
	}
	claims := claimsOf(r)
	if claims.Role == "agent" {
		if scoped := a.scopedAgentID(claims); scoped == nil || *scoped != id {
			write(w, 403, map[string]string{"message": "无权检测其他代理"})
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	summary, err := a.DetectConflicts(ctx, id)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "代理不存在" {
			status = http.StatusNotFound
		}
		write(w, status, map[string]string{"message": err.Error()})
		return
	}
	a.writeAudit(&claims.UserID, "conflict.detect", fmt.Sprintf("agent:%d", id), fmt.Sprintf("conflicts=%d created=%d updated=%d reopened=%d", summary.Conflicts, summary.Created, summary.Updated, summary.Reopened))
	write(w, 200, summary)
}

// conflictPreview 按代理策略返回将受影响的账号清单与 before 快照，不写外部库。
func (a *API) conflictPreview(w http.ResponseWriter, r *http.Request) {
	conflict, ok := a.loadConflictScoped(w, r)
	if !ok {
		return
	}
	var agent models.Agent
	if err := a.db.First(&agent, conflict.AgentID).Error; err != nil {
		write(w, 404, map[string]string{"message": "代理不存在"})
		return
	}
	policy := parsePolicy(agent.PolicyJSON)
	var members []models.ConflictMember
	a.db.Where("conflict_id = ?", conflict.ID).Find(&members)
	affected, skipped := partitionMembers(members)
	before := a.fenxBeforeSnapshot(r.Context(), uidsOf(affected))
	for _, row := range before {
		sanitizeFenxRow(row)
	}
	write(w, 200, map[string]any{
		"conflict": conflict,
		"policy":   policy.Action,
		"winner":   conflict.WinnerUID,
		"affected": affected,
		"skipped":  skipped,
		"before":   before,
	})
}

// partitionMembers 拆分执行集合（非 winner 且高置信 matched）与仅展示集合。
func partitionMembers(members []models.ConflictMember) (affected []models.ConflictMember, skipped []map[string]any) {
	for _, member := range members {
		switch {
		case member.IsWinner:
			skipped = append(skipped, map[string]any{"fenx_uid": member.FenxUID, "username": member.Username, "reason": "winner"})
		case member.MatchState != "" && member.MatchState != "matched":
			skipped = append(skipped, map[string]any{"fenx_uid": member.FenxUID, "username": member.Username, "reason": "low_confidence_" + member.MatchState})
		default:
			affected = append(affected, member)
		}
	}
	return affected, skipped
}

func uidsOf(members []models.ConflictMember) []string {
	uids := make([]string, 0, len(members))
	for _, member := range members {
		uids = append(uids, member.FenxUID)
	}
	return uids
}

// fenxBeforeSnapshot 读取执行前的外部库现状（内部使用，含完整字段）。
func (a *API) fenxBeforeSnapshot(ctx context.Context, uids []string) map[string]map[string]any {
	result := map[string]map[string]any{}
	if len(uids) == 0 || strings.TrimSpace(a.cfg.FenxDSN) == "" {
		return result
	}
	fenx := source.MySQLSource{Name: "fenx_site", DSN: a.cfg.FenxDSN}
	pkColumn, err := fenxPKColumn(ctx, fenx, a.cfg.FenxUsersTable)
	if err != nil {
		return result
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(uids)), ",")
	args := make([]any, len(uids))
	for i, uid := range uids {
		args[i] = uid
	}
	rows, err := fenx.QueryMaps(ctx, "SELECT * FROM `"+a.cfg.FenxUsersTable+"` WHERE `"+pkColumn+"` IN ("+placeholders+")", args...)
	if err != nil {
		return result
	}
	for _, row := range rows {
		uid := strings.TrimSpace(fmt.Sprint(row[pkColumn]))
		if uid != "" {
			result[uid] = row
		}
	}
	return result
}

// conflictActions 按策略对非 winner 成员执行禁用/删除（仅超管，幂等 + 二次确认）。
func (a *API) conflictActions(w http.ResponseWriter, r *http.Request) {
	conflict, ok := a.loadConflictScoped(w, r)
	if !ok {
		return
	}
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	var in struct {
		Confirm bool `json:"confirm"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || !in.Confirm {
		write(w, 400, map[string]string{"message": "需要 body 中 confirm=true 二次确认"})
		return
	}
	if idempotencyKey == "" {
		write(w, 400, map[string]string{"message": "缺少 Idempotency-Key 请求头"})
		return
	}
	if len(idempotencyKey) > 120 {
		write(w, 400, map[string]string{"message": "Idempotency-Key 过长"})
		return
	}
	var agent models.Agent
	if err := a.db.First(&agent, conflict.AgentID).Error; err != nil {
		write(w, 404, map[string]string{"message": "代理不存在"})
		return
	}
	policy := parsePolicy(agent.PolicyJSON)
	if policy.Action == "alert" {
		write(w, 200, map[string]any{"policy": "alert", "jobs": []models.ActionJob{}, "message": "当前策略为仅告警，未执行外部写操作"})
		return
	}
	var members []models.ConflictMember
	a.db.Where("conflict_id = ?", conflict.ID).Find(&members)
	affected, skipped := partitionMembers(members)
	claims := claimsOf(r)
	jobs := make([]models.ActionJob, 0, len(affected))
	failures := 0
	for _, member := range affected {
		key := idempotencyKey + ":" + member.FenxUID
		var existing models.ActionJob
		if err := a.db.Where("idempotency_key = ?", key).First(&existing).Error; err == nil {
			jobs = append(jobs, existing)
			continue
		}
		job := a.executeAction(r.Context(), conflict, policy.Action, member, key, claims.UserID)
		if job.Status == "failed" {
			failures++
		}
		jobs = append(jobs, job)
	}
	detail, _ := json.Marshal(map[string]any{"policy": policy.Action, "executed": len(jobs), "failed": failures, "skipped": len(skipped), "idempotency_key": idempotencyKey})
	a.writeAudit(&claims.UserID, "conflict."+policy.Action, fmt.Sprintf("conflict:%d", conflict.ID), string(detail))
	for i := range jobs {
		jobs[i] = sanitizeJobForResponse(jobs[i])
	}
	write(w, 200, map[string]any{"policy": policy.Action, "jobs": jobs, "skipped": skipped, "failed": failures})
}

// executeAction 对单个账号执行禁用或删除并落 ActionJob；删除失败不自动重试。
func (a *API) executeAction(ctx context.Context, conflict models.IPConflict, action string, member models.ConflictMember, key string, operatorID uint) models.ActionJob {
	job := models.ActionJob{
		ConflictID: conflict.ID, Action: action, Status: "pending", TargetUID: member.FenxUID,
		IdempotencyKey: key, OperatorID: &operatorID, CreatedAt: time.Now().UTC(),
	}
	before := a.fenxBeforeSnapshot(ctx, []string{member.FenxUID})
	if row, ok := before[member.FenxUID]; ok {
		data, _ := json.Marshal(row)
		job.BeforeJSON = string(data)
	}
	if err := a.db.Create(&job).Error; err != nil {
		var existing models.ActionJob
		if a.db.Where("idempotency_key = ?", key).First(&existing).Error == nil {
			return existing
		}
		job.Status = "failed"
		job.Error = "任务记录创建失败"
		return job
	}
	executed := time.Now().UTC()
	job.ExecutedAt = &executed
	fail := func(message string) models.ActionJob {
		job.Status = "failed"
		job.Error = message
		a.db.Save(&job)
		return job
	}
	fenx := source.MySQLSource{Name: "fenx_site", DSN: a.cfg.FenxDSN}
	pkColumn, err := fenxPKColumn(ctx, fenx, a.cfg.FenxUsersTable)
	if err != nil {
		return fail("外部库不可用")
	}
	switch action {
	case "disable":
		if _, err := fenx.Exec(ctx, "UPDATE `"+a.cfg.FenxUsersTable+"` SET `status` = ? WHERE `"+pkColumn+"` = ?", a.cfg.FenxDisabledStatus, member.FenxUID); err != nil {
			return fail("禁用失败")
		}
	case "delete":
		err := fenx.WithTx(ctx, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, "DELETE FROM `"+a.cfg.FenxUsersTable+"` WHERE `"+pkColumn+"` = ?", member.FenxUID)
			return err
		})
		if err != nil {
			return fail("删除失败")
		}
	default:
		return fail("未知动作")
	}
	job.Status = "succeeded"
	after := a.fenxBeforeSnapshot(ctx, []string{member.FenxUID})
	if row, okRow := after[member.FenxUID]; okRow {
		data, _ := json.Marshal(row)
		job.AfterJSON = string(data)
	}
	a.db.Save(&job)
	return job
}

// undoDisable 撤销成功的禁用，把 users.status 恢复为 before_json 原值（仅超管）。
func (a *API) undoDisable(w http.ResponseWriter, r *http.Request) {
	job, ok := a.loadActionJob(w, r)
	if !ok {
		return
	}
	if job.Action != "disable" || job.Status != "succeeded" {
		write(w, 409, map[string]string{"message": "仅成功的禁用动作可撤销"})
		return
	}
	var before map[string]any
	if err := json.Unmarshal([]byte(job.BeforeJSON), &before); err != nil {
		write(w, 409, map[string]string{"message": "缺少执行前快照，无法撤销"})
		return
	}
	originalStatus := strings.TrimSpace(fmt.Sprint(before["status"]))
	if originalStatus == "" {
		write(w, 409, map[string]string{"message": "执行前快照缺少原状态"})
		return
	}
	fenx := source.MySQLSource{Name: "fenx_site", DSN: a.cfg.FenxDSN}
	pkColumn, err := fenxPKColumn(r.Context(), fenx, a.cfg.FenxUsersTable)
	if err != nil {
		write(w, 502, map[string]string{"message": "外部库不可用"})
		return
	}
	if _, err := fenx.Exec(r.Context(), "UPDATE `"+a.cfg.FenxUsersTable+"` SET `status` = ? WHERE `"+pkColumn+"` = ?", originalStatus, job.TargetUID); err != nil {
		write(w, 502, map[string]string{"message": "恢复状态失败"})
		return
	}
	job.Status = "undone"
	after := a.fenxBeforeSnapshot(r.Context(), []string{job.TargetUID})
	if row, okRow := after[job.TargetUID]; okRow {
		data, _ := json.Marshal(row)
		job.AfterJSON = string(data)
	}
	a.db.Save(&job)
	claims := claimsOf(r)
	a.writeAudit(&claims.UserID, "action.undo_disable", fmt.Sprintf("action_job:%d", job.ID), fmt.Sprintf("target_uid=%s restored_status=%s", job.TargetUID, originalStatus))
	write(w, 200, sanitizeJobForResponse(job))
}

// retryAction 重试失败的禁用动作（仅超管；删除动作按设计不自动重试）。
func (a *API) retryAction(w http.ResponseWriter, r *http.Request) {
	job, ok := a.loadActionJob(w, r)
	if !ok {
		return
	}
	if job.Action != "disable" || job.Status != "failed" {
		write(w, 409, map[string]string{"message": "仅失败的禁用动作可重试"})
		return
	}
	fenx := source.MySQLSource{Name: "fenx_site", DSN: a.cfg.FenxDSN}
	pkColumn, err := fenxPKColumn(r.Context(), fenx, a.cfg.FenxUsersTable)
	if err != nil {
		write(w, 502, map[string]string{"message": "外部库不可用"})
		return
	}
	if _, err := fenx.Exec(r.Context(), "UPDATE `"+a.cfg.FenxUsersTable+"` SET `status` = ? WHERE `"+pkColumn+"` = ?", a.cfg.FenxDisabledStatus, job.TargetUID); err != nil {
		job.Error = "禁用失败"
		a.db.Save(&job)
		write(w, 502, map[string]string{"message": "重试仍失败"})
		return
	}
	executed := time.Now().UTC()
	job.ExecutedAt = &executed
	job.Status = "succeeded"
	job.Error = ""
	after := a.fenxBeforeSnapshot(r.Context(), []string{job.TargetUID})
	if row, okRow := after[job.TargetUID]; okRow {
		data, _ := json.Marshal(row)
		job.AfterJSON = string(data)
	}
	a.db.Save(&job)
	claims := claimsOf(r)
	a.writeAudit(&claims.UserID, "action.retry", fmt.Sprintf("action_job:%d", job.ID), fmt.Sprintf("target_uid=%s result=succeeded", job.TargetUID))
	write(w, 200, sanitizeJobForResponse(job))
}

func (a *API) loadActionJob(w http.ResponseWriter, r *http.Request) (models.ActionJob, bool) {
	var job models.ActionJob
	if a.db == nil {
		write(w, 503, map[string]string{"message": "数据库不可用"})
		return job, false
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		write(w, 400, map[string]string{"message": "动作编号无效"})
		return job, false
	}
	if err := a.db.First(&job, id).Error; err != nil {
		write(w, 404, map[string]string{"message": "动作记录不存在"})
		return job, false
	}
	return job, true
}
