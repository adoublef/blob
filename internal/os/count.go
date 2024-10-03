package os

import (
	"io"
	"sync/atomic"
)

type countReader struct {
	n atomic.Int64
	r io.Reader
}

func (c *countReader) Read(p []byte) (int, error) {
	nr, err := c.r.Read(p)
	if nr > 0 {
		c.n.Add(int64(nr))
	}
	return nr, err
}
