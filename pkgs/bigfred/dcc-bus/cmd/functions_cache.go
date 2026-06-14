package cmd

import "sync"

// fnKey identifies one DCC function bit tracked in FunctionsCache and
// timed-pulse state.
type fnKey struct {
	Addr uint16
	Fn   uint8
}

// FunctionsCache mirrors the last sent function bit per (addr, fn) so a
// rapid toggle does not reissue the same DCC packet.
type FunctionsCache struct {
	mu      sync.Mutex
	entries map[fnKey]bool
}

// NewFunctionsCache returns an empty function dedup cache.
func NewFunctionsCache() *FunctionsCache {
	return &FunctionsCache{entries: make(map[fnKey]bool, 32)}
}

// Seed aligns the cache with an authoritative function vector (e.g. a
// Redis snapshot on subscribe).
func (c *FunctionsCache) Seed(addr uint16, functions []bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for fn, on := range functions {
		if fn > maxDCCFunctionNum {
			break
		}
		c.entries[fnKey{Addr: addr, Fn: uint8(fn)}] = on
	}
}

// Get reports the cached on/off state for one function. ok is false when
// the entry is absent.
func (c *FunctionsCache) Get(addr uint16, fn uint8) (on bool, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	on, ok = c.entries[fnKey{Addr: addr, Fn: fn}]
	return on, ok
}

// Matches reports whether the cache alone believes addr/fn is already on.
func (c *FunctionsCache) Matches(addr uint16, fn uint8, on bool) bool {
	prev, ok := c.getLocked(addr, fn)
	return ok && prev == on
}

// Stage records the desired on/off state and returns the prior entry for
// rollback after a failed DCC write.
func (c *FunctionsCache) Stage(addr uint16, fn uint8, on bool) (previous bool, hadPrev bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := fnKey{Addr: addr, Fn: fn}
	previous, hadPrev = c.entries[key]
	c.entries[key] = on
	return previous, hadPrev
}

// Rollback restores the prior cache entry after Stage when SendFn fails.
func (c *FunctionsCache) Rollback(addr uint16, fn uint8, previous bool, hadPrev bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := fnKey{Addr: addr, Fn: fn}
	if hadPrev {
		c.entries[key] = previous
	} else {
		delete(c.entries, key)
	}
}

// Set unconditionally records one function bit.
func (c *FunctionsCache) Set(addr uint16, fn uint8, on bool) {
	c.mu.Lock()
	c.entries[fnKey{Addr: addr, Fn: fn}] = on
	c.mu.Unlock()
}

// ClearAddr drops every cached function bit for one locomotive address.
func (c *FunctionsCache) ClearAddr(addr uint16) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.entries {
		if k.Addr == addr {
			delete(c.entries, k)
		}
	}
}

func (c *FunctionsCache) getLocked(addr uint16, fn uint8) (on bool, ok bool) {
	on, ok = c.entries[fnKey{Addr: addr, Fn: fn}]
	return on, ok
}
