package state

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

const (
	obsFnRange              = 28
	commandSuppressWindow   = 750 * time.Millisecond
)

type locoStateRedis interface {
	GetLocoCurrentState(ctx context.Context, addr uint16) (contract.LocoStateWire, bool, error)
	StoreLocoCurrentState(ctx context.Context, snap contract.LocoStateWire, ttl time.Duration) error
}

// LocoStateStore is the in-memory authoritative loco state. Redis is a
// write-behind mirror flushed on a tick; estop uses priority flush.
type LocoStateStore struct {
	mu      sync.Mutex
	entries map[uint16]*locoEntry
	redis   locoStateRedis
	ttl     time.Duration
	dirty   map[uint16]struct{}
	dirtyMu sync.Mutex
	log     *logrus.Logger
}

type locoEntry struct {
	mu             sync.Mutex
	snap           contract.LocoStateWire
	commandedSpeed uint8
	commandedFwd   bool
	commandedAt    time.Time
}

// NewLocoStateStore returns an empty authoritative store.
func NewLocoStateStore(redis locoStateRedis, ttl time.Duration, log *logrus.Logger) *LocoStateStore {
	return &LocoStateStore{
		entries: make(map[uint16]*locoEntry, 64),
		redis:   redis,
		ttl:     ttl,
		dirty:   make(map[uint16]struct{}, 64),
		log:     log,
	}
}

// LoadMissingFromRedis seeds store entries for addresses that have no
// in-memory entry yet. Existing authoritative entries are left untouched.
func (s *LocoStateStore) LoadMissingFromRedis(ctx context.Context, addrs []uint16) {
	if s == nil || s.redis == nil {
		return
	}
	for _, addr := range addrs {
		s.mu.Lock()
		_, exists := s.entries[addr]
		s.mu.Unlock()
		if exists {
			continue
		}
		snap, ok, err := s.redis.GetLocoCurrentState(ctx, addr)
		if err != nil || !ok {
			continue
		}
		s.mu.Lock()
		if _, dup := s.entries[addr]; !dup {
			s.entries[addr] = &locoEntry{snap: snap}
		}
		s.mu.Unlock()
	}
}

// Snapshot returns a copy of addr's state (Forward=true when unknown).
func (s *LocoStateStore) Snapshot(addr uint16) contract.LocoStateWire {
	if s == nil {
		return contract.LocoStateWire{Address: addr, Forward: true}
	}
	s.mu.Lock()
	e, ok := s.entries[addr]
	s.mu.Unlock()
	if !ok {
		return contract.LocoStateWire{Address: addr, Forward: true}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.snap
}

// SetSpeed writes a command-authored speed and returns the new snapshot.
func (s *LocoStateStore) SetSpeed(addr uint16, speed uint8, forward bool, userID uint, source string) contract.LocoStateWire {
	e := s.entry(addr)
	e.mu.Lock()
	e.snap.Address = addr
	e.snap.Speed = speed
	e.snap.Forward = forward
	e.snap.ControlledByUserID = userID
	e.snap.Source = source
	e.snap.At = time.Now().UTC().UnixMilli()
	e.commandedSpeed = speed
	e.commandedFwd = forward
	e.commandedAt = time.Now()
	out := e.snap
	e.mu.Unlock()
	s.markDirty(addr)
	return out
}

// SetSpeedPreservingUser writes a command-authored speed without changing
// the controlling user. Use for server-originated commands that must not
// race a concurrent observation's userID reset.
func (s *LocoStateStore) SetSpeedPreservingUser(addr uint16, speed uint8, forward bool, source string) contract.LocoStateWire {
	e := s.entry(addr)
	e.mu.Lock()
	e.snap.Address = addr
	e.snap.Speed = speed
	e.snap.Forward = forward
	e.snap.Source = source
	e.snap.At = time.Now().UTC().UnixMilli()
	e.commandedSpeed = speed
	e.commandedFwd = forward
	e.commandedAt = time.Now()
	out := e.snap
	e.mu.Unlock()
	s.markDirty(addr)
	return out
}

// SetFunction toggles one function bit and returns the new snapshot.
// Pass userID == 0 to preserve the current controlling user.
func (s *LocoStateStore) SetFunction(addr uint16, userID uint, fn uint8, on bool, source string) contract.LocoStateWire {
	e := s.entry(addr)
	e.mu.Lock()
	e.snap.Address = addr
	if userID != 0 {
		e.snap.ControlledByUserID = userID
	}
	if len(e.snap.Functions) <= int(fn) {
		grown := make([]bool, fn+1)
		copy(grown, e.snap.Functions)
		e.snap.Functions = grown
	}
	e.snap.Functions[fn] = on
	e.snap.Source = source
	e.snap.At = time.Now().UTC().UnixMilli()
	out := e.snap
	e.mu.Unlock()
	s.markDirty(addr)
	return out
}

// SetFunctionPreservingUser toggles one function bit without changing the
// controlling user. Use for server-originated function commands so a
// concurrent observation cannot race the userID read/write.
func (s *LocoStateStore) SetFunctionPreservingUser(addr uint16, fn uint8, on bool, source string) contract.LocoStateWire {
	return s.SetFunction(addr, 0, fn, on, source)
}

// ApplyObservation merges a passive bus observation; returns (snapshot, changed).
func (s *LocoStateStore) ApplyObservation(o commandstation.LocoObservation, source string) (contract.LocoStateWire, bool) {
	addr := uint16(o.Addr)
	e := s.entry(addr)
	e.mu.Lock()
	changed := false

	// Capture whether BigFred's command window is active before any
	// suppression logic clears it. While the window is active, bus
	// echoes (motion or function) are not an external takeover and
	// must not drop throttle ownership.
	windowActive := !e.commandedAt.IsZero()

	suppressMotion := false
	if !e.commandedAt.IsZero() && (o.HasSpeed || o.HasForward) {
		switch {
		case time.Since(e.commandedAt) >= commandSuppressWindow:
			e.commandedAt = time.Time{}
		case (o.HasSpeed && contract.UISpeedFromWire(o.Speed) != e.commandedSpeed) ||
			(o.HasForward && o.Forward != e.commandedFwd):
			suppressMotion = true
		// Only release the window when the bus confirms BOTH commanded
		// dimensions. LocoNet emits speed and direction in separate frames
		// (OPC_LOCO_SPD / OPC_LOCO_DIRF); a partial echo that matches one
		// field must not clear suppression before the other arrives, or a
		// stale second frame could still snap the lever back.
		case o.HasSpeed && o.HasForward &&
			contract.UISpeedFromWire(o.Speed) == e.commandedSpeed &&
			o.Forward == e.commandedFwd:
			e.commandedAt = time.Time{}
		}
	}

	if !suppressMotion && o.HasSpeed {
		obs := contract.UISpeedFromWire(o.Speed)
		if e.snap.Speed != obs {
			e.snap.Speed = obs
			changed = true
		}
	}
	if !suppressMotion && o.HasForward && e.snap.Forward != o.Forward {
		e.snap.Forward = o.Forward
		changed = true
	}
	if o.FunctionMask != 0 {
		for fn := 0; fn <= obsFnRange; fn++ {
			bit := uint32(1) << uint(fn)
			if o.FunctionMask&bit == 0 {
				continue
			}
			on := o.FunctionBits&bit != 0
			if len(e.snap.Functions) <= fn {
				grown := make([]bool, fn+1)
				copy(grown, e.snap.Functions)
				e.snap.Functions = grown
			}
			if e.snap.Functions[fn] != on {
				e.snap.Functions[fn] = on
				changed = true
			}
		}
	}
	if changed {
		// Drop ownership only when no BigFred command window is active.
		// Inside the window a bus echo (including a function echo) is
		// not a takeover; BigFred still owns the speed/dir it commanded.
		if !windowActive {
			e.snap.ControlledByUserID = 0
			e.snap.Source = source
		}
		e.snap.At = time.Now().UTC().UnixMilli()
	}
	out := e.snap
	e.mu.Unlock()
	if changed {
		s.markDirty(addr)
	}
	return out, changed
}

// SetFromBus replaces state from a command-station read (slot reclaim sync).
func (s *LocoStateStore) SetFromBus(addr uint16, speed uint8, forward bool, fns []bool, userID uint) contract.LocoStateWire {
	e := s.entry(addr)
	e.mu.Lock()
	e.snap.Address = addr
	e.snap.Speed = speed
	e.snap.Forward = forward
	e.snap.ControlledByUserID = userID
	e.snap.Functions = append([]bool(nil), fns...)
	e.snap.Source = "bus-sync"
	e.snap.At = time.Now().UTC().UnixMilli()
	e.commandedAt = time.Time{}
	out := e.snap
	e.mu.Unlock()
	s.markDirty(addr)
	return out
}

// FlushLoop runs batched Redis writes every tick. Blocks until ctx cancel,
// then performs a final flush so shutdown does not lose recent state.
func (s *LocoStateStore) FlushLoop(ctx context.Context, tick time.Duration) {
	if s == nil {
		return
	}
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			s.flushAll(context.Background())
			return
		case <-t.C:
			s.flushAll(ctx)
		}
	}
}

func (s *LocoStateStore) flushAll(ctx context.Context) {
	s.dirtyMu.Lock()
	addrs := make([]uint16, 0, len(s.dirty))
	for a := range s.dirty {
		addrs = append(addrs, a)
	}
	s.dirtyMu.Unlock()
	// flushOne removes an addr from dirty only on success, so a Redis
	// outage leaves the addr dirty for the next tick (retry).
	for _, addr := range addrs {
		s.flushOne(ctx, addr)
	}
}

func (s *LocoStateStore) flushOne(ctx context.Context, addr uint16) {
	s.mu.Lock()
	e, ok := s.entries[addr]
	s.mu.Unlock()
	if !ok {
		return
	}
	e.mu.Lock()
	snap := e.snap
	atMs := snap.At
	e.mu.Unlock()
	if s.redis != nil {
		if err := s.redis.StoreLocoCurrentState(ctx, snap, s.ttl); err != nil {
			if s.log != nil {
				s.log.WithError(err).WithField("addr", addr).Debug("loco store: redis flush")
			}
			return
		}
	}
	// Only clear dirty if no newer write landed while the Redis SET was
	// in flight; otherwise a concurrent command would be lost.
	s.dirtyMu.Lock()
	e.mu.Lock()
	currentAt := e.snap.At
	e.mu.Unlock()
	if currentAt == atMs {
		delete(s.dirty, addr)
	}
	s.dirtyMu.Unlock()
}

func (s *LocoStateStore) markDirty(addr uint16) {
	s.dirtyMu.Lock()
	s.dirty[addr] = struct{}{}
	s.dirtyMu.Unlock()
}

// FlushNow forces an immediate flush of addr (estop).
func (s *LocoStateStore) FlushNow(ctx context.Context, addr uint16) {
	s.flushOne(ctx, addr)
}

func (s *LocoStateStore) entry(addr uint16) *locoEntry {
	s.mu.Lock()
	e, ok := s.entries[addr]
	if !ok {
		e = &locoEntry{snap: contract.LocoStateWire{Address: addr, Forward: true}}
		s.entries[addr] = e
	}
	s.mu.Unlock()
	return e
}
