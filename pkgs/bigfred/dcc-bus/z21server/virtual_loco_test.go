package z21server

import (
	"context"
	"encoding/hex"
	"net"
	"testing"
	"time"
)

func TestUnpairedGetLocoInfoVirtualEcho(t *testing.T) {
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
		CommandStationID: 2,
		Bind:             "127.0.0.1",
		Port:             uint16(port),
	})
	if err != nil {
		t.Fatal(err)
	}

	go func() { _ = srv.Run(ctx) }()
	time.Sleep(50 * time.Millisecond)

	client, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	getLocoInfo, _ := hex.DecodeString("09004000e3f0001f0c")
	if _, err := client.Write(getLocoInfo); err != nil {
		t.Fatal(err)
	}
	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 64)
	n, err := client.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if n < 10 || buf[4] != 0xEF {
		t.Fatalf("expected LAN_X_LOCO_INFO (0xEF), got % x", buf[:n])
	}
	addr, ok := parseLocoAddr(buf, 5)
	if !ok || addr != 31 {
		t.Fatalf("loco addr = %d ok=%v, want 31", addr, ok)
	}
}

func TestUnpairedSetLocoDriveVirtualEcho(t *testing.T) {
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
		CommandStationID: 2,
		Bind:             "127.0.0.1",
		Port:             uint16(port),
		SpeedSteps:       128,
	})
	if err != nil {
		t.Fatal(err)
	}

	go func() { _ = srv.Run(ctx) }()
	time.Sleep(50 * time.Millisecond)

	client, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	const addr uint16 = 31
	db3 := encodeDriveDB3(25, true, 3)
	drivePkt := []byte{0x0a, 0x00, 0x40, 0x00, 0xe4, 0x13, 0x00, byte(addr), db3, 0x00}
	drivePkt[9] = xorSum(drivePkt[4:9])
	if _, err := client.Write(drivePkt); err != nil {
		t.Fatal(err)
	}
	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 64)
	_, err = client.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	speed, forward := decodeDriveDB3(3, buf[8])
	if speed != 25 || !forward {
		t.Fatalf("echo speed=%d forward=%v, want 25 true", speed, forward)
	}
	clients := srv.registry.Snapshot()
	if len(clients) == 0 {
		t.Fatal("expected registered client")
	}
	if !srv.virtual.HasClient(clients[0].Key) {
		t.Fatal("expected virtual loco state stored for client")
	}
}

func TestClearVirtualLocoRemovesStore(t *testing.T) {
	srv, err := New(Config{LayoutID: 1, CommandStationID: 2})
	if err != nil {
		t.Fatal(err)
	}
	key := "z21:10.0.0.1:40001"
	srv.virtual.SetSpeed(key, 31, 10, true)
	srv.clearVirtualLoco(key)
	if srv.virtual.HasClient(key) {
		t.Fatal("clearVirtualLoco should remove client state")
	}
}

func TestUnpairedSetLocoFunctionToggleVirtualEcho(t *testing.T) {
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
		CommandStationID: 2,
		Bind:             "127.0.0.1",
		Port:             uint16(port),
	})
	if err != nil {
		t.Fatal(err)
	}

	go func() { _ = srv.Run(ctx) }()
	time.Sleep(50 * time.Millisecond)

	client, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// WLANmaus F1 toggle on loco 31.
	fnPkt, _ := hex.DecodeString("0a004000e4f8001f8100")
	if _, err := client.Write(fnPkt); err != nil {
		t.Fatal(err)
	}
	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 64)
	n, err := client.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if n < 10 || buf[4] != 0xEF {
		t.Fatalf("expected LAN_X_LOCO_INFO (0xEF), got % x", buf[:n])
	}
	// F1 is encoded in DB4 bit 0 (see encodeFunctionBytes).
	if buf[9]&0x01 == 0 {
		t.Fatalf("expected F1 on in echo, DB4=%02x", buf[9])
	}
}
