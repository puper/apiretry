package upstream

import (
	"net/http"

	"github.com/puper/apiretry/internal/config"
)

// Doer 封装 HTTP 请求执行接口，便于测试时替换为 mock
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	httpClient *http.Client
	config     *config.UpstreamConfig
}

// NewClient 创建上游客户端。Timeout 设为 0，超时由外部 context 和 retry 层控制。
func NewClient(cfg *config.UpstreamConfig) *Client {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
		IdleConnTimeout:       cfg.IdleConnTimeout,
		TLSHandshakeTimeout:   cfg.TLSHandshakeTimeout,
		ResponseHeaderTimeout: cfg.ResponseHeaderTimeout,
		ForceAttemptHTTP2:     cfg.ForceAttemptHTTP2,
	}
	return &Client{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   0,
		},
		config: cfg,
	}
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}
