package cmd

import (
	"context"
	"errors"
	"sort"

	"github.com/go-rel/rel"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/security"
)

const (
	minDCCAddress = uint16(1)
	maxDCCAddress = uint16(9999)
	// autoAllocateFirstDCCAddress is where automatic pool allocation starts
	// scanning. Addresses below it are left free for ad-hoc / factory-default
	// decoders (3 is the DCC factory address).
	autoAllocateFirstDCCAddress = uint16(50)
)

// DCCPool orchestrates per-user DCC address pools.
type DCCPool struct {
	db   rel.Repository
	pool *repo.DCCAddressRanges
	sec  security.DCCPoolSecurityContext
}

func NewDCCPool(db rel.Repository, pool *repo.DCCAddressRanges) *DCCPool {
	return &DCCPool{db: db, pool: pool}
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

	sort.SliceStable(clean, func(i, j int) bool { return clean[i].From < clean[j].From })

	err = repo.WithTransaction(ctx, s.db, func(tctx context.Context) error {
		if err := s.pool.DeleteAllForUser(tctx, userID); err != nil {
			return err
		}
		for _, r := range clean {
			row := domain.DCCAddressRange{
				UserID:   userID,
				FromAddr: r.From,
				ToAddr:   r.To,
			}
			if err := s.pool.Insert(tctx, &row); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
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

// allocateFreeDCCAddresses picks count addresses that no user owns yet,
// scanning upwards from autoAllocateFirstDCCAddress, and returns them
// merged into as few contiguous ranges as possible. It returns
// ErrDCCPoolExhausted when fewer than count addresses remain free.
func allocateFreeDCCAddresses(count int, existing []domain.DCCAddressRange) ([]PoolRange, error) {
	if count <= 0 {
		return nil, nil
	}
	occupied := make(map[uint16]struct{})
	for _, r := range existing {
		from, to := r.FromAddr, r.ToAddr
		if from < minDCCAddress {
			from = minDCCAddress
		}
		if to > maxDCCAddress {
			to = maxDCCAddress
		}
		for addr := from; addr <= to; addr++ {
			occupied[addr] = struct{}{}
		}
	}

	out := make([]PoolRange, 0, count)
	remaining := count
	for addr := autoAllocateFirstDCCAddress; addr <= maxDCCAddress && remaining > 0; addr++ {
		if _, taken := occupied[addr]; taken {
			continue
		}
		if n := len(out); n > 0 && out[n-1].To == addr-1 {
			out[n-1].To = addr
		} else {
			out = append(out, PoolRange{From: addr, To: addr})
		}
		remaining--
	}
	if remaining > 0 {
		return nil, svcerrors.ErrDCCPoolExhausted
	}
	return out, nil
}
