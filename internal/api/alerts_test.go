package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sunhao/k8smanage/internal/alert"
)

type fakeAlertProvider struct{ active []alert.ActiveAlert }

func (f fakeAlertProvider) Active() []alert.ActiveAlert { return f.active }

func newAlertRouter(t *testing.T) (http.Handler, alert.RuleStore) {
	t.Helper()
	rs, err := alert.NewSQLiteRuleStore(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { rs.Close() })
	r := NewRouter(Deps{
		Rules:  rs,
		Alerts: fakeAlertProvider{active: []alert.ActiveAlert{{RuleID: "r1", Target: "n1", State: alert.StateFiring}}},
	})
	return r, rs
}

func TestUpsertAndListRules(t *testing.T) {
	r, _ := newAlertRouter(t)

	// PUT 无 id → 自动生成
	rec := httptest.NewRecorder()
	body := `{"name":"内存高","kind":"node","metric":"mem","cmp":">","threshold":0.85,"forSec":60,"enabled":true}`
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/alerts/rules", strings.NewReader(body)))
	require.Equal(t, http.StatusOK, rec.Code)
	var created alert.AlertRule
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.NotEmpty(t, created.ID)

	// GET list
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/alerts/rules", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var rules []alert.AlertRule
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rules))
	require.Len(t, rules, 1)

	// DELETE
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/alerts/rules/"+created.ID, nil))
	require.Equal(t, http.StatusOK, rec.Code)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/alerts/rules", nil))
	var after []alert.AlertRule
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &after))
	require.Empty(t, after)
}

func TestUpsertRuleValidation(t *testing.T) {
	r, _ := newAlertRouter(t)
	rec := httptest.NewRecorder()
	body := `{"name":"x","kind":"bogus","metric":"cpu","cmp":">","threshold":1}`
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/alerts/rules", strings.NewReader(body)))
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestActiveAlerts(t *testing.T) {
	r, _ := newAlertRouter(t)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/alerts/active", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var active []alert.ActiveAlert
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &active))
	require.Len(t, active, 1)
	require.Equal(t, alert.StateFiring, active[0].State)
}
