package commandstation

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
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

	// rxCh receives every packet the transport reads off the bus.
	// A single dispatch goroutine owns it (see dispatch): it updates
	// the observation pipeline for ALL traffic — including packets
	// authored by external throttles — and forwards packets to syncCh
	// only while a request/response sequence is in flight.
	rxCh chan lnPacket
	// syncCh carries packets to the request/response waiters
	// (ensureSlotLocked / querySlotLocked). The dispatcher feeds it
	// only when syncActive is set so unsolicited bus traffic does not
	// pile up while nobody is waiting.
	syncCh     chan lnPacket
	syncActive atomic.Bool

	// obsCh streams observed state changes to StateObserver consumers.
	obsCh chan LocoObservation

	stop chan struct{}

	// serialize request/response sequences
	reqMu sync.Mutex

	// state caches
	slotMu   sync.Mutex
	slotByAd map[LocoAddr]byte
	slotAddr map[byte]LocoAddr // reverse map, needed to attribute bus traffic

	stateMu sync.Mutex
	dirfByA map[LocoAddr]byte
	sndByA  map[LocoAddr]byte
	// extFnByA caches functions F9..F28 per address. These are NOT held
	// in the command-station slot (they ride immediate DCC packets), so
	// the only authoritative copy is the one we keep here, updated both
	// when WE send and when we observe such a packet on the shared bus.
	extFnByA map[LocoAddr]uint32
}

func newLocoNetBase() *LocoNet {
	return &LocoNet{
		timeout:  4 * time.Second,
		rxCh:     make(chan lnPacket, 64),
		syncCh:   make(chan lnPacket, 64),
		obsCh:    make(chan LocoObservation, 64),
		stop:     make(chan struct{}),
		slotByAd: make(map[LocoAddr]byte),
		slotAddr: make(map[byte]LocoAddr),
		dirfByA:  make(map[LocoAddr]byte),
		sndByA:   make(map[LocoAddr]byte),
		extFnByA: make(map[LocoAddr]uint32),
	}
}

func NewLocoNetSerial(device string, baudrate int) (*LocoNet, error) {
	ln := newLocoNetBase()
	t, err := newLnSerialTransport(device, baudrate, ln.rxCh)
	if err != nil {
		return nil, err
	}
	ln.t = t
	go ln.dispatch()
	return ln, nil
}

func NewLocoNetTCP(host string, port uint16) (*LocoNet, error) {
	ln := newLocoNetBase()
	t, err := newLnTCPTransport(host, port, ln.rxCh)
	if err != nil {
		return nil, err
	}
	ln.t = t
	go ln.dispatch()
	return ln, nil
}

func (l *LocoNet) CleanUp() error {
	select {
	case <-l.stop:
	default:
		close(l.stop)
	}
	return l.t.Close()
}

// ObserveStates implements StateObserver: LocoNet is a shared bus, so
// every speed/direction/function packet — including those authored by
// an external throttle — is visible to the daemon and surfaces here.
func (l *LocoNet) ObserveStates() <-chan LocoObservation {
	return l.obsCh
}

// dispatch is the single owner of rxCh. It demultiplexes the shared bus
// into two consumers: the observation pipeline (always) and the
// request/response waiters (only while syncActive).
func (l *LocoNet) dispatch() {
	for {
		select {
		case <-l.stop:
			return
		case pkt, ok := <-l.rxCh:
			if !ok {
				return
			}
			logrus.Debugf("loconet RX: % X", []byte(pkt))
			l.observe(pkt)
			if l.syncActive.Load() {
				select {
				case l.syncCh <- pkt:
				default:
				}
			}
		}
	}
}

// observe parses a bus packet, refreshes the local caches and emits a
// LocoObservation for the change. Slot-keyed packets (SPD/DIRF/SND) are
// attributed via the reverse slot→addr map populated from slot reads.
func (l *LocoNet) observe(pkt []byte) {
	if len(pkt) < 2 {
		return
	}
	switch pkt[0] {
	case lnOPC_SL_RD_DATA:
		sd, ok := parseLnSlotData(pkt)
		if !ok {
			return
		}
		// System slots (≥120: programming 0x7C, fast clock 0x7B, …) do
		// not describe a locomotive; skip so a CV reply does not surface
		// as a bogus address-0 observation.
		if sd.Slot >= 120 {
			return
		}
		l.setSlot(sd.Addr, sd.Slot)
		l.setDirf(sd.Addr, sd.DirF)
		l.setSnd(sd.Addr, sd.Snd)
		fns := make(map[int]bool, 9)
		for fn := 0; fn <= 4; fn++ {
			fns[fn] = getFnFromDirf(sd.DirF, fn)
		}
		for fn := 5; fn <= 8; fn++ {
			fns[fn] = getFnFromSnd(sd.Snd, fn)
		}
		l.emit(LocoObservation{
			Addr:       sd.Addr,
			HasSpeed:   true,
			Speed:      sd.Speed,
			HasForward: true,
			Forward:    (sd.DirF & 0x20) != 0,
			Functions:  fns,
		})
	case lnOPC_LOCO_SPD:
		if len(pkt) < 4 {
			return
		}
		addr, ok := l.slotToAddr(pkt[1])
		if !ok {
			return
		}
		l.emit(LocoObservation{Addr: addr, HasSpeed: true, Speed: pkt[2]})
	case lnOPC_LOCO_DIRF:
		if len(pkt) < 4 {
			return
		}
		addr, ok := l.slotToAddr(pkt[1])
		if !ok {
			return
		}
		dirf := pkt[2]
		l.setDirf(addr, dirf)
		fns := make(map[int]bool, 5)
		for fn := 0; fn <= 4; fn++ {
			fns[fn] = getFnFromDirf(dirf, fn)
		}
		l.emit(LocoObservation{Addr: addr, HasForward: true, Forward: (dirf & 0x20) != 0, Functions: fns})
	case lnOPC_LOCO_SND:
		if len(pkt) < 4 {
			return
		}
		addr, ok := l.slotToAddr(pkt[1])
		if !ok {
			return
		}
		snd := pkt[2]
		l.setSnd(addr, snd)
		fns := make(map[int]bool, 4)
		for fn := 5; fn <= 8; fn++ {
			fns[fn] = getFnFromSnd(snd, fn)
		}
		l.emit(LocoObservation{Addr: addr, Functions: fns})
	case lnOPC_IMM_PACKET:
		// Extended functions (F9..F28) ride immediate DCC packets,
		// addressed by loco number rather than slot. Decode them so an
		// external throttle's F9+ changes are observed too.
		dcc := decodeImmDccPacket(pkt)
		if dcc == nil {
			return
		}
		addr, fns, ok := dccPacketFunctions(dcc)
		if !ok {
			return
		}
		l.mergeExtFn(addr, fns)
		l.emit(LocoObservation{Addr: addr, Functions: fns})
	}
}

func (l *LocoNet) emit(obs LocoObservation) {
	select {
	case l.obsCh <- obs:
	default:
		logrus.Debug("loconet: observation channel full, dropping update")
	}
}

// beginSync routes subsequent bus packets to the request/response
// waiter. Callers hold reqMu, so only one sequence runs at a time.
func (l *LocoNet) beginSync() {
	l.drainSync()
	l.syncActive.Store(true)
}

func (l *LocoNet) endSync() {
	l.syncActive.Store(false)
	l.drainSync()
}

func (l *LocoNet) drainSync() {
	for {
		select {
		case <-l.syncCh:
		default:
			return
		}
	}
}

// progTimeout is the default per-attempt deadline for a service-mode
// programming task; decoder service-mode reads can take a few seconds.
const progTimeout = 15 * time.Second

// ReadCV reads a single CV from a decoder on the programming (service)
// track via the programming slot (0x7C). POM reads over LocoNet need
// RailCom feedback and are not supported here.
func (l *LocoNet) ReadCV(mode Mode, lcv LocoCV, options ...ctxOptions) (int, error) {
	if mode != ProgrammingTrackMode {
		return 0, fmt.Errorf("ReadCV: LocoNet supports CV read only on the programming track (mode %q); POM read requires RailCom", mode)
	}
	ctx := RequestContext{timeout: progTimeout, retries: 0}
	applyMethodsToCtx(&ctx, options)
	cv0 := lcv.Cv.Translate() // 0-based CV address

	l.reqMu.Lock()
	defer l.reqMu.Unlock()
	l.beginSync()
	defer l.endSync()

	var lastErr error
	for i := 0; i <= int(ctx.retries); i++ {
		val, err := l.readCVLocked(cv0, ctx.timeout)
		if err == nil {
			return val, nil
		}
		logrus.Debugf("ReadCV: attempt %d/%d failed: %v", i+1, int(ctx.retries)+1, err)
		lastErr = err
	}
	return 0, lastErr
}

// WriteCV writes a single CV to a decoder on the programming (service)
// track via the programming slot (0x7C). Added alongside ReadCV so the
// LocoNet CV surface is symmetric; POM writes are not supported here.
func (l *LocoNet) WriteCV(mode Mode, lcv LocoCV, options ...ctxOptions) error {
	if mode != ProgrammingTrackMode {
		return fmt.Errorf("WriteCV: LocoNet supports CV write only on the programming track (mode %q)", mode)
	}
	ctx := RequestContext{timeout: progTimeout, retries: 0}
	applyMethodsToCtx(&ctx, options)
	cv0 := lcv.Cv.Translate()
	val := byte(lcv.Cv.Value)

	l.reqMu.Lock()
	defer l.reqMu.Unlock()
	l.beginSync()
	defer l.endSync()

	var lastErr error
	for i := 0; i <= int(ctx.retries); i++ {
		if err := l.writeCVLocked(cv0, val, ctx.timeout); err != nil {
			logrus.Debugf("WriteCV: attempt %d/%d failed: %v", i+1, int(ctx.retries)+1, err)
			lastErr = err
			continue
		}
		lastErr = nil
		break
	}
	if lastErr != nil {
		return lastErr
	}

	if ctx.verify {
		got, err := l.readCVLocked(cv0, ctx.timeout)
		if err != nil {
			return fmt.Errorf("WriteCV: cannot verify written value: %w", err)
		}
		if byte(got) != val {
			return fmt.Errorf("WriteCV: verify mismatch (wrote %d, read %d)", val, got)
		}
	}
	return nil
}

// readCVLocked runs one service-mode direct-byte read. Caller holds reqMu
// and has begun a sync window.
func (l *LocoNet) readCVLocked(cv0 uint16, timeout time.Duration) (int, error) {
	if err := l.sendLocked(lnBuildProgTask(lnPCMD_READ_DIRECT, cv0, 0)); err != nil {
		return 0, err
	}
	rep, err := l.awaitProgReplyLocked(time.Now().Add(timeout), false)
	if err != nil {
		return 0, err
	}
	if err := lnProgStatusError(rep.PStat); err != nil {
		return 0, err
	}
	return int(rep.Value), nil
}

// writeCVLocked runs one service-mode direct-byte write. Caller holds
// reqMu and has begun a sync window.
func (l *LocoNet) writeCVLocked(cv0 uint16, val byte, timeout time.Duration) error {
	if err := l.sendLocked(lnBuildProgTask(lnPCMD_WRITE_DIRECT, cv0, val)); err != nil {
		return err
	}
	// A write may be accepted "blind" (LACK 0x40) with no slot reply.
	rep, err := l.awaitProgReplyLocked(time.Now().Add(timeout), true)
	if err != nil {
		return err
	}
	return lnProgStatusError(rep.PStat)
}

// awaitProgReplyLocked consumes bus packets until the programming task
// resolves: an error LACK, a "blind" acceptance (success only when
// allowBlind), or the final OPC_SL_RD_DATA result from slot 0x7C.
func (l *LocoNet) awaitProgReplyLocked(deadline time.Time, allowBlind bool) (lnProgReply, error) {
	for time.Now().Before(deadline) {
		pkt, err := l.readPacketUntil(deadline)
		if err != nil {
			return lnProgReply{}, err
		}
		// Immediate long-acknowledge for the programmer task.
		if len(pkt) >= 4 && pkt[0] == lnOPC_LONG_ACK && pkt[1] == 0x7F {
			switch pkt[2] {
			case 0x01: // accepted; result follows in an E7 slot read
				continue
			case 0x40: // accepted blind; no slot reply will come
				if allowBlind {
					return lnProgReply{}, nil
				}
				return lnProgReply{}, errors.New("command station accepted programming task 'blind' (no result returned)")
			case 0x00:
				return lnProgReply{}, errors.New("programmer busy, task aborted")
			case 0x7F:
				return lnProgReply{}, errors.New("programming not implemented by command station")
			default:
				return lnProgReply{}, fmt.Errorf("unexpected programmer LACK code 0x%02X", pkt[2])
			}
		}
		if rep, ok := parseLnProgReply(pkt); ok {
			return rep, nil
		}
	}
	return lnProgReply{}, errors.New("timeout waiting for programming reply")
}

// lnProgStatusError maps a programming-slot PSTAT byte to an error.
func lnProgStatusError(pstat byte) error {
	switch {
	case pstat == 0:
		return nil
	case pstat&lnPSTAT_NO_DECODER != 0:
		return errors.New("no decoder detected on the programming track")
	case pstat&lnPSTAT_READ_FAIL != 0:
		return errors.New("decoder read failed (no acknowledge)")
	case pstat&lnPSTAT_WRITE_FAIL != 0:
		return errors.New("decoder write failed (no acknowledge)")
	case pstat&lnPSTAT_USER_ABORTED != 0:
		return errors.New("programming task aborted")
	default:
		return fmt.Errorf("programming failed (PSTAT=0x%02X)", pstat)
	}
}

func (l *LocoNet) SendFn(mode Mode, addr LocoAddr, num FuncNum, toggle bool) error {
	if mode != MainTrackMode {
		return fmt.Errorf("SendFn: unsupported mode '%s' in LocoNet", mode)
	}
	fn := int(num)
	if fn < 0 || fn > 28 {
		// F0..F8 ride the slot; F9..F28 ride immediate DCC packets.
		// F29+ would need the 0xD8.. groups / binary-state packets,
		// which are not implemented here.
		return fmt.Errorf("SendFn: unsupported function number %d (LocoNet driver supports F0-F28)", fn)
	}

	l.reqMu.Lock()
	defer l.reqMu.Unlock()
	l.beginSync()
	defer l.endSync()

	// F9..F28 are not stored in the slot; send them as an immediate DCC
	// function-group packet addressed by loco number (no slot needed).
	if fn >= 9 {
		return l.sendExtFnLocked(addr, fn, toggle)
	}

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
	l.beginSync()
	defer l.endSync()

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
	// F9..F28 are not in the slot; report them from the cache (the only
	// state we have, fed by our own sends and observed bus packets).
	extBits := l.getExtFn(addr)
	for fn := 9; fn <= 28; fn++ {
		if extBits&(1<<uint(fn)) != 0 {
			on = append(on, fn)
		}
	}
	return on, nil
}

func (l *LocoNet) SetSpeed(addr LocoAddr, speed uint8, forward bool, speedSteps uint8) error {
	l.reqMu.Lock()
	defer l.reqMu.Unlock()
	l.beginSync()
	defer l.endSync()

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
	l.beginSync()
	defer l.endSync()

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
	case pkt := <-l.syncCh:
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
	l.slotAddr[slot] = addr
	l.slotMu.Unlock()
}

// slotToAddr resolves a slot number back to a loco address using the
// reverse map populated whenever a slot read is seen on the bus.
func (l *LocoNet) slotToAddr(slot byte) (LocoAddr, bool) {
	l.slotMu.Lock()
	defer l.slotMu.Unlock()
	addr, ok := l.slotAddr[slot]
	return addr, ok
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

func (l *LocoNet) getExtFn(addr LocoAddr) uint32 {
	l.stateMu.Lock()
	defer l.stateMu.Unlock()
	return l.extFnByA[addr]
}

// setExtFn updates a single F9..F28 bit and returns the new full bitmask.
func (l *LocoNet) setExtFn(addr LocoAddr, fn int, on bool) uint32 {
	l.stateMu.Lock()
	defer l.stateMu.Unlock()
	bits := l.extFnByA[addr]
	if on {
		bits |= 1 << uint(fn)
	} else {
		bits &^= 1 << uint(fn)
	}
	l.extFnByA[addr] = bits
	return bits
}

// mergeExtFn folds an observed set of function bits into the cache.
func (l *LocoNet) mergeExtFn(addr LocoAddr, fns map[int]bool) {
	if addr == 0 || len(fns) == 0 {
		return
	}
	l.stateMu.Lock()
	defer l.stateMu.Unlock()
	bits := l.extFnByA[addr]
	for fn, on := range fns {
		if fn < 0 || fn > 31 {
			continue
		}
		if on {
			bits |= 1 << uint(fn)
		} else {
			bits &^= 1 << uint(fn)
		}
	}
	l.extFnByA[addr] = bits
}

// sendExtFnLocked sets one F9..F28 function via an immediate DCC packet.
// The whole group's bitmask is sent, taken from the per-loco cache, so
// other functions in the group are preserved. Caller holds reqMu.
func (l *LocoNet) sendExtFnLocked(addr LocoAddr, fn int, on bool) error {
	bits := l.setExtFn(addr, fn, on)
	dcc, ok := dccFnGroupPacket(addr, fn, bits)
	if !ok {
		return fmt.Errorf("SendFn: no DCC function group for F%d", fn)
	}
	imm, err := lnBuildImmPacket(dcc, lnImmRepeats)
	if err != nil {
		return err
	}
	return l.sendLocked(imm)
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
