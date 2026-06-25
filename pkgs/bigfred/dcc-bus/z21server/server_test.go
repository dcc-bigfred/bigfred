package z21server_test

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/z21server"
)

func TestHandshakeReplies(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ln, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	port := ln.LocalAddr().(*net.UDPAddr).Port
	_ = ln.Close()

	srv, err := z21server.New(z21server.Config{
		LayoutID:         1,
		CommandStationID: 2,
		Bind:             "127.0.0.1",
		Port:             uint16(port),
		Serial:           0x12345678,
	})
	if err != nil {
		t.Fatal(err)
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Run(ctx) }()
	defer cancel()
	time.Sleep(50 * time.Millisecond)

	client, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	serialReq := []byte{0x04, 0x00, 0x10, 0x00}
	if _, err := client.Write(serialReq); err != nil {
		t.Fatal(err)
	}
	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 64)
	n, err := client.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if binary.LittleEndian.Uint16(buf[2:4]) != 0x0010 {
		t.Fatalf("serial header: % x", buf[:n])
	}
	if binary.LittleEndian.Uint32(buf[4:8]) != 0x12345678 {
		t.Fatalf("serial value: % x", buf[:n])
	}

	hwReq := []byte{0x04, 0x00, 0x1A, 0x00}
	if _, err := client.Write(hwReq); err != nil {
		t.Fatal(err)
	}
	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	n, err = client.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if binary.LittleEndian.Uint16(buf[2:4]) != 0x001A {
		t.Fatalf("hwinfo header: % x", buf[:n])
	}
	if binary.LittleEndian.Uint32(buf[4:8]) != z21server.HwTypeZ21Black {
		t.Fatalf("hw type: % x", buf[4:8])
	}
	if binary.LittleEndian.Uint32(buf[8:12]) != z21server.FirmwareBCD {
		t.Fatalf("fw bcd: % x", buf[8:12])
	}

	versionReq := []byte{0x07, 0x00, 0x40, 0x00, 0x21, 0x21, 0x00}
	if _, err := client.Write(versionReq); err != nil {
		t.Fatal(err)
	}
	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	n, err = client.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if buf[4] != 0x63 || buf[7] != z21server.CmdStationIDZ21 {
		t.Fatalf("GET_VERSION reply: % x", buf[:n])
	}

	fwReq := []byte{0x07, 0x00, 0x40, 0x00, 0xF1, 0x0A, 0xFB}
	if _, err := client.Write(fwReq); err != nil {
		t.Fatal(err)
	}
	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	n, err = client.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if buf[4] != 0xF3 || buf[6] != z21server.FirmwareVersionMSB || buf[7] != z21server.FirmwareVersionLSB {
		t.Fatalf("GET_FIRMWARE reply: % x", buf[:n])
	}

	sysReq := []byte{0x04, 0x00, 0x85, 0x00}
	if _, err := client.Write(sysReq); err != nil {
		t.Fatal(err)
	}
	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	n, err = client.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if binary.LittleEndian.Uint16(buf[2:4]) != 0x0084 {
		t.Fatalf("systemstate header: % x", buf[:n])
	}
	if n < 20 {
		t.Fatalf("short systemstate reply: % x", buf[:n])
	}

	if srv.RegistryForTest().Len() != 1 {
		t.Fatalf("expected implicit login, registry len=%d", srv.RegistryForTest().Len())
	}
}
