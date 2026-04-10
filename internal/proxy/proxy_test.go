package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/puper/apiretry/internal/config"
	"github.com/puper/apiretry/internal/retry"
	"github.com/puper/apiretry/internal/stream"
	"github.com/puper/apiretry/internal/upstream"
	"log/slog"
)

func testConfig() *config.Config {
	cfg := config.DefaultConfig()
	cfg.Upstream.BaseURL = "http://test-upstream"
	cfg.Retry.MaxAttempts = 3
	cfg.Retry.MaxRetryDelayBudget = 10 * time.Second
	cfg.Retry.FirstByteTimeout = 3 * time.Second
	cfg.Retry.ChunkIdleTimeout = 5 * time.Second
	cfg.Retry.MaxPerRetryDelay = 2 * time.Second
	cfg.Retry.Schedule429 = []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond}
	cfg.Retry.Schedule5xx = []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond}
	cfg.Retry.JitterPercent = 0
	cfg.Limits.MaxRequestBodyBytes = 1024 * 1024
	return cfg
}

func testDoer(server *httptest.Server) upstream.Doer {
	client := server.Client()
	return &testDoerImpl{baseURL: server.URL, client: client}
}

type testDoerImpl struct {
	baseURL string
	client  *http.Client
}

func (t *testDoerImpl) Do(req *http.Request) (*http.Response, error) {
	return t.client.Do(req)
}

func TestHandler_StreamDetection(t *testing.T) {
	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"test"}`))
	}))
	defer server.Close()

	cfg := testConfig()
	cfg.Upstream.BaseURL = server.URL
	policy := retry.NewPolicy(&cfg.Retry)
	probe := &stream.DefaultProbe{}
	doer := testDoer(server)
	logger := slog.Default()

	handler := NewHandler(doer, policy, probe, cfg, logger)

	body := `{"model":"gpt-4","stream":true,"messages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if receivedPath != "/v1/chat/completions" {
		t.Fatalf("received path = %s, 期望 /v1/chat/completions", receivedPath)
	}
}

func TestHandler_NonStreamRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"test","choices":[]}`))
	}))
	defer server.Close()

	cfg := testConfig()
	cfg.Upstream.BaseURL = server.URL
	policy := retry.NewPolicy(&cfg.Retry)
	probe := &stream.DefaultProbe{}
	doer := testDoer(server)
	logger := slog.Default()

	handler := NewHandler(doer, policy, probe, cfg, logger)

	body := `{"model":"gpt-4","messages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, 期望 200", rec.Code)
	}
}

func TestHandler_BodyTooLarge(t *testing.T) {
	cfg := testConfig()
	cfg.Limits.MaxRequestBodyBytes = 10

	policy := retry.NewPolicy(&cfg.Retry)
	probe := &stream.DefaultProbe{}
	logger := slog.Default()

	doer := &noopDoer{}
	handler := NewHandler(doer, policy, probe, cfg, logger)

	largeBody := strings.Repeat("x", 100)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, 期望 413", rec.Code)
	}
}

type noopDoer struct{}

func (d *noopDoer) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}, nil
}
