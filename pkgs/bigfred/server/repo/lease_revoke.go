package repo

import (
	"context"
	"errors"
	"time"

	"github.com/go-rel/rel"
	"github.com/go-rel/rel/where"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

var errLeaseNotFound = errors.New("lease_not_found")

// RevokeVehicleLease sets revoked_at on a vehicle lease row.
func RevokeVehicleLease(ctx context.Context, r rel.Repository, id uint, now time.Time) error {
	var row domain.VehicleLease
	if err := r.Find(ctx, &row, where.Eq("id", id)); err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return errLeaseNotFound
		}
		return err
	}
	row.RevokedAt = &now
	return r.Update(ctx, &row)
}

// RevokeTrainLease sets revoked_at on a train lease row.
func RevokeTrainLease(ctx context.Context, r rel.Repository, id uint, now time.Time) error {
	var row domain.TrainLease
	if err := r.Find(ctx, &row, where.Eq("id", id)); err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return errLeaseNotFound
		}
		return err
	}
	row.RevokedAt = &now
	return r.Update(ctx, &row)
}

// FindVehicleLeaseByID loads one vehicle lease row.
func FindVehicleLeaseByID(ctx context.Context, r rel.Repository, id uint) (domain.VehicleLease, error) {
	var row domain.VehicleLease
	if err := r.Find(ctx, &row, where.Eq("id", id)); err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return row, errLeaseNotFound
		}
		return row, err
	}
	return row, nil
}

// FindTrainLeaseByID loads one train lease row.
func FindTrainLeaseByID(ctx context.Context, r rel.Repository, id uint) (domain.TrainLease, error) {
	var row domain.TrainLease
	if err := r.Find(ctx, &row, where.Eq("id", id)); err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return row, errLeaseNotFound
		}
		return row, err
	}
	return row, nil
}
