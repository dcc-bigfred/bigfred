package withrottle

import (
	"bufio"
	"net"
	"strings"
	"testing"
	"time"
)

func TestHUTriggersInitialBurst(t *testing.T) {
	srv := startWT(t, Config{
		LayoutID:         1,
		CommandStationID: 1,
		HeartbeatSecs:    10,
		TrackPowerOn:     true,
	})
	conn, r := dialWT(t, srv)
	wtWrite(t, conn, "HUengine-driver")
	if got := wtRead(t, r); got != "VN2.0" {
		t.Fatalf("first line after HU: got %q want VN2.0", got)
	}
	if got := wtRead(t, r); got != "*10" {
		t.Fatalf("second line after HU: got %q want *10", got)
	}
}

func TestHUReconnectWithSameDeviceGetsFreshBurst(t *testing.T) {
	srv := startWT(t, Config{
		LayoutID:         1,
		CommandStationID: 1,
		HeartbeatSecs:    10,
		TrackPowerOn:     true,
	})

	first, _ := dialWT(t, srv)
	wtWrite(t, first, "HUsame-device")
	assertInitialBurst(t, first)

	second, _ := dialWT(t, srv)
	wtWrite(t, second, "HUsame-device")
	assertInitialBurst(t, second)
}

func assertInitialBurst(t *testing.T, conn net.Conn) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	r := bufio.NewReader(conn)
	sawRoster := false
	for i := 0; i < 5; i++ {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		line = strings.TrimRight(line, "\r\n")
		switch i {
		case 0:
			if line != "VN2.0" {
				t.Fatalf("first burst line: got %q want VN2.0", line)
			}
		case 1:
			if line != "*10" {
				t.Fatalf("second burst line: got %q want *10", line)
			}
		}
		sawRoster = sawRoster || strings.HasPrefix(line, "RL")
	}
	if !sawRoster {
		t.Fatal("initial burst did not contain roster")
	}
}
