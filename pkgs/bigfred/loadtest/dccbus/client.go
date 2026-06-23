// Package dccbus implements a minimal dcc-bus WebSocket client for load
// testing. It mirrors the browser's DccBusContext request/ack flow.
package dccbus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
)

const ackTimeout = 8 * time.Second

// Client drives one dcc-bus WebSocket session.
type Client struct {
	conn *websocket.Conn
	log  *logrus.Logger

	mu      sync.Mutex
	sendMu  sync.Mutex
	pending map[string]chan protocol.AckPayload
	opened  protocol.DccBusOpenedPayload
	openedC chan struct{}

	pingInterval time.Duration
	cancelRead   context.CancelFunc
	done         chan struct{}
}

// Connect upgrades the WebSocket, waits for dcc-bus.opened and starts the
// heartbeat loop advertised by the daemon.
func Connect(ctx context.Context, wsURL, token string, log *logrus.Logger) (*Client, error) {
	if log == nil {
		log = logrus.New()
	}

	u, err := withToken(wsURL, token)
	if err != nil {
		return nil, err
	}

	conn, _, err := websocket.Dial(ctx, u, &websocket.DialOptions{
		Subprotocols: []string{},
	})
	if err != nil {
		return nil, fmt.Errorf("websocket dial: %w", err)
	}

	c := &Client{
		conn:    conn,
		log:     log,
		pending: make(map[string]chan protocol.AckPayload),
		openedC: make(chan struct{}),
		done:    make(chan struct{}),
	}

	readCtx, cancelRead := context.WithCancel(context.Background())
	c.cancelRead = cancelRead
	go c.readLoop(readCtx)

	select {
	case <-ctx.Done():
		_ = c.Close()
		return nil, ctx.Err()
	case <-c.openedC:
	case <-time.After(15 * time.Second):
		_ = c.Close()
		return nil, fmt.Errorf("timed out waiting for dcc-bus.opened")
	}

	if c.opened.HeartbeatSecs > 0 {
		c.pingInterval = time.Duration(c.opened.HeartbeatSecs * float64(time.Second))
	} else {
		c.pingInterval = 2 * time.Second
	}
	go c.pingLoop(readCtx)

	log.WithFields(logrus.Fields{
		"layoutId":         c.opened.LayoutID,
		"commandStationId": c.opened.CommandStationID,
		"speedSteps":       c.opened.SpeedSteps,
		"heartbeatSecs":    c.opened.HeartbeatSecs,
		"sessionId":        c.opened.SessionID,
	}).Info("dcc-bus session opened")

	return c, nil
}

// Close tears down the WebSocket and fails any in-flight ack waiters.
func (c *Client) Close() error {
	if c.cancelRead != nil {
		c.cancelRead()
	}
	select {
	case <-c.done:
	default:
		close(c.done)
	}
	if c.conn != nil {
		err := c.conn.Close(websocket.StatusNormalClosure, "load test finished")
		c.conn = nil
		return err
	}
	return nil
}

// Subscribe requests loco.state updates for the given addresses.
func (c *Client) Subscribe(ctx context.Context, addresses []uint16) error {
	_, err := c.send(ctx, protocol.TypeLocoSubscribe, protocol.LocoSubscribePayload{Addresses: addresses})
	return err
}

// SetSpeed sends loco.setSpeed.
func (c *Client) SetSpeed(ctx context.Context, address uint16, speed uint8, forward bool) error {
	_, err := c.send(ctx, protocol.TypeLocoSetSpeed, contract.LocoSetSpeedWire{
		Address: address,
		Speed:   speed,
		Forward: forward,
	})
	return err
}

// SetFunction sends loco.setFunction.
func (c *Client) SetFunction(ctx context.Context, address uint16, fn uint8, on bool) error {
	_, err := c.send(ctx, protocol.TypeLocoSetFunction, contract.LocoSetFunctionWire{
		Address:  address,
		Function: fn,
		On:       on,
	})
	return err
}

func (c *Client) send(ctx context.Context, frameType string, payload any) (protocol.AckPayload, error) {
	id := newID()
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return protocol.AckPayload{}, err
	}
	env := contract.EnvelopeWire{Type: frameType, ID: id, Payload: rawPayload}
	data, err := json.Marshal(env)
	if err != nil {
		return protocol.AckPayload{}, err
	}

	ackCh := make(chan protocol.AckPayload, 1)
	c.mu.Lock()
	c.pending[id] = ackCh
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	// Hold sendMu only for the actual write, not the ack wait.
	// Holding it through the ack select (up to ackTimeout = 8s) would
	// serialize all goroutines and stall every vehicle except the one
	// currently waiting.
	c.sendMu.Lock()
	writeErr := c.writeLocked(ctx, data)
	c.sendMu.Unlock()
	if writeErr != nil {
		return protocol.AckPayload{}, writeErr
	}

	timer := time.NewTimer(ackTimeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return protocol.AckPayload{}, ctx.Err()
	case ack := <-ackCh:
		if !ack.OK {
			if ack.Error != "" {
				return ack, fmt.Errorf("%s", ack.Error)
			}
			return ack, fmt.Errorf("request rejected")
		}
		return ack, nil
	case <-timer.C:
		return protocol.AckPayload{}, fmt.Errorf("ack timeout for %s", frameType)
	}
}

func (c *Client) write(ctx context.Context, data []byte) error {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	if conn == nil {
		return fmt.Errorf("dcc-bus offline")
	}
	return conn.Write(ctx, websocket.MessageText, data)
}

func (c *Client) writeLocked(ctx context.Context, data []byte) error {
	return c.write(ctx, data)
}

func (c *Client) readLoop(ctx context.Context) {
	defer func() {
		if c.cancelRead != nil {
			c.cancelRead()
		}
	}()

	for {
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			c.failPending("dcc_bus_offline")
			return
		}
		c.handleMessage(data)
	}
}

func (c *Client) handleMessage(data []byte) {
	var env contract.EnvelopeWire
	if err := json.Unmarshal(data, &env); err != nil {
		return
	}

	switch env.Type {
	case protocol.TypeAck:
		if env.ID == "" {
			return
		}
		var ack protocol.AckPayload
		if len(env.Payload) > 0 {
			_ = json.Unmarshal(env.Payload, &ack)
		}
		c.mu.Lock()
		ch, ok := c.pending[env.ID]
		c.mu.Unlock()
		if ok {
			ch <- ack
		}
	case protocol.TypeDccBusOpened:
		var opened protocol.DccBusOpenedPayload
		if len(env.Payload) > 0 {
			_ = json.Unmarshal(env.Payload, &opened)
		}
		c.opened = opened
		select {
		case <-c.openedC:
		default:
			close(c.openedC)
		}
	case protocol.TypeLocoError:
		var errPayload protocol.LocoErrorPayload
		if len(env.Payload) > 0 {
			_ = json.Unmarshal(env.Payload, &errPayload)
		}
		c.log.WithFields(logrus.Fields{
			"code":    errPayload.Code,
			"address": errPayload.Address,
			"detail":  errPayload.Detail,
		}).Warn("loco.error from dcc-bus")
	case protocol.TypePong:
		// heartbeat response; no action required
	}
}

func (c *Client) pingLoop(ctx context.Context) {
	ticker := time.NewTicker(c.pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case <-ticker.C:
			payload, _ := json.Marshal(protocol.PingPayload{})
			env := contract.EnvelopeWire{Type: protocol.TypePing, Payload: payload}
			data, err := json.Marshal(env)
			if err != nil {
				continue
			}
			c.sendMu.Lock()
			err = c.writeLocked(ctx, data)
			c.sendMu.Unlock()
			if err != nil {
				c.log.WithError(err).Debug("ping failed")
				return
			}
		}
	}
}

func (c *Client) failPending(code string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, ch := range c.pending {
		select {
		case ch <- protocol.AckPayload{OK: false, Error: code}:
		default:
		}
		delete(c.pending, id)
	}
}

func withToken(wsURL, token string) (string, error) {
	wsURL = strings.TrimSpace(wsURL)
	if wsURL == "" {
		return "", fmt.Errorf("dcc-bus-ws is required")
	}
	u, err := url.Parse(wsURL)
	if err != nil {
		return "", fmt.Errorf("parse dcc-bus-ws: %w", err)
	}
	q := u.Query()
	if q.Get("token") == "" && token != "" {
		q.Set("token", token)
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
}

func newID() string {
	return uuid.NewString()
}
