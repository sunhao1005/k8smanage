package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

// gopsutilCollector 读宿主机指标。CPU 用自维护的 times 差值算已用核数，
// 不依赖 gopsutil 包级全局状态（评审 #7：首拍无前值则返回 0）。
type gopsutilCollector struct {
	hostRoot string // 宿主根分区挂载点（容器内），disk 采集用

	mu        sync.Mutex
	prevTotal float64
	prevBusy  float64
	hasPrev   bool
}

// NewGopsutilCollector 构造采集器；hostRoot 为宿主盘在容器内的挂载点（如 "/host"），
// 本地运行传 "/"（Windows 传具体盘符路径）。
func NewGopsutilCollector(hostRoot string) NodeCollector {
	if hostRoot == "" {
		hostRoot = "/"
	}
	return &gopsutilCollector{hostRoot: hostRoot}
}

func (c *gopsutilCollector) Collect(ctx context.Context, nodeName string) (Sample, error) {
	s := Sample{Kind: TargetNode, Target: nodeName, TS: time.Now()}

	// 内存为必需项：失败则整次采集失败。
	vm, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return Sample{}, err
	}
	s.MemUse = vm.Used
	s.MemTot = vm.Total

	// CPU：用聚合 times 差值算已用核数；首拍无前值返回 0。
	s.CPU = c.cpuCores(ctx)

	// 以下为可选项：单项失败只置零，不拖垮整次采集（评审 N2）。
	if du, err := disk.UsageWithContext(ctx, c.hostRoot); err == nil {
		s.DiskUse = du.Used
		s.DiskTot = du.Total
	}
	if ios, err := net.IOCountersWithContext(ctx, false); err == nil && len(ios) > 0 {
		s.NetRx = ios[0].BytesRecv
		s.NetTx = ios[0].BytesSent
	}
	if avg, err := load.AvgWithContext(ctx); err == nil {
		s.Load1 = avg.Load1
	}
	return s, nil
}

// cpuCores 返回自上次调用以来的平均已用核数。
func (c *gopsutilCollector) cpuCores(ctx context.Context) float64 {
	times, err := cpu.TimesWithContext(ctx, false)
	if err != nil || len(times) == 0 {
		return 0
	}
	t := times[0]
	total := t.User + t.System + t.Idle + t.Nice + t.Iowait +
		t.Irq + t.Softirq + t.Steal + t.Guest + t.GuestNice
	busy := total - t.Idle - t.Iowait

	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.hasPrev {
		c.prevTotal, c.prevBusy, c.hasPrev = total, busy, true
		return 0 // 首拍无差值
	}
	dt := total - c.prevTotal
	db := busy - c.prevBusy
	c.prevTotal, c.prevBusy = total, busy
	if dt <= 0 {
		return 0
	}
	frac := db / dt
	if frac < 0 {
		frac = 0
	}
	// times[0] 是所有核累加，frac 即“整机平均忙比”；乘逻辑核数得已用核数。
	return frac * float64(numCPU(ctx))
}

func numCPU(ctx context.Context) int {
	n, err := cpu.CountsWithContext(ctx, true)
	if err != nil || n <= 0 {
		return 1
	}
	return n
}
