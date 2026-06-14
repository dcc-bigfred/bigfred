package repo

import (
	"context"
	"errors"

	"github.com/go-rel/rel"
	"github.com/go-rel/rel/sort"
	"github.com/go-rel/rel/where"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// ErrDccFunctionNotFound is returned when no function row matches.
var ErrDccFunctionNotFound = errors.New("dcc function not found")

// DccFunctions persists domain.DccFunction rows.
type DccFunctions struct {
	repo rel.Repository
}

// NewDccFunctions returns a DccFunctions repository.
func NewDccFunctions(r rel.Repository) *DccFunctions { return &DccFunctions{repo: r} }

// ListByVehicleID returns rows owned by a vehicle, sorted by position.
func (f *DccFunctions) ListByVehicleID(ctx context.Context, vehicleID uint) ([]domain.DccFunction, error) {
	var rows []domain.DccFunction
	err := f.repo.FindAll(ctx, &rows,
		where.Eq("vehicle_id", vehicleID),
		sort.Asc("position"),
		sort.Asc("num"),
	)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ListByTemplateID returns rows owned by a template, sorted by position.
func (f *DccFunctions) ListByTemplateID(ctx context.Context, templateID uint) ([]domain.DccFunction, error) {
	var rows []domain.DccFunction
	err := f.repo.FindAll(ctx, &rows,
		where.Eq("template_id", templateID),
		sort.Asc("position"),
		sort.Asc("num"),
	)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// FindByVehicleAndNum looks up one vehicle-owned slot.
func (f *DccFunctions) FindByVehicleAndNum(ctx context.Context, vehicleID uint, num uint8) (domain.DccFunction, error) {
	var row domain.DccFunction
	err := f.repo.Find(ctx, &row,
		where.Eq("vehicle_id", vehicleID).AndEq("num", num),
	)
	if err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return domain.DccFunction{}, ErrDccFunctionNotFound
		}
		return domain.DccFunction{}, err
	}
	return row, nil
}

// FindByTemplateAndNum looks up one template-owned slot.
func (f *DccFunctions) FindByTemplateAndNum(ctx context.Context, templateID uint, num uint8) (domain.DccFunction, error) {
	var row domain.DccFunction
	err := f.repo.Find(ctx, &row,
		where.Eq("template_id", templateID).AndEq("num", num),
	)
	if err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return domain.DccFunction{}, ErrDccFunctionNotFound
		}
		return domain.DccFunction{}, err
	}
	return row, nil
}

// Insert persists a new function row.
func (f *DccFunctions) Insert(ctx context.Context, row *domain.DccFunction) error {
	return f.repo.Insert(ctx, row)
}

// Update writes an existing function row.
func (f *DccFunctions) Update(ctx context.Context, row *domain.DccFunction) error {
	return f.repo.Update(ctx, row)
}

// Delete removes a function row.
func (f *DccFunctions) Delete(ctx context.Context, row *domain.DccFunction) error {
	return f.repo.Delete(ctx, row)
}

// DeleteAllByVehicleID removes every function row for a vehicle.
func (f *DccFunctions) DeleteAllByVehicleID(ctx context.Context, vehicleID uint) error {
	rows, err := f.ListByVehicleID(ctx, vehicleID)
	if err != nil {
		return err
	}
	for i := range rows {
		if err := f.Delete(ctx, &rows[i]); err != nil {
			return err
		}
	}
	return nil
}

// Transaction runs fn inside a REL transaction.
func (f *DccFunctions) Transaction(ctx context.Context, fn func(context.Context) error) error {
	return f.repo.Transaction(ctx, fn)
}
