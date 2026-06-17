// Sudo implements the layout-scoped "sudo" elevation flow
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

package cmd

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
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

// Sudo implements the elevation surface. The struct is safe
// for concurrent use — every shared field is guarded by `mu`.
type Sudo struct {
	elevations *repo.SudoElevations
	signalmen  *repo.LayoutSignalmen
	layouts    *Layout
	hub        SudoHubPort
	cfg        SudoConfig

	mu       sync.Mutex
	failures map[failureKey][]time.Time // (userID, layoutID) → recent failure stamps
	locks    map[failureKey]time.Time   // (userID, layoutID) → unlock-not-before
}

type failureKey struct {
	UserID   uint
	LayoutID uint
}

// NewSudo constructs a Sudo. `cfg` may be the zero
// value — every field falls back to DefaultSudoConfig when unset.
func NewSudo(
	elevations *repo.SudoElevations,
	signalmen *repo.LayoutSignalmen,
	layouts *Layout,
	hub SudoHubPort,
	cfg SudoConfig,
) *Sudo {
	if elevations == nil {
		panic("service.NewSudo: SudoElevations must not be nil")
	}
	if signalmen == nil {
		panic("service.NewSudo: LayoutSignalmen must not be nil")
	}
	if layouts == nil {
		panic("service.NewSudo: Layout must not be nil")
	}
	cfg = mergeSudoConfig(cfg)
	return &Sudo{
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
func (s *Sudo) TTL() time.Duration { return s.cfg.TTL }

// Sudo verifies the layout admin PIN and persists/refreshes the
// `(UserID, LayoutID)` SudoElevation row, granting the caller the
// `admin` role for `cfg.TTL` (2 min by default). A second call
// while the row is still active simply pushes ExpiresAt forward,
// mirroring the spec "click again → re-arm the timer".
//
// Failed attempts feed the in-memory rate-limiter. A locked tuple
// rejects with svcerrors.ErrSudoLocked until the lockout expires, regardless
// of whether the supplied PIN is correct (so the attacker can't
// distinguish "right PIN, locked out" from "wrong PIN" while the
// counter is full).
func (s *Sudo) Sudo(ctx context.Context, userID, layoutID uint, pin string) (domain.SudoElevation, error) {
	if userID == 0 || layoutID == 0 {
		return domain.SudoElevation{}, svcerrors.ErrSudoInvalidInput
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
func (s *Sudo) Revoke(ctx context.Context, userID, layoutID uint) error {
	if err := s.elevations.Delete(ctx, userID, layoutID); err != nil {
		return err
	}
	s.broadcastElevationChanged(userID, layoutID)
	return nil
}

// FindActive returns the active sudo grant the user holds inside
// the layout at `now`, or nil when absent. AuthService.Effective
// consumes it.
func (s *Sudo) FindActive(ctx context.Context, userID, layoutID uint, now time.Time) (*domain.SudoElevation, error) {
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
func (s *Sudo) GrantSignalman(ctx context.Context, userID, layoutID uint, pin string) error {
	if userID == 0 || layoutID == 0 {
		return svcerrors.ErrSudoInvalidInput
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
func (s *Sudo) GrantSignalmanToUser(ctx context.Context, grantedBy, targetUserID, layoutID uint) error {
	if grantedBy == 0 || targetUserID == 0 || layoutID == 0 {
		return svcerrors.ErrSudoInvalidInput
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
func (s *Sudo) RevokeSignalman(ctx context.Context, userID, layoutID uint) error {
	if err := s.signalmen.Delete(ctx, layoutID, userID); err != nil {
		return err
	}
	s.broadcastElevationChanged(userID, layoutID)
	return nil
}

// checkPIN verifies the layout admin PIN and feeds the rate-limiter
// on miss. Returns the unwrapped sudo-domain error; callers should
// treat it as opaque and propagate.
func (s *Sudo) checkPIN(ctx context.Context, userID, layoutID uint, pin string, now time.Time) error {
	key := failureKey{UserID: userID, LayoutID: layoutID}
	if locked, _ := s.lockedUntil(key, now); locked {
		return svcerrors.ErrSudoLocked
	}
	if err := s.layouts.VerifyAdminPIN(ctx, layoutID, pin); err != nil {
		if errors.Is(err, svcerrors.ErrLayoutAdminPINMismatch) {
			s.recordFailure(key, time.Now().UTC())
			return svcerrors.ErrSudoInvalidPIN
		}
		return err
	}
	s.clearFailures(key)
	return nil
}

// RunJanitor drives the periodic reap loop. It blocks until ctx is
// cancelled. Reaped rows trigger an `auth.elevationChanged`
// broadcast for every affected (user, layout) pair.
func (s *Sudo) RunJanitor(ctx context.Context) {
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

func (s *Sudo) reapOnce(ctx context.Context) {
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
func (s *Sudo) broadcastElevationChanged(userID, layoutID uint) {
	if s.hub == nil {
		return
	}
	s.hub.BroadcastElevationChanged(layoutID, userID)
}

// recordFailure appends the failure stamp and arms a lockout when
// the rolling counter reaches MaxFailures.
func (s *Sudo) recordFailure(key failureKey, now time.Time) {
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

func (s *Sudo) clearFailures(key failureKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.failures, key)
	delete(s.locks, key)
}

// lockedUntil reports whether the tuple is currently locked out and
// returns the unlock time when so. Stale lock entries (already
// expired) are pruned opportunistically.
func (s *Sudo) lockedUntil(key failureKey, now time.Time) (bool, time.Time) {
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
func (s *Sudo) LockedRetryAfter(userID, layoutID uint) int {
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
func (s *Sudo) String() string {
	if s == nil {
		return "<nil Sudo>"
	}
	return fmt.Sprintf("Sudo(ttl=%s, max_failures=%d, lock=%s)",
		s.cfg.TTL, s.cfg.MaxFailures, s.cfg.LockDuration)
}
