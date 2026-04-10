package retry

import (
	"fmt"
	"math/rand"
	"time"
)

type Backoff struct {
	schedule         []time.Duration
	jitterPercent    float64
	maxPerRetryDelay time.Duration
}

func NewBackoff(schedule []time.Duration, jitterPercent float64, maxPerRetryDelay time.Duration) *Backoff {
	return &Backoff{
		schedule:         schedule,
		jitterPercent:    jitterPercent,
		maxPerRetryDelay: maxPerRetryDelay,
	}
}

func (b *Backoff) Delay(attempt int) time.Duration {
	if len(b.schedule) == 0 {
		return 0
	}

	idx := attempt
	if idx >= len(b.schedule) {
		idx = len(b.schedule) - 1
	}

	delay := b.schedule[idx]

	if b.jitterPercent > 0 {
		jitter := 1 + (rand.Float64()*2-1)*b.jitterPercent
		delay = time.Duration(float64(delay) * jitter)
	}

	if b.maxPerRetryDelay > 0 && delay > b.maxPerRetryDelay {
		delay = b.maxPerRetryDelay
	}

	return delay
}

func ParseRetryAfter(header string) time.Duration {
	if header == "" {
		return 0
	}

	if sec, err := parseRetryAfterSeconds(header); err == nil && sec > 0 {
		return sec
	}

	if d, err := parseRetryAfterDate(header); err == nil && d > 0 {
		return d
	}

	return 0
}

func parseRetryAfterSeconds(s string) (time.Duration, error) {
	var sec int
	if _, err := fmt.Sscanf(s, "%d", &sec); err != nil {
		return 0, err
	}
	return time.Duration(sec) * time.Second, nil
}

func parseRetryAfterDate(s string) (time.Duration, error) {
	formats := []string{
		time.RFC1123,
		time.RFC850,
		time.ANSIC,
	}

	now := time.Now()
	for _, layout := range formats {
		if t, err := time.Parse(layout, s); err == nil {
			d := t.Sub(now)
			if d > 0 {
				return d, nil
			}
			return 0, nil
		}
	}
	return 0, nil
}
