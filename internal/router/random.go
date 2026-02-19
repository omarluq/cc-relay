package router

import (
	"crypto/rand"
	"math/big"
	"time"
)

func randIntn(upperBound int) int {
	if upperBound <= 0 {
		return 0
	}
	bigInt := big.NewInt(int64(upperBound))
	if v, err := rand.Int(rand.Reader, bigInt); err == nil {
		return int(v.Int64())
	}
	return int(time.Now().UnixNano() % int64(upperBound))
}
