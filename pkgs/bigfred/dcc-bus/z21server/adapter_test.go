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
	estopAddr  uint16
	setSpeed   *contract.LocoSetSpeedWire
	authorized bool
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

func (s *stubInboundDrive) Release(remotes.ThrottleActor, uint16) {}

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
