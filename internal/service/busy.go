package service

import "sync"

type busySet struct {
	mu  sync.Mutex
	ids map[int64]struct{}
}

func newBusySet() *busySet {
	return &busySet{ids: map[int64]struct{}{}}
}

func (b *busySet) tryAcquire(id int64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.ids[id]; ok {
		return false
	}
	b.ids[id] = struct{}{}
	return true
}

func (b *busySet) release(id int64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.ids, id)
}
