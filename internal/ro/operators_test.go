package ro

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/samber/ro"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogEach(t *testing.T) {
	t.Run("logs items without modifying stream", func(t *testing.T) {
		var buf bytes.Buffer
		logger := zerolog.New(&buf).Level(zerolog.DebugLevel)

		items := []int{1, 2, 3}
		source := StreamFromSlice(items)

		result := ro.Pipe1(source, LogEach[int](&logger, "test-stream"))

		results, err := Collect(result)

		require.NoError(t, err)
		assert.Equal(t, items, results, "values should not be modified")

		// Verify logging occurred
		logOutput := buf.String()
		assert.Contains(t, logOutput, "stream event")
		assert.Contains(t, logOutput, "test-stream")
	})

	t.Run("preserves stream values unchanged", func(t *testing.T) {
		var buf bytes.Buffer
		logger := zerolog.New(&buf).Level(zerolog.DebugLevel)

		items := []string{"hello", "world"}
		source := StreamFromSlice(items)

		result := ro.Pipe1(source, LogEach[string](&logger, "string-stream"))

		results, err := Collect(result)

		require.NoError(t, err)
		assert.Equal(t, items, results)
	})
}

func TestLogWithLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.InfoLevel)

	source := Just(42)

	result := ro.Pipe1(source, LogWithLevel[int](&logger, zerolog.InfoLevel))

	results, err := Collect(result)

	require.NoError(t, err)
	assert.Equal(t, []int{42}, results)
}

func TestWithTimeout(t *testing.T) {
	t.Run("passes values before timeout", func(t *testing.T) {
		source := Just(1, 2, 3)

		result := ro.Pipe1(source, WithTimeout[int](time.Second))

		results, err := Collect(result)

		require.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3}, results)
	})

	t.Run("times out on slow stream", func(t *testing.T) {
		ch := make(chan int)
		// Don't send anything - let it timeout

		result := ro.Pipe1(StreamFromChannel(ch), WithTimeout[int](50*time.Millisecond))

		done := make(chan struct{})
		var gotError bool

		go func() {
			_, err := Collect(result)
			gotError = err != nil
			close(done)
		}()

		// Close channel after test
		go func() {
			time.Sleep(200 * time.Millisecond)
			close(ch)
		}()

		select {
		case <-done:
			assert.True(t, gotError, "should have received timeout error")
		case <-time.After(500 * time.Millisecond):
			t.Fatal("test timeout")
		}
	})
}

func TestWithRetry(t *testing.T) {
	t.Run("succeeds without needing retry", func(t *testing.T) {
		source := Just(1, 2, 3)

		result := ro.Pipe1(source, WithRetry[int]())

		results, err := Collect(result)

		require.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3}, results)
	})
}

func TestCatch(t *testing.T) {
	t.Run("returns fallback on error", func(t *testing.T) {
		testErr := assert.AnError

		source := Throw[int](testErr)

		result := ro.Pipe1(source, Catch(func(_ error) ro.Observable[int] {
			return Just(42) // fallback value
		}))

		results, err := Collect(result)

		require.NoError(t, err)
		assert.Equal(t, []int{42}, results)
	})

	t.Run("passes through successful stream", func(t *testing.T) {
		source := Just(1, 2, 3)

		result := ro.Pipe1(source, Catch(func(_ error) ro.Observable[int] {
			return Just(0) // fallback - should not be used
		}))

		results, err := Collect(result)

		require.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3}, results)
	})
}

func TestDoOnNext(t *testing.T) {
	var sideEffects []int
	source := Just(1, 2, 3)

	result := ro.Pipe1(source, DoOnNext(func(i int) {
		sideEffects = append(sideEffects, i*10)
	}))

	results, err := Collect(result)

	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3}, results)
	assert.Equal(t, []int{10, 20, 30}, sideEffects)
}

func TestDoOnError(t *testing.T) {
	testErr := assert.AnError
	var capturedErr error

	source := Throw[int](testErr)

	result := ro.Pipe1(source, DoOnError[int](func(err error) {
		capturedErr = err
	}))

	_, _ = Collect(result)

	assert.Equal(t, testErr, capturedErr)
}

func TestDoOnComplete(t *testing.T) {
	completed := false
	source := Just(1, 2, 3)

	result := ro.Pipe1(source, DoOnComplete[int](func() {
		completed = true
	}))

	_, err := Collect(result)

	require.NoError(t, err)
	assert.True(t, completed)
}

func TestDistinctValues(t *testing.T) {
	items := []int{1, 2, 2, 3, 1, 4, 3}
	source := StreamFromSlice(items)

	result := ro.Pipe1(source, DistinctValues[int]())

	results, err := Collect(result)

	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3, 4}, results)
}

func TestDistinctBy(t *testing.T) {
	type item struct {
		Value string
		ID    int
	}

	items := []item{
		{ID: 1, Value: "a"},
		{ID: 2, Value: "b"},
		{ID: 1, Value: "c"}, // Duplicate ID
		{ID: 3, Value: "d"},
	}
	source := StreamFromSlice(items)

	result := ro.Pipe1(source, DistinctBy(func(i item) int { return i.ID }))

	results, err := Collect(result)

	require.NoError(t, err)
	assert.Len(t, results, 3)
	assert.Equal(t, 1, results[0].ID)
	assert.Equal(t, 2, results[1].ID)
	assert.Equal(t, 3, results[2].ID)
}

func TestSubscribeWithCallbacks(t *testing.T) {
	var values []int
	var completedCalled bool
	source := Just(1, 2, 3)

	done := make(chan struct{})

	sub := SubscribeWithCallbacks(
		source,
		func(i int) { values = append(values, i) },
		func(err error) { t.Errorf("unexpected error: %v", err) },
		func() {
			completedCalled = true
			close(done)
		},
	)

	<-done

	assert.Equal(t, []int{1, 2, 3}, values)
	assert.True(t, completedCalled)
	assert.NotNil(t, sub)
}

func TestSubscribeWithContext(t *testing.T) {
	ctx := context.Background()
	var values []int
	var completedCalled bool
	source := Just(1, 2, 3)

	done := make(chan struct{})

	sub := SubscribeWithContext(
		ctx,
		source,
		func(_ context.Context, i int) { values = append(values, i) },
		func(_ context.Context, err error) { t.Errorf("unexpected error: %v", err) },
		func(_ context.Context) {
			completedCalled = true
			close(done)
		},
	)

	<-done

	assert.Equal(t, []int{1, 2, 3}, values)
	assert.True(t, completedCalled)
	assert.NotNil(t, sub)
}
