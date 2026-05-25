package service

import (
	"context"
	"errors"
	"sort"

	"github.com/keskad/loco/pkgs/server/domain"
	"github.com/keskad/loco/pkgs/server/repo"
	"github.com/keskad/loco/pkgs/server/security"
)

const (
	minDCCAddress = uint16(1)
	maxDCCAddress = uint16(9999)
)

// DCC-pool sentinel errors. Mapped to status codes + i18n keys by the
// HTTP layer.
var (
	// ErrDCCPoolEmpty is returned when the admin submits no ranges.
	ErrDCCPoolEmpty = errors.New("dcc_pool_empty")
	// ErrDCCPoolRangeInvalid is returned when from > to or either
	// bound falls outside [1, 9999].
	ErrDCCPoolRangeInvalid = errors.New("dcc_pool_range_invalid")
	// ErrDCCPoolOverlap is returned when a range intersects another
	// user's pool row.
	ErrDCCPoolOverlap = errors.New("dcc_pool_overlap")
	// ErrDCCAddressOutsidePool is returned when a vehicle registration
	// or update points at an address not covered by any pool row.
	ErrDCCAddressOutsidePool = errors.New("dcc_address_outside_pool")

	// ErrDCCPoolForbidden is returned when a non-admin attempts a pool
	// mutation guarded by CanManageDCCPool.
	ErrDCCPoolForbidden = errors.New("forbidden")
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
	sec  security.DCCPoolSecurityContext
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

// Validate checks whether ranges could be assigned to userID without
// persisting anything. Pass userID = 0 when the account does not
// exist yet (Create).
func (s *DCCPoolService) Validate(ctx context.Context, userID uint, ranges []PoolRange) error {
	allRows, err := s.pool.ListAll(ctx)
	if err != nil {
		return err
	}
	_, err = validatePoolRanges(userID, ranges, allRows)
	return err
}

// Replace overwrites the user's pool with the supplied set. Each row
// must lie inside [1, 9999] and must not overlap any other user's
// ranges. Overlap within the same user's rows is tolerated — the
// membership test on AllowsAddress only needs *any* row to cover
// the candidate address.
func (s *DCCPoolService) Replace(ctx context.Context, eff domain.EffectiveRoles, userID uint, ranges []PoolRange) ([]domain.DCCAddressRange, error) {
	if err := s.checkManageDCCPool(eff); err != nil {
		return nil, err
	}
	allRows, err := s.pool.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	clean, err := validatePoolRanges(userID, ranges, allRows)
	if err != nil {
		return nil, err
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

// ListAll returns every pool row in the database.
func (s *DCCPoolService) ListAll(ctx context.Context) ([]domain.DCCAddressRange, error) {
	return s.pool.ListAll(ctx)
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
		FromAddr: minDCCAddress,
		ToAddr:   maxDCCAddress,
	})
}

// DeleteForUser removes every pool row owned by the user.
func (s *DCCPoolService) DeleteForUser(ctx context.Context, eff domain.EffectiveRoles, userID uint) error {
	if err := s.checkManageDCCPool(eff); err != nil {
		return err
	}
	return s.pool.DeleteAllForUser(ctx, userID)
}

func (s *DCCPoolService) checkManageDCCPool(eff domain.EffectiveRoles) error {
	decision := s.sec.CanManageDCCPool(eff)
	if decision.Allowed {
		return nil
	}
	switch decision.Reason {
	case "forbidden":
		return ErrDCCPoolForbidden
	default:
		return errors.New(decision.Reason)
	}
}

func validatePoolRanges(userID uint, ranges []PoolRange, existing []domain.DCCAddressRange) ([]PoolRange, error) {
	if len(ranges) == 0 {
		return nil, ErrDCCPoolEmpty
	}

	clean := make([]PoolRange, 0, len(ranges))
	for _, r := range ranges {
		if r.From < minDCCAddress || r.To > maxDCCAddress || r.From > r.To {
			return nil, ErrDCCPoolRangeInvalid
		}
		clean = append(clean, r)
	}

	for _, r := range clean {
		for _, row := range existing {
			if row.UserID == userID {
				continue
			}
			if poolRangesOverlap(r, PoolRange{From: row.FromAddr, To: row.ToAddr}) {
				return nil, ErrDCCPoolOverlap
			}
		}
	}

	return clean, nil
}

func poolRangesOverlap(a, b PoolRange) bool {
	return a.From <= b.To && b.From <= a.To
}
