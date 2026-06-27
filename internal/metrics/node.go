package metrics

import "context"

// NodeCollector 采集“本进程所在宿主机”的指标。
// 多节点时换成 agent 推送实现，API/前端不动（评审约束 3）。
type NodeCollector interface {
	// Collect 返回该节点当前 Sample（Kind=TargetNode）。
	Collect(ctx context.Context, nodeName string) (Sample, error)
}
