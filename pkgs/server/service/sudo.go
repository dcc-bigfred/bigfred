// SudoService implements the layout-scoped "sudo" elevation flow
// (§7a.7). It owns:
//
//   - the in-memory rate-limiter that throttles PIN attempts,
//   - the upsert / delete loop against repo.SudoElevations (admin
//     elevation) and repo.LayoutSignalmen (permanent signalman
//     self-grant — the engineer's-cap icon next to the padlock),
//   - the broadcast of `auth.elevationChanged` to every WS session
//     of the affected user,
//   - the periodic janitor that reaps expired sudo rows.
//
// The service is intentionally HTTP-agnostic — handlers receive
// already-typed inputs and unpack errors via errors.Is.

package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/keskad/loco/pkgs/server/domain"
	"github.com/keskad/loco/pkgs/server/repo"
	"github.com/keskad/loco/pkgs/server/ws"
)

// Sudo-related sentinel errors. The UI maps each onto an `errors:`
// i18n key (see web/src/i18n/locales/<l>/errors.json).
var (
	// ErrSudoInvalidInput is returned for empty/zero (userID,
	// layoutID) inputs — a defensive guard the HTTP layer normally
	// catches earlier.
	ErrSudoInvalidInput = errors.New("sudo_invalid_input")

	// ErrSudoInvalidPIN is the canonical "wrong PIN" surface.
	// Distinct from ErrLayoutAdminPINMismatch so the rate-limiter
	// owns the brute-force counter.
	ErrSudoInvalidPIN = errors.New("sudo_invalid_pin")

	// ErrSudoLocked is returned when the rate-limiter has rejected
	// further attempts for this (user, layout) pair. The HTTP
	// layer turns it into 429 with a Retry-After hint.
	ErrSudoLocked = errors.New("sudo_locked")
)

// SudoConfig groups the few knobs the service exposes. Defaults
// match §7a.7 of the spec.
type SudoConfig struct {
	// TTL is the lifetime of a single elevation. The spec mandates
	// 2 minutes; the field is a knob so tests can use a tighter
	// window (e.g. 50 ms) without sleeping.
	TTL time.Duration
	// FailWindow is the rolling window the rate-limiter inspects
	// when counting recent PIN failures.
	FailWindow time.Duration
	// MaxFailures is the number of failures inside FailWindow that
	// trips the per-(user,layout) lockout.
	MaxFailures int
	// LockDuration is how long a tripped lockout lasts.
	LockDuration time.Duration
	// JanitorInterval is how often the reap loop wakes up.
	JanitorInterval time.Duration
}

// DefaultSudoConfig matches the spec defaults (§7a.7). Used by the
// CLI bootstrap; tests construct their own config.
var DefaultSudoConfig = SudoConfig{
	TTL:             2 * time.Minute,
	FailWindow:      60 * time.Second,
	MaxFailures:     5,
	LockDuration:    60 * time.Second,
	JanitorInterval: 10 * time.Second,
}

// SudoService implements the elevation surface. The struct is safe
// for concurrent use — every shared field is guarded by `mu`.
type SudoService struct {
	elevations *repo.SudoElevations
	signalmen  *repo.LayoutSignalmen
	layouts    *LayoutService
	hub        *ws.Hub
	cfg        SudoConfig

	mu       sync.Mutex
	failures map[failureKey][]time.Time // (userID, layoutID) → recent failure stamps
	locks    map[failureKey]time.Time   // (userID, layoutID) → unlock-not-before
}

type failureKey struct {
	UserID   uint
	LayoutID uint
}

// NewSudoService constructs a SudoService. `cfg` may be the zero
// value — every field falls back to DefaultSudoConfig when unset.
func NewSudoService(
	elevations *repo.SudoElevations,
	signalmen *repo.LayoutSignalmen,
	layouts *LayoutService,
	hub *ws.Hub,
	cfg SudoConfig,
) *SudoService {
	if elevations == nil {
		panic("service.NewSudoService: SudoElevations must not be nil")
	}
	if signalmen == nil {
		panic("service.NewSudoService: LayoutSignalmen must not be nil")
	}
	if layouts == nil {
		panic("service.NewSudoService: LayoutService must not be nil")
	}
	cfg = mergeSudoConfig(cfg)
	return &SudoService{
		elevations: elevations,
		signalmen:  signalmen,
		layouts:    layouts,
		hub:        hub,
		cfg:        cfg,
		failures:   make(map[failureKey][]time.Time),
		locks:      make(map[failureKey]time.Time),
	}
}

func mergeSudoConfig(cfg SudoConfig) SudoConfig {
	if cfg.TTL == 0 {
		cfg.TTL = DefaultSudoConfig.TTL
	}
	if cfg.FailWindow == 0 {
		cfg.FailWindow = DefaultSudoConfig.FailWindow
	}
	if cfg.MaxFailures == 0 {
		cfg.MaxFailures = DefaultSudoConfig.MaxFailures
	}
	if cfg.LockDuration == 0 {
		cfg.LockDuration = DefaultSudoConfig.LockDuration
	}
	if cfg.JanitorInterval == 0 {
		cfg.JanitorInterval = DefaultSudoConfig.JanitorInterval
	}
	return cfg
}

// TTL exposes the configured elevation lifetime so the HTTP layer
// can echo `expiresAt` to the caller.
func (s *SudoService) TTL() time.Duration { return s.cfg.TTL }

// Sudo verifies the layout admin PIN and persists/refreshes the
// `(UserID, LayoutID)` SudoElevation row, granting the caller the
// `admin` role for `cfg.TTL` (2 min by default). A second call
// while the row is still active simply pushes ExpiresAt forward,
// mirroring the spec "click again → re-arm the timer".
//
// Failed attempts feed the in-memory rate-limiter. A locked tuple
// rejects with ErrSudoLocked until the lockout expires, regardless
// of whether the supplied PIN is correct (so the attacker can't
// distinguish "right PIN, locked out" from "wrong PIN" while the
// counter is full).
func (s *SudoService) Sudo(ctx context.Context, userID, layoutID uint, pin string) (domain.SudoElevation, error) {
	if userID == 0 || layoutID == 0 {
		return domain.SudoElevation{}, ErrSudoInvalidInput
	}

	now := time.Now().UTC()
	if err := s.checkPIN(ctx, userID, layoutID, pin, now); err != nil {
		return domain.SudoElevation{}, err
	}

	row := domain.SudoElevation{
		UserID:    userID,
		LayoutID:  layoutID,
		GrantedAt: now,
		ExpiresAt: now.Add(s.cfg.TTL),
	}
	if err := s.elevations.Upsert(ctx, &row); err != nil {
		return domain.SudoElevation{}, err
	}
	s.broadcastElevationChanged(userID, layoutID)
	return row, nil
}

// Revoke deletes the SudoElevation row for (userID, layoutID).
// Idempotent — a missing row returns nil so the UI's "lock the lock"
// button never surfaces a 404.
func (s *SudoService) Revoke(ctx context.Context, userID, layoutID uint) error {
	if err := s.elevations.Delete(ctx, userID, layoutID); err != nil {
		return err
	}
	s.broadcastElevationChanged(userID, layoutID)
	return nil
}

// FindActive returns the active sudo grant the user holds inside
// the layout at `now`, or nil when absent. AuthService.Effective
// consumes it.
func (s *SudoService) FindActive(ctx context.Context, userID, layoutID uint, now time.Time) (*domain.SudoElevation, error) {
	row, err := s.elevations.FindActive(ctx, userID, layoutID, now)
	if err != nil {
		if errors.Is(err, repo.ErrSudoElevationNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// GrantSignalman verifies the layout admin PIN and persists a
// **permanent** signalman grant for (UserID, LayoutID) by upserting
// a `layout_signalmen` row with `ExpiresAt = nil`. Unlike Sudo this
// is not time-bounded — clicking the engineer's-cap icon promotes
// the user inside the layout until they (or an admin) revoke it
// via RevokeSignalman.
//
// The PIN gate is the same one as Sudo and shares the rate-limiter
// state, so brute-forcing through either icon is equally throttled.
func (s *SudoService) GrantSignalman(ctx context.Context, userID, layoutID uint, pin string) error {
	if userID == 0 || layoutID == 0 {
		return ErrSudoInvalidInput
	}

	now := time.Now().UTC()
	if err := s.checkPIN(ctx, userID, layoutID, pin, now); err != nil {
		return err
	}

	row := domain.LayoutSignalman{
		LayoutID:  layoutID,
		UserID:    userID,
		GrantedBy: userID, // self-grant via the AppBar icon
		GrantedAt: now,
		ExpiresAt: nil,
	}
	if err := s.signalmen.Upsert(ctx, &row); err != nil {
		return err
	}
	s.broadcastElevationChanged(userID, layoutID)
	return nil
}

// GrantSignalmanToUser persists a permanent signalman grant for
// targetUserID inside layoutID. grantedBy is recorded on the row
// (typically an effective admin). Unlike GrantSignalman no PIN is
// verified here — the HTTP layer must gate the call.
func (s *SudoService) GrantSignalmanToUser(ctx context.Context, grantedBy, targetUserID, layoutID uint) error {
	if grantedBy == 0 || targetUserID == 0 || layoutID == 0 {
		return ErrSudoInvalidInput
	}
	if _, err := s.layouts.Get(ctx, layoutID); err != nil {
		return err
	}

	now := time.Now().UTC()
	row := domain.LayoutSignalman{
		LayoutID:  layoutID,
		UserID:    targetUserID,
		GrantedBy: grantedBy,
		GrantedAt: now,
		ExpiresAt: nil,
	}
	if err := s.signalmen.Upsert(ctx, &row); err != nil {
		return err
	}
	s.broadcastElevationChanged(targetUserID, layoutID)
	return nil
}

// RevokeSignalman drops the user's permanent signalman grant inside
// the layout. Idempotent — a missing row returns nil.
func (s *SudoService) RevokeSignalman(ctx context.Context, userID, layoutID uint) error {
	if err := s.signalmen.Delete(ctx, layoutID, userID); err != nil {
		return err
	}
	s.broadcastElevationChanged(userID, layoutID)
	return nil
}

// checkPIN verifies the layout admin PIN and feeds the rate-limiter
// on miss. Returns the unwrapped sudo-domain error; callers should
// treat it as opaque and propagate.
func (s *SudoService) checkPIN(ctx context.Context, userID, layoutID uint, pin string, now time.Time) error {
	key := failureKey{UserID: userID, LayoutID: layoutID}
	if locked, _ := s.lockedUntil(key, now); locked {
		return ErrSudoLocked
	}
	if err := s.layouts.VerifyAdminPIN(ctx, layoutID, pin); err != nil {
		if errors.Is(err, ErrLayoutAdminPINMismatch) {
			s.recordFailure(key, now)
			return ErrSudoInvalidPIN
		}
		return err
	}
	s.clearFailures(key)
	return nil
}

// RunJanitor drives the periodic reap loop. It blocks until ctx is
// cancelled. Reaped rows trigger an `auth.elevationChanged`
// broadcast for every affected (user, layout) pair.
func (s *SudoService) RunJanitor(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.JanitorInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.reapOnce(ctx)
		}
	}
}

func (s *SudoService) reapOnce(ctx context.Context) {
	rows, err := s.elevations.ReapExpired(ctx, time.Now().UTC())
	if err != nil {
		return
	}
	for _, r := range rows {
		s.broadcastElevationChanged(r.UserID, r.LayoutID)
	}
}

// broadcastElevationChanged emits the per-user fan-out described in
// §7a.7. The hub may be nil in tests — guard the call.
func (s *SudoService) broadcastElevationChanged(userID, layoutID uint) {
	if s.hub == nil {
		return
	}
	s.hub.BroadcastToUserInLayout(layoutID, userID, "auth.elevationChanged",
		ws.ElevationChangedPayload{
			LayoutID: layoutID,
			UserID:   userID,
		})
}

// recordFailure appends the failure stamp and arms a lockout when
// the rolling counter reaches MaxFailures.
func (s *SudoService) recordFailure(key failureKey, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := now.Add(-s.cfg.FailWindow)
	stamps := s.failures[key]
	pruned := stamps[:0]
	for _, t := range stamps {
		if t.After(cutoff) {
			pruned = append(pruned, t)
		}
	}
	pruned = append(pruned, now)
	s.failures[key] = pruned
	if len(pruned) >= s.cfg.MaxFailures {
		s.locks[key] = now.Add(s.cfg.LockDuration)
		s.failures[key] = nil
	}
}

func (s *SudoService) clearFailures(key failureKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.failures, key)
	delete(s.locks, key)
}

// lockedUntil reports whether the tuple is currently locked out and
// returns the unlock time when so. Stale lock entries (already
// expired) are pruned opportunistically.
func (s *SudoService) lockedUntil(key failureKey, now time.Time) (bool, time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	deadline, ok := s.locks[key]
	if !ok {
		return false, time.Time{}
	}
	if !deadline.After(now) {
		delete(s.locks, key)
		return false, time.Time{}
	}
	return true, deadline
}

// LockedRetryAfter returns the Retry-After hint (in seconds, rounded
// up) for a locked tuple, or 0 when the tuple is not locked. The
// HTTP layer reads it directly when shaping a 429 response.
func (s *SudoService) LockedRetryAfter(userID, layoutID uint) int {
	now := time.Now().UTC()
	_, deadline := s.lockedUntil(failureKey{UserID: userID, LayoutID: layoutID}, now)
	if deadline.IsZero() {
		return 0
	}
	d := deadline.Sub(now)
	if d <= 0 {
		return 0
	}
	secs := int(d.Round(time.Second) / time.Second)
	if secs < 1 {
		secs = 1
	}
	return secs
}

// String avoids accidental nil-deref panics inside log messages.
func (s *SudoService) String() string {
	if s == nil {
		return "<nil SudoService>"
	}
	return fmt.Sprintf("SudoService(ttl=%s, max_failures=%d, lock=%s)",
		s.cfg.TTL, s.cfg.MaxFailures, s.cfg.LockDuration)
}
