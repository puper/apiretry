package config

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Upstream UpstreamConfig `yaml:"upstream"`
	Retry    RetryConfig    `yaml:"retry"`
	Limits   LimitsConfig   `yaml:"limits"`
	Logging  LoggingConfig  `yaml:"logging"`
}

type ServerConfig struct {
	Addr         string        `yaml:"addr"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
}

type UpstreamConfig struct {
	BaseURL               string        `yaml:"base_url"`
	Timeout               time.Duration `yaml:"timeout"`
	ResponseHeaderTimeout time.Duration `yaml:"response_header_timeout"`
	TLSHandshakeTimeout   time.Duration `yaml:"tls_handshake_timeout"`
	IdleConnTimeout       time.Duration `yaml:"idle_conn_timeout"`
	MaxIdleConns          int           `yaml:"max_idle_conns"`
	MaxIdleConnsPerHost   int           `yaml:"max_idle_conns_per_host"`
	ForceAttemptHTTP2     bool          `yaml:"force_attempt_http2"`
}

type RetryConfig struct {
	MaxAttempts         int             `yaml:"max_attempts"`
	MaxRetryDelayBudget time.Duration   `yaml:"max_retry_delay_budget"`
	FirstByteTimeout    time.Duration   `yaml:"first_byte_timeout"`
	ChunkIdleTimeout    time.Duration   `yaml:"chunk_idle_timeout"`
	MaxPerRetryDelay    time.Duration   `yaml:"max_per_retry_delay"`
	RetryStatusCodes    []int           `yaml:"retry_status_codes"`
	Schedule429         []time.Duration `yaml:"schedule_429"`
	Schedule5xx         []time.Duration `yaml:"schedule_5xx"`
	JitterPercent       float64         `yaml:"jitter_percent"`
}

type LimitsConfig struct {
	MaxRequestBodyBytes int64 `yaml:"max_request_body_bytes"`
}

type LoggingConfig struct {
	Level string `yaml:"level"`
	JSON  bool   `yaml:"json"`
}

func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Addr:         ":8080",
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 0,
			IdleTimeout:  60 * time.Second,
		},
		Upstream: UpstreamConfig{
			Timeout:               120 * time.Second,
			ResponseHeaderTimeout: 20 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			IdleConnTimeout:       90 * time.Second,
			MaxIdleConns:          200,
			MaxIdleConnsPerHost:   100,
			ForceAttemptHTTP2:     true,
		},
		Retry: RetryConfig{
			MaxAttempts:         5,
			MaxRetryDelayBudget: 10 * time.Second,
			FirstByteTimeout:    8 * time.Second,
			ChunkIdleTimeout:    30 * time.Second,
			MaxPerRetryDelay:    3 * time.Second,
			RetryStatusCodes:    []int{429, 500, 502, 503, 504},
			Schedule429:         []time.Duration{200 * time.Millisecond, 500 * time.Millisecond, 1 * time.Second, 2 * time.Second, 3 * time.Second},
			Schedule5xx:         []time.Duration{100 * time.Millisecond, 300 * time.Millisecond, 700 * time.Millisecond, 1500 * time.Millisecond, 2500 * time.Millisecond},
			JitterPercent:       0.15,
		},
		Limits: LimitsConfig{
			MaxRequestBodyBytes: 10 * 1024 * 1024,
		},
		Logging: LoggingConfig{
			Level: "info",
			JSON:  true,
		},
	}
}

var envVarRe = regexp.MustCompile(`\$\{([^}]+)\}`)

func expandEnvVars(input string) string {
	return envVarRe.ReplaceAllStringFunc(input, func(match string) string {
		varName := envVarRe.FindStringSubmatch(match)[1]
		val := os.Getenv(varName)
		if val == "" {
			return match
		}
		return val
	})
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	expanded := expandEnvVars(string(data))
	cfg := DefaultConfig()
	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}
