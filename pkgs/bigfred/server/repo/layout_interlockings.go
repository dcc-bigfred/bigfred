package repo

import (
	"context"
	"errors"

	"github.com/go-rel/rel"
	"github.com/go-rel/rel/where"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// LayoutInterlockings is the persistence adapter for the layout ↔
// interlocking whitelist join table.
type LayoutInterlockings struct {
	repo rel.Repository
}

// NewLayoutInterlockings returns a LayoutInterlockings repository.
func NewLayoutInterlockings(r rel.Repository) *LayoutInterlockings {
	return &LayoutInterlockings{repo: r}
}

// ListByLayoutID returns every whitelist row for the given layout.
func (l *LayoutInterlockings) ListByLayoutID(ctx context.Context, layoutID uint) ([]domain.LayoutInterlocking, error) {
	var rows []domain.LayoutInterlocking
	err := l.repo.FindAll(ctx, &rows, where.Eq("layout_id", layoutID))
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// InterlockingIDsForLayout returns the interlocking ids whitelisted
// for a layout, in no particular order.
func (l *LayoutInterlockings) InterlockingIDsForLayout(ctx context.Context, layoutID uint) ([]uint, error) {
	rows, err := l.ListByLayoutID(ctx, layoutID)
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.InterlockingID)
	}
	return ids, nil
}

// Exists checks whether a (layout, interlocking) pair is already
// whitelisted.
func (l *LayoutInterlockings) Exists(ctx context.Context, layoutID, interlockingID uint) (bool, error) {
	n, err := l.repo.Count(ctx, "layout_interlockings",
		where.Eq("layout_id", layoutID),
		where.Eq("interlocking_id", interlockingID),
	)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// Insert persists a new whitelist row.
func (l *LayoutInterlockings) Insert(ctx context.Context, row *domain.LayoutInterlocking) error {
	return l.repo.Insert(ctx, row)
}

// DeleteByLayoutAndInterlocking removes a single whitelist row.
func (l *LayoutInterlockings) DeleteByLayoutAndInterlocking(ctx context.Context, layoutID, interlockingID uint) error {
	var row domain.LayoutInterlocking
	err := l.repo.Find(ctx, &row,
		where.Eq("layout_id", layoutID),
		where.Eq("interlocking_id", interlockingID),
	)
	if err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return nil
		}
		return err
	}
	return l.repo.Delete(ctx, &row)
}

// DeleteAllForLayout removes every whitelist row for a layout.
func (l *LayoutInterlockings) DeleteAllForLayout(ctx context.Context, layoutID uint) error {
	rows, err := l.ListByLayoutID(ctx, layoutID)
	if err != nil {
		return err
	}
	for i := range rows {
		if err := l.repo.Delete(ctx, &rows[i]); err != nil {
			return err
		}
	}
	return nil
}

// DeleteAllForInterlocking removes every whitelist row pointing at an
// interlocking (used when the interlocking itself is deleted).
func (l *LayoutInterlockings) DeleteAllForInterlocking(ctx context.Context, interlockingID uint) error {
	var rows []domain.LayoutInterlocking
	err := l.repo.FindAll(ctx, &rows, where.Eq("interlocking_id", interlockingID))
	if err != nil {
		return err
	}
	for i := range rows {
		if err := l.repo.Delete(ctx, &rows[i]); err != nil {
			return err
		}
	}
	return nil
}
