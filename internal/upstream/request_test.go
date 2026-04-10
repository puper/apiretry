package upstream

import (
	"bytes"
	"net/http"
	"net/url"
	"testing"

	"github.com/puper/apiretry/internal/config"
)

func TestBuildRequest_URLRewrite(t *testing.T) {
	cfg := &config.UpstreamConfig{
		BaseURL: "https://api.openai.com",
	}

	origReq, _ := http.NewRequest("POST", "http://localhost:8080/v1/chat/completions", bytes.NewReader([]byte(`{}`)))
	origReq.Header.Set("Content-Type", "application/json")

	newReq, err := BuildRequest(origReq, cfg, []byte(`{}`))
	if err != nil {
		t.Fatalf("BuildRequest error: %v", err)
	}

	if newReq.URL.Host != "api.openai.com" {
		t.Errorf("Host = %q, want %q", newReq.URL.Host, "api.openai.com")
	}
	if newReq.URL.Path != "/v1/chat/completions" {
		t.Errorf("Path = %q, want %q", newReq.URL.Path, "/v1/chat/completions")
	}
	if newReq.URL.Scheme != "https" {
		t.Errorf("Scheme = %q, want %q", newReq.URL.Scheme, "https")
	}
}

func TestBuildRequest_QueryParamsPreserved(t *testing.T) {
	cfg := &config.UpstreamConfig{
		BaseURL: "https://api.openai.com",
	}

	origReq, _ := http.NewRequest("GET", "http://localhost/v1/models?limit=10&sort=id", nil)

	newReq, err := BuildRequest(origReq, cfg, nil)
	if err != nil {
		t.Fatalf("BuildRequest error: %v", err)
	}

	if newReq.URL.RawQuery != "limit=10&sort=id" {
		t.Errorf("RawQuery = %q, want %q", newReq.URL.RawQuery, "limit=10&sort=id")
	}
}

func TestBuildRequest_HopByHopHeadersRemoved(t *testing.T) {
	cfg := &config.UpstreamConfig{
		BaseURL: "https://api.openai.com",
	}

	origReq, _ := http.NewRequest("POST", "http://localhost/v1/chat/completions", bytes.NewReader([]byte(`{}`)))
	origReq.Header.Set("Connection", "keep-alive")
	origReq.Header.Set("Transfer-Encoding", "chunked")
	origReq.Header.Set("X-Custom", "value")

	newReq, err := BuildRequest(origReq, cfg, []byte(`{}`))
	if err != nil {
		t.Fatalf("BuildRequest error: %v", err)
	}

	if newReq.Header.Get("Connection") != "" {
		t.Error("Connection header 应被移除")
	}
	if newReq.Header.Get("Transfer-Encoding") != "" {
		t.Error("Transfer-Encoding header 应被移除")
	}
	if newReq.Header.Get("X-Custom") != "value" {
		t.Error("X-Custom header 应被保留")
	}
}

func TestBuildRequest_BodyFromCachedBytes(t *testing.T) {
	cfg := &config.UpstreamConfig{
		BaseURL: "https://api.openai.com",
	}

	bodyContent := `{"model":"gpt-4","messages":[]}`
	origReq, _ := http.NewRequest("POST", "http://localhost/v1/chat/completions", bytes.NewReader([]byte(bodyContent)))
	origReq.Header.Set("Content-Type", "application/json")

	newReq, err := BuildRequest(origReq, cfg, []byte(bodyContent))
	if err != nil {
		t.Fatalf("BuildRequest error: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(newReq.Body)
	if buf.String() != bodyContent {
		t.Errorf("Body = %q, want %q", buf.String(), bodyContent)
	}
}

func TestBuildRequest_ContentTypePreserved(t *testing.T) {
	cfg := &config.UpstreamConfig{
		BaseURL: "https://api.openai.com",
	}

	origReq, _ := http.NewRequest("POST", "http://localhost/v1/chat/completions", bytes.NewReader([]byte(`{}`)))
	origReq.Header.Set("Content-Type", "text/plain")

	newReq, err := BuildRequest(origReq, cfg, []byte(`{}`))
	if err != nil {
		t.Fatalf("BuildRequest error: %v", err)
	}

	if newReq.Header.Get("Content-Type") != "text/plain" {
		t.Errorf("Content-Type = %q, want %q", newReq.Header.Get("Content-Type"), "text/plain")
	}
}

func TestBuildRequest_ContentTypeDefault(t *testing.T) {
	cfg := &config.UpstreamConfig{
		BaseURL: "https://api.openai.com",
	}

	origReq, _ := http.NewRequest("POST", "http://localhost/v1/chat/completions", bytes.NewReader([]byte(`{}`)))

	newReq, err := BuildRequest(origReq, cfg, []byte(`{}`))
	if err != nil {
		t.Fatalf("BuildRequest error: %v", err)
	}

	if newReq.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want %q", newReq.Header.Get("Content-Type"), "application/json")
	}
}

func TestBuildRequest_MethodPreserved(t *testing.T) {
	cfg := &config.UpstreamConfig{
		BaseURL: "https://api.openai.com",
	}

	methods := []string{"GET", "POST", "PUT", "DELETE"}
	for _, method := range methods {
		origReq, _ := http.NewRequest(method, "http://localhost/v1/models", nil)
		newReq, err := BuildRequest(origReq, cfg, nil)
		if err != nil {
			t.Fatalf("BuildRequest error: %v", err)
		}
		if newReq.Method != method {
			t.Errorf("Method = %q, want %q", newReq.Method, method)
		}
	}
}

func TestBuildRequest_AcceptDefault(t *testing.T) {
	cfg := &config.UpstreamConfig{
		BaseURL: "https://api.openai.com",
	}

	origReq, _ := http.NewRequest("POST", "http://localhost/v1/chat/completions", bytes.NewReader([]byte(`{}`)))

	newReq, err := BuildRequest(origReq, cfg, []byte(`{}`))
	if err != nil {
		t.Fatalf("BuildRequest error: %v", err)
	}

	if newReq.Header.Get("Accept") != "text/event-stream" {
		t.Errorf("Accept = %q, want %q", newReq.Header.Get("Accept"), "text/event-stream")
	}
}

func TestBuildRequest_AcceptPreserved(t *testing.T) {
	cfg := &config.UpstreamConfig{
		BaseURL: "https://api.openai.com",
	}

	origReq, _ := http.NewRequest("POST", "http://localhost/v1/chat/completions", bytes.NewReader([]byte(`{}`)))
	origReq.Header.Set("Accept", "application/json")

	newReq, err := BuildRequest(origReq, cfg, []byte(`{}`))
	if err != nil {
		t.Fatalf("BuildRequest error: %v", err)
	}

	if newReq.Header.Get("Accept") != "application/json" {
		t.Errorf("Accept = %q, want %q", newReq.Header.Get("Accept"), "application/json")
	}
}

func TestBuildRequest_ContextPreserved(t *testing.T) {
	cfg := &config.UpstreamConfig{
		BaseURL: "https://api.openai.com",
	}

	origReq, _ := http.NewRequest("POST", "http://localhost/v1/chat/completions", bytes.NewReader([]byte(`{}`)))

	newReq, err := BuildRequest(origReq, cfg, []byte(`{}`))
	if err != nil {
		t.Fatalf("BuildRequest error: %v", err)
	}

	if newReq.Context() != origReq.Context() {
		t.Error("Context 应被保留")
	}
}

func TestBuildRequest_BaseURLWithPath(t *testing.T) {
	cfg := &config.UpstreamConfig{
		BaseURL: "https://api.openai.com/proxy",
	}

	origReq, _ := http.NewRequest("POST", "http://localhost/v1/chat/completions", bytes.NewReader([]byte(`{}`)))
	origReq.Header.Set("Content-Type", "application/json")

	newReq, err := BuildRequest(origReq, cfg, []byte(`{}`))
	if err != nil {
		t.Fatalf("BuildRequest error: %v", err)
	}

	expected, _ := url.Parse("https://api.openai.com/v1/chat/completions")
	if newReq.URL.Path != expected.Path {
		t.Errorf("Path = %q, want %q", newReq.URL.Path, expected.Path)
	}
}
