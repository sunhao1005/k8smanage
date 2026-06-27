package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGopsutilCollect(t *testing.T) {
	// 用临时目录作为“宿主盘”，跨平台都能 statfs（评审 #3：disk 走配置路径）。
	c := NewGopsutilCollector(t.TempDir())
	ctx := context.Background()

	s, err := c.Collect(ctx, "n1")
	require.NoError(t, err)
	require.Equal(t, TargetNode, s.Kind)
	require.Equal(t, "n1", s.Target)
	require.Greater(t, s.MemTot, uint64(0))
	require.GreaterOrEqual(t, s.MemUse, uint64(0))
	require.Greater(t, s.DiskTot, uint64(0)) // 配置路径可量到盘容量
	require.GreaterOrEqual(t, s.CPU, 0.0)
	require.False(t, s.TS.IsZero())
}

func TestGopsutilCPUDelta(t *testing.T) {
	c := NewGopsutilCollector(t.TempDir())
	ctx := context.Background()

	first, err := c.Collect(ctx, "n1")
	require.NoError(t, err)
	require.Equal(t, 0.0, first.CPU) // 首拍无前值

	time.Sleep(50 * time.Millisecond)
	second, err := c.Collect(ctx, "n1")
	require.NoError(t, err)
	require.GreaterOrEqual(t, second.CPU, 0.0) // 第二拍起为差值，非负
}
