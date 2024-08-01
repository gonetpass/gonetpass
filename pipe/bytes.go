package pipe

import (
	"sync"
)

type BytesPool struct {
	size int
	pool *sync.Pool
}

func (b *BytesPool) Get() (p []byte) {
	p = b.pool.Get().([]byte)
	if cap(p) != b.size {
		return b.Get()
	}
	return p
}

func (b *BytesPool) Put(p []byte) {
	if cap(p) != b.size {
		return
	}
	b.pool.Put(p)
}

func NewBytesPool(size int) *BytesPool {
	b := &BytesPool{size: size, pool: &sync.Pool{}}
	b.pool.New = func() interface{} {
		buf := make([]byte, size)
		return buf
	}
	return b
}
