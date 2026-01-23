// Package proxy provides HTTP proxy functionality for cc-relay.
// This file provides SSE (Server-Sent Events) streaming utilities using samber/ro.
//
// SSE streaming utilities provide reactive stream processing for SSE events.
// They are designed to work alongside the existing handler.go implementation,
// providing an alternative approach for future refactoring.
//
// Current handler.go uses direct streaming (which is performant).
// These utilities can be used when reactive stream processing is beneficial:
//   - Transforming SSE events during streaming
//   - Filtering or aggregating events
//   - Composing multiple event streams
//   - Testing SSE processing in isolation
//
// When to use SSE stream utilities:
//   - Building custom SSE processing pipelines
//   - Unit testing SSE transformations
//   - Implementing SSE middleware
//
// When to use direct streaming (current approach):
//   - Simple passthrough proxying (handler.go)
//   - Maximum performance requirements
//   - Minimal transformation needed
package proxy

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/samber/ro"
)

// SSEEvent represents a Server-Sent Event.
// Fields match the SSE specification: https://html.spec.whatwg.org/multipage/server-sent-events.html
type SSEEvent struct {
	Event string
	ID    string
	Data  []byte
	Retry int
}

// String returns the SSE wire format representation of the event.
func (e SSEEvent) String() string {
	var buf bytes.Buffer
	if e.Event != "" {
		fmt.Fprintf(&buf, "event: %s\n", e.Event)
	}
	if e.ID != "" {
		fmt.Fprintf(&buf, "id: %s\n", e.ID)
	}
	if e.Retry > 0 {
		fmt.Fprintf(&buf, "retry: %d\n", e.Retry)
	}
	if len(e.Data) > 0 {
		// Split data on newlines and emit each as separate data: line
		lines := bytes.Split(e.Data, []byte("\n"))
		for _, line := range lines {
			fmt.Fprintf(&buf, "data: %s\n", line)
		}
	}
	buf.WriteString("\n")
	return buf.String()
}

// Bytes returns the SSE wire format representation as bytes.
func (e SSEEvent) Bytes() []byte {
	return []byte(e.String())
}

// ErrNotFlushable is returned when the ResponseWriter doesn't support flushing.
var ErrNotFlushable = errors.New("sse: ResponseWriter does not implement http.Flusher")

// ErrStreamClosed is returned when attempting to write to a closed stream.
var ErrStreamClosed = errors.New("sse: stream is closed")

// StreamSSE creates an Observable from an SSE response body.
// Events are parsed according to the SSE specification and emitted as they arrive.
// The stream completes when the response body is fully read or EOF is reached.
// The stream errors if parsing fails or the body read encounters an error.
//
// Note: The caller is responsible for closing the response body after the
// observable completes or errors.
//
// Example:
//
//	resp, _ := http.Get("https://api.example.com/events")
//	events := StreamSSE(resp.Body)
//	events.Subscribe(ro.NewObserver(
//	    func(e SSEEvent) { process(e) },
//	    func(err error) { handleError(err) },
//	    func() { resp.Body.Close() },
//	))
func StreamSSE(body io.Reader) ro.Observable[SSEEvent] {
	return ro.NewObservable(func(observer ro.Observer[SSEEvent]) ro.Teardown {
		parser := newSSEParser()
		parser.parseStream(bufio.NewReader(body), observer)
		return nil
	})
}

// sseParser handles SSE parsing state.
type sseParser struct {
	dataLines [][]byte
	event     SSEEvent
}

func newSSEParser() *sseParser {
	return &sseParser{}
}

// parseStream reads and parses SSE events from the reader.
func (p *sseParser) parseStream(reader *bufio.Reader, observer ro.Observer[SSEEvent]) {
	for {
		line, err := reader.ReadBytes('\n')
		p.processLine(line, observer)

		if err != nil {
			p.finalize(observer, err)
			return
		}
	}
}

// processLine handles a single line from the SSE stream.
func (p *sseParser) processLine(line []byte, observer ro.Observer[SSEEvent]) {
	if len(line) == 0 {
		return
	}

	line = trimLineEndings(line)

	if len(line) == 0 {
		p.emitEventIfReady(observer)
		return
	}

	p.parseField(line)
}

// trimLineEndings removes trailing \r and \n from a line.
func trimLineEndings(line []byte) []byte {
	line = bytes.TrimSuffix(line, []byte("\n"))
	return bytes.TrimSuffix(line, []byte("\r"))
}

// emitEventIfReady emits the current event if data has been accumulated.
func (p *sseParser) emitEventIfReady(observer ro.Observer[SSEEvent]) {
	if len(p.dataLines) == 0 {
		return
	}

	p.event.Data = bytes.Join(p.dataLines, []byte("\n"))
	observer.Next(p.event)
	p.event = SSEEvent{}
	p.dataLines = nil
}

// finalize handles end-of-stream, emitting any pending event.
func (p *sseParser) finalize(observer ro.Observer[SSEEvent], err error) {
	p.emitEventIfReady(observer)

	if errors.Is(err, io.EOF) {
		observer.Complete()
	} else {
		observer.Error(err)
	}
}

// parseField parses a single SSE field line and updates the event.
func (p *sseParser) parseField(line []byte) {
	if isComment(line) {
		return
	}

	field, value := splitFieldValue(line)
	p.setField(field, value)
}

// isComment returns true if the line is an SSE comment.
func isComment(line []byte) bool {
	return len(line) > 0 && line[0] == ':'
}

// splitFieldValue splits a line into field name and value.
func splitFieldValue(line []byte) (field, value []byte) {
	colonIdx := bytes.IndexByte(line, ':')
	if colonIdx == -1 {
		return line, nil
	}

	field = line[:colonIdx]
	value = line[colonIdx+1:]

	// Remove optional leading space from value
	if len(value) > 0 && value[0] == ' ' {
		value = value[1:]
	}

	return field, value
}

// setField updates the event with the parsed field.
func (p *sseParser) setField(field, value []byte) {
	switch string(field) {
	case "event":
		p.event.Event = string(value)
	case "data":
		p.dataLines = append(p.dataLines, value)
	case "id":
		p.event.ID = string(value)
	case "retry":
		p.setRetry(value)
	}
}

// setRetry parses and sets the retry field if valid.
func (p *sseParser) setRetry(value []byte) {
	if n, err := strconv.Atoi(string(value)); err == nil {
		p.event.Retry = n
	}
}

// ForwardSSE pipes SSE events from an Observable to an http.ResponseWriter.
// Sets appropriate SSE headers and flushes after each event.
// Blocks until the observable completes or errors.
//
// Returns ErrNotFlushable if the ResponseWriter doesn't support flushing.
// Returns any error that occurs during streaming.
//
// Example:
//
//	events := StreamSSE(upstreamResp.Body)
//	err := ForwardSSE(events, w)
//	if err != nil {
//	    // Handle error
//	}
func ForwardSSE(events ro.Observable[SSEEvent], w http.ResponseWriter) error {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		return ErrNotFlushable
	}

	errCh := make(chan error, 1)

	events.Subscribe(ro.NewObserver(
		func(event SSEEvent) {
			if _, err := w.Write(event.Bytes()); err != nil {
				errCh <- err
				return
			}
			flusher.Flush()
		},
		func(err error) {
			errCh <- err
		},
		func() {
			close(errCh)
		},
	))

	return <-errCh
}

// WriteSSEEvent writes a single SSE event to an http.ResponseWriter.
// Returns an error if the write fails or the writer doesn't support flushing.
//
// This is a convenience function for writing individual events without
// creating an observable stream.
//
// Example:
//
//	event := SSEEvent{Event: "message", Data: []byte("Hello")}
//	if err := WriteSSEEvent(w, event); err != nil {
//	    // Handle error
//	}
func WriteSSEEvent(w http.ResponseWriter, event SSEEvent) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return ErrNotFlushable
	}

	if _, err := w.Write(event.Bytes()); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

// Note: SetSSEHeaders is defined in sse.go and takes http.Header.
// Use SetSSEHeaders(w.Header()) to set headers on a ResponseWriter.

// FilterEvents creates an operator that filters SSE events by type.
// Events with a matching Event field are passed through, others are dropped.
//
// Example:
//
//	// Only keep message_delta events
//	filtered := ro.Pipe1(events, FilterEvents("message_delta"))
func FilterEvents(eventType string) func(ro.Observable[SSEEvent]) ro.Observable[SSEEvent] {
	return ro.Filter(func(e SSEEvent) bool {
		return e.Event == eventType
	})
}

// FilterEventsByPrefix creates an operator that filters SSE events by type prefix.
//
// Example:
//
//	// Keep all content_block_* events
//	filtered := ro.Pipe1(events, FilterEventsByPrefix("content_block_"))
func FilterEventsByPrefix(prefix string) func(ro.Observable[SSEEvent]) ro.Observable[SSEEvent] {
	return ro.Filter(func(e SSEEvent) bool {
		return strings.HasPrefix(e.Event, prefix)
	})
}

// MapEventData transforms the data field of each SSE event.
//
// Example:
//
//	// Add prefix to all event data
//	transformed := ro.Pipe1(events, MapEventData(func(data []byte) []byte {
//	    return append([]byte("prefix:"), data...)
//	}))
func MapEventData(mapper func([]byte) []byte) func(ro.Observable[SSEEvent]) ro.Observable[SSEEvent] {
	return ro.Map(func(e SSEEvent) SSEEvent {
		e.Data = mapper(e.Data)
		return e
	})
}

// CountEvents creates an operator that counts events and emits the running total.
// Useful for monitoring stream progress.
func CountEvents() func(ro.Observable[SSEEvent]) ro.Observable[int64] {
	return func(source ro.Observable[SSEEvent]) ro.Observable[int64] {
		return ro.Pipe1(source, ro.Count[SSEEvent]())
	}
}
