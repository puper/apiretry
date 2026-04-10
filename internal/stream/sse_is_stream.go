package stream

import (
	"net/http"
	"strings"
)

// IsStreamRequest 检测请求是否为流式请求。
// 优先检查 body 中的 "stream":true，否则检查 Accept: text/event-stream header。
func IsStreamRequest(r *http.Request, body []byte) bool {
	if len(body) > 0 {
		return strings.Contains(string(body), `"stream":true`) ||
			strings.Contains(string(body), `"stream" : true`)
	}
	return strings.Contains(r.Header.Get("Accept"), "text/event-stream")
}
