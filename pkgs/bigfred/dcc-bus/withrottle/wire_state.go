package withrottle

import (
	"errors"
	"net"
	"sync"
	"time"
)

const writeTimeout = 5 * time.Second

const maxWiThrottleFunction = 31

// throttleWire tracks one MultiThrottle instance on a WiThrottle connection.
type throttleWire struct {
	locos      map[uint16]string // addr → wire key (Snnn / Lnnn)
	forward    bool
	lastLoco   uint16
	speedSteps int
	lastSpeed  map[uint16]uint8 // last known DCC speed per acquired addr
}

// WireState holds WiThrottle wire fields that are not shared across protocols.
type WireState struct {
	mu sync.Mutex
	m  map[string]*wireClient
}

type wireClient struct {
	conn               net.Conn
	writeMu            sync.Mutex
	deviceID           string
	deviceName         string
	heartbeatMonitor   bool
	pairFnBuf          []string
	pairFnPrevFn       map[int]bool
	multiThrottle      map[byte]*throttleWire
	sentinelAcquired   bool
	sentinelThrottleID byte
	initialBurstSent   bool
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

// CloseAll closes every live TCP connection (daemon shutdown).
func (w *WireState) CloseAll() {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, c := range w.m {
		if c.conn != nil {
			_ = c.conn.Close()
		}
	}
}

// SetConn stores the live TCP connection for key, closing any prior connection.
func (w *WireState) SetConn(key string, conn net.Conn) {
	w.mu.Lock()
	c := w.client(key)
	c.writeMu.Lock()
	if c.conn != nil && c.conn != conn {
		_ = c.conn.Close()
	}
	c.conn = conn
	c.writeMu.Unlock()
	w.mu.Unlock()
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

// MarkInitialBurstSent records that the post-N initial burst was sent.
func (w *WireState) MarkInitialBurstSent(key string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.client(key).initialBurstSent = true
}

// InitialBurstSent reports whether the initial burst was already sent.
func (w *WireState) InitialBurstSent(key string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.client(key).initialBurstSent
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
func (w *WireState) SetSentinelAcquired(key string, acquired bool, throttleID byte) {
	w.mu.Lock()
	defer w.mu.Unlock()
	c := w.client(key)
	c.sentinelAcquired = acquired
	if acquired {
		c.sentinelThrottleID = throttleID
	} else {
		c.sentinelThrottleID = 0
	}
}

// SentinelThrottleID returns the throttle id that holds the pairing sentinel.
func (w *WireState) SentinelThrottleID(key string) byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.client(key).sentinelThrottleID
}

// SentinelAcquired reports whether the client holds the pairing sentinel.
func (w *WireState) SentinelAcquired(key string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.client(key).sentinelAcquired
}

// SetLastSpeed caches the last known DCC speed for one acquired loco.
func (w *WireState) SetLastSpeed(key string, id byte, addr uint16, speed uint8) {
	w.mu.Lock()
	defer w.mu.Unlock()
	tw := w.client(key).throttle(id)
	if tw.lastSpeed == nil {
		tw.lastSpeed = make(map[uint16]uint8, 4)
	}
	tw.lastSpeed[addr] = speed
}

// LastSpeed returns the cached DCC speed for addr on throttle id.
func (w *WireState) LastSpeed(key string, id byte, addr uint16) (uint8, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	c, ok := w.m[key]
	if !ok {
		return 0, false
	}
	tw, ok := c.multiThrottle[id]
	if !ok || tw.lastSpeed == nil {
		return 0, false
	}
	speed, ok := tw.lastSpeed[addr]
	return speed, ok
}

// ThrottleLocked returns throttle state; caller must hold WireState.mu.
func (c *wireClient) throttle(id byte) *throttleWire {
	t, ok := c.multiThrottle[id]
	if !ok {
		t = &throttleWire{
			locos:     make(map[uint16]string),
			forward:   true,
			lastSpeed: make(map[uint16]uint8, 4),
			speedSteps: 1,
		}
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
	if !ok || c.conn == nil {
		w.mu.Unlock()
		return errNoConn
	}
	c.writeMu.Lock()
	conn := c.conn
	w.mu.Unlock()
	defer c.writeMu.Unlock()
	_ = conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	payload := append([]byte(line), '\n')
	_, err := conn.Write(payload)
	return err
}
