package proxy

import (
	"context"
	"time"
)

type requestTimings struct {
	Auth    time.Duration
	Routing time.Duration
}

type timingsKey struct{}

func withRequestTimings(ctx context.Context) (context.Context, *requestTimings) {
	timings := &requestTimings{}
	return context.WithValue(ctx, timingsKey{}, timings), timings
}

func getRequestTimings(ctx context.Context) *requestTimings {
	if ctx == nil {
		return nil
	}
	if timings, ok := ctx.Value(timingsKey{}).(*requestTimings); ok {
		return timings
	}
	return nil
}
