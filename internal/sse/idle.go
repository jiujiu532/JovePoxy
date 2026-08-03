package sse

import (
	"fmt"
	"io"
	"time"
)

// DefaultIdleTimeout is the default max silence between upstream stream reads.
// After this long with no data, adapters fail the stream instead of hanging forever.
const DefaultIdleTimeout = 10 * time.Minute

// IdleReader wraps r and returns an error when no bytes arrive within idle.
// A non-positive idle disables the timeout and returns r unchanged.
//
// Implementation copies through an io.Pipe so the caller's buffer is never
// shared with a timed-out Read goroutine. On idle timeout the pipe is closed
// with error; the background Read may outlive this function until the
// underlying reader unblocks (acceptable for rare hang recovery).
func IdleReader(r io.Reader, idle time.Duration) io.Reader {
	if r == nil || idle <= 0 {
		return r
	}
	pr, pw := io.Pipe()
	go copyWithIdle(pw, r, idle)
	return pr
}

func copyWithIdle(pw *io.PipeWriter, r io.Reader, idle time.Duration) {
	type result struct {
		buf []byte
		n   int
		err error
	}
	for {
		done := make(chan result, 1)
		go func() {
			// Unique buffer per Read so a timed-out goroutine cannot race later iterations.
			buf := make([]byte, 32*1024)
			n, err := r.Read(buf)
			done <- result{buf: buf, n: n, err: err}
		}()
		timer := time.NewTimer(idle)
		select {
		case <-timer.C:
			_ = pw.CloseWithError(fmt.Errorf("stream idle timeout after %s", idle))
			// Leave Read goroutine; it will finish when upstream unblocks.
			return
		case res := <-done:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			if res.n > 0 {
				if _, err := pw.Write(res.buf[:res.n]); err != nil {
					return
				}
			}
			if res.err != nil {
				if res.err == io.EOF {
					_ = pw.Close()
					return
				}
				_ = pw.CloseWithError(res.err)
				return
			}
		}
	}
}
