package kube

import (
	"context"
	"fmt"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const restartAnnotation = "kubectl.kubernetes.io/restartedAt"

// pausedReplicasAnnotation 暂停时把原副本数存这，启用时读回。
const pausedReplicasAnnotation = "k8smanage.io/paused-replicas"

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

// Pause 暂停工作负载：记下当前副本数到注解，再缩到 0（Pod 移除、释放资源）。
func (a *Actions) Pause(ctx context.Context, ns, kind, name string) error {
	switch kind {
	case "Deployment":
		c := a.cs.AppsV1().Deployments(ns)
		d, err := c.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		pauseMeta(&d.ObjectMeta, replicas(d.Spec.Replicas))
		zero := int32(0)
		d.Spec.Replicas = &zero
		_, err = c.Update(ctx, d, metav1.UpdateOptions{})
		return err
	case "StatefulSet":
		c := a.cs.AppsV1().StatefulSets(ns)
		s, err := c.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		pauseMeta(&s.ObjectMeta, replicas(s.Spec.Replicas))
		zero := int32(0)
		s.Spec.Replicas = &zero
		_, err = c.Update(ctx, s, metav1.UpdateOptions{})
		return err
	default:
		return fmt.Errorf("%s 不支持暂停", kind)
	}
}

// Resume 启用工作负载：从注解读回原副本数（缺省 1）恢复，并清除注解。
func (a *Actions) Resume(ctx context.Context, ns, kind, name string) error {
	switch kind {
	case "Deployment":
		c := a.cs.AppsV1().Deployments(ns)
		d, err := c.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		r := resumeReplicas(d.Annotations)
		d.Spec.Replicas = &r
		delete(d.Annotations, pausedReplicasAnnotation)
		_, err = c.Update(ctx, d, metav1.UpdateOptions{})
		return err
	case "StatefulSet":
		c := a.cs.AppsV1().StatefulSets(ns)
		s, err := c.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		r := resumeReplicas(s.Annotations)
		s.Spec.Replicas = &r
		delete(s.Annotations, pausedReplicasAnnotation)
		_, err = c.Update(ctx, s, metav1.UpdateOptions{})
		return err
	default:
		return fmt.Errorf("%s 不支持启用", kind)
	}
}

// DeletePod 删除一个 Pod（由控制器自动重建）。
func (a *Actions) DeletePod(ctx context.Context, ns, name string) error {
	return a.cs.CoreV1().Pods(ns).Delete(ctx, name, metav1.DeleteOptions{})
}

// pauseMeta 若当前副本>0，把它记到注解（用于启用时恢复）。
func pauseMeta(m *metav1.ObjectMeta, cur int32) {
	if cur <= 0 {
		return
	}
	if m.Annotations == nil {
		m.Annotations = map[string]string{}
	}
	m.Annotations[pausedReplicasAnnotation] = strconv.Itoa(int(cur))
}

// resumeReplicas 从注解读回原副本数；缺失或非法则回落 1。
func resumeReplicas(anno map[string]string) int32 {
	if anno != nil {
		if v, ok := anno[pausedReplicasAnnotation]; ok {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				return int32(n)
			}
		}
	}
	return 1
}

func setRestartAnno(t *corev1.PodTemplateSpec, ts string) {
	if t.Annotations == nil {
		t.Annotations = map[string]string{}
	}
	t.Annotations[restartAnnotation] = ts
}
