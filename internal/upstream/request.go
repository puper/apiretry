package upstream

import (
	"bytes"
	"io"
	"net/http"
	"net/url"

	"github.com/puper/apiretry/internal/config"
	"github.com/puper/apiretry/internal/util"
)

// BuildRequest 将原始请求重写为发往上游 LLM API 的请求。
// bodyBytes 为已缓存的请求体字节，允许多次读取。
func BuildRequest(r *http.Request, cfg *config.UpstreamConfig, bodyBytes []byte) (*http.Request, error) {
	targetURL, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	targetURL.Path = r.URL.Path
	targetURL.RawQuery = r.URL.RawQuery

	newReq, err := http.NewRequestWithContext(
		r.Context(),
		r.Method,
		targetURL.String(),
		io.NopCloser(bytes.NewReader(bodyBytes)),
	)
	if err != nil {
		return nil, err
	}

	for k, vv := range r.Header {
		newReq.Header[k] = append([]string(nil), vv...)
	}
	util.RemoveHopByHopHeaders(newReq.Header)

	if newReq.Header.Get("Content-Type") == "" {
		newReq.Header.Set("Content-Type", "application/json")
	}

	if newReq.Header.Get("Accept") == "" {
		newReq.Header.Set("Accept", "text/event-stream")
	}

	return newReq, nil
}
