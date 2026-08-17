package z21server

import (
	"bytes"
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

func TestIsSetTrackPowerOn(t *testing.T) {
	t.Parallel()
	pkt := []byte{0x07, 0x00, 0x40, 0x00, 0x21, 0x81, 0xA0}
	if !isSetTrackPowerOn(HeaderXBus, pkt) {
		t.Fatal("expected LAN_X_SET_TRACK_POWER_ON")
	}
	if isSetTrackPowerOn(HeaderXBus, []byte{0x07, 0x00, 0x40, 0x00, 0x21, 0x80, 0xA1}) {
		t.Fatal("track power off is not track power on")
	}
}

func TestBuildBCStoppedReply(t *testing.T) {
	t.Parallel()
	reply := buildBCStoppedReply()
	if len(reply) != 7 {
		t.Fatalf("len = %d, want 7", len(reply))
	}
	if binary.LittleEndian.Uint16(reply[2:4]) != HeaderXBus {
		t.Fatalf("header = %x", reply[2:4])
	}
	if reply[4] != 0x81 {
		t.Fatalf("x[0] = %#x, want 0x81", reply[4])
	}
	if reply[5] != 0x00 {
		t.Fatalf("DB0 = %#x, want 0", reply[5])
	}
	if reply[6] != xorSum([]byte{0x81, 0x00}) {
		t.Fatalf("checksum = %#x", reply[6])
	}
}

func TestBuildTrackPowerBroadcasts(t *testing.T) {
	t.Parallel()
	if got := buildBCTrackPowerReply(false); !bytes.Equal(got, []byte{0x07, 0x00, 0x40, 0x00, 0x61, 0x00, 0x61}) {
		t.Fatalf("off = % X", got)
	}
	if got := buildBCTrackPowerReply(true); !bytes.Equal(got, []byte{0x07, 0x00, 0x40, 0x00, 0x61, 0x01, 0x60}) {
		t.Fatalf("on = % X", got)
	}
}

func TestRegistryCurrentLoco(t *testing.T) {
	t.Parallel()
	reg := NewRegistry(nil, nil)
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
