package util

import (
	"context"
	"time"
)

type contextKey string

const (
	attemptKey   contextKey = "attempt"
	startTimeKey contextKey = "start_time"
	requestIDKey contextKey = "request_id"
)

func WithAttemptNumber(ctx context.Context, n int) context.Context {
	return context.WithValue(ctx, attemptKey, n)
}

func AttemptFromContext(ctx context.Context) int {
	if v, ok := ctx.Value(attemptKey).(int); ok {
		return v
	}
	return 0
}

func WithStartTime(ctx context.Context, t time.Time) context.Context {
	return context.WithValue(ctx, startTimeKey, t)
}

func StartTimeFromContext(ctx context.Context) time.Time {
	if v, ok := ctx.Value(startTimeKey).(time.Time); ok {
		return v
	}
	return time.Time{}
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}
