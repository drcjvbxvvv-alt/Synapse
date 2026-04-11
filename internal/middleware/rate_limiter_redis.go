package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shaia/Synapse/pkg/logger"
)

// RedisRateLimiter is a cross-pod implementation backed by Redis.
//
// Key schema:
//
//	rl:fail:{key}  — INCR counter; expires after loginWindow (sliding window)
//	rl:lock:{key}  — presence indicates lock; expires after loginLockDuration
//
// Fail-open: when Redis is unavailable, requests are allowed through and a
// warning is logged. This prevents Redis outages from becoming login outages.
type RedisRateLimiter struct {
	client *redis.Client
}

// NewRedisRateLimiter returns a RedisRateLimiter using the given client.
func NewRedisRateLimiter(client *redis.Client) *RedisRateLimiter {
	return &RedisRateLimiter{client: client}
}

// Compile-time interface check.
var _ RateLimiter = (*RedisRateLimiter)(nil)

func (r *RedisRateLimiter) failKey(key string) string {
	return fmt.Sprintf("rl:fail:%s", key)
}

func (r *RedisRateLimiter) lockKey(key string) string {
	return fmt.Sprintf("rl:lock:%s", key)
}

func (r *RedisRateLimiter) IsLocked(key string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	exists, err := r.client.Exists(ctx, r.lockKey(key)).Result()
	if err != nil {
		logger.Warn("redis rate limiter: IsLocked failed, allowing request (fail-open)",
			"error", err, "key", key)
		return false
	}
	return exists > 0
}

func (r *RedisRateLimiter) RecordFailure(key string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Atomically increment the failure counter and reset its TTL.
	pipe := r.client.Pipeline()
	failKey := r.failKey(key)
	incr := pipe.Incr(ctx, failKey)
	pipe.Expire(ctx, failKey, loginWindow)
	if _, err := pipe.Exec(ctx); err != nil {
		logger.Warn("redis rate limiter: RecordFailure pipeline failed",
			"error", err, "key", key)
		return
	}

	// If threshold exceeded, set the lock key.
	if incr.Val() >= int64(loginMaxAttempts) {
		lockCtx, lockCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer lockCancel()
		if err := r.client.Set(lockCtx, r.lockKey(key), "1", loginLockDuration).Err(); err != nil {
			logger.Warn("redis rate limiter: set lock key failed",
				"error", err, "key", key)
		}
	}
}

func (r *RedisRateLimiter) Reset(key string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := r.client.Del(ctx, r.failKey(key), r.lockKey(key)).Err(); err != nil {
		logger.Warn("redis rate limiter: Reset failed",
			"error", err, "key", key)
	}
}
