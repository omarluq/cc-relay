package ratelimit

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
