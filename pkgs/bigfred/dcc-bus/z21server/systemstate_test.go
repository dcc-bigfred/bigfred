package z21server

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestDefaultSystemStateEncode(t *testing.T) {
	st := DefaultSystemState()
	data := st.encode()
	if len(data) != 16 {
		t.Fatalf("len=%d", len(data))
	}
	if int16(binary.LittleEndian.Uint16(data[0:2])) != emuMainCurrentMA {
		t.Fatalf("main current: %d", int16(binary.LittleEndian.Uint16(data[0:2])))
	}
	if int16(binary.LittleEndian.Uint16(data[4:6])) != emuFilteredMainCurrentMA {
		t.Fatalf("filtered current: %d", int16(binary.LittleEndian.Uint16(data[4:6])))
	}
	if int16(binary.LittleEndian.Uint16(data[6:8])) != emuTemperatureC {
		t.Fatalf("temperature: %d", int16(binary.LittleEndian.Uint16(data[6:8])))
	}
	if binary.LittleEndian.Uint16(data[8:10]) != emuSupplyVoltageMV {
		t.Fatalf("supply: %d", binary.LittleEndian.Uint16(data[8:10]))
	}
	if binary.LittleEndian.Uint16(data[10:12]) != emuVCCVoltageMV {
		t.Fatalf("vcc: %d", binary.LittleEndian.Uint16(data[10:12]))
	}
	if data[12]&csTrackVoltageOff != 0 {
		t.Fatalf("track voltage off: centralState=%02x", data[12])
	}
	if data[15] != capDCC|capLocoCmds|capAccessoryCmds {
		t.Fatalf("capabilities: %02x", data[15])
	}
}

func TestSystemStateGetDataReplyFields(t *testing.T) {
	s := &Server{cfg: Config{}}
	pkt, ok := s.handshakeReply([]byte{0x04, 0x00, 0x85, 0x00})
	if !ok {
		t.Fatal("expected reply")
	}
	if binary.LittleEndian.Uint16(pkt[2:4]) != HeaderSystemStateData {
		t.Fatalf("header: % x", pkt)
	}
	data := pkt[4:]
	if int16(binary.LittleEndian.Uint16(data[0:2])) != emuMainCurrentMA {
		t.Fatalf("main current in reply: % x", data)
	}
}

func TestBroadcastSystemStateFlagPushesUpdate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ln, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	port := ln.LocalAddr().(*net.UDPAddr).Port
	_ = ln.Close()

	srv, err := New(Config{
		LayoutID:         1,
		CommandStationID: 1,
		Bind:             "127.0.0.1",
		Port:             uint16(port),
	})
	if err != nil {
		t.Fatal(err)
	}

	go func() { _ = srv.Run(ctx) }()
	defer cancel()
	time.Sleep(50 * time.Millisecond)

	client, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	flagsReq := []byte{0x08, 0x00, 0x50, 0x00, 0x00, 0x01, 0x00, 0x00}
	if _, err := client.Write(flagsReq); err != nil {
		t.Fatal(err)
	}
	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 64)
	n, err := client.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if binary.LittleEndian.Uint16(buf[2:4]) != HeaderSystemStateData {
		t.Fatalf("expected systemstate push, got: % x", buf[:n])
	}
	if int16(binary.LittleEndian.Uint16(buf[4:6])) != emuMainCurrentMA {
		t.Fatalf("main current push: % x", buf[:n])
	}
}
