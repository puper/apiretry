package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/puper/apiretry/internal/util"
)

func TestRequestIDMiddleware_GenerateAndForward(t *testing.T) {
	t.Run("生成新ID", func(t *testing.T) {
		called := false
		var receivedID string
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			receivedID = util.RequestIDFromContext(r.Context())
		})

		handler := RequestIDMiddleware(next)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if !called {
			t.Fatal("next handler 未被调用")
		}
		if receivedID == "" {
			t.Fatal("生成的 X-Request-ID 不应为空")
		}
		if got := rec.Header().Get("X-Request-ID"); got == "" {
			t.Fatal("响应应包含 X-Request-ID header")
		}
		if got := rec.Header().Get("X-Request-ID"); got != receivedID {
			t.Errorf("响应 X-Request-ID = %q, 期望 %q", got, receivedID)
		}
	})

	t.Run("转发已有ID", func(t *testing.T) {
		var receivedID string
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedID = r.Header.Get("X-Request-ID")
		})

		handler := RequestIDMiddleware(next)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Request-ID", "custom-id-123")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if receivedID != "custom-id-123" {
			t.Errorf("转发 ID = %q, 期望 \"custom-id-123\"", receivedID)
		}
		if got := rec.Header().Get("X-Request-ID"); got != "custom-id-123" {
			t.Errorf("响应 X-Request-ID = %q, 期望 \"custom-id-123\"", got)
		}
	})
}

func TestBodySizeLimitMiddleware_Reject(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	handler := BodySizeLimitMiddleware(100, next)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("x"))
	req.ContentLength = 200
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if called {
		t.Fatal("body 超限时 next handler 不应被调用")
	}
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("状态码 = %d, 期望 %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestBodySizeLimitMiddleware_Allow(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	handler := BodySizeLimitMiddleware(1000, next)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("hello"))
	req.ContentLength = 5
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("body 未超限时 next handler 应被调用")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("状态码 = %d, 期望 %d", rec.Code, http.StatusOK)
	}
}

func TestLoggingMiddleware(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	})

	logger := slog.Default()
	handler := LoggingMiddleware(logger, next)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("next handler 应被调用")
	}
	if rec.Code != http.StatusCreated {
		t.Errorf("状态码 = %d, 期望 %d", rec.Code, http.StatusCreated)
	}
}

func TestResponseWriter_WriteWithoutWriteHeader(t *testing.T) {
	rw := &responseWriter{ResponseWriter: httptest.NewRecorder(), statusCode: 200}

	data := []byte("hello")
	n, err := rw.Write(data)
	if err != nil {
		t.Fatalf("Write 返回错误: %v", err)
	}
	if n != len(data) {
		t.Errorf("Write 返回 n = %d, 期望 %d", n, len(data))
	}
	if rw.statusCode != http.StatusOK {
		t.Errorf("statusCode = %d, 期望 %d (隐式 200)", rw.statusCode, http.StatusOK)
	}
	if !rw.written {
		t.Fatal("Write 应标记 written = true")
	}
}

func TestResponseWriter_DoubleWriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: 200}

	rw.WriteHeader(http.StatusNotFound)
	rw.WriteHeader(http.StatusOK)

	if rw.statusCode != http.StatusNotFound {
		t.Errorf("双重 WriteHeader 后 statusCode = %d, 期望 %d (第一次值)", rw.statusCode, http.StatusNotFound)
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("实际响应码 = %d, 期望 %d", rec.Code, http.StatusNotFound)
	}
}
