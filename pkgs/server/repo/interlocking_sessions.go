package repo

import (
	"context"
	"errors"
	"time"

	"github.com/go-rel/rel"
	"github.com/go-rel/rel/where"

	"github.com/keskad/loco/pkgs/server/domain"
)

// ErrInterlockingSessionNotFound is returned when no active session
// matches the query.
var ErrInterlockingSessionNotFound = errors.New("interlocking session not found")

// InterlockingSessions is the persistence adapter for interlocking
// occupation rows.
type InterlockingSessions struct {
	repo rel.Repository
}

// NewInterlockingSessions returns an InterlockingSessions repository.
func NewInterlockingSessions(r rel.Repository) *InterlockingSessions {
	return &InterlockingSessions{repo: r}
}

// FindActiveByInterlocking returns the open session for an
// interlocking, if any.
func (i *InterlockingSessions) FindActiveByInterlocking(ctx context.Context, interlockingID uint) (domain.InterlockingSession, error) {
	var row domain.InterlockingSession
	err := i.repo.Find(ctx, &row,
		where.Eq("interlocking_id", interlockingID),
		where.Nil("ended_at"),
	)
	if err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return domain.InterlockingSession{}, ErrInterlockingSessionNotFound
		}
		return domain.InterlockingSession{}, err
	}
	return row, nil
}

// FindActiveByUser returns the open session staffed by the user, if
// any (a signalman occupies at most one box at a time in v1).
func (i *InterlockingSessions) FindActiveByUser(ctx context.Context, userID uint) (domain.InterlockingSession, error) {
	var row domain.InterlockingSession
	err := i.repo.Find(ctx, &row,
		where.Eq("signalman_user_id", userID),
		where.Nil("ended_at"),
	)
	if err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return domain.InterlockingSession{}, ErrInterlockingSessionNotFound
		}
		return domain.InterlockingSession{}, err
	}
	return row, nil
}

// ListActive returns every open session.
func (i *InterlockingSessions) ListActive(ctx context.Context) ([]domain.InterlockingSession, error) {
	var rows []domain.InterlockingSession
	err := i.repo.FindAll(ctx, &rows, where.Nil("ended_at"))
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// Insert persists a new session row.
func (i *InterlockingSessions) Insert(ctx context.Context, row *domain.InterlockingSession) error {
	return i.repo.Insert(ctx, row)
}

// End marks a session closed.
func (i *InterlockingSessions) End(ctx context.Context, row *domain.InterlockingSession, endedAt time.Time) error {
	row.EndedAt = &endedAt
	return i.repo.Update(ctx, row)
}

// EndAllForUser closes every open session for a user (used when
// displacing or leaving).
func (i *InterlockingSessions) EndAllForUser(ctx context.Context, userID uint, endedAt time.Time) error {
	var rows []domain.InterlockingSession
	err := i.repo.FindAll(ctx, &rows,
		where.Eq("signalman_user_id", userID),
		where.Nil("ended_at"),
	)
	if err != nil {
		return err
	}
	for idx := range rows {
		if err := i.End(ctx, &rows[idx], endedAt); err != nil {
			return err
		}
	}
	return nil
}
