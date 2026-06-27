package alert

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/sunhao/k8smanage/internal/metrics"
)

type fakeMetrics struct {
	mu      sync.Mutex
	sample  metrics.Sample
	ok      bool
	targets []string
}

func (f *fakeMetrics) set(s metrics.Sample) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sample, f.ok = s, true
}
func (f *fakeMetrics) Latest(context.Context, metrics.TargetKind, string) (metrics.Sample, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sample, f.ok, nil
}
func (f *fakeMetrics) Targets(context.Context, metrics.TargetKind, time.Time) ([]string, error) {
	return f.targets, nil
}

type capNotifier struct{ events []Alert }

func (c *capNotifier) Notify(_ context.Context, a Alert) error {
	c.events = append(c.events, a)
	return nil
}

func newEngineWithRule(t *testing.T, r AlertRule) (*Engine, *fakeMetrics, *capNotifier) {
	t.Helper()
	rs, err := NewSQLiteRuleStore(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { rs.Close() })
	require.NoError(t, rs.Upsert(context.Background(), r))
	fm := &fakeMetrics{}
	cn := &capNotifier{}
	return NewEngine(rs, fm, cn), fm, cn
}

func TestEngineImmediateFireAndResolve(t *testing.T) {
	ctx := context.Background()
	e, fm, cn := newEngineWithRule(t, AlertRule{
		ID: "r1", Name: "cpu高", Kind: "node", Target: "n1", Metric: "cpu", Cmp: GT, Threshold: 1.0, ForSec: 0, Enabled: true,
	})
	t0 := time.Unix(1_700_000_000, 0)

	fm.set(metrics.Sample{Kind: metrics.TargetNode, Target: "n1", CPU: 2.0})
	e.Evaluate(ctx, t0)
	require.Len(t, cn.events, 1)
	require.Equal(t, StateFiring, cn.events[0].State)

	// 维持 firing 不重复通知
	e.Evaluate(ctx, t0.Add(time.Second))
	require.Len(t, cn.events, 1)

	// 回落 → 恢复通知
	fm.set(metrics.Sample{Kind: metrics.TargetNode, Target: "n1", CPU: 0.3})
	e.Evaluate(ctx, t0.Add(2*time.Second))
	require.Len(t, cn.events, 2)
	require.Equal(t, StateOK, cn.events[1].State)
}

func TestEngineForSecPending(t *testing.T) {
	ctx := context.Background()
	e, fm, cn := newEngineWithRule(t, AlertRule{
		ID: "r1", Name: "内存持续高", Kind: "node", Target: "n1", Metric: "mem", Cmp: GT, Threshold: 0.8, ForSec: 60, Enabled: true,
	})
	t0 := time.Unix(1_700_000_000, 0)
	fm.set(metrics.Sample{Kind: metrics.TargetNode, Target: "n1", MemUse: 90, MemTot: 100}) // 0.9 > 0.8

	e.Evaluate(ctx, t0)
	require.Empty(t, cn.events) // pending，未到 60s
	active := e.Active()
	require.Len(t, active, 1)
	require.Equal(t, StatePending, active[0].State)
	require.Equal(t, "内存持续高", active[0].RuleName) // 元数据已带上
	require.Equal(t, "mem", active[0].Metric)
	require.Equal(t, "n1", active[0].Target)

	e.Evaluate(ctx, t0.Add(30*time.Second))
	require.Empty(t, cn.events) // 仍 pending

	e.Evaluate(ctx, t0.Add(61*time.Second))
	require.Len(t, cn.events, 1) // 持续超 60s → firing
	require.Equal(t, StateFiring, cn.events[0].State)
}

func TestEngineMatchAllTargets(t *testing.T) {
	ctx := context.Background()
	e, fm, cn := newEngineWithRule(t, AlertRule{
		ID: "r1", Name: "任一节点cpu高", Kind: "node", Target: "", Metric: "cpu", Cmp: GT, Threshold: 1.0, ForSec: 0, Enabled: true,
	})
	fm.targets = []string{"n1", "n2"}
	fm.set(metrics.Sample{CPU: 2.0})

	e.Evaluate(ctx, time.Unix(1_700_000_000, 0))
	require.Len(t, cn.events, 2) // n1 和 n2 各触发一次
}

func TestEngineDisabledRuleSkipped(t *testing.T) {
	ctx := context.Background()
	e, fm, cn := newEngineWithRule(t, AlertRule{
		ID: "r1", Name: "x", Kind: "node", Target: "n1", Metric: "cpu", Cmp: GT, Threshold: 1.0, Enabled: false,
	})
	fm.set(metrics.Sample{CPU: 5.0})
	e.Evaluate(ctx, time.Unix(1, 0))
	require.Empty(t, cn.events)
}
