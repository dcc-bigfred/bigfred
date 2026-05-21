package commandstation

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type lnTransport interface {
	WritePacket(pkt []byte) error
	Close() error
}

type LocoNet struct {
	t lnTransport

	timeout time.Duration

	rxCh chan lnPacket

	// serialize request/response sequences
	reqMu sync.Mutex

	// state caches
	slotMu   sync.Mutex
	slotByAd map[LocoAddr]byte

	stateMu sync.Mutex
	dirfByA map[LocoAddr]byte
	sndByA  map[LocoAddr]byte
}

func NewLocoNetSerial(device string, baudrate int) (*LocoNet, error) {
	ln := &LocoNet{
		timeout:  4 * time.Second,
		rxCh:     make(chan lnPacket, 64),
		slotByAd: make(map[LocoAddr]byte),
		dirfByA:  make(map[LocoAddr]byte),
		sndByA:   make(map[LocoAddr]byte),
	}
	t, err := newLnSerialTransport(device, baudrate, ln.rxCh)
	if err != nil {
		return nil, err
	}
	ln.t = t
	return ln, nil
}

func NewLocoNetTCP(host string, port uint16) (*LocoNet, error) {
	ln := &LocoNet{
		timeout:  4 * time.Second,
		rxCh:     make(chan lnPacket, 64),
		slotByAd: make(map[LocoAddr]byte),
		dirfByA:  make(map[LocoAddr]byte),
		sndByA:   make(map[LocoAddr]byte),
	}
	t, err := newLnTCPTransport(host, port, ln.rxCh)
	if err != nil {
		return nil, err
	}
	ln.t = t
	return ln, nil
}

func (l *LocoNet) CleanUp() error {
	return l.t.Close()
}

func (l *LocoNet) WriteCV(mode Mode, lcv LocoCV, options ...ctxOptions) error {
	return errors.New("WriteCV not implemented for LocoNet")
}

func (l *LocoNet) ReadCV(mode Mode, lcv LocoCV, options ...ctxOptions) (int, error) {
	return 0, errors.New("ReadCV not implemented for LocoNet")
}

func (l *LocoNet) SendFn(mode Mode, addr LocoAddr, num FuncNum, toggle bool) error {
	if mode != MainTrackMode {
		return fmt.Errorf("SendFn: unsupported mode '%s' in LocoNet", mode)
	}
	fn := int(num)
	if fn < 0 || fn > 63 {
		return fmt.Errorf("SendFn: invalid function number %d", fn)
	}
	// This implementation supports F0..F8 via standard slot DIRF/SND messages.
	if fn > 8 {
		return fmt.Errorf("SendFn: function %d not supported over basic LocoNet slot commands (supports F0-F8)", fn)
	}

	l.reqMu.Lock()
	defer l.reqMu.Unlock()

	slot, err := l.ensureSlotLocked(addr)
	if err != nil {
		return err
	}

	// refresh current state so we don't clobber other bits
	if _, err := l.querySlotLocked(slot, addr); err != nil {
		// not fatal; we'll try with cached state
		logrus.Debugf("SendFn: slot query failed (continuing with cache): %v", err)
	}

	if fn <= 4 {
		dirf := l.getDirf(addr)
		dirf = setFnInDirf(dirf, fn, toggle)
		if err := l.sendLocked(lnBuildSetDirF(slot, dirf)); err != nil {
			return err
		}
		l.setDirf(addr, dirf)
		return nil
	}

	// fn 5..8
	snd := l.getSnd(addr)
	snd = setFnInSnd(snd, fn, toggle)
	if err := l.sendLocked(lnBuildSetSnd(slot, snd)); err != nil {
		return err
	}
	l.setSnd(addr, snd)
	return nil
}

func (l *LocoNet) ListFunctions(addr LocoAddr) ([]int, error) {
	l.reqMu.Lock()
	defer l.reqMu.Unlock()

	slot, err := l.ensureSlotLocked(addr)
	if err != nil {
		return nil, err
	}
	sd, err := l.querySlotLocked(slot, addr)
	if err != nil {
		return nil, err
	}

	var on []int
	for fn := 0; fn <= 8; fn++ {
		if fn <= 4 {
			if getFnFromDirf(sd.DirF, fn) {
				on = append(on, fn)
			}
		} else {
			if getFnFromSnd(sd.Snd, fn) {
				on = append(on, fn)
			}
		}
	}
	return on, nil
}

func (l *LocoNet) SetSpeed(addr LocoAddr, speed uint8, forward bool, speedSteps uint8) error {
	l.reqMu.Lock()
	defer l.reqMu.Unlock()

	slot, err := l.ensureSlotLocked(addr)
	if err != nil {
		return err
	}

	lnSpeed, err := scaleToLnSpeed(speed, speedSteps)
	if err != nil {
		return err
	}

	if err := l.sendLocked(lnBuildSetSpeed(slot, lnSpeed)); err != nil {
		return err
	}

	// Direction is carried in DIRF, so preserve functions and just set DIR bit.
	dirf := l.getDirf(addr)
	if forward {
		dirf |= 0x20
	} else {
		dirf &^= 0x20
	}
	if err := l.sendLocked(lnBuildSetDirF(slot, dirf)); err != nil {
		return err
	}
	l.setDirf(addr, dirf)
	return nil
}

func (l *LocoNet) GetSpeed(addr LocoAddr) (speed uint8, forward bool, err error) {
	l.reqMu.Lock()
	defer l.reqMu.Unlock()

	slot, err := l.ensureSlotLocked(addr)
	if err != nil {
		return 0, false, err
	}
	sd, err := l.querySlotLocked(slot, addr)
	if err != nil {
		return 0, false, err
	}
	forward = (sd.DirF & 0x20) != 0
	return uint8(sd.Speed), forward, nil
}

func (l *LocoNet) sendLocked(pkt []byte) error {
	if !lnChecksumOK(pkt) {
		return fmt.Errorf("refusing to send packet with invalid checksum: % X", pkt)
	}
	logrus.Debugf("loconet TX: % X", pkt)
	return l.t.WritePacket(pkt)
}

func (l *LocoNet) ensureSlotLocked(addr LocoAddr) (byte, error) {
	if slot, ok := l.getSlot(addr); ok {
		return slot, nil
	}

	// Request slot allocation/lookup.
	if err := l.sendLocked(lnBuildLocoAdr(addr)); err != nil {
		return 0, err
	}

	deadline := time.Now().Add(l.timeout)
	for time.Now().Before(deadline) {
		pkt, err := l.readPacketUntil(deadline)
		if err != nil {
			return 0, err
		}
		if sd, ok := parseLnSlotData(pkt); ok {
			// Cache state always.
			l.setSlot(sd.Addr, sd.Slot)
			l.setDirf(sd.Addr, sd.DirF)
			l.setSnd(sd.Addr, sd.Snd)
			if sd.Addr == addr {
				return sd.Slot, nil
			}
		}
	}
	return 0, fmt.Errorf("timeout waiting for slot data for loco %d", addr)
}

func (l *LocoNet) querySlotLocked(slot byte, addr LocoAddr) (lnSlotData, error) {
	if err := l.sendLocked(lnBuildRqSlotData(slot)); err != nil {
		return lnSlotData{}, err
	}
	deadline := time.Now().Add(l.timeout)
	for time.Now().Before(deadline) {
		pkt, err := l.readPacketUntil(deadline)
		if err != nil {
			return lnSlotData{}, err
		}
		if sd, ok := parseLnSlotData(pkt); ok {
			l.setSlot(sd.Addr, sd.Slot)
			l.setDirf(sd.Addr, sd.DirF)
			l.setSnd(sd.Addr, sd.Snd)
			if sd.Slot == slot && (addr == 0 || sd.Addr == addr) {
				return sd, nil
			}
		}
	}
	return lnSlotData{}, fmt.Errorf("timeout waiting for slot %d data", slot)
}

func (l *LocoNet) readPacketUntil(deadline time.Time) (lnPacket, error) {
	timeout := time.Until(deadline)
	if timeout <= 0 {
		return nil, errors.New("timeout")
	}
	select {
	case pkt := <-l.rxCh:
		logrus.Debugf("loconet RX: % X", []byte(pkt))
		return pkt, nil
	case <-time.After(timeout):
		return nil, errors.New("timeout")
	}
}

func (l *LocoNet) getSlot(addr LocoAddr) (byte, bool) {
	l.slotMu.Lock()
	defer l.slotMu.Unlock()
	slot, ok := l.slotByAd[addr]
	return slot, ok
}

func (l *LocoNet) setSlot(addr LocoAddr, slot byte) {
	if addr == 0 {
		return
	}
	l.slotMu.Lock()
	l.slotByAd[addr] = slot
	l.slotMu.Unlock()
}

func (l *LocoNet) getDirf(addr LocoAddr) byte {
	l.stateMu.Lock()
	defer l.stateMu.Unlock()
	return l.dirfByA[addr]
}

func (l *LocoNet) setDirf(addr LocoAddr, dirf byte) {
	l.stateMu.Lock()
	l.dirfByA[addr] = dirf
	l.stateMu.Unlock()
}

func (l *LocoNet) getSnd(addr LocoAddr) byte {
	l.stateMu.Lock()
	defer l.stateMu.Unlock()
	return l.sndByA[addr]
}

func (l *LocoNet) setSnd(addr LocoAddr, snd byte) {
	l.stateMu.Lock()
	l.sndByA[addr] = snd
	l.stateMu.Unlock()
}

// Function bit helpers.

func getFnFromDirf(dirf byte, fn int) bool {
	switch fn {
	case 0:
		return (dirf & 0x10) != 0
	case 1:
		return (dirf & 0x01) != 0
	case 2:
		return (dirf & 0x02) != 0
	case 3:
		return (dirf & 0x04) != 0
	case 4:
		return (dirf & 0x08) != 0
	default:
		return false
	}
}

func setFnInDirf(dirf byte, fn int, on bool) byte {
	var mask byte
	switch fn {
	case 0:
		mask = 0x10
	case 1:
		mask = 0x01
	case 2:
		mask = 0x02
	case 3:
		mask = 0x04
	case 4:
		mask = 0x08
	default:
		return dirf
	}
	if on {
		dirf |= mask
	} else {
		dirf &^= mask
	}
	return dirf
}

func getFnFromSnd(snd byte, fn int) bool {
	switch fn {
	case 5:
		return (snd & 0x01) != 0
	case 6:
		return (snd & 0x02) != 0
	case 7:
		return (snd & 0x04) != 0
	case 8:
		return (snd & 0x08) != 0
	default:
		return false
	}
}

func setFnInSnd(snd byte, fn int, on bool) byte {
	var mask byte
	switch fn {
	case 5:
		mask = 0x01
	case 6:
		mask = 0x02
	case 7:
		mask = 0x04
	case 8:
		mask = 0x08
	default:
		return snd
	}
	if on {
		snd |= mask
	} else {
		snd &^= mask
	}
	return snd
}

// --- TCP helper (shared parsing for LoconetOverTcp-style feeds) ---

func lnParseHexBytes(s string) ([]byte, error) {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return nil, fmt.Errorf("empty hex list")
	}
	out := make([]byte, 0, len(fields))
	for _, f := range fields {
		if len(f) == 1 {
			f = "0" + f
		}
		b, err := hexByte(f)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}

func hexByte(s string) (byte, error) {
	if len(s) != 2 {
		return 0, fmt.Errorf("invalid hex byte %q", s)
	}
	dec, err := hex.DecodeString(s)
	if err != nil || len(dec) != 1 {
		return 0, fmt.Errorf("invalid hex byte %q", s)
	}
	return dec[0], nil
}
