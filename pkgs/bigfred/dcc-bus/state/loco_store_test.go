package state

import (
	"context"
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

func TestStoreLoadFromRedis(t *testing.T) {
	redis := &memRedis{snaps: map[uint16]contract.LocoStateWire{
		10: {Address: 10, Speed: 4, Forward: true},
	}}
	s := NewLocoStateStore(redis, time.Minute, nil)
	s.LoadFromRedis(context.Background(), []uint16{10})
	got := s.Snapshot(10)
	if got.Speed != 4 {
		t.Fatalf("speed = %d, want 4", got.Speed)
	}
}
