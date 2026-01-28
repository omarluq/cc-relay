package di

import (
	"github.com/samber/do/v2"

	"github.com/omarluq/cc-relay/internal/proxy"
)

// SignatureCacheService wraps the thinking signature cache for DI.
type SignatureCacheService struct {
	Cache *proxy.SignatureCache
}

// NewSignatureCache creates the thinking signature cache using the main cache backend.
func NewSignatureCache(i do.Injector) (*SignatureCacheService, error) {
	cacheSvc := do.MustInvoke[*CacheService](i)

	// SignatureCache wraps the main cache for thinking block signatures
	sigCache := proxy.NewSignatureCache(cacheSvc.Cache)

	return &SignatureCacheService{Cache: sigCache}, nil
}
