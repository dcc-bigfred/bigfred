package commandstation

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"go.bug.st/serial"
)

type lnSerialTransport struct {
	port serial.Port
	stop chan struct{}
	// rxBytes counts every byte read off the wire since open. A LocoBuffer-class
	// adapter (e.g. Uhlenbrock 63120) echoes traffic from a live LocoNet bus, so
	// a count of zero is a strong signal the bus is dead / unpowered / miswired,
	// not that the addressed module failed to answer.
	rxBytes atomic.Uint64
}

func newLnSerialTransport(device string, baudrate int, rxCh chan<- lnPacket) (*lnSerialTransport, error) {
	if device == "" {
		return nil, fmt.Errorf("loconet serial: device is empty")
	}
	if baudrate <= 0 {
		return nil, fmt.Errorf("loconet serial: invalid baudrate %d", baudrate)
	}

	mode := &serial.Mode{
		BaudRate: baudrate,
		// LocoBuffer-USB behaves like a UART 8N1.
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	p, err := serial.Open(device, mode)
	if err != nil {
		return nil, fmt.Errorf("loconet serial: open %q: %w", device, err)
	}

	t := &lnSerialTransport{
		port: p,
		stop: make(chan struct{}),
	}
	logrus.WithFields(logrus.Fields{
		"device":   device,
		"baudrate": baudrate,
	}).Info("loconet command station: serial port open")
	go t.readLoop(rxCh)
	return t, nil
}

// RxByteCount reports how many bytes have been read off the wire since open.
func (t *lnSerialTransport) RxByteCount() uint64 {
	return t.rxBytes.Load()
}

func (t *lnSerialTransport) WritePacket(pkt []byte) error {
	_, err := t.port.Write(pkt)
	return err
}

func (t *lnSerialTransport) Close() error {
	select {
	case <-t.stop:
		// already closed
	default:
		close(t.stop)
	}
	return t.port.Close()
}

func (t *lnSerialTransport) readLoop(rxCh chan<- lnPacket) {
	var p lnStreamParser
	buf := make([]byte, 256)
	for {
		select {
		case <-t.stop:
			return
		default:
		}

		// Avoid blocking forever on read during shutdown.
		_ = t.port.SetReadTimeout(200 * time.Millisecond)
		n, err := t.port.Read(buf)
		if err != nil {
			// Some implementations return timeout errors as nil n=0; we just continue.
			logrus.Debugf("loconet serial read error: %v", err)
			continue
		}
		if n > 0 {
			t.rxBytes.Add(uint64(n))
		}
		for i := 0; i < n; i++ {
			if pkt, ok := p.PushByte(buf[i]); ok {
				if lnChecksumOK(pkt) {
					rxCh <- lnPacket(pkt)
				} else {
					logrus.Debugf("loconet serial: dropping packet (bad checksum): % X", pkt)
				}
			}
		}
	}
}

