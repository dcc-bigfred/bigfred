package repo

import (
	"context"
	"errors"

	"github.com/go-rel/rel"
	"github.com/go-rel/rel/sort"
	"github.com/go-rel/rel/where"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// ErrInterlockingNotFound is returned when no interlocking row matches.
var ErrInterlockingNotFound = errors.New("interlocking not found")

// Interlockings is the persistence adapter for domain.Interlocking.
type Interlockings struct {
	repo rel.Repository
}

// NewInterlockings returns an Interlockings repository.
func NewInterlockings(r rel.Repository) *Interlockings {
	return &Interlockings{repo: r}
}

// FindByID looks up an interlocking by primary key.
func (i *Interlockings) FindByID(ctx context.Context, id uint) (domain.Interlocking, error) {
	var row domain.Interlocking
	err := i.repo.Find(ctx, &row, where.Eq("id", id))
	if err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return domain.Interlocking{}, ErrInterlockingNotFound
		}
		return domain.Interlocking{}, err
	}
	return row, nil
}

// FindByName looks up an interlocking by its unique name.
func (i *Interlockings) FindByName(ctx context.Context, name string) (domain.Interlocking, error) {
	var row domain.Interlocking
	err := i.repo.Find(ctx, &row, where.Eq("name", name))
	if err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return domain.Interlocking{}, ErrInterlockingNotFound
		}
		return domain.Interlocking{}, err
	}
	return row, nil
}

// ListAll returns every interlocking ordered by name.
func (i *Interlockings) ListAll(ctx context.Context) ([]domain.Interlocking, error) {
	var rows []domain.Interlocking
	err := i.repo.FindAll(ctx, &rows, sort.Asc("name"))
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ListByIDs returns interlockings whose ids appear in the slice.
// An empty slice yields an empty result without hitting the DB.
func (i *Interlockings) ListByIDs(ctx context.Context, ids []uint) ([]domain.Interlocking, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	vals := make([]interface{}, len(ids))
	for j, id := range ids {
		vals[j] = id
	}
	var rows []domain.Interlocking
	err := i.repo.FindAll(ctx, &rows,
		where.In("id", vals...),
		sort.Asc("name"),
	)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// Insert persists a new interlocking.
func (i *Interlockings) Insert(ctx context.Context, row *domain.Interlocking) error {
	return i.repo.Insert(ctx, row)
}

// Update writes an existing interlocking back.
func (i *Interlockings) Update(ctx context.Context, row *domain.Interlocking) error {
	return i.repo.Update(ctx, row)
}

// Delete removes an interlocking row.
func (i *Interlockings) Delete(ctx context.Context, row *domain.Interlocking) error {
	return i.repo.Delete(ctx, row)
}
