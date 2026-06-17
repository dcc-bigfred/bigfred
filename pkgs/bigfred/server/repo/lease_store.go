package repo

import (
	"context"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// VehicleLeaseStore persists active vehicle drive leases.
type VehicleLeaseStore interface {
	ListActive(ctx context.Context, vehicleIDs []uint, now time.Time) ([]domain.VehicleLease, error)
	Insert(ctx context.Context, row *domain.VehicleLease) error
	Revoke(ctx context.Context, vehicleID uint, now time.Time) error
	RequiresJanitor() bool
}

// TrainLeaseStore persists active train drive leases.
type TrainLeaseStore interface {
	ListActive(ctx context.Context, trainIDs []uint, now time.Time) ([]domain.TrainLease, error)
	Insert(ctx context.Context, row *domain.TrainLease) error
	Revoke(ctx context.Context, trainID uint, now time.Time) error
	RequiresJanitor() bool
}
