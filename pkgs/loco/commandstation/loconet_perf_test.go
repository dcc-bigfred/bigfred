package commandstation

import (
	"sync"
	"testing"
	"time"
)

// recTransport is a fake lnTransport that records every frame written, so tests
// can assert exactly what hits the bus on the fast path.
type recTransport struct {
	mu   sync.Mutex
	pkts [][]byte
}

func (r *recTransport) WritePacket(pkt []byte) error {
	r.mu.Lock()
	r.pkts = append(r.pkts, append([]byte(nil), pkt...))
	r.mu.Unlock()
	return nil
}

func (r *recTransport) Close() error { return nil }

func (r *recTransport) packets() [][]byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([][]byte, len(r.pkts))
	copy(out, r.pkts)
	return out
}

func (r *recTransport) reset() {
	r.mu.Lock()
	r.pkts = nil
	r.mu.Unlock()
}

func (r *recTransport) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.pkts)
}

// newTestLoconet builds a driver wired to a recording transport with pacing
// disabled, isolating logic (coalescing, DIRF suppression) from timing.
func newTestLoconet() (*LocoNet, *recTransport) {
	l := newLocoNetBase()
	l.minTxGap = 0
	rec := &recTransport{}
	l.t = rec
	return l, rec
}

func countOpcode(pkts [][]byte, opc byte) int {
	n := 0
	for _, p := range pkts {
		if len(p) > 0 && p[0] == opc {
			n++
		}
	}
	return n
}

// B4: SetSpeed must send OPC_LOCO_DIRF only when the direction bit changes, not
// on every speed tick.
func TestSetSpeedSuppressesUnchangedDirf(t *testing.T) {
	l, rec := newTestLoconet()
	const addr LocoAddr = 31
	l.setSlot(addr, 5)

	// First move sets the direction (reverse -> forward): SPD + DIRF.
	if err := l.SetSpeed(addr, 10, true, 128); err != nil {
		t.Fatalf("SetSpeed #1: %v", err)
	}
	pkts := rec.packets()
	if got := countOpcode(pkts, lnOPC_LOCO_SPD); got != 1 {
		t.Fatalf("first move: SPD frames = %d, want 1", got)
	}
	if got := countOpcode(pkts, lnOPC_LOCO_DIRF); got != 1 {
		t.Fatalf("first move: DIRF frames = %d, want 1 (direction changed)", got)
	}

	// Same direction, new speed: SPD only, no DIRF.
	rec.reset()
	if err := l.SetSpeed(addr, 40, true, 128); err != nil {
		t.Fatalf("SetSpeed #2: %v", err)
	}
	pkts = rec.packets()
	if got := countOpcode(pkts, lnOPC_LOCO_SPD); got != 1 {
		t.Fatalf("same direction: SPD frames = %d, want 1", got)
	}
	if got := countOpcode(pkts, lnOPC_LOCO_DIRF); got != 0 {
		t.Fatalf("same direction: DIRF frames = %d, want 0 (direction unchanged)", got)
	}

	// Reversing must send DIRF again.
	rec.reset()
	if err := l.SetSpeed(addr, 40, false, 128); err != nil {
		t.Fatalf("SetSpeed #3: %v", err)
	}
	if got := countOpcode(rec.packets(), lnOPC_LOCO_DIRF); got != 1 {
		t.Fatalf("reversing: DIRF frames = %d, want 1", got)
	}
}

// B3: a speed frame superseded by a newer SetSpeed (higher generation) is
// dropped instead of consuming a transmission slot.
func TestWriteSpeedCoalescesStale(t *testing.T) {
	l, rec := newTestLoconet()
	const addr LocoAddr = 42
	l.setSlot(addr, 7)
	l.setDirf(addr, 0x20) // forward, so DIRF is never re-sent below

	g1 := l.nextSpeedGen(addr)
	g2 := l.nextSpeedGen(addr) // g2 supersedes g1

	// The stale generation must be dropped.
	if err := l.writeSpeed(addr, 7, 20, true, g1); err != nil {
		t.Fatalf("writeSpeed(stale): %v", err)
	}
	if rec.count() != 0 {
		t.Fatalf("stale frame was written (% X), expected coalesced/dropped", rec.packets())
	}

	// The current generation must go out.
	if err := l.writeSpeed(addr, 7, 30, true, g2); err != nil {
		t.Fatalf("writeSpeed(current): %v", err)
	}
	pkts := rec.packets()
	if got := countOpcode(pkts, lnOPC_LOCO_SPD); got != 1 {
		t.Fatalf("current: SPD frames = %d, want 1", got)
	}
	if got := countOpcode(pkts, lnOPC_LOCO_DIRF); got != 0 {
		t.Fatalf("current: DIRF frames = %d, want 0 (direction preseeded)", got)
	}
}

// B1: SendFn for a cached slot trusts the cache and sends a single frame with no
// blocking OPC_RQ_SL_DATA round trip.
func TestSendFnTrustsCacheNoRoundTrip(t *testing.T) {
	l, rec := newTestLoconet()
	const addr LocoAddr = 55
	l.setSlot(addr, 9)

	// F1 lives in DIRF.
	if err := l.SendFn(MainTrackMode, addr, 1, true); err != nil {
		t.Fatalf("SendFn F1: %v", err)
	}
	pkts := rec.packets()
	if len(pkts) != 1 {
		t.Fatalf("F1: wrote %d frames, want 1 (no slot query): % X", len(pkts), pkts)
	}
	if got := countOpcode(pkts, lnOPC_RQ_SL_DATA); got != 0 {
		t.Fatalf("F1: %d slot-query frames, want 0", got)
	}
	if pkts[0][0] != lnOPC_LOCO_DIRF {
		t.Fatalf("F1: opcode % X, want DIRF % X", pkts[0][0], lnOPC_LOCO_DIRF)
	}

	// F5 lives in SND.
	rec.reset()
	if err := l.SendFn(MainTrackMode, addr, 5, true); err != nil {
		t.Fatalf("SendFn F5: %v", err)
	}
	pkts = rec.packets()
	if len(pkts) != 1 || pkts[0][0] != lnOPC_LOCO_SND {
		t.Fatalf("F5: frames % X, want single SND % X", pkts, lnOPC_LOCO_SND)
	}
}

// The pacer enforces the minimum inter-frame gap so a burst cannot outrun the
// bus and overflow the transport buffer. Only the lower bound is asserted (a
// sleep never returns early), keeping the test non-flaky.
func TestPacerEnforcesMinGap(t *testing.T) {
	l, _ := newTestLoconet()
	l.minTxGap = 20 * time.Millisecond

	start := time.Now()
	for i := 0; i < 3; i++ {
		if err := l.sendLocked(lnBuildSetSpeed(5, byte(i))); err != nil {
			t.Fatalf("sendLocked: %v", err)
		}
	}
	// 3 frames => at least 2 gaps between them.
	if elapsed := time.Since(start); elapsed < 2*l.minTxGap {
		t.Fatalf("burst took %s, want >= %s (pacer not throttling)", elapsed, 2*l.minTxGap)
	}
}

// refreshSlots (keepalive) re-sends the last known speed for every cached slot
// so the master does not purge it.
func TestKeepaliveRefreshesCachedSlots(t *testing.T) {
	l, rec := newTestLoconet()
	const addr LocoAddr = 77
	l.setSlot(addr, 11)
	l.setDirf(addr, 0x20)
	if err := l.SetSpeed(addr, 25, true, 128); err != nil {
		t.Fatalf("SetSpeed: %v", err)
	}
	rec.reset()

	l.refreshSlots()
	pkts := rec.packets()
	if got := countOpcode(pkts, lnOPC_LOCO_SPD); got != 1 {
		t.Fatalf("keepalive: SPD frames = %d, want 1", got)
	}
}
