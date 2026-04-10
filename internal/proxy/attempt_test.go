package proxy

import (
	"testing"
	"time"
)

func TestAttemptContext_NextAttempt(t *testing.T) {
	ctx := NewAttemptContext(3, 10*time.Second)

	// 第1次尝试
	if !ctx.NextAttempt() {
		t.Fatal("第1次 NextAttempt 应返回 true")
	}
	if ctx.Attempt() != 1 {
		t.Fatalf("第1次尝试后 Attempt() = %d, 期望 1", ctx.Attempt())
	}

	// 第2次尝试
	if !ctx.NextAttempt() {
		t.Fatal("第2次 NextAttempt 应返回 true")
	}
	if ctx.Attempt() != 2 {
		t.Fatalf("第2次尝试后 Attempt() = %d, 期望 2", ctx.Attempt())
	}

	// 第3次尝试
	if !ctx.NextAttempt() {
		t.Fatal("第3次 NextAttempt 应返回 true")
	}
	if ctx.Attempt() != 3 {
		t.Fatalf("第3次尝试后 Attempt() = %d, 期望 3", ctx.Attempt())
	}

	// 超过最大次数
	if ctx.NextAttempt() {
		t.Fatal("第4次 NextAttempt 应返回 false（超过 maxAttempts=3）")
	}
}

func TestAttemptContext_RecordDelay(t *testing.T) {
	ctx := NewAttemptContext(3, 10*time.Second)

	ctx.RecordDelay(100 * time.Millisecond)
	ctx.RecordDelay(200 * time.Millisecond)

	if ctx.elapsedDelay != 300*time.Millisecond {
		t.Fatalf("elapsedDelay = %v, 期望 300ms", ctx.elapsedDelay)
	}
}

func TestAttemptContext_RemainingBudget(t *testing.T) {
	ctx := NewAttemptContext(3, 10*time.Second)

	// 初始预算应接近 10s
	remaining := ctx.RemainingBudget()
	if remaining <= 0 {
		t.Fatalf("初始剩余预算应 > 0, 实际 = %v", remaining)
	}
	if remaining > 10*time.Second {
		t.Fatalf("初始剩余预算不应超过 10s, 实际 = %v", remaining)
	}

	ctx.RecordDelay(3 * time.Second)
	remaining = ctx.RemainingBudget()
	if remaining > 7*time.Second {
		t.Fatalf("记录3s延迟后，剩余预算应约7s, 实际 = %v", remaining)
	}
}

func TestAttemptContext_RemainingBudget_Depleted(t *testing.T) {
	ctx := NewAttemptContext(3, 100*time.Millisecond)
	ctx.RecordDelay(200 * time.Millisecond)

	remaining := ctx.RemainingBudget()
	if remaining != 0 {
		t.Fatalf("预算耗尽后 RemainingBudget 应为 0, 实际 = %v", remaining)
	}
}

func TestAttemptContext_ZeroBudget(t *testing.T) {
	ctx := NewAttemptContext(2, 0)

	remaining := ctx.RemainingBudget()
	if remaining != 0 {
		t.Fatalf("零预算时 RemainingBudget 应为 0, 实际 = %v", remaining)
	}
}
