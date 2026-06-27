package kube

import (
	"context"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NodeInfo 是节点的页面投影。
type NodeInfo struct {
	Name           string   `json:"name"`
	Ready          bool     `json:"ready"`
	Roles          []string `json:"roles"`
	KubeletVersion string   `json:"kubeletVersion"`
}

// WorkloadInfo 是工作负载（Deployment/StatefulSet/DaemonSet）的页面投影。
type WorkloadInfo struct {
	Namespace string `json:"namespace"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Desired   int32  `json:"desired"`
	Ready     int32  `json:"ready"`
}

// PodInfo 是 Pod 的页面投影。
type PodInfo struct {
	Namespace  string   `json:"namespace"`
	Name       string   `json:"name"`
	Phase      string   `json:"phase"`
	Node       string   `json:"node"`
	Ready      bool     `json:"ready"`
	Containers []string `json:"containers"`
}

// Lister 从 client.Reader（真缓存或 fake client 都实现它，评审 #9）读对象。
type Lister struct {
	r client.Reader
}

func NewLister(r client.Reader) *Lister { return &Lister{r: r} }

func (l *Lister) Nodes(ctx context.Context) ([]NodeInfo, error) {
	var list corev1.NodeList
	if err := l.r.List(ctx, &list); err != nil {
		return nil, err
	}
	out := make([]NodeInfo, 0, len(list.Items))
	for i := range list.Items {
		n := &list.Items[i]
		out = append(out, NodeInfo{
			Name:           n.Name,
			Ready:          nodeReady(n),
			Roles:          nodeRoles(n),
			KubeletVersion: n.Status.NodeInfo.KubeletVersion,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Workloads 列工作负载；ns 为空时跨全部命名空间。
func (l *Lister) Workloads(ctx context.Context, ns string) ([]WorkloadInfo, error) {
	var opts []client.ListOption
	if ns != "" {
		opts = append(opts, client.InNamespace(ns))
	}
	var out []WorkloadInfo

	var deps appsv1.DeploymentList
	if err := l.r.List(ctx, &deps, opts...); err != nil {
		return nil, err
	}
	for i := range deps.Items {
		d := &deps.Items[i]
		out = append(out, WorkloadInfo{d.Namespace, "Deployment", d.Name, replicas(d.Spec.Replicas), d.Status.ReadyReplicas})
	}

	var sts appsv1.StatefulSetList
	if err := l.r.List(ctx, &sts, opts...); err != nil {
		return nil, err
	}
	for i := range sts.Items {
		s := &sts.Items[i]
		out = append(out, WorkloadInfo{s.Namespace, "StatefulSet", s.Name, replicas(s.Spec.Replicas), s.Status.ReadyReplicas})
	}

	var ds appsv1.DaemonSetList
	if err := l.r.List(ctx, &ds, opts...); err != nil {
		return nil, err
	}
	for i := range ds.Items {
		d := &ds.Items[i]
		out = append(out, WorkloadInfo{d.Namespace, "DaemonSet", d.Name, d.Status.DesiredNumberScheduled, d.Status.NumberReady})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// Pods 列出 Pod；ns 为空时跨全部命名空间。
func (l *Lister) Pods(ctx context.Context, ns string) ([]PodInfo, error) {
	var opts []client.ListOption
	if ns != "" {
		opts = append(opts, client.InNamespace(ns))
	}
	var list corev1.PodList
	if err := l.r.List(ctx, &list, opts...); err != nil {
		return nil, err
	}
	out := make([]PodInfo, 0, len(list.Items))
	for i := range list.Items {
		p := &list.Items[i]
		containers := make([]string, 0, len(p.Spec.Containers))
		for _, c := range p.Spec.Containers {
			containers = append(containers, c.Name)
		}
		out = append(out, PodInfo{
			Namespace:  p.Namespace,
			Name:       p.Name,
			Phase:      string(p.Status.Phase),
			Node:       p.Spec.NodeName,
			Ready:      podReady(p),
			Containers: containers,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func podReady(p *corev1.Pod) bool {
	for _, c := range p.Status.Conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

func nodeReady(n *corev1.Node) bool {
	for _, c := range n.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

func nodeRoles(n *corev1.Node) []string {
	var roles []string
	const prefix = "node-role.kubernetes.io/"
	for k := range n.Labels {
		if strings.HasPrefix(k, prefix) {
			if r := strings.TrimPrefix(k, prefix); r != "" {
				roles = append(roles, r)
			}
		}
	}
	sort.Strings(roles)
	return roles
}

func replicas(p *int32) int32 {
	if p == nil {
		return 1
	}
	return *p
}
