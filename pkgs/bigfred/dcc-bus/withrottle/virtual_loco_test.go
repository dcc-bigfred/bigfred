package withrottle

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"context"

	"github.com/dcc-bigfred/bigfred/pkgs/bigfred/contract"
)

func TestUnpairedSentinelSpeedVirtualEcho(t *testing.T) {
	srv := startWT(t, Config{
		LayoutID:         1,
		CommandStationID: 1,
		HeartbeatSecs:    10,
		TrackPowerOn:     true,
	})
	conn, r := dialWT(t, srv)
	handshakeHU(t, conn, r, "test-device")

	key := locoKeyForAddr(srv.cfg.PairingAddr)
	wtWrite(t, conn, fmt.Sprintf("M0+%s%s", key, propSep))
	drainUntilLine(t, r, func(line string) bool { return strings.Contains(line, "<;>s1") })

	wtWrite(t, conn, fmt.Sprintf("M0A%s%sV30", key, propSep))
	gotV := false
	for i := 0; i < 40; i++ {
		line := wtRead(t, r)
		if strings.Contains(line, "<;>V30") {
			gotV = true
			break
		}
	}
	if !gotV {
		t.Fatal("expected M…A…V30 echo for unpaired sentinel throttle")
	}
	if !srv.virtual.HasClient("withrottle:test-device") {
		t.Fatal("expected virtual loco state for WiThrottle client")
	}
}

func TestClearVirtualLocoRemovesStore(t *testing.T) {
	srv, err := New(Config{LayoutID: 1, CommandStationID: 1})
	if err != nil {
		t.Fatal(err)
	}
	key := "withrottle:device"
	srv.virtual.SetSpeed(key, srv.cfg.PairingAddr, 10, true)
	srv.clearVirtualLoco(key)
	if srv.virtual.HasClient(key) {
		t.Fatal("clearVirtualLoco should remove client state")
	}
}

func TestCommanderGetsNoEcho(t *testing.T) {
	srv := startWT(t, Config{
		LayoutID:         1,
		CommandStationID: 1,
		SpeedSteps:       128,
		Drive:            acquireOrderDrive{},
	})
	c1, r1 := dialWT(t, srv)
	handshakeHU(t, c1, r1, "origin")
	pairDevice(srv, "origin")
	c2, r2 := dialWT(t, srv)
	handshakeHU(t, c2, r2, "peer")
	pairDevice(srv, "peer")

	wtWrite(t, c1, "M0+S3<;>S3")
	drainUntilLine(t, r1, func(line string) bool { return strings.Contains(line, "<;>s1") })
	wtWrite(t, c2, "M0+S3<;>S3")
	drainUntilLine(t, r2, func(line string) bool { return strings.Contains(line, "<;>s1") })
	drainQuiet(t, r1, 80*time.Millisecond)
	drainQuiet(t, r2, 80*time.Millisecond)

	srv.OnLocoStateChanged(context.Background(), contract.LocoStateWire{Address: 3, Speed: 42, Forward: false}, "withrottle:origin")

	_ = c1.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	if _, err := r1.ReadString('\n'); err == nil {
		t.Fatal("commander must not receive fanout echo")
	}
	_ = c2.SetReadDeadline(time.Now().Add(time.Second))
	saw := false
	for i := 0; i < 20; i++ {
		line := wtRead(t, r2)
		if strings.Contains(line, "<;>V") {
			saw = true
			break
		}
	}
	if !saw {
		t.Fatal("peer must receive loco notify")
	}
}
