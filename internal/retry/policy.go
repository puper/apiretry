package retry

import (
	"context"
	"time"

	"github.com/puper/apiretry/internal/config"
	"github.com/puper/apiretry/internal/util"
)

type Policy struct {
	classifier          *Classifier
	backoff429          *Backoff
	backoff5xx          *Backoff
	maxRetryDelayBudget time.Duration
	maxPerRetryDelay    time.Duration
	maxAttempts         int
}

func NewPolicy(cfg *config.RetryConfig) *Policy {
	return &Policy{
		classifier:          NewClassifier(cfg.RetryStatusCodes),
		backoff429:          NewBackoff(cfg.Schedule429, cfg.JitterPercent, cfg.MaxPerRetryDelay),
		backoff5xx:          NewBackoff(cfg.Schedule5xx, cfg.JitterPercent, cfg.MaxPerRetryDelay),
		maxRetryDelayBudget: cfg.MaxRetryDelayBudget,
		maxPerRetryDelay:    cfg.MaxPerRetryDelay,
		maxAttempts:         cfg.MaxAttempts,
	}
}

func (p *Policy) Decide(input DecideInput) util.RetryDecision {
	decision := p.classifier.Classify(input)
	if !decision.ShouldRetry {
		return decision
	}

	if input.Attempt >= p.maxAttempts {
		return util.RetryDecision{
			ShouldRetry: false,
			Class:       decision.Class,
			Reason:      "max attempts exceeded",
		}
	}

	var delay time.Duration
	if input.RetryAfterHeader != "" {
		if ra := ParseRetryAfter(input.RetryAfterHeader); ra > 0 {
			delay = ra
			if p.maxPerRetryDelay > 0 && delay > p.maxPerRetryDelay {
				delay = p.maxPerRetryDelay
			}
		}
	}

	if delay == 0 {
		b := p.selectBackoff(decision.Class)
		delay = b.Delay(input.Attempt)
	}

	if p.maxRetryDelayBudget > 0 && input.ElapsedDelay+delay > p.maxRetryDelayBudget {
		return util.RetryDecision{
			ShouldRetry: false,
			Class:       decision.Class,
			Reason:      "retry delay budget exceeded",
		}
	}

	decision.Delay = delay
	return decision
}

func (p *Policy) selectBackoff(class util.RetryClass) *Backoff {
	if class == util.RetryRateLimit {
		return p.backoff429
	}
	return p.backoff5xx
}

func Sleep(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
