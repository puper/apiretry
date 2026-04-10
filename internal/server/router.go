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
	mux.Handle("/", handler)

	var h http.Handler = mux
	h = RequestIDMiddleware(h)
	h = LoggingMiddleware(logger, h)
	h = BodySizeLimitMiddleware(cfg.Limits.MaxRequestBodyBytes, h)

	return h
}
