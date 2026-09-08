package httpapi

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"fenx/backend/internal/models"
	"fenx/backend/internal/source"
)

// kkudUserWritableColumns 允许写入的 kkud 用户列（JSON 字段名 → 实测列名常量）。
var kkudUserWritableColumns = map[string]string{
	"user": "user", "password": "password", "daili": "daili", "dailiurl": "dailiurl",
	"adminurl": "adminurl", "admin": "admin", "tgjifen": "tgjifen", "superadmin": "superadmin",
	"vip": "vip", "tgip": "tgip", "mac": "mac", "qq": "QQ", "jine": "jine", "txjl": "txjl",
}

// kkudVIPValue 规范化 VIP 值：true/"1" → '1'，其余 → ”（该列语义：空串=非 VIP）。
func kkudVIPValue(value any) string {
	switch v := value.(type) {
	case bool:
		if v {
			return "1"
		}
	case string:
		if v == "1" || strings.EqualFold(v, "true") {
			return "1"
		}
	case float64:
		if v == 1 {
			return "1"
		}
	}
	return ""
}

func (a *API) kkudSource(w http.ResponseWriter) (source.MySQLSource, bool) {
	kkud, err := a.externalSource("kkud")
	if err != nil {
		write(w, 503, map[string]string{"message": err.Error()})
		return kkud, false
	}
	return kkud, true
}

// kkudUsers 直连 kkud 库查询用户（仅超管；password 列脱敏为 ***）。
func (a *API) kkudUsers(w http.ResponseWriter, r *http.Request) {
	kkud, ok := a.kkudSource(w)
	if !ok {
		return
	}
	conditions := []string{"1=1"}
	args := []any{}
	if user := strings.TrimSpace(r.URL.Query().Get("user")); user != "" {
		conditions = append(conditions, "`user` LIKE ?")
		args = append(args, "%"+user+"%")
	}
	if daili := strings.TrimSpace(r.URL.Query().Get("daili")); daili != "" {
		conditions = append(conditions, "`daili` = ?")
		args = append(args, daili)
	}
	if vip := strings.TrimSpace(r.URL.Query().Get("vip")); vip == "true" || vip == "1" {
		conditions = append(conditions, "`vip` = '1'")
	}
	limit, offset := pageParams(r, 50, 200)
	args = append(args, limit, offset)
	rows, err := kkud.QueryMaps(r.Context(), "SELECT * FROM `"+source.KKUDTable+"` WHERE "+strings.Join(conditions, " AND ")+" ORDER BY `id` DESC LIMIT ? OFFSET ?", args...)
	if err != nil {
		source.InvalidateTableColumns(kkud.DSN, source.KKUDTable)
		write(w, 502, map[string]string{"message": "查询失败"})
		return
	}
	for _, row := range rows {
		if _, exists := row["password"]; exists {
			row["password"] = "***"
		}
	}
	write(w, 200, map[string]any{"items": rows, "limit": limit, "offset": offset})
}

// createKKUDUser 创建 kkud 用户（仅超管；必填 user/password，明文存储——该系统设计如此）。
func (a *API) createKKUDUser(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		write(w, 503, map[string]string{"message": "数据库不可用"})
		return
	}
	kkud, ok := a.kkudSource(w)
	if !ok {
		return
	}
	var in map[string]any
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		write(w, 400, map[string]string{"message": "参数无效"})
		return
	}
	user := strings.TrimSpace(fmt.Sprint(in["user"]))
	password := fmt.Sprint(in["password"])
	if user == "" || password == "" || password == "<nil>" {
		write(w, 400, map[string]string{"message": "user 和 password 不能为空"})
		return
	}
	columns := []string{"`user`", "`password`"}
	placeholders := []string{"?", "?"}
	args := []any{user, password}
	for field, column := range kkudUserWritableColumns {
		if field == "user" || field == "password" {
			continue
		}
		value, exists := in[field]
		if !exists {
			continue
		}
		if field == "vip" {
			value = kkudVIPValue(value)
		}
		columns = append(columns, "`"+column+"`")
		placeholders = append(placeholders, "?")
		args = append(args, value)
	}
	id, err := kkud.Insert(r.Context(), "INSERT INTO `"+source.KKUDTable+"` ("+strings.Join(columns, ", ")+") VALUES ("+strings.Join(placeholders, ", ")+")", args...)
	if err != nil {
		write(w, 502, map[string]string{"message": "创建失败"})
		return
	}
	claims := claimsOf(r)
	a.writeAudit(&claims.UserID, "kkud.create_user", fmt.Sprintf("kkud_user:%d", id), fmt.Sprintf("user=%s daili=%s", user, fmt.Sprint(in["daili"])))
	write(w, 201, map[string]any{"id": id, "user": user})
}

// updateKKUDUser 白名单更新 kkud 用户可选字段（仅超管；不含 user/password）。
func (a *API) updateKKUDUser(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if _, err := parseID(id); err != nil {
		write(w, 400, map[string]string{"message": "用户编号无效"})
		return
	}
	kkud, ok := a.kkudSource(w)
	if !ok {
		return
	}
	var patch map[string]any
	if json.NewDecoder(r.Body).Decode(&patch) != nil {
		write(w, 400, map[string]string{"message": "参数无效"})
		return
	}
	assignments := []string{}
	args := []any{}
	fields := []string{}
	for field, value := range patch {
		column, allowed := kkudUserWritableColumns[field]
		if !allowed || field == "user" || field == "password" {
			continue
		}
		if field == "vip" {
			value = kkudVIPValue(value)
		}
		assignments = append(assignments, "`"+column+"` = ?")
		args = append(args, value)
		fields = append(fields, field)
	}
	if len(assignments) == 0 {
		write(w, 400, map[string]string{"message": "没有可更新的字段"})
		return
	}
	args = append(args, id)
	affected, err := kkud.Exec(r.Context(), "UPDATE `"+source.KKUDTable+"` SET "+strings.Join(assignments, ", ")+" WHERE `id` = ?", args...)
	if err != nil {
		write(w, 502, map[string]string{"message": "更新失败"})
		return
	}
	if affected == 0 {
		write(w, 404, map[string]string{"message": "kkud 用户不存在或无变化"})
		return
	}
	claims := claimsOf(r)
	a.writeAudit(&claims.UserID, "kkud.update_user", "kkud_user:"+id, "fields="+strings.Join(fields, ","))
	write(w, 200, map[string]any{"id": id, "updated": affected, "fields": fields})
}

// deleteKKUDUser 删除 kkud 用户（仅超管；先把整行归档进 ActionJob.before_json 再删）。
func (a *API) deleteKKUDUser(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		write(w, 503, map[string]string{"message": "数据库不可用"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if _, err := parseID(id); err != nil {
		write(w, 400, map[string]string{"message": "用户编号无效"})
		return
	}
	kkud, ok := a.kkudSource(w)
	if !ok {
		return
	}
	rows, err := kkud.QueryMaps(r.Context(), "SELECT * FROM `"+source.KKUDTable+"` WHERE `id` = ?", id)
	if err != nil {
		write(w, 502, map[string]string{"message": "查询失败"})
		return
	}
	if len(rows) == 0 {
		write(w, 404, map[string]string{"message": "kkud 用户不存在"})
		return
	}
	claims := claimsOf(r)
	beforeJSON, _ := json.Marshal(rows[0])
	job := models.ActionJob{
		ConflictID: 0, Action: "delete_kkud_user", Status: "pending", TargetUID: id,
		BeforeJSON: jsonbPtr(string(beforeJSON)), IdempotencyKey: fmt.Sprintf("kkud-delete:%s:%d", id, time.Now().UTC().UnixNano()),
		OperatorID: &claims.UserID, CreatedAt: time.Now().UTC(),
	}
	if err := a.db.Create(&job).Error; err != nil {
		log.Printf("kkud delete archive failed for user %s: %v", id, err)
		write(w, 500, map[string]string{"message": "归档失败，未执行删除"})
		return
	}
	if _, err := kkud.Exec(r.Context(), "DELETE FROM `"+source.KKUDTable+"` WHERE `id` = ?", id); err != nil {
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
	a.writeAudit(&claims.UserID, "kkud.delete_user", "kkud_user:"+id, fmt.Sprintf("action_job=%d user=%s", job.ID, fmt.Sprint(rows[0]["user"])))
	write(w, 200, map[string]any{"id": id, "deleted": true, "action_job_id": job.ID})
}
