package repo

import (
	"context"
	"errors"

	"github.com/go-rel/rel"
	"github.com/go-rel/rel/sort"
	"github.com/go-rel/rel/where"

	"github.com/keskad/loco/pkgs/server/domain"
)

// ErrCommandStationNotFound is returned by CommandStations.FindByID /
// FindByName when no matching row exists.
var ErrCommandStationNotFound = errors.New("command station not found")

// CommandStations is the persistence adapter for domain.CommandStation.
// Construct with NewCommandStations; the zero value is unusable.
type CommandStations struct {
	repo rel.Repository
}

// NewCommandStations returns a CommandStations repository bound to r.
func NewCommandStations(r rel.Repository) *CommandStations {
	return &CommandStations{repo: r}
}

// FindByID looks up a command station by primary key.
func (s *CommandStations) FindByID(ctx context.Context, id uint) (domain.CommandStation, error) {
	var row domain.CommandStation
	err := s.repo.Find(ctx, &row, where.Eq("id", id))
	if err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return domain.CommandStation{}, ErrCommandStationNotFound
		}
		return domain.CommandStation{}, err
	}
	return row, nil
}

// FindByName looks up a command station by its unique name.
func (s *CommandStations) FindByName(ctx context.Context, name string) (domain.CommandStation, error) {
	var row domain.CommandStation
	err := s.repo.Find(ctx, &row, where.Eq("name", name))
	if err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return domain.CommandStation{}, ErrCommandStationNotFound
		}
		return domain.CommandStation{}, err
	}
	return row, nil
}

// ListAll returns every command station ordered by name.
func (s *CommandStations) ListAll(ctx context.Context) ([]domain.CommandStation, error) {
	var rows []domain.CommandStation
	err := s.repo.FindAll(ctx, &rows, sort.Asc("name"))
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ListByIDs returns command stations whose ids appear in the slice.
// An empty slice yields an empty result without hitting the DB.
func (s *CommandStations) ListByIDs(ctx context.Context, ids []uint) ([]domain.CommandStation, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	vals := make([]interface{}, len(ids))
	for i, id := range ids {
		vals[i] = id
	}
	var rows []domain.CommandStation
	err := s.repo.FindAll(ctx, &rows,
		where.In("id", vals...),
		sort.Asc("name"),
	)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// Insert persists a new row. Timestamps are caller-owned.
func (s *CommandStations) Insert(ctx context.Context, row *domain.CommandStation) error {
	return s.repo.Insert(ctx, row)
}

// Update writes an existing row back.
func (s *CommandStations) Update(ctx context.Context, row *domain.CommandStation) error {
	return s.repo.Update(ctx, row)
}

// Delete removes a row.
func (s *CommandStations) Delete(ctx context.Context, row *domain.CommandStation) error {
	return s.repo.Delete(ctx, row)
}
