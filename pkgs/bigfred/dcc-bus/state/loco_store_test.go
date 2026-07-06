package state

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

type memRedis struct {
	mu    sync.Mutex
	snaps map[uint16]contract.LocoStateWire
	puts  int
}

func (m *memRedis) GetLocoCurrentState(_ context.Context, addr uint16) (contract.LocoStateWire, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.snaps[addr]
	return s, ok, nil
}

func (m *memRedis) StoreLocoCurrentState(_ context.Context, snap contract.LocoStateWire, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.snaps == nil {
		m.snaps = make(map[uint16]contract.LocoStateWire)
	}
	m.snaps[snap.Address] = snap
	m.puts++
	return nil
}

type failRedis struct {
	err error
}

func (f *failRedis) GetLocoCurrentState(_ context.Context, _ uint16) (contract.LocoStateWire, bool, error) {
	return contract.LocoStateWire{}, false, nil
}

func (f *failRedis) StoreLocoCurrentState(_ context.Context, _ contract.LocoStateWire, _ time.Duration) error {
	return f.err
}

func expireCommandSuppress(s *LocoStateStore, addr uint16) {
	e := s.entry(addr)
	e.mu.Lock()
	e.commandedAt = time.Time{}
	e.mu.Unlock()
}

func TestStoreSuppressesStaleEchoAfterCommand(t *testing.T) {
	s := NewLocoStateStore(nil, time.Minute, nil)
	s.SetSpeed(10, 80, true, 1, "throttle")

	snap, changed := s.ApplyObservation(commandstation.LocoObservation{
		Addr: 10, HasSpeed: true, Speed: 50,
	}, "external")
	if changed {
		t.Fatal("stale echo must not change snap")
	}
	if snap.Speed != 80 || snap.ControlledByUserID != 1 {
		t.Fatalf("snap=%+v, want speed=80 user=1", snap)
	}

	_, changed = s.ApplyObservation(commandstation.LocoObservation{
		Addr: 10, HasSpeed: true, Speed: 80,
	}, "external")
	if changed {
		t.Fatal("confirming echo should be no-op")
	}

	expireCommandSuppress(s, 10)
	snap, changed = s.ApplyObservation(commandstation.LocoObservation{
		Addr: 10, HasSpeed: true, Speed: 30,
	}, "external")
	if !changed || snap.Speed != 30 {
		t.Fatalf("after window expired, observation must apply: %+v changed=%v", snap, changed)
	}
}

// TestStoreSplitSpeedForwardEcho verifies that a partial (speed-only)
// confirming echo does NOT release the suppression window, so a later
// stale direction frame cannot snap the lever back. LocoNet emits speed
// and direction in separate OPC_LOCO_SPD / OPC_LOCO_DIRF frames.
func TestStoreSplitSpeedForwardEcho(t *testing.T) {
	s := NewLocoStateStore(nil, time.Minute, nil)
	s.SetSpeed(10, 80, true, 1, "throttle")

	// Good speed echo (matches) but no direction field: must NOT clear window.
	_, changed := s.ApplyObservation(commandstation.LocoObservation{
		Addr: 10, HasSpeed: true, Speed: 80,
	}, "external")
	if changed {
		t.Fatal("matching speed-only echo should be no-op")
	}

	// Stale direction echo arriving after the partial speed confirm:
	// window still active → must be suppressed.
	snap, changed := s.ApplyObservation(commandstation.LocoObservation{
		Addr: 10, HasForward: true, Forward: false,
	}, "external")
	if changed {
		t.Fatalf("stale direction echo must be suppressed: %+v", snap)
	}
	if !snap.Forward || snap.ControlledByUserID != 1 {
		t.Fatalf("snap=%+v, want forward=true user=1", snap)
	}

	// Full confirming echo (both dims present and matching) releases window.
	_, changed = s.ApplyObservation(commandstation.LocoObservation{
		Addr: 10, HasSpeed: true, Speed: 80, HasForward: true, Forward: true,
	}, "external")
	if changed {
		t.Fatal("full confirming echo should be no-op")
	}

	// Window now released: external change applies.
	snap, changed = s.ApplyObservation(commandstation.LocoObservation{
		Addr: 10, HasSpeed: true, Speed: 20,
	}, "external")
	if !changed || snap.Speed != 20 {
		t.Fatalf("after full confirm, observation must apply: %+v changed=%v", snap, changed)
	}
}

// TestStoreFunctionEchoDuringSuppressionKeepsUser verifies that a bus
// function echo arriving inside the motion suppression window applies
// the function change but does not drop throttle ownership.
func TestStoreFunctionEchoDuringSuppressionKeepsUser(t *testing.T) {
	s := NewLocoStateStore(nil, time.Minute, nil)
	s.SetSpeed(10, 80, true, 1, "throttle")

	snap, changed := s.ApplyObservation(commandstation.LocoObservation{
		Addr:         10,
		FunctionMask: 1 << 0,
		FunctionBits: 1 << 0,
	}, "external")
	if !changed {
		t.Fatal("function change should apply")
	}
	if !snap.Functions[0] {
		t.Fatal("F0 should be on")
	}
	if snap.ControlledByUserID != 1 {
		t.Fatalf("userID = %d, want 1 (function echo must not drop ownership)", snap.ControlledByUserID)
	}
	if snap.Speed != 80 {
		t.Fatalf("speed = %d, want 80 (commanded speed protected)", snap.Speed)
	}
}

// TestStoreSetSpeedPreservingUser verifies the server-commanded path
// does not touch ControlledByUserID while still recording the command.
func TestStoreSetSpeedPreservingUser(t *testing.T) {
	s := NewLocoStateStore(nil, time.Minute, nil)
	s.SetSpeed(10, 50, true, 7, "throttle")

	out := s.SetSpeedPreservingUser(10, 0, true, "estop")
	if out.Speed != 0 {
		t.Fatalf("speed = %d, want 0", out.Speed)
	}
	if out.ControlledByUserID != 7 {
		t.Fatalf("userID = %d, want 7 (preserved)", out.ControlledByUserID)
	}

	// Stale speed echo after the estop command must be suppressed.
	snap, changed := s.ApplyObservation(commandstation.LocoObservation{
		Addr: 10, HasSpeed: true, Speed: 50,
	}, "external")
	if changed {
		t.Fatalf("stale echo after SetSpeedPreservingUser must be suppressed: %+v", snap)
	}
	if snap.Speed != 0 || snap.ControlledByUserID != 7 {
		t.Fatalf("snap=%+v, want speed=0 user=7", snap)
	}
}

func TestStoreSetSpeedAndSnapshot(t *testing.T) {
	s := NewLocoStateStore(nil, time.Minute, nil)
	out := s.SetSpeed(10, 5, true, 7, "throttle")
	if out.Speed != 5 || out.ControlledByUserID != 7 {
		t.Fatalf("SetSpeed = %+v", out)
	}
	got := s.Snapshot(10)
	if got.Speed != 5 {
		t.Fatalf("Snapshot speed = %d, want 5", got.Speed)
	}
}

func TestStoreApplyObservationMerge(t *testing.T) {
	s := NewLocoStateStore(nil, time.Minute, nil)
	s.SetSpeed(10, 5, true, 1, "throttle")
	expireCommandSuppress(s, 10)
	snap, changed := s.ApplyObservation(commandstation.LocoObservation{
		Addr:       10,
		HasSpeed:   true,
		Speed:      8,
		HasForward: true,
		Forward:    false,
	}, "external")
	if !changed {
		t.Fatal("expected change")
	}
	if snap.Speed != 8 || snap.Forward || snap.ControlledByUserID != 0 {
		t.Fatalf("snap = %+v", snap)
	}
	_, changed = s.ApplyObservation(commandstation.LocoObservation{
		Addr:     10,
		HasSpeed: true,
		Speed:    8,
	}, "external")
	if changed {
		t.Fatal("expected no change on identical observation")
	}
}

func TestStoreSerializesPerAddr(t *testing.T) {
	s := NewLocoStateStore(nil, time.Minute, nil)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(spd uint8) {
			defer wg.Done()
			s.SetSpeed(10, spd, true, 1, "throttle")
		}(uint8(i))
		wg.Add(1)
		go func(spd uint8) {
			defer wg.Done()
			s.ApplyObservation(commandstation.LocoObservation{
				Addr: 10, HasSpeed: true, Speed: spd,
			}, "external")
		}(uint8(i + 1))
	}
	wg.Wait()
	got := s.Snapshot(10)
	if got.Address != 10 {
		t.Fatalf("snap = %+v", got)
	}
}

func TestStoreFlushLoopWritesRedis(t *testing.T) {
	redis := &memRedis{}
	s := NewLocoStateStore(redis, time.Minute, nil)
	s.SetSpeed(10, 3, true, 0, "throttle")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.FlushLoop(ctx, 20*time.Millisecond)
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	if redis.puts == 0 {
		t.Fatal("expected redis flush from FlushLoop")
	}
}

func TestStoreFlushNowPriority(t *testing.T) {
	redis := &memRedis{}
	s := NewLocoStateStore(redis, time.Minute, nil)
	s.SetSpeed(10, 0, true, 1, "estop")
	s.FlushNow(context.Background(), 10)
	if redis.puts != 1 {
		t.Fatalf("puts = %d, want 1", redis.puts)
	}
}

func TestStoreLoadMissingFromRedis(t *testing.T) {
	redis := &memRedis{snaps: map[uint16]contract.LocoStateWire{
		10: {Address: 10, Speed: 4, Forward: true},
	}}
	s := NewLocoStateStore(redis, time.Minute, nil)
	s.LoadMissingFromRedis(context.Background(), []uint16{10})
	got := s.Snapshot(10)
	if got.Speed != 4 {
		t.Fatalf("speed = %d, want 4", got.Speed)
	}
}

func TestStoreLoadMissingFromRedisSkipsExisting(t *testing.T) {
	redis := &memRedis{snaps: map[uint16]contract.LocoStateWire{
		10: {Address: 10, Speed: 4, Forward: true},
	}}
	s := NewLocoStateStore(redis, time.Minute, nil)
	s.SetSpeed(10, 80, true, 1, "throttle")
	s.LoadMissingFromRedis(context.Background(), []uint16{10})
	got := s.Snapshot(10)
	if got.Speed != 80 {
		t.Fatalf("speed = %d, want 80 (live state preserved)", got.Speed)
	}
}

// TestStoreFlushAllRetriesOnRedisFailure verifies that a failed Redis
// write leaves the addr dirty so the next tick retries it.
func TestStoreFlushAllRetriesOnRedisFailure(t *testing.T) {
	redis := &memRedis{}
	s := NewLocoStateStore(redis, time.Minute, nil)
	s.SetSpeed(10, 3, true, 0, "throttle")

	s.flushAll(context.Background()) // success → dirty cleared
	s.dirtyMu.Lock()
	dirtyCount := len(s.dirty)
	s.dirtyMu.Unlock()
	if dirtyCount != 0 {
		t.Fatalf("dirty = %d after successful flush, want 0", dirtyCount)
	}

	failStore := NewLocoStateStore(&failRedis{err: errors.New("redis down")}, time.Minute, nil)
	failStore.SetSpeed(20, 9, true, 0, "throttle")
	failStore.flushAll(context.Background()) // failure → dirty retained
	failStore.dirtyMu.Lock()
	dirtyCount = len(failStore.dirty)
	failStore.dirtyMu.Unlock()
	if dirtyCount != 1 {
		t.Fatalf("dirty = %d after failed flush, want 1 (retry)", dirtyCount)
	}
}
