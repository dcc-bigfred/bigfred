package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/state"
	"github.com/keskad/loco/pkgs/bigfred/remotes"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

func TestAuthorizeHandsetDrive(t *testing.T) {
	rs, cleanup := testRedis(t)
	defer cleanup()
	r, err := NewRouter(context.Background(), Config{
		Station:          &commandstation.StubStation{},
		Hub:              &stubHub{},
		Redis:            rs,
		LayoutID:         2,
		CommandStationID: 1,
		SpeedSteps:       128,
		AllowedVehicles: contract.AllowedVehicles{
			LayoutID: 2,
			Vehicles: []contract.AllowedVehicle{
				{Addr: 3, ControllerUserIDs: []uint{9}},
				{Addr: 10, ControllerUserIDs: []uint{9}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	scopeAll := remotes.DriveScope{AllowAllVehicles: true}
	scopeAddr3 := remotes.DriveScope{AllowedAddrs: []uint16{3}}

	if !r.AuthorizeHandsetDrive(9, 3, scopeAll) {
		t.Fatal("expected allow-all for addr 3")
	}
	if r.AuthorizeHandsetDrive(9, 3, remotes.DriveScope{AllowedAddrs: []uint16{10}}) {
		t.Fatal("expected deny when addr not in scope")
	}
	if !r.AuthorizeHandsetDrive(9, 3, scopeAddr3) {
		t.Fatal("expected allowed addr in scope")
	}
	if r.AuthorizeHandsetDrive(99, 3, scopeAll) {
		t.Fatal("expected deny for user not on roster")
	}
}

func TestApplyHandsetPilotEStop_singleLoco(t *testing.T) {
	t.Parallel()
	var calledAddr uint16
	st := &commandstation.StubStation{
		SetSpeedFn: func(addr commandstation.LocoAddr, speed uint8, _ bool, _ uint8) error {
			calledAddr = uint16(addr)
			if speed != 1 {
				t.Fatalf("speed = %d, want 1", speed)
			}
			return nil
		},
	}
	rs, cleanup := testRedis(t)
	defer cleanup()
	ctx := context.Background()
	const addr uint16 = 5

	if err := rs.StoreLocoCurrentState(ctx, contract.LocoStateWire{
		Address: addr,
		Speed:   30,
	}, StateTTL); err != nil {
		t.Fatal(err)
	}

	r, err := NewRouter(ctx, Config{
		Station:          st,
		Hub:              &stubHub{},
		Redis:            rs,
		LayoutID:         2,
		CommandStationID: 1,
		SpeedSteps:       128,
		AllowedVehicles: contract.AllowedVehicles{
			LayoutID: 2,
			Vehicles: []contract.AllowedVehicle{{Addr: addr}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	r.ApplyHandsetPilotEStop(ctx, remotes.HandsetSession{ClientKey: "192.168.0.1:1234", UserID: 7}, addr)
	if calledAddr != addr {
		t.Fatalf("SetSpeed addr = %d, want %d", calledAddr, addr)
	}
}

func TestTriggerLayoutRadioStop_publishes(t *testing.T) {
	t.Parallel()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	const layoutID uint = 3
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	rs := state.NewRedis(client, layoutID, 1)
	ctx := context.Background()

	sub := client.Subscribe(ctx, contract.LayoutRadioStopChannel(layoutID))
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatal(err)
	}
	ch := sub.Channel()

	r := &Router{redis: rs}
	if err := r.TriggerLayoutRadioStop(ctx, 11, "handset"); err != nil {
		t.Fatal(err)
	}

	select {
	case msg := <-ch:
		if msg.Payload == "" {
			t.Fatal("empty payload")
		}
	case <-time.After(time.Second):
		t.Fatal("expected radio stop publish")
	}
}
