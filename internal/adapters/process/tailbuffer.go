package process

import (
	"bytes"
	"sync"
)

// tailBuffer is an io.Writer that keeps at most limit bytes, discarding the
// oldest ones. It is safe for concurrent writes because os/exec may copy
// stdout and stderr from separate goroutines into the same writer.
type tailBuffer struct {
	mu        sync.Mutex
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func newTailBuffer(limit int) *tailBuffer {
	return &tailBuffer{limit: limit}
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	n := len(p)
	if n >= t.limit {
		t.buf.Reset()
		t.buf.Write(p[n-t.limit:])
		t.truncated = true
		return n, nil
	}
	if overflow := t.buf.Len() + n - t.limit; overflow > 0 {
		t.buf.Next(overflow)
		t.truncated = true
	}
	t.buf.Write(p)
	return n, nil
}

func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.truncated {
		return "[... output truncated ...]\n" + t.buf.String()
	}
	return t.buf.String()
}
