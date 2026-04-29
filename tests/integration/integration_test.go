package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/puper/apiretry/internal/config"
	"github.com/puper/apiretry/internal/proxy"
	"github.com/puper/apiretry/internal/retry"
	"github.com/puper/apiretry/internal/server"
	"github.com/puper/apiretry/internal/stream"
)

func integrationTestConfig(upstreamURL string) *config.Config {
	cfg := config.DefaultConfig()
	cfg.Upstream.BaseURL = upstreamURL
	cfg.Retry.MaxAttempts = 3
	cfg.Retry.MaxRetryDelayBudget = 2 * time.Second
	cfg.Retry.FirstByteTimeout = 100 * time.Millisecond
	cfg.Retry.ChunkIdleTimeout = 200 * time.Millisecond
	cfg.Retry.MaxPerRetryDelay = 500 * time.Millisecond
	cfg.Retry.Schedule429 = []time.Duration{50 * time.Millisecond, 100 * time.Millisecond, 200 * time.Millisecond}
	cfg.Retry.Schedule5xx = []time.Duration{50 * time.Millisecond, 100 * time.Millisecond, 200 * time.Millisecond}
	cfg.Retry.JitterPercent = 0
	cfg.Retry.RetryStatusCodes = []int{429, 500, 502, 503, 504}
	cfg.Limits.MaxRequestBodyBytes = 10 * 1024 * 1024
	cfg.Server.ReadTimeout = 5 * time.Second
	cfg.Server.WriteTimeout = 0
	cfg.Server.IdleTimeout = 60 * time.Second
	return cfg
}

type doerFromClient struct {
	client *http.Client
}

func (d *doerFromClient) Do(req *http.Request) (*http.Response, error) {
	return d.client.Do(req)
}

func setupProxy(t *testing.T, upstreamHandler http.HandlerFunc) *httptest.Server {
	t.Helper()
	upstreamSrv := httptest.NewServer(upstreamHandler)
	t.Cleanup(func() { upstreamSrv.Close() })

	cfg := integrationTestConfig(upstreamSrv.URL)
	httpClient := upstreamSrv.Client()
	doer := &doerFromClient{client: httpClient}
	policy := retry.NewPolicy(&cfg.Retry)
	probe := &stream.DefaultProbe{}
	logger := slog.Default()

	handler := proxy.NewHandler(doer, policy, probe, cfg, logger)
	router := server.NewRouter(handler, cfg, logger)

	proxySrv := httptest.NewServer(router)
	t.Cleanup(func() { proxySrv.Close() })
	return proxySrv
}

func setupProxyWithConfig(t *testing.T, cfg *config.Config, upstreamHandler http.HandlerFunc) *httptest.Server {
	t.Helper()
	upstreamSrv := httptest.NewServer(upstreamHandler)
	t.Cleanup(func() { upstreamSrv.Close() })

	cfg.Upstream.BaseURL = upstreamSrv.URL
	httpClient := upstreamSrv.Client()
	doer := &doerFromClient{client: httpClient}
	policy := retry.NewPolicy(&cfg.Retry)
	probe := &stream.DefaultProbe{}
	logger := slog.Default()

	handler := proxy.NewHandler(doer, policy, probe, cfg, logger)
	router := server.NewRouter(handler, cfg, logger)

	proxySrv := httptest.NewServer(router)
	t.Cleanup(func() { proxySrv.Close() })
	return proxySrv
}

func TestIntegration_NonStream_HappyPath(t *testing.T) {
	proxySrv := setupProxy(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"chatcmpl-1","choices":[]}`))
	})

	body := `{"model":"gpt-4","messages":[]}`
	resp, err := http.Post(proxySrv.URL+"/v1/chat/completions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应体失败: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("解析 JSON 失败: %v", err)
	}
	if result["id"] != "chatcmpl-1" {
		t.Errorf("id = %v, 期望 \"chatcmpl-1\"", result["id"])
	}
}

func TestIntegration_NonStream_429ThenSuccess(t *testing.T) {
	var callCount int32

	proxySrv := setupProxy(t, func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&callCount, 1)
		if count == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limited"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"chatcmpl-2","choices":[]}`))
	})

	body := `{"model":"gpt-4","messages":[]}`
	resp, err := http.Post(proxySrv.URL+"/v1/chat/completions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200 (调用次数: %d)", resp.StatusCode, atomic.LoadInt32(&callCount))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应体失败: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("解析 JSON 失败: %v", err)
	}
	if result["id"] != "chatcmpl-2" {
		t.Errorf("id = %v, 期望 \"chatcmpl-2\"", result["id"])
	}
}

func TestIntegration_NonStream_AllFail(t *testing.T) {
	var callCount int32

	cfg := config.DefaultConfig()
	cfg.Retry.MaxAttempts = 2
	cfg.Retry.MaxRetryDelayBudget = 500 * time.Millisecond
	cfg.Retry.Schedule5xx = []time.Duration{10 * time.Millisecond}
	cfg.Retry.JitterPercent = 0
	cfg.Retry.RetryStatusCodes = []int{429, 500, 502, 503, 504}
	cfg.Limits.MaxRequestBodyBytes = 10 * 1024 * 1024

	proxySrv := setupProxyWithConfig(t, cfg, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal"}`))
	})

	body := `{"model":"gpt-4","messages":[]}`
	resp, err := http.Post(proxySrv.URL+"/v1/chat/completions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusGatewayTimeout {
		t.Fatalf("状态码 = %d, 期望 504 (调用次数: %d)", resp.StatusCode, atomic.LoadInt32(&callCount))
	}
}

func TestIntegration_Stream_HappyPath(t *testing.T) {
	proxySrv := setupProxy(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("upstream 不支持 Flusher")
		}

		events := []string{
			`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"content":"Hello"}}]}`,
			`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"content":" world"}}]}`,
			`data: [DONE]`,
		}
		for _, evt := range events {
			fmt.Fprintf(w, "%s\n\n", evt)
			flusher.Flush()
		}
	})

	body := `{"model":"gpt-4","stream":true,"messages":[]}`
	req, err := http.NewRequest(http.MethodPost, proxySrv.URL+"/v1/chat/completions", strings.NewReader(body))
	if err != nil {
		t.Fatalf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取流失败: %v", err)
	}

	s := string(data)
	if !strings.Contains(s, `"id":"chatcmpl-1"`) {
		t.Errorf("流响应应包含 chatcmpl-1, got: %s", s)
	}
	if !strings.Contains(s, "[DONE]") {
		t.Errorf("流响应应包含 [DONE], got: %s", s)
	}
}

func TestIntegration_Stream_429ThenSuccess(t *testing.T) {
	var callCount int32

	proxySrv := setupProxy(t, func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&callCount, 1)
		if count == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limited"}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("upstream 不支持 Flusher")
		}

		events := []string{
			`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"content":"Hi"}}]}`,
			`data: [DONE]`,
		}
		for _, evt := range events {
			fmt.Fprintf(w, "%s\n\n", evt)
			flusher.Flush()
		}
	})

	body := `{"model":"gpt-4","stream":true,"messages":[]}`
	req, err := http.NewRequest(http.MethodPost, proxySrv.URL+"/v1/chat/completions", strings.NewReader(body))
	if err != nil {
		t.Fatalf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取流失败: %v", err)
	}
	if !strings.Contains(string(data), "[DONE]") {
		t.Errorf("流响应应包含 [DONE]")
	}
}

func TestIntegration_Stream_FirstByteTimeoutThenSuccess(t *testing.T) {
	var callCount int32

	cfg := config.DefaultConfig()
	cfg.Retry.MaxAttempts = 3
	cfg.Retry.MaxRetryDelayBudget = 2 * time.Second
	cfg.Retry.FirstByteTimeout = 100 * time.Millisecond
	cfg.Retry.ChunkIdleTimeout = 200 * time.Millisecond
	cfg.Retry.MaxPerRetryDelay = 500 * time.Millisecond
	cfg.Retry.Schedule429 = []time.Duration{10 * time.Millisecond}
	cfg.Retry.Schedule5xx = []time.Duration{10 * time.Millisecond}
	cfg.Retry.JitterPercent = 0
	cfg.Retry.RetryStatusCodes = []int{429, 500, 502, 503, 504}
	cfg.Limits.MaxRequestBodyBytes = 10 * 1024 * 1024

	proxySrv := setupProxyWithConfig(t, cfg, func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&callCount, 1)
		if count == 1 {
			// 首包超时：连接后不发送数据
			time.Sleep(500 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		events := []string{
			`data: {"id":"chatcmpl-to","choices":[{"index":0,"delta":{"content":"OK"}}]}`,
			`data: [DONE]`,
		}
		for _, evt := range events {
			fmt.Fprintf(w, "%s\n\n", evt)
			flusher.Flush()
		}
	})

	body := `{"model":"gpt-4","stream":true,"messages":[]}`
	req, err := http.NewRequest(http.MethodPost, proxySrv.URL+"/v1/chat/completions", strings.NewReader(body))
	if err != nil {
		t.Fatalf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("状态码 = %d, 期望 200, body: %s", resp.StatusCode, string(b))
	}
}

func TestIntegration_HealthEndpoint(t *testing.T) {
	proxySrv := setupProxy(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	resp, err := http.Get(proxySrv.URL + "/health")
	if err != nil {
		t.Fatalf("请求 /health 失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("/health 状态码 = %d, 期望 200", resp.StatusCode)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("解析 health 响应失败: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("health status = %q, 期望 \"ok\"", result["status"])
	}
}

func TestIntegration_PassThroughV1Path(t *testing.T) {
	var receivedPath string
	proxySrv := setupProxy(t, func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"path":"%s"}`, r.URL.Path)
	})

	tests := []struct {
		name string
		path string
	}{
		{"v1_chat_completions", "/v1/chat/completions"},
		{"v1_models", "/v1/models"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receivedPath = ""
			resp, err := http.Post(proxySrv.URL+tt.path, "application/json", bytes.NewReader([]byte(`{}`)))
			if err != nil {
				t.Fatalf("请求 %s 失败: %v", tt.path, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("%s 状态码 = %d, 期望 200", tt.path, resp.StatusCode)
			}
			if receivedPath != tt.path {
				t.Errorf("收到路径 = %q, 期望 %q", receivedPath, tt.path)
			}
		})
	}
}

func TestIntegration_RejectNonV1PathBeforeUpstream(t *testing.T) {
	var upstreamCalls int32
	proxySrv := setupProxy(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamCalls, 1)
		w.WriteHeader(http.StatusTooManyRequests)
	})

	resp, err := http.Get(proxySrv.URL + "/pinfo.php")
	if err != nil {
		t.Fatalf("请求探测路径失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("探测路径状态码 = %d, 期望 %d", resp.StatusCode, http.StatusNotFound)
	}
	if got := atomic.LoadInt32(&upstreamCalls); got != 0 {
		t.Fatalf("探测路径不应访问上游，避免上游限流响应进入重试循环；实际访问次数 = %d", got)
	}
}
