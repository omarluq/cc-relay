package cache_test

import (
	"errors"
	"testing"

	"github.com/omarluq/cc-relay/internal/cache"
)

func TestIgnoreCacheErr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err  error
		name string
	}{
		{name: "nil error", err: nil},
		{name: "not found error", err: cache.ErrNotFound},
		{name: "closed error", err: cache.ErrClosed},
		{name: "generic error", err: errors.New("some error")},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			// ignoreCacheErr should not panic for any error
			cache.IgnoreCacheErr(testCase.err)
		})
	}
}
