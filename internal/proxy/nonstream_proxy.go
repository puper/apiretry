package proxy

import (
	"io"
	"log/slog"
	"net/http"

	"github.com/puper/apiretry/internal/config"
	"github.com/puper/apiretry/internal/retry"
	"github.com/puper/apiretry/internal/upstream"
	"github.com/puper/apiretry/internal/util"
)

type NonStreamProxy struct {
	doer   upstream.Doer
	policy *retry.Policy
	cfg    *config.Config
	logger *slog.Logger
}

func NewNonStreamProxy(doer upstream.Doer, policy *retry.Policy, cfg *config.Config, logger *slog.Logger) *NonStreamProxy {
	return &NonStreamProxy{
		doer:   doer,
		policy: policy,
		cfg:    cfg,
		logger: logger,
	}
}

func (nsp *NonStreamProxy) ServeHTTP(w http.ResponseWriter, r *http.Request, bodyBytes []byte) {
	ctx := r.Context()
	reqLogger := nsp.logger.With(
		"request_id", util.RequestIDFromContext(ctx),
		"method", r.Method,
		"path", r.URL.Path,
	)
	attemptCtx := NewAttemptContext(nsp.cfg.Retry.MaxAttempts, nsp.cfg.Retry.MaxRetryDelayBudget)

	var lastErr error

	for {
		if !attemptCtx.NextAttempt() {
			reqLogger.Error("retry budget exceeded",
				"attempts", attemptCtx.Attempt(),
				"budget", nsp.cfg.Retry.MaxRetryDelayBudget,
				"last_error", lastErr,
			)
			util.WriteProxyError(w, http.StatusGatewayTimeout, lastErr.Error(), "proxy_retry_exhausted")
			return
		}

		if ctx.Err() != nil {
			return
		}

		upstreamReq, err := upstream.BuildRequest(r, &nsp.cfg.Upstream, bodyBytes)
		if err != nil {
			reqLogger.Error("build upstream request failed", "error", err)
			util.WriteProxyError(w, http.StatusBadGateway, err.Error(), "proxy_upstream_error")
			return
		}

		resp, err := nsp.doer.Do(upstreamReq)

		if err != nil {
			decision := nsp.policy.Decide(retry.DecideInput{
				Attempt:      attemptCtx.Attempt() - 1,
				Err:          err,
				ElapsedDelay: attemptCtx.elapsedDelay,
			})
			reqLogger.Info("upstream network error",
				"attempt", attemptCtx.Attempt(),
				"error", err,
				"should_retry", decision.ShouldRetry,
				"class", decision.Class,
			)
			if !decision.ShouldRetry {
				util.WriteProxyError(w, http.StatusBadGateway, err.Error(), "proxy_upstream_error")
				return
			}
			if sleepErr := retry.Sleep(ctx, decision.Delay); sleepErr != nil {
				return
			}
			attemptCtx.RecordDelay(decision.Delay)
			lastErr = err
			continue
		}

		bodyData, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()

		if readErr != nil {
			decision := nsp.policy.Decide(retry.DecideInput{
				Attempt:      attemptCtx.Attempt() - 1,
				Err:          readErr,
				ElapsedDelay: attemptCtx.elapsedDelay,
			})
			reqLogger.Info("upstream body read error",
				"attempt", attemptCtx.Attempt(),
				"error", readErr,
				"should_retry", decision.ShouldRetry,
			)
			if !decision.ShouldRetry {
				util.WriteProxyError(w, http.StatusBadGateway, readErr.Error(), "proxy_upstream_error")
				return
			}
			if sleepErr := retry.Sleep(ctx, decision.Delay); sleepErr != nil {
				return
			}
			attemptCtx.RecordDelay(decision.Delay)
			lastErr = readErr
			continue
		}

		if resp.StatusCode != http.StatusOK {
			retryAfter := resp.Header.Get("Retry-After")
			decision := nsp.policy.Decide(retry.DecideInput{
				Attempt:          attemptCtx.Attempt() - 1,
				StatusCode:       resp.StatusCode,
				RetryAfterHeader: retryAfter,
				ElapsedDelay:     attemptCtx.elapsedDelay,
			})
			reqLogger.Info("upstream HTTP error",
				"attempt", attemptCtx.Attempt(),
				"status", resp.StatusCode,
				"should_retry", decision.ShouldRetry,
				"class", decision.Class,
			)

			if !decision.ShouldRetry {
				if decision.Reason == "max attempts exceeded" || decision.Reason == "retry delay budget exceeded" {
					util.WriteProxyError(w, http.StatusGatewayTimeout,
						(&util.BudgetExceededError{Budget: nsp.cfg.Retry.MaxRetryDelayBudget, Attempts: attemptCtx.Attempt(), LastErr: &util.UpstreamError{StatusCode: resp.StatusCode, Body: string(bodyData)}}).Error(),
						"proxy_retry_exhausted")
				} else {
					util.CopyResponseHeaders(w.Header(), resp.Header)
					w.WriteHeader(resp.StatusCode)
					w.Write(bodyData)
				}
				return
			}

			if sleepErr := retry.Sleep(ctx, decision.Delay); sleepErr != nil {
				return
			}
			attemptCtx.RecordDelay(decision.Delay)
			lastErr = &util.UpstreamError{StatusCode: resp.StatusCode, Body: string(bodyData)}
			continue
		}

		util.CopyResponseHeaders(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)
		w.Write(bodyData)
		return
	}
}
