package slotlease

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

type fakeStation struct {
	mu        sync.Mutex
	acquired  []uint16
	released  []uint16
	acquireFn func(addr uint16) error
}

func (f *fakeStation) AcquireSlot(addr commandstation.LocoAddr) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.acquireFn != nil {
		if err := f.acquireFn(uint16(addr)); err != nil {
			return err
		}
	}
	f.acquired = append(f.acquired, uint16(addr))
	return nil
}

func (f *fakeStation) ReleaseSlot(addr commandstation.LocoAddr) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.released = append(f.released, uint16(addr))
	return nil
}

type fakeWriter struct {
	mu    sync.Mutex
	calls []struct {
		addr      uint16
		speed     uint8
		forward   bool
		emergency bool
	}
}

func (f *fakeWriter) SetSpeed(addr uint16, speed uint8, forward bool, emergency bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, struct {
		addr      uint16
		speed     uint8
		forward   bool
		emergency bool
	}{addr, speed, forward, emergency})
	return nil
}

func (f *fakeWriter) estopBeforeRelease(addr uint16) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.calls {
		if c.addr == addr && c.emergency {
			return true
		}
	}
	return false
}

type fakeStore struct {
	mu    sync.Mutex
	snaps map[uint16]contract.LocoStateWire
}

func newFakeStore() *fakeStore {
	return &fakeStore{snaps: make(map[uint16]contract.LocoStateWire)}
}

func (s *fakeStore) Snapshot(addr uint16) contract.LocoStateWire {
	s.mu.Lock()
	defer s.mu.Unlock()
	if snap, ok := s.snaps[addr]; ok {
		return snap
	}
	return contract.LocoStateWire{Address: addr, Forward: true}
}

func (s *fakeStore) SetSpeedPreservingUser(addr uint16, speed uint8, forward bool, source string) contract.LocoStateWire {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := s.snaps[addr]
	snap.Address = addr
	snap.Speed = speed
	snap.Forward = forward
	s.snaps[addr] = snap
	return snap
}

type fakeHub struct{}

func (fakeHub) BroadcastLocoState(context.Context, contract.LocoStateWire) {}

func newTestLeaser(station *fakeStation, maxPerUser, maxSlots int) *Leaser {
	st := station
	if st == nil {
		st = &fakeStation{}
	}
	return New(st, &fakeWriter{}, newFakeStore(), fakeHub{}, nil, Config{
		MaxPerUser:   maxPerUser,
		MaxSlots:     maxSlots,
		IdleTimeout:  time.Minute,
		ReleaseGrace: 0,
	})
}

func TestSelectDeselectReleasesSlot(t *testing.T) {
	st := &fakeStation{}
	w := &fakeWriter{}
	l := New(st, w, newFakeStore(), fakeHub{}, nil, Config{MaxSlots: 10, ReleaseGrace: 0})

	if _, err := l.Select(1, "s1", "ws", 42); err != nil {
		t.Fatalf("Select: %v", err)
	}
	if len(st.acquired) != 1 || st.acquired[0] != 42 {
		t.Fatalf("acquired = %v, want [42]", st.acquired)
	}

	l.Deselect(1, "s1", 42)

	st.mu.Lock()
	released := append([]uint16(nil), st.released...)
	st.mu.Unlock()
	if len(released) != 1 || released[0] != 42 {
		t.Fatalf("released = %v, want [42]", released)
	}
	if !w.estopBeforeRelease(42) {
		t.Fatal("e-stop should precede release")
	}
}

func TestBigFredSlotBudgetShortCircuitsAcquire(t *testing.T) {
	st := &fakeStation{}
	l := newTestLeaser(st, 8, 3)

	for i := uint16(1); i <= 3; i++ {
		if _, err := l.Select(uint(i), "s", "ws", 100+i); err != nil {
			t.Fatalf("Select %d: %v", i, err)
		}
	}
	if _, err := l.Select(4, "s4", "ws", 200); err != ErrBigFredSlotBudgetExceeded {
		t.Fatalf("4th Select err = %v, want ErrBigFredSlotBudgetExceeded", err)
	}
	st.mu.Lock()
	n := len(st.acquired)
	st.mu.Unlock()
	if n != 3 {
		t.Fatalf("AcquireSlot calls = %d, want 3 (no 4th bus call)", n)
	}
}

func TestSwitcherDeselectThenSelect(t *testing.T) {
	st := &fakeStation{}
	l := newTestLeaser(st, 8, 10)

	if _, err := l.Select(1, "s1", "ws", 10); err != nil {
		t.Fatal(err)
	}
	l.Deselect(1, "s1", 10)
	if _, err := l.Select(1, "s1", "ws", 20); err != nil {
		t.Fatal(err)
	}
	st.mu.Lock()
	acquired := append([]uint16(nil), st.acquired...)
	released := append([]uint16(nil), st.released...)
	st.mu.Unlock()
	if len(acquired) != 2 || acquired[1] != 20 {
		t.Fatalf("acquired = %v", acquired)
	}
	if len(released) != 1 || released[0] != 10 {
		t.Fatalf("released = %v, want [10]", released)
	}
}

func TestPerUserCapEvictsOldest(t *testing.T) {
	l := newTestLeaser(&fakeStation{}, 2, 0)

	if _, err := l.Select(1, "s1", "ws", 11); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Select(1, "s1", "ws", 22); err != nil {
		t.Fatal(err)
	}
	evicted, err := l.Select(1, "s1", "ws", 33)
	if err != nil {
		t.Fatalf("3rd Select: %v", err)
	}
	if evicted != 11 {
		t.Fatalf("evicted = %d, want 11", evicted)
	}
	if l.LeaseCount() != 2 {
		t.Fatalf("lease count = %d, want 2", l.LeaseCount())
	}
}

func TestDriveGateDeniesSelect(t *testing.T) {
	st := &fakeStation{}
	l := New(st, &fakeWriter{}, newFakeStore(), fakeHub{}, func(userID uint, addr uint16) error {
		return ErrNotAllowed
	}, Config{MaxSlots: 10})

	if _, err := l.Select(1, "s1", "ws", 5); err != ErrNotAllowed {
		t.Fatalf("err = %v, want ErrNotAllowed", err)
	}
	st.mu.Lock()
	n := len(st.acquired)
	st.mu.Unlock()
	if n != 0 {
		t.Fatalf("AcquireSlot calls = %d, want 0", n)
	}
}

func TestSweepIdleReleasesRemoteOnly(t *testing.T) {
	st := &fakeStation{}
	l := newTestLeaser(st, 8, 10)
	l.idleTimeout = 30 * time.Second

	if _, err := l.Select(1, "r1", "z21", 50); err != nil {
		t.Fatal(err)
	}
	l.mu.Lock()
	le := l.leases[50]
	le.lastDriveAt[le.holderOrder[0]] = time.Now().Add(-time.Minute)
	l.mu.Unlock()

	l.SweepIdle(time.Now())

	st.mu.Lock()
	released := len(st.released)
	st.mu.Unlock()
	if released != 1 {
		t.Fatalf("released = %d, want 1 after idle sweep", released)
	}
}

func TestSweepIdleKeepsWSHolder(t *testing.T) {
	st := &fakeStation{}
	l := newTestLeaser(st, 8, 10)
	l.idleTimeout = 30 * time.Second

	if _, err := l.Select(1, "s1", "ws", 60); err != nil {
		t.Fatal(err)
	}
	l.mu.Lock()
	le := l.leases[60]
	le.lastDriveAt[le.holderOrder[0]] = time.Now().Add(-time.Minute)
	l.mu.Unlock()

	l.SweepIdle(time.Now())

	if l.LeaseCount() != 1 {
		t.Fatal("WS lease should survive idle sweep")
	}
	st.mu.Lock()
	released := len(st.released)
	st.mu.Unlock()
	if released != 0 {
		t.Fatalf("released = %d, want 0", released)
	}
}

func TestBudgetCountsPhysicalSlots(t *testing.T) {
	l := newTestLeaser(&fakeStation{}, 8, 2)

	l.OnSlotInUse(commandstation.LocoAddr(10))
	l.OnSlotInUse(commandstation.LocoAddr(11))
	snap := l.DiagnosticSnapshot()
	if snap.Used != 2 {
		t.Fatalf("Used = %d, want 2 (physical slots on CS)", snap.Used)
	}
	if _, err := l.Reserve(1, "s", "ws", 12); err != ErrBigFredSlotBudgetExceeded {
		t.Fatalf("Reserve on full budget err = %v, want ErrBigFredSlotBudgetExceeded", err)
	}
	if _, err := l.Reserve(1, "s", "ws", 10); err != nil {
		t.Fatalf("Reserve on existing physical lease: %v", err)
	}
	snap = l.DiagnosticSnapshot()
	if snap.Used != 2 {
		t.Fatalf("Used after reuse = %d, want 2", snap.Used)
	}
}

func putLeaseInGrace(l *Leaser, addr uint16, releaseAt time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	le := l.leases[addr]
	if le == nil {
		le = &lease{
			addr:        addr,
			kind:        leaseSingle,
			holders:     make(map[holderKey]struct{}),
			lastDriveAt: make(map[holderKey]time.Time),
		}
		l.leases[addr] = le
	}
	le.holders = make(map[holderKey]struct{})
	le.holderOrder = nil
	le.acquiredAt = time.Now()
	le.releaseAt = releaseAt
}

func TestGraceEvictFreesBudgetForReserve(t *testing.T) {
	st := &fakeStation{}
	l := newTestLeaser(st, 8, 2)

	putLeaseInGrace(l, 20, time.Now().Add(30*time.Minute))
	putLeaseInGrace(l, 21, time.Now().Add(40*time.Minute))
	if snap := l.DiagnosticSnapshot(); snap.Used != 2 {
		t.Fatalf("Used = %d, want 2 grace leases occupying budget", snap.Used)
	}

	if _, err := l.Reserve(1, "s", "ws", 30); err != nil {
		t.Fatalf("Reserve after grace evict: %v", err)
	}

	st.mu.Lock()
	released := append([]uint16(nil), st.released...)
	st.mu.Unlock()
	if len(released) != 1 {
		t.Fatalf("released = %v, want one grace eviction", released)
	}
	if released[0] != 21 {
		t.Fatalf("released addr = %d, want 21 (newest grace first)", released[0])
	}
	if _, ok := l.leases[21]; ok {
		t.Fatal("lease 21 should be removed")
	}
	if _, ok := l.leases[20]; !ok {
		t.Fatal("lease 20 should remain in grace")
	}
}

func TestGraceEvictRejectsWhenNoGraceCandidates(t *testing.T) {
	st := &fakeStation{}
	l := newTestLeaser(st, 8, 1)

	l.OnSlotInUse(commandstation.LocoAddr(10))
	if _, err := l.Reserve(1, "s", "ws", 20); err != ErrBigFredSlotBudgetExceeded {
		t.Fatalf("Reserve err = %v, want ErrBigFredSlotBudgetExceeded", err)
	}
	st.mu.Lock()
	n := len(st.released)
	st.mu.Unlock()
	if n != 0 {
		t.Fatalf("released = %d, want 0", n)
	}
}

func TestGraceEvictCapsAtFiveAttempts(t *testing.T) {
	st := &fakeStation{}
	l := newTestLeaser(st, 8, 2)
	now := time.Now()

	for i := uint16(0); i < 7; i++ {
		putLeaseInGrace(l, 10+i, now.Add(time.Duration(i)*time.Minute))
	}
	if snap := l.DiagnosticSnapshot(); snap.Used != 7 {
		t.Fatalf("Used = %d, want 7", snap.Used)
	}

	if _, err := l.Reserve(1, "s", "ws", 99); err != ErrBigFredSlotBudgetExceeded {
		t.Fatalf("Reserve err = %v, want budget exceeded after 5 evictions", err)
	}

	st.mu.Lock()
	released := append([]uint16(nil), st.released...)
	st.mu.Unlock()
	if len(released) != 5 {
		t.Fatalf("released %v, want 5 (D20 attempt cap)", released)
	}
	want := []uint16{16, 15, 14, 13, 12}
	for i, addr := range want {
		if released[i] != addr {
			t.Fatalf("released[%d] = %d, want %d (newest grace first)", i, released[i], addr)
		}
	}
}

func TestGraceEvictDoesNotTakeActiveLease(t *testing.T) {
	st := &fakeStation{}
	l := newTestLeaser(st, 8, 2)

	if _, err := l.Select(1, "s", "ws", 10); err != nil {
		t.Fatal(err)
	}
	putLeaseInGrace(l, 20, time.Now().Add(time.Hour))

	if _, err := l.Reserve(1, "s2", "ws", 30); err != nil {
		t.Fatalf("Reserve: %v", err)
	}

	st.mu.Lock()
	released := append([]uint16(nil), st.released...)
	st.mu.Unlock()
	if len(released) != 1 || released[0] != 20 {
		t.Fatalf("released = %v, want [20]", released)
	}
	if _, ok := l.leases[10]; !ok {
		t.Fatal("active lease 10 must remain")
	}
}

func TestSelectTrainGraceEvict(t *testing.T) {
	st := &fakeStation{}
	l := newTestLeaser(st, 8, 2)

	putLeaseInGrace(l, 20, time.Now().Add(10*time.Minute))
	putLeaseInGrace(l, 21, time.Now().Add(20*time.Minute))

	if err := l.SelectTrain(1, "s", "ws", "t1", []uint16{30, 31}); err != nil {
		t.Fatalf("SelectTrain: %v", err)
	}

	st.mu.Lock()
	released := append([]uint16(nil), st.released...)
	st.mu.Unlock()
	if len(released) != 2 {
		t.Fatalf("released = %v, want two grace evictions (one per needed slot)", released)
	}
	if released[0] != 21 || released[1] != 20 {
		t.Fatalf("released order = %v, want [21 20] newest first", released)
	}
	if l.LeaseCount() < 2 {
		t.Fatalf("lease count = %d, want train members retained", l.LeaseCount())
	}
}
