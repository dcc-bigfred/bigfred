package repo

import (
	"context"
	"errors"

	"github.com/go-rel/rel"
	"github.com/go-rel/rel/sort"
	"github.com/go-rel/rel/where"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// ErrLayoutVehicleNotFound is returned when no roster row matches.
var ErrLayoutVehicleNotFound = errors.New("layout vehicle not found")

// ErrLayoutTrainNotFound is the train-shaped sibling sentinel.
var ErrLayoutTrainNotFound = errors.New("layout train not found")

// LayoutVehicles is the persistence adapter for the layout ↔ vehicle
// roster join table.
type LayoutVehicles struct {
	repo rel.Repository
}

// NewLayoutVehicles returns a LayoutVehicles repository.
func NewLayoutVehicles(r rel.Repository) *LayoutVehicles {
	return &LayoutVehicles{repo: r}
}

// ListByLayout returns every roster row for a layout, ordered by
// AddedAt so the dashboard table feels like a chronological log.
func (l *LayoutVehicles) ListByLayout(ctx context.Context, layoutID uint) ([]domain.LayoutVehicle, error) {
	var rows []domain.LayoutVehicle
	err := l.repo.FindAll(ctx, &rows,
		where.Eq("layout_id", layoutID),
		sort.Asc("added_at"),
	)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ListByVehicle returns every roster row that references the
// vehicle. Used by the catalogue-side mutation hooks to compute the
// set of layouts that need a fan-out broadcast.
func (l *LayoutVehicles) ListByVehicle(ctx context.Context, vehicleID uint) ([]domain.LayoutVehicle, error) {
	var rows []domain.LayoutVehicle
	err := l.repo.FindAll(ctx, &rows, where.Eq("vehicle_id", vehicleID))
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// FindByLayoutAndVehicle returns the join row for one (layout,
// vehicle) pair.
func (l *LayoutVehicles) FindByLayoutAndVehicle(ctx context.Context, layoutID, vehicleID uint) (domain.LayoutVehicle, error) {
	var row domain.LayoutVehicle
	err := l.repo.Find(ctx, &row,
		where.Eq("layout_id", layoutID),
		where.Eq("vehicle_id", vehicleID),
	)
	if err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return domain.LayoutVehicle{}, ErrLayoutVehicleNotFound
		}
		return domain.LayoutVehicle{}, err
	}
	return row, nil
}

// Insert persists a new roster row.
func (l *LayoutVehicles) Insert(ctx context.Context, row *domain.LayoutVehicle) error {
	return l.repo.Insert(ctx, row)
}

// Delete removes a single roster row.
func (l *LayoutVehicles) Delete(ctx context.Context, row *domain.LayoutVehicle) error {
	return l.repo.Delete(ctx, row)
}

// DeleteAllForVehicle removes every roster row pointing at a
// vehicle (used when the catalogue row itself is deleted).
func (l *LayoutVehicles) DeleteAllForVehicle(ctx context.Context, vehicleID uint) error {
	var rows []domain.LayoutVehicle
	if err := l.repo.FindAll(ctx, &rows, where.Eq("vehicle_id", vehicleID)); err != nil {
		return err
	}
	for i := range rows {
		if err := l.repo.Delete(ctx, &rows[i]); err != nil {
			return err
		}
	}
	return nil
}

// LayoutTrains is the persistence adapter for the layout ↔ train
// roster join table.
type LayoutTrains struct {
	repo rel.Repository
}

// NewLayoutTrains returns a LayoutTrains repository.
func NewLayoutTrains(r rel.Repository) *LayoutTrains {
	return &LayoutTrains{repo: r}
}

// ListByLayout returns every roster row for a layout.
func (l *LayoutTrains) ListByLayout(ctx context.Context, layoutID uint) ([]domain.LayoutTrain, error) {
	var rows []domain.LayoutTrain
	err := l.repo.FindAll(ctx, &rows,
		where.Eq("layout_id", layoutID),
		sort.Asc("added_at"),
	)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ListByTrain returns every roster row referencing the train. Used
// by the catalogue-side mutation hooks for fan-out broadcasts.
func (l *LayoutTrains) ListByTrain(ctx context.Context, trainID uint) ([]domain.LayoutTrain, error) {
	var rows []domain.LayoutTrain
	err := l.repo.FindAll(ctx, &rows, where.Eq("train_id", trainID))
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// FindByLayoutAndTrain returns the join row for one (layout, train)
// pair.
func (l *LayoutTrains) FindByLayoutAndTrain(ctx context.Context, layoutID, trainID uint) (domain.LayoutTrain, error) {
	var row domain.LayoutTrain
	err := l.repo.Find(ctx, &row,
		where.Eq("layout_id", layoutID),
		where.Eq("train_id", trainID),
	)
	if err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return domain.LayoutTrain{}, ErrLayoutTrainNotFound
		}
		return domain.LayoutTrain{}, err
	}
	return row, nil
}

// Insert persists a new roster row.
func (l *LayoutTrains) Insert(ctx context.Context, row *domain.LayoutTrain) error {
	return l.repo.Insert(ctx, row)
}

// Delete removes a single roster row.
func (l *LayoutTrains) Delete(ctx context.Context, row *domain.LayoutTrain) error {
	return l.repo.Delete(ctx, row)
}

// DeleteAllForTrain removes every roster row pointing at a train
// (used when the train itself is deleted).
func (l *LayoutTrains) DeleteAllForTrain(ctx context.Context, trainID uint) error {
	var rows []domain.LayoutTrain
	if err := l.repo.FindAll(ctx, &rows, where.Eq("train_id", trainID)); err != nil {
		return err
	}
	for i := range rows {
		if err := l.repo.Delete(ctx, &rows[i]); err != nil {
			return err
		}
	}
	return nil
}
