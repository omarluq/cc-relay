package ratelimit

import "time"

// NormalizeInterval exports normalizeInterval for testing.
var NormalizeInterval = normalizeInterval

// Verify NormalizeInterval has the expected type at compile time.
var _ func(time.Duration) time.Duration = NormalizeInterval

// GetRPMLimit returns the RPM limit (for testing).
func (l *TokenBucketLimiter) GetRPMLimit() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.rpmLimit
}

// GetTPMLimit returns the TPM limit (for testing).
func (l *TokenBucketLimiter) GetTPMLimit() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.tpmLimit
}
