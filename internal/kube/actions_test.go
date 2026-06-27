package kube

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestActionsScale(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: ptr(1)},
	}
	cs := fake.NewSimpleClientset(dep)
	a := NewActions(cs)
	ctx := context.Background()

	require.NoError(t, a.Scale(ctx, "default", "Deployment", "web", 3))
	got, err := cs.AppsV1().Deployments("default").Get(ctx, "web", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, int32(3), *got.Spec.Replicas)

	require.Error(t, a.Scale(ctx, "default", "DaemonSet", "x", 2)) // 不支持
	require.Error(t, a.Scale(ctx, "default", "Deployment", "web", -1))
}

func TestActionsPauseResume(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: ptr(3)},
	}
	cs := fake.NewSimpleClientset(dep)
	a := NewActions(cs)
	ctx := context.Background()

	// 暂停：记下副本数 3 到注解，缩到 0
	require.NoError(t, a.Pause(ctx, "default", "Deployment", "web"))
	got, _ := cs.AppsV1().Deployments("default").Get(ctx, "web", metav1.GetOptions{})
	require.Equal(t, int32(0), *got.Spec.Replicas)
	require.Equal(t, "3", got.Annotations[pausedReplicasAnnotation])

	// 启用：从注解恢复到 3，清除注解
	require.NoError(t, a.Resume(ctx, "default", "Deployment", "web"))
	got, _ = cs.AppsV1().Deployments("default").Get(ctx, "web", metav1.GetOptions{})
	require.Equal(t, int32(3), *got.Spec.Replicas)
	require.NotContains(t, got.Annotations, pausedReplicasAnnotation)
}

func TestActionsResumeWithoutAnnotationDefaultsToOne(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: ptr(0)},
	}
	cs := fake.NewSimpleClientset(dep)
	a := NewActions(cs)
	require.NoError(t, a.Resume(context.Background(), "default", "Deployment", "web"))
	got, _ := cs.AppsV1().Deployments("default").Get(context.Background(), "web", metav1.GetOptions{})
	require.Equal(t, int32(1), *got.Spec.Replicas)
}

func TestActionsRestart(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: ptr(1)},
	}
	cs := fake.NewSimpleClientset(dep)
	a := NewActions(cs)
	ctx := context.Background()

	require.NoError(t, a.Restart(ctx, "default", "Deployment", "web"))
	got, err := cs.AppsV1().Deployments("default").Get(ctx, "web", metav1.GetOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, got.Spec.Template.Annotations[restartAnnotation])
}

func TestActionsDeletePod(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p1", Namespace: "default"}}
	cs := fake.NewSimpleClientset(pod)
	a := NewActions(cs)
	ctx := context.Background()

	require.NoError(t, a.DeletePod(ctx, "default", "p1"))
	_, err := cs.CoreV1().Pods("default").Get(ctx, "p1", metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err))
}
