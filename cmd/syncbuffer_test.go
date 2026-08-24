package cmd

import (
	"bytes"
	"sync"
)

// syncBuffer protects test log access because RunServe leaves background
// goroutines running — the scheduler and the sync-error reporter, among others
// — that keep logging after the call returns and while the test reads what was
// logged. zerolog.SyncWriter only serializes writers, so it would not protect
// the test's String read.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
