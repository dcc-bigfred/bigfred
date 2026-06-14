package commandstation

import (
	"bufio"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// lnTCPBinaryTransport speaks RAW LocoNet bytes over a TCP socket: every
// message is exchanged as its on-the-wire bytes (opcode … checksum
// inclusive), exactly like the serial transport but over TCP. There is no
// ASCII SEND/RECEIVE framing.
//
// This is the protocol RocRail's lbtcp client speaks
// (rocdigs/impl/loconet/lbtcp.c): it reads single bytes, derives the
// message length from the opcode and reassembles the frame. lnTCPASCIITransport
// (loconet_tcp.go) is the other RocRail variant — the ASCII LbServer
// protocol (rocdigs/impl/loconet/lbserver.c). Some gateways / command
// stations (e.g. raw LocoNet-over-TCP bridges) speak this binary form
// rather than LbServer, in which case the ASCII transport connects but
// every request times out because no `RECEIVE` line ever arrives.
type lnTCPBinaryTransport struct {
	conn net.Conn
	stop chan struct{}
	// rxBytes counts every byte read off the socket since connect, so the
	// LNCV diagnostics can tell "dead bus" apart from "module did not
	// answer" (same role as in the serial transport).
	rxBytes atomic.Uint64
}

func newLnTCPBinaryTransport(host string, port uint16, rxCh chan<- lnPacket) (*lnTCPBinaryTransport, error) {
	if host == "" {
		return nil, fmt.Errorf("loconet tcp (binary): host is empty")
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("loconet tcp (binary): dial %s: %w", addr, err)
	}
	// Disable Nagle so a single short LocoNet message is not delayed
	// waiting to be batched (RocRail's lbtcp does SocketOp.setNodelay too).
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetNoDelay(true)
	}
	t := &lnTCPBinaryTransport{
		conn: conn,
		stop: make(chan struct{}),
	}
	logrus.WithField("addr", addr).Info("loconet command station: TCP (binary) connected")
	go t.readLoop(rxCh)
	return t, nil
}

// RxByteCount reports how many bytes have been read off the socket since
// connect.
func (t *lnTCPBinaryTransport) RxByteCount() uint64 {
	return t.rxBytes.Load()
}

func (t *lnTCPBinaryTransport) WritePacket(pkt []byte) error {
	_, err := t.conn.Write(pkt)
	return err
}

func (t *lnTCPBinaryTransport) Close() error {
	select {
	case <-t.stop:
	default:
		close(t.stop)
	}
	return t.conn.Close()
}

func (t *lnTCPBinaryTransport) readLoop(rxCh chan<- lnPacket) {
	// Blocking reads, reassembling frames with the shared stream parser
	// (the same one the serial transport uses). Close() unblocks the read
	// by closing the connection.
	var p lnStreamParser
	r := bufio.NewReader(t.conn)
	buf := make([]byte, 256)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			t.rxBytes.Add(uint64(n))
			for i := 0; i < n; i++ {
				pkt, ok := p.PushByte(buf[i])
				if !ok {
					continue
				}
				if lnChecksumOK(pkt) {
					rxCh <- lnPacket(pkt)
				} else {
					logrus.Debugf("loconet tcp (binary): dropping packet (bad checksum): % X", pkt)
				}
			}
		}
		if err != nil {
			select {
			case <-t.stop:
				// Intentional shutdown.
				return
			default:
			}
			logrus.Debugf("loconet tcp (binary): read loop terminated: %v", err)
			return
		}
	}
}
