package proxy

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/puper/apiretry/internal/config"
	"github.com/puper/apiretry/internal/retry"
	"github.com/puper/apiretry/internal/stream"
	"github.com/puper/apiretry/internal/upstream"
	"github.com/puper/apiretry/internal/util"
)

type StreamProxy struct {
	doer   upstream.Doer
	policy *retry.Policy
	probe  stream.StreamProbe
	cfg    *config.Config
	logger *slog.Logger
}

func NewStreamProxy(doer upstream.Doer, policy *retry.Policy, probe stream.StreamProbe, cfg *config.Config, logger *slog.Logger) *StreamProxy {
	return &StreamProxy{
		doer:   doer,
		policy: policy,
		probe:  probe,
		cfg:    cfg,
		logger: logger,
	}
}

func (sp *StreamProxy) ServeHTTP(w http.ResponseWriter, r *http.Request, bodyBytes []byte) {
	ctx := r.Context()
	reqLogger := sp.logger.With(
		"request_id", util.RequestIDFromContext(ctx),
		"method", r.Method,
		"path", r.URL.Path,
	)
	attemptCtx := NewAttemptContext(sp.cfg.Retry.MaxAttempts, sp.cfg.Retry.MaxRetryDelayBudget)
	firstByteTimeout := sp.cfg.Retry.FirstByteTimeout
	chunkIdleTimeout := sp.cfg.Retry.ChunkIdleTimeout

	var lastErr error

	for {
		if !attemptCtx.NextAttempt() {
			reqLogger.Error("retry budget exceeded",
				"attempts", attemptCtx.Attempt(),
				"budget", sp.cfg.Retry.MaxRetryDelayBudget,
				"last_error", lastErr,
			)
			util.WriteProxyError(w, http.StatusGatewayTimeout, lastErr.Error(), "proxy_retry_exhausted")
			return
		}

		if ctx.Err() != nil {
			return
		}

		upstreamReq, err := upstream.BuildRequest(r, &sp.cfg.Upstream, bodyBytes)
		if err != nil {
			reqLogger.Error("build upstream request failed", "error", err)
			util.WriteProxyError(w, http.StatusBadGateway, err.Error(), "proxy_upstream_error")
			return
		}

		resp, err := sp.doer.Do(upstreamReq)

		if err != nil {
			decision := sp.policy.Decide(retry.DecideInput{
				Attempt:           attemptCtx.Attempt() - 1,
				Err:               err,
				IsBeforeFirstByte: true,
				ElapsedDelay:      attemptCtx.elapsedDelay,
			})
			reqLogger.Info("upstream network error",
				"attempt", attemptCtx.Attempt(),
				"error", err,
				"should_retry", decision.ShouldRetry,
				"class", decision.Class,
			)
			if !decision.ShouldRetry {
				if decision.Reason == "max attempts exceeded" || decision.Reason == "retry delay budget exceeded" {
					util.WriteProxyError(w, http.StatusGatewayTimeout,
						(&util.BudgetExceededError{Budget: sp.cfg.Retry.MaxRetryDelayBudget, Attempts: attemptCtx.Attempt(), LastErr: err}).Error(),
						"proxy_retry_exhausted")
				} else {
					util.WriteProxyError(w, http.StatusBadGateway, err.Error(), "proxy_upstream_error")
				}
				return
			}
			if sleepErr := retry.Sleep(ctx, decision.Delay); sleepErr != nil {
				return
			}
			attemptCtx.RecordDelay(decision.Delay)
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			bodyData, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			resp.Body.Close()

			retryAfter := resp.Header.Get("Retry-After")
			decision := sp.policy.Decide(retry.DecideInput{
				Attempt:           attemptCtx.Attempt() - 1,
				StatusCode:        resp.StatusCode,
				IsBeforeFirstByte: true,
				RetryAfterHeader:  retryAfter,
				ElapsedDelay:      attemptCtx.elapsedDelay,
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
						(&util.BudgetExceededError{Budget: sp.cfg.Retry.MaxRetryDelayBudget, Attempts: attemptCtx.Attempt(), LastErr: &util.UpstreamError{StatusCode: resp.StatusCode, Body: string(bodyData)}}).Error(),
						"proxy_retry_exhausted")
				} else {
					util.WriteProxyError(w, resp.StatusCode, string(bodyData), "proxy_upstream_error")
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

		preRead, rest, _, probeErr := sp.probe.ProbeFirstEvent(ctx, resp.Body, firstByteTimeout)
		if probeErr != nil {
			decision := sp.policy.Decide(retry.DecideInput{
				Attempt:           attemptCtx.Attempt() - 1,
				Err:               probeErr,
				IsBeforeFirstByte: true,
				ElapsedDelay:      attemptCtx.elapsedDelay,
			})
			reqLogger.Info("first byte probe failed",
				"attempt", attemptCtx.Attempt(),
				"error", probeErr,
				"should_retry", decision.ShouldRetry,
				"class", decision.Class,
			)
			if !decision.ShouldRetry {
				if _, ok := probeErr.(*util.FirstByteTimeoutError); ok {
					util.WriteProxyError(w, http.StatusGatewayTimeout, probeErr.Error(), "proxy_first_byte_timeout")
				} else if probeErr == ctx.Err() {
					return
				} else {
					util.WriteProxyError(w, http.StatusBadGateway, probeErr.Error(), "proxy_upstream_error")
				}
				return
			}
			if ctx.Err() != nil {
				return
			}
			if sleepErr := retry.Sleep(ctx, decision.Delay); sleepErr != nil {
				return
			}
			attemptCtx.RecordDelay(decision.Delay)
			lastErr = probeErr
			continue
		}

		// 首包闸门通过 — 提交响应给客户端，此后不可重试
		util.CopyResponseHeaders(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)
		w.Write(preRead)

		fw := stream.NewFlushWriter(w)
		fw.Flush()

		copyErr := sp.copyStreamWithIdleTimeout(ctx, fw, rest, chunkIdleTimeout)
		rest.Close()
		fw.Flush()

		if copyErr != nil && !errors.Is(copyErr, io.EOF) {
			reqLogger.Error("stream copy error",
				"error", copyErr,
				"attempt", attemptCtx.Attempt(),
			)
		}
		return
	}
}

// copyStreamWithIdleTimeout 逐块复制流数据，每次读操作受 chunkIdleTimeout 限制。
// 使用 time.AfterFunc 实现空闲超时：一旦超时，取消读取 context。
func (sp *StreamProxy) copyStreamWithIdleTimeout(ctx context.Context, fw *stream.FlushWriter, rest io.ReadCloser, chunkIdleTimeout time.Duration) error {
	if chunkIdleTimeout <= 0 {
		_, err := io.Copy(fw, rest)
		return err
	}

	buf := make([]byte, 32*1024)
	for {
		type readResult struct {
			n   int
			err error
		}
		ch := make(chan readResult, 1)

		timer := time.AfterFunc(chunkIdleTimeout, func() {
			// 空闲超时到达，尝试中断读取（依赖底层连接关闭）
		})

		go func() {
			n, err := rest.Read(buf)
			ch <- readResult{n: n, err: err}
		}()

		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case result := <-ch:
			timer.Stop()
			if result.n > 0 {
				if _, writeErr := fw.Write(buf[:result.n]); writeErr != nil {
					return writeErr
				}
			}
			if result.err != nil {
				return result.err
			}
		}
	}
}
