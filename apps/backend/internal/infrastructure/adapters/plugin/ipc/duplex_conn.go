package ipc

import (
	"io"
	"sync"
)

// DuplexConn adapts independent reader/writer handles into one io.ReadWriteCloser.
// This is useful for stdio and extra pipe pairs where read and write ends are
// distinct file descriptors.
type DuplexConn struct {
	r         io.ReadCloser
	w         io.WriteCloser
	closeOnce sync.Once
}

// NewDuplexConn builds a new duplex connection from separated read/write ends.
func NewDuplexConn(r io.ReadCloser, w io.WriteCloser) *DuplexConn {
	return &DuplexConn{r: r, w: w}
}

// Read implements io.Reader.
func (c *DuplexConn) Read(p []byte) (int, error) {
	return c.r.Read(p)
}

// Write implements io.Writer.
func (c *DuplexConn) Write(p []byte) (int, error) {
	return c.w.Write(p)
}

// Close closes both sides exactly once.
func (c *DuplexConn) Close() error {
	var closeErr error
	c.closeOnce.Do(func() {
		if err := c.r.Close(); err != nil {
			closeErr = err
		}
		if err := c.w.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
	})
	return closeErr
}
