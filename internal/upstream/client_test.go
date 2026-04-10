package upstream

import (
	"net/http"
	"testing"
	"time"

	"github.com/puper/apiretry/internal/config"
)

func TestNewClient_TransportConfig(t *testing.T) {
	cfg := &config.UpstreamConfig{
		BaseURL:               "https://api.openai.com",
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   50,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ForceAttemptHTTP2:     true,
	}

	c := NewClient(cfg)

	transport, ok := c.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Transport 不是 *http.Transport")
	}

	if transport.MaxIdleConns != cfg.MaxIdleConns {
		t.Errorf("MaxIdleConns = %d, want %d", transport.MaxIdleConns, cfg.MaxIdleConns)
	}
	if transport.MaxIdleConnsPerHost != cfg.MaxIdleConnsPerHost {
		t.Errorf("MaxIdleConnsPerHost = %d, want %d", transport.MaxIdleConnsPerHost, cfg.MaxIdleConnsPerHost)
	}
	if transport.IdleConnTimeout != cfg.IdleConnTimeout {
		t.Errorf("IdleConnTimeout = %v, want %v", transport.IdleConnTimeout, cfg.IdleConnTimeout)
	}
	if transport.TLSHandshakeTimeout != cfg.TLSHandshakeTimeout {
		t.Errorf("TLSHandshakeTimeout = %v, want %v", transport.TLSHandshakeTimeout, cfg.TLSHandshakeTimeout)
	}
	if transport.ResponseHeaderTimeout != cfg.ResponseHeaderTimeout {
		t.Errorf("ResponseHeaderTimeout = %v, want %v", transport.ResponseHeaderTimeout, cfg.ResponseHeaderTimeout)
	}
	if transport.ForceAttemptHTTP2 != cfg.ForceAttemptHTTP2 {
		t.Errorf("ForceAttemptHTTP2 = %v, want %v", transport.ForceAttemptHTTP2, cfg.ForceAttemptHTTP2)
	}
}

func TestNewClient_TimeoutIsZero(t *testing.T) {
	cfg := &config.UpstreamConfig{
		BaseURL: "https://api.openai.com",
		Timeout: 120 * time.Second,
	}

	c := NewClient(cfg)

	if c.httpClient.Timeout != 0 {
		t.Errorf("Timeout = %v, want 0 (超时由 context 控制)", c.httpClient.Timeout)
	}
}
