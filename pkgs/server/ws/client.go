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

	go c.writeLoop(ctx, cancel)
	c.readLoop(ctx)
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
			continue
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
