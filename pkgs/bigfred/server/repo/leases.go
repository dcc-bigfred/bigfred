package repo

import (
	"context"
	"time"

	"github.com/go-rel/rel"
	"github.com/go-rel/rel/where"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// Leases is the shared persistence contract implemented by both the
// vehicle and train lease repositories. T is the concrete lease row
// type (domain.VehicleLease or domain.TrainLease); the repositories
// differ only in the owning foreign-key column they filter on.
//
// Deprecated: prefer VehicleLeaseStore / TrainLeaseStore.
type Leases[T domain.Lease, K ~string] interface {
	ListActive(ctx context.Context, keyIDs []K, now time.Time) ([]T, error)
	Insert(ctx context.Context, row *T) error
}

// leaseRepo is the generic REL-backed Leases implementation. keyColumn
// is the owning foreign key the rows are filtered on (vehicle_id or
// train_id).
type leaseRepo[T domain.Lease, K ~string] struct {
	repo      rel.Repository
	keyColumn string
}

// NewVehicleLeases returns the vehicle lease repository.
func NewVehicleLeases(r rel.Repository) VehicleLeaseStore {
	return &leaseRepo[domain.VehicleLease, domain.VehicleID]{repo: r, keyColumn: "vehicle_id"}
}

// NewTrainLeases returns the train lease repository.
func NewTrainLeases(r rel.Repository) TrainLeaseStore {
	return &leaseRepo[domain.TrainLease, domain.TrainID]{repo: r, keyColumn: "train_id"}
}

func (l *leaseRepo[T, K]) ListActive(ctx context.Context, keyIDs []K, now time.Time) ([]T, error) {
	if len(keyIDs) == 0 {
		return nil, nil
	}
	vals := make([]interface{}, len(keyIDs))
	for i, id := range keyIDs {
		vals[i] = id
	}
	var rows []T
	if err := l.repo.FindAll(ctx, &rows, where.In(l.keyColumn, vals...)); err != nil {
		return nil, err
	}
	active := make([]T, 0, len(rows))
	for _, row := range rows {
		if row.IsActive(now) {
			active = append(active, row)
		}
	}
	return active, nil
}

func (l *leaseRepo[T, K]) Insert(ctx context.Context, row *T) error {
	return l.repo.Insert(ctx, row)
}

func (l *leaseRepo[T, K]) RequiresJanitor() bool { return true }

func (l *leaseRepo[T, K]) Revoke(ctx context.Context, keyID K, now time.Time) error {
	var rows []T
	if err := l.repo.FindAll(ctx, &rows, where.Eq(l.keyColumn, keyID)); err != nil {
		return err
	}
	for i := range rows {
		if !rows[i].IsActive(now) {
			continue
		}
		switch row := any(&rows[i]).(type) {
		case *domain.VehicleLease:
			row.RevokedAt = &now
			if err := l.repo.Update(ctx, row); err != nil {
				return err
			}
		case *domain.TrainLease:
			row.RevokedAt = &now
			if err := l.repo.Update(ctx, row); err != nil {
				return err
			}
		}
	}
	return nil
}

var (
	_ VehicleLeaseStore = (*leaseRepo[domain.VehicleLease, domain.VehicleID])(nil)
	_ TrainLeaseStore   = (*leaseRepo[domain.TrainLease, domain.TrainID])(nil)
)
