package stream

import (
	"net/http"
)

// FlushWriter 包装 http.ResponseWriter，在每次 Write 后自动 Flush。
// 优先使用 http.ResponseController（Go 1.22+），降级到类型断言 http.Flusher。
type FlushWriter struct {
	w http.ResponseWriter
}

func NewFlushWriter(w http.ResponseWriter) *FlushWriter {
	return &FlushWriter{w: w}
}

func (fw *FlushWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	if err == nil {
		fw.Flush()
	}
	return n, err
}

func (fw *FlushWriter) Flush() {
	rc := http.NewResponseController(fw.w)
	if err := rc.Flush(); err == nil {
		return
	}
	if f, ok := fw.w.(http.Flusher); ok {
		f.Flush()
	}
}
