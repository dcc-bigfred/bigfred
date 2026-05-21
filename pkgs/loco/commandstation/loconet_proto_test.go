package commandstation

import "testing"

func TestLnChecksum(t *testing.T) {
	// Example from LoconetOverTcp docs: RECEIVE 83 7C
	pkt := []byte{0x83, 0x7C}
	if !lnChecksumOK(pkt) {
		t.Fatalf("expected checksum OK for % X", pkt)
	}
}

func TestLnFixedLen(t *testing.T) {
	// 0x83 => class with len=2 (bits5..6 = 00)
	l, ok := lnMsgLen(0x83, []byte{0x83})
	if !ok || l != 2 {
		t.Fatalf("expected len=2, got %d ok=%v", l, ok)
	}
	// 0xA0 => 4-byte
	l, ok = lnMsgLen(0xA0, []byte{0xA0})
	if !ok || l != 4 {
		t.Fatalf("expected len=4 for 0xA0, got %d ok=%v", l, ok)
	}
}

func TestLnVarLen(t *testing.T) {
	// SL_RD_DATA is variable, second byte is 0x0E
	buf := []byte{lnOPC_SL_RD_DATA, 0x0E}
	l, ok := lnMsgLen(buf[0], buf)
	if !ok || l != 14 {
		t.Fatalf("expected len=14, got %d ok=%v", l, ok)
	}
}

func TestParseSlotData(t *testing.T) {
	// Construct a minimal valid E7 0E message with checksum.
	// Slot=3, addr=5, speed=10, dirf=0x20 (forward), snd=0x02 (F6 on).
	msg := []byte{
		lnOPC_SL_RD_DATA, 0x0E,
		0x03, // slot
		0x00, // stat1
		0x05, // adr low
		0x0A, // speed
		0x20, // dirf
		0x00, // trk
		0x00, // stat2
		0x00, // adr high
		0x02, // snd
		0x00, // id1
		0x00, // id2
	}
	pkt := lnAppendChecksum(msg)
	sd, ok := parseLnSlotData(pkt)
	if !ok {
		t.Fatalf("expected parse ok for % X", pkt)
	}
	if sd.Slot != 0x03 || sd.Addr != 5 || sd.Speed != 0x0A || sd.DirF != 0x20 || sd.Snd != 0x02 {
		t.Fatalf("unexpected slot data: %+v", sd)
	}
}

