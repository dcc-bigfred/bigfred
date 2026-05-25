package repo

import (
	"context"
	"errors"
	"time"

	"github.com/go-rel/rel"
	"github.com/go-rel/rel/where"

	"github.com/keskad/loco/pkgs/server/domain"
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
