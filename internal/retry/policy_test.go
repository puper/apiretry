package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/puper/apiretry/internal/config"
	"github.com/puper/apiretry/internal/util"
)

func testConfig() *config.RetryConfig {
	return &config.RetryConfig{
		MaxAttempts:         5,
		MaxRetryDelayBudget: 10 * time.Second,
		FirstByteTimeout:    8 * time.Second,
		MaxPerRetryDelay:    3 * time.Second,
		RetryStatusCodes:    []int{429, 500, 502, 503, 504},
		Schedule429: []time.Duration{
			200 * time.Millisecond,
			500 * time.Millisecond,
			1 * time.Second,
			2 * time.Second,
			3 * time.Second,
		},
		Schedule5xx: []time.Duration{
			100 * time.Millisecond,
			300 * time.Millisecond,
			700 * time.Millisecond,
			1500 * time.Millisecond,
			2500 * time.Millisecond,
		},
		JitterPercent: 0,
	}
}

func TestPolicy_Decide_429UsesSchedule429(t *testing.T) {
	p := NewPolicy(testConfig())
	decision := p.Decide(DecideInput{
		Attempt:    0,
		StatusCode: 429,
	})
	if !decision.ShouldRetry {
		t.Fatal("expected ShouldRetry=true")
	}
	if decision.Class != util.RetryRateLimit {
		t.Errorf("Class = %v, want %v", decision.Class, util.RetryRateLimit)
	}
	if decision.Delay != 200*time.Millisecond {
		t.Errorf("Delay = %v, want %v", decision.Delay, 200*time.Millisecond)
	}
}

func TestPolicy_Decide_5xxUsesSchedule5xx(t *testing.T) {
	p := NewPolicy(testConfig())
	decision := p.Decide(DecideInput{
		Attempt:    1,
		StatusCode: 500,
	})
	if !decision.ShouldRetry {
		t.Fatal("expected ShouldRetry=true")
	}
	if decision.Class != util.RetryServerError {
		t.Errorf("Class = %v, want %v", decision.Class, util.RetryServerError)
	}
	if decision.Delay != 300*time.Millisecond {
		t.Errorf("Delay = %v, want %v", decision.Delay, 300*time.Millisecond)
	}
}

func TestPolicy_Decide_BudgetExceeded(t *testing.T) {
	p := NewPolicy(testConfig())
	decision := p.Decide(DecideInput{
		Attempt:      0,
		StatusCode:   500,
		ElapsedDelay: 10 * time.Second,
	})
	if decision.ShouldRetry {
		t.Fatal("expected ShouldRetry=false (budget exceeded)")
	}
}

func TestPolicy_Decide_MaxAttemptsExceeded(t *testing.T) {
	p := NewPolicy(testConfig())
	decision := p.Decide(DecideInput{
		Attempt:    5,
		StatusCode: 500,
	})
	if decision.ShouldRetry {
		t.Fatal("expected ShouldRetry=false (max attempts)")
	}
}

func TestPolicy_Decide_NonRetryableStatus(t *testing.T) {
	p := NewPolicy(testConfig())
	decision := p.Decide(DecideInput{
		Attempt:    0,
		StatusCode: 400,
	})
	if decision.ShouldRetry {
		t.Fatal("expected ShouldRetry=false for 400")
	}
}

func TestPolicy_Decide_RetryAfterOverrides(t *testing.T) {
	p := NewPolicy(testConfig())
	decision := p.Decide(DecideInput{
		Attempt:          0,
		StatusCode:       429,
		RetryAfterHeader: "2",
	})
	if !decision.ShouldRetry {
		t.Fatal("expected ShouldRetry=true")
	}
	if decision.Delay != 2*time.Second {
		t.Errorf("Delay = %v, want %v (from Retry-After)", decision.Delay, 2*time.Second)
	}
}

func TestPolicy_Decide_RetryAfterCappedByMaxPerRetryDelay(t *testing.T) {
	p := NewPolicy(testConfig())
	decision := p.Decide(DecideInput{
		Attempt:          0,
		StatusCode:       429,
		RetryAfterHeader: "10",
	})
	if !decision.ShouldRetry {
		t.Fatal("expected ShouldRetry=true")
	}
	if decision.Delay != 3*time.Second {
		t.Errorf("Delay = %v, want %v (capped by maxPerRetryDelay)", decision.Delay, 3*time.Second)
	}
}

func TestPolicy_Decide_ContextCanceled(t *testing.T) {
	p := NewPolicy(testConfig())
	decision := p.Decide(DecideInput{
		Attempt: 0,
		Err:     context.Canceled,
	})
	if decision.ShouldRetry {
		t.Fatal("expected ShouldRetry=false for context.Canceled")
	}
}

func TestSleep_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := Sleep(ctx, 5*time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Sleep() error = %v, want context.Canceled", err)
	}
}

func TestSleep_Normal(t *testing.T) {
	ctx := context.Background()
	start := time.Now()
	err := Sleep(ctx, 50*time.Millisecond)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Sleep() error = %v, want nil", err)
	}
	if elapsed < 40*time.Millisecond {
		t.Errorf("Sleep() elapsed = %v, want >= ~50ms", elapsed)
	}
}
