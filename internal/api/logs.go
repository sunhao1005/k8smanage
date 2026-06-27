package api

import (
	"io"
	"net/http"

	"github.com/coder/websocket"
)

func (s *server) handleLogs(w http.ResponseWriter, r *http.Request) {
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
	follow := q.Get("follow") == "1"

	// 鉴权已在中间件完成（WS 用 ?token=）。Origin 校验放宽，靠不可猜的 token 防 CSRF。
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer c.CloseNow()
	ctx := r.Context()

	stream, err := s.d.Streamer.PodLogs(ctx, ns, pod, container, follow)
	if err != nil {
		c.Close(websocket.StatusInternalError, "打开日志流失败")
		return
	}
	defer stream.Close()

	buf := make([]byte, 8192)
	for {
		n, rerr := stream.Read(buf)
		if n > 0 {
			if werr := c.Write(ctx, websocket.MessageText, buf[:n]); werr != nil {
				return // 客户端断开
			}
		}
		if rerr != nil {
			if rerr == io.EOF {
				c.Close(websocket.StatusNormalClosure, "")
			} else {
				c.Close(websocket.StatusInternalError, "读取日志出错")
			}
			return
		}
	}
}
