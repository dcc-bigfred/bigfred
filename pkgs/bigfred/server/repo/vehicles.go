package repo

import (
	"context"
	"errors"

	"github.com/go-rel/rel"
	"github.com/go-rel/rel/sort"
	"github.com/go-rel/rel/where"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// ErrVehicleNotFound is returned when no vehicle row matches.
var ErrVehicleNotFound = errors.New("vehicle not found")

// Vehicles is the persistence adapter for domain.Vehicle.
type Vehicles struct {
	repo rel.Repository
}

// NewVehicles returns a Vehicles repository.
func NewVehicles(r rel.Repository) *Vehicles { return &Vehicles{repo: r} }

// FindByID looks up a vehicle by primary key.
func (v *Vehicles) FindByID(ctx context.Context, id domain.VehicleID) (domain.Vehicle, error) {
	var row domain.Vehicle
	err := v.repo.Find(ctx, &row, where.Eq("id", id))
	if err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return domain.Vehicle{}, ErrVehicleNotFound
		}
		return domain.Vehicle{}, err
	}
	return row, nil
}

// FindByDCCAddress looks up a vehicle by its (unique) DCC address.
// Dummies (DCC = NULL) cannot be located via this helper.
func (v *Vehicles) FindByDCCAddress(ctx context.Context, addr uint16) (domain.Vehicle, error) {
	var row domain.Vehicle
	err := v.repo.Find(ctx, &row, where.Eq("dcc_address", addr))
	if err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return domain.Vehicle{}, ErrVehicleNotFound
		}
		return domain.Vehicle{}, err
	}
	return row, nil
}

// FindByExternalID looks up a vehicle by its globally-unique external id
// (assigned by an integrating client such as the Android catalogue).
// Rows created inside BigFred (ExternalID == NULL) are not locatable here.
func (v *Vehicles) FindByExternalID(ctx context.Context, externalID string) (domain.Vehicle, error) {
	var row domain.Vehicle
	err := v.repo.Find(ctx, &row, where.Eq("external_id", externalID))
	if err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return domain.Vehicle{}, ErrVehicleNotFound
		}
		return domain.Vehicle{}, err
	}
	return row, nil
}

// CountByOwner returns how many vehicles are owned by the user. Used
// by the user-deletion guard so an admin cannot delete a driver that
// still has vehicles in the catalogue.
func (v *Vehicles) CountByOwner(ctx context.Context, ownerID uint) (int, error) {
	return v.repo.Count(ctx, "vehicles", where.Eq("owner_user_id", ownerID))
}

// ListAll returns every vehicle in the catalogue (all owners).
func (v *Vehicles) ListAll(ctx context.Context) ([]domain.Vehicle, error) {
	var rows []domain.Vehicle
	err := v.repo.FindAll(ctx, &rows, sort.Asc("name"))
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ListByOwner returns every vehicle owned by the user.
func (v *Vehicles) ListByOwner(ctx context.Context, ownerID uint) ([]domain.Vehicle, error) {
	var rows []domain.Vehicle
	err := v.repo.FindAll(ctx, &rows,
		where.Eq("owner_user_id", ownerID),
		sort.Asc("name"),
	)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ListByIDs returns vehicles by primary-key set.
func (v *Vehicles) ListByIDs(ctx context.Context, ids []domain.VehicleID) ([]domain.Vehicle, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	vals := make([]interface{}, len(ids))
	for i, id := range ids {
		vals[i] = id
	}
	var rows []domain.Vehicle
	err := v.repo.FindAll(ctx, &rows,
		where.In("id", vals...),
		sort.Asc("name"),
	)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ListByTemplateID returns every vehicle linked to a function template.
func (v *Vehicles) ListByTemplateID(ctx context.Context, templateID uint) ([]domain.Vehicle, error) {
	var rows []domain.Vehicle
	err := v.repo.FindAll(ctx, &rows,
		where.Eq("template_id", templateID),
		sort.Asc("name"),
	)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// Insert persists a new vehicle.
func (v *Vehicles) Insert(ctx context.Context, row *domain.Vehicle) error {
	return v.repo.Insert(ctx, row)
}

// Update writes an existing vehicle back.
func (v *Vehicles) Update(ctx context.Context, row *domain.Vehicle) error {
	return v.repo.Update(ctx, row)
}

// Delete removes a vehicle row.
func (v *Vehicles) Delete(ctx context.Context, row *domain.Vehicle) error {
	return v.repo.Delete(ctx, row)
}
