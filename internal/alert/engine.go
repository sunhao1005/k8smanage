package alert

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/sunhao/k8smanage/internal/metrics"
)

// Metrics 是引擎依赖的指标读取子集（*metrics.MetricStore 的子集）。
type Metrics interface {
	Latest(ctx context.Context, kind metrics.TargetKind, target string) (metrics.Sample, bool, error)
	Targets(ctx context.Context, kind metrics.TargetKind, since time.Time) ([]string, error)
}

// ActiveAlert 是当前处于 pending/firing 的告警（供 API 展示）。
type ActiveAlert struct {
	RuleID    string     `json:"ruleId"`
	RuleName  string     `json:"ruleName"`
	Kind      string     `json:"kind"`
	Target    string     `json:"target"`
	Metric    string     `json:"metric"`
	State     AlertState `json:"state"`
	Value     float64    `json:"value"`
	Threshold float64    `json:"threshold"`
	Since     time.Time  `json:"since"`
}

type ruleState struct {
	state AlertState
	since time.Time // 进入 pending 的时刻
	value float64   // 最近一次评估值
	// 规则元数据快照，供 Active() 展示（避免再查规则）。
	ruleName  string
	kind      string
	metric    string
	threshold float64
}

// Engine 周期评估规则并在状态翻转时通知。
type Engine struct {
	rules    RuleStore
	m        Metrics
	notifier Notifier
	log      *slog.Logger

	mu     sync.Mutex
	states map[string]*ruleState // key = ruleID + "\x00" + target
}

func NewEngine(rules RuleStore, m Metrics, notifier Notifier) *Engine {
	return &Engine{rules: rules, m: m, notifier: notifier, log: slog.Default(), states: map[string]*ruleState{}}
}

// Run 阻塞运行评估循环，每 interval 评估一次，直到 ctx 取消。
func (e *Engine) Run(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			e.Evaluate(ctx, time.Now())
		}
	}
}

// Evaluate 评估一轮（now 显式传入，便于测试）。
func (e *Engine) Evaluate(ctx context.Context, now time.Time) {
	rules, err := e.rules.List(ctx)
	if err != nil {
		e.log.Warn("读取告警规则失败", "err", err)
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	seen := map[string]bool{}
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		for _, target := range e.resolveTargets(ctx, r, now) {
			key := r.ID + "\x00" + target
			seen[key] = true
			e.evalOne(ctx, r, target, key, now)
		}
	}
	// 清理已不存在的规则/目标状态，避免泄漏。
	for k := range e.states {
		if !seen[k] {
			delete(e.states, k)
		}
	}
}

func (e *Engine) resolveTargets(ctx context.Context, r AlertRule, now time.Time) []string {
	if r.Target != "" {
		return []string{r.Target}
	}
	// 匹配全部：取最近 5 分钟有样本的 target。
	ts, err := e.m.Targets(ctx, metrics.TargetKind(r.Kind), now.Add(-5*time.Minute))
	if err != nil {
		e.log.Warn("枚举告警目标失败", "kind", r.Kind, "err", err)
		return nil
	}
	return ts
}

func (e *Engine) evalOne(ctx context.Context, r AlertRule, target, key string, now time.Time) {
	sample, ok, err := e.m.Latest(ctx, metrics.TargetKind(r.Kind), target)
	if err != nil || !ok {
		return
	}
	val, ok := valueFor(r.Metric, sample)
	if !ok {
		return
	}
	st := e.states[key]
	if st == nil {
		st = &ruleState{state: StateOK}
		e.states[key] = st
	}
	st.value = val
	st.ruleName, st.kind, st.metric, st.threshold = r.Name, r.Kind, r.Metric, r.Threshold

	if breached(val, r.Cmp, r.Threshold) {
		switch st.state {
		case StateOK:
			st.state = StatePending
			st.since = now
			if r.ForSec <= 0 { // 无需持续，立即 firing
				st.state = StateFiring
				e.fire(ctx, r, target, val, now)
			}
		case StatePending:
			if now.Sub(st.since) >= time.Duration(r.ForSec)*time.Second {
				st.state = StateFiring
				e.fire(ctx, r, target, val, now)
			}
		case StateFiring:
			// 维持
		}
	} else {
		if st.state == StateFiring {
			e.resolve(ctx, r, target, val, now)
		}
		st.state = StateOK
	}
}

func (e *Engine) fire(ctx context.Context, r AlertRule, target string, val float64, now time.Time) {
	a := Alert{RuleID: r.ID, RuleName: r.Name, Kind: r.Kind, Target: target, Metric: r.Metric,
		Value: val, Threshold: r.Threshold, Cmp: r.Cmp, State: StateFiring, Time: now}
	a.Text = summary(a)
	e.log.Warn("告警触发", "rule", r.Name, "target", target, "value", val)
	if err := e.notifier.Notify(ctx, a); err != nil {
		e.log.Warn("发送告警通知失败", "err", err)
	}
}

func (e *Engine) resolve(ctx context.Context, r AlertRule, target string, val float64, now time.Time) {
	a := Alert{RuleID: r.ID, RuleName: r.Name, Kind: r.Kind, Target: target, Metric: r.Metric,
		Value: val, Threshold: r.Threshold, Cmp: r.Cmp, State: StateOK, Time: now}
	a.Text = summary(a)
	e.log.Info("告警恢复", "rule", r.Name, "target", target, "value", val)
	if err := e.notifier.Notify(ctx, a); err != nil {
		e.log.Warn("发送恢复通知失败", "err", err)
	}
}

// Active 返回当前 pending/firing 的告警快照。
func (e *Engine) Active() []ActiveAlert {
	e.mu.Lock()
	defer e.mu.Unlock()
	// 需要规则元数据；从 states 的 key 反查较弱，这里仅返回状态层信息。
	out := []ActiveAlert{}
	for key, st := range e.states {
		if st.state == StateOK {
			continue
		}
		ruleID, target := splitKey(key)
		out = append(out, ActiveAlert{
			RuleID: ruleID, RuleName: st.ruleName, Kind: st.kind, Target: target,
			Metric: st.metric, State: st.state, Value: st.value, Threshold: st.threshold, Since: st.since,
		})
	}
	return out
}

func splitKey(key string) (ruleID, target string) {
	for i := 0; i < len(key); i++ {
		if key[i] == 0 {
			return key[:i], key[i+1:]
		}
	}
	return key, ""
}

func valueFor(metric string, s metrics.Sample) (float64, bool) {
	switch metric {
	case "cpu":
		return s.CPU, true
	case "load1":
		return s.Load1, true
	case "mem":
		if s.MemTot > 0 {
			return float64(s.MemUse) / float64(s.MemTot), true
		}
		return 0, false
	case "disk":
		if s.DiskTot > 0 {
			return float64(s.DiskUse) / float64(s.DiskTot), true
		}
		return 0, false
	default:
		return 0, false
	}
}

func breached(v float64, cmp Comparator, th float64) bool {
	switch cmp {
	case GT:
		return v > th
	case LT:
		return v < th
	default:
		return false
	}
}
