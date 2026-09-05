package z21server

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/dcc-bigfred/bigfred/pkgs/bigfred/contract"
	"github.com/dcc-bigfred/bigfred/pkgs/bigfred/remotepairing"
)

func startZ21(t *testing.T, cfg Config) *Server {
	t.Helper()
	ln, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	port := ln.LocalAddr().(*net.UDPAddr).Port
	_ = ln.Close()
	cfg.LayoutID = 1
	cfg.CommandStationID = 2
	cfg.Bind = "127.0.0.1"
	cfg.Port = uint16(port)
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = srv.Run(ctx) }()
	time.Sleep(50 * time.Millisecond)
	return srv
}

func dialZ21(t *testing.T, srv *Server) *net.UDPConn {
	t.Helper()
	client, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: int(srv.cfg.Port)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestUnpairedDriveNeverReachesRouter(t *testing.T) {
	drive := &stubInboundDrive{authorized: true}
	srv := startZ21(t, Config{Drive: drive, SpeedSteps: 128})
	client := dialZ21(t, srv)
	const addr uint16 = 31
	db3 := encodeDriveDB3(25, true, 3)
	pkt := []byte{0x0a, 0x00, 0x40, 0x00, 0xe4, 0x13, 0x00, byte(addr), db3, 0x00}
	pkt[9] = xorSum(pkt[4:9])
	if _, err := client.Write(pkt); err != nil {
		t.Fatal(err)
	}
	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 64)
	if _, err := client.Read(buf); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	if drive.setSpeed != nil {
		t.Fatalf("unpaired SetSpeed reached router: %+v", *drive.setSpeed)
	}
	if drive.estopAddr != 0 {
		t.Fatal("unpaired estop reached router")
	}
}

func TestLogoffUnpairs(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	store := remotepairing.NewStore(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	ctx := context.Background()
	req, err := store.CreateZ21PairingRequest(ctx, remotepairing.CreateZ21PairingInput{
		LayoutID:         1,
		CommandStationID: 2,
		UserID:           7,
		AllowAllVehicles: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := startZ21(t, Config{Store: store})
	client := dialZ21(t, srv)
	if _, err := client.Write([]byte{0x04, 0x00, 0x10, 0x00}); err != nil {
		t.Fatal(err)
	}
	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 64)
	if _, err := client.Read(buf); err != nil {
		t.Fatal(err)
	}
	clients := srv.registry.Snapshot()
	if len(clients) != 1 {
		t.Fatalf("clients=%d", len(clients))
	}
	key := clients[0].Key
	if _, ok, _, err := store.PairViaCV3CV4(ctx, 1, 2, req.PairingCV3, req.PairingCV4, key, contract.NowMS()); err != nil || !ok {
		t.Fatalf("pair: ok=%v err=%v", ok, err)
	}
	srv.registry.SetPaired(key, &contract.Z21PairingActiveWire{ClientKey: key, UserID: 7})
	if _, err := client.Write([]byte{0x04, 0x00, 0x30, 0x00}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok, err := store.GetActiveByClientKey(ctx, 1, 2, key); err == nil && !ok {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("LOGOFF should unpair Redis session")
}

func TestIPStickinessKeying(t *testing.T) {
	srv := startZ21(t, Config{IPStickiness: true})
	c1 := dialZ21(t, srv)
	c2 := dialZ21(t, srv)
	pkt := []byte{0x04, 0x00, 0x10, 0x00}
	if _, err := c1.Write(pkt); err != nil {
		t.Fatal(err)
	}
	if _, err := c2.Write(pkt); err != nil {
		t.Fatal(err)
	}
	time.Sleep(80 * time.Millisecond)
	clients := srv.registry.Snapshot()
	if len(clients) != 1 {
		t.Fatalf("sticky IP should collapse to one client, got %d", len(clients))
	}
	if clients[0].Endpoint != "127.0.0.1" {
		t.Fatalf("endpoint=%q want 127.0.0.1", clients[0].Endpoint)
	}
}
