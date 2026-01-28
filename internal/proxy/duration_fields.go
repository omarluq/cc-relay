package proxy

import (
	"time"

	"github.com/rs/zerolog"
)

// addDurationFields logs a human-friendly duration with dynamic precision.
func addDurationFields(event *zerolog.Event, name string, d time.Duration) *zerolog.Event {
	if d <= 0 {
		return event
	}
	return event.Str(name, formatDuration(d))
}

// addDurationFieldsCtx logs a human-friendly duration with dynamic precision.
func addDurationFieldsCtx(ctx *zerolog.Context, name string, d time.Duration) {
	if d <= 0 {
		return
	}
	*ctx = ctx.Str(name, formatDuration(d))
}
