package httpapi

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"fenx/backend/internal/models"
	"fenx/backend/internal/source"
	"gorm.io/gorm/clause"
)

// externalDBConfig 外连 MySQL 的连接配置（存 sys_config 表，键为 <prefix>_host 等）。
type externalDBConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	Name     string `json:"name"`
	User     string `json:"user"`
	Password string `json:"password,omitempty"`
}

func (c externalDBConfig) complete() bool {
	return strings.TrimSpace(c.Host) != "" && strings.TrimSpace(c.Name) != "" && strings.TrimSpace(c.User) != ""
}

func (c externalDBConfig) dsn() string {
	port := strings.TrimSpace(c.Port)
	if port == "" {
		port = "3306"
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true&timeout=15s",
		strings.TrimSpace(c.User), c.Password, strings.TrimSpace(c.Host), port, strings.TrimSpace(c.Name))
}

func externalConfigKeys(prefix string) []string {
	return []string{prefix + "_host", prefix + "_port", prefix + "_name", prefix + "_user", prefix + "_password"}
}

// loadExternalConfig 从 sys_config 读取外连配置；host/name/user 齐全才视为已配置。
func (a *API) loadExternalConfig(prefix string) (externalDBConfig, bool) {
	var cfg externalDBConfig
	if a.db == nil {
		return cfg, false
	}
	var rows []models.SysConfig
	if err := a.db.Where("key IN ?", externalConfigKeys(prefix)).Find(&rows).Error; err != nil {
		return cfg, false
	}
	for _, row := range rows {
		switch row.Key {
		case prefix + "_host":
			cfg.Host = row.Value
		case prefix + "_port":
			cfg.Port = row.Value
		case prefix + "_name":
			cfg.Name = row.Value
		case prefix + "_user":
			cfg.User = row.Value
		case prefix + "_password":
			cfg.Password = row.Value
		}
	}
	return cfg, cfg.complete()
}

// saveExternalConfig 覆盖写 sys_config 中该前缀的连接配置；password 为空串保持原值。
// 返回保存前后的 DSN，供调用方失效旧连接池。
func (a *API) saveExternalConfig(prefix string, in externalDBConfig, operatorID uint) (oldDSN, newDSN string, err error) {
	old, oldOK := a.loadExternalConfig(prefix)
	if oldOK {
		oldDSN = old.dsn()
	}
	if strings.TrimSpace(in.Password) == "" {
		in.Password = old.Password
	}
	values := map[string]string{
		prefix + "_host":     strings.TrimSpace(in.Host),
		prefix + "_port":     strings.TrimSpace(in.Port),
		prefix + "_name":     strings.TrimSpace(in.Name),
		prefix + "_user":     strings.TrimSpace(in.User),
		prefix + "_password": in.Password,
	}
	now := time.Now().UTC()
	for key, value := range values {
		row := models.SysConfig{Key: key, Value: value, UpdatedAt: now, UpdatedBy: &operatorID}
		if err := a.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at", "updated_by"}),
		}).Create(&row).Error; err != nil {
			return oldDSN, "", fmt.Errorf("配置保存失败")
		}
	}
	newCfg, ok := a.loadExternalConfig(prefix)
	if ok {
		newDSN = newCfg.dsn()
	}
	return oldDSN, newDSN, nil
}

var errExternalDBNotConfigured = fmt.Errorf("未配置外连数据库，请先在系统设置中配置")

type externalSourceEntry struct {
	src     source.MySQLSource
	err     error
	expires time.Time
}

var externalSourceCache = struct {
	sync.RWMutex
	entries map[string]externalSourceEntry
}{entries: map[string]externalSourceEntry{}}

const externalSourceTTL = 5 * time.Minute

// externalSource 解析外连数据源（进程内缓存 5 分钟；PUT settings 保存后立即失效）。
// 优先 sys_config，未配置时回退环境变量（首次引导用）。name 为 "kkud" 或 "fenx"。
func (a *API) externalSource(name string) (source.MySQLSource, error) {
	externalSourceCache.RLock()
	entry, ok := externalSourceCache.entries[name]
	externalSourceCache.RUnlock()
	if ok && time.Now().Before(entry.expires) {
		return entry.src, entry.err
	}
	src, err := a.resolveExternalSource(name)
	externalSourceCache.Lock()
	externalSourceCache.entries[name] = externalSourceEntry{src, err, time.Now().Add(externalSourceTTL)}
	externalSourceCache.Unlock()
	return src, err
}

// invalidateExternalSource 失效指定数据源（"kkud"/"fenx"）的 resolver 缓存。
func invalidateExternalSource(name string) {
	externalSourceCache.Lock()
	delete(externalSourceCache.entries, name)
	externalSourceCache.Unlock()
}

// resolveExternalSource 实际解析逻辑（sys_config 优先，env 兜底）。
func (a *API) resolveExternalSource(name string) (source.MySQLSource, error) {
	display := name
	if name == "fenx" {
		display = "fenx_site"
	}
	if cfg, ok := a.loadExternalConfig(name); ok {
		return source.MySQLSource{Name: display, DSN: cfg.dsn()}, nil
	}
	fallback := a.cfg.KKUDDSNFallback
	if name == "fenx" {
		fallback = a.cfg.FenxDSNFallback
	}
	if strings.TrimSpace(fallback) != "" {
		return source.MySQLSource{Name: display, DSN: fallback}, nil
	}
	return source.MySQLSource{Name: display}, errExternalDBNotConfigured
}

// invalidateExternalPool 配置变更后失效旧连接池与旧 DSN 的表结构元数据缓存
// （新旧 DSN 不同才需要）。
func invalidateExternalPool(oldDSN, newDSN string) {
	if oldDSN != "" && oldDSN != newDSN {
		source.InvalidatePool(oldDSN)
		for _, table := range []string{source.KKUDTable, source.FenxUsersTable, source.FenxLoginLogTable} {
			source.InvalidateTableColumns(oldDSN, table)
		}
	}
}
