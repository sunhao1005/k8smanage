// Package config 从环境变量加载运行配置。凭据（webhook、token）只从 env 读，不入库。
package config

import (
	"os"
	"strconv"
)

type Config struct {
	Addr            string // HTTP 监听地址
	IntervalSec     int    // 采集间隔（秒）
	RetentionSec    int    // 样本保留时长（秒），超期由 Prune 清理
	DBPath          string // SQLite 文件路径；须落在可写卷上（评审 N3：rootfs 只读）
	HostRoot        string // 宿主根分区在容器内的挂载点，用于 disk 采集（评审 #3）
	NodeName        string // 由 downward API NODE_NAME 注入
	BasePath        string // 子路径前缀（如 /k8smanage）；空=根路径，供域名子路径反代
	AlertWebhook    string // 告警通知 webhook 地址
	AuthUser        string // 登录用户名（与 AuthPass 同时设置才启用账密登录）
	AuthPass        string // 登录密码（置于 k8s Secret）
	AuthToken       string // 静态 API Token（供脚本调用；与账密登录可并存）
	SessionSecret   string // 会话令牌签名密钥；空则启动随机生成（重启后旧会话失效）
	SessionTTLHours int    // 会话有效期（小时），默认 24
	DisableK8s      bool   // K8SM_DISABLE_K8S=1 时不连集群（纯本机指标模式）
}

// K8sEnabled 是否尝试连接 k8s 集群。
func (c Config) K8sEnabled() bool { return !c.DisableK8s }

func Load() Config {
	return Config{
		Addr:            env("K8SM_ADDR", ":8080"),
		IntervalSec:     envInt("K8SM_INTERVAL_SEC", 15),
		RetentionSec:    envInt("K8SM_RETENTION_SEC", 7*24*3600),
		DBPath:          env("K8SM_DB_PATH", "/data/k8smanage.db"),
		HostRoot:        env("K8SM_HOST_ROOT", "/"),
		NodeName:        env("NODE_NAME", ""),
		BasePath:        env("K8SM_BASE_PATH", ""),
		AlertWebhook:    env("K8SM_ALERT_WEBHOOK", ""),
		AuthUser:        env("K8SM_AUTH_USER", ""),
		AuthPass:        env("K8SM_AUTH_PASS", ""),
		AuthToken:       env("K8SM_AUTH_TOKEN", ""),
		SessionSecret:   env("K8SM_SESSION_SECRET", ""),
		SessionTTLHours: envInt("K8SM_SESSION_TTL_HOURS", 24),
		DisableK8s:      os.Getenv("K8SM_DISABLE_K8S") == "1",
	}
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func envInt(k string, d int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return d
}
