package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/helpers"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/security"
	"github.com/keskad/loco/pkgs/bigfred/server/validation"
)

// TrainMemberInput is the validated payload of one TrainMember row.
type TrainMemberInput struct {
	VehicleID        domain.VehicleID
	Reversed         bool
	SpeedMultiplier  float64 // 0 → default 1.0
	ExcludeFromSpeed      bool
	StartDelayMs          int
	AccelRampMs           int
	AccelRampMaxSteps     int
	BrakeRampMs           int
	BrakeRampMaxSteps     int
}

// TrainMemberPatchInput is the validated PATCH payload for one member.
type TrainMemberPatchInput struct {
	SpeedMultiplier       *float64
	ExcludeFromSpeed      *bool
	StartDelayMs          *int
	AccelRampMs           *int
	AccelRampMaxSteps     *int
	BrakeRampMs           *int
	BrakeRampMaxSteps     *int
}

// TrainCreateInput is the validated payload of Train.Create.
type TrainCreateInput struct {
	OwnerUserID uint
	Name        string
	Members     []TrainMemberInput
}

// TrainUpdateInput is the validated payload of Train.Update.
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

// Train implements the CRUD lifecycle for domain.Train (§4.1).
// Every member vehicle must be owned by the caller (goal 7).
type Train struct {
	trains       *repo.Trains
	members      *repo.TrainMembers
	vehicles     *repo.Vehicles
	layoutTrains *repo.LayoutTrains
	users        *repo.Users
	sec          security.TrainSecurityContext
}

// NewTrain constructs a Train use-case handler.
func NewTrain(
	t *repo.Trains,
	m *repo.TrainMembers,
	v *repo.Vehicles,
	layoutTrains *repo.LayoutTrains,
	users *repo.Users,
) *Train {
	return &Train{
		trains:       t,
		members:      m,
		vehicles:     v,
		layoutTrains: layoutTrains,
		users:        users,
	}
}

// ListOwned returns every train owned by the user with member rows hydrated.
func (t *Train) ListOwned(ctx context.Context, ownerID uint) ([]TrainDetail, error) {
	trains, err := t.trains.ListByOwner(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	return t.hydrateDetails(ctx, trains)
}

// TrainCatalogueEntry is one row of the global train catalogue.
type TrainCatalogueEntry struct {
	Train             domain.Train
	Members           []domain.TrainMember
	OwnerLogin        string
	OwnerOrganization string
	OnLayout          bool
}

// ListCatalogue returns every registered train enriched with members,
// owner metadata and whether it is on the given layout roster.
func (t *Train) ListCatalogue(ctx context.Context, layoutID uint) ([]TrainCatalogueEntry, error) {
	trains, err := t.trains.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	onLayout := make(map[domain.TrainID]struct{})
	if t.layoutTrains != nil {
		rows, err := t.layoutTrains.ListByLayout(ctx, layoutID)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			onLayout[row.TrainID] = struct{}{}
		}
	}
	logins := make(map[uint]struct {
		login        string
		organization string
	})
	out := make([]TrainCatalogueEntry, 0, len(trains))
	for _, train := range trains {
		info, ok := logins[train.OwnerUserID]
		if !ok && t.users != nil {
			u, err := t.users.FindByID(ctx, train.OwnerUserID)
			if err != nil {
				info.login = "?"
			} else {
				info.login = u.Login
				info.organization = u.Organization
			}
			logins[train.OwnerUserID] = info
		}
		members, err := t.members.ListByTrain(ctx, train.ID)
		if err != nil {
			return nil, err
		}
		_, on := onLayout[train.ID]
		out = append(out, TrainCatalogueEntry{
			Train:             train,
			Members:           members,
			OwnerLogin:        info.login,
			OwnerOrganization: info.organization,
			OnLayout:          on,
		})
	}
	return out, nil
}

func (t *Train) hydrateDetails(ctx context.Context, trains []domain.Train) ([]TrainDetail, error) {
	out := make([]TrainDetail, 0, len(trains))
	for _, tr := range trains {
		members, err := t.members.ListByTrain(ctx, tr.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, TrainDetail{Train: tr, Members: members})
	}
	return out, nil
}

// Get loads a train with its members.
func (t *Train) Get(ctx context.Context, id domain.TrainID) (TrainDetail, error) {
	tr, err := t.trains.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrTrainNotFound) {
			return TrainDetail{}, svcerrors.ErrTrainNotFound
		}
		return TrainDetail{}, err
	}
	members, err := t.members.ListByTrain(ctx, tr.ID)
	if err != nil {
		return TrainDetail{}, err
	}
	return TrainDetail{Train: tr, Members: members}, nil
}

// ListByIDsForLayout bulk-loads trains with members for layout-roster enrichment.
func (t *Train) ListByIDsForLayout(ctx context.Context, ids []domain.TrainID) ([]TrainDetail, error) {
	trains, err := t.trains.ListByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]TrainDetail, 0, len(trains))
	for _, tr := range trains {
		members, err := t.members.ListByTrain(ctx, tr.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, TrainDetail{Train: tr, Members: members})
	}
	return out, nil
}

// Create inserts a new train with its member rows.
func (t *Train) Create(ctx context.Context, in TrainCreateInput) (TrainDetail, error) {
	name, err := validation.SanitiseTrainName(in.Name)
	if err != nil {
		return TrainDetail{}, err
	}
	if _, err := t.trains.FindByOwnerAndName(ctx, in.OwnerUserID, name); err == nil {
		return TrainDetail{}, svcerrors.ErrTrainNameTaken
	} else if !errors.Is(err, repo.ErrTrainNotFound) {
		return TrainDetail{}, err
	}
	if len(in.Members) == 0 {
		return TrainDetail{}, svcerrors.ErrTrainNoMembers
	}
	if err := t.validateMembers(ctx, in.OwnerUserID, in.Members); err != nil {
		return TrainDetail{}, err
	}

	now := time.Now().UTC()
	for attempt := 0; attempt < domain.MaxCatalogueIDRetries; attempt++ {
		id, err := domain.NewTrainID()
		if err != nil {
			return TrainDetail{}, err
		}
		row := domain.Train{
			ID:          id,
			Source:      domain.EntitySourceLocal,
			OwnerUserID: in.OwnerUserID,
			Name:        name,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := t.trains.Insert(ctx, &row); err != nil {
			if helpers.IsUniqueViolation(err) {
				continue
			}
			return TrainDetail{}, err
		}
		if err := t.replaceMembers(ctx, row.ID, in.Members); err != nil {
			return TrainDetail{}, err
		}
		return t.Get(ctx, row.ID)
	}
	return TrainDetail{}, fmt.Errorf("train id generation exhausted after %d retries", domain.MaxCatalogueIDRetries)
}

// Update renames and/or replaces the member list of an existing train.
func (t *Train) Update(ctx context.Context, actorID uint, trainID domain.TrainID, eff domain.EffectiveRoles, in TrainUpdateInput) (TrainDetail, error) {
	tr, err := t.trains.FindByID(ctx, trainID)
	if err != nil {
		if errors.Is(err, repo.ErrTrainNotFound) {
			return TrainDetail{}, svcerrors.ErrTrainNotFound
		}
		return TrainDetail{}, err
	}
	if err := t.checkTrainMutate(eff, actorID, tr.OwnerUserID); err != nil {
		return TrainDetail{}, err
	}

	if in.Name != nil {
		name, err := validation.SanitiseTrainName(*in.Name)
		if err != nil {
			return TrainDetail{}, err
		}
		if name != tr.Name {
			if other, err := t.trains.FindByOwnerAndName(ctx, tr.OwnerUserID, name); err == nil {
				if other.ID != tr.ID {
					return TrainDetail{}, svcerrors.ErrTrainNameTaken
				}
			} else if !errors.Is(err, repo.ErrTrainNotFound) {
				return TrainDetail{}, err
			}
			tr.Name = name
		}
	}

	if in.Members != nil {
		if len(*in.Members) == 0 {
			return TrainDetail{}, svcerrors.ErrTrainNoMembers
		}
		if err := t.validateMembers(ctx, tr.OwnerUserID, *in.Members); err != nil {
			return TrainDetail{}, err
		}
		if err := t.replaceMembers(ctx, tr.ID, *in.Members); err != nil {
			return TrainDetail{}, err
		}
	}

	tr.UpdatedAt = time.Now().UTC()
	if err := t.trains.Update(ctx, &tr); err != nil {
		return TrainDetail{}, err
	}
	return t.Get(ctx, tr.ID)
}

// Delete removes a train and its member rows.
func (t *Train) Delete(ctx context.Context, actorID uint, trainID domain.TrainID, eff domain.EffectiveRoles) (domain.Train, error) {
	tr, err := t.trains.FindByID(ctx, trainID)
	if err != nil {
		if errors.Is(err, repo.ErrTrainNotFound) {
			return domain.Train{}, svcerrors.ErrTrainNotFound
		}
		return domain.Train{}, err
	}
	if err := t.checkTrainMutate(eff, actorID, tr.OwnerUserID); err != nil {
		return domain.Train{}, err
	}
	if err := t.members.DeleteAllForTrain(ctx, tr.ID); err != nil {
		return domain.Train{}, err
	}
	if err := t.trains.Delete(ctx, &tr); err != nil {
		return domain.Train{}, err
	}
	return tr, nil
}

// UpdateMember patches member consist settings.
// The leading member's multiplier and speed-control participation are immutable.
func (t *Train) UpdateMember(
	ctx context.Context,
	actorID uint,
	trainID domain.TrainID,
	memberID uint,
	eff domain.EffectiveRoles,
	patch TrainMemberPatchInput,
) (domain.TrainMember, error) {
	if patch.SpeedMultiplier == nil && patch.ExcludeFromSpeed == nil &&
		patch.StartDelayMs == nil && patch.AccelRampMs == nil && patch.AccelRampMaxSteps == nil &&
		patch.BrakeRampMs == nil && patch.BrakeRampMaxSteps == nil {
		return domain.TrainMember{}, svcerrors.ErrTrainMemberPatchEmpty
	}
	tr, err := t.trains.FindByID(ctx, trainID)
	if err != nil {
		if errors.Is(err, repo.ErrTrainNotFound) {
			return domain.TrainMember{}, svcerrors.ErrTrainNotFound
		}
		return domain.TrainMember{}, err
	}
	if err := t.checkTrainMutate(eff, actorID, tr.OwnerUserID); err != nil {
		return domain.TrainMember{}, err
	}
	member, err := t.members.FindByID(ctx, memberID)
	if err != nil {
		if errors.Is(err, repo.ErrTrainMemberNotFound) {
			return domain.TrainMember{}, svcerrors.ErrTrainMemberMissing
		}
		return domain.TrainMember{}, err
	}
	if member.TrainID != trainID {
		return domain.TrainMember{}, svcerrors.ErrTrainMemberMissing
	}
	allMembers, err := t.members.ListByTrain(ctx, trainID)
	if err != nil {
		return domain.TrainMember{}, err
	}
	vehicleIDs := make([]domain.VehicleID, 0, len(allMembers))
	for _, m := range allMembers {
		vehicleIDs = append(vehicleIDs, m.VehicleID)
	}
	vehicles, err := t.vehicles.ListByIDs(ctx, vehicleIDs)
	if err != nil {
		return domain.TrainMember{}, err
	}
	byID := make(map[domain.VehicleID]domain.Vehicle, len(vehicles))
	for _, v := range vehicles {
		byID[v.ID] = v
	}
	leading, isLeading := leadingMember(allMembers, byID)
	if isLeading && leading.ID == memberID {
		if patch.SpeedMultiplier != nil {
			return domain.TrainMember{}, svcerrors.ErrTrainLeadingMultiplierImmutable
		}
		if patch.ExcludeFromSpeed != nil && *patch.ExcludeFromSpeed {
			return domain.TrainMember{}, svcerrors.ErrTrainLeadingSpeedControlImmutable
		}
	}
	if patch.SpeedMultiplier != nil {
		if err := validation.ValidateSpeedMultiplier(*patch.SpeedMultiplier); err != nil {
			return domain.TrainMember{}, err
		}
		member.SpeedMultiplier = *patch.SpeedMultiplier
	}
	if patch.ExcludeFromSpeed != nil {
		member.ExcludeFromSpeed = *patch.ExcludeFromSpeed
	}
	if patch.StartDelayMs != nil {
		if err := validation.ValidateStartDelayMs(*patch.StartDelayMs); err != nil {
			return domain.TrainMember{}, err
		}
		member.StartDelayMs = *patch.StartDelayMs
	}
	if patch.AccelRampMs != nil {
		if err := validation.ValidateAccelRampMs(*patch.AccelRampMs); err != nil {
			return domain.TrainMember{}, err
		}
		member.AccelRampMs = *patch.AccelRampMs
	}
	if patch.AccelRampMaxSteps != nil {
		if err := validation.ValidateAccelRampMaxSteps(*patch.AccelRampMaxSteps); err != nil {
			return domain.TrainMember{}, err
		}
		member.AccelRampMaxSteps = *patch.AccelRampMaxSteps
	}
	if patch.BrakeRampMs != nil {
		if err := validation.ValidateBrakeRampMs(*patch.BrakeRampMs); err != nil {
			return domain.TrainMember{}, err
		}
		member.BrakeRampMs = *patch.BrakeRampMs
	}
	if patch.BrakeRampMaxSteps != nil {
		if err := validation.ValidateBrakeRampMaxSteps(*patch.BrakeRampMaxSteps); err != nil {
			return domain.TrainMember{}, err
		}
		member.BrakeRampMaxSteps = *patch.BrakeRampMaxSteps
	}
	if err := t.members.Update(ctx, &member); err != nil {
		return domain.TrainMember{}, err
	}
	return member, nil
}

// UpdateMemberMultiplier sets speedMultiplier on one member row.
// Deprecated: use UpdateMember. Kept for tests and internal callers.
func (t *Train) UpdateMemberMultiplier(
	ctx context.Context,
	actorID uint,
	trainID domain.TrainID,
	memberID uint,
	eff domain.EffectiveRoles,
	multiplier float64,
) (domain.TrainMember, error) {
	return t.UpdateMember(ctx, actorID, trainID, memberID, eff, TrainMemberPatchInput{
		SpeedMultiplier: &multiplier,
	})
}

func (t *Train) checkTrainMutate(eff domain.EffectiveRoles, actorID, ownerUserID uint) error {
	decision := t.sec.CanMutateTrain(eff, actorID, ownerUserID)
	if decision.Allowed {
		return nil
	}
	switch decision.Reason {
	case security.ReasonTrainNotOwned:
		return svcerrors.ErrTrainNotOwned
	default:
		return errors.New(decision.Reason)
	}
}

func (t *Train) validateMembers(ctx context.Context, ownerID uint, members []TrainMemberInput) error {
	seen := make(map[domain.VehicleID]struct{}, len(members))
	for _, m := range members {
		if m.VehicleID.IsZero() {
			return svcerrors.ErrTrainMemberMissing
		}
		if _, dup := seen[m.VehicleID]; dup {
			return svcerrors.ErrTrainMemberMissing
		}
		seen[m.VehicleID] = struct{}{}

		row, err := t.vehicles.FindByID(ctx, m.VehicleID)
		if err != nil {
			if errors.Is(err, repo.ErrVehicleNotFound) {
				return svcerrors.ErrTrainMemberMissing
			}
			return err
		}
		if row.OwnerUserID != ownerID {
			return svcerrors.ErrTrainMemberNotOwned
		}
	}
	return nil
}

func (t *Train) replaceMembers(ctx context.Context, trainID domain.TrainID, members []TrainMemberInput) error {
	if err := t.members.DeleteAllForTrain(ctx, trainID); err != nil {
		return err
	}
	for i, m := range members {
		mult := m.SpeedMultiplier
		if mult == 0 {
			mult = validation.DefaultSpeedMultiplier
		}
		if err := validation.ValidateSpeedMultiplier(mult); err != nil {
			return err
		}
		if err := validation.ValidateStartDelayMs(m.StartDelayMs); err != nil {
			return err
		}
		maxSteps := m.AccelRampMaxSteps
		if maxSteps == 0 {
			maxSteps = validation.DefaultAccelRampMaxSteps
		}
		brakeSteps := m.BrakeRampMaxSteps
		if brakeSteps == 0 {
			brakeSteps = validation.DefaultBrakeRampMaxSteps
		}
		if err := validation.ValidateAccelRampMs(m.AccelRampMs); err != nil {
			return err
		}
		if err := validation.ValidateAccelRampMaxSteps(maxSteps); err != nil {
			return err
		}
		if err := validation.ValidateBrakeRampMs(m.BrakeRampMs); err != nil {
			return err
		}
		if err := validation.ValidateBrakeRampMaxSteps(brakeSteps); err != nil {
			return err
		}
		row := domain.TrainMember{
			TrainID:            trainID,
			VehicleID:          m.VehicleID,
			Position:           i,
			Reversed:           m.Reversed,
			SpeedMultiplier:    mult,
			ExcludeFromSpeed:   m.ExcludeFromSpeed,
			StartDelayMs:       m.StartDelayMs,
			AccelRampMs:        m.AccelRampMs,
			AccelRampMaxSteps:  maxSteps,
			BrakeRampMs:        m.BrakeRampMs,
			BrakeRampMaxSteps:  brakeSteps,
		}
		if err := t.members.Insert(ctx, &row); err != nil {
			return err
		}
	}
	return nil
}

// LeadingMember returns the first member with a DCC address in Position
// order, plus whether one was found.
func LeadingMember(members []domain.TrainMember, vehicles map[domain.VehicleID]domain.Vehicle) (domain.TrainMember, bool) {
	return leadingMember(members, vehicles)
}

func leadingMember(members []domain.TrainMember, vehicles map[domain.VehicleID]domain.Vehicle) (domain.TrainMember, bool) {
	for _, m := range members {
		v, ok := vehicles[m.VehicleID]
		if ok && v.DCCAddress != nil && !m.ExcludeFromSpeed {
			return m, true
		}
	}
	return domain.TrainMember{}, false
}
