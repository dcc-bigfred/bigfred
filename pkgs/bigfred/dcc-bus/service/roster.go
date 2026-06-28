// Package service holds helper structs reused by cmd handlers: roster
// cache, DCC writers, function dedup, and the external state feed.
package service

import (
	"sync"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

// RosterCache holds the in-memory allowed_vehicles snapshot published by
// loco-server over Redis (§7e.3).
type RosterCache struct {
	layoutID uint
	mu       sync.RWMutex
	allowed  map[uint16]struct{}
	byAddr   map[uint16]contract.AllowedVehicle
}

// NewRosterCache returns a cache bound to one layout daemon instance.
func NewRosterCache(layoutID uint) *RosterCache {
	return &RosterCache{
		layoutID: layoutID,
		allowed:  make(map[uint16]struct{}, 16),
		byAddr:   make(map[uint16]contract.AllowedVehicle, 16),
	}
}

// ApplySnapshot replaces the drivable roster from a loco-server snapshot.
// Returns false when the snapshot targets another layout.
func (c *RosterCache) ApplySnapshot(snap contract.AllowedVehicles) bool {
	if snap.LayoutID != 0 && snap.LayoutID != c.layoutID {
		return false
	}
	allowed := make(map[uint16]struct{}, len(snap.Vehicles))
	byAddr := make(map[uint16]contract.AllowedVehicle, len(snap.Vehicles))
	for _, v := range snap.Vehicles {
		allowed[v.Addr] = struct{}{}
		byAddr[v.Addr] = v
	}
	c.mu.Lock()
	c.allowed = allowed
	c.byAddr = byAddr
	c.mu.Unlock()
	return true
}

// DiffRemoved returns DCC addresses on the current roster that are absent
// from snap. Call before ApplySnapshot to retire locos falling off the layout.
func (c *RosterCache) DiffRemoved(snap contract.AllowedVehicles) []uint16 {
	if snap.LayoutID != 0 && snap.LayoutID != c.layoutID {
		return nil
	}
	next := make(map[uint16]struct{}, len(snap.Vehicles))
	for _, v := range snap.Vehicles {
		next[v.Addr] = struct{}{}
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	removed := make([]uint16, 0)
	for addr := range c.allowed {
		if _, keep := next[addr]; !keep {
			removed = append(removed, addr)
		}
	}
	return removed
}

// AllowedAddrs returns every DCC address on the layout roster.
func (c *RosterCache) AllowedAddrs() []uint16 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	addrs := make([]uint16, 0, len(c.allowed))
	for addr := range c.allowed {
		addrs = append(addrs, addr)
	}
	return addrs
}

// AllowedVehicle returns catalogue metadata for one roster address.
func (c *RosterCache) AllowedVehicle(addr uint16) (contract.AllowedVehicle, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.byAddr[addr]
	return v, ok
}

// Snapshot returns the current allowed-vehicles view for roster emission.
func (c *RosterCache) Snapshot() contract.AllowedVehicles {
	c.mu.RLock()
	defer c.mu.RUnlock()
	vehicles := make([]contract.AllowedVehicle, 0, len(c.byAddr))
	for _, v := range c.byAddr {
		vehicles = append(vehicles, v)
	}
	return contract.AllowedVehicles{
		LayoutID: c.layoutID,
		Vehicles: vehicles,
	}
}

// IsOnLayout reports whether addr is on the layout roster.
func (c *RosterCache) IsOnLayout(addr uint16) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.allowed[addr]
	return ok
}
