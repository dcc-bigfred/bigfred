package z21server

import (
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestIsSetStop(t *testing.T) {
	t.Parallel()
	pkt := []byte{0x05, 0x00, 0x40, 0x00, 0x80, 0x80}
	if !isSetStop(HeaderXBus, pkt) {
		t.Fatal("expected LAN_X_SET_STOP")
	}
	if isSetStop(HeaderXBus, []byte{0x05, 0x00, 0x40, 0x00, 0x21, 0x80}) {
		t.Fatal("track power off is not set stop")
	}
}

func TestIsSetTrackPowerOff(t *testing.T) {
	t.Parallel()
	pkt := []byte{0x06, 0x00, 0x40, 0x00, 0x21, 0x80, 0xA1}
	if !isSetTrackPowerOff(HeaderXBus, pkt) {
		t.Fatal("expected LAN_X_SET_TRACK_POWER_OFF")
	}
	if isSetTrackPowerOff(HeaderXBus, []byte{0x05, 0x00, 0x40, 0x00, 0x80, 0x80}) {
		t.Fatal("set stop is not track power off")
	}
}

func TestBuildBCStoppedReply(t *testing.T) {
	t.Parallel()
	reply := buildBCStoppedReply()
	if len(reply) != 6 {
		t.Fatalf("len = %d, want 6", len(reply))
	}
	if binary.LittleEndian.Uint16(reply[2:4]) != HeaderXBus {
		t.Fatalf("header = %x", reply[2:4])
	}
	if reply[4] != 0x81 {
		t.Fatalf("x[0] = %#x, want 0x81", reply[4])
	}
	if reply[5] != xorSum([]byte{0x81}) {
		t.Fatalf("checksum = %#x", reply[5])
	}
}

func TestRegistryCurrentLoco(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	now := time.Now().UTC()
	addr := &net.UDPAddr{IP: net.IPv4(192, 168, 0, 1), Port: 21105}
	c := reg.Touch(addr, now, false)
	if reg.CurrentLoco(c.Key) != 0 {
		t.Fatal("expected zero with no history")
	}
	reg.SubscribeLoco(c.Key, 3)
	if reg.CurrentLoco(c.Key) != 3 {
		t.Fatalf("fallback = %d, want newest subscription 3", reg.CurrentLoco(c.Key))
	}
	reg.SetLastActiveLoco(c.Key, 7)
	if reg.CurrentLoco(c.Key) != 7 {
		t.Fatalf("active = %d, want 7", reg.CurrentLoco(c.Key))
	}
}
