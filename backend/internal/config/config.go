package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr      string
	DatabaseURL     string
	RedisURL        string
	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	AdminUsername   string
	AdminPassword   string
	DecisionSecret  string
	KKUDDSNFallback string
	FenxDSNFallback string
	FrontendDist    string
}

func Load() Config {
	ttl, _ := strconv.Atoi(env("JWT_ACCESS_TTL_MINUTES", "120"))
	refreshDays, _ := strconv.Atoi(env("JWT_REFRESH_TTL_DAYS", "30"))
	return Config{
		ListenAddr:      env("BACKEND_ADDR", ":8080"),
		DatabaseURL:     env("DATABASE_URL", "postgres://fenx:fenx@postgres:5432/fenx?sslmode=disable"),
		RedisURL:        env("REDIS_URL", "redis://redis:6379/0"),
		JWTSecret:       env("JWT_SECRET", "change-me-in-production"),
		AccessTokenTTL:  time.Duration(ttl) * time.Minute,
		RefreshTokenTTL: time.Duration(refreshDays) * 24 * time.Hour,
		AdminUsername:   env("INITIAL_ADMIN_USERNAME", "admin"),
		AdminPassword:   env("INITIAL_ADMIN_PASSWORD", "change-me"),
		DecisionSecret:  env("DECISION_SHARED_SECRET", "change-me-decision-secret"),
		KKUDDSNFallback: withConnectTimeout(mysqlDSN("KKUD")),
		FenxDSNFallback: withConnectTimeout(mysqlDSN("FENX")),
		FrontendDist:    env("FRONTEND_DIST", ""),
	}
}

func mysqlDSN(prefix string) string {
	if dsn := os.Getenv(prefix + "_DB_DSN"); dsn != "" {
		return dsn
	}
	host := os.Getenv(prefix + "_DB_HOST")
	name := os.Getenv(prefix + "_DB_NAME")
	user := os.Getenv(prefix + "_DB_USER")
	if host == "" || name == "" || user == "" {
		return ""
	}
	port := env(prefix+"_DB_PORT", "3306")
	return user + ":" + os.Getenv(prefix+"_DB_PASSWORD") + "@tcp(" + host + ":" + port + ")/" + name + "?charset=utf8mb4&parseTime=true"
}

// withConnectTimeout 远程 MySQL 连接较慢，统一补连接超时参数（已有则不覆盖）。
func withConnectTimeout(dsn string) string {
	if dsn == "" || strings.Contains(dsn, "timeout=") {
		return dsn
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + "timeout=15s"
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
