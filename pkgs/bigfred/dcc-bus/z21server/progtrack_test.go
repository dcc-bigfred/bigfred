package z21server

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/keskad/loco/pkgs/bigfred/z21pairing"
)

func buildProgTrackCVWrite(cvWire int, value byte) []byte {
	x := []byte{0x24, 0x12, byte(cvWire >> 8), byte(cvWire), value}
	x = append(x, xorSum(x))
	return buildXBusReply(x)
}

func buildProgTrackCVRead(cvWire int) []byte {
	x := []byte{0x23, 0x11, byte(cvWire >> 8), byte(cvWire)}
	x = append(x, xorSum(x))
	return buildXBusReply(x)
}

func TestParseProgTrackCVWrite(t *testing.T) {
	pkt := buildProgTrackCVWrite(cvWireCV3, 122)
	cvWire, value, ok := parseProgTrackCVWrite(pkt)
	if !ok || cvWire != cvWireCV3 || value != 122 {
		t.Fatalf("parse: ok=%v cvWire=%d value=%d", ok, cvWire, value)
	}
}

func TestProgTrackCVWritePairsUnpairedClient(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	store := z21pairing.NewStore(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	ctx := context.Background()
	req, err := store.CreatePairingRequest(ctx, z21pairing.CreatePairingRequestInput{
		LayoutID:         1,
		CommandStationID: 2,
		UserID:           3,
		AllowedAddrs:     []uint16{5},
	})
	if err != nil {
		t.Fatal(err)
	}

	reg := NewRegistry()
	addr := &net.UDPAddr{IP: net.IPv4(10, 0, 0, 9), Port: 40009}
	client := reg.Touch(addr, time.Now().UTC(), false)
	handler := NewPairingHandler(store, 1, 2, reg, nil)

	writeCV3 := buildProgTrackCVWrite(cvWireCV3, byte(req.PairingCV3))
	cvWire, value, ok := parseProgTrackCVWrite(writeCV3)
	if !ok {
		t.Fatal("parse CV3 write")
	}
	client.setVirtualCV(progTrackLoco, cvWire, byte(value))
	if _, active := handler.Handle(ctx, client, cvWire, value); active != nil {
		t.Fatal("expected incomplete after CV3 only")
	}

	writeCV4 := buildProgTrackCVWrite(cvWireCV4, byte(req.PairingCV4))
	cvWire, value, ok = parseProgTrackCVWrite(writeCV4)
	if !ok {
		t.Fatal("parse CV4 write")
	}
	client.setVirtualCV(progTrackLoco, cvWire, byte(value))
	_, active := handler.Handle(ctx, client, cvWire, value)
	if active == nil || active.UserID != 3 {
		t.Fatalf("expected paired, got %+v", active)
	}
}

func TestProgTrackCVReadVirtualValue(t *testing.T) {
	reg := NewRegistry()
	addr := &net.UDPAddr{IP: net.IPv4(10, 0, 0, 10), Port: 40010}
	client := reg.Touch(addr, time.Now().UTC(), false)
	client.setVirtualCV(progTrackLoco, cvWireCV4, 145)

	s := &Server{registry: reg}
	s.handleProgTrackCVRead(context.Background(), addr, client, buildProgTrackCVRead(cvWireCV4))
	// no conn — handler should not panic; virtual read path exercised via direct call
	value, found := client.getVirtualCV(progTrackLoco, cvWireCV4)
	if !found || value != 145 {
		t.Fatalf("virtual cv: found=%v value=%d", found, value)
	}
}
