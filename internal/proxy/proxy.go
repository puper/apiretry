package proxy

import (
	"log/slog"
	"net/http"

	"github.com/puper/apiretry/internal/config"
	"github.com/puper/apiretry/internal/retry"
	"github.com/puper/apiretry/internal/stream"
	"github.com/puper/apiretry/internal/upstream"
	"github.com/puper/apiretry/internal/util"
)

type Handler struct {
	streamProxy    *StreamProxy
	nonStreamProxy *NonStreamProxy
	cfg            *config.Config
	logger         *slog.Logger
}

func NewHandler(doer upstream.Doer, policy *retry.Policy, probe stream.StreamProbe, cfg *config.Config, logger *slog.Logger) *Handler {
	return &Handler{
		streamProxy:    NewStreamProxy(doer, policy, probe, cfg, logger),
		nonStreamProxy: NewNonStreamProxy(doer, policy, cfg, logger),
		cfg:            cfg,
		logger:         logger,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := util.ReadAndCacheBody(r, h.cfg.Limits.MaxRequestBodyBytes)
	if err != nil {
		if _, ok := err.(*util.BodyTooLargeError); ok {
			util.WriteProxyError(w, http.StatusRequestEntityTooLarge, "request body too large", "proxy_body_too_large")
			return
		}
		util.WriteProxyError(w, http.StatusBadRequest, err.Error(), "proxy_body_read_error")
		return
	}

	if stream.IsStreamRequest(r, bodyBytes) {
		h.streamProxy.ServeHTTP(w, r, bodyBytes)
	} else {
		h.nonStreamProxy.ServeHTTP(w, r, bodyBytes)
	}
}
