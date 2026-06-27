package api

import "net/http"

func (s *server) handleNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := s.d.Lister.Nodes(r.Context())
	if err != nil {
		writeErr(w, http.StatusBadGateway, "读取节点失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, nodes)
}
