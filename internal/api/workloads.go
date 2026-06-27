package api

import "net/http"

func (s *server) handleWorkloads(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("ns") // 空=全部命名空间
	wls, err := s.d.Lister.Workloads(r.Context(), ns)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "读取工作负载失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, wls)
}

func (s *server) handlePods(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("ns") // 空=全部命名空间
	pods, err := s.d.Lister.Pods(r.Context(), ns)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "读取 Pod 失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pods)
}
