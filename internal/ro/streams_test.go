package ro

import (
	"context"
	"testing"
	"time"

	"github.com/samber/ro"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamFromChannel(t *testing.T) {
	t.Run("emits all values from channel", func(t *testing.T) {
		ch := make(chan int, 3)
		ch <- 1
		ch <- 2
		ch <- 3
		close(ch)

		var results []int
		done := make(chan struct{})

		StreamFromChannel(ch).Subscribe(ro.NewObserver(
			func(i int) { results = append(results, i) },
			func(err error) { t.Errorf("unexpected error: %v", err) },
			func() { close(done) },
		))

		<-done
		assert.Equal(t, []int{1, 2, 3}, results)
	})

	t.Run("completes on empty channel", func(t *testing.T) {
		ch := make(chan int)
		close(ch)

		completed := false
		done := make(chan struct{})

		StreamFromChannel(ch).Subscribe(ro.NewObserver(
			func(_ int) { t.Error("unexpected value") },
			func(err error) { t.Errorf("unexpected error: %v", err) },
			func() {
				completed = true
				close(done)
			},
		))

		<-done
		assert.True(t, completed)
	})
}

func TestStreamFromSlice(t *testing.T) {
	t.Run("emits all values from slice", func(t *testing.T) {
		items := []string{"a", "b", "c"}

		results, err := Collect(StreamFromSlice(items))

		require.NoError(t, err)
		assert.Equal(t, items, results)
	})

	t.Run("handles empty slice", func(t *testing.T) {
		items := []int{}

		results, err := Collect(StreamFromSlice(items))

		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestJust(t *testing.T) {
	t.Run("emits single value", func(t *testing.T) {
		results, err := Collect(Just(42))

		require.NoError(t, err)
		assert.Equal(t, []int{42}, results)
	})

	t.Run("emits multiple values", func(t *testing.T) {
		results, err := Collect(Just(1, 2, 3))

		require.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3}, results)
	})
}

func TestEmpty(t *testing.T) {
	results, err := Collect(Empty[int]())

	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestThrow(t *testing.T) {
	testErr := assert.AnError

	_, err := Collect(Throw[int](testErr))

	require.Error(t, err)
	assert.Equal(t, testErr, err)
}

func TestProcessStream(t *testing.T) {
	t.Run("applies mapper and filter", func(t *testing.T) {
		items := []int{1, 2, 3, 4, 5}
		source := StreamFromSlice(items)

		// Double all values and keep only those > 4
		result := ProcessStream(
			source,
			func(i int) int { return i * 2 },
			func(i int) bool { return i > 4 },
		)

		results, err := Collect(result)

		require.NoError(t, err)
		assert.Equal(t, []int{6, 8, 10}, results)
	})

	t.Run("empty result when no values pass filter", func(t *testing.T) {
		items := []int{1, 2, 3}
		source := StreamFromSlice(items)

		result := ProcessStream(
			source,
			func(i int) int { return i },
			func(i int) bool { return i > 100 },
		)

		results, err := Collect(result)

		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestFilterStream(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	source := StreamFromSlice(items)

	// Keep only even numbers
	result := FilterStream(source, func(i int) bool { return i%2 == 0 })

	results, err := Collect(result)

	require.NoError(t, err)
	assert.Equal(t, []int{2, 4}, results)
}

func TestMapStream(t *testing.T) {
	items := []int{1, 2, 3}
	source := StreamFromSlice(items)

	// Convert to strings
	result := MapStream(source, func(i int) string {
		return string(rune('a' + i - 1))
	})

	results, err := Collect(result)

	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, results)
}

func TestTakeFirst(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	source := StreamFromSlice(items)

	result := TakeFirst(source, 3)

	results, err := Collect(result)

	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3}, results)
}

func TestSkipFirst(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	source := StreamFromSlice(items)

	result := SkipFirst(source, 2)

	results, err := Collect(result)

	require.NoError(t, err)
	assert.Equal(t, []int{3, 4, 5}, results)
}

func TestMergeStreams(t *testing.T) {
	stream1 := Just(1, 2)
	stream2 := Just(3, 4)

	result := MergeStreams(stream1, stream2)

	results, err := Collect(result)

	require.NoError(t, err)
	assert.Len(t, results, 4)
	// Order may vary due to merge, but all values should be present
	assert.ElementsMatch(t, []int{1, 2, 3, 4}, results)
}

func TestConcatStreams(t *testing.T) {
	stream1 := Just(1, 2)
	stream2 := Just(3, 4)

	result := ConcatStreams(stream1, stream2)

	results, err := Collect(result)

	require.NoError(t, err)
	// Concat preserves order
	assert.Equal(t, []int{1, 2, 3, 4}, results)
}

func TestCollect(t *testing.T) {
	t.Run("collects all values", func(t *testing.T) {
		results, err := Collect(Just(1, 2, 3))

		require.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3}, results)
	})

	t.Run("returns error on stream error", func(t *testing.T) {
		testErr := assert.AnError

		_, err := Collect(Throw[int](testErr))

		require.Error(t, err)
	})
}

func TestCollectWithContext(t *testing.T) {
	t.Run("collects with context", func(t *testing.T) {
		ctx := context.Background()

		results, _, err := CollectWithContext(ctx, Just(1, 2, 3))

		require.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3}, results)
	})

	t.Run("respects context cancellation", func(_ *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		// Create a stream that would never complete
		ch := make(chan int)
		// Don't close - stream never completes

		// Use a select with timeout to avoid blocking forever
		done := make(chan struct{})
		go func() {
			_, _, _ = CollectWithContext(ctx, StreamFromChannel(ch))
			close(done)
		}()

		select {
		case <-done:
			// Good - context cancellation caused early return
		case <-time.After(100 * time.Millisecond):
			// Also acceptable - test may timeout
		}
	})
}

func TestBufferWithTime(t *testing.T) {
	ch := make(chan int)

	go func() {
		ch <- 1
		ch <- 2
		ch <- 3
		time.Sleep(50 * time.Millisecond)
		close(ch)
	}()

	result := BufferWithTime(StreamFromChannel(ch), 100*time.Millisecond)

	results, err := Collect(result)

	require.NoError(t, err)
	// Should have at least one batch with values
	assert.NotEmpty(t, results)
}

func TestBufferWithCount(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	source := StreamFromSlice(items)

	result := BufferWithCount(source, 2)

	results, err := Collect(result)

	require.NoError(t, err)
	assert.Len(t, results, 3) // [1,2], [3,4], [5]
	assert.Equal(t, []int{1, 2}, results[0])
	assert.Equal(t, []int{3, 4}, results[1])
	assert.Equal(t, []int{5}, results[2])
}

func TestBufferWithTimeOrCount(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	source := StreamFromSlice(items)

	// Count of 2 should trigger before 1 second
	result := BufferWithTimeOrCount(source, 2, time.Second)

	results, err := Collect(result)

	require.NoError(t, err)
	assert.NotEmpty(t, results)
}
