package metrics

import (
	"context"
	"log/slog"
	"time"
)

// PodMetricsReader 读取本集群 Pod 指标（M2 由 metrics-server 实现；M1 传 nil）。
type PodMetricsReader interface {
	Read(ctx context.Context) ([]Sample, error)
}

// Collector 周期采集节点 + Pod 指标并写入存储，并定期清理过期样本。
type Collector struct {
	store    MetricStore
	node     NodeCollector
	pod      PodMetricsReader // 可为 nil
	nodeName string
	log      *slog.Logger
}

func NewCollector(store MetricStore, node NodeCollector, pod PodMetricsReader, nodeName string) *Collector {
	return &Collector{store: store, node: node, pod: pod, nodeName: nodeName, log: slog.Default()}
}

// tick 采集一拍。各数据源独立容错：某源失败只记日志，已成功的样本照常落库（评审 N2）。
func (c *Collector) tick(ctx context.Context) error {
	var samples []Sample

	if s, err := c.node.Collect(ctx, c.nodeName); err != nil {
		c.log.Warn("采集节点指标失败", "node", c.nodeName, "err", err)
	} else {
		samples = append(samples, s)
	}

	if c.pod != nil {
		if ps, err := c.pod.Read(ctx); err != nil {
			// metrics-server 启动初期可能未就绪，仅告警不影响节点样本（评审 N2）。
			c.log.Warn("采集 Pod 指标失败", "err", err)
		} else {
			samples = append(samples, ps...)
		}
	}

	if err := c.store.Write(ctx, samples); err != nil {
		c.log.Error("写入样本失败", "err", err)
		return err
	}
	return nil
}

// Run 阻塞运行采集循环，直到 ctx 取消。interval 采集间隔，retention 保留时长。
func (c *Collector) Run(ctx context.Context, interval, retention time.Duration) {
	_ = c.tick(ctx) // 立即采一拍（首拍 CPU 为 0，属预期）
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	// 清理频率取保留期的 1/10，至少每分钟一次。
	pruneEvery := retention / 10
	if pruneEvery < time.Minute {
		pruneEvery = time.Minute
	}
	pruneTicker := time.NewTicker(pruneEvery)
	defer pruneTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.log.Info("采集循环退出")
			return
		case <-ticker.C:
			_ = c.tick(ctx)
		case <-pruneTicker.C:
			if n, err := c.store.Prune(ctx, time.Now().Add(-retention)); err != nil {
				c.log.Warn("清理过期样本失败", "err", err)
			} else if n > 0 {
				c.log.Info("清理过期样本", "rows", n)
			}
		}
	}
}
