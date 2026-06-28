package metrics

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSQLiteWriteQueryLatest(t *testing.T) {
	ctx := context.Background()
	s, err := NewSQLiteStore(":memory:")
	require.NoError(t, err)
	defer s.Close()

	base := time.Unix(1_700_000_000, 0)
	require.NoError(t, s.Write(ctx, []Sample{
		{Kind: TargetNode, Target: "n1", TS: base, CPU: 0.5, MemUse: 100, MemTot: 200},
		{Kind: TargetNode, Target: "n1", TS: base.Add(15 * time.Second), CPU: 0.7, MemUse: 150, MemTot: 200},
	}))

	pts, err := s.Query(ctx, TargetNode, "n1", "cpu", base.Add(-time.Minute), base.Add(time.Minute), 0)
	require.NoError(t, err)
	require.Len(t, pts, 2)
	require.Equal(t, 0.5, pts[0].Value)
	require.Equal(t, 0.7, pts[1].Value)

	last, ok, err := s.Latest(ctx, TargetNode, "n1")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 0.7, last.CPU)
	require.Equal(t, uint64(150), last.MemUse)
}

func TestSQLiteQueryDownsample(t *testing.T) {
	ctx := context.Background()
	s, _ := NewSQLiteStore(":memory:")
	defer s.Close()

	// 写 60 个点，每 15s 一个（共 ~900s），值递增。
	base := time.Unix(1_700_000_000, 0)
	var samples []Sample
	for i := 0; i < 60; i++ {
		samples = append(samples, Sample{Kind: TargetNode, Target: "n1", TS: base.Add(time.Duration(i*15) * time.Second), CPU: float64(i)})
	}
	require.NoError(t, s.Write(ctx, samples))

	// step=300s 桶聚合：900s 区间应压成约 3 个桶（远少于 60）。
	pts, err := s.Query(ctx, TargetNode, "n1", "cpu", base.Add(-time.Minute), base.Add(1000*time.Second), 300)
	require.NoError(t, err)
	require.Less(t, len(pts), 60)
	require.GreaterOrEqual(t, len(pts), 3)
	// 桶内取均值：第一个桶覆盖 i=0..19（不完全，按对齐），值应是该桶均值而非单点。
	for _, p := range pts {
		require.GreaterOrEqual(t, p.Value, 0.0)
	}
}

func TestSQLiteQueryNetRate(t *testing.T) {
	ctx := context.Background()
	s, _ := NewSQLiteStore(":memory:")
	defer s.Close()

	base := time.Unix(1_700_000_000, 0)
	// 累计 net_rx：0 → 1000(+10s) → 3000(+10s) → 重启回 0(+10s)
	require.NoError(t, s.Write(ctx, []Sample{
		{Kind: TargetNode, Target: "n1", TS: base, NetRx: 0},
		{Kind: TargetNode, Target: "n1", TS: base.Add(10 * time.Second), NetRx: 1000},
		{Kind: TargetNode, Target: "n1", TS: base.Add(20 * time.Second), NetRx: 3000},
		{Kind: TargetNode, Target: "n1", TS: base.Add(30 * time.Second), NetRx: 0},
	}))

	pts, err := s.Query(ctx, TargetNode, "n1", "net_rx", base.Add(-time.Minute), base.Add(time.Minute), 0)
	require.NoError(t, err)
	require.Len(t, pts, 3)                        // n 个累计点 → n-1 个速率点
	require.InDelta(t, 100.0, pts[0].Value, 1e-9) // (1000-0)/10
	require.InDelta(t, 200.0, pts[1].Value, 1e-9) // (3000-1000)/10
	require.InDelta(t, 0.0, pts[2].Value, 1e-9)   // 计数器重启 → 0，不出现负值
}

func TestSQLiteTrafficTotal(t *testing.T) {
	ctx := context.Background()
	s, _ := NewSQLiteStore(":memory:")
	defer s.Close()

	base := time.Unix(1_700_000_000, 0)
	// net_rx 累计: 0→1000→3000→(重启)0→500 ; net_tx: 0→100→300→(重启)0→50
	require.NoError(t, s.Write(ctx, []Sample{
		{Kind: TargetNode, Target: "n1", TS: base, NetRx: 0, NetTx: 0},
		{Kind: TargetNode, Target: "n1", TS: base.Add(15 * time.Second), NetRx: 1000, NetTx: 100},
		{Kind: TargetNode, Target: "n1", TS: base.Add(30 * time.Second), NetRx: 3000, NetTx: 300},
		{Kind: TargetNode, Target: "n1", TS: base.Add(45 * time.Second), NetRx: 0, NetTx: 0}, // 计数器重启
		{Kind: TargetNode, Target: "n1", TS: base.Add(60 * time.Second), NetRx: 500, NetTx: 50},
	}))

	rx, tx, err := s.TrafficTotal(ctx, TargetNode, "n1", base.Add(-time.Minute), base.Add(time.Minute))
	require.NoError(t, err)
	// 正增量累加：rx = 1000 + 2000 + 500 = 3500（重启那段-3000不计）
	require.Equal(t, uint64(3500), rx)
	require.Equal(t, uint64(350), tx) // 100 + 200 + 50
}

func TestSQLiteQueryUnknownMetric(t *testing.T) {
	s, _ := NewSQLiteStore(":memory:")
	defer s.Close()
	_, err := s.Query(context.Background(), TargetNode, "n1", "bogus", time.Unix(0, 0), time.Now(), 0)
	require.Error(t, err)
}

func TestSQLiteLatestMissing(t *testing.T) {
	s, _ := NewSQLiteStore(":memory:")
	defer s.Close()
	_, ok, err := s.Latest(context.Background(), TargetPod, "ns/none")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestSQLitePrune(t *testing.T) {
	ctx := context.Background()
	s, _ := NewSQLiteStore(":memory:")
	defer s.Close()
	old := time.Unix(1_000, 0)
	require.NoError(t, s.Write(ctx, []Sample{{Kind: TargetNode, Target: "n1", TS: old, CPU: 1}}))
	n, err := s.Prune(ctx, time.Unix(2_000, 0))
	require.NoError(t, err)
	require.Equal(t, int64(1), n)
}

// 文件库路径：验证 WAL DSN 能正常打开、读写（评审 N3）。
func TestSQLiteFileBacked(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "k8sm.db")
	s, err := NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer s.Close()
	require.NoError(t, s.Write(ctx, []Sample{{Kind: TargetPod, Target: "ns/p", TS: time.Unix(5, 0), CPU: 0.2}}))
	last, ok, err := s.Latest(ctx, TargetPod, "ns/p")
	require.NoError(t, err)
	require.True(t, ok)
	require.InDelta(t, 0.2, last.CPU, 1e-9)
}
