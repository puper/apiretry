package util

import (
	"fmt"
	"time"
)

// RetryClass 表示重试分类
type RetryClass string

const (
	RetryNone         RetryClass = "none"
	RetryRateLimit    RetryClass = "rate_limit"
	RetryServerError  RetryClass = "server_error"
	RetryNetworkError RetryClass = "network_error"
	RetryFirstByte    RetryClass = "first_byte"
)

// RetryDecision 表示重试决策结果
type RetryDecision struct {
	ShouldRetry bool
	Class       RetryClass
	Reason      string
	Delay       time.Duration
}

// Retryable 是一个接口，标识错误是否可重试
type Retryable interface {
	IsRetryable() bool
}

// UpstreamError 表示上游返回的错误
type UpstreamError struct {
	StatusCode int
	Body       string
}

func (e *UpstreamError) Error() string {
	return fmt.Sprintf("upstream error: status=%d, body=%s", e.StatusCode, e.Body)
}

func (e *UpstreamError) IsRetryable() bool {
	return IsRetryableHTTPStatus(e.StatusCode)
}

// FirstByteTimeoutError 表示首包超时错误
type FirstByteTimeoutError struct {
	Timeout time.Duration
}

func (e *FirstByteTimeoutError) Error() string {
	return fmt.Sprintf("first byte timeout after %v", e.Timeout)
}

func (e *FirstByteTimeoutError) IsRetryable() bool {
	return true
}

// BudgetExceededError 表示重试预算耗尽
type BudgetExceededError struct {
	Budget   time.Duration
	Attempts int
	LastErr  error
}

func (e *BudgetExceededError) Error() string {
	return fmt.Sprintf("retry budget exceeded: budget=%v, attempts=%d, last_error=%v", e.Budget, e.Attempts, e.LastErr)
}

func (e *BudgetExceededError) Unwrap() error {
	return e.LastErr
}

func (e *BudgetExceededError) IsRetryable() bool {
	return false
}

// StreamAbortedError 表示流在中途断开（首包后不可重试）
type StreamAbortedError struct {
	Reason string
}

func (e *StreamAbortedError) Error() string {
	return fmt.Sprintf("stream aborted: %s", e.Reason)
}

func (e *StreamAbortedError) IsRetryable() bool {
	return false
}

// 判断 HTTP 状态码是否可重试
func IsRetryableHTTPStatus(code int) bool {
	switch code {
	case 429, 500, 502, 503, 504:
		return true
	default:
		return false
	}
}
