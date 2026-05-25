package repo

import (
	"context"

	"github.com/go-rel/rel"
	"github.com/go-rel/rel/sort"
	"github.com/go-rel/rel/where"

	"github.com/keskad/loco/pkgs/server/domain"
)

// DCCAddressRanges is the persistence adapter for domain.DCCAddressRange.
type DCCAddressRanges struct {
	repo rel.Repository
}

// NewDCCAddressRanges returns a DCCAddressRanges repository.
func NewDCCAddressRanges(r rel.Repository) *DCCAddressRanges {
	return &DCCAddressRanges{repo: r}
}

// ListByUser returns every pool row for a user, ordered by FromAddr
// so the UI can render them as a sorted list without re-sorting.
func (d *DCCAddressRanges) ListByUser(ctx context.Context, userID uint) ([]domain.DCCAddressRange, error) {
	var rows []domain.DCCAddressRange
	err := d.repo.FindAll(ctx, &rows,
		where.Eq("user_id", userID),
		sort.Asc("from_addr"),
	)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// Insert persists a new pool row.
func (d *DCCAddressRanges) Insert(ctx context.Context, row *domain.DCCAddressRange) error {
	return d.repo.Insert(ctx, row)
}

// DeleteAllForUser removes every pool row for the user. Used by the
// admin "replace pool" endpoint, which always writes a fresh set.
func (d *DCCAddressRanges) DeleteAllForUser(ctx context.Context, userID uint) error {
	var rows []domain.DCCAddressRange
	if err := d.repo.FindAll(ctx, &rows, where.Eq("user_id", userID)); err != nil {
		return err
	}
	for i := range rows {
		if err := d.repo.Delete(ctx, &rows[i]); err != nil {
			return err
		}
	}
	return nil
}
