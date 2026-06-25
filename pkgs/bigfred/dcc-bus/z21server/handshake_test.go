package z21server

import (
	"encoding/binary"
	"testing"
)

func TestBuildGetVersionReply(t *testing.T) {
	pkt := buildGetVersionReply()
	if len(pkt) != 9 {
		t.Fatalf("len=%d want 9: % x", len(pkt), pkt)
	}
	if binary.LittleEndian.Uint16(pkt[2:4]) != HeaderXBus {
		t.Fatalf("header: % x", pkt)
	}
	if pkt[4] != 0x63 || pkt[5] != 0x21 || pkt[6] != XBusProtocolVersion || pkt[7] != CmdStationIDZ21 {
		t.Fatalf("payload: % x", pkt[4:9])
	}
	if pkt[8] != xorSum(pkt[4:8]) {
		t.Fatalf("xor: got %02x want %02x", pkt[8], xorSum(pkt[4:8]))
	}
}

func TestBuildFirmwareVersionReply(t *testing.T) {
	pkt := buildFirmwareVersionReply()
	if pkt[4] != 0xF3 || pkt[5] != 0x0A || pkt[6] != FirmwareVersionMSB || pkt[7] != FirmwareVersionLSB {
		t.Fatalf("payload: % x", pkt[4:9])
	}
}

func TestBuildHWInfoReplyRocoIdentity(t *testing.T) {
	pkt := buildHWInfoReply(HwTypeZ21Black, FirmwareBCD)
	if binary.LittleEndian.Uint32(pkt[4:8]) != HwTypeZ21Black {
		t.Fatalf("hw type: % x", pkt[4:8])
	}
	if binary.LittleEndian.Uint32(pkt[8:12]) != FirmwareBCD {
		t.Fatalf("fw bcd: % x", pkt[8:12])
	}
}

func TestHandshakeReplyXBusProbes(t *testing.T) {
	s := &Server{cfg: Config{LayoutID: 2, CommandStationID: 1, Serial: 0x12345678}}

	versionReq := []byte{0x07, 0x00, 0x40, 0x00, 0x21, 0x21, 0x00}
	reply, ok := s.handshakeReply(versionReq)
	if !ok || reply[4] != 0x63 {
		t.Fatalf("GET_VERSION: ok=%v reply=% x", ok, reply)
	}

	fwReq := []byte{0x07, 0x00, 0x40, 0x00, 0xF1, 0x0A, 0xFB}
	reply, ok = s.handshakeReply(fwReq)
	if !ok || reply[4] != 0xF3 {
		t.Fatalf("GET_FIRMWARE: ok=%v reply=% x", ok, reply)
	}
}

func TestRocoVirtualSerial(t *testing.T) {
	if got := rocoVirtualSerial(2, 1); got != 258_002_001 {
		t.Fatalf("serial=%d", got)
	}
}
