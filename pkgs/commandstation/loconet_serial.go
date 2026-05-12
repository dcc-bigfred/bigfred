package commandstation

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"go.bug.st/serial"
)

type lnSerialTransport struct {
	port serial.Port
	stop chan struct{}
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
	go t.readLoop(rxCh)
	return t, nil
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

