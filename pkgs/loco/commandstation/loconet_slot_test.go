package commandstation

import (
	"sync"
	"testing"
	"time"
)

// slotServerTransport is a fake command station for slot tests. It records
// every TX frame and answers OPC_LOCO_ADR / OPC_MOVE_SLOTS with a slot read,
// so AcquireSlot can complete its request/response handshake on a loopback.
type slotServerTransport struct {
	rxCh chan<- lnPacket

	mu    sync.Mutex
	tx    [][]byte
	slot  byte // slot number the station reports for the queried address
	stat1 byte // STAT1 returned for OPC_LOCO_ADR (e.g. COMMON or IN_USE)
}

func (s *slotServerTransport) WritePacket(pkt []byte) error {
	s.mu.Lock()
	s.tx = append(s.tx, append([]byte(nil), pkt...))
	slot, stat1 := s.slot, s.stat1
	s.mu.Unlock()

	if len(pkt) == 0 {
		return nil
	}
	switch pkt[0] {
	case lnOPC_LOCO_ADR:
		addr := LocoAddr(pkt[2]&0x7F) | (LocoAddr(pkt[1]&0x7F) << 7)
		s.rxCh <- buildSlotRead(slot, stat1, addr)
	case lnOPC_MOVE_SLOTS:
		// NULL MOVE (src==dst): confirm by echoing the slot as IN_USE.
		if pkt[1] == pkt[2] {
			s.rxCh <- buildSlotRead(pkt[1], lnSLOT_IN_USE, 0)
		}
	}
	return nil
}

func (s *slotServerTransport) Close() error { return nil }

func (s *slotServerTransport) txFrames() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([][]byte, len(s.tx))
	copy(out, s.tx)
	return out
}

func buildSlotRead(slot, stat1 byte, addr LocoAddr) lnPacket {
	msg := []byte{
		lnOPC_SL_RD_DATA, 0x0E,
		slot,
		stat1,
		byte(addr & 0x7F), // adr lo
		0x00,              // speed
		0x20,              // dirf (forward)
		0x00,              // trk
		0x00,              // stat2
		byte((addr >> 7) & 0x7F), // adr hi
		0x00,                     // snd
		0x00,                     // id1
		0x00,                     // id2
	}
	return lnPacket(lnAppendChecksum(msg))
}

func newSlotTestLoconet(slot, stat1 byte) (*LocoNet, *slotServerTransport) {
	l := newLocoNetBase()
	l.minTxGap = 0
	srv := &slotServerTransport{rxCh: l.rxCh, slot: slot, stat1: stat1}
	l.t = srv
	go l.dispatch()
	return l, srv
}

// AcquireSlot must reclaim a slot the command station purged to COMMON while
// the loco was idle: it queries fresh, caches the slot, and asserts IN_USE via
// a NULL MOVE.
func TestAcquireSlotReclaimsCommonSlot(t *testing.T) {
	l, srv := newSlotTestLoconet(5, lnSLOT_COMMON)
	t.Cleanup(func() { close(l.stop) })
	const addr LocoAddr = 31

	if err := l.AcquireSlot(addr); err != nil {
		t.Fatalf("AcquireSlot: %v", err)
	}

	if slot, ok := l.getSlot(addr); !ok || slot != 5 {
		t.Fatalf("cached slot = %d (ok=%v), want 5", slot, ok)
	}
	tx := srv.txFrames()
	if countOpcode(tx, lnOPC_LOCO_ADR) != 1 {
		t.Fatalf("expected one OPC_LOCO_ADR, got % X", tx)
	}
	if countOpcode(tx, lnOPC_MOVE_SLOTS) != 1 {
		t.Fatalf("expected one NULL MOVE for a COMMON slot, got % X", tx)
	}
}

// An already IN_USE slot (e.g. owned by a physical FRED, or already by BigFred)
// must not be stolen: no NULL MOVE is sent.
func TestAcquireSlotSkipsNullMoveWhenInUse(t *testing.T) {
	l, srv := newSlotTestLoconet(7, lnSLOT_IN_USE)
	t.Cleanup(func() { close(l.stop) })

	if err := l.AcquireSlot(42); err != nil {
		t.Fatalf("AcquireSlot: %v", err)
	}
	if countOpcode(srv.txFrames(), lnOPC_MOVE_SLOTS) != 0 {
		t.Fatalf("must not NULL MOVE an already IN_USE slot")
	}
}

// When the command station reassigns the loco to a different slot while idle,
// AcquireSlot must re-map the cache (forward and reverse) so later speed/function
// frames target the new slot, not the stale one.
func TestAcquireSlotRemapsReassignedSlot(t *testing.T) {
	l, _ := newSlotTestLoconet(9, lnSLOT_COMMON)
	t.Cleanup(func() { close(l.stop) })
	const addr LocoAddr = 55

	// Seed a stale mapping: addr was previously on slot 3.
	l.setSlot(addr, 3)
	if got, _ := l.slotToAddr(3); got != addr {
		t.Fatalf("precondition: slot 3 should map to addr %d", addr)
	}

	if err := l.AcquireSlot(addr); err != nil {
		t.Fatalf("AcquireSlot: %v", err)
	}

	if slot, ok := l.getSlot(addr); !ok || slot != 9 {
		t.Fatalf("cached slot = %d (ok=%v), want 9 after reassignment", slot, ok)
	}
	if _, ok := l.slotToAddr(3); ok {
		t.Fatal("stale reverse mapping for old slot 3 was not cleared")
	}
	if got, ok := l.slotToAddr(9); !ok || got != addr {
		t.Fatalf("reverse mapping for new slot 9 = %d (ok=%v), want %d", got, ok, addr)
	}
}

// AcquireSlot rejects address 0 (the dispatch/system slot sentinel).
func TestAcquireSlotRejectsZeroAddr(t *testing.T) {
	l, _ := newSlotTestLoconet(1, lnSLOT_COMMON)
	t.Cleanup(func() { close(l.stop) })
	if err := l.AcquireSlot(0); err == nil {
		t.Fatal("expected error for addr 0")
	}
}

// Guard against a regression where AcquireSlot blocks forever if the station
// never answers: it must honour slotTimeout and return an error.
func TestAcquireSlotTimesOut(t *testing.T) {
	l := newLocoNetBase()
	l.minTxGap = 0
	l.slotTimeout = 50 * time.Millisecond
	l.t = &recTransport{} // records writes, never replies
	go l.dispatch()
	t.Cleanup(func() { close(l.stop) })

	start := time.Now()
	if err := l.AcquireSlot(31); err == nil {
		t.Fatal("expected timeout error when station is silent")
	}
	// Two attempts (initial + lnSlotAcquireRetries) bounded by slotTimeout each.
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("AcquireSlot took %s, expected to fail fast", elapsed)
	}
}
