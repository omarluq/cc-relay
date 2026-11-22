package proxy

import (
	"context"
	"net/http"
	"net/http/httptrace"
	"sync/atomic"
)

// failureReportingTransport reports round trips that failed because of the
// upstream or the network path to it.
//
// ReverseProxy.ErrorHandler can't be used for this. It also receives errors
// caused by the client's request and errors raised after a response arrived,
// and counting those would let a client open a healthy provider's circuit.
type failureReportingTransport struct {
	next   http.RoundTripper
	report func(ctx context.Context, err error)
}

// RoundTrip forwards the request and reports the error if the upstream is at fault.
func (t *failureReportingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	var progress roundTripProgress
	traced := request.WithContext(httptrace.WithClientTrace(request.Context(), progress.trace()))

	response, err := t.next.RoundTrip(traced)
	if err == nil {
		return response, nil
	}

	// A done context means the client went away. Trailers come from the client
	// and can be rejected locally after the headers were written.
	if request.Context().Err() == nil && len(request.Trailer) == 0 && progress.upstreamAtFault() {
		t.report(request.Context(), err)
	}
	return response, err
}

// roundTripProgress records how far a round trip got. Trace hooks can run on
// the transport's goroutines, so the fields are atomic.
type roundTripProgress struct {
	connecting   atomic.Bool
	connected    atomic.Bool
	writeStarted atomic.Bool
}

func (p *roundTripProgress) trace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		GetConn: func(string) {
			p.connecting.Store(true)
			p.connected.Store(false) // A retry needs a connection of its own.
		},
		GotConn:          func(httptrace.GotConnInfo) { p.connected.Store(true) },
		WroteHeaderField: func(string, []string) { p.writeStarted.Store(true) },
	}
}

// upstreamAtFault reports whether a failed round trip either couldn't get a
// connection or failed after the request started to be written. net/http
// rejects bad requests outside those two windows: before it asks for a
// connection, or (HTTP/2 header checks) before it writes the first header.
func (p *roundTripProgress) upstreamAtFault() bool {
	if !p.connecting.Load() {
		return false
	}
	return !p.connected.Load() || p.writeStarted.Load()
}
