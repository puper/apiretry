package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/puper/apiretry/internal/retry"
	"github.com/puper/apiretry/internal/stream"
	"github.com/puper/apiretry/internal/util"
)

type mockProbe struct {
	probeFn func(ctx context.Context, body io.ReadCloser, timeout time.Duration) (preRead []byte, rest io.ReadCloser, event *stream.SSEEvent, err error)
}

func (m *mockProbe) ProbeFirstEvent(ctx context.Context, body io.ReadCloser, timeout time.Duration) (preRead []byte, rest io.ReadCloser, event *stream.SSEEvent, err error) {
	return m.probeFn(ctx, body, timeout)
}

func sseEventData(id int) string {
	return fmt.Sprintf("data: {\"id\":\"chatcmpl-%d\",\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n", id)
}

func TestStreamProxy_HappyPath(t *testing.T) {
	sseBody := "data: {\"id\":\"chatcmpl-1\",\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\ndata: {\"id\":\"chatcmpl-2\",\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\ndata: [DONE]\n\n"

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sseBody))
	}))
	defer server.Close()

	cfg := testConfig()
	cfg.Upstream.BaseURL = server.URL
	policy := retry.NewPolicy(&cfg.Retry)
	logger := slog.Default()

	probe := &mockProbe{probeFn: func(ctx context.Context, body io.ReadCloser, timeout time.Duration) ([]byte, io.ReadCloser, *stream.SSEEvent, error) {
		all, _ := io.ReadAll(body)
		body.Close()
		preRead := all[:len(all)/2]
		rest := io.NopCloser(strings.NewReader(string(all[len(all)/2:])))
		return preRead, rest, &stream.SSEEvent{Data: `{"id":"chatcmpl-1"}`}, nil
	}}

	doer := testDoer(server)
	sp := NewStreamProxy(doer, policy, probe, cfg, logger)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"stream":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	sp.ServeHTTP(rec, req, []byte(`{"stream":true}`))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, 期望 200, body=%s", rec.Code, rec.Body.String())
	}
	if callCount != 1 {
		t.Fatalf("上游调用次数 = %d, 期望 1", callCount)
	}
	if rec.Body.String() != sseBody {
		t.Fatalf("响应体不匹配: got %q", rec.Body.String())
	}
}

func TestStreamProxy_EOFNotLoggedAsError(t *testing.T) {
	sseBody := "data: {\"id\":\"chatcmpl-1\",\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\ndata: [DONE]\n\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sseBody))
	}))
	defer server.Close()

	cfg := testConfig()
	cfg.Upstream.BaseURL = server.URL
	policy := retry.NewPolicy(&cfg.Retry)
	probe := &stream.DefaultProbe{}

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	doer := testDoer(server)
	sp := NewStreamProxy(doer, policy, probe, cfg, logger)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"stream":true}`))
	req.Header.Set("Accept", "text/event-stream")
	rec := httptest.NewRecorder()

	sp.ServeHTTP(rec, req, []byte(`{"stream":true}`))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, 期望 200", rec.Code)
	}
	if strings.Contains(logs.String(), `"msg":"stream copy error"`) {
		t.Fatalf("正常 EOF 不应记录 stream copy error: %s", logs.String())
	}
}

func TestStreamProxy_LogHasRequestContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer server.Close()

	cfg := testConfig()
	cfg.Upstream.BaseURL = server.URL
	cfg.Retry.MaxAttempts = 1
	policy := retry.NewPolicy(&cfg.Retry)
	probe := &stream.DefaultProbe{}

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	doer := testDoer(server)
	sp := NewStreamProxy(doer, policy, probe, cfg, logger)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"stream":true}`))
	req = req.WithContext(util.WithRequestID(req.Context(), "req-s-1"))
	rec := httptest.NewRecorder()

	sp.ServeHTTP(rec, req, []byte(`{"stream":true}`))

	logText := logs.String()
	if !strings.Contains(logText, `"msg":"upstream HTTP error"`) {
		t.Fatalf("未找到 upstream HTTP error 日志: %s", logText)
	}
	if !strings.Contains(logText, `"request_id":"req-s-1"`) {
		t.Fatalf("日志缺少 request_id: %s", logText)
	}
	if !strings.Contains(logText, fmt.Sprintf(`"method":"%s"`, http.MethodPost)) {
		t.Fatalf("日志缺少 method: %s", logText)
	}
	if !strings.Contains(logText, `"path":"/v1/chat/completions"`) {
		t.Fatalf("日志缺少 path: %s", logText)
	}
}

func TestStreamProxy_FirstByteTimeoutRetry(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			time.Sleep(2 * time.Second)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {\"id\":\"chatcmpl-1\",\"choices\":[]}\n\n"))
	}))
	defer server.Close()

	cfg := testConfig()
	cfg.Upstream.BaseURL = server.URL
	cfg.Retry.FirstByteTimeout = 100 * time.Millisecond
	policy := retry.NewPolicy(&cfg.Retry)
	logger := slog.Default()

	probe := &mockProbe{probeFn: func(ctx context.Context, body io.ReadCloser, timeout time.Duration) ([]byte, io.ReadCloser, *stream.SSEEvent, error) {
		callCount++
		return nil, nil, nil, &util.FirstByteTimeoutError{Timeout: timeout}
	}}

	doer := testDoer(server)
	sp := NewStreamProxy(doer, policy, probe, cfg, logger)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"stream":true}`))
	rec := httptest.NewRecorder()

	sp.ServeHTTP(rec, req, []byte(`{"stream":true}`))

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, 期望 504", rec.Code)
	}
}

func TestStreamProxy_HTTP429Retry(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limited"}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {\"id\":\"chatcmpl-1\",\"choices\":[]}\n\n"))
	}))
	defer server.Close()

	cfg := testConfig()
	cfg.Upstream.BaseURL = server.URL
	policy := retry.NewPolicy(&cfg.Retry)
	logger := slog.Default()
	probe := &stream.DefaultProbe{}

	doer := testDoer(server)
	sp := NewStreamProxy(doer, policy, probe, cfg, logger)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"stream":true}`))
	req.Header.Set("Accept", "text/event-stream")
	rec := httptest.NewRecorder()

	sp.ServeHTTP(rec, req, []byte(`{"stream":true}`))

	if callCount < 2 {
		t.Fatalf("上游调用次数 = %d, 期望 >= 2", callCount)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, 期望 200", rec.Code)
	}
}

func TestStreamProxy_HTTP500Retry(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"internal server error"}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {\"id\":\"chatcmpl-1\",\"choices\":[]}\n\n"))
	}))
	defer server.Close()

	cfg := testConfig()
	cfg.Upstream.BaseURL = server.URL
	policy := retry.NewPolicy(&cfg.Retry)
	logger := slog.Default()
	probe := &stream.DefaultProbe{}

	doer := testDoer(server)
	sp := NewStreamProxy(doer, policy, probe, cfg, logger)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"stream":true}`))
	req.Header.Set("Accept", "text/event-stream")
	rec := httptest.NewRecorder()

	sp.ServeHTTP(rec, req, []byte(`{"stream":true}`))

	if callCount < 2 {
		t.Fatalf("上游调用次数 = %d, 期望 >= 2", callCount)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, 期望 200", rec.Code)
	}
}

func TestStreamProxy_BudgetExceeded(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"server error"}`))
	}))
	defer server.Close()

	cfg := testConfig()
	cfg.Upstream.BaseURL = server.URL
	cfg.Retry.MaxRetryDelayBudget = 200 * time.Millisecond
	cfg.Retry.Schedule5xx = []time.Duration{100 * time.Millisecond}
	policy := retry.NewPolicy(&cfg.Retry)
	logger := slog.Default()
	probe := &stream.DefaultProbe{}

	doer := testDoer(server)
	sp := NewStreamProxy(doer, policy, probe, cfg, logger)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"stream":true}`))
	rec := httptest.NewRecorder()

	sp.ServeHTTP(rec, req, []byte(`{"stream":true}`))

	if callCount == 0 {
		t.Fatal("期望至少一次上游调用")
	}
	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, 期望 504", rec.Code)
	}
}

func TestStreamProxy_ClientDisconnect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: test\n\n"))
	}))
	defer server.Close()

	cfg := testConfig()
	cfg.Upstream.BaseURL = server.URL
	policy := retry.NewPolicy(&cfg.Retry)
	logger := slog.Default()
	probe := &stream.DefaultProbe{}

	doer := testDoer(server)
	sp := NewStreamProxy(doer, policy, probe, cfg, logger)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"stream":true}`))
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	sp.ServeHTTP(rec, req, []byte(`{"stream":true}`))
}

func TestStreamProxy_DataIntegrity(t *testing.T) {
	var events []string
	for i := 0; i < 10; i++ {
		events = append(events, fmt.Sprintf("data: {\"id\":\"chatcmpl-%d\",\"choices\":[{\"delta\":{\"content\":\"chunk%d\"}}]}\n\n", i, i))
	}
	events = append(events, "data: [DONE]\n\n")
	fullBody := strings.Join(events, "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fullBody))
	}))
	defer server.Close()

	cfg := testConfig()
	cfg.Upstream.BaseURL = server.URL
	policy := retry.NewPolicy(&cfg.Retry)
	logger := slog.Default()

	probe := &mockProbe{probeFn: func(ctx context.Context, body io.ReadCloser, timeout time.Duration) ([]byte, io.ReadCloser, *stream.SSEEvent, error) {
		all, _ := io.ReadAll(body)
		body.Close()
		return all, io.NopCloser(strings.NewReader("")), &stream.SSEEvent{Data: `{"id":"chatcmpl-0"}`}, nil
	}}

	doer := testDoer(server)
	sp := NewStreamProxy(doer, policy, probe, cfg, logger)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"stream":true}`))
	req.Header.Set("Accept", "text/event-stream")
	rec := httptest.NewRecorder()

	sp.ServeHTTP(rec, req, []byte(`{"stream":true}`))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, 期望 200", rec.Code)
	}

	received := rec.Body.String()
	for i := 0; i < 10; i++ {
		expected := fmt.Sprintf("\"content\":\"chunk%d\"", i)
		if !strings.Contains(received, expected) {
			t.Fatalf("响应体缺少 chunk%d 的数据", i)
		}
	}
}
