package keypool

import (
	"crypto/rand"
	"math/big"
	"time"
)

// randIntn returns a non-negative integer in [0, n). If n <= 0 it returns 0.
// It uses crypto/rand to produce a secure random value and falls back to a
// time-based source if crypto randomness fails.
func randIntn(n int) int {
	if n <= 0 {
		return 0
	}
	maxVal := big.NewInt(int64(n))
	if v, err := rand.Int(rand.Reader, maxVal); err == nil {
		return int(v.Int64())
	}
	return int(time.Now().UnixNano() % int64(n))
}