package cmd

import (
	"context"
	"errors"
	"sort"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/security"
)

const (
	minDCCAddress = uint16(1)
	maxDCCAddress = uint16(9999)
)

// DCCPool orchestrates per-user DCC address pools.
type DCCPool struct {
	pool *repo.DCCAddressRanges
	sec  security.DCCPoolSecurityContext
}

func NewDCCPool(pool *repo.DCCAddressRanges) *DCCPool {
	return &DCCPool{pool: pool}
}

func (s *DCCPool) List(ctx context.Context, userID uint) ([]domain.DCCAddressRange, error) {
	return s.pool.ListByUser(ctx, userID)
}

func (s *DCCPool) Validate(ctx context.Context, userID uint, ranges []PoolRange) error {
	allRows, err := s.pool.ListAll(ctx)
	if err != nil {
		return err
	}
	_, err = validatePoolRanges(userID, ranges, allRows)
	return err
}

func (s *DCCPool) Replace(ctx context.Context, eff domain.EffectiveRoles, userID uint, ranges []PoolRange) ([]domain.DCCAddressRange, error) {
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

func (s *DCCPool) ListAll(ctx context.Context) ([]domain.DCCAddressRange, error) {
	return s.pool.ListAll(ctx)
}

func (s *DCCPool) AllowsAddress(ctx context.Context, userID uint, addr uint16) (bool, error) {
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

func (s *DCCPool) SeedAdminPoolIfEmpty(ctx context.Context, adminUserID uint) error {
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

func (s *DCCPool) DeleteForUser(ctx context.Context, eff domain.EffectiveRoles, userID uint) error {
	if err := s.checkManageDCCPool(eff); err != nil {
		return err
	}
	return s.pool.DeleteAllForUser(ctx, userID)
}

func (s *DCCPool) checkManageDCCPool(eff domain.EffectiveRoles) error {
	decision := s.sec.CanManageDCCPool(eff)
	if decision.Allowed {
		return nil
	}
	switch decision.Reason {
	case security.ReasonForbidden:
		return svcerrors.ErrDCCPoolForbidden
	default:
		return errors.New(decision.Reason)
	}
}

func validatePoolRanges(userID uint, ranges []PoolRange, existing []domain.DCCAddressRange) ([]PoolRange, error) {
	if len(ranges) == 0 {
		return nil, svcerrors.ErrDCCPoolEmpty
	}

	clean := make([]PoolRange, 0, len(ranges))
	for _, r := range ranges {
		if r.From < minDCCAddress || r.To > maxDCCAddress || r.From > r.To {
			return nil, svcerrors.ErrDCCPoolRangeInvalid
		}
		clean = append(clean, r)
	}

	for _, r := range clean {
		for _, row := range existing {
			if row.UserID == userID {
				continue
			}
			if poolRangesOverlap(r, PoolRange{From: row.FromAddr, To: row.ToAddr}) {
				return nil, svcerrors.ErrDCCPoolOverlap
			}
		}
	}

	return clean, nil
}

func poolRangesOverlap(a, b PoolRange) bool {
	return a.From <= b.To && b.From <= a.To
}
