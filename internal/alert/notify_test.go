package alert

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWebhookNotifier(t *testing.T) {
	var got Alert
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := NewWebhookNotifier(srv.URL)
	a := Alert{RuleName: "cpu高", Kind: "node", Target: "n1", Metric: "cpu", Value: 2.0, Threshold: 1.0, Cmp: GT, State: StateFiring}
	a.Text = summary(a)
	require.NoError(t, n.Notify(context.Background(), a))

	require.Equal(t, "cpu高", got.RuleName)
	require.Equal(t, StateFiring, got.State)
	require.Contains(t, got.Text, "告警")
}

func TestWebhookNotifierBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	n := NewWebhookNotifier(srv.URL)
	require.Error(t, n.Notify(context.Background(), Alert{}))
}
