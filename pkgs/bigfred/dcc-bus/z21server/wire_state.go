package z21server

import "sync"

// WireState holds Z21 LAN wire fields that are not shared across protocols.
type WireState struct {
	mu sync.Mutex
	m  map[string]*wireClient
}

type wireClient struct {
	BroadcastFlags  uint32
	pairCV3         *int
	pairCV4         *int
	pairFnBuf       []string
	pairFnPrevGroup map[byte]byte
	virtualCV       map[uint32]byte
}

// NewWireState returns an empty Z21 wire-state table.
func NewWireState() *WireState {
	return &WireState{m: make(map[string]*wireClient)}
}

func (w *WireState) client(key string) *wireClient {
	c, ok := w.m[key]
	if !ok {
		c = &wireClient{}
		w.m[key] = c
	}
	return c
}

// Remove drops wire state for one client key.
func (w *WireState) Remove(key string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.m, key)
}

// SetBroadcastFlags stores LAN_SET_BROADCASTFLAGS on a client.
func (w *WireState) SetBroadcastFlags(key string, flags uint32) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.client(key).BroadcastFlags = flags
}

// BroadcastFlags returns LAN_SET_BROADCASTFLAGS for key.
func (w *WireState) BroadcastFlags(key string) uint32 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.client(key).BroadcastFlags
}

// BufferPairingCV records one CV3/CV4 POM value while pairing.
func (w *WireState) BufferPairingCV(key string, cvWire int, value int) (cv3, cv4 int, ready bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.client(key).bufferPairingCV(cvWire, value)
}

// ClearPairingBuffer resets in-flight pairing entry buffers.
func (w *WireState) ClearPairingBuffer(key string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if c, ok := w.m[key]; ok {
		c.clearPairingBuffer()
	}
}

// BufferPairingFn records one function-key ON press while pairing.
func (w *WireState) BufferPairingFn(key string, fn int) (cv3, cv4 int, ready bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.client(key).bufferPairingFn(fn)
}

// PairingFnRisingEdges returns function numbers that turned on in a group update.
func (w *WireState) PairingFnRisingEdges(key string, group, fnByte byte) []int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.client(key).pairingFnRisingEdges(group, fnByte)
}

// SetVirtualCV stores a virtual CV value for one client.
func (w *WireState) SetVirtualCV(key string, loco uint16, cvWire int, value byte) {
	w.mu.Lock()
	defer w.mu.Unlock()
	c := w.client(key)
	if c.virtualCV == nil {
		c.virtualCV = make(map[uint32]byte)
	}
	c.virtualCV[virtualCVKey(loco, cvWire)] = value
}

// GetVirtualCV reads a virtual CV value for one client.
func (w *WireState) GetVirtualCV(key string, loco uint16, cvWire int) (byte, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	c, ok := w.m[key]
	if !ok || c.virtualCV == nil {
		return 0, false
	}
	v, ok := c.virtualCV[virtualCVKey(loco, cvWire)]
	return v, ok
}
