package commandstation

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"go.bug.st/serial"
)

// lnSerialWriteTimeout bounds a single WritePacket. go.bug.st/serial opens
// the port in BLOCKING mode with hardware flow control DISABLED and offers
// no write timeout, so port.Write() is a raw blocking syscall. LocoNet runs
// at 16.66 kbit/s; when the adapter (e.g. Uhlenbrock 63120) stops draining
// bytes off USB because its USB→LocoNet buffer filled, the kernel tty buffer
// fills and Write() blocks — potentially forever. Because SetSpeed/SendFn
// hold reqMu across the write, a single stuck write would otherwise freeze
// the whole interface until a process restart. The watchdog turns that hang
// into a bounded error and triggers a reconnect instead.
const lnSerialWriteTimeout = 2 * time.Second

// lnSerialReadTimeout is the per-Read poll interval. A timeout returns
// (0, nil) and is not an error; it lets the loop notice stop/reconnect.
const lnSerialReadTimeout = 200 * time.Millisecond

// lnSerialReconnectBackoff is the delay between failed reopen attempts.
const lnSerialReconnectBackoff = time.Second

var (
	errSerialNotConnected = errors.New("loconet serial: port not connected (reconnecting)")
	errSerialWriteTimeout = fmt.Errorf("loconet serial: write timed out after %s (adapter/bus backpressure)", lnSerialWriteTimeout)
)

type lnSerialTransport struct {
	device   string
	baudrate int
	rxCh     chan<- lnPacket

	// mu guards port, which is swapped out by the supervisor on reconnect.
	mu   sync.Mutex
	port serial.Port

	// writeMu serializes WritePacket so at most one blocking write (and its
	// watchdog goroutine) is ever in flight against the current fd.
	writeMu sync.Mutex

	// reconnectCh signals the supervisor to (re)open the port. Buffered so a
	// signal is never lost and never blocks the caller.
	reconnectCh chan struct{}
	stop        chan struct{}

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

	p, err := openLnSerialPort(device, baudrate)
	if err != nil {
		return nil, err
	}

	t := &lnSerialTransport{
		device:      device,
		baudrate:    baudrate,
		rxCh:        rxCh,
		port:        p,
		reconnectCh: make(chan struct{}, 1),
		stop:        make(chan struct{}),
	}
	logrus.WithFields(logrus.Fields{
		"device":   device,
		"baudrate": baudrate,
	}).Info("loconet command station: serial port open")
	go t.supervisor()
	go t.readLoop()
	return t, nil
}

func openLnSerialPort(device string, baudrate int) (serial.Port, error) {
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
	return p, nil
}

// RxByteCount reports how many bytes have been read off the wire since open.
func (t *lnSerialTransport) RxByteCount() uint64 {
	return t.rxBytes.Load()
}

// WritePacket writes one LocoNet frame with a watchdog. The blocking
// port.Write runs in its own goroutine; if it does not finish within
// lnSerialWriteTimeout the port is considered wedged: we trigger a reconnect
// (which closes the fd and unblocks the stuck write) and return an error so
// the caller releases reqMu instead of hanging the whole interface.
func (t *lnSerialTransport) WritePacket(pkt []byte) error {
	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	t.mu.Lock()
	port := t.port
	t.mu.Unlock()
	if port == nil {
		return errSerialNotConnected
	}

	done := make(chan error, 1)
	go func() {
		_, err := port.Write(pkt)
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.signalReconnect()
		}
		return err
	case <-time.After(lnSerialWriteTimeout):
		// Stuck write: ask the supervisor to reconnect, which closes this fd
		// and unblocks the goroutine above. Wait (bounded) for it to unwind
		// so two writes never race on a live fd.
		t.signalReconnect()
		select {
		case <-done:
		case <-time.After(lnSerialWriteTimeout):
		}
		return errSerialWriteTimeout
	}
}

func (t *lnSerialTransport) Close() error {
	select {
	case <-t.stop:
		// already closed
	default:
		close(t.stop)
	}
	t.mu.Lock()
	port := t.port
	t.port = nil
	t.mu.Unlock()
	if port != nil {
		return port.Close()
	}
	return nil
}

// signalReconnect asks the supervisor to reopen the port. Non-blocking and
// idempotent: a pending signal coalesces multiple failures into one reopen.
func (t *lnSerialTransport) signalReconnect() {
	select {
	case t.reconnectCh <- struct{}{}:
	default:
	}
}

// supervisor owns port (re)opening. It serializes reconnects so the readLoop
// and WritePacket only ever see a fully-open or nil port.
func (t *lnSerialTransport) supervisor() {
	for {
		select {
		case <-t.stop:
			return
		case <-t.reconnectCh:
			t.doReconnect()
		}
	}
}

func (t *lnSerialTransport) doReconnect() {
	t.mu.Lock()
	old := t.port
	t.port = nil
	t.mu.Unlock()
	if old != nil {
		// Purge any queued output then close, which unblocks a stuck Write.
		_ = old.ResetOutputBuffer()
		_ = old.Close()
	}

	for {
		select {
		case <-t.stop:
			return
		default:
		}
		p, err := openLnSerialPort(t.device, t.baudrate)
		if err != nil {
			logrus.WithError(err).WithField("device", t.device).
				Warn("loconet serial: reconnect failed, retrying")
			select {
			case <-t.stop:
				return
			case <-time.After(lnSerialReconnectBackoff):
			}
			continue
		}
		t.mu.Lock()
		t.port = p
		t.mu.Unlock()
		logrus.WithFields(logrus.Fields{
			"device":   t.device,
			"baudrate": t.baudrate,
		}).Info("loconet command station: serial port reconnected")
		return
	}
}

func (t *lnSerialTransport) readLoop() {
	var p lnStreamParser
	buf := make([]byte, 256)
	for {
		select {
		case <-t.stop:
			return
		default:
		}

		t.mu.Lock()
		port := t.port
		t.mu.Unlock()
		if port == nil {
			// Reconnect in progress; wait briefly and re-check.
			select {
			case <-t.stop:
				return
			case <-time.After(50 * time.Millisecond):
			}
			continue
		}

		// Avoid blocking forever on read during shutdown / reconnect.
		_ = port.SetReadTimeout(lnSerialReadTimeout)
		n, err := port.Read(buf)
		if err != nil {
			// A closed port (reconnect/shutdown) or a transient I/O error.
			// Reset the framing parser and back off briefly so we never
			// busy-spin on a persistent error.
			p = lnStreamParser{}
			select {
			case <-t.stop:
				return
			case <-time.After(20 * time.Millisecond):
			}
			continue
		}
		if n > 0 {
			t.rxBytes.Add(uint64(n))
		}
		for i := 0; i < n; i++ {
			pkt, ok := p.PushByte(buf[i])
			if !ok {
				continue
			}
			if !lnChecksumOK(pkt) {
				logrus.Debugf("loconet serial: dropping packet (bad checksum): % X", pkt)
				continue
			}
			select {
			case t.rxCh <- lnPacket(pkt):
			case <-t.stop:
				return
			}
		}
	}
}
