package server

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/puper/apiretry/internal/config"
)

func TestNewRouter_LoggingHasRequestID(t *testing.T) {
	cfg := config.DefaultConfig()

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	router := NewRouter(handler, cfg, logger)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"stream":true}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Header().Get("X-Request-ID") == "" {
		t.Fatal("响应头 X-Request-ID 不应为空")
	}

	logText := logs.String()
	if !strings.Contains(logText, `"msg":"request completed"`) {
		t.Fatalf("未找到 request completed 日志: %s", logText)
	}
	if strings.Contains(logText, `"request_id":""`) {
		t.Fatalf("request_id 不应为空: %s", logText)
	}
}
