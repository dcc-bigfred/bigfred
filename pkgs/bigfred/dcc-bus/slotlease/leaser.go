package slotlease

import (
	"context"
	stderrors "errors"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

const (
	defaultMaxPerUser     = 8
	defaultMaxSlots       = 80
	defaultIdleTimeout    = 60 * time.Second
	defaultSwitcherGrace  = 60 * time.Second
	defaultGraceEvictMax  = 5
	slotReleaseStopGrace  = 500 * time.Millisecond
	slotReleaseRetryCount = 3
	slotReleaseRetryGap   = 100 * time.Millisecond
	releaseQueueSize      = 64
	// slotReconcileMaxPerCycle bounds how many leased addresses a single
	// ReconcileSlots pass probes on the LocoNet bus, so reconciliation never
	// floods the bus. Go's randomized map iteration naturally rotates which
	// addresses are sampled across cycles, so all leases are eventually covered.
	slotReconcileMaxPerCycle = 16
)

type leaseKind uint8

const (
	leaseSingle leaseKind = iota
	leaseTrain
)

// HolderKey identifies one driver session on a lease.
type HolderKey struct {
	UserID  uint
	Session string
	Source  string // "ws" | "z21" | "withrottle"
}

type holderKey = HolderKey

type lease struct {
	addr         uint16
	kind         leaseKind
	trainID      string
	holders      map[holderKey]struct{}
	holderOrder  []holderKey
	lastDriveAt  map[holderKey]time.Time
	acquiredAt   time.Time
	acquiring    bool
	releaseAfter         bool // set when last holder left during acquire
	pendingReleaseReason ReleaseReason
	releaseAt            time.Time // non-zero => deferred release scheduled (switcher grace)
}

type trainLease struct {
	trainID string
	addrs   []uint16
}

// releaseJob is one background e-stop-then-release scheduled off a
// latency-sensitive path (Reserve on the WS read loop).
type releaseJob struct {
	addr   uint16
	reason ReleaseReason
}

// DriveGate reports whether userID may drive addr. nil allows all.
type DriveGate func(userID uint, addr uint16) error

// SlotStation is the command-station slot surface the leaser uses.
type SlotStation interface {
	AcquireSlot(addr commandstation.LocoAddr) error
	ReleaseSlot(addr commandstation.LocoAddr) error
}

// LocoStore reads and updates authoritative loco snapshots for e-stop release.
type LocoStore interface {
	Snapshot(addr uint16) contract.LocoStateWire
	SetSpeedPreservingUser(addr uint16, speed uint8, forward bool, source string) contract.LocoStateWire
}

// SpeedWriter issues stop/e-stop commands before slot release.
type SpeedWriter interface {
	SetSpeed(addr uint16, speed uint8, forward bool, emergency bool) error
}

// StateBroadcaster fans loco.state after release e-stop.
type StateBroadcaster interface {
	BroadcastLocoState(ctx context.Context, snap contract.LocoStateWire)
}

// Config holds leaser limits. Zero values pick defaults.
type Config struct {
	MaxPerUser          int
	MaxSlots            int // 0 = unlimited (Z21)
	IdleTimeout         time.Duration
	IdleTimeoutDisabled bool          // explicit zero from admin disables sweep
	ReleaseGrace        time.Duration // stop grace before ReleaseSlot; 0 = default
	SwitcherGrace       time.Duration // grace window before deferred release; 0 = default
	Metrics             Recorder
}

// Leaser is the reference-counted owner of command-station slots for driven
// vehicles. Viewers do not create holders — only drivers do.
type Leaser struct {
	mu              sync.Mutex
	leases          map[uint16]*lease
	trains          map[string]*trainLease
	perUser         map[uint]int
	userAddrOrder   map[uint][]uint16 // FIFO per user for cap eviction
	releasePending  map[uint16]struct{}
	releasing       map[uint16]struct{} // addrs with a scheduled background release (reuse-race guard)
	station         SlotStation
	writer          SpeedWriter
	store           LocoStore
	hub             StateBroadcaster
	gate            DriveGate
	maxPerUser      int
	maxSlots        int
	idleTimeout     time.Duration
	releaseGrace    time.Duration
	switcherGrace   time.Duration
	metrics         Recorder
	diagCh          chan struct{}
	releaseCh       chan releaseJob
	stop            chan struct{}
	suppressExternal atomic.Bool
}

// New returns a Leaser with defaults from Config.
func New(station SlotStation, writer SpeedWriter, store LocoStore, hub StateBroadcaster, gate DriveGate, cfg Config) *Leaser {
	maxPerUser := cfg.MaxPerUser
	if maxPerUser <= 0 {
		maxPerUser = defaultMaxPerUser
	}
	maxSlots := cfg.MaxSlots
	if maxSlots < 0 {
		maxSlots = defaultMaxSlots
	}
	// maxSlots == 0 means unlimited (Z21 / tests).
	idle := cfg.IdleTimeout
	if cfg.IdleTimeoutDisabled {
		idle = 0
	} else if idle <= 0 {
		idle = defaultIdleTimeout
	}
	grace := cfg.ReleaseGrace
	if grace <= 0 {
		grace = slotReleaseStopGrace
	}
	sg := cfg.SwitcherGrace
	if sg <= 0 {
		sg = defaultSwitcherGrace
	}
	return &Leaser{
		leases:         make(map[uint16]*lease, 32),
		trains:         make(map[string]*trainLease, 8),
		perUser:        make(map[uint]int, 16),
		userAddrOrder:  make(map[uint][]uint16, 16),
		releasePending: make(map[uint16]struct{}, 8),
		releasing:      make(map[uint16]struct{}, 8),
		station:        station,
		writer:         writer,
		store:          store,
		hub:            hub,
		gate:           gate,
		maxPerUser:     maxPerUser,
		maxSlots:       maxSlots,
		idleTimeout:    idle,
		releaseGrace:   grace,
		switcherGrace:  sg,
		metrics:        recorderOrNoop(cfg.Metrics),
		diagCh:         make(chan struct{}, 8),
		releaseCh:      make(chan releaseJob, releaseQueueSize),
		stop:           make(chan struct{}),
	}
}

// Reserve records a driver holder for addr WITHOUT acquiring the command-
// station slot — the driver (LocoNet) acquires the slot itself when SetSpeed
// is issued, and OnSlotInUse confirms it. Reserve checks the CanDrive gate,
// the global BigFred slot budget (maxSlots, counting every slot physically
// IN_USE on the command station) and the per-user cap with oldest-
// first eviction. If a lease for addr exists and is in a deferred-release
// (switcher grace) window, the deferred release is cancelled and the slot
// is reused. Returns the addr evicted by the per-user cap, if any.
func (l *Leaser) Reserve(userID uint, session string, source string, addr uint16) (evicted uint16, err error) {
	l.metrics.RecordSelect(source)
	if l.gate != nil {
		if err := l.gate(userID, addr); err != nil {
			l.metrics.RecordNotAllowed()
			return 0, ErrNotAllowed
		}
	}
	key := holderKey{userID, session, source}

	l.mu.Lock()
	needsPhysicalSlot := l.leases[addr] == nil || !l.leases[addr].leaseOccupiesSlotLocked()
	if l.maxSlots > 0 && needsPhysicalSlot && !l.ensureBudgetHeadroomLocked(1) {
		l.mu.Unlock()
		l.metrics.RecordBudgetExceeded()
		return 0, ErrBigFredSlotBudgetExceeded
	}
	if l.perUser[userID] >= l.maxPerUser {
		evictAddr, ok := l.oldestEvictableAddrLocked(userID, addr)
		if !ok {
			l.mu.Unlock()
			return 0, ErrVehicleCapExceeded
		}
		l.mu.Unlock()
		l.deselect(userID, session, evictAddr, ReleaseCapEvict, false, true)
		l.metrics.RecordCapEvict()
		evicted = evictAddr
		l.mu.Lock()
	}
	le, ok := l.leases[addr]
	if !ok {
		le = &lease{
			addr:        addr,
			kind:        leaseSingle,
			holders:     make(map[holderKey]struct{}, 2),
			lastDriveAt: make(map[holderKey]time.Time, 2),
		}
		l.leases[addr] = le
	}
	// If a background release for this addr is still scheduled, cancel it and
	// reuse the physical slot that is still IN_USE on the command station.
	if _, scheduled := l.releasing[addr]; scheduled {
		delete(l.releasing, addr)
		if le.acquiredAt.IsZero() {
			le.acquiredAt = time.Now()
		}
	}
	// Reuse a slot that was scheduled for deferred release (switcher grace):
	// cancel the pending release and keep the slot IN_USE.
	le.releaseAt = time.Time{}
	if _, had := le.holders[key]; !had {
		le.holders[key] = struct{}{}
		le.holderOrder = append(le.holderOrder, key)
		l.perUser[userID]++
		l.appendUserAddrLocked(userID, addr)
	}
	le.lastDriveAt[key] = time.Now()
	if le.acquiredAt.IsZero() {
		// AcquiredAt is confirmed by OnSlotInUse once the driver reports the
		// slot IN_USE. Leave zero here; diag shows the lease as "pending slot".
	}
	l.notifyDiagLocked()
	l.mu.Unlock()
	return evicted, nil
}

// Select records a driver holder and acquires the command-station slot when
// the lease goes from zero to one holder. Returns the addr evicted by the
// per-user cap, if any. Prefer Reserve for paths where the driver acquires
// the slot itself (LocoNet SetSpeed); Select is for callers that need the
// slot synchronously (handset first-drive, tests).
func (l *Leaser) Select(userID uint, session string, source string, addr uint16) (evicted uint16, err error) {
	evicted, err = l.Reserve(userID, session, source, addr)
	if err != nil {
		return evicted, err
	}
	key := holderKey{userID, session, source}

	var needAcquire bool
	l.mu.Lock()
	le := l.leases[addr]
	if le == nil {
		l.mu.Unlock()
		return evicted, nil
	}
	if len(le.holders) == 1 && !le.acquiring && le.acquiredAt.IsZero() {
		needAcquire = true
		le.acquiring = true
	}
	l.mu.Unlock()
	if !needAcquire {
		return evicted, nil
	}

	var acquireErr error
	if l.station != nil {
		acquireErr = l.station.AcquireSlot(commandstation.LocoAddr(addr))
	}

	var releaseAfter bool
	var pendingReason ReleaseReason
	l.mu.Lock()
	le = l.leases[addr]
	if le == nil {
		l.mu.Unlock()
		return evicted, acquireErr
	}
	le.acquiring = false
	releaseAfter = le.releaseAfter
	pendingReason = le.pendingReleaseReason
	le.releaseAfter = false
	le.pendingReleaseReason = ""
	if acquireErr != nil {
		l.removeHolderLocked(le, key)
		if len(le.holders) == 0 {
			delete(l.leases, addr)
		}
		l.notifyDiagLocked()
		l.mu.Unlock()
		if stderrors.Is(acquireErr, ErrNoFreeSlot) {
			l.metrics.RecordNoFreeSlot()
		}
		return evicted, acquireErr
	}
	le.acquiredAt = time.Now()
	l.metrics.RecordLeased(source)
	l.notifyDiagLocked()
	l.mu.Unlock()

	if releaseAfter {
		reason := pendingReason
		if reason == "" {
			reason = ReleaseSwitcherChange
		}
		l.stopAndRelease(addr, reason)
	}
	return evicted, nil
}

// SelectTrain atomically leases every powered member of a train. On any
// failure it rolls back members already acquired in this call.
func (l *Leaser) SelectTrain(userID uint, session string, source string, trainID string, addrs []uint16) error {
	if len(addrs) == 0 {
		return nil
	}
	l.mu.Lock()
	newSlots := 0
	for _, addr := range addrs {
		le := l.leases[addr]
		if le == nil || !le.leaseOccupiesSlotLocked() {
			newSlots++
		}
	}
	if l.perUser[userID]+len(addrs) > l.maxPerUser {
		l.mu.Unlock()
		return ErrVehicleCapExceeded
	}
	if l.maxSlots > 0 && !l.ensureBudgetHeadroomLocked(newSlots) {
		l.mu.Unlock()
		l.metrics.RecordBudgetExceeded()
		return ErrBigFredSlotBudgetExceeded
	}
	l.mu.Unlock()

	acquired := make([]uint16, 0, len(addrs))
	for _, addr := range addrs {
		if l.gate != nil {
			if err := l.gate(userID, addr); err != nil {
				l.metrics.RecordNotAllowed()
				for _, a := range acquired {
					l.Deselect(userID, session, a)
				}
				return ErrNotAllowed
			}
		}
		_, err := l.Select(userID, session, source, addr)
		if err != nil {
			for _, a := range acquired {
				l.Deselect(userID, session, a)
			}
			return err
		}
		acquired = append(acquired, addr)
	}

	l.mu.Lock()
	l.trains[trainID] = &trainLease{trainID: trainID, addrs: append([]uint16(nil), addrs...)}
	for _, addr := range addrs {
		if le := l.leases[addr]; le != nil {
			le.kind = leaseTrain
			le.trainID = trainID
		}
	}
	l.mu.Unlock()
	return nil
}

// DrivenAddrs returns the locomotive addresses userID currently holds as a driver.
func (l *Leaser) DrivenAddrs(userID uint) []uint16 {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []uint16
	for addr, le := range l.leases {
		for k := range le.holders {
			if k.UserID == userID {
				out = append(out, addr)
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Deselect drops one holder; e-stop-then-release when the last holder leaves.
// For switcher-change with a grace window use DeselectDeferred instead.
func (l *Leaser) Deselect(userID uint, session string, addr uint16) {
	l.deselect(userID, session, addr, ReleaseSwitcherChange, false, false)
}

// Touch updates lastDriveAt for a remote holder (Z21/WiThrottle drive activity).
func (l *Leaser) Touch(userID uint, session string, addr uint16) {
	key := holderKey{userID, session, ""}
	l.mu.Lock()
	defer l.mu.Unlock()
	le, ok := l.leases[addr]
	if !ok {
		return
	}
	for k := range le.holders {
		if k.UserID == userID && k.Session == session {
			key = k
			break
		}
	}
	if _, ok := le.holders[key]; ok {
		le.lastDriveAt[key] = time.Now()
		l.notifyDiagLocked()
	}
}

// ReleaseSession drops every holder for session and releases empty leases.
func (l *Leaser) ReleaseSession(session string) {
	var toRelease []uint16
	l.mu.Lock()
	for addr, le := range l.leases {
		var remove []holderKey
		for k := range le.holders {
			if k.Session == session {
				remove = append(remove, k)
			}
		}
		for _, k := range remove {
			last := l.removeHolderLocked(le, k)
			if !last {
				continue
			}
			if le.acquiring {
				le.releaseAfter = true
				le.pendingReleaseReason = ReleaseSessionClose
			} else {
				delete(l.leases, addr)
				toRelease = append(toRelease, addr)
			}
		}
	}
	l.notifyDiagLocked()
	l.mu.Unlock()
	for _, addr := range toRelease {
		l.stopAndRelease(addr, ReleaseSessionClose)
	}
}

// SweepIdle e-stop-then-releases remote-only leases past idleTimeout.
func (l *Leaser) SweepIdle(now time.Time) {
	if l.idleTimeout <= 0 {
		return
	}
	var toRelease []uint16
	l.mu.Lock()
	for addr, le := range l.leases {
		if le.acquiring {
			continue
		}
		// External (bus-observed) slots have no drive activity to time and are
		// not BigFred's to release; the driver reports their release.
		if !le.leaseHasBigFredHolderLocked() {
			continue
		}
		if !l.leaseIsRemoteOnlyLocked(le) {
			continue
		}
		youngest := l.youngestDriveLocked(le)
		if now.Sub(youngest) < l.idleTimeout {
			continue
		}
		delete(l.leases, addr)
		toRelease = append(toRelease, addr)
	}
	l.notifyDiagLocked()
	l.mu.Unlock()
	for _, addr := range toRelease {
		l.stopAndRelease(addr, ReleaseIdleTimeout)
	}
	l.drainReleasePending()
}

// IdleTimeout returns the configured remote idle window (0 when disabled).
func (l *Leaser) IdleTimeout() time.Duration {
	if l == nil {
		return 0
	}
	return l.idleTimeout
}

// LeaseCount returns the number of active slot leases (for diagnostics/budget).
func (l *Leaser) LeaseCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.leases)
}

// budgetActiveLocked returns how many command-station slots BigFred currently
// occupies on the LocoNet master (leases with a confirmed IN_USE slot, plus
// grace-window leases whose slot is not released yet). Reserve-only bookings
// (acquiredAt still zero) do not count until the driver reports IN_USE.
func (l *Leaser) budgetActiveLocked() int {
	n := 0
	for _, le := range l.leases {
		if le.leaseOccupiesSlotLocked() {
			n++
		}
	}
	return n
}

func (le *lease) leaseOccupiesSlotLocked() bool {
	if !le.acquiredAt.IsZero() {
		return true
	}
	// Switcher grace: holders dropped but slot still IN_USE until SweepDeferred.
	return !le.releaseAt.IsZero()
}

// ensureBudgetHeadroomLocked tries to free budget for needed new physical slots
// (D20). Caller must hold l.mu. It may drop and re-acquire l.mu while
// enqueueing background releases on evicted grace leases.
func (l *Leaser) ensureBudgetHeadroomLocked(needed int) bool {
	if l.maxSlots <= 0 || needed <= 0 {
		return true
	}
	for attempts := 0; attempts < defaultGraceEvictMax && l.budgetActiveLocked()+needed > l.maxSlots; attempts++ {
		toRelease := l.takeDeferredReleasesLocked(1)
		if len(toRelease) == 0 {
			return false
		}
		for _, addr := range toRelease {
			l.releasing[addr] = struct{}{}
		}
		l.notifyDiagLocked()
		l.mu.Unlock()
		for _, addr := range toRelease {
			l.enqueueRelease(addr, ReleaseGraceEvict)
		}
		l.mu.Lock()
	}
	return l.budgetActiveLocked()+needed <= l.maxSlots
}

// takeDeferredReleasesLocked removes up to max switcher-grace leases (no
// holders, releaseAt set), newest deferred first. Caller must hold l.mu.
func (l *Leaser) takeDeferredReleasesLocked(max int) []uint16 {
	if max <= 0 {
		return nil
	}
	type candidate struct {
		addr uint16
		at   time.Time
	}
	var cands []candidate
	for addr, le := range l.leases {
		if le.releaseAt.IsZero() || len(le.holders) > 0 {
			continue
		}
		cands = append(cands, candidate{addr, le.releaseAt})
	}
	sort.Slice(cands, func(i, j int) bool {
		return cands[i].at.After(cands[j].at)
	})
	if len(cands) > max {
		cands = cands[:max]
	}
	out := make([]uint16, 0, len(cands))
	for _, c := range cands {
		delete(l.leases, c.addr)
		out = append(out, c.addr)
	}
	return out
}

func (le *lease) leaseHasBigFredHolderLocked() bool {
	for k := range le.holders {
		if k.UserID != 0 {
			return true
		}
	}
	return false
}

// SuppressExternal skips creating synthetic external leases while enabled.
// Used during daemon boot-stop so transient slot acquires do not pollute the
// lease table.
func (l *Leaser) SuppressExternal(on bool) {
	if l == nil {
		return
	}
	l.suppressExternal.Store(on)
}

// OnSlotInUse implements commandstation.SlotObserver. The driver calls it
// after a slot for addr becomes IN_USE. If BigFred already holds a lease
// (Reserve/Select created one), this confirms the lease's acquiredAt. If no
// lease exists, an external lease (UserID=0, source="external") is created so
// the diag table and metrics reflect the bus reality. External leases count
// toward the global slot budget (physical CS occupancy) but not per-user cap;
// idle/switch sweeps do not release them — only OnSlotReleased does.
func (l *Leaser) OnSlotInUse(addr commandstation.LocoAddr) {
	if l == nil || addr == 0 {
		return
	}
	l.mu.Lock()
	le, ok := l.leases[uint16(addr)]
	if ok {
		if le.acquiredAt.IsZero() {
			le.acquiredAt = time.Now()
			l.notifyDiagLocked()
		}
		l.mu.Unlock()
		return
	}
	if l.suppressExternal.Load() {
		l.mu.Unlock()
		return
	}
	le = &lease{
		addr:        uint16(addr),
		kind:        leaseSingle,
		holders:     map[holderKey]struct{}{{UserID: 0, Session: "", Source: "external"}: {}},
		holderOrder: []holderKey{{UserID: 0, Session: "", Source: "external"}},
		lastDriveAt: map[holderKey]time.Time{},
		acquiredAt:  time.Now(),
	}
	l.leases[uint16(addr)] = le
	l.metrics.RecordLeased("external")
	l.notifyDiagLocked()
	l.mu.Unlock()
}

// OnSlotReleased implements commandstation.SlotObserver. The driver calls it
// after a slot for addr is no longer IN_USE (released to COMMON, purged, or
// reassigned). The lease is dropped bookkeeping-only — no e-stop, because the
// driver already released the slot. Pending deferred releases are cancelled.
func (l *Leaser) OnSlotReleased(addr commandstation.LocoAddr) {
	if l == nil || addr == 0 {
		return
	}
	a16 := uint16(addr)
	l.mu.Lock()
	le, ok := l.leases[a16]
	if !ok {
		l.mu.Unlock()
		return
	}
	external := !le.leaseHasBigFredHolderLocked()
	l.dropLeaseBookkeepingLocked(a16)
	l.notifyDiagLocked()
	l.mu.Unlock()
	if external {
		l.metrics.RecordReleased("external")
	} else {
		l.metrics.RecordReleased(ReleaseSwitcherChange)
	}
}

// SlotProber actively reports whether addr's slot is still IN_USE on the
// command station. The LocoNet driver (commandstation.SlotReconciler) satisfies
// this directly.
type SlotProber interface {
	SlotStatus(addr commandstation.LocoAddr) (inUse bool, known bool, err error)
}

// ReconcileSlots probes the command station for leases that currently occupy a
// physical slot and drops (bookkeeping-only, no e-stop) those the station
// confirms are no longer IN_USE. This reclaims orphaned "external" leases
// (estop/control-path artifacts) and BigFred leases lost to missed bus events,
// regardless of how the slot was allocated. Leases the station still reports
// IN_USE are never touched (a live physical throttle keeps its slot).
// Reserve-only bookings (acquiredAt zero) and in-flight acquires are skipped.
//
// To bound bus traffic it probes at most slotReconcileMaxPerCycle leased
// addresses per call; Go's randomized map iteration rotates the sample so all
// leases are eventually covered across successive cycles.
func (l *Leaser) ReconcileSlots(prober SlotProber) {
	if l == nil || prober == nil {
		return
	}
	type cand struct {
		addr       uint16
		acquiredAt time.Time
	}
	l.mu.Lock()
	cands := make([]cand, 0, slotReconcileMaxPerCycle)
	for addr, le := range l.leases {
		if le.acquiring || !le.leaseOccupiesSlotLocked() {
			continue
		}
		cands = append(cands, cand{addr, le.acquiredAt})
		if len(cands) >= slotReconcileMaxPerCycle {
			break
		}
	}
	l.mu.Unlock()

	dropped := 0
	for _, c := range cands {
		inUse, known, err := prober.SlotStatus(commandstation.LocoAddr(c.addr))
		if err != nil || !known || inUse {
			continue
		}
		l.mu.Lock()
		le, ok := l.leases[c.addr]
		if !ok || !le.acquiredAt.Equal(c.acquiredAt) {
			l.mu.Unlock()
			continue
		}
		l.dropLeaseBookkeepingLocked(c.addr)
		l.notifyDiagLocked()
		l.mu.Unlock()
		dropped++
	}
	for i := 0; i < dropped; i++ {
		l.metrics.RecordReleased(ReleaseReconcile)
	}
}

// ForceRelease unconditionally releases the slot lease for addr regardless of
// holders — an admin action from the slots-diagnostics page to reclaim a stuck
// slot (including an orphaned "external" lease) without waiting for a
// reconciliation cycle. It e-stops the loco, releases the physical slot and
// drops all bookkeeping. Returns true when a lease existed.
func (l *Leaser) ForceRelease(addr uint16) bool {
	if l == nil || addr == 0 {
		return false
	}
	l.mu.Lock()
	_, ok := l.leases[addr]
	if !ok {
		l.mu.Unlock()
		return false
	}
	l.dropLeaseBookkeepingLocked(addr)
	l.notifyDiagLocked()
	l.mu.Unlock()
	l.stopAndRelease(addr, ReleaseAdminManual)
	return true
}

// dropLeaseBookkeepingLocked removes lease bookkeeping for addr. Caller must
// hold l.mu and the lease must exist.
func (l *Leaser) dropLeaseBookkeepingLocked(addr uint16) {
	le := l.leases[addr]
	if le == nil {
		return
	}
	for k := range le.holders {
		if k.UserID != 0 {
			l.perUser[k.UserID]--
			if l.perUser[k.UserID] <= 0 {
				delete(l.perUser, k.UserID)
			}
			l.removeUserAddrLocked(k.UserID, addr)
		}
	}
	delete(l.leases, addr)
	delete(l.releasePending, addr)
	delete(l.releasing, addr)
}

// DeselectDeferred drops one holder and, if it was the last BigFred holder,
// schedules a deferred release after SwitcherGrace instead of releasing
// immediately. This lets the user switch A→B→A without the driver re-acquiring
// A's slot on every switch (anti-jongling / slot reuse). A subsequent Reserve
// for addr cancels the pending release. SweepDeferred performs the actual
// release once the grace window elapses.
func (l *Leaser) DeselectDeferred(userID uint, session string, addr uint16) {
	l.deselect(userID, session, addr, ReleaseSwitcherChange, true, false)
}

func (l *Leaser) deselect(userID uint, session string, addr uint16, reason ReleaseReason, deferred, async bool) {
	l.mu.Lock()
	le, ok := l.leases[addr]
	if !ok {
		l.mu.Unlock()
		l.metrics.RecordDeselect("_")
		return
	}
	var found holderKey
	for k := range le.holders {
		if k.UserID == userID && k.Session == session {
			found = k
			break
		}
	}
	if found.UserID == 0 {
		l.mu.Unlock()
		l.metrics.RecordDeselect("_")
		return
	}
	l.metrics.RecordDeselect(found.Source)
	last := l.removeHolderLocked(le, found)
	if le.acquiring && last {
		le.releaseAfter = true
		le.pendingReleaseReason = reason
		l.notifyDiagLocked()
		l.mu.Unlock()
		return
	}
	if last {
		if deferred {
			le.releaseAt = time.Now().Add(l.switcherGrace)
			l.notifyDiagLocked()
			l.mu.Unlock()
			return
		}
		delete(l.leases, addr)
		if async {
			l.releasing[addr] = struct{}{}
		}
		l.notifyDiagLocked()
		l.mu.Unlock()
		if async {
			l.enqueueRelease(addr, reason)
		} else {
			l.stopAndRelease(addr, reason)
		}
		return
	}
	l.notifyDiagLocked()
	l.mu.Unlock()
}

// SweepDeferred releases leases whose deferred-release (switcher grace)
// window has elapsed. Called from the daemon's sweep loop alongside SweepIdle.
func (l *Leaser) SweepDeferred(now time.Time) {
	var toRelease []uint16
	l.mu.Lock()
	for addr, le := range l.leases {
		if le.releaseAt.IsZero() {
			continue
		}
		if !now.Before(le.releaseAt) {
			delete(l.leases, addr)
			toRelease = append(toRelease, addr)
		}
	}
	l.notifyDiagLocked()
	l.mu.Unlock()
	for _, addr := range toRelease {
		l.stopAndRelease(addr, ReleaseSwitcherChange)
	}
}

func (l *Leaser) appendUserAddrLocked(userID uint, addr uint16) {
	order := l.userAddrOrder[userID]
	for _, a := range order {
		if a == addr {
			return
		}
	}
	l.userAddrOrder[userID] = append(order, addr)
}

func (l *Leaser) removeUserAddrLocked(userID uint, addr uint16) {
	order := l.userAddrOrder[userID]
	for i, a := range order {
		if a == addr {
			l.userAddrOrder[userID] = append(order[:i], order[i+1:]...)
			return
		}
	}
}

func (l *Leaser) oldestEvictableAddrLocked(userID uint, skip uint16) (uint16, bool) {
	order := l.userAddrOrder[userID]
	for _, addr := range order {
		if addr == skip {
			continue
		}
		le, ok := l.leases[addr]
		if !ok {
			continue
		}
		if l.holderCountForUserLocked(le, userID) > 0 {
			return addr, true
		}
	}
	return 0, false
}

func (l *Leaser) holderCountForUserLocked(le *lease, userID uint) int {
	n := 0
	for k := range le.holders {
		if k.UserID == userID {
			n++
		}
	}
	return n
}

func (l *Leaser) removeHolderLocked(le *lease, key holderKey) (last bool) {
	if _, ok := le.holders[key]; !ok {
		return false
	}
	delete(le.holders, key)
	delete(le.lastDriveAt, key)
	for i, k := range le.holderOrder {
		if k == key {
			le.holderOrder = append(le.holderOrder[:i], le.holderOrder[i+1:]...)
			break
		}
	}
	l.perUser[key.UserID]--
	if l.perUser[key.UserID] <= 0 {
		delete(l.perUser, key.UserID)
	}
	l.removeUserAddrLocked(key.UserID, le.addr)
	return len(le.holders) == 0
}

func (l *Leaser) leaseIsRemoteOnlyLocked(le *lease) bool {
	for k := range le.holders {
		if k.Source == "ws" {
			return false
		}
	}
	return len(le.holders) > 0
}

func (l *Leaser) youngestDriveLocked(le *lease) time.Time {
	var youngest time.Time
	for k, at := range le.lastDriveAt {
		if _, ok := le.holders[k]; !ok {
			continue
		}
		if youngest.IsZero() || at.After(youngest) {
			youngest = at
		}
	}
	return youngest
}

func (l *Leaser) stopAndRelease(addr uint16, reason ReleaseReason) {
	if l.releaseAbortedByReuse(addr) {
		return
	}
	l.metrics.RecordReleaseEstop()
	ctx := context.Background()
	forward := true
	if l.store != nil {
		forward = l.store.Snapshot(addr).Forward
	}
	if l.writer != nil {
		err := l.writer.SetSpeed(addr, 0, forward, true)
		if err != nil && !stderrors.Is(err, commandstation.ErrSpeedSuperseded) {
			select {
			case <-time.After(l.releaseGrace):
			case <-l.stop:
			}
		}
	}
	if l.store != nil && l.hub != nil {
		snap := l.store.SetSpeedPreservingUser(addr, 0, forward, "slot_release")
		l.hub.BroadcastLocoState(ctx, snap)
	}
	if l.releaseAbortedByReuse(addr) {
		return
	}
	if err := l.releaseSlotWithRetry(addr); err != nil {
		l.mu.Lock()
		l.releasePending[addr] = struct{}{}
		l.notifyDiagLocked()
		l.mu.Unlock()
		l.metrics.RecordReleasePending()
		l.notifyDiag()
		return
	}
	l.metrics.RecordReleased(reason)
	l.notifyDiag()
}

func (l *Leaser) releaseSlotWithRetry(addr uint16) error {
	if l.releaseAbortedByReuse(addr) {
		return nil
	}
	if l.station == nil {
		return nil
	}
	var last error
	for i := 0; i < slotReleaseRetryCount; i++ {
		last = l.station.ReleaseSlot(commandstation.LocoAddr(addr))
		if last == nil {
			return nil
		}
		if i < slotReleaseRetryCount-1 {
			time.Sleep(slotReleaseRetryGap)
		}
	}
	return last
}

// releaseAbortedByReuse reports whether a background release for addr was
// cancelled because Reserve re-leased the address before the physical
// ReleaseSlot completed. Clears a stale releasing mark when reuse wins.
func (l *Leaser) releaseAbortedByReuse(addr uint16) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.leases[addr] == nil {
		return false
	}
	delete(l.releasing, addr)
	return true
}

func (l *Leaser) drainReleasePending() {
	l.mu.Lock()
	pending := make([]uint16, 0, len(l.releasePending))
	for addr := range l.releasePending {
		pending = append(pending, addr)
	}
	l.mu.Unlock()
	for _, addr := range pending {
		if err := l.releaseSlotWithRetry(addr); err == nil {
			l.mu.Lock()
			delete(l.releasePending, addr)
			l.mu.Unlock()
			l.metrics.RecordReleased(ReleasePendingRetry)
			l.notifyDiag()
		}
	}
}

// enqueueRelease schedules an e-stop-then-release on the background release
// worker so a latency-sensitive caller (Reserve on the WS read loop) never
// blocks on a bus round-trip. The lease bookkeeping is already removed under
// mu and the addr marked in l.releasing; only the physical e-stop +
// ReleaseSlot is deferred. If the worker queue is full it releases inline so a
// slot is never silently leaked. MUST be called without holding l.mu.
func (l *Leaser) enqueueRelease(addr uint16, reason ReleaseReason) {
	select {
	case l.releaseCh <- releaseJob{addr: addr, reason: reason}:
	default:
		l.releaseScheduled(addr, reason)
	}
}

// releaseScheduled runs stopAndRelease unless the addr was re-leased before
// the worker ran (guard against yanking a freshly re-acquired slot). Clears
// the l.releasing mark either way.
func (l *Leaser) releaseScheduled(addr uint16, reason ReleaseReason) {
	l.mu.Lock()
	_, scheduled := l.releasing[addr]
	reused := l.leases[addr] != nil
	delete(l.releasing, addr)
	l.mu.Unlock()
	if !scheduled || reused {
		return
	}
	l.stopAndRelease(addr, reason)
}

// RunReleaseWorker drains scheduled background releases until ctx is
// cancelled. Wire it once per leaser from the daemon alongside RunIdleSweep.
func (l *Leaser) RunReleaseWorker(ctx context.Context) {
	if l == nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-l.releaseCh:
			l.releaseScheduled(job.addr, job.reason)
		}
	}
}
