package ws

import (
	"context"
	"encoding/json"
	"time"

	"github.com/coder/websocket"
)

const (
	writeWait  = 10 * time.Second
	pingPeriod = 30 * time.Second
)

// Client wraps one browser WebSocket connection.
type Client struct {
	conn    *websocket.Conn
	hub     *Hub
	session *DriveSession
	send    chan Envelope
}

// Session returns the DriveSession associated with this client. Used
// by control handlers to scope decisions to the connecting user.
func (c *Client) Session() *DriveSession { return c.session }

// Send enqueues an envelope for delivery. Non-blocking — if the
// outbound buffer is full, the envelope is dropped (matches the
// Broadcast path semantics in hub.go).
func (c *Client) Send(env Envelope) {
	select {
	case c.send <- env:
	default:
	}
}

// SendTyped is the typed convenience wrapper around Send.
func (c *Client) SendTyped(eventType string, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	c.Send(Envelope{Type: eventType, Payload: raw})
}

// SendAck enqueues an ack frame correlated with the request ID.
func (c *Client) SendAck(requestID string, ok bool, errCode string) {
	body, _ := json.Marshal(map[string]any{"ok": ok, "error": errCode})
	c.Send(Envelope{Type: "ack", ID: requestID, Payload: body})
}

// NewClient constructs a Client bound to hub and session.
func NewClient(conn *websocket.Conn, hub *Hub, session *DriveSession) *Client {
	return &Client{
		conn:    conn,
		hub:     hub,
		session: session,
		send:    make(chan Envelope, 64),
	}
}

// Serve starts the read and write loops. It blocks until the
// connection closes.
func (c *Client) Serve(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if ctl := c.hub.ControlHandler(); ctl != nil {
		ctl.HandleOpened(ctx, c)
	}

	go c.writeLoop(ctx, cancel)
	c.readLoop(ctx)

	if ctl := c.hub.ControlHandler(); ctl != nil {
		ctl.HandleClosed(context.Background(), c)
	}
}

func (c *Client) readLoop(ctx context.Context) {
	defer func() { c.hub.unregister <- c }()

	for {
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			return
		}
		var env Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			continue
		}
		if env.Type == "ping" {
			c.Send(Envelope{Type: "pong", ID: env.ID})
			continue
		}
		if ctl := c.hub.ControlHandler(); ctl != nil {
			ctl.HandleEnvelope(ctx, c, env)
		}
	}
}

func (c *Client) writeLoop(ctx context.Context, cancel context.CancelFunc) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return
		case env, ok := <-c.send:
			if !ok {
				return
			}
			writeCtx, writeCancel := context.WithTimeout(ctx, writeWait)
			payload, err := json.Marshal(env)
			writeCancel()
			if err != nil {
				return
			}
			writeCtx, writeCancel = context.WithTimeout(ctx, writeWait)
			err = c.conn.Write(writeCtx, websocket.MessageText, payload)
			writeCancel()
			if err != nil {
				return
			}
		case <-ticker.C:
			pingCtx, pingCancel := context.WithTimeout(ctx, writeWait)
			err := c.conn.Ping(pingCtx)
			pingCancel()
			if err != nil {
				return
			}
		}
	}
}
