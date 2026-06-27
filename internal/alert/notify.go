package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Alert 是一次通知事件（触发或恢复）。
type Alert struct {
	RuleID    string     `json:"ruleId"`
	RuleName  string     `json:"ruleName"`
	Kind      string     `json:"kind"`
	Target    string     `json:"target"`
	Metric    string     `json:"metric"`
	Value     float64    `json:"value"`
	Threshold float64    `json:"threshold"`
	Cmp       Comparator `json:"cmp"`
	State     AlertState `json:"state"` // firing | ok(=恢复)
	Time      time.Time  `json:"time"`
	Text      string     `json:"text"` // 人读摘要
}

// Notifier 发送告警通知。
type Notifier interface {
	Notify(ctx context.Context, a Alert) error
}

// NopNotifier 不发送（未配置 webhook 时用）。
type NopNotifier struct{}

func (NopNotifier) Notify(context.Context, Alert) error { return nil }

// WebhookNotifier 把告警以 JSON POST 到 webhook（通用格式，含人读 text 字段）。
type WebhookNotifier struct {
	URL    string
	Client *http.Client
}

func NewWebhookNotifier(url string) *WebhookNotifier {
	return &WebhookNotifier{URL: url, Client: &http.Client{Timeout: 10 * time.Second}}
}

func (n *WebhookNotifier) Notify(ctx context.Context, a Alert) error {
	body, err := json.Marshal(a)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook 返回 %d", resp.StatusCode)
	}
	return nil
}

// summary 生成人读摘要。
func summary(a Alert) string {
	if a.State == StateOK {
		return fmt.Sprintf("[恢复] %s：%s/%s 的 %s=%.3f 已回落", a.RuleName, a.Kind, a.Target, a.Metric, a.Value)
	}
	return fmt.Sprintf("[告警] %s：%s/%s 的 %s=%.3f %s %.3f", a.RuleName, a.Kind, a.Target, a.Metric, a.Value, a.Cmp, a.Threshold)
}
