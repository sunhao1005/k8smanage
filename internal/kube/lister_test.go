package kube

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func ptr(i int32) *int32 { return &i }

func TestStripManagedFields(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:          "p",
		ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "kubelet"}},
	}}
	out, err := stripManagedFields(pod)
	require.NoError(t, err)
	require.Nil(t, out.(*corev1.Pod).ManagedFields, "managedFields 应被剥离")
}

func TestListerNodes(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "n1",
			Labels: map[string]string{"node-role.kubernetes.io/control-plane": "true"},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
			NodeInfo:   corev1.NodeSystemInfo{KubeletVersion: "v1.31.0+k3s1"},
		},
	}
	c := fake.NewClientBuilder().WithScheme(clientgoscheme.Scheme).WithObjects(node).Build()
	l := NewLister(c)

	nodes, err := l.Nodes(context.Background())
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, "n1", nodes[0].Name)
	require.True(t, nodes[0].Ready)
	require.Equal(t, []string{"control-plane"}, nodes[0].Roles)
	require.Equal(t, "v1.31.0+k3s1", nodes[0].KubeletVersion)
}

func TestListerWorkloads(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: ptr(3)},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: 2},
	}
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default"},
		Spec:       appsv1.StatefulSetSpec{Replicas: ptr(1)},
		Status:     appsv1.StatefulSetStatus{ReadyReplicas: 1},
	}
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "kube-system"},
		Status:     appsv1.DaemonSetStatus{DesiredNumberScheduled: 1, NumberReady: 1},
	}
	c := fake.NewClientBuilder().WithScheme(clientgoscheme.Scheme).WithObjects(dep, sts, ds).Build()
	l := NewLister(c)

	all, err := l.Workloads(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, all, 3)

	def, err := l.Workloads(context.Background(), "default")
	require.NoError(t, err)
	require.Len(t, def, 2)

	// 校验 Deployment 的副本数映射正确。
	var web WorkloadInfo
	for _, w := range def {
		if w.Name == "web" {
			web = w
		}
	}
	require.Equal(t, "Deployment", web.Kind)
	require.Equal(t, int32(3), web.Desired)
	require.Equal(t, int32(2), web.Ready)
}
