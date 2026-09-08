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
	"github.com/redis/go-redis/v9"
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
	api.StartScheduler(context.Background())
	server := &http.Server{Addr: cfg.ListenAddr, Handler: withCORS(httpapi.WithSPA(cfg.FrontendDist, api.Routes())), ReadHeaderTimeout: 10 * time.Second}
	log.Printf("fenx backend listening on %s", cfg.ListenAddr)
	log.Fatal(server.ListenAndServe())
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
