package commandstation

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// TCP transport in LoconetOverTcp style:
// - client sends lines: "SEND <hex bytes...>\r\n"
// - server sends lines: "RECEIVE <hex bytes...>\r\n"
// See: https://loconetovertcp.sourceforge.net/Protocol/LoconetOverTcp.html
type lnTCPASCIITransport struct {
	conn net.Conn
	stop chan struct{}
	w    *bufio.Writer
}

func newLnTCPASCIITransport(host string, port uint16, rxCh chan<- lnPacket) (*lnTCPASCIITransport, error) {
	if host == "" {
		return nil, fmt.Errorf("loconet tcp: host is empty")
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("loconet tcp: dial %s: %w", addr, err)
	}
	t := &lnTCPASCIITransport{
		conn: conn,
		stop: make(chan struct{}),
		w:    bufio.NewWriter(conn),
	}
	logrus.WithField("addr", addr).Info("loconet command station: TCP connected")
	go t.readLoop(rxCh)
	return t, nil
}

func (t *lnTCPASCIITransport) WritePacket(pkt []byte) error {
	// Ensure checksum is correct; servers may reject invalid SEND.
	if !lnChecksumOK(pkt) {
		return fmt.Errorf("loconet tcp: invalid checksum, refusing SEND: % X", pkt)
	}
	var sb strings.Builder
	sb.WriteString("SEND")
	for _, b := range pkt {
		sb.WriteString(fmt.Sprintf(" %02X", b))
	}
	sb.WriteString("\r\n")
	if _, err := t.w.WriteString(sb.String()); err != nil {
		return err
	}
	return t.w.Flush()
}

func (t *lnTCPASCIITransport) Close() error {
	select {
	case <-t.stop:
	default:
		close(t.stop)
	}
	return t.conn.Close()
}

func (t *lnTCPASCIITransport) readLoop(rxCh chan<- lnPacket) {
	// Read full lines with a blocking reader, the same way RocRail's
	// lbserver client does (rocdigs/impl/loconet/lbserver.c). Earlier this
	// loop armed a 500 ms read deadline before every ReadString and, on
	// timeout, dropped the line it had read so far. bufio has already
	// consumed those bytes from the socket, so a RECEIVE reply that
	// straddled a deadline boundary (or arrived in two TCP segments) lost
	// its head and the tail was mis-parsed — the reply a request was
	// waiting for never arrived and the request timed out. A plain
	// blocking read never loses bytes; Close() unblocks it by closing the
	// connection.
	r := bufio.NewReader(t.conn)
	for {
		line, err := r.ReadString('\n')
		if line != "" {
			t.handleLine(line, rxCh)
		}
		if err != nil {
			select {
			case <-t.stop:
				// Intentional shutdown: Close() closed the connection.
				return
			default:
			}
			// A real read error (EOF / connection reset / closed) is
			// terminal for this connection. Exit instead of busy-looping;
			// the supervisor restarts the daemon to reconnect.
			logrus.Debugf("loconet tcp: read loop terminated: %v", err)
			return
		}
	}
}

// handleLine parses one LoconetOverTcp protocol line. It forwards RECEIVE
// payloads (a LocoNet message in space-separated hex) onto rxCh and logs
// every other token (VERSION, SENT, ERROR, TIMESTAMP, …).
func (t *lnTCPASCIITransport) handleLine(line string, rxCh chan<- lnPacket) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	// Locate the RECEIVE token anywhere in the line rather than only as a
	// strict prefix: the v2 protocol may emit a TIMESTAMP token ahead of
	// it on the same line, and RocRail's lbserver scans for it the same
	// lenient way (StrOp.find(msgStr, "RECEIVE")).
	const recv = "RECEIVE "
	idx := strings.Index(line, recv)
	if idx < 0 {
		logrus.Debugf("loconet tcp: %s", line)
		return
	}
	hexPart := strings.TrimSpace(line[idx+len(recv):])
	pkt, err := lnParseHexBytes(hexPart)
	if err != nil {
		logrus.Debugf("loconet tcp: cannot parse RECEIVE %q: %v", line, err)
		return
	}
	// A RECEIVE line carries exactly one LocoNet message. Trim it to the
	// length implied by the opcode (RocRail reads only <msglen> bytes too)
	// so any trailing token on a verbose server line does not get folded
	// into the checksum and wrongly reject an otherwise valid message.
	if n, ok := lnMsgLen(pkt[0], pkt); ok && n >= 2 && n <= len(pkt) {
		pkt = pkt[:n]
	}
	if lnChecksumOK(pkt) {
		rxCh <- lnPacket(pkt)
	} else {
		logrus.Debugf("loconet tcp: dropping packet (bad checksum): % X", pkt)
	}
}
