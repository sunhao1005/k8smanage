package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/sunhao/k8smanage/internal/kube"
	"github.com/sunhao/k8smanage/internal/metrics"
)

func ptr(i int32) *int32 { return &i }

// 装配一套测试依赖：fake k8s（1 节点 + 1 Deployment 全就绪）+ 内存 store（含 n1 一条样本）。
func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "n1"},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
		},
	}
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: ptr(2)},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: 2},
	}
	c := fake.NewClientBuilder().WithScheme(clientgoscheme.Scheme).WithObjects(node, dep).Build()

	store, err := metrics.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	base := time.Unix(1_700_000_000, 0)
	require.NoError(t, store.Write(context.Background(), []metrics.Sample{
		{Kind: metrics.TargetNode, Target: "n1", TS: base, CPU: 0.4, MemUse: 100, MemTot: 200},
		{Kind: metrics.TargetNode, Target: "n1", TS: base.Add(15 * time.Second), CPU: 0.6, MemUse: 120, MemTot: 200},
	}))

	return NewRouter(Deps{Lister: kube.NewLister(c), Store: store})
}

func TestOverview(t *testing.T) {
	r := newTestRouter(t)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/overview", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var ov Overview
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &ov))
	require.Len(t, ov.Nodes, 1)
	require.Equal(t, "n1", ov.Nodes[0].Name)
	require.True(t, ov.Nodes[0].Ready)
	require.True(t, ov.Nodes[0].HasData)
	require.Equal(t, 0.6, ov.Nodes[0].CPU) // 最新样本
	require.Equal(t, 1, ov.Workloads.Total)
	require.Equal(t, 1, ov.Workloads.Ready)
}

func TestMetricsQuery(t *testing.T) {
	r := newTestRouter(t)
	rec := httptest.NewRecorder()
	// 窄区间（<500s）不触发降采样，返回原始两点。
	req := httptest.NewRequest(http.MethodGet,
		"/api/metrics/query?kind=node&target=n1&metric=cpu&from=1699999900&to=1700000100", nil)
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var pts []metrics.Point
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &pts))
	require.Len(t, pts, 2)
	require.Equal(t, 0.4, pts[0].Value)
	require.Equal(t, 0.6, pts[1].Value)
}

// 大区间自动降采样：两点落入同一桶，合并为 1 点（均值 0.5）。
func TestMetricsQueryDownsamples(t *testing.T) {
	r := newTestRouter(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/api/metrics/query?kind=node&target=n1&metric=cpu&from=1699990000&to=1700100000", nil)
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var pts []metrics.Point
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &pts))
	require.Len(t, pts, 1)
	require.InDelta(t, 0.5, pts[0].Value, 1e-9)
}

func TestMetricsQueryBadKind(t *testing.T) {
	r := newTestRouter(t)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/metrics/query?kind=bogus&target=n1&metric=cpu", nil))
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestMetricsQueryBadMetric(t *testing.T) {
	r := newTestRouter(t)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/metrics/query?kind=node&target=n1&metric=bogus", nil))
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestNodesAndWorkloads(t *testing.T) {
	r := newTestRouter(t)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/nodes", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var nodes []kube.NodeInfo
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &nodes))
	require.Len(t, nodes, 1)

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/workloads", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var wls []kube.WorkloadInfo
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &wls))
	require.Len(t, wls, 1)
	require.Equal(t, "web", wls[0].Name)
}
