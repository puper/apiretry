package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_ValidConfig(t *testing.T) {
	content := `
server:
  addr: ":9090"
  read_timeout: 30s
  write_timeout: 60s
  idle_timeout: 120s

upstream:
  base_url: "https://api.example.com"
  timeout: 60s
  response_header_timeout: 10s

retry:
  max_attempts: 3
  first_byte_timeout: 5s
  schedule_429:
    - 100ms
    - 200ms
  schedule_5xx:
    - 50ms
    - 100ms

limits:
  max_request_body_bytes: 5242880

logging:
  level: debug
  json: false
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("写入临时配置文件失败: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load 返回错误: %v", err)
	}

	// 验证默认值被覆盖
	if cfg.Server.Addr != ":9090" {
		t.Errorf("Server.Addr = %q, 期望 \":9090\"", cfg.Server.Addr)
	}
	if cfg.Server.ReadTimeout != 30*time.Second {
		t.Errorf("Server.ReadTimeout = %v, 期望 30s", cfg.Server.ReadTimeout)
	}
	if cfg.Server.WriteTimeout != 60*time.Second {
		t.Errorf("Server.WriteTimeout = %v, 期望 60s", cfg.Server.WriteTimeout)
	}
	if cfg.Server.IdleTimeout != 120*time.Second {
		t.Errorf("Server.IdleTimeout = %v, 期望 120s", cfg.Server.IdleTimeout)
	}
	if cfg.Upstream.BaseURL != "https://api.example.com" {
		t.Errorf("Upstream.BaseURL = %q, 期望 \"https://api.example.com\"", cfg.Upstream.BaseURL)
	}
	if cfg.Retry.MaxAttempts != 3 {
		t.Errorf("Retry.MaxAttempts = %d, 期望 3", cfg.Retry.MaxAttempts)
	}
	if cfg.Retry.FirstByteTimeout != 5*time.Second {
		t.Errorf("Retry.FirstByteTimeout = %v, 期望 5s", cfg.Retry.FirstByteTimeout)
	}
	if cfg.Limits.MaxRequestBodyBytes != 5242880 {
		t.Errorf("Limits.MaxRequestBodyBytes = %d, 期望 5242880", cfg.Limits.MaxRequestBodyBytes)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Logging.Level = %q, 期望 \"debug\"", cfg.Logging.Level)
	}
	if cfg.Logging.JSON != false {
		t.Errorf("Logging.JSON = %v, 期望 false", cfg.Logging.JSON)
	}
	if len(cfg.Retry.Schedule429) != 2 {
		t.Fatalf("Retry.Schedule429 长度 = %d, 期望 2", len(cfg.Retry.Schedule429))
	}
	if cfg.Retry.Schedule429[0] != 100*time.Millisecond {
		t.Errorf("Schedule429[0] = %v, 期望 100ms", cfg.Retry.Schedule429[0])
	}
}

func TestLoad_DefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Server.Addr != ":8080" {
		t.Errorf("默认 Server.Addr = %q, 期望 \":8080\"", cfg.Server.Addr)
	}
	if cfg.Server.ReadTimeout != 15*time.Second {
		t.Errorf("默认 Server.ReadTimeout = %v, 期望 15s", cfg.Server.ReadTimeout)
	}
	if cfg.Server.IdleTimeout != 60*time.Second {
		t.Errorf("默认 Server.IdleTimeout = %v, 期望 60s", cfg.Server.IdleTimeout)
	}
	if cfg.Upstream.Timeout != 120*time.Second {
		t.Errorf("默认 Upstream.Timeout = %v, 期望 120s", cfg.Upstream.Timeout)
	}
	if cfg.Retry.MaxAttempts != 5 {
		t.Errorf("默认 Retry.MaxAttempts = %d, 期望 5", cfg.Retry.MaxAttempts)
	}
	if cfg.Retry.MaxRetryDelayBudget != 10*time.Second {
		t.Errorf("默认 Retry.MaxRetryDelayBudget = %v, 期望 10s", cfg.Retry.MaxRetryDelayBudget)
	}
	if cfg.Retry.FirstByteTimeout != 8*time.Second {
		t.Errorf("默认 Retry.FirstByteTimeout = %v, 期望 8s", cfg.Retry.FirstByteTimeout)
	}
	if cfg.Limits.MaxRequestBodyBytes != 10*1024*1024 {
		t.Errorf("默认 Limits.MaxRequestBodyBytes = %d, 期望 10485760", cfg.Limits.MaxRequestBodyBytes)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("默认 Logging.Level = %q, 期望 \"info\"", cfg.Logging.Level)
	}
	if cfg.Logging.JSON != true {
		t.Errorf("默认 Logging.JSON = %v, 期望 true", cfg.Logging.JSON)
	}
	if len(cfg.Retry.RetryStatusCodes) != 5 {
		t.Errorf("默认 Retry.RetryStatusCodes 长度 = %d, 期望 5", len(cfg.Retry.RetryStatusCodes))
	}
}

func TestExpandEnvVars(t *testing.T) {
	// 设置环境变量
	os.Setenv("TEST_CONFIG_VAR", "hello_world")
	defer os.Unsetenv("TEST_CONFIG_VAR")

	tests := []struct {
		name     string
		input    string
		want     string
		setEnv   map[string]string
		cleanEnv []string
	}{
		{
			name:   "替换单个变量",
			input:  "base_url: ${TEST_CONFIG_VAR}",
			want:   "base_url: hello_world",
			setEnv: map[string]string{"TEST_CONFIG_VAR": "hello_world"},
		},
		{
			name:  "未知变量保持原样",
			input: "key: ${NONEXISTENT_VAR_12345}",
			want:  "key: ${NONEXISTENT_VAR_12345}",
		},
		{
			name:   "多个变量替换",
			input:  "url: ${TEST_CONFIG_VAR}/path",
			want:   "url: hello_world/path",
			setEnv: map[string]string{"TEST_CONFIG_VAR": "hello_world"},
		},
		{
			name:  "HOME 环境变量",
			input: "path: ${HOME}/config",
			want:  "path: " + os.Getenv("HOME") + "/config",
		},
		{
			name:  "无变量纯文本",
			input: "level: info",
			want:  "level: info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.setEnv {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}
			got := expandEnvVars(tt.input)
			if got != tt.want {
				t.Errorf("expandEnvVars(%q) = %q, 期望 %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("期望返回错误，但得到了 nil")
	}
}

func TestDurationParsing(t *testing.T) {
	tests := []struct {
		yaml  string
		field string
		want  time.Duration
	}{
		{"read_timeout: 10s\n", "10s", 10 * time.Second},
		{"read_timeout: 200ms\n", "200ms", 200 * time.Millisecond},
		{"read_timeout: 1m\n", "1m", 1 * time.Minute},
		{"read_timeout: 500ms\n", "500ms", 500 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			content := "server:\n  " + tt.yaml + "  addr: \":8080\"\n"
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				t.Fatalf("写入临时配置文件失败: %v", err)
			}

			cfg, err := Load(path)
			if err != nil {
				t.Fatalf("Load 返回错误: %v", err)
			}
			if cfg.Server.ReadTimeout != tt.want {
				t.Errorf("ReadTimeout = %v, 期望 %v", cfg.Server.ReadTimeout, tt.want)
			}
		})
	}
}
