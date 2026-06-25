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

func TestPairingDigitsFromFn(t *testing.T) {
	tests := []struct {
		fn   int
		want string
		ok   bool
	}{
		{0, "0", true},
		{5, "5", true},
		{9, "9", true},
		{10, "10", true},
		{32, "32", true},
		{-1, "", false},
		{33, "", false},
	}
	for _, tc := range tests {
		got, ok := pairingDigitsFromFn(tc.fn)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("fn %d: got %q ok=%v want %q ok=%v", tc.fn, got, ok, tc.want, tc.ok)
		}
	}
}

func TestBufferPairingFnSingleDigits(t *testing.T) {
	client := &Client{}
	presses := []int{1, 2, 2, 1, 4, 5}
	for i, fn := range presses {
		cv3, cv4, ready := client.BufferPairingFn(fn)
		if i < len(presses)-1 {
			if ready {
				t.Fatalf("press %d: unexpected ready", i)
			}
			continue
		}
		if !ready || cv3 != 122 || cv4 != 145 {
			t.Fatalf("final press: ready=%v cv3=%d cv4=%d", ready, cv3, cv4)
		}
	}
}

func TestBufferPairingFnTwoDigitShortcuts(t *testing.T) {
	client := &Client{}
	shortcuts := []int{12, 2, 1, 4, 5}
	for i, fn := range shortcuts {
		cv3, cv4, ready := client.BufferPairingFn(fn)
		if i < len(shortcuts)-1 {
			if ready {
				t.Fatalf("press %d: unexpected ready", i)
			}
			continue
		}
		if !ready || cv3 != 122 || cv4 != 145 {
			t.Fatalf("final press: ready=%v cv3=%d cv4=%d", ready, cv3, cv4)
		}
	}
}

func TestBufferPairingFnRingDropsOldest(t *testing.T) {
	client := &Client{}
	for _, fn := range []int{9, 9, 9, 9, 9, 9, 1, 2, 2, 1, 4, 5} {
		cv3, cv4, ready := client.BufferPairingFn(fn)
		if !ready {
			continue
		}
		if cv3 != 122 || cv4 != 145 {
			t.Fatalf("pair after ring: cv3=%d cv4=%d buf=%v", cv3, cv4, client.pairFnBuf)
		}
		return
	}
	t.Fatal("expected pairing after ring overwrites oldest presses")
}

func TestPairingFnRisingEdgesOnlyOnPress(t *testing.T) {
	client := &Client{}
	const groupF0F4 byte = 0x20

	risen := client.pairingFnRisingEdges(groupF0F4, 0x01) // F1 on
	if len(risen) != 1 || risen[0] != 1 {
		t.Fatalf("F1 on: got %v", risen)
	}
	if len(client.pairingFnRisingEdges(groupF0F4, 0x01)) != 0 {
		t.Fatal("held F1 should not count again")
	}
	risen = client.pairingFnRisingEdges(groupF0F4, 0x03) // F1+F2 on
	if len(risen) != 1 || risen[0] != 2 {
		t.Fatalf("F2 on while F1 held: got %v", risen)
	}
	if len(client.pairingFnRisingEdges(groupF0F4, 0x02)) != 0 {
		t.Fatal("F1 off should not count as digit entry")
	}
	if len(client.pairingFnRisingEdges(groupF0F4, 0x00)) != 0 {
		t.Fatal("all off should not count")
	}
}

func TestPairingHandlerCompletesOnFunctionKeys(t *testing.T) {
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
	addr := &net.UDPAddr{IP: net.IPv4(10, 0, 0, 3), Port: 40002}
	client := reg.Touch(addr, time.Now().UTC(), false)
	handler := NewPairingHandler(store, 1, 2, reg, nil)

	for _, fn := range []int{
		req.PairingCV3 / 100,
		(req.PairingCV3 / 10) % 10,
		req.PairingCV3 % 10,
		req.PairingCV4 / 100,
		(req.PairingCV4 / 10) % 10,
		req.PairingCV4 % 10,
	} {
		_, active := handler.HandleFn(ctx, client, fn)
		if active != nil {
			if active.UserID != 7 || active.PairingCV3 != req.PairingCV3 || active.PairingCV4 != req.PairingCV4 {
				t.Fatalf("unexpected active %+v", active)
			}
			return
		}
	}
	t.Fatal("expected pairing after six function presses")
}
