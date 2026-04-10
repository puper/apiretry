package proxy

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"log/slog"

	"github.com/puper/apiretry/internal/retry"
	"github.com/puper/apiretry/internal/util"
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

func TestNonStreamProxy_LogHasRequestContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer server.Close()

	cfg := testConfig()
	cfg.Upstream.BaseURL = server.URL
	cfg.Retry.MaxAttempts = 1
	policy := retry.NewPolicy(&cfg.Retry)

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	doer := testDoer(server)
	nsp := NewNonStreamProxy(doer, policy, cfg, logger)

	body := `{"model":"gpt-4","messages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req = req.WithContext(util.WithRequestID(req.Context(), "req-ns-1"))
	rec := httptest.NewRecorder()

	nsp.ServeHTTP(rec, req, []byte(body))

	logText := logs.String()
	if !strings.Contains(logText, `"msg":"upstream HTTP error"`) {
		t.Fatalf("未找到 upstream HTTP error 日志: %s", logText)
	}
	if !strings.Contains(logText, `"request_id":"req-ns-1"`) {
		t.Fatalf("日志缺少 request_id: %s", logText)
	}
	if !strings.Contains(logText, fmt.Sprintf(`"method":"%s"`, http.MethodPost)) {
		t.Fatalf("日志缺少 method: %s", logText)
	}
	if !strings.Contains(logText, `"path":"/v1/chat/completions"`) {
		t.Fatalf("日志缺少 path: %s", logText)
	}
}
