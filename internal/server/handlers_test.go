package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	HealthHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("状态码 = %d, 期望 %d", rec.Code, http.StatusOK)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, 期望 \"application/json\"", ct)
	}

	var result map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("解析 JSON 失败: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("status = %q, 期望 \"ok\"", result["status"])
	}
}

func TestReadyHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	ReadyHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("状态码 = %d, 期望 %d", rec.Code, http.StatusOK)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, 期望 \"application/json\"", ct)
	}

	var result map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("解析 JSON 失败: %v", err)
	}
	if result["status"] != "ready" {
		t.Errorf("status = %q, 期望 \"ready\"", result["status"])
	}
}
