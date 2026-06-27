package kube

import (
	"context"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// Streamer 提供流式能力：Pod 日志与容器 exec。需要 clientset + rest.Config。
type Streamer struct {
	cs  kubernetes.Interface
	cfg *rest.Config
}

func NewStreamer(cs kubernetes.Interface, cfg *rest.Config) *Streamer {
	return &Streamer{cs: cs, cfg: cfg}
}

// PodLogs 返回 Pod 日志流（follow=true 持续输出）。调用方负责 Close。
func (s *Streamer) PodLogs(ctx context.Context, ns, pod, container string, follow bool) (io.ReadCloser, error) {
	opts := &corev1.PodLogOptions{Container: container, Follow: follow}
	return s.cs.CoreV1().Pods(ns).GetLogs(pod, opts).Stream(ctx)
}

// ExecParams 描述一次 exec 会话。
type ExecParams struct {
	Namespace string
	Pod       string
	Container string
	Command   []string
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
	TTY       bool
	Resize    remotecommand.TerminalSizeQueue
}

// Exec 在容器内执行命令并桥接 IO。
func (s *Streamer) Exec(ctx context.Context, p ExecParams) error {
	req := s.cs.CoreV1().RESTClient().Post().
		Resource("pods").Name(p.Pod).Namespace(p.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: p.Container,
			Command:   p.Command,
			Stdin:     p.Stdin != nil,
			Stdout:    p.Stdout != nil,
			Stderr:    p.Stderr != nil && !p.TTY, // TTY 模式下 stderr 合并到 stdout
			TTY:       p.TTY,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(s.cfg, "POST", req.URL())
	if err != nil {
		return err
	}
	return exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:             p.Stdin,
		Stdout:            p.Stdout,
		Stderr:            p.Stderr,
		Tty:               p.TTY,
		TerminalSizeQueue: p.Resize,
	})
}
