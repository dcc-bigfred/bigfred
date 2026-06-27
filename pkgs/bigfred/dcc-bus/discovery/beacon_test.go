package discovery

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/z21server"
)

func TestRunBeaconDeliversFrame(t *testing.T) {
	const port = 42105
	frame := z21server.SerialReply(z21server.VirtualSerial(1, 2))
	dest := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port}

	ln, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- runBeacon(ctx, dest, frame, nil)
	}()

	deadline := time.Now().Add(3 * time.Second)
	buf := make([]byte, 64)
	var got bool
	for time.Now().Before(deadline) {
		_ = ln.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		n, _, err := ln.ReadFromUDP(buf)
		if err != nil {
			continue
		}
		if string(buf[:n]) == string(frame) {
			got = true
			break
		}
	}
	cancel()
	<-done
	if !got {
		t.Fatal("did not receive beacon frame within timeout")
	}
}

func TestRunZ21BeaconViaRun(t *testing.T) {
	const port = 42106
	frame := z21server.SerialReply(z21server.VirtualSerial(3, 4))

	ln, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, RunConfig{
			Z21Beacon: &Z21BeaconConfig{
				Port:             port,
				LayoutID:         3,
				CommandStationID: 4,
			},
		}, nil, WithBeaconDest(&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port}))
	}()

	deadline := time.Now().Add(5 * time.Second)
	buf := make([]byte, 64)
	var got bool
	for time.Now().Before(deadline) {
		_ = ln.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		n, _, err := ln.ReadFromUDP(buf)
		if err != nil {
			continue
		}
		if string(buf[:n]) == string(frame) {
			got = true
			break
		}
	}
	cancel()
	<-done
	if !got {
		t.Fatal("Run did not emit z21 beacon")
	}
}

func TestRunRegistersServices(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	err := Run(ctx, RunConfig{
		Services: []ServiceConfig{
			{
				Protocol:         contract.RemoteProtocolWithrottle,
				LayoutID:         1,
				CommandStationID: 2,
				InstanceName:     "Bench #2",
				Port:             41235,
			},
		},
	}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
}
