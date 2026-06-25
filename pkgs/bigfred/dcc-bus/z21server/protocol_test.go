package z21server

import (
	"encoding/binary"
	"testing"
)

func TestSplitZ21Datagram(t *testing.T) {
	t.Parallel()
	a := []byte{0x04, 0x00, 0x10, 0x00}
	b := []byte{0x07, 0x00, 0x40, 0x00, 0x21, 0x24, 0x05}
	datagram := append(append([]byte{}, a...), b...)
	pkts := splitZ21Datagram(datagram)
	if len(pkts) != 2 {
		t.Fatalf("splitZ21Datagram returned %d packets, want 2", len(pkts))
	}
	if binary.LittleEndian.Uint16(pkts[0][2:4]) != HeaderGetSerialNumber {
		t.Fatalf("first header = %#04x", binary.LittleEndian.Uint16(pkts[0][2:4]))
	}
	if binary.LittleEndian.Uint16(pkts[1][2:4]) != HeaderXBus || pkts[1][5] != 0x24 {
		t.Fatalf("second packet = % X", pkts[1])
	}
}

func TestSplitZ21DatagramSingle(t *testing.T) {
	t.Parallel()
	pkt := []byte{0x06, 0x00, 0x35, 0x00, 0x01, 0x02}
	got := splitZ21Datagram(pkt)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
}

func TestBuildAppKeepaliveReply(t *testing.T) {
	t.Parallel()
	reply := buildAppKeepaliveReply([]byte{0x06, 0x00, 0x35, 0x00, 0x00, 0x00})
	if len(reply) < 7 || reply[4] != 0x62 {
		t.Fatalf("expected LAN_X_STATUS_CHANGED, got % X", reply)
	}
}
