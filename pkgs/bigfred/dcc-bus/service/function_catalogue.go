package service

import (
	"sync"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

// FunctionCatalogueCache holds the in-memory vehicle_functions snapshot
// published by loco-server over Redis.
type FunctionCatalogueCache struct {
	layoutID uint
	mu       sync.RWMutex
	byAddr   map[uint16][]contract.FunctionDefinition
}

// NewFunctionCatalogueCache returns a cache bound to one layout daemon.
func NewFunctionCatalogueCache(layoutID uint) *FunctionCatalogueCache {
	return &FunctionCatalogueCache{
		layoutID: layoutID,
		byAddr:   make(map[uint16][]contract.FunctionDefinition, 16),
	}
}

// ApplySnapshot replaces the catalogue from a loco-server snapshot.
func (c *FunctionCatalogueCache) ApplySnapshot(snap contract.VehicleFunctions) bool {
	if snap.LayoutID != 0 && snap.LayoutID != c.layoutID {
		return false
	}
	byAddr := make(map[uint16][]contract.FunctionDefinition, len(snap.Vehicles))
	for _, v := range snap.Vehicles {
		fns := append([]contract.FunctionDefinition(nil), v.Functions...)
		byAddr[v.Addr] = fns
	}
	c.mu.Lock()
	c.byAddr = byAddr
	c.mu.Unlock()
	return true
}

// FunctionsForAddr returns resolved function metadata for one DCC address.
func (c *FunctionCatalogueCache) FunctionsForAddr(addr uint16) []contract.FunctionDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()
	fns := c.byAddr[addr]
	if len(fns) == 0 {
		return nil
	}
	out := make([]contract.FunctionDefinition, len(fns))
	copy(out, fns)
	return out
}

// Snapshot returns the current catalogue view.
func (c *FunctionCatalogueCache) Snapshot() contract.VehicleFunctions {
	c.mu.RLock()
	defer c.mu.RUnlock()
	vehicles := make([]contract.VehicleFunctionCatalogue, 0, len(c.byAddr))
	for addr, fns := range c.byAddr {
		vehicles = append(vehicles, contract.VehicleFunctionCatalogue{
			Addr:      addr,
			Functions: append([]contract.FunctionDefinition(nil), fns...),
		})
	}
	return contract.VehicleFunctions{
		LayoutID: c.layoutID,
		Vehicles: vehicles,
	}
}
