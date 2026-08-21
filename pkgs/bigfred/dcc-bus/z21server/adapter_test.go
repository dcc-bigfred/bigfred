package z21server

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/remotes"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

type stubInboundDrive struct {
	estopAddr   uint16
	releaseAddr uint16
	setSpeed    *contract.LocoSetSpeedWire
	authorized  bool
}

func (s *stubInboundDrive) AuthorizeDrive(uint, uint16, remotes.DriveScope) bool {
	return s.authorized
}

func (s *stubInboundDrive) CollectHandsetDriveTargets(context.Context, uint, []uint16, remotes.DriveScope) []uint16 {
	return nil
}

func (s *stubInboundDrive) ApplyHandsetIdleBrake(context.Context, remotes.HandsetSession, []uint16, remotes.DriveScope) {
}

func (s *stubInboundDrive) ApplyHandsetPilotEStop(_ context.Context, _ remotes.HandsetSession, addr uint16) {
	s.estopAddr = addr
}

func (s *stubInboundDrive) TriggerLayoutRadioStop(context.Context, uint, string) error {
	return nil
}

func (s *stubInboundDrive) TriggerStationTrackPowerOn(context.Context, uint, string) error {
	return nil
}

func (s *stubInboundDrive) ReadLocoCV(uint16, commandstation.CVNum) (int, error) {
	return 0, nil
}

func (s *stubInboundDrive) SetSpeed(_ context.Context, _ remotes.ThrottleActor, _ remotes.ThrottleResponder, req contract.LocoSetSpeedWire) remotes.CommandResult {
	cp := req
	s.setSpeed = &cp
	return remotes.CommandResult{OK: true}
}

func (s *stubInboundDrive) SetFunction(context.Context, remotes.ThrottleActor, remotes.ThrottleResponder, contract.LocoSetFunctionWire) remotes.CommandResult {
	return remotes.CommandResult{OK: true}
}

func (s *stubInboundDrive) Subscribe(context.Context, remotes.ThrottleActor, remotes.ThrottleResponder, []uint16) remotes.CommandResult {
	return remotes.CommandResult{OK: true}
}

func (s *stubInboundDrive) Release(_ remotes.ThrottleActor, addr uint16) {
	s.releaseAddr = addr
}

func (s *stubInboundDrive) LocoSnapshot(uint16) contract.LocoStateWire {
	return contract.LocoStateWire{}
}

func TestHandleSetLocoDriveEStopRoutesToPilotEStop(t *testing.T) {
	t.Parallel()
	const addr uint16 = 31

	srv, err := New(Config{LayoutID: 1, CommandStationID: 2, SpeedSteps: 128})
	if err != nil {
		t.Fatal(err)
	}
	drive := &stubInboundDrive{authorized: true}
	adapter := NewAdapter(srv, drive)

	reg := srv.registry
	remote := &net.UDPAddr{IP: net.IPv4(10, 0, 0, 1), Port: 40001}
	client := reg.Touch(remote, time.Now().UTC(), false)
	reg.SetPaired(client.Key, &contract.Z21PairingActiveWire{
		ClientKey: client.Key,
		UserID:    7,
	})

	pkt := buildSetLocoDrivePkt(addr, 3, encodeDriveDB3(1, true, 3))
	adapter.HandleSetLocoDrive(context.Background(), client, pkt)

	if drive.estopAddr != addr {
		t.Fatalf("estop addr = %d, want %d", drive.estopAddr, addr)
	}
	if drive.setSpeed != nil {
		t.Fatalf("SetSpeed called with %+v, want estop path only", *drive.setSpeed)
	}
}

func TestHandleSetLocoDriveMovingSpeedUsesSetSpeed(t *testing.T) {
	t.Parallel()
	const addr uint16 = 31

	srv, err := New(Config{LayoutID: 1, CommandStationID: 2, SpeedSteps: 128})
	if err != nil {
		t.Fatal(err)
	}
	drive := &stubInboundDrive{authorized: true}
	adapter := NewAdapter(srv, drive)

	reg := srv.registry
	remote := &net.UDPAddr{IP: net.IPv4(10, 0, 0, 2), Port: 40002}
	client := reg.Touch(remote, time.Now().UTC(), false)
	reg.SetPaired(client.Key, &contract.Z21PairingActiveWire{
		ClientKey: client.Key,
		UserID:    7,
	})

	pkt := buildSetLocoDrivePkt(addr, 3, encodeDriveDB3(10, true, 3))
	adapter.HandleSetLocoDrive(context.Background(), client, pkt)

	if drive.estopAddr != 0 {
		t.Fatalf("unexpected estop for addr %d", drive.estopAddr)
	}
	if drive.setSpeed == nil || drive.setSpeed.Speed != 10 || drive.setSpeed.Address != addr {
		t.Fatalf("SetSpeed = %+v, want speed 10 for addr %d", drive.setSpeed, addr)
	}
}

func TestHandleSetLocoEStopRoutesToPilotEStop(t *testing.T) {
	t.Parallel()
	const addr uint16 = 31
	srv, err := New(Config{LayoutID: 1, CommandStationID: 2, SpeedSteps: 128})
	if err != nil {
		t.Fatal(err)
	}
	drive := &stubInboundDrive{authorized: true}
	adapter := NewAdapter(srv, drive)
	client := srv.registry.Touch(
		&net.UDPAddr{IP: net.IPv4(10, 0, 0, 3), Port: 40003},
		time.Now().UTC(),
		false,
	)
	srv.registry.SetPaired(client.Key, &contract.Z21PairingActiveWire{
		ClientKey: client.Key,
		UserID:    7,
	})
	x := []byte{0x92, 0x00, byte(addr)}
	x = append(x, xorSum(x))
	adapter.HandleSetLocoEStop(context.Background(), client, buildXBusReply(x))
	if drive.estopAddr != addr {
		t.Fatalf("estop addr = %d, want %d", drive.estopAddr, addr)
	}
}

func TestHandlePurgeLocoReleasesAndUnsubscribes(t *testing.T) {
	t.Parallel()
	const addr uint16 = 31
	srv, err := New(Config{LayoutID: 1, CommandStationID: 2, SpeedSteps: 128})
	if err != nil {
		t.Fatal(err)
	}
	drive := &stubInboundDrive{authorized: true}
	adapter := NewAdapter(srv, drive)
	client := srv.registry.Touch(
		&net.UDPAddr{IP: net.IPv4(10, 0, 0, 4), Port: 40004},
		time.Now().UTC(),
		false,
	)
	srv.registry.SubscribeLoco(client.Key, addr)
	srv.registry.SetPaired(client.Key, &contract.Z21PairingActiveWire{
		ClientKey:        client.Key,
		UserID:           7,
		AllowAllVehicles: true,
	})
	x := []byte{0xE3, 0x44, 0x00, byte(addr)}
	x = append(x, xorSum(x))
	adapter.HandlePurgeLoco(client, buildXBusReply(x))
	if drive.releaseAddr != addr {
		t.Fatalf("release addr = %d, want %d", drive.releaseAddr, addr)
	}
	if srv.registry.SubscribedTo(client.Key, addr) {
		t.Fatal("purged loco is still subscribed")
	}
}
