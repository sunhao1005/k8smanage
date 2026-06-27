package kube

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const restartAnnotation = "kubectl.kubernetes.io/restartedAt"

// Actions 封装对工作负载/Pod 的写操作，走 typed clientset（缓存只读，不能写）。
type Actions struct {
	cs kubernetes.Interface
}

func NewActions(cs kubernetes.Interface) *Actions { return &Actions{cs: cs} }

// Scale 调整 Deployment/StatefulSet 的副本数（DaemonSet 无副本概念，不支持）。
func (a *Actions) Scale(ctx context.Context, ns, kind, name string, replicas int32) error {
	if replicas < 0 {
		return fmt.Errorf("replicas 不能为负数")
	}
	switch kind {
	case "Deployment":
		d, err := a.cs.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		d.Spec.Replicas = &replicas
		_, err = a.cs.AppsV1().Deployments(ns).Update(ctx, d, metav1.UpdateOptions{})
		return err
	case "StatefulSet":
		s, err := a.cs.AppsV1().StatefulSets(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		s.Spec.Replicas = &replicas
		_, err = a.cs.AppsV1().StatefulSets(ns).Update(ctx, s, metav1.UpdateOptions{})
		return err
	default:
		return fmt.Errorf("%s 不支持扩缩容", kind)
	}
}

// Restart 通过给 Pod 模板打时间戳注解触发滚动重启（等价于 kubectl rollout restart）。
func (a *Actions) Restart(ctx context.Context, ns, kind, name string) error {
	now := time.Now().Format(time.RFC3339)
	switch kind {
	case "Deployment":
		d, err := a.cs.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		setRestartAnno(&d.Spec.Template, now)
		_, err = a.cs.AppsV1().Deployments(ns).Update(ctx, d, metav1.UpdateOptions{})
		return err
	case "StatefulSet":
		s, err := a.cs.AppsV1().StatefulSets(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		setRestartAnno(&s.Spec.Template, now)
		_, err = a.cs.AppsV1().StatefulSets(ns).Update(ctx, s, metav1.UpdateOptions{})
		return err
	case "DaemonSet":
		d, err := a.cs.AppsV1().DaemonSets(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		setRestartAnno(&d.Spec.Template, now)
		_, err = a.cs.AppsV1().DaemonSets(ns).Update(ctx, d, metav1.UpdateOptions{})
		return err
	default:
		return fmt.Errorf("%s 不支持重启", kind)
	}
}

// DeletePod 删除一个 Pod（由控制器自动重建）。
func (a *Actions) DeletePod(ctx context.Context, ns, name string) error {
	return a.cs.CoreV1().Pods(ns).Delete(ctx, name, metav1.DeleteOptions{})
}

func setRestartAnno(t *corev1.PodTemplateSpec, ts string) {
	if t.Annotations == nil {
		t.Annotations = map[string]string{}
	}
	t.Annotations[restartAnnotation] = ts
}
