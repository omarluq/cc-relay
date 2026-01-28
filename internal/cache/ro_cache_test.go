package cache

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/samber/ro"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCache implements the Cache interface for testing.
type mockCache struct {
	setErr error
	getErr error
	delErr error
	data   map[string][]byte
	mu     sync.RWMutex
	closed bool
}

func newMockCache() *mockCache {
	return &mockCache{
		data: make(map[string][]byte),
	}
}

func (m *mockCache) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.closed {
		return nil, ErrClosed
	}
	val, ok := m.data[key]
	if !ok {
		return nil, ErrNotFound
	}
	return val, nil
}

func (m *mockCache) Set(_ context.Context, key string, value []byte) error {
	return m.SetWithTTL(context.Background(), key, value, 0)
}

func (m *mockCache) SetWithTTL(_ context.Context, key string, value []byte, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.setErr != nil {
		return m.setErr
	}
	if m.closed {
		return ErrClosed
	}
	m.data[key] = value
	return nil
}

func (m *mockCache) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.delErr != nil {
		return m.delErr
	}
	if m.closed {
		return ErrClosed
	}
	delete(m.data, key)
	return nil
}

func (m *mockCache) Exists(_ context.Context, key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return false, ErrClosed
	}
	_, ok := m.data[key]
	return ok, nil
}

func (m *mockCache) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func TestNewROCache(t *testing.T) {
	t.Parallel()

	mock := newMockCache()
	ttl := 5 * time.Minute

	roCache := NewROCache(mock, ttl)

	assert.NotNil(t, roCache)
	assert.Equal(t, ttl, roCache.GetTTL())
	assert.Equal(t, mock, roCache.Underlying())
}

func TestROCacheGet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mock := newMockCache()
	mock.data["key1"] = []byte("value1")

	roCache := NewROCache(mock, time.Minute)

	t.Run("cache hit", func(t *testing.T) {
		t.Parallel()
		results, err := ro.Collect(roCache.Get(ctx, "key1"))
		require.NoError(t, err)
		assert.Equal(t, [][]byte{[]byte("value1")}, results)
	})

	t.Run("cache miss", func(t *testing.T) {
		t.Parallel()
		_, err := ro.Collect(roCache.Get(ctx, "nonexistent"))
		assert.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotFound))
	})
}

func TestROCacheSet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mock := newMockCache()
	roCache := NewROCache(mock, time.Minute)

	t.Run("set success", func(t *testing.T) {
		t.Parallel()
		_, err := ro.Collect(roCache.Set(ctx, "key1", []byte("value1")))
		require.NoError(t, err)

		// Verify the value was stored
		val, err := mock.Get(ctx, "key1")
		require.NoError(t, err)
		assert.Equal(t, []byte("value1"), val)
	})

	t.Run("set with custom ttl", func(t *testing.T) {
		t.Parallel()
		_, err := ro.Collect(roCache.SetWithTTL(ctx, "key2", []byte("value2"), 10*time.Second))
		require.NoError(t, err)

		val, err := mock.Get(ctx, "key2")
		require.NoError(t, err)
		assert.Equal(t, []byte("value2"), val)
	})
}

func TestROCacheExists(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mock := newMockCache()
	mock.data["exists"] = []byte("value")

	roCache := NewROCache(mock, time.Minute)

	t.Run("key exists", func(t *testing.T) {
		t.Parallel()
		results, err := ro.Collect(roCache.Exists(ctx, "exists"))
		require.NoError(t, err)
		assert.Equal(t, []bool{true}, results)
	})

	t.Run("key does not exist", func(t *testing.T) {
		t.Parallel()
		results, err := ro.Collect(roCache.Exists(ctx, "missing"))
		require.NoError(t, err)
		assert.Equal(t, []bool{false}, results)
	})
}

func TestROCacheInvalidate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mock := newMockCache()
	mock.data["key1"] = []byte("value1")

	roCache := NewROCache(mock, time.Minute)

	_, err := ro.Collect(roCache.Invalidate(ctx, "key1"))
	require.NoError(t, err)

	// Verify key was deleted
	_, err = mock.Get(ctx, "key1")
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestROCacheInvalidateMany(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mock := newMockCache()
	mock.data["key1"] = []byte("value1")
	mock.data["key2"] = []byte("value2")
	mock.data["key3"] = []byte("value3")

	roCache := NewROCache(mock, time.Minute)

	keys := []string{"key1", "key2", "nonexistent"}
	results, err := ro.Collect(roCache.InvalidateMany(ctx, keys))
	require.NoError(t, err)
	assert.Equal(t, keys, results)

	// Verify keys were deleted
	_, err = mock.Get(ctx, "key1")
	assert.True(t, errors.Is(err, ErrNotFound))
	_, err = mock.Get(ctx, "key2")
	assert.True(t, errors.Is(err, ErrNotFound))
	// key3 should still exist
	val, err := mock.Get(ctx, "key3")
	require.NoError(t, err)
	assert.Equal(t, []byte("value3"), val)
}

func TestROCacheGetOrFetch(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("cache hit - returns cached value", func(t *testing.T) {
		t.Parallel()
		mock := newMockCache()
		mock.data["key"] = []byte("cached")
		roCache := NewROCache(mock, time.Minute)

		fetchCalled := false
		fetch := func() ro.Observable[[]byte] {
			fetchCalled = true
			return ro.Just([]byte("fetched"))
		}

		results, err := ro.Collect(roCache.GetOrFetch(ctx, "key", fetch))
		require.NoError(t, err)
		assert.Equal(t, [][]byte{[]byte("cached")}, results)
		assert.False(t, fetchCalled, "fetch should not be called on cache hit")
	})

	t.Run("cache miss - fetches and caches", func(t *testing.T) {
		t.Parallel()
		mock := newMockCache()
		roCache := NewROCache(mock, time.Minute)

		fetch := func() ro.Observable[[]byte] {
			return ro.Just([]byte("fetched"))
		}

		results, err := ro.Collect(roCache.GetOrFetch(ctx, "key", fetch))
		require.NoError(t, err)
		assert.Equal(t, [][]byte{[]byte("fetched")}, results)

		// Verify value was cached
		val, err := mock.Get(ctx, "key")
		require.NoError(t, err)
		assert.Equal(t, []byte("fetched"), val)
	})

	t.Run("fetch error propagates", func(t *testing.T) {
		t.Parallel()
		mock := newMockCache()
		roCache := NewROCache(mock, time.Minute)

		fetchErr := errors.New("fetch failed")
		fetch := func() ro.Observable[[]byte] {
			return ro.Throw[[]byte](fetchErr)
		}

		_, err := ro.Collect(roCache.GetOrFetch(ctx, "key", fetch))
		assert.Error(t, err)
		assert.Equal(t, fetchErr, err)
	})
}

func TestGetOrFetchTyped(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	type User struct {
		Name string `json:"name"`
		ID   int    `json:"id"`
	}

	t.Run("cache hit - deserializes cached value", func(t *testing.T) {
		t.Parallel()
		mock := newMockCache()
		cachedUser := User{ID: 1, Name: "Cached"}
		data, _ := json.Marshal(cachedUser)
		mock.data["user:1"] = data

		roCache := NewROCache(mock, time.Minute)

		fetch := func() ro.Observable[User] {
			return ro.Just(User{ID: 1, Name: "Fetched"})
		}

		results, err := ro.Collect(GetOrFetchTyped(ctx, roCache, "user:1", fetch))
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, cachedUser, results[0])
	})

	t.Run("cache miss - fetches and caches typed value", func(t *testing.T) {
		t.Parallel()
		mock := newMockCache()
		roCache := NewROCache(mock, time.Minute)

		fetchedUser := User{ID: 2, Name: "Fetched"}
		fetch := func() ro.Observable[User] {
			return ro.Just(fetchedUser)
		}

		results, err := ro.Collect(GetOrFetchTyped(ctx, roCache, "user:2", fetch))
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, fetchedUser, results[0])

		// Verify it was cached
		data, err := mock.Get(ctx, "user:2")
		require.NoError(t, err)
		var cachedUser User
		require.NoError(t, json.Unmarshal(data, &cachedUser))
		assert.Equal(t, fetchedUser, cachedUser)
	})

	t.Run("invalid cached JSON - refetches", func(t *testing.T) {
		t.Parallel()
		mock := newMockCache()
		mock.data["user:3"] = []byte("invalid json")

		roCache := NewROCache(mock, time.Minute)

		fetchedUser := User{ID: 3, Name: "Refetched"}
		fetch := func() ro.Observable[User] {
			return ro.Just(fetchedUser)
		}

		results, err := ro.Collect(GetOrFetchTyped(ctx, roCache, "user:3", fetch))
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, fetchedUser, results[0])
	})

	t.Run("fetch returns no value - error", func(t *testing.T) {
		t.Parallel()
		mock := newMockCache()
		roCache := NewROCache(mock, time.Minute)

		fetch := func() ro.Observable[User] {
			return ro.Empty[User]()
		}

		_, err := ro.Collect(GetOrFetchTyped(ctx, roCache, "user:4", fetch))
		assert.Error(t, err)
		assert.True(t, errors.Is(err, ErrCacheFetchFailed))
	})
}

func TestStream(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	type Item struct {
		Value string `json:"value"`
		ID    int    `json:"id"`
	}

	mock := newMockCache()
	roCache := NewROCache(mock, time.Minute)

	items := []Item{
		{ID: 1, Value: "one"},
		{ID: 2, Value: "two"},
		{ID: 3, Value: "three"},
	}

	source := ro.FromSlice(items)
	keyFunc := func(i Item) string {
		return "item:" + string(rune('0'+i.ID))
	}

	cached := Stream(ctx, roCache, source, keyFunc)

	results, err := ro.Collect(cached)
	require.NoError(t, err)
	assert.Equal(t, items, results)

	// Verify all items were cached
	for _, item := range items {
		data, err := mock.Get(ctx, keyFunc(item))
		require.NoError(t, err)

		var cached Item
		require.NoError(t, json.Unmarshal(data, &cached))
		assert.Equal(t, item, cached)
	}
}

func TestROCacheErrorHandling(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("get error propagates", func(t *testing.T) {
		t.Parallel()
		mock := newMockCache()
		mock.getErr = errors.New("get error")
		roCache := NewROCache(mock, time.Minute)

		_, err := ro.Collect(roCache.Get(ctx, "key"))
		assert.Error(t, err)
	})

	t.Run("set error propagates", func(t *testing.T) {
		t.Parallel()
		mock := newMockCache()
		mock.setErr = errors.New("set error")
		roCache := NewROCache(mock, time.Minute)

		_, err := ro.Collect(roCache.Set(ctx, "key", []byte("value")))
		assert.Error(t, err)
	})

	t.Run("delete error propagates", func(t *testing.T) {
		t.Parallel()
		mock := newMockCache()
		mock.delErr = errors.New("delete error")
		roCache := NewROCache(mock, time.Minute)

		_, err := ro.Collect(roCache.Invalidate(ctx, "key"))
		assert.Error(t, err)
	})
}

func TestROCacheConcurrentAccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mock := newMockCache()
	roCache := NewROCache(mock, time.Minute)

	var wg sync.WaitGroup
	const numGoroutines = 10
	const numOps = 100

	// Concurrent sets
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := "key-" + string(rune('0'+id))
				_, _ = ro.Collect(roCache.Set(ctx, key, []byte("value")))
			}
		}(i)
	}

	// Concurrent gets
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := "key-" + string(rune('0'+id))
				_, _ = ro.Collect(roCache.Get(ctx, key))
			}
		}(i)
	}

	wg.Wait()
	// Test passes if no race conditions detected
}

func BenchmarkROCacheGet(b *testing.B) {
	ctx := context.Background()
	mock := newMockCache()
	mock.data["key"] = []byte("value")
	roCache := NewROCache(mock, time.Minute)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ro.Collect(roCache.Get(ctx, "key"))
	}
}

func BenchmarkROCacheGetOrFetchCacheHit(b *testing.B) {
	ctx := context.Background()
	mock := newMockCache()
	mock.data["key"] = []byte("cached")
	roCache := NewROCache(mock, time.Minute)

	fetch := func() ro.Observable[[]byte] {
		return ro.Just([]byte("fetched"))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ro.Collect(roCache.GetOrFetch(ctx, "key", fetch))
	}
}

func BenchmarkROCacheGetOrFetchCacheMiss(b *testing.B) {
	ctx := context.Background()
	mock := newMockCache()
	roCache := NewROCache(mock, time.Minute)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fetch := func() ro.Observable[[]byte] {
			return ro.Just([]byte("fetched"))
		}
		key := "key-" + string(rune('0'+i%10))
		_, _ = ro.Collect(roCache.GetOrFetch(ctx, key, fetch))
	}
}
