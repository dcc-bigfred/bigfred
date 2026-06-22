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
	Name        string
	Kind        domain.VehicleKind
	Number      string
	DCCAddress  *uint16

	Rp1Function             *uint8
	EmergencyLightsFunction *uint8
	DeadManSwitchOption     *domain.DeadManSwitchOption
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
	for attempt := 0; attempt < domain.MaxCatalogueIDRetries; attempt++ {
		id, err := domain.NewVehicleID()
		if err != nil {
			return domain.Vehicle{}, err
		}
		row := domain.Vehicle{
			ID:                      id,
			Source:                  domain.EntitySourceLocal,
			DCCAddress:              in.DCCAddress,
			OwnerUserID:             in.OwnerUserID,
			Name:                    name,
			Kind:                    in.Kind,
			Number:                  number,
			Rp1Function:             rp1Fn,
			EmergencyLightsFunction: emergFn,
			DeadManSwitchOption:     dmsOpt,
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
