package router

import (
	"crypto/rand"
	"math/big"
	"time"
)

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
