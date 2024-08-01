package bytes

import (
	"bytes"
	"context"
	"io"
	"sync"
	"time"
)

type Buffer struct {
	src         *bytes.Buffer
	readTimeout time.Duration
	mu          sync.Mutex
	ctx         context.Context
	cancel      context.CancelFunc
}

func NewBuffer(readTimeout time.Duration) *Buffer {
	ctx, cancel := context.WithCancel(context.Background())
	return &Buffer{
		src:         &bytes.Buffer{},
		readTimeout: readTimeout,
		ctx:         ctx,
		cancel:      cancel,
	}
}

func (r *Buffer) write(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.src.Write(p)
}

func (r *Buffer) read(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.src.Read(p)
}

func (r *Buffer) Write(p []byte) (n int, err error) {
	return r.write(p)
}

func (r *Buffer) Done() {
	r.cancel()
}

func (r *Buffer) Read(p []byte) (n int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.readTimeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
			n, err = r.read(p)
			if err != nil && err != io.EOF {
				return 0, err
			}
			if n > 0 {
				return n, nil
			}
			select {
			case <-r.ctx.Done():
				return 0, nil
			default:
				time.Sleep(time.Millisecond)
			}
		}
	}
}
