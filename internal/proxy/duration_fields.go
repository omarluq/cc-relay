package proxy

import (
	"time"

	"github.com/rs/zerolog"
)

// addDurationFields logs an exact microsecond value plus a human-friendly duration.
func addDurationFields(event *zerolog.Event, name string, d time.Duration) *zerolog.Event {
	if d <= 0 {
		return event
	}
	event = event.Int64(name+"_us", d.Microseconds())
	return event.Str(name, d.String())
}

// addDurationFieldsCtx logs an exact microsecond value plus a human-friendly duration.
func addDurationFieldsCtx(ctx *zerolog.Context, name string, d time.Duration) {
	if d <= 0 {
		return
	}
	*ctx = ctx.Int64(name+"_us", d.Microseconds())
	*ctx = ctx.Str(name, d.String())
}
