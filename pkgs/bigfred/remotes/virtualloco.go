package remotes

import (
	"sync"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

const virtualLocoFunctionCount = 32

// VirtualLocoStore holds per-(client, address) simulated locomotive state for
// unpaired handsets. It is protocol-agnostic: Z21 and WiThrottle gateways both
// use it so an unpaired handheld shows a working throttle during pairing
// (speed, direction, functions) without touching the DCC bus.
type VirtualLocoStore struct {
	mu sync.Mutex
	m  map[string]map[uint16]*virtualLoco
}

type virtualLoco struct {
	speed     uint8
	forward   bool
	functions [virtualLocoFunctionCount]bool
}

// NewVirtualLocoStore returns an empty simulated-loco store.
func NewVirtualLocoStore() *VirtualLocoStore {
	return &VirtualLocoStore{m: make(map[string]map[uint16]*virtualLoco)}
}

func (s *VirtualLocoStore) at(clientKey string, addr uint16) *virtualLoco {
	locos, ok := s.m[clientKey]
	if !ok {
		locos = make(map[uint16]*virtualLoco)
		s.m[clientKey] = locos
	}
	st, ok := locos[addr]
	if !ok {
		st = &virtualLoco{forward: true}
		locos[addr] = st
	}
	return st
}

// Snapshot returns the simulated state for (clientKey, addr), defaulting to
// stopped/forward when the handset has not touched this address yet.
func (s *VirtualLocoStore) Snapshot(clientKey string, addr uint16) contract.LocoStateWire {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.at(clientKey, addr).toWire(addr)
}

// SetSpeed records simulated speed/direction and returns the new snapshot.
func (s *VirtualLocoStore) SetSpeed(clientKey string, addr uint16, speed uint8, forward bool) contract.LocoStateWire {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.at(clientKey, addr)
	st.speed = speed
	st.forward = forward
	return st.toWire(addr)
}

// SetFunction toggles one simulated function and returns the new snapshot.
func (s *VirtualLocoStore) SetFunction(clientKey string, addr uint16, fn int, on bool) contract.LocoStateWire {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.at(clientKey, addr)
	if fn >= 0 && fn < len(st.functions) {
		st.functions[fn] = on
	}
	return st.toWire(addr)
}

// ToggleFunction flips one simulated function and returns the new snapshot.
func (s *VirtualLocoStore) ToggleFunction(clientKey string, addr uint16, fn int) contract.LocoStateWire {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.at(clientKey, addr)
	if fn >= 0 && fn < len(st.functions) {
		st.functions[fn] = !st.functions[fn]
	}
	return st.toWire(addr)
}

// RemoveClient drops all simulated locos for a client (called on eviction or pairing).
func (s *VirtualLocoStore) RemoveClient(clientKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, clientKey)
}

// HasClient reports whether clientKey has any simulated loco state.
func (s *VirtualLocoStore) HasClient(clientKey string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.m[clientKey]
	return ok
}

func (l *virtualLoco) toWire(addr uint16) contract.LocoStateWire {
	fns := make([]bool, len(l.functions))
	copy(fns, l.functions[:])
	return contract.LocoStateWire{
		Address:   addr,
		Speed:     l.speed,
		Forward:   l.forward,
		Functions: fns,
	}
}
