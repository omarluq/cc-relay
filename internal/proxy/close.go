package proxy

import "io"

func closeBody(body io.ReadCloser) {
	if err := body.Close(); err != nil {
		_ = err
	}
}
