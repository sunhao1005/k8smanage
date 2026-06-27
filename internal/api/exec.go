package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/coder/websocket"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/sunhao/k8smanage/internal/kube"
)

func (s *server) handleExec(w http.ResponseWriter, r *http.Request) {
	if s.d.Streamer == nil {
		writeErr(w, http.StatusServiceUnavailable, "未连接集群")
		return
	}
	q := r.URL.Query()
	ns, pod, container := q.Get("ns"), q.Get("pod"), q.Get("container")
	if ns == "" || pod == "" {
		writeErr(w, http.StatusBadRequest, "需要 ns 和 pod 参数")
		return
	}
	shell := q.Get("shell")
	if shell == "" {
		shell = "/bin/sh"
	}

	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer c.CloseNow()
	ctx := r.Context()

	conn := newWSConn(ctx, c)
	err = s.d.Streamer.Exec(ctx, kube.ExecParams{
		Namespace: ns, Pod: pod, Container: container,
		Command: []string{shell},
		Stdin:   conn, Stdout: conn, TTY: true, Resize: conn,
	})
	if err != nil {
		c.Close(websocket.StatusInternalError, "exec 失败")
		return
	}
	c.Close(websocket.StatusNormalClosure, "")
}

// wsConn 把 WebSocket 适配为 exec 所需的 stdin(io.Reader)/stdout(io.Writer)/resize 队列。
// 协议：文本消息若是 {"type":"resize","cols":C,"rows":R} 视为窗口调整，否则及二进制消息均当作 stdin。
type wsConn struct {
	ctx        context.Context
	c          *websocket.Conn
	rbuf       []byte
	sizes      chan remotecommand.TerminalSize
	closeSizes sync.Once
}

func newWSConn(ctx context.Context, c *websocket.Conn) *wsConn {
	return &wsConn{ctx: ctx, c: c, sizes: make(chan remotecommand.TerminalSize, 1)}
}

func (x *wsConn) Read(p []byte) (int, error) {
	for len(x.rbuf) == 0 {
		typ, data, err := x.c.Read(x.ctx)
		if err != nil {
			x.closeSizes.Do(func() { close(x.sizes) })
			return 0, err
		}
		if typ == websocket.MessageText {
			if sz, ok := parseResize(data); ok {
				select {
				case x.sizes <- sz:
				default: // 丢弃过期的 resize，不阻塞
				}
				continue
			}
		}
		x.rbuf = data
	}
	n := copy(p, x.rbuf)
	x.rbuf = x.rbuf[n:]
	return n, nil
}

func (x *wsConn) Write(p []byte) (int, error) {
	if err := x.c.Write(x.ctx, websocket.MessageBinary, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Next 实现 remotecommand.TerminalSizeQueue。
func (x *wsConn) Next() *remotecommand.TerminalSize {
	s, ok := <-x.sizes
	if !ok {
		return nil
	}
	return &s
}

func parseResize(data []byte) (remotecommand.TerminalSize, bool) {
	var m struct {
		Type string `json:"type"`
		Cols uint16 `json:"cols"`
		Rows uint16 `json:"rows"`
	}
	if err := json.Unmarshal(data, &m); err != nil || m.Type != "resize" {
		return remotecommand.TerminalSize{}, false
	}
	return remotecommand.TerminalSize{Width: m.Cols, Height: m.Rows}, true
}
