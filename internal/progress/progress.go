package progress

import (
	"io"
	"sync/atomic"
)

// Reader wraps an io.Reader to track the number of bytes read.
// It is safe for concurrent access.
type Reader struct {
	reader    io.Reader
	bytesRead atomic.Int64
	total     int64
}

func NewReader(r io.Reader, total int64) *Reader {
	return &Reader{
		reader: r,
		total:  total,
	}
}

func (pr *Reader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.bytesRead.Add(int64(n))
	return n, err
}

func (pr *Reader) BytesRead() int64 {
	return pr.bytesRead.Load()
}

func (pr *Reader) Total() int64 {
	return pr.total
}

// Indeterminate is true when total size is unknown (non-regular file, pipe, etc.).
func (pr *Reader) Indeterminate() bool {
	return pr.total < 0
}

// Percent returns a value between 0.0 and 1.0. For indeterminate readers, always 0.
func (pr *Reader) Percent() float64 {
	if pr.total <= 0 {
		return 0
	}
	p := float64(pr.bytesRead.Load()) / float64(pr.total)
	if p > 1.0 {
		return 1.0
	}
	return p
}
