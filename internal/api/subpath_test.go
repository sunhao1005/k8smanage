package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func TestNormalizeBasePath(t *testing.T) {
	require.Equal(t, "", NormalizeBasePath(""))
	require.Equal(t, "", NormalizeBasePath("/"))
	require.Equal(t, "/k8smanage", NormalizeBasePath("k8smanage"))
	require.Equal(t, "/k8smanage", NormalizeBasePath("/k8smanage/"))
	require.Equal(t, "/a/b", NormalizeBasePath(" /a/b/ "))
}

func staticFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":           &fstest.MapFile{Data: []byte(`<!doctype html><html><head><meta charset="utf-8"></head><body><div id="root"></div></body></html>`)},
		"assets/app-abc123.js": &fstest.MapFile{Data: []byte(`console.log("k8sm-bundle")`)},
	}
}

func TestRouterSubpath(t *testing.T) {
	r := NewRouter(Deps{Static: staticFS(), BasePath: "/k8smanage"})

	// 健康检查固定在根（探针不受前缀影响）
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "ok", rec.Body.String())

	// API 在前缀下可达
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/k8smanage/api/config", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	// 不带前缀的 API 不存在
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	require.Equal(t, http.StatusNotFound, rec.Code)

	// 前缀下的静态资源必须返回真实文件内容（而非回退到 index.html）
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/k8smanage/assets/app-abc123.js", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "k8sm-bundle")
	require.NotContains(t, rec.Body.String(), "<div id=\"root\"") // 不能是 index.html

	// 前缀首页注入 <base href> 与 __BASE_PATH__
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/k8smanage/", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `<base href="/k8smanage/">`)
	require.Contains(t, rec.Body.String(), `window.__BASE_PATH__="/k8smanage"`)

	// 无尾斜杠前缀 → 跳转带斜杠
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/k8smanage", nil))
	require.Equal(t, http.StatusMovedPermanently, rec.Code)
	require.Equal(t, "/k8smanage/", rec.Header().Get("Location"))
}

func TestRouterRootBaseInjectsRootHref(t *testing.T) {
	r := NewRouter(Deps{Static: staticFS()}) // base 空 = 根
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `<base href="/">`)
	require.Contains(t, rec.Body.String(), `window.__BASE_PATH__=""`)
}
