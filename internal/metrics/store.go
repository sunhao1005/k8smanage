// Package metrics 负责采集（节点/Pod）与时序存储。
package metrics

import (
	"context"
	"time"
)

// TargetKind 区分样本归属：节点 or Pod。
type TargetKind string

const (
	TargetNode TargetKind = "node"
	TargetPod  TargetKind = "pod"
)

// Sample 一条采集样本（一个 target 一个时刻的一组指标）。
type Sample struct {
	Kind    TargetKind
	Target  string // 节点名 或 "ns/pod"
	TS      time.Time
	CPU     float64 // 已用核数（cores）
	MemUse  uint64  // 已用内存 bytes
	MemTot  uint64  // 总内存 bytes（Pod 为 limit，0 表示无 limit）
	DiskUse uint64  // 仅节点：根分区已用 bytes
	DiskTot uint64  // 仅节点
	NetRx   uint64  // 仅节点：累计接收 bytes（计数器，速率在查询/采集侧求差，见评审 N1）
	NetTx   uint64  // 仅节点：累计发送 bytes（计数器）
	Load1   float64 // 仅节点
}

// Point 区间查询返回的单点（按 metric 投影后）。
type Point struct {
	TS    time.Time
	Value float64
}

// MetricStore 抽象存储；默认 SQLite 实现，可换 VictoriaMetrics（评审约束 4）。
type MetricStore interface {
	Write(ctx context.Context, s []Sample) error
	// Query 取某 target 某 metric 在 [from,to] 的序列；
	// metric ∈ {cpu, mem, disk, net_rx, net_tx, load1}。
	// stepSec>0 时按 step 秒桶聚合（AVG）做降采样，避免长区间返回海量点；<=0 返回原始点。
	Query(ctx context.Context, kind TargetKind, target, metric string, from, to time.Time, stepSec int) ([]Point, error)
	// Latest 取某 target 最近一条样本（总览用）。
	Latest(ctx context.Context, kind TargetKind, target string) (Sample, bool, error)
	// Targets 列出某 kind 在 since 之后有样本的去重 target（告警"匹配全部"用）。
	Targets(ctx context.Context, kind TargetKind, since time.Time) ([]string, error)
	// Prune 删除早于 cutoff 的样本（环形保留）。
	Prune(ctx context.Context, cutoff time.Time) (int64, error)
	Close() error
}
