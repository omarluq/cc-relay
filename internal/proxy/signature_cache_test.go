package proxy

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/cc-relay/internal/cache"
)

func TestGetModelGroup(t *testing.T) {
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
			got := GetModelGroup(tt.model)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestSignatureCache_CacheKey(t *testing.T) {
	cfg := cache.Config{
		Mode: cache.ModeSingle,
		Ristretto: cache.RistrettoConfig{
			NumCounters: 1e4,
			MaxCost:     1 << 20,
			BufferItems: 64,
		},
	}

	c, err := cache.New(context.Background(), &cfg)
	require.NoError(t, err)
	defer c.Close()

	sc := NewSignatureCache(c)
	require.NotNil(t, sc)

	// Test deterministic key generation
	key1 := sc.cacheKey("claude", "thinking text")
	key2 := sc.cacheKey("claude", "thinking text")
	assert.Equal(t, key1, key2, "same input should produce same key")

	// Test different text produces different key
	key3 := sc.cacheKey("claude", "different thinking text")
	assert.NotEqual(t, key1, key3, "different text should produce different key")

	// Test different model group produces different key
	key4 := sc.cacheKey("gpt", "thinking text")
	assert.NotEqual(t, key1, key4, "different model group should produce different key")

	// Test key format
	assert.Contains(t, key1, "sig:claude:")
	assert.Len(t, key1, len("sig:claude:")+SignatureHashLen)
}

func TestSignatureCache_GetSet(t *testing.T) {
	cfg := cache.Config{
		Mode: cache.ModeSingle,
		Ristretto: cache.RistrettoConfig{
			NumCounters: 1e4,
			MaxCost:     1 << 20,
			BufferItems: 64,
		},
	}

	c, err := cache.New(context.Background(), &cfg)
	require.NoError(t, err)
	defer c.Close()

	sc := NewSignatureCache(c)
	ctx := context.Background()

	// Generate a valid signature (>= MinSignatureLen)
	validSig := "sig_" + string(make([]byte, MinSignatureLen))
	for i := 4; i < len(validSig); i++ {
		validSig = validSig[:i] + "a" + validSig[i+1:]
	}
	validSig = "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz"

	// Test cache miss
	got := sc.Get(ctx, "claude-sonnet-4", "thinking text")
	assert.Empty(t, got, "should return empty on cache miss")

	// Test cache set and get
	sc.Set(ctx, "claude-sonnet-4", "thinking text", validSig)

	// Ristretto needs a small delay for async set
	time.Sleep(10 * time.Millisecond)

	got = sc.Get(ctx, "claude-sonnet-4", "thinking text")
	assert.Equal(t, validSig, got, "should return cached signature")

	// Test same model group retrieval (different model, same group)
	got = sc.Get(ctx, "claude-3-opus", "thinking text")
	assert.Equal(t, validSig, got, "should return signature for same model group")

	// Test different model group (should miss)
	got = sc.Get(ctx, "gpt-4", "thinking text")
	assert.Empty(t, got, "should miss for different model group")
}

func TestSignatureCache_SkipsShortSignatures(t *testing.T) {
	cfg := cache.Config{
		Mode: cache.ModeSingle,
		Ristretto: cache.RistrettoConfig{
			NumCounters: 1e4,
			MaxCost:     1 << 20,
			BufferItems: 64,
		},
	}

	c, err := cache.New(context.Background(), &cfg)
	require.NoError(t, err)
	defer c.Close()

	sc := NewSignatureCache(c)
	ctx := context.Background()

	// Try to set a short signature (should be skipped)
	shortSig := "short"
	sc.Set(ctx, "claude-sonnet-4", "thinking text", shortSig)

	time.Sleep(10 * time.Millisecond)

	// Should not be cached
	got := sc.Get(ctx, "claude-sonnet-4", "thinking text")
	assert.Empty(t, got, "short signature should not be cached")
}

func TestSignatureCache_NilCache(t *testing.T) {
	// NewSignatureCache with nil should return nil
	sc := NewSignatureCache(nil)
	assert.Nil(t, sc)

	// Operations on nil SignatureCache should be safe
	var nilSC *SignatureCache
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
	tests := []struct {
		name      string
		modelName string
		signature string
		expected  bool
	}{
		{"empty signature", "claude", "", false},
		{"short signature", "claude", "abc", false},
		{"just under minimum", "claude", string(make([]byte, MinSignatureLen-1)), false},
		{"exactly minimum", "claude", string(make([]byte, MinSignatureLen)), true},
		{"valid long signature", "claude", string(make([]byte, 100)), true},
		{"gemini sentinel", "gemini-pro", GeminiSignatureSentinel, true},
		{"gemini sentinel invalid for non-gemini", "claude", GeminiSignatureSentinel, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidSignature(tt.modelName, tt.signature)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestSignatureCache_GeminiSentinel(t *testing.T) {
	cfg := cache.Config{
		Mode: cache.ModeSingle,
		Ristretto: cache.RistrettoConfig{
			NumCounters: 1e4,
			MaxCost:     1 << 20,
			BufferItems: 64,
		},
	}

	c, err := cache.New(context.Background(), &cfg)
	require.NoError(t, err)
	defer c.Close()

	sc := NewSignatureCache(c)
	ctx := context.Background()

	// Gemini sentinel should be cached even though it's short
	sc.Set(ctx, "gemini-pro", "thinking text", GeminiSignatureSentinel)

	time.Sleep(10 * time.Millisecond)

	got := sc.Get(ctx, "gemini-pro", "thinking text")
	assert.Equal(t, GeminiSignatureSentinel, got)
}
