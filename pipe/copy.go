package pipe

import (
	"context"
	"errors"
	"io"
	"sync/atomic"
	"time"
)

func NewBeat() *uint64 {
	beat := uint64(0)
	return &beat
}

var (
	buffer = NewBytesPool(CopyBuf)
)

func Copy(ctx context.Context, dst io.Writer, src io.Reader, readTimeout time.Duration, beat *uint64) (written int64, err error) {
	readErr := make(chan error)
	done, cancel := context.WithCancel(context.Background())
	ticker := time.NewTicker(readTimeout)
	e := NewExec()
	defer e.Exec(func() {
		cancel()
		close(readErr)
		ticker.Stop()
	})
	go func() {
		buf := buffer.Get().([]byte)
		defer func() {
			cancel()
			buffer.Put(buf)
		}()
		var rErr, wErr error
		n, nw := 0, 0
		for {
			select {
			case <-done.Done():
				return
			case <-ctx.Done():
				return
			default:

			}
			n, rErr = src.Read(buf)
			if n > 0 {
				select {
				case <-done.Done():
					return
				case <-ctx.Done():
					return
				default:
					ticker.Reset(readTimeout)
					if beat != nil {
						//read到数据，beat+1
						atomic.AddUint64(beat, 1)
					}
					nw, wErr = dst.Write(buf[:n])
					if nw != n {
						wErr = io.ErrShortWrite
					}
					e.Exec(func() {
						select {
						case <-done.Done():
							return
						case <-ctx.Done():
							return
						default:
							written += int64(nw)
							if wErr != nil {
								readErr <- wErr
								return
							}
						}
					})
				}
			}
			if rErr != nil {
				e.Exec(func() {
					select {
					case <-done.Done():
						return
					case <-ctx.Done():
						return
					default:
						readErr <- rErr
					}
				})
				return
			}
		}
	}()
	var old uint64
	for {
		if beat != nil {
			old = atomic.LoadUint64(beat)
		}
		ticker.Reset(readTimeout)
		select {
		case err = <-readErr:
			return
		case <-done.Done():
			err = done.Err()
			return
		case <-ctx.Done():
			err = ctx.Err()
			return
		case <-ticker.C:
			//心跳机制，当超时的时候，基于beat的其他协程有跳动，就不退出
			if beat != nil && atomic.LoadUint64(beat) > old {
				continue
			}
			err = errors.New("timeout")
			return
		}
	}
}
