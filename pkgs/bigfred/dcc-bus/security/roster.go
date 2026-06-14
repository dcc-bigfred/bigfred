package security

import (
	"sync"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

// RosterGate evaluates subscribe and drive commands against the
// in-memory allowed_vehicles snapshot (§7e.3 / §7a.3).
type RosterGate struct {
	layoutID uint
	mu       sync.RWMutex
	allowed  map[uint16]struct{}
	byAddr   map[uint16]contract.AllowedVehicle
}

// NewRosterGate returns a gate bound to one layout daemon instance.
func NewRosterGate(layoutID uint) *RosterGate {
	return &RosterGate{
		layoutID: layoutID,
		allowed:  make(map[uint16]struct{}, 16),
		byAddr:   make(map[uint16]contract.AllowedVehicle, 16),
	}
}

// ApplySnapshot replaces the drivable roster from a loco-server
// snapshot. Returns false when the snapshot targets another layout.
func (g *RosterGate) ApplySnapshot(snap contract.AllowedVehicles) bool {
	if snap.LayoutID != 0 && snap.LayoutID != g.layoutID {
		return false
	}
	allowed := make(map[uint16]struct{}, len(snap.Vehicles))
	byAddr := make(map[uint16]contract.AllowedVehicle, len(snap.Vehicles))
	for _, v := range snap.Vehicles {
		allowed[v.Addr] = struct{}{}
		byAddr[v.Addr] = v
	}
	g.mu.Lock()
	g.allowed = allowed
	g.byAddr = byAddr
	g.mu.Unlock()
	return true
}

// AllowedAddrs returns every DCC address on the layout roster.
func (g *RosterGate) AllowedAddrs() []uint16 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	addrs := make([]uint16, 0, len(g.allowed))
	for addr := range g.allowed {
		addrs = append(addrs, addr)
	}
	return addrs
}

// AllowedVehicle returns catalogue metadata for one roster address.
func (g *RosterGate) AllowedVehicle(addr uint16) (contract.AllowedVehicle, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	v, ok := g.byAddr[addr]
	return v, ok
}

// IsLocoAllowedOnLayout reports whether addr is on the layout roster.
func (g *RosterGate) IsLocoAllowedOnLayout(addr uint16) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	_, ok := g.allowed[addr]
	return ok
}

// UserCanControlLoco reports whether userID may drive the locomotive at
// addr, using controllerUserIds from the allowed_vehicles snapshot.
func (g *RosterGate) UserCanControlLoco(userID uint, addr uint16) bool {
	g.mu.RLock()
	v, ok := g.byAddr[addr]
	g.mu.RUnlock()
	if !ok {
		return false
	}
	for _, id := range v.ControllerUserIDs {
		if id == userID {
			return true
		}
	}
	return false
}

// CanDrive returns whether userID may issue throttle commands for addr.
func (g *RosterGate) CanDrive(userID uint, addr uint16) Decision {
	if !g.IsLocoAllowedOnLayout(addr) {
		return Deny(ReasonVehicleNotOnLayout)
	}
	if !g.UserCanControlLoco(userID, addr) {
		return Deny(ReasonNotAuthorized)
	}
	return Allow
}

// DenyDriveCommand returns a non-empty ack reason when the user may
// not issue drive commands for addr (roster membership or control rights).
func (g *RosterGate) DenyDriveCommand(userID uint, addr uint16) string {
	if d := g.CanDrive(userID, addr); !d.Allowed {
		return d.Reason
	}
	return ""
}
