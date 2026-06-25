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

func TestPairingHandlerCompletesOnCV3CV4(t *testing.T) {
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
		UserID:           7,
		AllowedAddrs:     []uint16{3},
	})
	if err != nil {
		t.Fatal(err)
	}

	reg := NewRegistry()
	addr := &net.UDPAddr{IP: net.IPv4(10, 0, 0, 2), Port: 40001}
	client := reg.Touch(addr, time.Now().UTC())
	handler := NewPairingHandler(store, 1, 2, reg)

	if _, active := handler.Handle(ctx, client, 2, req.PairingCV3); active != nil {
		t.Fatal("expected incomplete after CV3 only")
	}
	_, active := handler.Handle(ctx, client, 3, req.PairingCV4)
	if active == nil || active.UserID != 7 {
		t.Fatalf("expected paired session, got %+v", active)
	}
	if client.Paired == nil || client.Paired.ClientKey != client.Key {
		t.Fatalf("client mirror: %+v", client.Paired)
	}
}

func TestPOMWriteByteParse(t *testing.T) {
	pkt := buildPOMWriteByte(99, 3, 145)
	cvWire, value, ok := parsePOMWriteByte(pkt)
	if !ok || cvWire != 3 || value != 145 {
		t.Fatalf("parse pom: ok=%v cv=%d val=%d", ok, cvWire, value)
	}
}
