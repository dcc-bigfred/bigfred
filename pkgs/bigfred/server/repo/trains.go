package repo

import (
	"context"
	"errors"

	"github.com/go-rel/rel"
	"github.com/go-rel/rel/sort"
	"github.com/go-rel/rel/where"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// ErrTrainNotFound is returned when no train row matches.
var ErrTrainNotFound = errors.New("train not found")

// Trains is the persistence adapter for domain.Train.
type Trains struct {
	repo rel.Repository
}

// NewTrains returns a Trains repository.
func NewTrains(r rel.Repository) *Trains { return &Trains{repo: r} }

// FindByID looks up a train by primary key.
func (t *Trains) FindByID(ctx context.Context, id domain.TrainID) (domain.Train, error) {
	var row domain.Train
	err := t.repo.Find(ctx, &row, where.Eq("id", id))
	if err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return domain.Train{}, ErrTrainNotFound
		}
		return domain.Train{}, err
	}
	return row, nil
}

// FindByOwnerAndName helps the service layer reject duplicate train
// names per-owner without a unique-violation SQL error.
func (t *Trains) FindByOwnerAndName(ctx context.Context, ownerID uint, name string) (domain.Train, error) {
	var row domain.Train
	err := t.repo.Find(ctx, &row,
		where.Eq("owner_user_id", ownerID),
		where.Eq("name", name),
	)
	if err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return domain.Train{}, ErrTrainNotFound
		}
		return domain.Train{}, err
	}
	return row, nil
}

// CountByOwner returns how many trains are owned by the user. Used
// by the user-deletion guard so an admin cannot delete a driver that
// still has trains in the catalogue.
func (t *Trains) CountByOwner(ctx context.Context, ownerID uint) (int, error) {
	return t.repo.Count(ctx, "trains", where.Eq("owner_user_id", ownerID))
}

// ListByOwner returns every train owned by the user.
func (t *Trains) ListByOwner(ctx context.Context, ownerID uint) ([]domain.Train, error) {
	var rows []domain.Train
	err := t.repo.FindAll(ctx, &rows,
		where.Eq("owner_user_id", ownerID),
		sort.Asc("name"),
	)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ListByIDs returns trains by primary-key set.
func (t *Trains) ListByIDs(ctx context.Context, ids []domain.TrainID) ([]domain.Train, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	vals := make([]interface{}, len(ids))
	for i, id := range ids {
		vals[i] = id
	}
	var rows []domain.Train
	err := t.repo.FindAll(ctx, &rows,
		where.In("id", vals...),
		sort.Asc("name"),
	)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// Insert persists a new train row.
func (t *Trains) Insert(ctx context.Context, row *domain.Train) error {
	return t.repo.Insert(ctx, row)
}

// Update writes an existing train back.
func (t *Trains) Update(ctx context.Context, row *domain.Train) error {
	return t.repo.Update(ctx, row)
}

// Delete removes a train row.
func (t *Trains) Delete(ctx context.Context, row *domain.Train) error {
	return t.repo.Delete(ctx, row)
}

// TrainMembers is the persistence adapter for domain.TrainMember.
type TrainMembers struct {
	repo rel.Repository
}

// NewTrainMembers returns a TrainMembers repository.
func NewTrainMembers(r rel.Repository) *TrainMembers { return &TrainMembers{repo: r} }

// ListByTrain returns every member of the train, ordered by Position
// (so the throttle UI renders the consist top-to-bottom in the right
// sequence).
func (m *TrainMembers) ListByTrain(ctx context.Context, trainID domain.TrainID) ([]domain.TrainMember, error) {
	var rows []domain.TrainMember
	err := m.repo.FindAll(ctx, &rows,
		where.Eq("train_id", trainID),
		sort.Asc("position"),
	)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ListByVehicle returns every train_members row referencing the
// vehicle. Used to refresh layout train snapshots in Redis when a
// member's DCC address changes outside the layout vehicle roster.
func (m *TrainMembers) ListByVehicle(ctx context.Context, vehicleID domain.VehicleID) ([]domain.TrainMember, error) {
	var rows []domain.TrainMember
	err := m.repo.FindAll(ctx, &rows, where.Eq("vehicle_id", vehicleID))
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ErrTrainMemberNotFound is returned when no train_members row matches.
var ErrTrainMemberNotFound = errors.New("train member not found")

// FindByID looks up one train member row.
func (m *TrainMembers) FindByID(ctx context.Context, id uint) (domain.TrainMember, error) {
	var row domain.TrainMember
	err := m.repo.Find(ctx, &row, where.Eq("id", id))
	if err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return domain.TrainMember{}, ErrTrainMemberNotFound
		}
		return domain.TrainMember{}, err
	}
	return row, nil
}

// Update persists changes to an existing member row.
func (m *TrainMembers) Update(ctx context.Context, row *domain.TrainMember) error {
	return m.repo.Update(ctx, row)
}

// CountReferencingVehicle is used by VehicleService.Delete to refuse
// deleting a vehicle that is still part of any train.
func (m *TrainMembers) CountReferencingVehicle(ctx context.Context, vehicleID domain.VehicleID) (int, error) {
	return m.repo.Count(ctx, "train_members", where.Eq("vehicle_id", vehicleID))
}

// Insert persists a new member row.
func (m *TrainMembers) Insert(ctx context.Context, row *domain.TrainMember) error {
	return m.repo.Insert(ctx, row)
}

// DeleteAllForTrain removes every member of a train (used when
// replacing the entire member list and on train deletion).
func (m *TrainMembers) DeleteAllForTrain(ctx context.Context, trainID domain.TrainID) error {
	var rows []domain.TrainMember
	if err := m.repo.FindAll(ctx, &rows, where.Eq("train_id", trainID)); err != nil {
		return err
	}
	for i := range rows {
		if err := m.repo.Delete(ctx, &rows[i]); err != nil {
			return err
		}
	}
	return nil
}
