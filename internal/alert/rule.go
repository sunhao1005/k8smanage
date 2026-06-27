// Package alert 提供阈值告警：规则存储、评估状态机、webhook 通知。
package alert

import "context"

// Comparator 阈值方向。
type Comparator string

const (
	GT Comparator = ">"
	LT Comparator = "<"
)

// AlertState 规则在某 target 上的当前状态。
type AlertState string

const (
	StateOK      AlertState = "ok"
	StatePending AlertState = "pending" // 已越阈但未到持续时长
	StateFiring  AlertState = "firing"
)

// AlertRule 一条阈值规则。
// Metric 语义：cpu=已用核数，load1=负载，mem/disk=使用率(0..1)。
type AlertRule struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Kind      string     `json:"kind"`   // "node" | "pod"
	Target    string     `json:"target"` // 空=匹配该 Kind 的全部 target
	Metric    string     `json:"metric"` // cpu | mem | disk | load1
	Cmp       Comparator `json:"cmp"`
	Threshold float64    `json:"threshold"`
	ForSec    int        `json:"forSec"` // 持续多少秒才 firing
	Enabled   bool       `json:"enabled"`
}

// RuleStore 持久化告警规则。
type RuleStore interface {
	List(ctx context.Context) ([]AlertRule, error)
	Upsert(ctx context.Context, r AlertRule) error
	Delete(ctx context.Context, id string) error
	Close() error
}
