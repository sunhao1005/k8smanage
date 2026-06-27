package api

import (
	"net/http"
	"strings"

	"github.com/sunhao/k8smanage/internal/auth"
)

// authMiddleware 用 Authenticator 校验请求；a 为 nil 或未启用鉴权时放行。
// 凭证来源：Authorization: Bearer <token>，或 ?token=<token>（WebSocket 用）。
func authMiddleware(a *auth.Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if a == nil || !a.Enabled() {
				next.ServeHTTP(w, r)
				return
			}
			if a.Valid(credFrom(r)) {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		})
	}
}

func credFrom(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return r.URL.Query().Get("token")
}
