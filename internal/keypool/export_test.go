package keypool

import (
	"time"

	"github.com/omarluq/cc-relay/internal/ratelimit"
)

// ContainsKeyID checks if a key ID exists in the pool (for testing).
func (p *KeyPool) ContainsKeyID(keyID string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	_, ok := p.keyMap[keyID]
	return ok
}

// GetProvider returns the pool's provider name (for testing).
func (p *KeyPool) GetProvider() string {
	return p.provider
}

// GetKeys returns the pool's keys slice (for testing).
func (p *KeyPool) GetKeys() []*KeyMetadata {
	return p.keys
}

// GetKeyMap returns the pool's keyMap (for testing).
func (p *KeyPool) GetKeyMap() map[string]*KeyMetadata {
	return p.keyMap
}

// GetLimiters returns the pool's limiters (for testing).
func (p *KeyPool) GetLimiters() map[string]ratelimit.RateLimiter {
	return p.limiters
}

// GetSelector returns the pool's selector (for testing).
func (p *KeyPool) GetSelector() KeySelector {
	return p.selector
}

// GetLimitersLen returns the number of limiters (for testing).
func (p *KeyPool) GetLimitersLen() int {
	return len(p.limiters)
}

// GetRPMLimit returns the RPM limit under lock (for testing).
func (k *KeyMetadata) GetRPMLimit() int {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.RPMLimit
}

// GetITPMLimit returns the ITPM limit under lock (for testing).
func (k *KeyMetadata) GetITPMLimit() int {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.ITPMLimit
}

// GetOTPMLimit returns the OTPM limit under lock (for testing).
func (k *KeyMetadata) GetOTPMLimit() int {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.OTPMLimit
}

// GetRPMRemaining returns the RPM remaining under lock (for testing).
func (k *KeyMetadata) GetRPMRemaining() int {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.RPMRemaining
}

// GetITPMRemaining returns the ITPM remaining under lock (for testing).
func (k *KeyMetadata) GetITPMRemaining() int {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.ITPMRemaining
}

// GetOTPMRemaining returns the OTPM remaining under lock (for testing).
func (k *KeyMetadata) GetOTPMRemaining() int {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.OTPMRemaining
}

// GetRPMResetAt returns the RPM reset time under lock (for testing).
func (k *KeyMetadata) GetRPMResetAt() time.Time {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.RPMResetAt
}

// GetCooldownUntil returns the cooldown time under lock (for testing).
func (k *KeyMetadata) GetCooldownUntil() time.Time {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.CooldownUntil
}

// TestRLock locks the key mutex for reading (for testing).
func (k *KeyMetadata) TestRLock() {
	k.mu.RLock()
}

// TestRUnlock unlocks the key mutex (for testing).
func (k *KeyMetadata) TestRUnlock() {
	k.mu.RUnlock()
}

// TestLock locks the key mutex for writing (for testing).
func (k *KeyMetadata) TestLock() {
	k.mu.Lock()
}

// TestUnlock unlocks the key mutex for writing (for testing).
func (k *KeyMetadata) TestUnlock() {
	k.mu.Unlock()
}
