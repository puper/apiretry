package upstream

import (
	"net/http"
	"testing"
	"time"
)

func TestExtractInfo(t *testing.T) {
	start := time.Now()
	// 模拟一段耗时
	duration := 50 * time.Millisecond

	resp := &http.Response{
		StatusCode: 200,
	}

	info := ExtractInfo(resp, start.Add(-duration))

	if info.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", info.StatusCode)
	}
	if info.Duration < duration {
		t.Errorf("Duration = %v, want >= %v", info.Duration, duration)
	}
}

func TestExtractInfo_ErrorStatus(t *testing.T) {
	start := time.Now()

	resp := &http.Response{
		StatusCode: 429,
	}

	info := ExtractInfo(resp, start)

	if info.StatusCode != 429 {
		t.Errorf("StatusCode = %d, want 429", info.StatusCode)
	}
}
