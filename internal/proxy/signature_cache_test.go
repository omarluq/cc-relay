package proxy_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/cc-relay/internal/proxy"
)

// testValidSignature is a valid signature long enough for caching (>= MinSignatureLen).
const testValidSignature = "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz"

func TestGetModelGroup(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		model    string
		expected string
	}{
		{"claude sonnet 4", "claude-sonnet-4-20250514", "claude"},
		{"claude 3 opus", "claude-3-opus-20240229", "claude"},
		{"claude 3.5 sonnet", "claude-3-5-sonnet-20241022", "claude"},
		{"claude with uppercase", "Claude-3-Opus", "claude"},
		{"gpt 4", "gpt-4-turbo", "gpt"},
		{"gpt 4o", "gpt-4o", "gpt"},
		{"gpt with uppercase", "GPT-4", "gpt"},
		{"gemini pro", "gemini-1.5-pro", "gemini"},
		{"gemini flash", "gemini-2.0-flash", "gemini"},
		{"unknown model", "llama-3-70b", "llama-3-70b"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := proxy.GetModelGroup(tt.model)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestSignatureCacheCacheKey(t *testing.T) {
	t.Parallel()
	sigCache, cleanup := proxy.NewTestSignatureCache(t)
	defer cleanup()
	require.NotNil(t, sigCache)

	// Test deterministic key generation
	key1 := proxy.CacheKey(sigCache, "claude", "thinking text")
	key2 := proxy.CacheKey(sigCache, "claude", "thinking text")
	assert.Equal(t, key1, key2, "same input should produce same key")

	// Test different text produces different key
	key3 := proxy.CacheKey(sigCache, "claude", "different thinking text")
	assert.NotEqual(t, key1, key3, "different text should produce different key")

	// Test different model group produces different key
	key4 := proxy.CacheKey(sigCache, "gpt", "thinking text")
	assert.NotEqual(t, key1, key4, "different model group should produce different key")

	// Test key format
	assert.Contains(t, key1, "sig:claude:")
	assert.Len(t, key1, len("sig:claude:")+proxy.SignatureHashLen)
}

func TestSignatureCacheGetSet(t *testing.T) {
	t.Parallel()
	sigCache, cleanup := proxy.NewTestSignatureCache(t)
	defer cleanup()
	ctx := context.Background()

	// Generate a valid signature (>= MinSignatureLen)
	validSig := "sig_" + string(make([]byte, proxy.MinSignatureLen))
	for i := 4; i < len(validSig); i++ {
		validSig = validSig[:i] + "a" + validSig[i+1:]
	}
	validSig = testValidSignature

	// Test cache miss
	got := sigCache.Get(ctx, "claude-sonnet-4", "thinking text")
	assert.Empty(t, got, "should return empty on cache miss")

	// Test cache set and get
	sigCache.Set(ctx, "claude-sonnet-4", "thinking text", validSig)

	// Ristretto needs a small delay for async set
	time.Sleep(10 * time.Millisecond)

	got = sigCache.Get(ctx, "claude-sonnet-4", "thinking text")
	assert.Equal(t, validSig, got, "should return cached signature")

	// Test same model group retrieval (different model, same group)
	got = sigCache.Get(ctx, "claude-3-opus", "thinking text")
	assert.Equal(t, validSig, got, "should return signature for same model group")

	// Test different model group (should miss)
	got = sigCache.Get(ctx, "gpt-4", "thinking text")
	assert.Empty(t, got, "should miss for different model group")
}

func TestSignatureCacheSkipsShortSignatures(t *testing.T) {
	t.Parallel()
	sigCache, cleanup := proxy.NewTestSignatureCache(t)
	defer cleanup()
	ctx := context.Background()

	// Try to set a short signature (should be skipped)
	shortSig := "short"
	sigCache.Set(ctx, "claude-sonnet-4", "thinking text", shortSig)

	time.Sleep(10 * time.Millisecond)

	// Should not be cached
	got := sigCache.Get(ctx, "claude-sonnet-4", "thinking text")
	assert.Empty(t, got, "short signature should not be cached")
}

func TestSignatureCacheNilCache(t *testing.T) {
	t.Parallel(
	// NewSignatureCache with nil should return nil
	)

	sigCache := proxy.NewSignatureCache(nil)
	assert.Nil(t, sigCache)

	// Operations on nil SignatureCache should be safe
	var nilSC *proxy.SignatureCache
	ctx := context.Background()

	// Get should return empty
	got := nilSC.Get(ctx, "claude", "text")
	assert.Empty(t, got)

	// Set should not panic
	assert.NotPanics(t, func() {
		nilSC.Set(ctx, "claude", "text", "signature")
	})
}

func TestIsValidSignature(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		modelName string
		signature string
		expected  bool
	}{
		{"empty signature", "claude", "", false},
		{"short signature", "claude", "abc", false},
		{"just under minimum", "claude", string(make([]byte, proxy.MinSignatureLen-1)), false},
		{"exactly minimum", "claude", string(make([]byte, proxy.MinSignatureLen)), true},
		{"valid long signature", "claude", string(make([]byte, 100)), true},
		{"gemini sentinel", "gemini-pro", proxy.GeminiSignatureSentinel, true},
		{"gemini sentinel invalid for non-gemini", "claude", proxy.GeminiSignatureSentinel, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := proxy.IsValidSignature(tt.modelName, tt.signature)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestSignatureCacheGeminiSentinel(t *testing.T) {
	t.Parallel()
	sigCache, cleanup := proxy.NewTestSignatureCache(t)
	defer cleanup()
	ctx := context.Background()

	// Gemini sentinel should be cached even though it's short
	sigCache.Set(ctx, "gemini-pro", "thinking text", proxy.GeminiSignatureSentinel)

	time.Sleep(10 * time.Millisecond)

	got := sigCache.Get(ctx, "gemini-pro", "thinking text")
	assert.Equal(t, proxy.GeminiSignatureSentinel, got)
}
