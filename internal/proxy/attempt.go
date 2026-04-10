package proxy

import (
	"time"
)

// AttemptContext 追踪单次请求的重试状态，包括尝试次数和延迟预算。
type AttemptContext struct {
	attempt      int
	startTime    time.Time
	elapsedDelay time.Duration
	maxAttempts  int
	maxBudget    time.Duration
}

// NewAttemptContext 创建重试上下文。
func NewAttemptContext(maxAttempts int, maxBudget time.Duration) *AttemptContext {
	return &AttemptContext{
		maxAttempts: maxAttempts,
		maxBudget:   maxBudget,
		startTime:   time.Now(),
	}
}

// NextAttempt 递增尝试计数器，并检查是否超过最大重试次数。
// 返回 true 表示可以继续重试，false 表示已达上限。
func (a *AttemptContext) NextAttempt() bool {
	a.attempt++
	return a.attempt <= a.maxAttempts
}

// Attempt 返回当前尝试次数（从 1 开始）。
func (a *AttemptContext) Attempt() int {
	return a.attempt
}

// RecordDelay 累加已消耗的延迟时间。
func (a *AttemptContext) RecordDelay(d time.Duration) {
	a.elapsedDelay += d
}

// RemainingBudget 返回剩余可用延迟预算。
// 计算：总预算 - 已消耗延迟 - 从开始到现在的自然流逝时间。
func (a *AttemptContext) RemainingBudget() time.Duration {
	elapsed := time.Since(a.startTime)
	remaining := a.maxBudget - a.elapsedDelay - elapsed
	if remaining < 0 {
		return 0
	}
	return remaining
}
