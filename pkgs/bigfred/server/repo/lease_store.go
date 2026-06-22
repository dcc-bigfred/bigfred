package repo

import (
	"context"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// VehicleLeaseStore persists active vehicle drive leases in Redis.
type VehicleLeaseStore interface {
	Get(ctx context.Context, vehicleID domain.VehicleID) (domain.VehicleLease, bool, error)
	ListActive(ctx context.Context, vehicleIDs []domain.VehicleID, now time.Time) ([]domain.VehicleLease, error)
	ListByOwner(ctx context.Context, ownerID uint) ([]domain.VehicleLease, error)
	ListByLessee(ctx context.Context, lesseeID uint) ([]domain.VehicleLease, error)
	ListAll(ctx context.Context) ([]domain.VehicleLease, error)
	Create(ctx context.Context, row *domain.VehicleLease, overwrite bool) (bool, error)
	Update(ctx context.Context, row *domain.VehicleLease) error
	Revoke(ctx context.Context, vehicleID domain.VehicleID) error
}

// TrainLeaseStore persists active train drive leases in Redis.
type TrainLeaseStore interface {
	Get(ctx context.Context, trainID domain.TrainID) (domain.TrainLease, bool, error)
	ListActive(ctx context.Context, trainIDs []domain.TrainID, now time.Time) ([]domain.TrainLease, error)
	ListByOwner(ctx context.Context, ownerID uint) ([]domain.TrainLease, error)
	ListByLessee(ctx context.Context, lesseeID uint) ([]domain.TrainLease, error)
	ListAll(ctx context.Context) ([]domain.TrainLease, error)
	Create(ctx context.Context, row *domain.TrainLease, overwrite bool) (bool, error)
	Update(ctx context.Context, row *domain.TrainLease) error
	Revoke(ctx context.Context, trainID domain.TrainID) error
}
