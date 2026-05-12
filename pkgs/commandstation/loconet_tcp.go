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
type lnTCPTransport struct {
	conn net.Conn
	stop chan struct{}
	w    *bufio.Writer
}

func newLnTCPTransport(host string, port uint16, rxCh chan<- lnPacket) (*lnTCPTransport, error) {
	if host == "" {
		return nil, fmt.Errorf("loconet tcp: host is empty")
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("loconet tcp: dial %s: %w", addr, err)
	}
	t := &lnTCPTransport{
		conn: conn,
		stop: make(chan struct{}),
		w:    bufio.NewWriter(conn),
	}
	go t.readLoop(rxCh)
	return t, nil
}

func (t *lnTCPTransport) WritePacket(pkt []byte) error {
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

func (t *lnTCPTransport) Close() error {
	select {
	case <-t.stop:
	default:
		close(t.stop)
	}
	return t.conn.Close()
}

func (t *lnTCPTransport) readLoop(rxCh chan<- lnPacket) {
	r := bufio.NewReader(t.conn)
	for {
		select {
		case <-t.stop:
			return
		default:
		}

		_ = t.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		line, err := r.ReadString('\n')
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			logrus.Debugf("loconet tcp read error: %v", err)
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Expected tokens: VERSION, RECEIVE, SENT, ERROR...
		if strings.HasPrefix(line, "RECEIVE ") {
			hexPart := strings.TrimSpace(strings.TrimPrefix(line, "RECEIVE "))
			pkt, err := lnParseHexBytes(hexPart)
			if err != nil {
				logrus.Debugf("loconet tcp: cannot parse RECEIVE %q: %v", line, err)
				continue
			}
			if lnChecksumOK(pkt) {
				rxCh <- lnPacket(pkt)
			} else {
				logrus.Debugf("loconet tcp: dropping packet (bad checksum): % X", pkt)
			}
			continue
		}
		if strings.HasPrefix(line, "VERSION") {
			logrus.Debugf("loconet tcp: %s", line)
			continue
		}
		if strings.HasPrefix(line, "SENT ") {
			logrus.Debugf("loconet tcp: %s", line)
			continue
		}
		// ignore other tokens
		logrus.Debugf("loconet tcp: %s", line)
	}
}

