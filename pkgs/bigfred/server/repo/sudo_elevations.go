package repo

import (
	"context"
	"errors"
	"time"

	"github.com/go-rel/rel"
	"github.com/go-rel/rel/where"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// ErrSudoElevationNotFound is returned by SudoElevations.FindActive
// when no row matches. Callers MUST treat it as "not elevated".
var ErrSudoElevationNotFound = errors.New("sudo elevation not found")

// SudoElevations is the persistence adapter for domain.SudoElevation
// (§7a.7). Construct it with NewSudoElevations; the zero value is
// unusable.
type SudoElevations struct {
	repo rel.Repository
}

var _ SudoElevationStore = (*SudoElevations)(nil)

// NewSudoElevations returns a SudoElevations repository bound to the
// given REL instance.
func NewSudoElevations(r rel.Repository) *SudoElevations {
	return &SudoElevations{repo: r}
}

// RequiresJanitor implements SudoElevationStore.
func (s *SudoElevations) RequiresJanitor() bool { return true }

// FindActive returns the active grant for (userID, layoutID) at
// `now`. Returns ErrSudoElevationNotFound when absent or expired.
func (s *SudoElevations) FindActive(
	ctx context.Context, userID, layoutID uint, now time.Time,
) (domain.SudoElevation, error) {
	var row domain.SudoElevation
	err := s.repo.Find(ctx, &row,
		where.Eq("user_id", userID),
		where.Eq("layout_id", layoutID),
	)
	if err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return domain.SudoElevation{}, ErrSudoElevationNotFound
		}
		return domain.SudoElevation{}, err
	}
	if !row.IsActive(now) {
		return domain.SudoElevation{}, ErrSudoElevationNotFound
	}
	return row, nil
}

// Upsert atomically persists a new grant or refreshes the timer of
// an existing one. The DB-level uniqueness on (user_id, layout_id)
// guarantees the "at most one active row per pair" invariant.
//
// Returns the persisted row (with ID + final timestamps) so callers
// can broadcast `auth.elevationChanged` immediately.
func (s *SudoElevations) Upsert(ctx context.Context, row *domain.SudoElevation) error {
	var existing domain.SudoElevation
	err := s.repo.Find(ctx, &existing,
		where.Eq("user_id", row.UserID),
		where.Eq("layout_id", row.LayoutID),
	)
	if err == nil {
		existing.GrantedAt = row.GrantedAt
		existing.ExpiresAt = row.ExpiresAt
		if err := s.repo.Update(ctx, &existing); err != nil {
			return err
		}
		*row = existing
		return nil
	}
	if !errors.Is(err, rel.ErrNotFound) {
		return err
	}
	return s.repo.Insert(ctx, row)
}

// Delete removes the grant for (userID, layoutID). Idempotent —
// returns nil when no row exists so callers don't have to special-case
// "already revoked".
func (s *SudoElevations) Delete(ctx context.Context, userID, layoutID uint) error {
	var row domain.SudoElevation
	err := s.repo.Find(ctx, &row,
		where.Eq("user_id", userID),
		where.Eq("layout_id", layoutID),
	)
	if err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return nil
		}
		return err
	}
	return s.repo.Delete(ctx, &row)
}

// ReapExpired deletes every grant whose ExpiresAt has passed. The
// returned slice carries the rows that were just removed so the
// janitor can broadcast `auth.elevationChanged` for the affected
// users.
func (s *SudoElevations) ReapExpired(ctx context.Context, now time.Time) ([]domain.SudoElevation, error) {
	var rows []domain.SudoElevation
	err := s.repo.FindAll(ctx, &rows, where.Lte("expires_at", now))
	if err != nil {
		return nil, err
	}
	for i := range rows {
		if err := s.repo.Delete(ctx, &rows[i]); err != nil {
			return nil, err
		}
	}
	return rows, nil
}
