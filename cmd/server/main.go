// Command server 是 k8smanage 的进程入口。
// M2：加载配置 → 打开存储 → 连 k8s（informer 缓存 + metrics-server）
// → 启动采集循环 + HTTP API（embed 前端）。无集群时降级为仅本机指标。
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/client-go/kubernetes"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/sunhao/k8smanage/internal/alert"
	"github.com/sunhao/k8smanage/internal/api"
	"github.com/sunhao/k8smanage/internal/auth"
	"github.com/sunhao/k8smanage/internal/config"
	"github.com/sunhao/k8smanage/internal/kube"
	"github.com/sunhao/k8smanage/internal/metrics"
	"github.com/sunhao/k8smanage/web"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(log)

	cfg := config.Load()
	log.Info("启动 k8smanage", "addr", cfg.Addr, "interval_sec", cfg.IntervalSec, "db", cfg.DBPath)

	authn := auth.New(cfg.AuthUser, cfg.AuthPass, cfg.AuthToken, cfg.SessionSecret,
		time.Duration(cfg.SessionTTLHours)*time.Hour)
	switch {
	case !authn.Enabled():
		log.Warn("未配置鉴权：任何人都能访问，切勿暴露到不可信网络（设 K8SM_AUTH_USER/K8SM_AUTH_PASS 开启登录）")
	case authn.LoginEnabled():
		log.Info("已启用账号密码登录", "user", cfg.AuthUser)
	default:
		log.Info("已启用 API Token 鉴权（未设账密登录）")
	}

	store, err := metrics.NewSQLiteStore(cfg.DBPath)
	if err != nil {
		log.Error("打开存储失败", "err", err)
		os.Exit(1)
	}
	defer store.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 连 k8s：成功则启用对象缓存 + Pod 指标 + 写操作/流式；失败则降级（仅本机节点指标）。
	kb := kubeBundle{lister: emptyLister{}}
	if cfg.K8sEnabled() {
		if b, err := connectKube(ctx, log); err != nil {
			log.Warn("连接 k8s 失败，降级为仅本机指标模式", "err", err)
		} else {
			kb = b
		}
	}

	node := metrics.NewGopsutilCollector(cfg.HostRoot)
	collector := metrics.NewCollector(store, node, kb.pod, nodeName(cfg))
	go collector.Run(ctx,
		time.Duration(cfg.IntervalSec)*time.Second,
		time.Duration(cfg.RetentionSec)*time.Second)

	// 告警：规则存同库不同表，引擎与采集同频评估。
	ruleStore, err := alert.NewSQLiteRuleStore(cfg.DBPath)
	if err != nil {
		log.Error("打开告警规则存储失败", "err", err)
		os.Exit(1)
	}
	defer ruleStore.Close()
	var notifier alert.Notifier = alert.NopNotifier{}
	if cfg.AlertWebhook != "" {
		notifier = alert.NewWebhookNotifier(cfg.AlertWebhook)
		log.Info("告警 webhook 已配置")
	}
	engine := alert.NewEngine(ruleStore, store, notifier)
	go engine.Run(ctx, time.Duration(cfg.IntervalSec)*time.Second)

	router := api.NewRouter(api.Deps{
		Lister:   kb.lister,
		Store:    store,
		Actions:  kb.actions,
		Streamer: kb.streamer,
		Rules:    ruleStore,
		Alerts:   engine,
		Auth:     authn,
		Static:   web.Dist(),
		BasePath: cfg.BasePath,
	})
	srv := &http.Server{Addr: cfg.Addr, Handler: router}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("HTTP 服务异常", "err", err)
			stop()
		}
	}()
	log.Info("HTTP 已就绪", "addr", cfg.Addr)

	<-ctx.Done()
	log.Info("收到退出信号，正在关闭")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	log.Info("已退出")
}

// kubeBundle 聚合连接 k8s 后得到的各项能力。
type kubeBundle struct {
	lister   api.Lister
	pod      metrics.PodMetricsReader
	actions  api.Actions
	streamer api.Streamer
}

// connectKube 建立 informer 缓存、metrics-server 客户端、typed clientset（写/流式）。
func connectKube(ctx context.Context, log *slog.Logger) (kubeBundle, error) {
	cfg, err := kube.RESTConfig()
	if err != nil {
		return kubeBundle{}, err
	}
	cache, err := kube.NewCache(ctx, cfg)
	if err != nil {
		return kubeBundle{}, err
	}
	mc, err := metricsclient.NewForConfig(cfg)
	if err != nil {
		return kubeBundle{}, err
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return kubeBundle{}, err
	}
	log.Info("已连接 k8s：informer 缓存 + metrics-server + clientset 就绪")
	return kubeBundle{
		lister:   kube.NewLister(cache),
		pod:      metrics.NewPodMetricsReader(mc),
		actions:  kube.NewActions(cs),
		streamer: kube.NewStreamer(cs, cfg),
	}, nil
}

// emptyLister 在无集群时返回空列表，保证 API 不崩。
type emptyLister struct{}

func (emptyLister) Nodes(context.Context) ([]kube.NodeInfo, error) { return nil, nil }
func (emptyLister) Workloads(context.Context, string) ([]kube.WorkloadInfo, error) {
	return nil, nil
}
func (emptyLister) Pods(context.Context, string) ([]kube.PodInfo, error) { return nil, nil }

func nodeName(cfg config.Config) string {
	if cfg.NodeName != "" {
		return cfg.NodeName
	}
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return "localhost"
}
