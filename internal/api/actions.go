package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *server) handleScale(w http.ResponseWriter, r *http.Request) {
	if s.d.Actions == nil {
		writeErr(w, http.StatusServiceUnavailable, "未连接集群")
		return
	}
	ns, kind, name := chi.URLParam(r, "ns"), chi.URLParam(r, "kind"), chi.URLParam(r, "name")
	var body struct {
		Replicas *int32 `json:"replicas"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Replicas == nil {
		writeErr(w, http.StatusBadRequest, "请求体需包含 replicas")
		return
	}
	if err := s.d.Actions.Scale(r.Context(), ns, kind, name, *body.Replicas); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	slog.Info("扩缩容", "ns", ns, "kind", kind, "name", name, "replicas", *body.Replicas) // 写操作审计
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "replicas": *body.Replicas})
}

func (s *server) handleRestart(w http.ResponseWriter, r *http.Request) {
	if s.d.Actions == nil {
		writeErr(w, http.StatusServiceUnavailable, "未连接集群")
		return
	}
	ns, kind, name := chi.URLParam(r, "ns"), chi.URLParam(r, "kind"), chi.URLParam(r, "name")
	if err := s.d.Actions.Restart(r.Context(), ns, kind, name); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	slog.Info("重启", "ns", ns, "kind", kind, "name", name)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *server) handleDeletePod(w http.ResponseWriter, r *http.Request) {
	if s.d.Actions == nil {
		writeErr(w, http.StatusServiceUnavailable, "未连接集群")
		return
	}
	ns, name := chi.URLParam(r, "ns"), chi.URLParam(r, "name")
	if err := s.d.Actions.DeletePod(r.Context(), ns, name); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	slog.Info("删除 Pod", "ns", ns, "name", name)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
