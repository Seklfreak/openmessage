package client

import "context"

// GMContext returns the context to hand to libgm phone requests.
//
// Upstream libgm takes a context.Context on every phone request (mautrix
// c0a2d38a24dc) so callers can cancel one in flight. OpenMessage's seams —
// backfill, the MCP tool handlers, the bridge adapter — are shaped around the
// App and carry no request-scoped context to thread down, and libgm still
// applies its own 60s hard timeout per request and cancels outstanding waiters
// when the client disconnects. Until those seams grow a context, this is the
// single documented place that decision lives.
func GMContext() context.Context { return context.Background() }
