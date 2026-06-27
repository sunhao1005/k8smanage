package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/sunhao/k8smanage/internal/metrics"
)

// handleMetricsQuery 查询某 target 某 metric 的时序点。
// 参数：kind=node|pod, target, metric, from, to（unix 秒，可省，默认最近 1 小时）。
func (s *server) handleMetricsQuery(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var kind metrics.TargetKind
	switch q.Get("kind") {
	case "node":
		kind = metrics.TargetNode
	case "pod":
		kind = metrics.TargetPod
	default:
		writeErr(w, http.StatusBadRequest, "kind 必须为 node 或 pod")
		return
	}

	target := q.Get("target")
	if target == "" {
		writeErr(w, http.StatusBadRequest, "缺少 target")
		return
	}
	metric := q.Get("metric")
	if metric == "" {
		writeErr(w, http.StatusBadRequest, "缺少 metric")
		return
	}

	now := time.Now()
	to := parseUnix(q.Get("to"), now)
	from := parseUnix(q.Get("from"), to.Add(-time.Hour))
	if !from.Before(to) {
		writeErr(w, http.StatusBadRequest, "from 必须早于 to")
		return
	}

	// 服务端降采样：把返回点数控制在 maxPoints 以内，避免长区间海量点。
	// 显式 step 参数优先；否则按区间自动计算桶大小。
	const maxPoints = 500
	stepSec := 0
	if s := q.Get("step"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			stepSec = n
		}
	} else if rangeSec := int(to.Sub(from).Seconds()); rangeSec > maxPoints {
		stepSec = rangeSec / maxPoints
	}

	pts, err := s.d.Store.Query(r.Context(), kind, target, metric, from, to, stepSec)
	if err != nil {
		// 未知 metric 等输入错误归 400
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pts)
}

func parseUnix(s string, def time.Time) time.Time {
	if s == "" {
		return def
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(n, 0)
	}
	return def
}
