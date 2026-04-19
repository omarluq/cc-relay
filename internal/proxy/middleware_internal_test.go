package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetBodyPreview_NilBody(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/test", http.NoBody)
	preview := getBodyPreview(req)

	if preview != "" {
		t.Errorf("getBodyPreview() = %q, want empty", preview)
	}
}

func TestGetBodyPreview_TruncatesLongBody(t *testing.T) {
	t.Parallel()

	longBody := strings.Repeat("a", 300)
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/test", strings.NewReader(longBody))
	preview := getBodyPreview(req)

	if len(preview) != 203 { // 200 + "..."
		t.Errorf("getBodyPreview() length = %d, want 203", len(preview))
	}
	if !strings.HasSuffix(preview, "...") {
		t.Errorf("getBodyPreview() should end with \"...\", got %q", preview)
	}
}

func TestGetBodyPreview_ShortBodyNotTruncated(t *testing.T) {
	t.Parallel()

	shortBody := "short body"
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/test", strings.NewReader(shortBody))
	preview := getBodyPreview(req)

	if preview != shortBody {
		t.Errorf("getBodyPreview() = %q, want %q", preview, shortBody)
	}
}

func TestGetBodyPreview_EmptyBody(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/test", strings.NewReader(""))
	preview := getBodyPreview(req)

	if preview != "" {
		t.Errorf("getBodyPreview() = %q, want empty for empty body", preview)
	}
}

func TestGetBodyPreview_RedactsAPIKey(t *testing.T) {
	t.Parallel()

	bodyWithKey := `{"api_key":"secret123","prompt":"test"}`
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/test", strings.NewReader(bodyWithKey))
	preview := getBodyPreview(req)

	if strings.Contains(preview, "secret123") {
		t.Errorf("getBodyPreview() should redact api_key value, got %q", preview)
	}
}

func TestGetBodyPreview_RedactsXAPIKey(t *testing.T) {
	t.Parallel()

	bodyWithKey := `{"x-api-key":"secret123","prompt":"test"}`
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/test", strings.NewReader(bodyWithKey))
	preview := getBodyPreview(req)

	if strings.Contains(preview, "secret123") {
		t.Errorf("getBodyPreview() should redact x-api-key value, got %q", preview)
	}
}

func TestGetBodyPreview_RedactsToken(t *testing.T) {
	t.Parallel()

	bodyWithKey := `{"token":"secret123","prompt":"test"}`
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/test", strings.NewReader(bodyWithKey))
	preview := getBodyPreview(req)

	if strings.Contains(preview, "secret123") {
		t.Errorf("getBodyPreview() should redact token value, got %q", preview)
	}
}

func TestGetBodyPreview_RedactsPassword(t *testing.T) {
	t.Parallel()

	bodyWithKey := `{"password":"secret123","prompt":"test"}`
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/test", strings.NewReader(bodyWithKey))
	preview := getBodyPreview(req)

	if strings.Contains(preview, "secret123") {
		t.Errorf("getBodyPreview() should redact password value, got %q", preview)
	}
}
