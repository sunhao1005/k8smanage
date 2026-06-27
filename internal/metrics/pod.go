package metrics

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
)

// podMetricsReader 通过 metrics-server 的 metrics.k8s.io API 读取 Pod 指标。
type podMetricsReader struct {
	mc metricsclient.Interface
}

// NewPodMetricsReader 构造读取器；mc 为 metrics clientset（in-cluster 装配）。
func NewPodMetricsReader(mc metricsclient.Interface) PodMetricsReader {
	return &podMetricsReader{mc: mc}
}

func (p *podMetricsReader) Read(ctx context.Context) ([]Sample, error) {
	list, err := p.mc.MetricsV1beta1().PodMetricses(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	now := time.Now()
	out := make([]Sample, 0, len(list.Items))
	for i := range list.Items {
		pm := &list.Items[i]
		var milliCPU int64
		var memBytes int64
		for _, ct := range pm.Containers {
			milliCPU += ct.Usage.Cpu().MilliValue()
			memBytes += ct.Usage.Memory().Value()
		}
		out = append(out, Sample{
			Kind:   TargetPod,
			Target: pm.Namespace + "/" + pm.Name,
			TS:     now,
			CPU:    float64(milliCPU) / 1000.0, // 毫核 → 核
			MemUse: uint64(memBytes),
		})
	}
	return out, nil
}
