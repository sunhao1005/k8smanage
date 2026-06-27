package alert

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRuleStoreCRUD(t *testing.T) {
	ctx := context.Background()
	s, err := NewSQLiteRuleStore(filepath.Join(t.TempDir(), "rules.db"))
	require.NoError(t, err)
	defer s.Close()

	r := AlertRule{ID: "r1", Name: "节点内存高", Kind: "node", Metric: "mem", Cmp: GT, Threshold: 0.85, ForSec: 60, Enabled: true}
	require.NoError(t, s.Upsert(ctx, r))

	list, err := s.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "节点内存高", list[0].Name)
	require.Equal(t, GT, list[0].Cmp)
	require.True(t, list[0].Enabled)
	require.Equal(t, 0.85, list[0].Threshold)

	// 更新（同 ID）
	r.Threshold = 0.9
	r.Enabled = false
	require.NoError(t, s.Upsert(ctx, r))
	list, _ = s.List(ctx)
	require.Len(t, list, 1)
	require.Equal(t, 0.9, list[0].Threshold)
	require.False(t, list[0].Enabled)

	require.NoError(t, s.Delete(ctx, "r1"))
	list, _ = s.List(ctx)
	require.Empty(t, list)
}
