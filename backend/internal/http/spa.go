package httpapi

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// WithSPA 把 API 处理器包上一层 SPA 静态托管：
// /api/ 走 API，其余路径先尝试静态文件，找不到回退到 index.html（history 路由）。
// dist 为空时不启用静态托管。
func WithSPA(dist string, api http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || dist == "" {
			api.ServeHTTP(w, r)
			return
		}
		clean := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		full := filepath.Join(dist, clean)
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			http.ServeFile(w, r, full)
			return
		}
		http.ServeFile(w, r, filepath.Join(dist, "index.html"))
	})
}
