package withrottle

import (
	"bufio"
	"context"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dcc-bigfred/bigfred/pkgs/bigfred/contract"
	"github.com/dcc-bigfred/bigfred/pkgs/bigfred/remotes"
	"github.com/dcc-bigfred/proto/go/pkgs/commandstation"
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
func (acquireOrderDrive) TriggerStationTrackPowerOn(context.Context, uint, string) error {
	return nil
}
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
	return contract.LocoStateWire{Forward: true}
}

type speedProbeDrive struct {
	acquireOrderDrive
	mu sync.Mutex
	n  int
}

func (d *speedProbeDrive) SetSpeed(context.Context, remotes.ThrottleActor, remotes.ThrottleResponder, contract.LocoSetSpeedWire) remotes.CommandResult {
	d.mu.Lock()
	d.n++
	d.mu.Unlock()
	return remotes.CommandResult{OK: true}
}

func (d *speedProbeDrive) calls() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.n
}

func TestAcquireSnapshotFollowsDefaultDump(t *testing.T) {
	srv := startWT(t, Config{
		LayoutID:         1,
		CommandStationID: 1,
		SpeedSteps:       128,
		Drive:            acquireOrderDrive{},
	})
	conn, r := dialWT(t, srv)
	handshakeHU(t, conn, r, "order-test")
	pairDevice(srv, "order-test")
	wtWrite(t, conn, "M0+S3<;>S3")

	defaultSpeedAt := -1
	snapshotSpeedAt := -1
	lineNo := 0
	for i := 0; i < 80; i++ {
		line := wtRead(t, r)
		switch {
		case line == "M0AS3<;>V0":
			defaultSpeedAt = lineNo
		case strings.HasPrefix(line, "M0AS3<;>V"):
			snapshotSpeedAt = lineNo
		}
		lineNo++
		if line == "M0AS3<;>R0" && defaultSpeedAt >= 0 && snapshotSpeedAt > defaultSpeedAt {
			break
		}
	}
	if defaultSpeedAt < 0 || snapshotSpeedAt <= defaultSpeedAt {
		t.Fatalf("default speed at %d, snapshot speed at %d", defaultSpeedAt, snapshotSpeedAt)
	}
}

type snapshotSpeedDrive struct{ acquireOrderDrive }

func (snapshotSpeedDrive) LocoSnapshot(uint16) contract.LocoStateWire {
	return contract.LocoStateWire{Speed: 42, Forward: false}
}

func TestAcquireDumpUsesSnapshotSpeed(t *testing.T) {
	srv := startWT(t, Config{
		LayoutID:         1,
		CommandStationID: 1,
		SpeedSteps:       128,
		Drive:            snapshotSpeedDrive{},
	})
	conn, r := dialWT(t, srv)
	handshakeHU(t, conn, r, "snap-speed")
	pairDevice(srv, "snap-speed")
	wtWrite(t, conn, "M0+S3<;>S3")
	wantV := "M0AS3<;>V" + strconv.Itoa(wireSpeedFromDCC(42, 128))
	var dumpV, dumpR string
	for i := 0; i < 80; i++ {
		line := wtRead(t, r)
		if strings.HasPrefix(line, "M0AS3<;>V") && dumpV == "" {
			dumpV = line
		}
		if strings.HasPrefix(line, "M0AS3<;>R") && dumpR == "" {
			dumpR = line
		}
		if dumpV != "" && dumpR != "" {
			break
		}
	}
	if dumpV != wantV {
		t.Fatalf("dump speed %q want %q", dumpV, wantV)
	}
	if dumpR != "M0AS3<;>R0" {
		t.Fatalf("dump dir %q want M0AS3<;>R0", dumpR)
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
		srv.adapter.HandleAction(context.Background(), client, '0', "S3", 3, "V10")
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

type failSubscribeDrive struct{ acquireOrderDrive }

func (failSubscribeDrive) Subscribe(context.Context, remotes.ThrottleActor, remotes.ThrottleResponder, []uint16) remotes.CommandResult {
	return remotes.CommandResult{OK: false, Code: "busy"}
}

func TestHandleAcquireFailureReleasesLoco(t *testing.T) {
	srv := startWT(t, Config{
		LayoutID:         1,
		CommandStationID: 1,
		SpeedSteps:       128,
		Drive:            failSubscribeDrive{},
	})
	conn, r := dialWT(t, srv)
	handshakeHU(t, conn, r, "fail-acquire")
	pairDevice(srv, "fail-acquire")
	wtWrite(t, conn, "M0+S3<;>S3")
	var lines []string
	for i := 0; i < 80; i++ {
		line := wtRead(t, r)
		lines = append(lines, line)
		if strings.HasPrefix(line, "HM") {
			break
		}
	}
	var sawRelease, sawHM bool
	for _, line := range lines {
		if line == "M0-S3<;>" {
			sawRelease = true
		}
		if line == "HMbusy" {
			sawHM = true
		}
	}
	if !sawRelease || !sawHM {
		t.Fatalf("lines=%q want M0-S3 release and HMbusy", lines)
	}
	srv.registry.withThrottle("withrottle:fail-acquire", '0', func(tw *throttleWire) {
		if _, ok := tw.locos[3]; ok {
			t.Fatal("failed acquire left loco in throttle wire")
		}
	})
}

func TestJoinHMAndTruncate(t *testing.T) {
	if got := joinHM("", ""); got != "" {
		t.Fatalf("empty join = %q", got)
	}
	if got := joinHM("busy", ""); got != "busy" {
		t.Fatalf("code only = %q", got)
	}
	if got := joinHM("", "detail"); got != "detail" {
		t.Fatalf("detail only = %q", got)
	}
	got := truncateUTF8(strings.Repeat("x", 80), maxHMPayload)
	if len(got) != maxHMPayload {
		t.Fatalf("len=%d want %d", len(got), maxHMPayload)
	}
	if got == "" {
		t.Fatal("truncated payload must stay non-empty")
	}
}

func TestSendLocoErrorTruncatesPayload(t *testing.T) {
	srv, err := New(Config{LayoutID: 1, CommandStationID: 1, SpeedSteps: 128, Drive: acquireOrderDrive{}})
	if err != nil {
		t.Fatal(err)
	}
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	client := srv.registry.TouchByDeviceId("hm-trunc", serverConn, time.Now().UTC())
	resp := NewResponder(srv, client, '0')
	detail := strings.Repeat("e", 80)
	done := make(chan error, 1)
	go func() {
		done <- resp.SendLocoError(context.Background(), 3, "command_station_error", detail)
	}()
	if err := clientConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(clientConn)
	if !scanner.Scan() {
		t.Fatalf("no HM line: %v", scanner.Err())
	}
	line := strings.TrimRight(scanner.Text(), "\r\n")
	if !strings.HasPrefix(line, "HM") {
		t.Fatalf("line = %q", line)
	}
	if payload := line[2:]; len(payload) > maxHMPayload {
		t.Fatalf("HM payload len=%d want <= %d (%q)", len(payload), maxHMPayload, payload)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestSendLocoErrorSkipsEmpty(t *testing.T) {
	srv, err := New(Config{LayoutID: 1, CommandStationID: 1, SpeedSteps: 128, Drive: acquireOrderDrive{}})
	if err != nil {
		t.Fatal(err)
	}
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	client := srv.registry.TouchByDeviceId("hm-empty", serverConn, time.Now().UTC())
	resp := NewResponder(srv, client, '0')
	if err := resp.SendLocoError(context.Background(), 3, "", ""); err != nil {
		t.Fatal(err)
	}
	if err := clientConn.SetReadDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 8)
	n, err := clientConn.Read(buf)
	if n != 0 {
		t.Fatalf("unexpected bytes %q", buf[:n])
	}
	if err == nil {
		t.Fatal("expected timeout, got line")
	}
}
