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

// Vehicle implements the CRUD lifecycle for domain.Vehicle (§4.1).
// Pool checks compose via DCCPoolPort; authority via security.
type Vehicle struct {
	vehicles       *repo.Vehicles
	pool           DCCPoolPort
	trainMembers   *repo.TrainMembers
	layoutVehicles *repo.LayoutVehicles
	users          *repo.Users
	sec            security.VehicleSecurityContext
}

// NewVehicle constructs a Vehicle use-case handler.
func NewVehicle(
	v *repo.Vehicles,
	pool DCCPoolPort,
	members *repo.TrainMembers,
	layoutVehicles *repo.LayoutVehicles,
	users *repo.Users,
) *Vehicle {
	return &Vehicle{
		vehicles:       v,
		pool:           pool,
		trainMembers:   members,
		layoutVehicles: layoutVehicles,
		users:          users,
	}
}

// Get loads a vehicle by primary key.
func (v *Vehicle) Get(ctx context.Context, id domain.VehicleID) (domain.Vehicle, error) {
	row, err := v.vehicles.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrVehicleNotFound) {
			return domain.Vehicle{}, svcerrors.ErrVehicleNotFound
		}
		return domain.Vehicle{}, err
	}
	return row, nil
}

// ListOwned returns every vehicle owned by the user.
func (v *Vehicle) ListOwned(ctx context.Context, ownerID uint) ([]domain.Vehicle, error) {
	return v.vehicles.ListByOwner(ctx, ownerID)
}

// VehicleCatalogueEntry is one row of the global vehicle catalogue.
type VehicleCatalogueEntry struct {
	Vehicle           domain.Vehicle
	OwnerLogin        string
	OwnerOrganization string
	OnLayout          bool
}

// ListCatalogue returns every registered vehicle enriched with owner
// metadata and whether it is on the given layout roster.
func (v *Vehicle) ListCatalogue(ctx context.Context, layoutID uint) ([]VehicleCatalogueEntry, error) {
	vehicles, err := v.vehicles.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	onLayout := make(map[domain.VehicleID]struct{})
	if v.layoutVehicles != nil {
		rows, err := v.layoutVehicles.ListByLayout(ctx, layoutID)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			onLayout[row.VehicleID] = struct{}{}
		}
	}
	logins := make(map[uint]struct {
		login        string
		organization string
	})
	out := make([]VehicleCatalogueEntry, 0, len(vehicles))
	for _, vehicle := range vehicles {
		info, ok := logins[vehicle.OwnerUserID]
		if !ok && v.users != nil {
			u, err := v.users.FindByID(ctx, vehicle.OwnerUserID)
			if err != nil {
				info.login = "?"
			} else {
				info.login = u.Login
				info.organization = u.Organization
			}
			logins[vehicle.OwnerUserID] = info
		}
		_, on := onLayout[vehicle.ID]
		out = append(out, VehicleCatalogueEntry{
			Vehicle:           vehicle,
			OwnerLogin:        info.login,
			OwnerOrganization: info.organization,
			OnLayout:          on,
		})
	}
	return out, nil
}

// VehicleCreateInput is the validated payload of Vehicle.Create.
// DCCAddress is a pointer so the dummy case (no address) is
// representable end-to-end.
type VehicleCreateInput struct {
	OwnerUserID uint
	// ExternalID/Source identify rows synced from an integrating client
	// (e.g. the Android catalogue). Zero values keep the local semantics.
	ExternalID *string
	Source     domain.EntitySource

	Name       string
	Kind       domain.VehicleKind
	Number     string
	DCCAddress *uint16

	Rp1Function             *uint8
	EmergencyLightsFunction *uint8
	DeadManSwitchOption     *domain.DeadManSwitchOption

	Carrier      string
	Assignment   string
	Epoch        string
	RevisionDate *string // YYYY-MM-DD or nil/empty
}

// Create registers a new vehicle owned by the caller.
func (v *Vehicle) Create(ctx context.Context, in VehicleCreateInput) (domain.Vehicle, error) {
	name, err := validation.SanitiseVehicleName(in.Name)
	if err != nil {
		return domain.Vehicle{}, err
	}
	if !in.Kind.IsValid() {
		return domain.Vehicle{}, svcerrors.ErrVehicleKindInvalid
	}

	number := validation.TrimVehicleNumber(in.Number)
	carrier := validation.TrimVehicleCarrier(in.Carrier)
	assignment := validation.TrimVehicleAssignment(in.Assignment)
	epoch, err := validation.ParseVehicleEpoch(in.Epoch)
	if err != nil {
		return domain.Vehicle{}, err
	}
	revisionDate, err := validation.ParseVehicleRevisionDate(in.RevisionDate)
	if err != nil {
		return domain.Vehicle{}, err
	}

	if in.DCCAddress != nil {
		if err := v.checkDCCAddress(ctx, in.OwnerUserID, *in.DCCAddress, ""); err != nil {
			return domain.Vehicle{}, err
		}
	}
	rp1Fn, emergFn, dmsOpt, err := validation.ResolveVehicleDeadManFields(
		in.Rp1Function, in.EmergencyLightsFunction, in.DeadManSwitchOption,
	)
	if err != nil {
		return domain.Vehicle{}, err
	}

	now := time.Now().UTC()
	source := in.Source
	if source == "" {
		source = domain.EntitySourceLocal
	}
	for attempt := 0; attempt < domain.MaxCatalogueIDRetries; attempt++ {
		id, err := domain.NewVehicleID()
		if err != nil {
			return domain.Vehicle{}, err
		}
		row := domain.Vehicle{
			ID:                      id,
			ExternalID:              in.ExternalID,
			Source:                  source,
			DCCAddress:              in.DCCAddress,
			OwnerUserID:             in.OwnerUserID,
			Name:                    name,
			Kind:                    in.Kind,
			Number:                  number,
			Rp1Function:             rp1Fn,
			EmergencyLightsFunction: emergFn,
			DeadManSwitchOption:     dmsOpt,
			Carrier:                 carrier,
			Assignment:              assignment,
			RevisionDate:            revisionDate,
			Epoch:                   epoch,
			CreatedAt:               now,
			UpdatedAt:               now,
		}
		if err := v.vehicles.Insert(ctx, &row); err != nil {
			if helpers.IsUniqueViolation(err) {
				continue
			}
			return domain.Vehicle{}, err
		}
		return row, nil
	}
	return domain.Vehicle{}, fmt.Errorf("vehicle id generation exhausted after %d retries", domain.MaxCatalogueIDRetries)
}

// VehicleUpdateInput is the validated payload of Vehicle.Update.
// Every field is a pointer so callers can distinguish "not provided"
// from "explicitly cleared".
type VehicleUpdateInput struct {
	Name   *string
	Kind   *domain.VehicleKind
	Number *string

	Rp1Function             *uint8
	EmergencyLightsFunction *uint8
	DeadManSwitchOption     *domain.DeadManSwitchOption

	// Catalogue metadata — always applied when the dialog submits
	// (frontend always sends these keys).
	Carrier      string
	Assignment   string
	Epoch        string
	RevisionDate *string // YYYY-MM-DD or nil/empty to clear

	// DCCAddress carries a tri-state via IsSet/Value:
	//   - IsSet == false           → leave the column alone;
	//   - IsSet == true, Value nil → mark as dummy (NULL);
	//   - IsSet == true, Value set → set / change the address.
	DCCAddress VehicleAddressPatch
}

// VehicleAddressPatch is the tri-state used by VehicleUpdateInput.
type VehicleAddressPatch struct {
	IsSet bool
	Value *uint16
}

// Update mutates an existing vehicle in place.
func (v *Vehicle) Update(ctx context.Context, actorID uint, vehicleID domain.VehicleID, eff domain.EffectiveRoles, in VehicleUpdateInput) (domain.Vehicle, error) {
	row, err := v.Get(ctx, vehicleID)
	if err != nil {
		return domain.Vehicle{}, err
	}
	if err := v.checkVehicleMutate(eff, actorID, row.OwnerUserID); err != nil {
		return domain.Vehicle{}, err
	}

	if in.Name != nil {
		name, err := validation.SanitiseVehicleName(*in.Name)
		if err != nil {
			return domain.Vehicle{}, err
		}
		row.Name = name
	}
	if in.Kind != nil {
		if !in.Kind.IsValid() {
			return domain.Vehicle{}, svcerrors.ErrVehicleKindInvalid
		}
		row.Kind = *in.Kind
	}
	if in.Number != nil {
		row.Number = validation.TrimVehicleNumber(*in.Number)
	}

	if in.DCCAddress.IsSet {
		if in.DCCAddress.Value == nil {
			row.DCCAddress = nil
		} else {
			newAddr := *in.DCCAddress.Value
			if err := v.checkDCCAddress(ctx, row.OwnerUserID, newAddr, row.ID); err != nil {
				return domain.Vehicle{}, err
			}
			row.DCCAddress = &newAddr
		}
	}
	if in.Rp1Function != nil {
		if !domain.IsValidDccFunctionNum(*in.Rp1Function) {
			return domain.Vehicle{}, svcerrors.ErrVehicleDccFunctionInvalid
		}
		row.Rp1Function = *in.Rp1Function
	}
	if in.EmergencyLightsFunction != nil {
		if !domain.IsValidDccFunctionNum(*in.EmergencyLightsFunction) {
			return domain.Vehicle{}, svcerrors.ErrVehicleDccFunctionInvalid
		}
		row.EmergencyLightsFunction = *in.EmergencyLightsFunction
	}
	if in.DeadManSwitchOption != nil {
		if !in.DeadManSwitchOption.IsValid() {
			return domain.Vehicle{}, svcerrors.ErrVehicleDeadManSwitchInvalid
		}
		row.DeadManSwitchOption = *in.DeadManSwitchOption
	}

	row.Carrier = validation.TrimVehicleCarrier(in.Carrier)
	row.Assignment = validation.TrimVehicleAssignment(in.Assignment)
	epoch, err := validation.ParseVehicleEpoch(in.Epoch)
	if err != nil {
		return domain.Vehicle{}, err
	}
	row.Epoch = epoch
	revisionDate, err := validation.ParseVehicleRevisionDate(in.RevisionDate)
	if err != nil {
		return domain.Vehicle{}, err
	}
	row.RevisionDate = revisionDate

	row.UpdatedAt = time.Now().UTC()
	if err := v.vehicles.Update(ctx, &row); err != nil {
		return domain.Vehicle{}, err
	}
	return row, nil
}

// Delete removes the vehicle when it is not referenced by any train.
func (v *Vehicle) Delete(ctx context.Context, actorID uint, vehicleID domain.VehicleID, eff domain.EffectiveRoles) (domain.Vehicle, error) {
	row, err := v.Get(ctx, vehicleID)
	if err != nil {
		return domain.Vehicle{}, err
	}
	if err := v.checkVehicleMutate(eff, actorID, row.OwnerUserID); err != nil {
		return domain.Vehicle{}, err
	}
	n, err := v.trainMembers.CountReferencingVehicle(ctx, row.ID)
	if err != nil {
		return domain.Vehicle{}, err
	}
	if n > 0 {
		return domain.Vehicle{}, svcerrors.ErrVehicleInUse
	}
	if err := v.vehicles.Delete(ctx, &row); err != nil {
		return domain.Vehicle{}, err
	}
	return row, nil
}

// toUpdateInput projects a full create payload onto the tri-state update
// input, marking every field as "set". The sync client always sends the
// complete vehicle, mirroring the web dialog's always-overwrite semantics.
func (in VehicleCreateInput) toUpdateInput() VehicleUpdateInput {
	kind := in.Kind
	return VehicleUpdateInput{
		Name:                    &in.Name,
		Kind:                    &kind,
		Number:                  &in.Number,
		Rp1Function:             in.Rp1Function,
		EmergencyLightsFunction: in.EmergencyLightsFunction,
		DeadManSwitchOption:     in.DeadManSwitchOption,
		Carrier:                 in.Carrier,
		Assignment:              in.Assignment,
		Epoch:                   in.Epoch,
		RevisionDate:            in.RevisionDate,
		DCCAddress:              VehicleAddressPatch{IsSet: true, Value: in.DCCAddress},
	}
}

// UpsertByExternalID creates or overwrites the vehicle identified by a
// globally-unique external id. Ownership is enforced on the update path so
// a client cannot clobber another user's row. Returns created == true when
// a new row was inserted.
func (v *Vehicle) UpsertByExternalID(ctx context.Context, actorID uint, eff domain.EffectiveRoles, externalID string, in VehicleCreateInput) (domain.Vehicle, bool, error) {
	in.OwnerUserID = actorID
	in.ExternalID = &externalID
	if in.Source == "" {
		in.Source = domain.EntitySourceAndroidCatalog
	}

	existing, err := v.vehicles.FindByExternalID(ctx, externalID)
	if err != nil {
		if errors.Is(err, repo.ErrVehicleNotFound) {
			row, cerr := v.Create(ctx, in)
			return row, true, cerr
		}
		return domain.Vehicle{}, false, err
	}
	if err := v.checkVehicleMutate(eff, actorID, existing.OwnerUserID); err != nil {
		return domain.Vehicle{}, false, err
	}
	row, err := v.Update(ctx, actorID, existing.ID, eff, in.toUpdateInput())
	return row, false, err
}

// DeleteByExternalID removes the vehicle identified by external id, reusing
// Delete for the ownership + train-reference guards.
func (v *Vehicle) DeleteByExternalID(ctx context.Context, actorID uint, eff domain.EffectiveRoles, externalID string) (domain.Vehicle, error) {
	existing, err := v.vehicles.FindByExternalID(ctx, externalID)
	if err != nil {
		if errors.Is(err, repo.ErrVehicleNotFound) {
			return domain.Vehicle{}, svcerrors.ErrVehicleNotFound
		}
		return domain.Vehicle{}, err
	}
	return v.Delete(ctx, actorID, existing.ID, eff)
}

func (v *Vehicle) checkVehicleMutate(eff domain.EffectiveRoles, actorID, ownerUserID uint) error {
	decision := v.sec.CanMutateVehicle(eff, actorID, ownerUserID)
	if decision.Allowed {
		return nil
	}
	switch decision.Reason {
	case security.ReasonVehicleNotOwned:
		return svcerrors.ErrVehicleNotOwned
	default:
		return errors.New(decision.Reason)
	}
}

func (v *Vehicle) checkDCCAddress(ctx context.Context, ownerID uint, addr uint16, excludeID domain.VehicleID) error {
	allowed, err := v.pool.AllowsAddress(ctx, ownerID, addr)
	if err != nil {
		return err
	}
	if !allowed {
		return svcerrors.ErrDCCAddressOutsidePool
	}
	existing, err := v.vehicles.FindByDCCAddress(ctx, addr)
	if err != nil {
		if errors.Is(err, repo.ErrVehicleNotFound) {
			return nil
		}
		return err
	}
	if existing.ID != excludeID {
		return svcerrors.ErrDCCAddressTaken
	}
	return nil
}
