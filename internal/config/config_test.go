package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadDefaults(t *testing.T) {
	// 清空可能影响默认值的环境变量
	for _, k := range []string{"K8SM_ADDR", "K8SM_INTERVAL_SEC", "K8SM_RETENTION_SEC", "K8SM_HOST_ROOT", "NODE_NAME", "K8SM_ALERT_WEBHOOK", "K8SM_AUTH_TOKEN"} {
		t.Setenv(k, "")
	}
	t.Setenv("K8SM_DB_PATH", "/tmp/x.db")
	c := Load()
	require.Equal(t, ":8080", c.Addr)
	require.Equal(t, 15, c.IntervalSec)
	require.Equal(t, "/tmp/x.db", c.DBPath)
	require.Equal(t, 7*24*3600, c.RetentionSec)
	require.Equal(t, "/", c.HostRoot) // 评审 N3/#3：宿主盘挂载点，默认 "/"
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("K8SM_ADDR", ":9090")
	t.Setenv("K8SM_INTERVAL_SEC", "30")
	t.Setenv("K8SM_HOST_ROOT", "/host")
	t.Setenv("NODE_NAME", "node-a")
	t.Setenv("K8SM_AUTH_TOKEN", "secret")
	c := Load()
	require.Equal(t, ":9090", c.Addr)
	require.Equal(t, 30, c.IntervalSec)
	require.Equal(t, "/host", c.HostRoot)
	require.Equal(t, "node-a", c.NodeName)
	require.Equal(t, "secret", c.AuthToken)
}

func TestLoadInvalidIntFallsBack(t *testing.T) {
	t.Setenv("K8SM_INTERVAL_SEC", "not-a-number")
	c := Load()
	require.Equal(t, 15, c.IntervalSec)
}
