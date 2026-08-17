package withrottle

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/remotes"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

type acquireOrderDrive struct{}

func (acquireOrderDrive) AuthorizeDrive(uint, uint16, remotes.DriveScope) bool { return true }
func (acquireOrderDrive) CollectHandsetDriveTargets(context.Context, uint, []uint16, remotes.DriveScope) []uint16 {
	return nil
}
func (acquireOrderDrive) ApplyHandsetIdleBrake(context.Context, remotes.HandsetSession, []uint16, remotes.DriveScope) {
}
func (acquireOrderDrive) ApplyHandsetPilotEStop(context.Context, remotes.HandsetSession, uint16) {
}
func (acquireOrderDrive) TriggerLayoutRadioStop(context.Context, uint, string) error { return nil }
func (acquireOrderDrive) ReadLocoCV(uint16, commandstation.CVNum) (int, error) {
	return 0, nil
}
func (acquireOrderDrive) SetSpeed(context.Context, remotes.ThrottleActor, remotes.ThrottleResponder, contract.LocoSetSpeedWire) remotes.CommandResult {
	return remotes.CommandResult{OK: true}
}
func (acquireOrderDrive) SetFunction(context.Context, remotes.ThrottleActor, remotes.ThrottleResponder, contract.LocoSetFunctionWire) remotes.CommandResult {
	return remotes.CommandResult{OK: true}
}
func (acquireOrderDrive) Subscribe(ctx context.Context, _ remotes.ThrottleActor, resp remotes.ThrottleResponder, addrs []uint16) remotes.CommandResult {
	resp.Subscribe(addrs...)
	_ = resp.SendLocoState(ctx, contract.LocoStateWire{
		Address: addrs[0],
		Speed:   42,
		Forward: false,
	})
	return remotes.CommandResult{OK: true}
}
func (acquireOrderDrive) Release(remotes.ThrottleActor, uint16) {}
func (acquireOrderDrive) LocoSnapshot(uint16) contract.LocoStateWire {
	return contract.LocoStateWire{}
}

func TestAcquireSnapshotFollowsDefaultDump(t *testing.T) {
	srv, err := New(Config{
		LayoutID:         1,
		CommandStationID: 1,
		SpeedSteps:       128,
		Drive:            acquireOrderDrive{},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	client := srv.registry.TouchByDeviceId("order-test", serverConn, time.Now().UTC())
	srv.registry.SetPaired(client.Key, &contract.RemoteSessionWire{
		ClientKey:        client.Key,
		UserID:           7,
		AllowAllVehicles: true,
	})

	done := make(chan struct{})
	go func() {
		srv.adapter.HandleAcquire(context.Background(), client, MCommand{
			ThrottleID: '0',
			Op:         MOpAdd,
			LocoKey:    "S3",
		})
		close(done)
	}()

	if err := clientConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(clientConn)
	defaultSpeedAt := -1
	snapshotSpeedAt := -1
	lineNo := 0
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r\n")
		switch {
		case line == "M0AS3<;>V0":
			defaultSpeedAt = lineNo
		case strings.HasPrefix(line, "M0AS3<;>V"):
			snapshotSpeedAt = lineNo
		}
		lineNo++
		if line == "M0AS3<;>R0" {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if defaultSpeedAt < 0 || snapshotSpeedAt <= defaultSpeedAt {
		t.Fatalf("default speed at %d, snapshot speed at %d", defaultSpeedAt, snapshotSpeedAt)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("acquire did not finish")
	}
}

type denyDrive struct{ acquireOrderDrive }

func (denyDrive) AuthorizeDrive(uint, uint16, remotes.DriveScope) bool { return false }

func TestHandleActionUnauthorizedSendsHM(t *testing.T) {
	srv, err := New(Config{
		LayoutID:         1,
		CommandStationID: 1,
		SpeedSteps:       128,
		Drive:            denyDrive{},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	client := srv.registry.TouchByDeviceId("deny-test", serverConn, time.Now().UTC())
	srv.registry.SetPaired(client.Key, &contract.RemoteSessionWire{
		ClientKey:        client.Key,
		UserID:           7,
		AllowAllVehicles: true,
	})
	srv.registry.withThrottle(client.Key, '0', func(tw *throttleWire) {
		if tw.locos == nil {
			tw.locos = make(map[uint16]string)
		}
		tw.locos[3] = "S3"
		tw.lastLoco = 3
	})

	done := make(chan struct{})
	go func() {
		srv.adapter.HandleAction(context.Background(), client, MCommand{
			ThrottleID: '0',
			Op:         MOpAction,
			LocoKey:    "S3",
			Properties: []string{"V10"},
		})
		close(done)
	}()

	if err := clientConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(clientConn)
	if !scanner.Scan() {
		t.Fatalf("no HM line: %v", scanner.Err())
	}
	if got := strings.TrimRight(scanner.Text(), "\r\n"); got != "HMNot authorized" {
		t.Fatalf("line = %q, want HMNot authorized", got)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("HandleAction did not finish")
	}
}
