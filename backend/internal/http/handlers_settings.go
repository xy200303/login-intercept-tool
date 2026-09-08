package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// getExternalDBSettings 读取外连数据库配置（仅超管；永不返回密码明文）。
func (a *API) getExternalDBSettings(w http.ResponseWriter, _ *http.Request) {
	if a.db == nil {
		write(w, 503, map[string]string{"message": "数据库不可用"})
		return
	}
	view := func(prefix string) map[string]any {
		cfg, _ := a.loadExternalConfig(prefix)
		return map[string]any{
			"host": cfg.Host, "port": cfg.Port, "name": cfg.Name, "user": cfg.User,
			"has_password": cfg.Password != "",
		}
	}
	write(w, 200, map[string]any{"kkud": view("kkud"), "fenx": view("fenx")})
}

// putExternalDBSettings 保存外连数据库配置（仅超管；password 空串=保持原值；保存后失效旧连接池）。
func (a *API) putExternalDBSettings(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		write(w, 503, map[string]string{"message": "数据库不可用"})
		return
	}
	var in struct {
		KKUD externalDBConfig `json:"kkud"`
		Fenx externalDBConfig `json:"fenx"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		write(w, 400, map[string]string{"message": "参数无效"})
		return
	}
	claims := claimsOf(r)
	updated := make([]string, 0, 2)
	if in.KKUD.complete() {
		oldDSN, newDSN, err := a.saveExternalConfig("kkud", in.KKUD, claims.UserID)
		if err != nil {
			write(w, 500, map[string]string{"message": "kkud 配置保存失败"})
			return
		}
		invalidateExternalPool(oldDSN, newDSN)
		updated = append(updated, "kkud")
	}
	if in.Fenx.complete() {
		oldDSN, newDSN, err := a.saveExternalConfig("fenx", in.Fenx, claims.UserID)
		if err != nil {
			write(w, 500, map[string]string{"message": "fenx 配置保存失败"})
			return
		}
		invalidateExternalPool(oldDSN, newDSN)
		updated = append(updated, "fenx")
	}
	if len(updated) == 0 {
		write(w, 400, map[string]string{"message": "kkud/fenx 配置均需 host、name、user"})
		return
	}
	detail := fmt.Sprintf("updated=%s kkud=%s@%s/%s fenx=%s@%s/%s",
		strings.Join(updated, ","),
		in.KKUD.User, in.KKUD.Host, in.KKUD.Name, in.Fenx.User, in.Fenx.Host, in.Fenx.Name)
	a.writeAudit(&claims.UserID, "settings.external_db", "sys_config", detail)
	write(w, 200, map[string]any{"updated": updated})
}
