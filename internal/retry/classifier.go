package retry

import (
	"context"
	"net"
	"time"

	"github.com/puper/apiretry/internal/util"
)

type DecideInput struct {
	Attempt           int
	StatusCode        int
	Err               error
	IsBeforeFirstByte bool
	RetryAfterHeader  string
	ElapsedDelay      time.Duration
}

type Classifier struct {
	retryableStatusCodes []int
}

func NewClassifier(retryableCodes []int) *Classifier {
	return &Classifier{retryableStatusCodes: retryableCodes}
}

func (c *Classifier) Classify(input DecideInput) util.RetryDecision {
	if input.StatusCode > 0 {
		if input.StatusCode == 429 {
			return util.RetryDecision{
				ShouldRetry: true,
				Class:       util.RetryRateLimit,
				Reason:      "status 429 rate limit",
			}
		}

		for _, code := range c.retryableStatusCodes {
			if input.StatusCode == code {
				return util.RetryDecision{
					ShouldRetry: true,
					Class:       util.RetryServerError,
					Reason:      "retryable server error",
				}
			}
		}

		return util.RetryDecision{
			ShouldRetry: false,
			Class:       util.RetryNone,
			Reason:      "non-retryable status",
		}
	}

	if input.Err != nil {
		if input.Err == context.Canceled {
			return util.RetryDecision{
				ShouldRetry: false,
				Class:       util.RetryNone,
				Reason:      "context canceled",
			}
		}

		if _, ok := input.Err.(*util.FirstByteTimeoutError); ok && input.IsBeforeFirstByte {
			return util.RetryDecision{
				ShouldRetry: true,
				Class:       util.RetryFirstByte,
				Reason:      "first byte timeout",
			}
		}

		if netErr, ok := input.Err.(net.Error); ok && netErr.Temporary() {
			return util.RetryDecision{
				ShouldRetry: true,
				Class:       util.RetryNetworkError,
				Reason:      "temporary network error",
			}
		}
	}

	if input.Err != nil {
		return util.RetryDecision{
			ShouldRetry: true,
			Class:       util.RetryNetworkError,
			Reason:      "unknown error treated as network error",
		}
	}

	return util.RetryDecision{
		ShouldRetry: false,
		Class:       util.RetryNone,
		Reason:      "no error and no status code",
	}
}
