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
type Leases[T domain.Lease] interface {
	// ListActive returns every active (non-revoked, unexpired) lease
	// for the supplied owning ids at `now`. An empty id slice yields
	// nil without hitting the DB.
	ListActive(ctx context.Context, keyIDs []uint, now time.Time) ([]T, error)
	// Insert persists a new lease row. Timestamps are caller-owned.
	Insert(ctx context.Context, row *T) error
}

// leaseRepo is the generic REL-backed Leases implementation. keyColumn
// is the owning foreign key the rows are filtered on (vehicle_id or
// train_id).
type leaseRepo[T domain.Lease] struct {
	repo      rel.Repository
	keyColumn string
}

// NewVehicleLeases returns the vehicle lease repository.
func NewVehicleLeases(r rel.Repository) Leases[domain.VehicleLease] {
	return &leaseRepo[domain.VehicleLease]{repo: r, keyColumn: "vehicle_id"}
}

// NewTrainLeases returns the train lease repository.
func NewTrainLeases(r rel.Repository) Leases[domain.TrainLease] {
	return &leaseRepo[domain.TrainLease]{repo: r, keyColumn: "train_id"}
}

func (l *leaseRepo[T]) ListActive(ctx context.Context, keyIDs []uint, now time.Time) ([]T, error) {
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

func (l *leaseRepo[T]) Insert(ctx context.Context, row *T) error {
	return l.repo.Insert(ctx, row)
}
