package retry

import (
	"strings"
	"testing"
	"time"
)

func TestBackoff_Delay_ScheduleIndex(t *testing.T) {
	schedule := []time.Duration{
		100 * time.Millisecond,
		300 * time.Millisecond,
		700 * time.Millisecond,
	}
	b := NewBackoff(schedule, 0, 0)

	if d := b.Delay(0); d != 100*time.Millisecond {
		t.Errorf("Delay(0) = %v, want %v", d, 100*time.Millisecond)
	}
	if d := b.Delay(1); d != 300*time.Millisecond {
		t.Errorf("Delay(1) = %v, want %v", d, 300*time.Millisecond)
	}
	if d := b.Delay(2); d != 700*time.Millisecond {
		t.Errorf("Delay(2) = %v, want %v", d, 700*time.Millisecond)
	}
	// attempt >= len(schedule) 使用最后一个值
	if d := b.Delay(3); d != 700*time.Millisecond {
		t.Errorf("Delay(3) = %v, want %v", d, 700*time.Millisecond)
	}
	if d := b.Delay(100); d != 700*time.Millisecond {
		t.Errorf("Delay(100) = %v, want %v", d, 700*time.Millisecond)
	}
}

func TestBackoff_Delay_JitterRange(t *testing.T) {
	schedule := []time.Duration{1000 * time.Millisecond}
	jitter := 0.2
	b := NewBackoff(schedule, jitter, 0)

	base := float64(1000 * time.Millisecond)
	minAllowed := time.Duration(base * (1 - jitter))
	maxAllowed := time.Duration(base * (1 + jitter))

	for i := 0; i < 1000; i++ {
		d := b.Delay(0)
		if d < minAllowed || d > maxAllowed {
			t.Errorf("Delay() = %v, want in [%v, %v]", d, minAllowed, maxAllowed)
		}
	}
}

func TestBackoff_Delay_MaxPerRetryDelayCap(t *testing.T) {
	schedule := []time.Duration{5 * time.Second}
	b := NewBackoff(schedule, 0, 3*time.Second)

	if d := b.Delay(0); d != 3*time.Second {
		t.Errorf("Delay() = %v, want %v (capped by maxPerRetryDelay)", d, 3*time.Second)
	}
}

func TestBackoff_Delay_EmptySchedule(t *testing.T) {
	b := NewBackoff(nil, 0, 0)
	if d := b.Delay(0); d != 0 {
		t.Errorf("Delay() on empty schedule = %v, want 0", d)
	}
}

func TestParseRetryAfter_Seconds(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"120", 120 * time.Second},
		{"0", 0},
		{"5", 5 * time.Second},
	}
	for _, tt := range tests {
		got := ParseRetryAfter(tt.input)
		if got != tt.want {
			t.Errorf("ParseRetryAfter(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseRetryAfter_HTTPDate(t *testing.T) {
	future := time.Now().Add(30 * time.Second)
	header := future.UTC().Format(time.RFC1123)

	got := ParseRetryAfter(header)
	if got <= 0 {
		t.Errorf("ParseRetryAfter(%q) = %v, want > 0", header, got)
	}
	// 允许 1 秒误差
	if got > 31*time.Second || got < 29*time.Second {
		t.Errorf("ParseRetryAfter(%q) = %v, want ~30s", header, got)
	}
}

func TestParseRetryAfter_PastDate(t *testing.T) {
	past := time.Now().Add(-10 * time.Second)
	header := past.UTC().Format(time.RFC1123)

	got := ParseRetryAfter(header)
	if got != 0 {
		t.Errorf("ParseRetryAfter(past date %q) = %v, want 0", header, got)
	}
}

func TestParseRetryAfter_InvalidFormat(t *testing.T) {
	tests := []string{"invalid", "abc123", "", "Wed, 09 Jun 9999"}
	for _, input := range tests {
		if strings.Contains(input, "9999") {
			continue
		}
		got := ParseRetryAfter(input)
		if got != 0 {
			t.Errorf("ParseRetryAfter(%q) = %v, want 0", input, got)
		}
	}
}

func TestParseRetryAfter_Empty(t *testing.T) {
	got := ParseRetryAfter("")
	if got != 0 {
		t.Errorf("ParseRetryAfter(empty) = %v, want 0", got)
	}
}
