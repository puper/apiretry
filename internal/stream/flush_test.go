package stream

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockFlusher struct {
	http.ResponseWriter
	flushed bool
}

func (m *mockFlusher) Flush() {
	m.flushed = true
}

func TestFlushWriter_Write(t *testing.T) {
	rec := httptest.NewRecorder()
	fw := NewFlushWriter(rec)

	data := []byte("hello world")
	n, err := fw.Write(data)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if n != len(data) {
		t.Errorf("Write() returned %d, want %d", n, len(data))
	}
	if !bytes.Equal(rec.Body.Bytes(), data) {
		t.Errorf("body = %q, want %q", rec.Body.String(), string(data))
	}
}

func TestFlushWriter_Flush(t *testing.T) {
	rec := httptest.NewRecorder()
	mf := &mockFlusher{ResponseWriter: rec}
	fw := NewFlushWriter(mf)

	fw.Flush()
	if !mf.flushed {
		t.Error("expected Flush to be called on underlying Flusher")
	}
}

func TestFlushWriter_WriteAndFlush(t *testing.T) {
	rec := httptest.NewRecorder()
	mf := &mockFlusher{ResponseWriter: rec}
	fw := NewFlushWriter(mf)

	_, err := fw.Write([]byte("data"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if !mf.flushed {
		t.Error("expected Flush to be called after Write")
	}
}
