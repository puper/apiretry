package util

import (
	"net/http"
	"testing"
)

func TestRemoveHopByHopHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("Connection", "keep-alive")
	h.Set("Transfer-Encoding", "chunked")
	h.Set("Keep-Alive", "timeout=5")
	h.Set("Proxy-Authenticate", "Basic")
	h.Set("Proxy-Authorization", "Basic abc")
	h.Set("TE", "trailers")
	h.Set("Trailers", "X-Trailer")
	h.Set("Upgrade", "websocket")
	h.Set("Content-Type", "application/json")
	h.Set("Authorization", "Bearer token")

	RemoveHopByHopHeaders(h)

	for _, hdr := range hopByHopHeaders {
		if v := h.Get(hdr); v != "" {
			t.Errorf("hop-by-hop header %q 应被移除, 但值为 %q", hdr, v)
		}
	}
	if v := h.Get("Content-Type"); v != "application/json" {
		t.Errorf("Content-Type = %q, 期望 \"application/json\"", v)
	}
	if v := h.Get("Authorization"); v != "Bearer token" {
		t.Errorf("Authorization = %q, 期望 \"Bearer token\"", v)
	}
}

func TestCopyResponseHeaders(t *testing.T) {
	src := http.Header{}
	src.Set("Content-Type", "text/event-stream")
	src.Set("X-Custom", "value")
	src.Set("Connection", "keep-alive")
	src.Set("Transfer-Encoding", "chunked")

	dst := http.Header{}
	CopyResponseHeaders(dst, src)

	if dst.Get("Content-Type") != "text/event-stream" {
		t.Errorf("Content-Type = %q, 期望 \"text/event-stream\"", dst.Get("Content-Type"))
	}
	if dst.Get("X-Custom") != "value" {
		t.Errorf("X-Custom = %q, 期望 \"value\"", dst.Get("X-Custom"))
	}
	if dst.Get("Connection") != "" {
		t.Errorf("Connection 应被移除, 值为 %q", dst.Get("Connection"))
	}
	if dst.Get("Transfer-Encoding") != "" {
		t.Errorf("Transfer-Encoding 应被移除, 值为 %q", dst.Get("Transfer-Encoding"))
	}
}
