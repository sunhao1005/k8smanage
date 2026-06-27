package api

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// spaFileServer 服务嵌入的前端；找不到的路径回退到 index.html（SPA 前端路由）。
// base 非空时把它注入 index.html（<base href> + window.__BASE_PATH__），
// 让资源与 API/WS 都走子路径前缀，支持域名子路径反代。
func spaFileServer(fsys fs.FS, base string) http.HandlerFunc {
	fileServer := http.FileServer(http.FS(fsys))
	index := buildIndex(fsys, base)
	serveIndex := func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "" || p == "index.html" {
			serveIndex(w)
			return
		}
		if f, err := fsys.Open(p); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		serveIndex(w) // SPA 回退
	}
}

// buildIndex 读取 index.html 并按 base 注入 <base href> 与运行时变量（启动时算一次）。
func buildIndex(fsys fs.FS, base string) []byte {
	b, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		return []byte("<!doctype html><title>k8smanage</title><div id=\"root\"></div>")
	}
	href, bp := "/", ""
	if base != "" {
		href, bp = base+"/", base
	}
	inject := fmt.Sprintf(`<base href="%s"><script>window.__BASE_PATH__=%q;</script>`, href, bp)
	return []byte(strings.Replace(string(b), "<head>", "<head>"+inject, 1))
}
