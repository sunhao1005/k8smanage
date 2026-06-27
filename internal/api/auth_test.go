package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/sunhao/k8smanage/internal/auth"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
}

func TestAuthDisabledAllowsAll(t *testing.T) {
	h := authMiddleware(nil)(okHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/overview", nil))
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthRejectsMissingToken(t *testing.T) {
	a := auth.New("admin", "pw", "", "k", time.Hour)
	h := authMiddleware(a)(okHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/overview", nil))
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthAcceptsSessionBearer(t *testing.T) {
	a := auth.New("admin", "pw", "", "k", time.Hour)
	tok, _ := a.Login("admin", "pw")
	h := authMiddleware(a)(okHandler())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/overview", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthAcceptsQueryToken(t *testing.T) {
	a := auth.New("", "", "apikey", "k", time.Hour)
	h := authMiddleware(a)(okHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/logs?token=apikey", nil))
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthRejectsWrongToken(t *testing.T) {
	a := auth.New("admin", "pw", "", "k", time.Hour)
	h := authMiddleware(a)(okHandler())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/overview", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
