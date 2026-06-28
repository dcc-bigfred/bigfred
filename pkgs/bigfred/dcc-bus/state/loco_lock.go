package state

import "sync"

// LocoLocks serializes per-locomotive read-modify-write of Redis snapshots
// so the command path and the observer feed cannot race on the same address.
type LocoLocks struct {
	mu    sync.Mutex
	locks map[uint16]*sync.Mutex
}

// NewLocoLocks returns an empty lock table.
func NewLocoLocks() *LocoLocks {
	return &LocoLocks{locks: make(map[uint16]*sync.Mutex, 32)}
}

// Acquire locks addr and returns an unlock function.
func (l *LocoLocks) Acquire(addr uint16) func() {
	l.mu.Lock()
	m, ok := l.locks[addr]
	if !ok {
		m = &sync.Mutex{}
		l.locks[addr] = m
	}
	l.mu.Unlock()
	m.Lock()
	return m.Unlock
}
