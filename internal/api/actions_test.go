package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeActions struct {
	lastReplicas int32
	restarted    bool
	paused       bool
	resumed      bool
	deleted      bool
}

func (f *fakeActions) Scale(_ context.Context, _, _, _ string, replicas int32) error {
	f.lastReplicas = replicas
	return nil
}
func (f *fakeActions) Restart(_ context.Context, _, _, _ string) error {
	f.restarted = true
	return nil
}
func (f *fakeActions) Pause(_ context.Context, _, _, _ string) error  { f.paused = true; return nil }
func (f *fakeActions) Resume(_ context.Context, _, _, _ string) error { f.resumed = true; return nil }
func (f *fakeActions) DeletePod(_ context.Context, _, _ string) error { f.deleted = true; return nil }

func TestHandleScale(t *testing.T) {
	fa := &fakeActions{}
	r := NewRouter(Deps{Actions: fa})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/workloads/default/Deployment/web/scale",
		strings.NewReader(`{"replicas":3}`))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int32(3), fa.lastReplicas)
}

func TestHandleScaleMissingBody(t *testing.T) {
	r := NewRouter(Deps{Actions: &fakeActions{}})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/workloads/default/Deployment/web/scale",
		strings.NewReader(`{}`))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleRestartAndDelete(t *testing.T) {
	fa := &fakeActions{}
	r := NewRouter(Deps{Actions: fa})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/workloads/default/Deployment/web/restart", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, fa.restarted)

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/pods/default/web-abc", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, fa.deleted)
}

func TestHandlePauseResume(t *testing.T) {
	fa := &fakeActions{}
	r := NewRouter(Deps{Actions: fa})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/workloads/default/Deployment/web/pause", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, fa.paused)

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/workloads/default/Deployment/web/resume", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, fa.resumed)
}

// 无集群（Actions=nil）时写操作返回 503。
func TestHandleActionsUnavailable(t *testing.T) {
	r := NewRouter(Deps{})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/workloads/default/Deployment/web/restart", nil))
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}
