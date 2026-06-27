package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/sunhao/k8smanage/internal/alert"
)

func (s *server) handleListRules(w http.ResponseWriter, r *http.Request) {
	if s.d.Rules == nil {
		writeErr(w, http.StatusServiceUnavailable, "未启用告警")
		return
	}
	rules, err := s.d.Rules.List(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if rules == nil {
		rules = []alert.AlertRule{}
	}
	writeJSON(w, http.StatusOK, rules)
}

func (s *server) handleUpsertRule(w http.ResponseWriter, r *http.Request) {
	if s.d.Rules == nil {
		writeErr(w, http.StatusServiceUnavailable, "未启用告警")
		return
	}
	var rule alert.AlertRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeErr(w, http.StatusBadRequest, "请求体不是合法 JSON")
		return
	}
	if rule.Kind != "node" && rule.Kind != "pod" {
		writeErr(w, http.StatusBadRequest, "kind 必须为 node 或 pod")
		return
	}
	if rule.Cmp != alert.GT && rule.Cmp != alert.LT {
		writeErr(w, http.StatusBadRequest, "cmp 必须为 > 或 <")
		return
	}
	if rule.ID == "" {
		rule.ID = uuid.NewString()
	}
	if err := s.d.Rules.Upsert(r.Context(), rule); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	slog.Info("保存告警规则", "id", rule.ID, "name", rule.Name)
	writeJSON(w, http.StatusOK, rule)
}

func (s *server) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	if s.d.Rules == nil {
		writeErr(w, http.StatusServiceUnavailable, "未启用告警")
		return
	}
	id := chi.URLParam(r, "id")
	if err := s.d.Rules.Delete(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	slog.Info("删除告警规则", "id", id)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *server) handleActiveAlerts(w http.ResponseWriter, r *http.Request) {
	if s.d.Alerts == nil {
		writeJSON(w, http.StatusOK, []alert.ActiveAlert{})
		return
	}
	writeJSON(w, http.StatusOK, s.d.Alerts.Active())
}
