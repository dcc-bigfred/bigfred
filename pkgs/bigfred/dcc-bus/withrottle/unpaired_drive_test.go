package withrottle

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestUnpairedNonSentinelDriveSendsNotPaired(t *testing.T) {
	srv, conn, r := startUnpairedThrottle(t)
	defer conn.Close()

	wtWrite(t, conn, "HUtest-device")
	drainUntilLine(t, r, func(line string) bool { return strings.HasPrefix(line, "HT") })

	wtWrite(t, conn, fmt.Sprintf("M0AS5%sF10", propSep))
	if got := wtRead(t, r); got != "HMNot paired" {
		t.Fatalf("unpaired F0 on S5: got %q want HMNot paired", got)
	}

	wtWrite(t, conn, fmt.Sprintf("M0AS5%sV30", propSep))
	if got := wtRead(t, r); got != "HMNot paired" {
		t.Fatalf("unpaired V on S5: got %q", got)
	}

	wtWrite(t, conn, fmt.Sprintf("M0AS5%sR0", propSep))
	if got := wtRead(t, r); got != "HMNot paired" {
		t.Fatalf("unpaired R on S5: got %q", got)
	}

	key := locoKeyForAddr(srv.cfg.PairingAddr)
	wtWrite(t, conn, fmt.Sprintf("M0+%s%s", key, propSep))
	drainUntilLine(t, r, func(line string) bool { return strings.Contains(line, "<;>s1") })

	wtWrite(t, conn, fmt.Sprintf("M0AS5%sF10", propSep))
	if got := wtRead(t, r); got != "HMNot paired" {
		t.Fatalf("F0 on S5 while sentinel acquired: got %q", got)
	}

	wtWrite(t, conn, fmt.Sprintf("M0A%s%sF10", key, propSep))
	got := wtRead(t, r)
	if got == "HMNot paired" {
		t.Fatal("sentinel F0 must not send HMNot paired")
	}
}

func startUnpairedThrottle(t *testing.T) (*Server, net.Conn, *bufio.Reader) {
	t.Helper()
	srv := startWT(t, Config{
		LayoutID:         1,
		CommandStationID: 1,
		HeartbeatSecs:    10,
		TrackPowerOn:     true,
	})
	conn, r := dialWT(t, srv)
	return srv, conn, r
}

func TestClientPPAIgnored(t *testing.T) {
	_, conn, r := startUnpairedThrottle(t)
	defer conn.Close()
	wtWrite(t, conn, "HUppa")
	drainUntilLine(t, r, func(line string) bool { return strings.HasPrefix(line, "HT") })
	wtWrite(t, conn, "PPA0")
	_ = conn.SetReadDeadline(time.Now().Add(80 * time.Millisecond))
	buf := make([]byte, 8)
	n, _ := conn.Read(buf)
	if n != 0 {
		t.Fatalf("client PPA must not broadcast, got %q", buf[:n])
	}
}
