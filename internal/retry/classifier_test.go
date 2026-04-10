package retry

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/puper/apiretry/internal/util"
)

type tempNetErr struct{}

func (e *tempNetErr) Error() string   { return "temporary network error" }
func (e *tempNetErr) Timeout() bool   { return false }
func (e *tempNetErr) Temporary() bool { return true }

type nonTempNetErr struct{}

func (e *nonTempNetErr) Error() string   { return "non-temporary network error" }
func (e *nonTempNetErr) Timeout() bool   { return false }
func (e *nonTempNetErr) Temporary() bool { return false }

var _ net.Error = (*tempNetErr)(nil)
var _ net.Error = (*nonTempNetErr)(nil)

func TestClassifier_Classify(t *testing.T) {
	codes := []int{500, 502, 503, 504}
	c := NewClassifier(codes)

	tests := []struct {
		name      string
		input     DecideInput
		want      util.RetryClass
		retryable bool
	}{
		{
			name:      "429 rate limit",
			input:     DecideInput{StatusCode: 429},
			want:      util.RetryRateLimit,
			retryable: true,
		},
		{
			name:      "500 server error",
			input:     DecideInput{StatusCode: 500},
			want:      util.RetryServerError,
			retryable: true,
		},
		{
			name:      "502 server error",
			input:     DecideInput{StatusCode: 502},
			want:      util.RetryServerError,
			retryable: true,
		},
		{
			name:      "503 server error",
			input:     DecideInput{StatusCode: 503},
			want:      util.RetryServerError,
			retryable: true,
		},
		{
			name:      "504 server error",
			input:     DecideInput{StatusCode: 504},
			want:      util.RetryServerError,
			retryable: true,
		},
		{
			name:      "400 not retryable",
			input:     DecideInput{StatusCode: 400},
			want:      util.RetryNone,
			retryable: false,
		},
		{
			name:      "401 not retryable",
			input:     DecideInput{StatusCode: 401},
			want:      util.RetryNone,
			retryable: false,
		},
		{
			name:      "403 not retryable",
			input:     DecideInput{StatusCode: 403},
			want:      util.RetryNone,
			retryable: false,
		},
		{
			name:      "404 not retryable",
			input:     DecideInput{StatusCode: 404},
			want:      util.RetryNone,
			retryable: false,
		},
		{
			name:      "409 not retryable",
			input:     DecideInput{StatusCode: 409},
			want:      util.RetryNone,
			retryable: false,
		},
		{
			name:      "422 not retryable",
			input:     DecideInput{StatusCode: 422},
			want:      util.RetryNone,
			retryable: false,
		},
		{
			name:      "context canceled",
			input:     DecideInput{Err: context.Canceled},
			want:      util.RetryNone,
			retryable: false,
		},
		{
			name: "first byte timeout before first byte",
			input: DecideInput{
				Err:               &util.FirstByteTimeoutError{Timeout: 5e9},
				IsBeforeFirstByte: true,
			},
			want:      util.RetryFirstByte,
			retryable: true,
		},
		{
			name: "first byte timeout after first byte not retryable",
			input: DecideInput{
				Err:               &util.FirstByteTimeoutError{Timeout: 5e9},
				IsBeforeFirstByte: false,
			},
			want:      util.RetryNetworkError,
			retryable: true,
		},
		{
			name:      "temporary net error",
			input:     DecideInput{Err: &tempNetErr{}},
			want:      util.RetryNetworkError,
			retryable: true,
		},
		{
			name:      "non-temporary net error falls through to default",
			input:     DecideInput{Err: &nonTempNetErr{}},
			want:      util.RetryNetworkError,
			retryable: true,
		},
		{
			name:      "generic error defaults to network error",
			input:     DecideInput{Err: errors.New("something broke")},
			want:      util.RetryNetworkError,
			retryable: true,
		},
		{
			name:      "no error no status",
			input:     DecideInput{},
			want:      util.RetryNone,
			retryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.Classify(tt.input)
			if got.Class != tt.want {
				t.Errorf("Classify() Class = %v, want %v", got.Class, tt.want)
			}
			if got.ShouldRetry != tt.retryable {
				t.Errorf("Classify() ShouldRetry = %v, want %v", got.ShouldRetry, tt.retryable)
			}
		})
	}
}
