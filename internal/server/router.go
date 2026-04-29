package server

import (
	"log/slog"
	"net/http"

	"github.com/puper/apiretry/internal/config"
)

func NewRouter(handler http.Handler, cfg *config.Config, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", HealthHandler)
	mux.HandleFunc("/ready", ReadyHandler)
	// 只代理 OpenAI 兼容 API 路径，避免互联网探测请求命中上游后被 429/5xx 策略反复重试。
	mux.Handle("/v1/", handler)

	var h http.Handler = mux
	h = LoggingMiddleware(logger, h)
	h = RequestIDMiddleware(h)
	h = BodySizeLimitMiddleware(cfg.Limits.MaxRequestBodyBytes, h)

	return h
}
