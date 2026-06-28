package discovery

import (
	"context"
	"fmt"
	"net"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

const z21BeaconInterval = 2 * time.Second

// RunBeacon periodically broadcasts a LAN_GET_SERIAL_NUMBER reply to 255.255.255.255:port.
// This is an unsolicited reply frame (not a client probe). Stock Z21 apps may still rely on
// active UDP probes to port 21105, which the inbound server answers on demand.
func RunBeacon(ctx context.Context, port int, frame []byte, log logrus.FieldLogger) error {
	dest := &net.UDPAddr{
		IP:   net.IPv4bcast,
		Port: port,
	}
	return runBeacon(ctx, dest, frame, log)
}

func runBeacon(ctx context.Context, dest *net.UDPAddr, frame []byte, log logrus.FieldLogger) error {
	if dest == nil || dest.Port <= 0 || dest.Port > 65535 {
		return fmt.Errorf("discovery: invalid beacon destination %v", dest)
	}
	if len(frame) == 0 {
		return fmt.Errorf("discovery: empty beacon frame")
	}
	if log == nil {
		log = logrus.New()
	}

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return fmt.Errorf("discovery: beacon listen: %w", err)
	}
	defer conn.Close()
	if dest.IP.Equal(net.IPv4bcast) {
		if err := setUDPBroadcast(conn); err != nil {
			return fmt.Errorf("discovery: enable broadcast: %w", err)
		}
	}

	log.WithFields(logrus.Fields{
		"dest":   dest.String(),
		"length": len(frame),
	}).Info("z21 discovery beacon started")

	ticker := time.NewTicker(z21BeaconInterval)
	defer ticker.Stop()

	send := func() error {
		if _, err := conn.WriteToUDP(frame, dest); err != nil {
			return err
		}
		return nil
	}
	if err := send(); err != nil {
		return fmt.Errorf("discovery: beacon send: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := send(); err != nil {
				log.WithError(err).Warn("z21 discovery beacon send failed")
			}
		}
	}
}

func setUDPBroadcast(conn *net.UDPConn) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	var opErr error
	err = raw.Control(func(fd uintptr) {
		opErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
	})
	if err != nil {
		return err
	}
	return opErr
}
