package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/security"
)

// Train sentinel errors.
var (
	ErrTrainNotFound      = errors.New("train_not_found")
	ErrTrainNameRequired  = errors.New("train_name_required")
	ErrTrainNameTaken     = errors.New("train_name_taken")
	ErrTrainNoMembers     = errors.New("train_no_members")
	ErrTrainMemberNotOwned = errors.New("train_member_not_owned")
	ErrTrainMemberMissing  = errors.New("train_member_missing")
	ErrTrainNotOwned       = errors.New("train_not_owned")
)

const maxTrainNameLen = 64

// TrainMemberInput is the validated payload of one TrainMember row.
type TrainMemberInput struct {
	VehicleID uint
	Reversed  bool
}

// TrainCreateInput is the validated payload of TrainService.Create.
type TrainCreateInput struct {
	OwnerUserID uint
	Name        string
	Members     []TrainMemberInput
}

// TrainUpdateInput is the validated payload of TrainService.Update.
// Members is a tri-state via a pointer to a slice: nil leaves the
// member list untouched, an empty slice would fail validation.
type TrainUpdateInput struct {
	Name    *string
	Members *[]TrainMemberInput
}

// TrainDetail bundles a Train row with its ordered member list.
type TrainDetail struct {
	Train   domain.Train
	Members []domain.TrainMember
}

// TrainService implements the CRUD lifecycle for domain.Train.
// Composition rule: every member vehicle must be OWNED by the caller
// (goal 7 — leasing transfers driving authority, not edit rights, so
// a leased vehicle cannot live inside someone else's train).
type TrainService struct {
	trains   *repo.Trains
	members  *repo.TrainMembers
	vehicles *repo.Vehicles
	sec      security.TrainSecurityContext
}

// NewTrainService constructs a TrainService.
func NewTrainService(t *repo.Trains, m *repo.TrainMembers, v *repo.Vehicles) *TrainService {
	return &TrainService{trains: t, members: m, vehicles: v}
}

// ListOwned returns every train owned by the user with member rows
// hydrated so the dashboard can render "X (3 vehicles)".
func (s *TrainService) ListOwned(ctx context.Context, ownerID uint) ([]TrainDetail, error) {
	trains, err := s.trains.ListByOwner(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	out := make([]TrainDetail, 0, len(trains))
	for _, t := range trains {
		members, err := s.members.ListByTrain(ctx, t.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, TrainDetail{Train: t, Members: members})
	}
	return out, nil
}

// Get loads a train with its members.
func (s *TrainService) Get(ctx context.Context, id uint) (TrainDetail, error) {
	t, err := s.trains.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrTrainNotFound) {
			return TrainDetail{}, ErrTrainNotFound
		}
		return TrainDetail{}, err
	}
	members, err := s.members.ListByTrain(ctx, t.ID)
	if err != nil {
		return TrainDetail{}, err
	}
	return TrainDetail{Train: t, Members: members}, nil
}

// ListByIDsForLayout is a helper for layout-roster enrichment: bulk
// load + members in one shot, ordered by name.
func (s *TrainService) ListByIDsForLayout(ctx context.Context, ids []uint) ([]TrainDetail, error) {
	trains, err := s.trains.ListByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]TrainDetail, 0, len(trains))
	for _, t := range trains {
		members, err := s.members.ListByTrain(ctx, t.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, TrainDetail{Train: t, Members: members})
	}
	return out, nil
}

// Create inserts a new train with its member rows.
func (s *TrainService) Create(ctx context.Context, in TrainCreateInput) (TrainDetail, error) {
	name, err := sanitiseTrainName(in.Name)
	if err != nil {
		return TrainDetail{}, err
	}
	if _, err := s.trains.FindByOwnerAndName(ctx, in.OwnerUserID, name); err == nil {
		return TrainDetail{}, ErrTrainNameTaken
	} else if !errors.Is(err, repo.ErrTrainNotFound) {
		return TrainDetail{}, err
	}
	if len(in.Members) == 0 {
		return TrainDetail{}, ErrTrainNoMembers
	}
	if err := s.validateMembers(ctx, in.OwnerUserID, in.Members); err != nil {
		return TrainDetail{}, err
	}

	now := time.Now().UTC()
	row := domain.Train{
		OwnerUserID: in.OwnerUserID,
		Name:        name,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.trains.Insert(ctx, &row); err != nil {
		return TrainDetail{}, err
	}
	if err := s.replaceMembers(ctx, row.ID, in.Members); err != nil {
		return TrainDetail{}, err
	}

	return s.Get(ctx, row.ID)
}

// Update renames and/or replaces the member list of an existing
// train. Authority is decided by TrainSecurityContext.CanMutateTrain
// (§7a.3).
func (s *TrainService) Update(ctx context.Context, actorID, trainID uint, eff domain.EffectiveRoles, in TrainUpdateInput) (TrainDetail, error) {
	t, err := s.trains.FindByID(ctx, trainID)
	if err != nil {
		if errors.Is(err, repo.ErrTrainNotFound) {
			return TrainDetail{}, ErrTrainNotFound
		}
		return TrainDetail{}, err
	}
	if err := s.checkTrainMutate(eff, actorID, t.OwnerUserID); err != nil {
		return TrainDetail{}, err
	}

	if in.Name != nil {
		name, err := sanitiseTrainName(*in.Name)
		if err != nil {
			return TrainDetail{}, err
		}
		if name != t.Name {
			if other, err := s.trains.FindByOwnerAndName(ctx, t.OwnerUserID, name); err == nil {
				if other.ID != t.ID {
					return TrainDetail{}, ErrTrainNameTaken
				}
			} else if !errors.Is(err, repo.ErrTrainNotFound) {
				return TrainDetail{}, err
			}
			t.Name = name
		}
	}

	if in.Members != nil {
		if len(*in.Members) == 0 {
			return TrainDetail{}, ErrTrainNoMembers
		}
		if err := s.validateMembers(ctx, t.OwnerUserID, *in.Members); err != nil {
			return TrainDetail{}, err
		}
		if err := s.replaceMembers(ctx, t.ID, *in.Members); err != nil {
			return TrainDetail{}, err
		}
	}

	t.UpdatedAt = time.Now().UTC()
	if err := s.trains.Update(ctx, &t); err != nil {
		return TrainDetail{}, err
	}
	return s.Get(ctx, t.ID)
}

// Delete removes a train and its member rows. Authority is decided
// by TrainSecurityContext.CanMutateTrain (§7a.3).
func (s *TrainService) Delete(ctx context.Context, actorID, trainID uint, eff domain.EffectiveRoles) (domain.Train, error) {
	t, err := s.trains.FindByID(ctx, trainID)
	if err != nil {
		if errors.Is(err, repo.ErrTrainNotFound) {
			return domain.Train{}, ErrTrainNotFound
		}
		return domain.Train{}, err
	}
	if err := s.checkTrainMutate(eff, actorID, t.OwnerUserID); err != nil {
		return domain.Train{}, err
	}
	if err := s.members.DeleteAllForTrain(ctx, t.ID); err != nil {
		return domain.Train{}, err
	}
	if err := s.trains.Delete(ctx, &t); err != nil {
		return domain.Train{}, err
	}
	return t, nil
}

func (s *TrainService) checkTrainMutate(eff domain.EffectiveRoles, actorID, ownerUserID uint) error {
	decision := s.sec.CanMutateTrain(eff, actorID, ownerUserID)
	if decision.Allowed {
		return nil
	}
	switch decision.Reason {
	case "train_not_owned":
		return ErrTrainNotOwned
	default:
		return errors.New(decision.Reason)
	}
}

// validateMembers walks the candidate member list and confirms each
// vehicle exists and is owned by the actor. Duplicate vehicle ids
// in the input are rejected with ErrTrainMemberMissing (the unique
// index would also catch it, but a service-side message is friendlier).
func (s *TrainService) validateMembers(ctx context.Context, ownerID uint, members []TrainMemberInput) error {
	seen := make(map[uint]struct{}, len(members))
	for _, m := range members {
		if m.VehicleID == 0 {
			return ErrTrainMemberMissing
		}
		if _, dup := seen[m.VehicleID]; dup {
			return ErrTrainMemberMissing
		}
		seen[m.VehicleID] = struct{}{}

		row, err := s.vehicles.FindByID(ctx, m.VehicleID)
		if err != nil {
			if errors.Is(err, repo.ErrVehicleNotFound) {
				return ErrTrainMemberMissing
			}
			return err
		}
		if row.OwnerUserID != ownerID {
			return ErrTrainMemberNotOwned
		}
	}
	return nil
}

// replaceMembers swaps the train_members rows in one shot. Position
// is assigned from the input order so the caller controls render
// sequence by sorting the slice before sending it.
func (s *TrainService) replaceMembers(ctx context.Context, trainID uint, members []TrainMemberInput) error {
	if err := s.members.DeleteAllForTrain(ctx, trainID); err != nil {
		return err
	}
	for i, m := range members {
		row := domain.TrainMember{
			TrainID:   trainID,
			VehicleID: m.VehicleID,
			Position:  i,
			Reversed:  m.Reversed,
		}
		if err := s.members.Insert(ctx, &row); err != nil {
			return err
		}
	}
	return nil
}

func sanitiseTrainName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", ErrTrainNameRequired
	}
	if len(name) > maxTrainNameLen {
		name = name[:maxTrainNameLen]
	}
	return name, nil
}
