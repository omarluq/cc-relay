package proxy

import "io"

// closeBody closes the provided io.ReadCloser and discards any error returned.
// It ensures the reader is closed without propagating closure errors.
func closeBody(body io.ReadCloser) {
	if err := body.Close(); err != nil {
		_ = err
	}
}