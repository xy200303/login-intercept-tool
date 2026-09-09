package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"fenx/backend/internal/models"
	"gorm.io/gorm/clause"
)

var guardConfigCache = struct {
	sync.RWMutex
	apiURL  string
	apiKey  string
	expires time.Time
}{}

const guardConfigTTL = 5 * time.Minute

// guardConfig 从 sys_config 读取拦截记录接口配置（进程内缓存 5 分钟，保存后立即失效）。
func (a *API) guardConfig() (apiURL, apiKey string) {
	guardConfigCache.RLock()
	if time.Now().Before(guardConfigCache.expires) {
		apiURL, apiKey = guardConfigCache.apiURL, guardConfigCache.apiKey
		guardConfigCache.RUnlock()
		return apiURL, apiKey
	}
	guardConfigCache.RUnlock()
	if a.db != nil {
		var rows []models.SysConfig
		if err := a.db.Where("key IN ?", []string{"guard_api_url", "guard_api_key"}).Find(&rows).Error; err == nil {
			for _, row := range rows {
				switch row.Key {
				case "guard_api_url":
					apiURL = row.Value
				case "guard_api_key":
					apiKey = row.Value
				}
			}
		}
	}
	guardConfigCache.Lock()
	guardConfigCache.apiURL, guardConfigCache.apiKey = apiURL, apiKey
	guardConfigCache.expires = time.Now().Add(guardConfigTTL)
	guardConfigCache.Unlock()
	return apiURL, apiKey
}

// invalidateGuardConfig 保存 guard 配置后失效缓存。
func invalidateGuardConfig() {
	guardConfigCache.Lock()
	guardConfigCache.expires = time.Time{}
	guardConfigCache.Unlock()
}

// setConfigValue upsert 单个 sys_config 键。
func (a *API) setConfigValue(key, value string, operatorID uint) error {
	row := models.SysConfig{Key: key, Value: value, UpdatedAt: time.Now().UTC(), UpdatedBy: &operatorID}
	return a.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at", "updated_by"}),
	}).Create(&row).Error
}

type guardRecord struct {
	Time    string `json:"time"`
	Action  string `json:"action"`
	Account string `json:"account"`
	IP      string `json:"ip"`
}

// guardRecords 代理 PHP 站点的拦截记录接口（fenx_guard_api.php），原样透传记录。
func (a *API) guardRecords(w http.ResponseWriter, r *http.Request) {
	apiURL, apiKey := a.guardConfig()
	if strings.TrimSpace(apiURL) == "" {
		write(w, 503, map[string]string{"message": "未配置拦截记录接口，请先在系统设置中配置"})
		return
	}
	sep := "?"
	if strings.Contains(apiURL, "?") {
		sep = "&"
	}
	endpoint := apiURL + sep + "key=" + url.QueryEscape(apiKey) + "&limit=500"
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		write(w, 502, map[string]string{"message": "拦截记录接口获取失败"})
		return
	}
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		write(w, 502, map[string]string{"message": "拦截记录接口获取失败"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		write(w, 502, map[string]string{"message": "拦截记录接口获取失败"})
		return
	}
	var out struct {
		Records []guardRecord `json:"records"`
		Count   int           `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		write(w, 502, map[string]string{"message": "拦截记录接口返回格式异常"})
		return
	}
	if out.Records == nil {
		out.Records = []guardRecord{}
	}
	write(w, 200, out)
}
