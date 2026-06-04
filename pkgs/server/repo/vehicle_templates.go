package repo

import (
	"context"
	"errors"

	"github.com/go-rel/rel"
	"github.com/go-rel/rel/sort"
	"github.com/go-rel/rel/where"

	"github.com/keskad/loco/pkgs/server/domain"
)

// ErrVehicleTemplateNotFound is returned when no template row matches.
var ErrVehicleTemplateNotFound = errors.New("vehicle template not found")

// VehicleTemplates persists domain.VehicleTemplate rows.
type VehicleTemplates struct {
	repo rel.Repository
}

// NewVehicleTemplates returns a VehicleTemplates repository.
func NewVehicleTemplates(r rel.Repository) *VehicleTemplates {
	return &VehicleTemplates{repo: r}
}

// FindByID looks up a template by primary key.
func (t *VehicleTemplates) FindByID(ctx context.Context, id uint) (domain.VehicleTemplate, error) {
	var row domain.VehicleTemplate
	err := t.repo.Find(ctx, &row, where.Eq("id", id))
	if err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return domain.VehicleTemplate{}, ErrVehicleTemplateNotFound
		}
		return domain.VehicleTemplate{}, err
	}
	return row, nil
}

// List returns every template ordered by name.
func (t *VehicleTemplates) List(ctx context.Context) ([]domain.VehicleTemplate, error) {
	var rows []domain.VehicleTemplate
	err := t.repo.FindAll(ctx, &rows, sort.Asc("name"))
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// Insert persists a new template.
func (t *VehicleTemplates) Insert(ctx context.Context, row *domain.VehicleTemplate) error {
	return t.repo.Insert(ctx, row)
}

// Update writes an existing template.
func (t *VehicleTemplates) Update(ctx context.Context, row *domain.VehicleTemplate) error {
	return t.repo.Update(ctx, row)
}

// Delete removes a template row.
func (t *VehicleTemplates) Delete(ctx context.Context, row *domain.VehicleTemplate) error {
	return t.repo.Delete(ctx, row)
}

// CountVehiclesLinked counts vehicles with template_id = id and no detach.
func (t *VehicleTemplates) CountVehiclesLinked(ctx context.Context, templateID uint) (int, error) {
	return t.repo.Count(ctx, "vehicles",
		where.Eq("template_id", templateID).AndNil("functions_detached_at"),
	)
}
