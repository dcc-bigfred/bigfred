package withrottle

import (
	"errors"
	"net"
	"sync"
)

// throttleWire tracks one MultiThrottle instance on a WiThrottle connection.
type throttleWire struct {
	locos      map[uint16]string // addr → wire key (Snnn / Lnnn)
	forward    bool
	lastLoco   uint16
	speedSteps int
}

// WireState holds WiThrottle wire fields that are not shared across protocols.
type WireState struct {
	mu sync.Mutex
	m  map[string]*wireClient
}

type wireClient struct {
	conn             net.Conn
	deviceID         string
	deviceName       string
	heartbeatMonitor bool
	pairFnBuf        []string
	pairFnPrevFn     map[int]bool
	multiThrottle    map[byte]*throttleWire
	sentinelAcquired bool
}

// NewWireState returns an empty WiThrottle wire-state table.
func NewWireState() *WireState {
	return &WireState{m: make(map[string]*wireClient)}
}

var errNoConn = errors.New("withrottle: no connection")

func (w *WireState) client(key string) *wireClient {
	c, ok := w.m[key]
	if !ok {
		c = &wireClient{multiThrottle: make(map[byte]*throttleWire)}
		w.m[key] = c
	}
	return c
}

// Remove drops wire state for one client key.
func (w *WireState) Remove(key string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if c, ok := w.m[key]; ok && c.conn != nil {
		_ = c.conn.Close()
	}
	delete(w.m, key)
}

// SetConn stores the live TCP connection for key.
func (w *WireState) SetConn(key string, conn net.Conn) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.client(key).conn = conn
}

// Conn returns the TCP connection for key.
func (w *WireState) Conn(key string) net.Conn {
	w.mu.Lock()
	defer w.mu.Unlock()
	c, ok := w.m[key]
	if !ok {
		return nil
	}
	return c.conn
}

// WithThrottle runs fn with the throttle instance locked.
func (w *WireState) WithThrottle(key string, id byte, fn func(*throttleWire)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	fn(w.client(key).throttle(id))
}

// SetDeviceID records the HU device id for key.
func (w *WireState) SetDeviceID(key, deviceID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.client(key).deviceID = deviceID
}

// DeviceID returns the HU device id for key.
func (w *WireState) DeviceID(key string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.client(key).deviceID
}

// SetDeviceName records the N-line device name for key.
func (w *WireState) SetDeviceName(key, name string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.client(key).deviceName = name
}

// DeviceName returns the N-line device name for key.
func (w *WireState) DeviceName(key string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.client(key).deviceName
}

// SetHeartbeatMonitor toggles the dead-man switch for key.
func (w *WireState) SetHeartbeatMonitor(key string, on bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.client(key).heartbeatMonitor = on
}

// HeartbeatMonitor reports whether the client enabled heartbeat monitoring.
func (w *WireState) HeartbeatMonitor(key string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.client(key).heartbeatMonitor
}

// BufferPairingFn records one function-key ON press while pairing.
func (w *WireState) BufferPairingFn(key string, fn int) (code string, ready bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.client(key).BufferPairingFn(fn)
}

// PairingFnRisingEdge reports whether fn turned on (rising edge).
func (w *WireState) PairingFnRisingEdge(key string, fn int, on bool) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.client(key).pairingFnRisingEdge(fn, on)
}

// ClearPairingBuffer resets in-flight pairing digit buffers.
func (w *WireState) ClearPairingBuffer(key string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if c, ok := w.m[key]; ok {
		c.clearPairingBuffer()
	}
}

// SetSentinelAcquired records whether the pairing sentinel is held.
func (w *WireState) SetSentinelAcquired(key string, acquired bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.client(key).sentinelAcquired = acquired
}

// SentinelAcquired reports whether the client holds the pairing sentinel.
func (w *WireState) SentinelAcquired(key string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.client(key).sentinelAcquired
}

// ThrottleLocked returns throttle state; caller must hold WireState.mu.
func (c *wireClient) throttle(id byte) *throttleWire {
	t, ok := c.multiThrottle[id]
	if !ok {
		t = &throttleWire{locos: make(map[uint16]string), speedSteps: 1}
		c.multiThrottle[id] = t
	}
	return t
}

// FindThrottleForAddr returns the throttle id and loco wire key for addr.
func (w *WireState) FindThrottleForAddr(key string, addr uint16) (id byte, locoKey string, ok bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	c, exists := w.m[key]
	if !exists {
		return 0, "", false
	}
	for tid, tw := range c.multiThrottle {
		if lk, found := tw.locos[addr]; found {
			return tid, lk, true
		}
	}
	return 0, "", false
}

// WriteLine writes one WiThrottle line terminated by LF to the client conn.
func (w *WireState) WriteLine(key, line string) error {
	w.mu.Lock()
	c, ok := w.m[key]
	var conn net.Conn
	if ok {
		conn = c.conn
	}
	w.mu.Unlock()
	if conn == nil {
		return errNoConn
	}
	_, err := conn.Write(append([]byte(line), '\n'))
	return err
}
