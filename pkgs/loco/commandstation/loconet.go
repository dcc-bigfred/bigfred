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

// lnRxCounter is implemented by transports that can report how many bytes they
// have read off the wire (serial). Used to tell "dead bus" apart from "module
// did not answer" in LNCV diagnostics.
type lnRxCounter interface {
	RxByteCount() uint64
}

// rxByteCount returns the number of bytes read off the wire, or 0 if the
// transport does not track it.
func (l *LocoNet) rxByteCount() uint64 {
	if c, ok := l.t.(lnRxCounter); ok {
		return c.RxByteCount()
	}
	return 0
}

type LocoNet struct {
	t lnTransport

	// timeout bounds request/response sequences that legitimately take a while
	// (LNCV programming, manual dispatch). Slot speed/function ops use the much
	// shorter slotTimeout instead.
	timeout time.Duration

	// slotTimeout bounds a single slot request/response (LOCO_ADR, RQ_SL_DATA,
	// NULL MOVE). LocoNet replies arrive well under 30 ms; a tight bound keeps a
	// lost reply from stalling slot acquisition — and thus the whole fleet — for
	// seconds.
	slotTimeout time.Duration

	// TX pacing: txMu serializes every transport write and enforces a minimum
	// inter-frame gap (minTxGap) matched to the 16.66 kbit/s bus, so bursts from
	// many locomotives queue at the driver instead of overflowing the
	// serial/socket buffer.
	txMu     sync.Mutex
	lastTxAt time.Time
	minTxGap time.Duration

	// keepaliveInterval is how often active slots are re-touched so the master
	// does not purge them to COMMON after ~200 s of inactivity (spec §4.3).
	keepaliveInterval time.Duration

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

	stateMu     sync.Mutex
	dirfByA     map[LocoAddr]byte
	sndByA      map[LocoAddr]byte
	spdByA      map[LocoAddr]byte   // last commanded/observed slot speed, for keepalive
	speedGenByA map[LocoAddr]uint64 // per-address SetSpeed generation, for TX coalescing
	// extFnByA caches functions F9..F28 per address. These are NOT held
	// in the command-station slot (they ride immediate DCC packets), so
	// the only authoritative copy is the one we keep here, updated both
	// when WE send and when we observe such a packet on the shared bus.
	extFnByA map[LocoAddr]uint32

	// metrics holds lock-free hot-path counters. Always non-nil; bumping it is
	// near-free and OTel-agnostic (see loconet_metrics.go).
	metrics *lnMetrics
}

// LocoNet TX/timeout tuning. These trade a touch of latency for the ability to
// drive many locomotives at once over the 16.66 kbit/s bus.
const (
	// lnDefaultMinTxGap paces transmissions to ~the bus drain rate. A 4-byte
	// LocoNet frame plus its mandatory CD backoff occupies ~3.6 ms of bus time,
	// so a ~5 ms floor keeps the driver from outrunning the wire and overflowing
	// the transport buffer when many locomotives move at once.
	lnDefaultMinTxGap = 5 * time.Millisecond

	// lnDefaultSlotTimeout bounds slot request/response sequences.
	lnDefaultSlotTimeout = 600 * time.Millisecond

	// lnSlotAcquireRetries retries a timed-out slot acquisition once before
	// giving up, since a single dropped reply is common on a busy bus.
	lnSlotAcquireRetries = 1

	// lnKeepaliveInterval re-touches active slots well within the ~200 s purge
	// window (spec §4.3 recommends ~100 s).
	lnKeepaliveInterval = 90 * time.Second
)

func newLocoNetBase() *LocoNet {
	return &LocoNet{
		timeout:           4 * time.Second,
		slotTimeout:       lnDefaultSlotTimeout,
		minTxGap:          lnDefaultMinTxGap,
		keepaliveInterval: lnKeepaliveInterval,
		rxCh:              make(chan lnPacket, 64),
		syncCh:            make(chan lnPacket, 64),
		obsCh:             make(chan LocoObservation, 64),
		stop:              make(chan struct{}),
		slotByAd:          make(map[LocoAddr]byte),
		slotAddr:          make(map[byte]LocoAddr),
		dirfByA:           make(map[LocoAddr]byte),
		sndByA:            make(map[LocoAddr]byte),
		spdByA:            make(map[LocoAddr]byte),
		speedGenByA:       make(map[LocoAddr]uint64),
		extFnByA:          make(map[LocoAddr]uint32),
		metrics:           newLnMetrics(),
	}
}

// MetricsSnapshot implements the MetricsSource interface: it returns the
// current cumulative counters plus instantaneous gauges (active slots, channel
// depths) and any reliability counters the transport tracks. OTel-free by
// design; the dcc-bus telemetry layer maps it onto instruments.
func (l *LocoNet) MetricsSnapshot() LnMetricsSnapshot {
	s := l.metrics.snapshot()

	// RX bytes come from the transport (it counts raw bytes off the wire).
	s.RxBytes = l.rxByteCount()

	// Fold in transport-level reliability counters when available.
	if st, ok := l.t.(lnStatsTransport); ok {
		ts := st.lnTransportStats()
		if ts.RxBytes > 0 {
			s.RxBytes = ts.RxBytes
		}
		s.BadChecksum = ts.BadChecksum
		s.Reconnects = ts.Reconnects
		s.WriteTimeouts = ts.WriteTimeouts
		// Prefer the transport's write-error tally when it tracks one.
		if ts.WriteErrors > s.TxErrors {
			s.TxErrors = ts.WriteErrors
		}
	}

	// Gauges.
	l.slotMu.Lock()
	s.SlotsActive = int64(len(l.slotByAd))
	l.slotMu.Unlock()
	s.RxQueueLen, s.RxQueueCap = int64(len(l.rxCh)), int64(cap(l.rxCh))
	s.ObsQueueLen, s.ObsQueueCap = int64(len(l.obsCh)), int64(cap(l.obsCh))
	s.SyncQueueLen, s.SyncQueueCap = int64(len(l.syncCh)), int64(cap(l.syncCh))
	return s
}

func NewLocoNetSerial(device string, baudrate int) (*LocoNet, error) {
	ln := newLocoNetBase()
	t, err := newLnSerialTransport(device, baudrate, ln.rxCh)
	if err != nil {
		return nil, err
	}
	ln.t = t
	go ln.dispatch()
	go ln.keepaliveLoop()
	return ln, nil
}

func NewLocoNetTCP(host string, port uint16) (*LocoNet, error) {
	ln := newLocoNetBase()
	t, err := newLnTCPASCIITransport(host, port, ln.rxCh)
	if err != nil {
		return nil, err
	}
	ln.t = t
	go ln.dispatch()
	go ln.keepaliveLoop()
	return ln, nil
}

// NewLocoNetTCPBinary connects to a gateway that speaks RAW LocoNet bytes
// over TCP (no ASCII SEND/RECEIVE framing) — the protocol of RocRail's
// lbtcp client. Use this when a LoconetOverTcp/LbServer (ASCII) connection
// dials successfully but every request times out because the peer streams
// binary LocoNet instead of `RECEIVE` lines.
func NewLocoNetTCPBinary(host string, port uint16) (*LocoNet, error) {
	ln := newLocoNetBase()
	t, err := newLnTCPBinaryTransport(host, port, ln.rxCh)
	if err != nil {
		return nil, err
	}
	ln.t = t
	go ln.dispatch()
	go ln.keepaliveLoop()
	return ln, nil
}

// SetTimeout adjusts the request/response deadline for LocoNet operations.
func (l *LocoNet) SetTimeout(d time.Duration) {
	if d > 0 {
		l.timeout = d
	}
}

func (l *LocoNet) CleanUp() error {
	// Release all cached slots before closing the transport so the command
	// station knows BigFred no longer owns any locomotives. A physical FRED
	// can then claim them immediately after BigFred disconnects.
	l.releaseAllSlots()
	select {
	case <-l.stop:
	default:
		close(l.stop)
	}
	return l.t.Close()
}

// releaseAllSlots sends OPC_SLOT_STAT1 COMMON for every currently cached
// slot, atomically clears the cache, and logs the result.
// Fire-and-forget: OPC_SLOT_STAT1 does not produce a reply.
func (l *LocoNet) releaseAllSlots() {
	type pair struct {
		addr LocoAddr
		slot byte
	}
	l.slotMu.Lock()
	if len(l.slotByAd) == 0 {
		l.slotMu.Unlock()
		return
	}
	pairs := make([]pair, 0, len(l.slotByAd))
	for addr, slot := range l.slotByAd {
		pairs = append(pairs, pair{addr, slot})
	}
	l.slotByAd = make(map[LocoAddr]byte)
	l.slotAddr = make(map[byte]LocoAddr)
	l.slotMu.Unlock()

	l.reqMu.Lock()
	defer l.reqMu.Unlock()
	for _, p := range pairs {
		if err := l.sendLocked(lnBuildSlotStat1(p.slot, lnSLOT_COMMON)); err != nil {
			logrus.WithError(err).Debugf("loconet: releaseAllSlots: slot %d addr %d", p.slot, p.addr)
			continue
		}
		logrus.Debugf("loconet: released slot %d (addr %d) on shutdown", p.slot, p.addr)
	}
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
			if len(pkt) > 0 {
				l.metrics.countRx(pkt[0])
			}
			l.observe(pkt)
			if l.syncActive.Load() {
				select {
				case l.syncCh <- pkt:
				default:
					l.metrics.incr(&l.metrics.syncDropped)
				}
			}
		}
	}
}

// observe parses a bus packet, refreshes the local caches and emits a
// LocoObservation for the change. Slot-keyed packets (SPD/DIRF/SND) are
// attributed via the reverse slot→addr map populated from slot reads.
// applySlotData refreshes BigFred's per-loco cache (slot mapping, direction,
// the F0..F8 function groups and speed) from an OPC_SL_RD_DATA frame seen on
// the bus. It is the single place that maps slot data onto the cache, shared by
// the passive observer and the request paths (slot acquire / query) so a
// locomotive subscription deterministically refreshes the cache straight from
// the bus reply instead of relying on observation timing. It intentionally does
// not emit a LocoObservation: observe() owns fan-out so upper layers are
// notified exactly once.
func (l *LocoNet) applySlotData(sd lnSlotData) {
	l.setSlot(sd.Addr, sd.Slot)
	l.setDirf(sd.Addr, sd.DirF)
	l.setSnd(sd.Addr, sd.Snd)
	l.setSpd(sd.Addr, sd.Speed)
}

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
		l.applySlotData(sd)
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
		l.setSpd(addr, pkt[2])
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
		l.metrics.incr(&l.metrics.obsDropped)
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

	// F9..F28 are not stored in the slot; send them as an immediate DCC
	// function-group packet addressed by loco number (no slot needed).
	if fn >= 9 {
		return l.sendExtFn(addr, fn, toggle)
	}

	slot, err := l.acquireSlot(addr)
	if err != nil {
		return err
	}

	// Trust the cached DIRF/SND state. The driver keeps it current from bus
	// observation (observe()), so the previous per-call OPC_RQ_SL_DATA round
	// trip was redundant — and, held under the global request lock, it
	// serialized every other locomotive behind a ~15 ms wait on each function
	// toggle. Dropping it is the single biggest fleet-latency win.
	if fn <= 4 {
		dirf := setFnInDirf(l.getDirf(addr), fn, toggle)
		if err := l.sendLocked(lnBuildSetDirF(slot, dirf)); err != nil {
			return err
		}
		l.setDirf(addr, dirf)
		return nil
	}

	snd := setFnInSnd(l.getSnd(addr), fn, toggle)
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

// SetSpeed sets speed and direction. On the hot path (slot already cached) it
// takes no request/response lock: it paces a single OPC_LOCO_SPD frame — plus
// OPC_LOCO_DIRF only when the direction bit actually changes — so 20+ locos can
// be driven without serializing behind each other. Acquiring a slot for a
// not-yet-seen address still goes through the request/response path once.
func (l *LocoNet) SetSpeed(addr LocoAddr, speed uint8, forward bool, speedSteps uint8) error {
	lnSpeed, err := scaleToLnSpeed(speed, speedSteps)
	if err != nil {
		return err
	}
	slot, err := l.acquireSlot(addr)
	if err != nil {
		return err
	}
	gen := l.nextSpeedGen(addr)
	return l.writeSpeed(addr, slot, lnSpeed, forward, gen)
}

// writeSpeed paces and writes the speed frame, coalescing superseded updates:
// if a newer SetSpeed for this address arrived while we waited for the bus, the
// stale frame is dropped instead of wasting a scarce transmission slot. The
// direction frame is sent only when the DIR bit changed (it rarely does during
// a throttle sweep), halving speed-related bus traffic.
func (l *LocoNet) writeSpeed(addr LocoAddr, slot, lnSpeed byte, forward bool, gen uint64) error {
	l.txMu.Lock()
	defer l.txMu.Unlock()

	l.pace()
	if l.currentSpeedGen(addr) != gen {
		l.metrics.incr(&l.metrics.txCoalesced)
		return nil // superseded by a newer SetSpeed; drop this stale frame
	}
	if err := l.writeRaw(lnBuildSetSpeed(slot, lnSpeed)); err != nil {
		return err
	}
	l.setSpd(addr, lnSpeed)

	dirf := l.getDirf(addr)
	want := dirf
	if forward {
		want |= 0x20
	} else {
		want &^= 0x20
	}
	if want != dirf {
		l.pace()
		if err := l.writeRaw(lnBuildSetDirF(slot, want)); err != nil {
			return err
		}
		l.setDirf(addr, want)
	}
	return nil
}

// acquireSlot returns the slot for addr, allocating it through the
// request/response path (under reqMu) the first time. Subsequent calls hit the
// cache and return without taking any lock, keeping the fast path lock-free so
// one loco's command never waits on another's.
func (l *LocoNet) acquireSlot(addr LocoAddr) (byte, error) {
	if slot, ok := l.getSlot(addr); ok {
		return slot, nil
	}
	l.reqMu.Lock()
	defer l.reqMu.Unlock()
	// Another goroutine may have acquired it while we waited for reqMu.
	if slot, ok := l.getSlot(addr); ok {
		return slot, nil
	}
	l.beginSync()
	defer l.endSync()

	var lastErr error
	for i := 0; i <= lnSlotAcquireRetries; i++ {
		if i > 0 {
			l.metrics.incr(&l.metrics.slotRetries)
		}
		slot, err := l.ensureSlotLocked(addr)
		if err == nil {
			l.metrics.incr(&l.metrics.slotAcquires)
			return slot, nil
		}
		lastErr = err
		logrus.Debugf("loconet: slot acquire attempt %d/%d for addr %d failed: %v",
			i+1, lnSlotAcquireRetries+1, addr, err)
	}
	l.metrics.incr(&l.metrics.slotAcquireFails)
	return 0, lastErr
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

// sendLocked transmits one frame with bus pacing. Despite the historical name
// it now self-synchronizes: it takes txMu and honours the inter-frame gap, so
// it is safe to call both from the request/response path (under reqMu) and from
// the lock-free fast path (SetSpeed/SendFn with a cached slot).
func (l *LocoNet) sendLocked(pkt []byte) error {
	l.txMu.Lock()
	defer l.txMu.Unlock()
	l.pace()
	return l.writeRaw(pkt)
}

// pace blocks until the minimum inter-frame gap has elapsed since the last
// transmission, throttling the driver to the bus drain rate. Caller holds txMu.
func (l *LocoNet) pace() {
	if l.minTxGap <= 0 {
		return
	}
	if wait := l.minTxGap - time.Since(l.lastTxAt); wait > 0 {
		l.metrics.addPaceWait(int64(wait))
		time.Sleep(wait)
	}
}

// writeRaw validates and writes one frame to the transport, advancing the
// pacing clock. Caller holds txMu.
func (l *LocoNet) writeRaw(pkt []byte) error {
	if !lnChecksumOK(pkt) {
		return fmt.Errorf("refusing to send packet with invalid checksum: % X", pkt)
	}
	logrus.Debugf("loconet TX: % X", pkt)
	err := l.t.WritePacket(pkt)
	l.lastTxAt = time.Now()
	if err != nil {
		l.metrics.incr(&l.metrics.txErrors)
	} else if len(pkt) > 0 {
		l.metrics.countTx(pkt[0], len(pkt))
	}
	return err
}

func (l *LocoNet) ensureSlotLocked(addr LocoAddr) (byte, error) {
	if slot, ok := l.getSlot(addr); ok {
		return slot, nil
	}
	return l.acquireSlotFreshLocked(addr)
}

// acquireSlotFreshLocked always queries the command station for addr's slot,
// refreshes the cache (re-mapping if the slot number changed) and asserts
// IN_USE via NULL MOVE. Unlike ensureSlotLocked it ignores the cache, so it
// reclaims a slot the command station purged to COMMON or reassigned to another
// loco while BigFred was idle. NULL MOVE runs only when the slot is not already
// IN_USE, so an active physical throttle (FRED) currently owning the slot is
// never stolen. Caller holds reqMu and has called beginSync.
func (l *LocoNet) acquireSlotFreshLocked(addr LocoAddr) (byte, error) {
	// Request slot allocation/lookup.
	if err := l.sendLocked(lnBuildLocoAdr(addr)); err != nil {
		return 0, err
	}

	deadline := time.Now().Add(l.slotTimeout)
	for time.Now().Before(deadline) {
		pkt, err := l.readPacketUntil(deadline)
		if err != nil {
			return 0, err
		}
		if sd, ok := parseLnSlotData(pkt); ok {
			// Refresh BigFred's cache from the bus reply (slot, direction,
			// F0..F8 and speed) so a subscription reflects the command
			// station's current view of the loco.
			l.applySlotData(sd)
			if sd.Addr == addr {
				// Promote slot to IN_USE via NULL MOVE so BigFred is the
				// authoritative throttle. Without this, the slot stays
				// COMMON and the command station may allow another throttle
				// to steal it. Failure is non-fatal: log and continue.
				if sd.Stat1&lnSLOT_STA_MASK != lnSLOT_IN_USE {
					if err := l.nullMoveLocked(sd.Slot); err != nil {
						logrus.WithError(err).Debugf("loconet: null move for slot %d addr %d skipped", sd.Slot, addr)
					}
				}
				return sd.Slot, nil
			}
		}
	}
	return 0, fmt.Errorf("timeout waiting for slot data for loco %d", addr)
}

// AcquireSlot makes BigFred the authoritative server-side owner of addr's slot.
// It queries the command station fresh (ignoring the cache) and asserts IN_USE,
// reclaiming a slot the master purged to COMMON or reassigned while the loco was
// idle — so a client leaving and returning to the throttle never silently loses
// control. Slots are owned per-locomotive by the server, independent of any
// session; the drive-permission layer is enforced separately by the caller.
//
// Idempotent and intended for the subscribe path, not the per-tick speed path:
// it performs a command-station round trip. An already-IN_USE slot (e.g. held by
// a physical FRED) is left untouched.
func (l *LocoNet) AcquireSlot(addr LocoAddr) error {
	if addr == 0 {
		return fmt.Errorf("loconet: AcquireSlot invalid addr 0")
	}
	l.reqMu.Lock()
	defer l.reqMu.Unlock()
	l.beginSync()
	defer l.endSync()

	var lastErr error
	for i := 0; i <= lnSlotAcquireRetries; i++ {
		if i > 0 {
			l.metrics.incr(&l.metrics.slotRetries)
		}
		if _, err := l.acquireSlotFreshLocked(addr); err == nil {
			l.metrics.incr(&l.metrics.slotAcquires)
			return nil
		} else {
			lastErr = err
			logrus.Debugf("loconet: AcquireSlot attempt %d/%d for addr %d failed: %v",
				i+1, lnSlotAcquireRetries+1, addr, err)
		}
	}
	l.metrics.incr(&l.metrics.slotAcquireFails)
	return lastErr
}

// nullMoveLocked sends OPC_MOVE_SLOTS with src==dst (NULL MOVE), which
// promotes the slot from COMMON or IDLE to IN_USE on the command station.
// Caller must hold reqMu and have called beginSync.
// Returns an error only when the command station explicitly rejects the move;
// a timeout is treated as success (some non-Digitrax masters are silent).
func (l *LocoNet) nullMoveLocked(slot byte) error {
	if err := l.sendLocked(lnBuildMoveSlots(slot, slot)); err != nil {
		return err
	}
	deadline := time.Now().Add(l.slotTimeout)
	for time.Now().Before(deadline) {
		pkt, err := l.readPacketUntil(deadline)
		if err != nil {
			// Timeout treated as silent acceptance (non-Digitrax masters).
			return nil
		}
		if sd, ok := parseLnSlotData(pkt); ok && sd.Slot == slot {
			return nil
		}
		// OPC_LONG_ACK for OPC_MOVE_SLOTS: B4 <BA&0x7F=0x3A> <code> <chk>
		if len(pkt) >= 4 && pkt[0] == lnOPC_LONG_ACK && pkt[1] == (lnOPC_MOVE_SLOTS&0x7F) {
			if pkt[2] == 0x00 {
				l.metrics.incr(&l.metrics.lackRejections)
				return fmt.Errorf("loconet: null move rejected by command station for slot %d", slot)
			}
			return nil // any non-zero code means accepted
		}
	}
	return nil // timeout → silent acceptance
}

func (l *LocoNet) querySlotLocked(slot byte, addr LocoAddr) (lnSlotData, error) {
	if err := l.sendLocked(lnBuildRqSlotData(slot)); err != nil {
		return lnSlotData{}, err
	}
	deadline := time.Now().Add(l.slotTimeout)
	for time.Now().Before(deadline) {
		pkt, err := l.readPacketUntil(deadline)
		if err != nil {
			return lnSlotData{}, err
		}
		if sd, ok := parseLnSlotData(pkt); ok {
			l.applySlotData(sd)
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
	// If the command station moved this loco to a different slot (purge +
	// reassignment), drop the stale reverse mapping so bus traffic on the old
	// slot is no longer misattributed to this address.
	if prev, ok := l.slotByAd[addr]; ok && prev != slot {
		delete(l.slotAddr, prev)
	}
	l.slotByAd[addr] = slot
	l.slotAddr[slot] = addr
	l.slotMu.Unlock()
}

func (l *LocoNet) clearSlot(addr LocoAddr, slot byte) {
	l.slotMu.Lock()
	delete(l.slotByAd, addr)
	delete(l.slotAddr, slot)
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

// ReleaseSlot writes OPC_SLOT_STAT1 with COMMON status, relinquishing
// BigFred's ownership of the slot without stopping the locomotive. The slot
// remains in the command station's table (loco address and speed preserved)
// but is now claimable by any throttle. The local cache entry is removed.
//
// This is a fire-and-forget operation: OPC_SLOT_STAT1 produces no reply.
func (l *LocoNet) ReleaseSlot(addr LocoAddr) error {
	slot, ok := l.getSlot(addr)
	if !ok {
		return nil // never acquired, nothing to do
	}
	l.reqMu.Lock()
	defer l.reqMu.Unlock()
	if err := l.sendLocked(lnBuildSlotStat1(slot, lnSLOT_COMMON)); err != nil {
		return err
	}
	l.clearSlot(addr, slot)
	l.metrics.incr(&l.metrics.slotReleases)
	logrus.Debugf("loconet: released slot %d for addr %d (set COMMON)", slot, addr)
	return nil
}

// DispatchSlot moves the slot for addr into the LocoNet dispatch slot
// (OPC_MOVE_SLOTS src=slot, dst=0). A physical throttle (e.g. a FRED) can
// then claim it by sending a dispatch GET (OPC_MOVE_SLOTS 0 0).
// The loco should be stopped before calling this to avoid runaway behaviour.
func (l *LocoNet) DispatchSlot(addr LocoAddr) error {
	slot, ok := l.getSlot(addr)
	if !ok {
		return fmt.Errorf("loconet: no tracked slot for addr %d", addr)
	}
	l.reqMu.Lock()
	defer l.reqMu.Unlock()
	l.beginSync()
	defer l.endSync()

	if err := l.sendLocked(lnBuildMoveSlots(slot, 0)); err != nil {
		return err
	}
	deadline := time.Now().Add(l.timeout)
	for time.Now().Before(deadline) {
		pkt, err := l.readPacketUntil(deadline)
		if err != nil {
			return fmt.Errorf("loconet: timeout waiting for dispatch PUT reply (slot %d, addr %d)", slot, addr)
		}
		if sd, ok := parseLnSlotData(pkt); ok && sd.Slot == slot {
			l.clearSlot(addr, slot)
			l.metrics.incr(&l.metrics.slotDispatches)
			logrus.Debugf("loconet: dispatched slot %d for addr %d", slot, addr)
			return nil
		}
		if len(pkt) >= 4 && pkt[0] == lnOPC_LONG_ACK && pkt[1] == (lnOPC_MOVE_SLOTS&0x7F) {
			if pkt[2] == 0x00 {
				l.metrics.incr(&l.metrics.lackRejections)
				return fmt.Errorf("loconet: dispatch PUT rejected for slot %d (addr %d)", slot, addr)
			}
			l.clearSlot(addr, slot)
			l.metrics.incr(&l.metrics.slotDispatches)
			logrus.Debugf("loconet: dispatched slot %d for addr %d (LACK)", slot, addr)
			return nil
		}
	}
	return fmt.Errorf("loconet: timeout waiting for dispatch PUT reply (slot %d, addr %d)", slot, addr)
}

// AcquireDispatched claims the slot currently held in the LocoNet dispatch
// slot (OPC_MOVE_SLOTS src=0, dst=0) and caches it locally.
// Returns (0, nil) when the dispatch slot is empty.
// After acquiring, a NULL MOVE is performed to confirm IN_USE ownership.
func (l *LocoNet) AcquireDispatched() (LocoAddr, error) {
	l.reqMu.Lock()
	defer l.reqMu.Unlock()
	l.beginSync()
	defer l.endSync()

	if err := l.sendLocked(lnBuildMoveSlots(0, 0)); err != nil {
		return 0, err
	}
	deadline := time.Now().Add(l.timeout)
	for time.Now().Before(deadline) {
		pkt, err := l.readPacketUntil(deadline)
		if err != nil {
			return 0, errors.New("loconet: timeout waiting for dispatch GET reply")
		}
		if sd, ok := parseLnSlotData(pkt); ok {
			l.setSlot(sd.Addr, sd.Slot)
			l.setDirf(sd.Addr, sd.DirF)
			l.setSnd(sd.Addr, sd.Snd)
			// Confirm ownership with NULL MOVE (non-fatal if rejected).
			if err := l.nullMoveLocked(sd.Slot); err != nil {
				logrus.WithError(err).Debugf("loconet: null move after dispatch GET failed for slot %d", sd.Slot)
			}
			logrus.Debugf("loconet: acquired dispatched slot %d for addr %d", sd.Slot, sd.Addr)
			return sd.Addr, nil
		}
		// LONG_ACK code 0x00 means the dispatch slot is empty.
		if len(pkt) >= 4 && pkt[0] == lnOPC_LONG_ACK && pkt[1] == (lnOPC_MOVE_SLOTS&0x7F) {
			if pkt[2] == 0x00 {
				return 0, nil // no dispatched slot
			}
		}
	}
	return 0, errors.New("loconet: timeout waiting for dispatch GET reply")
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

func (l *LocoNet) setSpd(addr LocoAddr, spd byte) {
	if addr == 0 {
		return
	}
	l.stateMu.Lock()
	l.spdByA[addr] = spd
	l.stateMu.Unlock()
}

func (l *LocoNet) getSpd(addr LocoAddr) (byte, bool) {
	l.stateMu.Lock()
	defer l.stateMu.Unlock()
	spd, ok := l.spdByA[addr]
	return spd, ok
}

// nextSpeedGen bumps and returns the per-address SetSpeed generation. Each
// SetSpeed reserves a generation before writing; writeSpeed drops its frame if a
// newer generation has since been reserved (TX coalescing).
func (l *LocoNet) nextSpeedGen(addr LocoAddr) uint64 {
	l.stateMu.Lock()
	defer l.stateMu.Unlock()
	l.speedGenByA[addr]++
	return l.speedGenByA[addr]
}

func (l *LocoNet) currentSpeedGen(addr LocoAddr) uint64 {
	l.stateMu.Lock()
	defer l.stateMu.Unlock()
	return l.speedGenByA[addr]
}

// keepaliveLoop periodically re-touches every cached slot so the master does
// not purge it to COMMON after ~200 s of inactivity (spec §4.3). Without this a
// parked locomotive silently loses BigFred's IN_USE ownership.
func (l *LocoNet) keepaliveLoop() {
	if l.keepaliveInterval <= 0 {
		return
	}
	ticker := time.NewTicker(l.keepaliveInterval)
	defer ticker.Stop()
	for {
		select {
		case <-l.stop:
			return
		case <-ticker.C:
			l.refreshSlots()
		}
	}
}

// refreshSlots re-sends each cached slot's last known speed, a harmless no-op
// move that resets the master's purge timer.
func (l *LocoNet) refreshSlots() {
	type pair struct {
		addr LocoAddr
		slot byte
	}
	l.slotMu.Lock()
	pairs := make([]pair, 0, len(l.slotByAd))
	for addr, slot := range l.slotByAd {
		pairs = append(pairs, pair{addr, slot})
	}
	l.slotMu.Unlock()

	for _, p := range pairs {
		spd, ok := l.getSpd(p.addr)
		if !ok {
			continue
		}
		if err := l.sendLocked(lnBuildSetSpeed(p.slot, spd)); err != nil {
			logrus.WithError(err).Debugf("loconet keepalive: slot %d addr %d", p.slot, p.addr)
			continue
		}
		l.metrics.incr(&l.metrics.keepaliveRefresh)
	}
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

// sendExtFn sets one F9..F28 function via an immediate DCC packet. The whole
// group's bitmask is sent, taken from the per-loco cache, so other functions in
// the group are preserved. No slot and no request/response lock are needed; the
// single frame is paced like any other write.
func (l *LocoNet) sendExtFn(addr LocoAddr, fn int, on bool) error {
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
