package service

import (
	"context"
	"errors"
	"sort"

	"github.com/keskad/loco/pkgs/server/domain"
	"github.com/keskad/loco/pkgs/server/repo"
)

// DCC-pool sentinel errors. Mapped to status codes + i18n keys by the
// HTTP layer.
var (
	// ErrDCCPoolRangeInvalid is returned when from > to or either
	// bound is zero (DCC addresses start at 1).
	ErrDCCPoolRangeInvalid = errors.New("dcc_pool_range_invalid")
	// ErrDCCAddressOutsidePool is returned when a vehicle registration
	// or update points at an address not covered by any pool row.
	ErrDCCAddressOutsidePool = errors.New("dcc_address_outside_pool")
)

// DCCPoolService is the read+write facade over the per-user DCC
// address pool (§3a.1, goal 3). Two responsibilities:
//
//  1. AllowsAddress — predicate used by VehicleService before
//     accepting a non-dummy vehicle registration / update.
//  2. Replace — admin-side "replace pool" operation. Wholesale
//     replacement is intentional: it keeps the audit trail simple and
//     avoids per-row PATCH semantics.
//
// SeedAdminPoolIfEmpty grants the bootstrap admin a wide window so a
// freshly initialised installation can register vehicles without an
// extra round trip through the admin pool page.
type DCCPoolService struct {
	pool *repo.DCCAddressRanges
}

// NewDCCPoolService returns a DCCPoolService.
func NewDCCPoolService(pool *repo.DCCAddressRanges) *DCCPoolService {
	return &DCCPoolService{pool: pool}
}

// List returns the pool rows owned by the user, ordered by lower
// bound for stable UI rendering.
func (s *DCCPoolService) List(ctx context.Context, userID uint) ([]domain.DCCAddressRange, error) {
	return s.pool.ListByUser(ctx, userID)
}

// PoolRange is the validated input row of DCCPoolService.Replace.
type PoolRange struct {
	From uint16
	To   uint16
}

// Replace overwrites the user's pool with the supplied set. Each row
// is sanitised (from ≤ to, both non-zero). Overlap is tolerated —
// the membership test on AllowsAddress only needs *any* row to cover
// the candidate address.
func (s *DCCPoolService) Replace(ctx context.Context, userID uint, ranges []PoolRange) ([]domain.DCCAddressRange, error) {
	clean := make([]PoolRange, 0, len(ranges))
	for _, r := range ranges {
		if r.From == 0 || r.To == 0 || r.From > r.To {
			return nil, ErrDCCPoolRangeInvalid
		}
		clean = append(clean, r)
	}

	if err := s.pool.DeleteAllForUser(ctx, userID); err != nil {
		return nil, err
	}

	sort.SliceStable(clean, func(i, j int) bool { return clean[i].From < clean[j].From })

	for _, r := range clean {
		row := domain.DCCAddressRange{
			UserID:   userID,
			FromAddr: r.From,
			ToAddr:   r.To,
		}
		if err := s.pool.Insert(ctx, &row); err != nil {
			return nil, err
		}
	}
	return s.pool.ListByUser(ctx, userID)
}

// AllowsAddress reports whether the user's pool covers the candidate
// DCC address. Returns true ONLY when at least one row contains addr.
func (s *DCCPoolService) AllowsAddress(ctx context.Context, userID uint, addr uint16) (bool, error) {
	rows, err := s.pool.ListByUser(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, r := range rows {
		if r.Contains(addr) {
			return true, nil
		}
	}
	return false, nil
}

// SeedAdminPoolIfEmpty inserts a generous default pool (1..9999) for
// the admin user when no pool row exists for that user yet. The
// startup sequence calls it right after SeedAdmin so the bootstrap
// installation can register vehicles immediately.
func (s *DCCPoolService) SeedAdminPoolIfEmpty(ctx context.Context, adminUserID uint) error {
	rows, err := s.pool.ListByUser(ctx, adminUserID)
	if err != nil {
		return err
	}
	if len(rows) > 0 {
		return nil
	}
	return s.pool.Insert(ctx, &domain.DCCAddressRange{
		UserID:   adminUserID,
		FromAddr: 1,
		ToAddr:   9999,
	})
}
