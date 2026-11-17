package proxy_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/cc-relay/internal/proxy"
)

func TestResponseWriterPreservesStreamingFlush(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	writer := proxy.NewTestResponseWriter()
	writer.ResponseWriter = recorder

	require.NoError(t, http.NewResponseController(writer).Flush())
	assert.True(t, recorder.Flushed)
}
