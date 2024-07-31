package pipe

import (
	"sync"
)

func NewBytesPool(size int) *sync.Pool {
	sp := &sync.Pool{}
	sp.New = func() interface{} {
		buf := make([]byte, size)
		return buf
	}
	return sp
}
