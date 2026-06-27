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
