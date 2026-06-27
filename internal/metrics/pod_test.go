package metrics

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"
	metricsapi "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

func TestPodMetricsReaderRead(t *testing.T) {
	pm := metricsapi.PodMetrics{
		ObjectMeta: metav1.ObjectMeta{Name: "web-abc", Namespace: "default"},
		Containers: []metricsapi.ContainerMetrics{
			{Name: "app", Usage: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("250m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			}},
			{Name: "sidecar", Usage: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("250m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			}},
		},
	}
	cs := metricsfake.NewSimpleClientset()
	// fake tracker 对聚合类型 List 复数名匹配有坑，用 reactor 直接返回列表。
	// metrics.k8s.io 里 PodMetrics 的资源名是 "pods"（非 podmetricses）。
	cs.PrependReactor("list", "pods", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, &metricsapi.PodMetricsList{Items: []metricsapi.PodMetrics{pm}}, nil
	})

	r := NewPodMetricsReader(cs)
	samples, err := r.Read(context.Background())
	require.NoError(t, err)
	require.Len(t, samples, 1)

	s := samples[0]
	require.Equal(t, TargetPod, s.Kind)
	require.Equal(t, "default/web-abc", s.Target)
	require.InDelta(t, 0.5, s.CPU, 1e-9)              // 250m + 250m = 0.5 核
	require.Equal(t, uint64(256*1024*1024), s.MemUse) // 128Mi + 128Mi
}
