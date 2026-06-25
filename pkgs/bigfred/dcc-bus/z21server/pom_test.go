package z21server

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

func buildPOMReadByte(locoAddr uint16, cvWire int) []byte {
	adrMSB := byte((locoAddr >> 8) & 0x3F)
	if locoAddr >= 128 {
		adrMSB |= 0xC0
	}
	adrLSB := byte(locoAddr & 0xFF)
	db3 := byte(pomReadByteOption | byte((cvWire>>8)&0x03))
	db4 := byte(cvWire & 0xFF)
	x := []byte{0xE6, 0x30, adrMSB, adrLSB, db3, db4, 0}
	x = append(x, xorSum(x))
	return buildXBusReply(x)
}

func TestVirtualPOMReadWriteUnpaired(t *testing.T) {
	client := &Client{Key: "test"}
	write := buildPOMWriteByte(3, 5, 42)
	s := &Server{pairing: NewPairingHandler(nil, 1, 1, NewRegistry(), nil)}
	s.handlePOMWrite(context.Background(), client, write)

	read := buildPOMReadByte(3, 5)
	loco, cvWire, ok := parsePOMReadByte(read)
	if !ok || loco != 3 || cvWire != 5 {
		t.Fatalf("parse read: %v %d %d", ok, loco, cvWire)
	}
	v, found := client.getVirtualCV(3, 5)
	if !found || v != 42 {
		t.Fatalf("virtual cv: found=%v v=%d", found, v)
	}
}

func TestPOMWriteIgnoredWhenPaired(t *testing.T) {
	client := &Client{
		Key:    "test",
		Paired: &contract.Z21PairingActiveWire{UserID: 1},
	}
	s := &Server{pairing: NewPairingHandler(nil, 1, 1, NewRegistry(), nil)}
	s.handlePOMWrite(context.Background(), client, buildPOMWriteByte(3, 5, 99))
	if _, found := client.getVirtualCV(3, 5); found {
		t.Fatal("paired client should not get virtual cv writes")
	}
}

func TestPOMReadVirtualValue(t *testing.T) {
	s := &Server{}
	client := &Client{Key: "test"}
	client.setVirtualCV(3, 10, 77)

	reply := buildCVResultReply(10, 0)
	s.handlePOMRead(context.Background(), &client.Addr, client, buildPOMReadByte(3, 10))
	_ = reply

	value, found := client.getVirtualCV(3, 10)
	if !found || value != 77 {
		t.Fatalf("virtual store: %d found=%v", value, found)
	}
}

func TestBuildCVResultReply(t *testing.T) {
	pkt := buildCVResultReply(10, 42)
	if pkt[4] != 0x64 || pkt[5] != 0x14 || pkt[8] != 42 {
		t.Fatalf("cv result: % x", pkt[4:10])
	}
}

func TestRMBusGetDataEmptyReply(t *testing.T) {
	reply := buildRMBusDataReply(1)
	if len(reply) != 15 {
		t.Fatalf("len=%d want 15: % x", len(reply), reply)
	}
	if binary.LittleEndian.Uint16(reply[2:4]) != HeaderRMBusDataChanged {
		t.Fatalf("header: % x", reply[2:4])
	}
	if reply[4] != 1 {
		t.Fatalf("group: %d", reply[4])
	}
	for i := 5; i < 15; i++ {
		if reply[i] != 0 {
			t.Fatalf("expected zero feedback at %d: % x", i, reply)
		}
	}
}

func TestLocoModeReplyDCC(t *testing.T) {
	reply := buildLocoModeReply(3)
	if len(reply) != 7 {
		t.Fatalf("len=%d", len(reply))
	}
	if binary.LittleEndian.Uint16(reply[2:4]) != HeaderGetLocoMode {
		t.Fatalf("header: % x", reply[2:4])
	}
	if reply[4] != 0 || reply[5] != 3 || reply[6] != 0 {
		t.Fatalf("addr/mode: % x", reply[4:])
	}
}

func TestParsePOMReadByte(t *testing.T) {
	pkt := buildPOMReadByte(42, 255)
	loco, cvWire, ok := parsePOMReadByte(pkt)
	if !ok || loco != 42 || cvWire != 255 {
		t.Fatalf("parse: ok=%v loco=%d cv=%d", ok, loco, cvWire)
	}
}
