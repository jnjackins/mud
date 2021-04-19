package tail

import (
	"context"
	"io"
	"time"
)

type Reader struct {
	io.Reader
	ctx      context.Context
	interval time.Duration
}

func (r *Reader) Read(p []byte) (int, error) {
	for r.ctx.Err() == nil {
		if n, err := r.Reader.Read(p); err != io.EOF {
			return n, err
		}
		time.Sleep(r.interval)
	}
	return 0, r.ctx.Err()
}

func New(ctx context.Context, r io.Reader, interval time.Duration) *Reader {
	return &Reader{
		Reader:   r,
		ctx:      ctx,
		interval: interval,
	}
}
