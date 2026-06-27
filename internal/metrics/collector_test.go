package metrics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeNode struct{ err error }

func (f fakeNode) Collect(_ context.Context, name string) (Sample, error) {
	if f.err != nil {
		return Sample{}, f.err
	}
	return Sample{Kind: TargetNode, Target: name, TS: time.Unix(1, 0), CPU: 0.3, MemTot: 100}, nil
}

type fakePod struct {
	samples []Sample
	err     error
}

func (f fakePod) Read(_ context.Context) ([]Sample, error) { return f.samples, f.err }

func TestCollectorTick(t *testing.T) {
	ctx := context.Background()
	store, _ := NewSQLiteStore(":memory:")
	defer store.Close()

	c := NewCollector(store, fakeNode{}, nil, "n1")
	require.NoError(t, c.tick(ctx))

	_, ok, err := store.Latest(ctx, TargetNode, "n1")
	require.NoError(t, err)
	require.True(t, ok)
}

func TestCollectorTickWithPods(t *testing.T) {
	ctx := context.Background()
	store, _ := NewSQLiteStore(":memory:")
	defer store.Close()

	pods := fakePod{samples: []Sample{{Kind: TargetPod, Target: "ns/p1", TS: time.Unix(1, 0), CPU: 0.1}}}
	c := NewCollector(store, fakeNode{}, pods, "n1")
	require.NoError(t, c.tick(ctx))

	_, ok, _ := store.Latest(ctx, TargetPod, "ns/p1")
	require.True(t, ok)
}

// 评审 N2：Pod 源失败时，节点样本仍须落库。
func TestCollectorPodFailureKeepsNodeSample(t *testing.T) {
	ctx := context.Background()
	store, _ := NewSQLiteStore(":memory:")
	defer store.Close()

	c := NewCollector(store, fakeNode{}, fakePod{err: errors.New("metrics-server not ready")}, "n1")
	require.NoError(t, c.tick(ctx))

	_, ok, _ := store.Latest(ctx, TargetNode, "n1")
	require.True(t, ok, "Pod 采集失败不应影响节点样本落库")
}
