package api

import (
	"context"
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sunhao/k8smanage/internal/alert"
	"github.com/sunhao/k8smanage/internal/auth"
	"github.com/sunhao/k8smanage/internal/kube"
	"github.com/sunhao/k8smanage/internal/metrics"
)

// Lister 是 API 依赖的对象读取能力（*kube.Lister 实现；便于测试替换）。
type Lister interface {
	Nodes(ctx context.Context) ([]kube.NodeInfo, error)
	Workloads(ctx context.Context, ns string) ([]kube.WorkloadInfo, error)
	Pods(ctx context.Context, ns string) ([]kube.PodInfo, error)
}

// Actions 是工作负载/Pod 写操作（*kube.Actions 实现）。
type Actions interface {
	Scale(ctx context.Context, ns, kind, name string, replicas int32) error
	Restart(ctx context.Context, ns, kind, name string) error
	Pause(ctx context.Context, ns, kind, name string) error
	Resume(ctx context.Context, ns, kind, name string) error
	DeletePod(ctx context.Context, ns, name string) error
}

// Streamer 是日志/exec 流式能力（*kube.Streamer 实现）。
type Streamer interface {
	PodLogs(ctx context.Context, ns, pod, container string, follow bool) (io.ReadCloser, error)
	Exec(ctx context.Context, p kube.ExecParams) error
}

// AlertProvider 提供当前告警快照（*alert.Engine 实现）。
type AlertProvider interface {
	Active() []alert.ActiveAlert
}

// Deps 是组装 API 路由所需的依赖。
type Deps struct {
	Lister   Lister
	Store    metrics.MetricStore
	Actions  Actions             // 无集群时为 nil，相关路由返回 503
	Streamer Streamer            // 无集群时为 nil
	Rules    alert.RuleStore     // 告警规则存储
	Alerts   AlertProvider       // 当前告警
	Auth     *auth.Authenticator // 账密登录 + API Token 鉴权；nil 则不鉴权
	Static   fs.FS               // 前端静态资源；为 nil 时不挂静态路由（测试用）
	BasePath string              // 子路径前缀（如 /k8smanage）；空=根路径
}

// NewRouter 装配 chi 路由。健康检查固定在根（探针不受子路径影响）；
// 其余（API + 前端）整体挂到 BasePath 前缀下，支持域名子路径反代。
func NewRouter(d Deps) http.Handler {
	base := NormalizeBasePath(d.BasePath)

	outer := chi.NewRouter()
	outer.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	inner := newAppRouter(d, base)
	if base == "" {
		outer.Mount("/", inner)
	} else {
		// 用 StripPrefix 真正剥掉前缀（改写 r.URL.Path），
		// 这样 chi 路由与 http.FileServer 看到的都是剥离后的路径，静态资源才能命中。
		outer.Handle(base+"/*", http.StripPrefix(base, inner))
		// 访问无尾斜杠的前缀时跳到带斜杠版本。
		outer.Get(base, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, base+"/", http.StatusMovedPermanently)
		})
	}
	return outer
}

// newAppRouter 构建实际业务路由（API + 静态），相对自身根。
func newAppRouter(d Deps, base string) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	s := &server{d: d}

	r.Route("/api", func(r chi.Router) {
		// 免鉴权：登录、鉴权配置探测。
		r.Post("/login", s.handleLogin)
		r.Get("/config", s.handleAuthConfig)

		// 需鉴权的其余 API。
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware(d.Auth))
			r.Get("/overview", s.handleOverview)
			r.Get("/nodes", s.handleNodes)
			r.Get("/workloads", s.handleWorkloads)
			r.Get("/pods", s.handlePods)
			r.Get("/metrics/query", s.handleMetricsQuery)

			// 写操作（M3）
			r.Post("/workloads/{ns}/{kind}/{name}/scale", s.handleScale)
			r.Post("/workloads/{ns}/{kind}/{name}/restart", s.handleRestart)
			r.Post("/workloads/{ns}/{kind}/{name}/pause", s.handlePause)
			r.Post("/workloads/{ns}/{kind}/{name}/resume", s.handleResume)
			r.Delete("/pods/{ns}/{name}", s.handleDeletePod)

			// 流式（M3）
			r.Get("/logs", s.handleLogs)
			r.Get("/exec", s.handleExec)

			// 告警（M4）
			r.Get("/alerts/rules", s.handleListRules)
			r.Put("/alerts/rules", s.handleUpsertRule)
			r.Delete("/alerts/rules/{id}", s.handleDeleteRule)
			r.Get("/alerts/active", s.handleActiveAlerts)
		})
	})

	if d.Static != nil {
		r.Handle("/*", spaFileServer(d.Static, base))
	}
	return r
}

type server struct {
	d Deps
}

// NormalizeBasePath 规整子路径前缀：去空白与尾斜杠、补前导斜杠；空或 "/" → ""。
func NormalizeBasePath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimRight(p, "/")
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}
