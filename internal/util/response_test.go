package util

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteProxyError(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteProxyError(rec, http.StatusBadGateway, "upstream error", "proxy_upstream_error")

	if rec.Code != http.StatusBadGateway {
		t.Errorf("状态码 = %d, 期望 %d", rec.Code, http.StatusBadGateway)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, 期望 \"application/json\"", ct)
	}

	var resp ProxyErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("解析 JSON 失败: %v", err)
	}
	if resp.Error.Message != "upstream error" {
		t.Errorf("message = %q, 期望 \"upstream error\"", resp.Error.Message)
	}
	if resp.Error.Type != "proxy_error" {
		t.Errorf("type = %q, 期望 \"proxy_error\"", resp.Error.Type)
	}
	if resp.Error.Code != "proxy_upstream_error" {
		t.Errorf("code = %q, 期望 \"proxy_upstream_error\"", resp.Error.Code)
	}
}
