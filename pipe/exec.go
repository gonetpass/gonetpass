package pipe

import "sync"

type Exec struct {
	mu sync.Mutex
}

func NewExec() *Exec {
	return &Exec{
		mu: sync.Mutex{},
	}
}

func (e *Exec) Exec(f func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	f()
}
