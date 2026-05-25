package repo

import (
	"context"
	"errors"

	"github.com/go-rel/rel"
	"github.com/go-rel/rel/sort"
	"github.com/go-rel/rel/where"

	"github.com/keskad/loco/pkgs/server/domain"
)

// ErrLayoutNotFound is returned by Layouts.FindByID / FindByName when
// no matching row exists. Higher layers translate it to a 404 / 422 as
// appropriate.
var ErrLayoutNotFound = errors.New("layout not found")

// Layouts is the persistence adapter for domain.Layout. Construct it
// with NewLayouts; the zero value is unusable.
type Layouts struct {
	repo rel.Repository
}

// NewLayouts returns a Layouts repository bound to the given REL
// instance.
func NewLayouts(r rel.Repository) *Layouts { return &Layouts{repo: r} }

// FindByID looks up a layout by primary key.
func (l *Layouts) FindByID(ctx context.Context, id uint) (domain.Layout, error) {
	var layout domain.Layout
	err := l.repo.Find(ctx, &layout, where.Eq("id", id))
	if err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return domain.Layout{}, ErrLayoutNotFound
		}
		return domain.Layout{}, err
	}
	return layout, nil
}

// FindByName looks up a layout by its case-sensitive Name. Used by
// the seeder to test idempotency and by service-level "name already
// taken" guards.
func (l *Layouts) FindByName(ctx context.Context, name string) (domain.Layout, error) {
	var layout domain.Layout
	err := l.repo.Find(ctx, &layout, where.Eq("name", name))
	if err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return domain.Layout{}, ErrLayoutNotFound
		}
		return domain.Layout{}, err
	}
	return layout, nil
}

// FindSystem returns the single system layout row (IsSystem = true).
// The partial unique index installed by the migration guarantees the
// row is unique, so this never returns more than one.
func (l *Layouts) FindSystem(ctx context.Context) (domain.Layout, error) {
	var layout domain.Layout
	err := l.repo.Find(ctx, &layout, where.Eq("is_system", true))
	if err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return domain.Layout{}, ErrLayoutNotFound
		}
		return domain.Layout{}, err
	}
	return layout, nil
}

// ListAll returns every layout, ordered with the system row first
// (so it stays anchored in admin tables) and the rest alphabetically.
func (l *Layouts) ListAll(ctx context.Context) ([]domain.Layout, error) {
	var layouts []domain.Layout
	err := l.repo.FindAll(ctx, &layouts,
		sort.Desc("is_system"),
		sort.Asc("name"),
	)
	if err != nil {
		return nil, err
	}
	return layouts, nil
}

// ListSelectable returns rows the login dropdown should offer: every
// non-locked layout. The system layout is always present (it cannot
// be locked, enforced by the DB CHECK + service rule).
func (l *Layouts) ListSelectable(ctx context.Context) ([]domain.Layout, error) {
	var layouts []domain.Layout
	err := l.repo.FindAll(ctx, &layouts,
		where.Eq("locked", false),
		sort.Desc("is_system"),
		sort.Asc("name"),
	)
	if err != nil {
		return nil, err
	}
	return layouts, nil
}

// Insert persists a new layout. Callers MUST set CreatedAt/UpdatedAt
// and CreatedBy beforehand; this layer never invents timestamps.
func (l *Layouts) Insert(ctx context.Context, layout *domain.Layout) error {
	return l.repo.Insert(ctx, layout)
}

// Update writes an existing layout back to the database. The caller
// is responsible for bumping UpdatedAt.
func (l *Layouts) Update(ctx context.Context, layout *domain.Layout) error {
	return l.repo.Update(ctx, layout)
}

// Delete removes a layout row. Callers are expected to have already
// rejected attempts to delete the system row (§3a.1).
func (l *Layouts) Delete(ctx context.Context, layout *domain.Layout) error {
	return l.repo.Delete(ctx, layout)
}
