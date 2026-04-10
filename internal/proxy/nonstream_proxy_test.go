package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/puper/apiretry/internal/retry"
	"log/slog"
)

func TestNonStreamProxy_HappyPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"chatcmpl-1","choices":[]}`))
	}))
	defer server.Close()

	cfg := testConfig()
	cfg.Upstream.BaseURL = server.URL
	policy := retry.NewPolicy(&cfg.Retry)
	logger := slog.Default()
	doer := testDoer(server)

	nsp := NewNonStreamProxy(doer, policy, cfg, logger)

	body := `{"model":"gpt-4","messages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	nsp.ServeHTTP(rec, req, []byte(body))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, 期望 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "chatcmpl-1") {
		t.Fatalf("响应体应包含 chatcmpl-1, got: %s", rec.Body.String())
	}
}

func TestNonStreamProxy_429Retry(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limited"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"chatcmpl-1","choices":[]}`))
	}))
	defer server.Close()

	cfg := testConfig()
	cfg.Upstream.BaseURL = server.URL
	policy := retry.NewPolicy(&cfg.Retry)
	logger := slog.Default()
	doer := testDoer(server)

	nsp := NewNonStreamProxy(doer, policy, cfg, logger)

	body := `{"model":"gpt-4","messages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	nsp.ServeHTTP(rec, req, []byte(body))

	if callCount < 2 {
		t.Fatalf("上游调用次数 = %d, 期望 >= 2", callCount)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, 期望 200", rec.Code)
	}
}

func TestNonStreamProxy_503Retry(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error":"service unavailable"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"chatcmpl-1","choices":[]}`))
	}))
	defer server.Close()

	cfg := testConfig()
	cfg.Upstream.BaseURL = server.URL
	policy := retry.NewPolicy(&cfg.Retry)
	logger := slog.Default()
	doer := testDoer(server)

	nsp := NewNonStreamProxy(doer, policy, cfg, logger)

	body := `{"model":"gpt-4","messages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	nsp.ServeHTTP(rec, req, []byte(body))

	if callCount < 2 {
		t.Fatalf("上游调用次数 = %d, 期望 >= 2", callCount)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, 期望 200", rec.Code)
	}
}

func TestNonStreamProxy_BudgetExceeded(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer server.Close()

	cfg := testConfig()
	cfg.Upstream.BaseURL = server.URL
	cfg.Retry.MaxRetryDelayBudget = 200 * time.Millisecond
	cfg.Retry.Schedule5xx = []time.Duration{50 * time.Millisecond}
	policy := retry.NewPolicy(&cfg.Retry)
	logger := slog.Default()
	doer := testDoer(server)

	nsp := NewNonStreamProxy(doer, policy, cfg, logger)

	body := `{"model":"gpt-4","messages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	nsp.ServeHTTP(rec, req, []byte(body))

	if callCount == 0 {
		t.Fatal("期望至少一次上游调用")
	}
	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, 期望 504", rec.Code)
	}
}
