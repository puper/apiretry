package util

import "net/http"

// hop-by-hop headers 不应被代理转发（RFC 2616 §13.5.1）
var hopByHopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"TE",
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}

func RemoveHopByHopHeaders(h http.Header) {
	for _, hdr := range hopByHopHeaders {
		h.Del(hdr)
	}
}

func CopyResponseHeaders(dst, src http.Header) {
	for k, vv := range src {
		dst[k] = append([]string(nil), vv...)
	}
	RemoveHopByHopHeaders(dst)
}
