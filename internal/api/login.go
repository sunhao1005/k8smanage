package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// handleLogin 校验账号密码，成功返回会话令牌。无需鉴权。
func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if s.d.Auth == nil || !s.d.Auth.LoginEnabled() {
		writeErr(w, http.StatusBadRequest, "未启用账号密码登录")
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "请求体不是合法 JSON")
		return
	}
	token, ok := s.d.Auth.Login(body.Username, body.Password)
	if !ok {
		slog.Warn("登录失败", "user", body.Username)
		writeErr(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	slog.Info("登录成功", "user", body.Username)
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

// handleAuthConfig 告诉前端是否启用鉴权 / 是否支持账密登录。无需鉴权。
func (s *server) handleAuthConfig(w http.ResponseWriter, _ *http.Request) {
	enabled, login := false, false
	if s.d.Auth != nil {
		enabled = s.d.Auth.Enabled()
		login = s.d.Auth.LoginEnabled()
	}
	writeJSON(w, http.StatusOK, map[string]bool{"authEnabled": enabled, "loginEnabled": login})
}
