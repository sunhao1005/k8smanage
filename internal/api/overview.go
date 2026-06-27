package api

import (
	"net/http"

	"github.com/sunhao/k8smanage/internal/kube"
	"github.com/sunhao/k8smanage/internal/metrics"
)

// NodeOverview 是节点信息 + 最新水位的合成视图。
type NodeOverview struct {
	kube.NodeInfo
	CPU     float64 `json:"cpu"` // 已用核数（最新样本）
	MemUse  uint64  `json:"memUse"`
	MemTot  uint64  `json:"memTot"`
	DiskUse uint64  `json:"diskUse"`
	DiskTot uint64  `json:"diskTot"`
	Load1   float64 `json:"load1"`
	HasData bool    `json:"hasData"` // 是否已采到样本
}

// Overview 是总览响应。
type Overview struct {
	Nodes     []NodeOverview `json:"nodes"`
	Workloads struct {
		Total int `json:"total"`
		Ready int `json:"ready"` // Desired>0 且 Ready==Desired 的工作负载数
	} `json:"workloads"`
}

func (s *server) handleOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nodes, err := s.d.Lister.Nodes(ctx)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "读取节点失败: "+err.Error())
		return
	}

	var resp Overview
	resp.Nodes = make([]NodeOverview, 0, len(nodes))
	for _, n := range nodes {
		no := NodeOverview{NodeInfo: n}
		if sm, ok, err := s.d.Store.Latest(ctx, metrics.TargetNode, n.Name); err == nil && ok {
			no.CPU = sm.CPU
			no.MemUse, no.MemTot = sm.MemUse, sm.MemTot
			no.DiskUse, no.DiskTot = sm.DiskUse, sm.DiskTot
			no.Load1 = sm.Load1
			no.HasData = true
		}
		resp.Nodes = append(resp.Nodes, no)
	}

	wls, err := s.d.Lister.Workloads(ctx, "")
	if err != nil {
		writeErr(w, http.StatusBadGateway, "读取工作负载失败: "+err.Error())
		return
	}
	resp.Workloads.Total = len(wls)
	for _, wl := range wls {
		if wl.Desired > 0 && wl.Ready == wl.Desired {
			resp.Workloads.Ready++
		}
	}

	writeJSON(w, http.StatusOK, resp)
}
