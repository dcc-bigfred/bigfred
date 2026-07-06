package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/auth"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/slotlease"
)

const (
	slotsSnapshotType     = "slots.snapshot"
	slotsDiagMinInterval  = 500 * time.Millisecond
	slotsDiagTickerPeriod = time.Second
)

// SlotsDiagConfig wires the admin slot-diagnostic WebSocket.
type SlotsDiagConfig struct {
	Leaser         *slotlease.Leaser
	Metrics        slotlease.Recorder
	Log            *logrus.Logger
	AllowedOrigins []string
	Verifier       *auth.Verifier
}

// SlotsDiagHandler streams throttled slots.snapshot frames (D19).
// It is loopback-only; loco-server gates admin role before proxying.
type SlotsDiagHandler struct {
	leaser         *slotlease.Leaser
	metrics        slotlease.Recorder
	log            *logrus.Logger
	allowedOrigins []string
	verifier       *auth.Verifier
}

// NewSlotsDiagHandler returns a handler for /admin/slots/ws.
func NewSlotsDiagHandler(cfg SlotsDiagConfig) *SlotsDiagHandler {
	return &SlotsDiagHandler{
		leaser:         cfg.Leaser,
		metrics:        slotlease.RecorderOrNoop(cfg.Metrics),
		log:            cfg.Log,
		allowedOrigins: cfg.AllowedOrigins,
		verifier:       cfg.Verifier,
	}
}

// ServeHTTP upgrades and streams diagnostic snapshots until the client disconnects.
func (h *SlotsDiagHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.leaser == nil {
		http.NotFound(w, r)
		return
	}
	token := r.URL.Query().Get("token")
	if h.verifier != nil {
		if _, err := h.verifier.Verify(token); err != nil {
			if h.log != nil {
				h.log.WithError(err).Debug("dcc-bus slots diag reject upgrade")
			}
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	accept := &websocket.AcceptOptions{
		OriginPatterns:     h.allowedOrigins,
		InsecureSkipVerify: len(h.allowedOrigins) == 0,
	}
	conn, err := websocket.Accept(w, r, accept)
	if err != nil {
		if h.log != nil {
			h.log.WithError(err).Debug("dcc-bus slots diag upgrade failed")
		}
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	h.metrics.RecordDiagnosticSubscriberOpened()
	defer h.metrics.RecordDiagnosticSubscriberClosed()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Read loop: process inbound frames so control closes are noticed promptly
	// and respond to app-level pings with pong (useWsConnection pong-watchdog
	// closes the socket after pongTimeoutMs without one). Other frames are
	// discarded; the diag stream is server-push only.
	go func() {
		pongFrame, _ := json.Marshal(struct {
			Type string `json:"type"`
		}{Type: protocol.TypePong})
		for {
			if _, data, err := conn.Read(ctx); err != nil {
				cancel()
				return
			} else {
				var env struct{ Type string `json:"type"` }
				if json.Unmarshal(data, &env) == nil && env.Type == protocol.TypePing {
					_ = conn.Write(ctx, websocket.MessageText, pongFrame)
				}
			}
		}
	}()

	h.sendSnapshot(ctx, conn)

	var throttleMu sync.Mutex
	lastSent := time.Now()
	sendThrottled := func() {
		throttleMu.Lock()
		defer throttleMu.Unlock()
		if time.Since(lastSent) < slotsDiagMinInterval {
			return
		}
		if err := h.sendSnapshot(ctx, conn); err != nil {
			return
		}
		lastSent = time.Now()
	}

	ticker := time.NewTicker(slotsDiagTickerPeriod)
	defer ticker.Stop()

	events := h.leaser.DiagEvents()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := h.sendSnapshot(ctx, conn); err != nil {
				return
			}
			throttleMu.Lock()
			lastSent = time.Now()
			throttleMu.Unlock()
		case _, ok := <-events:
			if !ok {
				return
			}
			sendThrottled()
		}
	}
}

func (h *SlotsDiagHandler) sendSnapshot(ctx context.Context, conn *websocket.Conn) error {
	snap := h.leaser.DiagnosticSnapshot()
	frame := struct {
		Type    string                    `json:"type"`
		Payload slotlease.SlotsDiagnostic `json:"payload"`
	}{
		Type:    slotsSnapshotType,
		Payload: snap,
	}
	data, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, data)
}
