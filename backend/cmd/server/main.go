package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"fenx/backend/internal/auth"
	"fenx/backend/internal/config"
	"fenx/backend/internal/db"
	httpapi "fenx/backend/internal/http"
	"fenx/backend/internal/models"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func main() {
	cfg := config.Load()
	database, err := db.Open(cfg)
	if err != nil {
		log.Printf("postgres unavailable, starting with health-only mode: %v", err)
		database = nil
	}
	if database != nil {
		if err := db.Migrate(database); err != nil {
			log.Fatal(err)
		}
		ensureAdminUser(database, cfg)
	}
	redisOptions, err := redis.ParseURL(cfg.RedisURL)
	if err == nil {
		redisClient := redis.NewClient(redisOptions)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if err := redisClient.Ping(ctx).Err(); err != nil {
			log.Printf("redis unavailable, queue features degraded: %v", err)
		}
		cancel()
		_ = redisClient.Close()
	}

	jwtManager := auth.NewManager(cfg.JWTSecret, cfg.AccessTokenTTL)
	api := httpapi.New(cfg, database, jwtManager)
	server := &http.Server{Addr: cfg.ListenAddr, Handler: withCORS(httpapi.WithSPA(cfg.FrontendDist, api.Routes())), ReadHeaderTimeout: 10 * time.Second}
	log.Printf("fenx backend listening on %s", cfg.ListenAddr)
	log.Fatal(server.ListenAndServe())
}

// ensureAdminUser 把环境变量超管 upsert 成 platform_users 行（仅不存在时创建），
// 让超管也走统一用户体系（refresh token 需要 user_id）。登录校验仍是环境变量优先，
// 所以改 INITIAL_ADMIN_PASSWORD 后新密码立即可用；库里的哈希只用于 refresh/审计关联。
func ensureAdminUser(database *gorm.DB, cfg config.Config) {
	var existing models.PlatformUser
	if err := database.Where("username = ?", cfg.AdminUsername).First(&existing).Error; err == nil {
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("admin user bootstrap skipped: %v", err)
		return
	}
	user := models.PlatformUser{Username: cfg.AdminUsername, PasswordHash: string(hash), Role: "super_admin", Status: "active"}
	if err := database.Create(&user).Error; err != nil {
		log.Printf("admin user bootstrap skipped: %v", err)
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", os.Getenv("FRONTEND_ORIGIN"))
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Idempotency-Key")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
