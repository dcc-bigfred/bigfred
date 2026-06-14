package repo

import (
	"context"
	"errors"
	"time"

	"github.com/go-rel/rel"
	"github.com/go-rel/rel/where"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// ErrLayoutSignalmanNotFound is returned when no matching grant exists.
var ErrLayoutSignalmanNotFound = errors.New("layout signalman not found")

// LayoutSignalmen is the persistence adapter for layout-scoped
// signalman grants.
type LayoutSignalmen struct {
	repo rel.Repository
}

// NewLayoutSignalmen returns a LayoutSignalmen repository.
func NewLayoutSignalmen(r rel.Repository) *LayoutSignalmen {
	return &LayoutSignalmen{repo: r}
}

// FindActiveGrant returns the grant row for (layout, user) when it
// has not expired. Returns ErrLayoutSignalmanNotFound when absent or
// expired.
func (l *LayoutSignalmen) FindActiveGrant(ctx context.Context, layoutID, userID uint, now time.Time) (domain.LayoutSignalman, error) {
	var row domain.LayoutSignalman
	err := l.repo.Find(ctx, &row,
		where.Eq("layout_id", layoutID),
		where.Eq("user_id", userID),
	)
	if err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return domain.LayoutSignalman{}, ErrLayoutSignalmanNotFound
		}
		return domain.LayoutSignalman{}, err
	}
	if row.ExpiresAt != nil && !row.ExpiresAt.After(now) {
		return domain.LayoutSignalman{}, ErrLayoutSignalmanNotFound
	}
	return row, nil
}

// HasActiveGrant reports whether the user holds a non-expired
// signalman grant in the layout.
func (l *LayoutSignalmen) HasActiveGrant(ctx context.Context, layoutID, userID uint, now time.Time) (bool, error) {
	_, err := l.FindActiveGrant(ctx, layoutID, userID, now)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, ErrLayoutSignalmanNotFound) {
		return false, nil
	}
	return false, err
}

// Upsert persists a permanent (ExpiresAt = nil) or temporary grant.
// When a row already exists for (layout_id, user_id) the timestamps
// are refreshed in place — the unique index on the join keeps the
// "at most one row per (layout, user)" invariant intact. The
// supplied struct is rewritten with the persisted ID so the caller
// can audit / broadcast straight after the call.
func (l *LayoutSignalmen) Upsert(ctx context.Context, row *domain.LayoutSignalman) error {
	var existing domain.LayoutSignalman
	err := l.repo.Find(ctx, &existing,
		where.Eq("layout_id", row.LayoutID),
		where.Eq("user_id", row.UserID),
	)
	if err == nil {
		existing.GrantedBy = row.GrantedBy
		existing.GrantedAt = row.GrantedAt
		existing.ExpiresAt = row.ExpiresAt
		if err := l.repo.Update(ctx, &existing); err != nil {
			return err
		}
		*row = existing
		return nil
	}
	if !errors.Is(err, rel.ErrNotFound) {
		return err
	}
	return l.repo.Insert(ctx, row)
}

// Delete removes the grant for (layoutID, userID). Idempotent —
// returns nil when no row exists.
func (l *LayoutSignalmen) Delete(ctx context.Context, layoutID, userID uint) error {
	var row domain.LayoutSignalman
	err := l.repo.Find(ctx, &row,
		where.Eq("layout_id", layoutID),
		where.Eq("user_id", userID),
	)
	if err != nil {
		if errors.Is(err, ErrLayoutSignalmanNotFound) || errors.Is(err, rel.ErrNotFound) {
			return nil
		}
		return err
	}
	return l.repo.Delete(ctx, &row)
}
