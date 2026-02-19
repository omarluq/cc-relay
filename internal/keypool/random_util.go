package keypool

import (
	"crypto/rand"
	"math/big"
	"time"
)

func randIntn(upperBound int) int {
	if upperBound <= 0 {
		return 0
	}
	maxVal := big.NewInt(int64(upperBound))
	if v, err := rand.Int(rand.Reader, maxVal); err == nil {
		return int(v.Int64())
	}
	return int(time.Now().UnixNano() % int64(upperBound))
}
