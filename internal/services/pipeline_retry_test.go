package services

import (
	"testing"
	"time"
)

func TestNewRetryPolicy_Defaults(t *testing.T) {
	p := NewRetryPolicy(0, "")
	if p.MaxRetries != 0 {
		t.Errorf("expected 0 max retries, got %d", p.MaxRetries)
	}
	if p.Backoff != RetryBackoffExponential {
		t.Errorf("expected exponential backoff, got %q", p.Backoff)
	}
}

func TestNewRetryPolicy_Clamped(t *testing.T) {
	p := NewRetryPolicy(20, "fixed")
	if p.MaxRetries != 10 {
		t.Errorf("expected clamped to 10, got %d", p.MaxRetries)
	}
	if p.Backoff != RetryBackoffFixed {
		t.Errorf("expected fixed backoff, got %q", p.Backoff)
	}
}

func TestNewRetryPolicy_NegativeToZero(t *testing.T) {
	p := NewRetryPolicy(-5, "exponential")
	if p.MaxRetries != 0 {
		t.Errorf("expected 0 for negative input, got %d", p.MaxRetries)
	}
}

func TestRetryPolicy_ShouldRetry(t *testing.T) {
	tests := []struct {
		name       string
		maxRetries int
		attempt    int
		want       bool
	}{
		{"no retry configured", 0, 0, false},
		{"first attempt, 3 retries", 3, 0, true},
		{"second attempt, 3 retries", 3, 1, true},
		{"last attempt, 3 retries", 3, 2, true},
		{"exhausted, 3 retries", 3, 3, false},
		{"over max, 3 retries", 3, 5, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := NewRetryPolicy(tc.maxRetries, "exponential")
			got := p.ShouldRetry(tc.attempt)
			if got != tc.want {
				t.Errorf("ShouldRetry(%d) = %v, want %v", tc.attempt, got, tc.want)
			}
		})
	}
}

func TestRetryPolicy_Delay_Exponential(t *testing.T) {
	p := NewRetryPolicy(5, "exponential")

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 5 * time.Second},                // 5s * 2^0 = 5s
		{1, 10 * time.Second},               // 5s * 2^1 = 10s
		{2, 20 * time.Second},               // 5s * 2^2 = 20s
		{3, 40 * time.Second},               // 5s * 2^3 = 40s
		{4, 80 * time.Second},               // 5s * 2^4 = 80s
		{5, 160 * time.Second},              // 5s * 2^5 = 160s
		{10, 5 * time.Minute},               // capped at 5min
		{20, 5 * time.Minute},               // capped at 5min
	}

	for _, tc := range tests {
		got := p.Delay(tc.attempt)
		if got != tc.want {
			t.Errorf("Delay(%d) = %v, want %v", tc.attempt, got, tc.want)
		}
	}
}

func TestRetryPolicy_Delay_Fixed(t *testing.T) {
	p := NewRetryPolicy(3, "fixed")

	for attempt := 0; attempt < 5; attempt++ {
		got := p.Delay(attempt)
		if got != 5*time.Second {
			t.Errorf("Delay(%d) = %v, want 5s", attempt, got)
		}
	}
}

func TestRetryPolicy_Delay_NegativeAttempt(t *testing.T) {
	p := NewRetryPolicy(3, "exponential")
	got := p.Delay(-1)
	if got != 5*time.Second {
		t.Errorf("Delay(-1) = %v, want 5s", got)
	}
}
