package util

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadAndCacheBody_NormalBody(t *testing.T) {
	body := `{"model":"gpt-4","messages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))

	data, err := ReadAndCacheBody(req, 1024)
	if err != nil {
		t.Fatalf("ReadAndCacheBody 返回错误: %v", err)
	}
	if string(data) != body {
		t.Errorf("data = %q, 期望 %q", string(data), body)
	}

	data2, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("二次读取 body 失败: %v", err)
	}
	if string(data2) != body {
		t.Errorf("二次读取 data = %q, 期望 %q", string(data2), body)
	}
}

func TestReadAndCacheBody_NilBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Body = nil

	data, err := ReadAndCacheBody(req, 1024)
	if err != nil {
		t.Fatalf("nil body 不应返回错误, got: %v", err)
	}
	if data != nil {
		t.Errorf("nil body 应返回 nil data, got: %v", data)
	}
}

func TestReadAndCacheBody_TooLarge(t *testing.T) {
	bigBody := strings.Repeat("x", 200)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(bigBody))

	data, err := ReadAndCacheBody(req, 100)
	if err == nil {
		t.Fatal("期望返回 BodyTooLargeError, got nil")
	}
	if data != nil {
		t.Errorf("超限时 data 应为 nil, got: %v", data)
	}
	btle, ok := err.(*BodyTooLargeError)
	if !ok {
		t.Fatalf("错误类型应为 *BodyTooLargeError, got: %T", err)
	}
	if btle.MaxBytes != 100 {
		t.Errorf("MaxBytes = %d, 期望 100", btle.MaxBytes)
	}
}

func TestDrainBody_Normal(t *testing.T) {
	body := io.NopCloser(strings.NewReader("some data to drain"))
	DrainBody(body, 1024)
}

func TestDrainBody_Nil(t *testing.T) {
	DrainBody(nil, 1024)
}
