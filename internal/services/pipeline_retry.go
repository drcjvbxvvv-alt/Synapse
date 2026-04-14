package services

import (
	"math"
	"time"
)

// ---------------------------------------------------------------------------
// Step Retry — exponential / fixed backoff
//
// 設計原則（CICD_ARCHITECTURE §7.9）：
//   - 每個 Step 可個別設定 maxRetries（0 = 不重試）
//   - 支援 exponential（預設）或 fixed backoff 策略
//   - 基礎延遲 5s，最大延遲 5min，exponential 以 2^attempt 計算
//   - RetryCount 記錄在 StepRun 上，供前端顯示
// ---------------------------------------------------------------------------

const (
	// RetryBackoffExponential 指數退避（預設）
	RetryBackoffExponential = "exponential"
	// RetryBackoffFixed 固定間隔退避
	RetryBackoffFixed = "fixed"

	// retryBaseDelay 基礎延遲
	retryBaseDelay = 5 * time.Second
	// retryMaxDelay 最大延遲
	retryMaxDelay = 5 * time.Minute
)

// RetryPolicy 從 StepDef 建立的重試策略。
type RetryPolicy struct {
	MaxRetries int
	Backoff    string // "exponential" or "fixed"
}

// NewRetryPolicy 從 StepDef 建立 RetryPolicy。
func NewRetryPolicy(maxRetries int, backoff string) RetryPolicy {
	if maxRetries < 0 {
		maxRetries = 0
	}
	if maxRetries > 10 {
		maxRetries = 10 // 上限防止無限重試
	}
	if backoff != RetryBackoffFixed {
		backoff = RetryBackoffExponential // 預設
	}
	return RetryPolicy{
		MaxRetries: maxRetries,
		Backoff:    backoff,
	}
}

// ShouldRetry 判斷是否應重試。
func (p RetryPolicy) ShouldRetry(currentAttempt int) bool {
	return p.MaxRetries > 0 && currentAttempt < p.MaxRetries
}

// Delay 計算第 attempt 次重試的等待時間（0-indexed）。
func (p RetryPolicy) Delay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	switch p.Backoff {
	case RetryBackoffFixed:
		return retryBaseDelay
	default: // exponential
		// 5s * 2^attempt → 5s, 10s, 20s, 40s, ...
		delay := time.Duration(float64(retryBaseDelay) * math.Pow(2, float64(attempt)))
		if delay > retryMaxDelay {
			delay = retryMaxDelay
		}
		return delay
	}
}
