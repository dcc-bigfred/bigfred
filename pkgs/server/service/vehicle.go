package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/keskad/loco/pkgs/server/domain"
	"github.com/keskad/loco/pkgs/server/repo"
)

// Vehicle sentinel errors.
var (
	// ErrVehicleNotFound is returned when no vehicle row matches.
	ErrVehicleNotFound = errors.New("vehicle_not_found")

	// ErrVehicleNameRequired covers blank/whitespace-only names. The
	// HTTP layer turns this into 422.
	ErrVehicleNameRequired = errors.New("vehicle_name_required")

	// ErrVehicleKindInvalid is returned for a `kind` outside the
	// closed catalogue (VehicleKinds).
	ErrVehicleKindInvalid = errors.New("vehicle_kind_invalid")

	// ErrDCCAddressTaken is returned when another vehicle already
	// owns the requested DCC address (DCC is a globally-unique
	// namespace; dummies bypass this via NULL).
	ErrDCCAddressTaken = errors.New("dcc_address_taken")

	// ErrVehicleNotOwned is the ownership check failure. Editing or
	// deleting another user's vehicle returns 403 via this error.
	ErrVehicleNotOwned = errors.New("vehicle_not_owned")

	// ErrVehicleInUse blocks deletion when other rows still
	// reference the vehicle (currently: train_members).
	ErrVehicleInUse = errors.New("vehicle_in_use")
)

const (
	maxVehicleNameLen   = 64
	maxVehicleNumberLen = 32
)

// VehicleService implements the CRUD lifecycle for domain.Vehicle.
// Pool checks (goal 4) live in DCCPoolService; this service composes
// them with ownership/audit-light validation. Audit-log integration
// is deferred to the M3 milestone — the structure here is friendly
// to a later hook (Create / Update / Delete are the natural
// audit points).
type VehicleService struct {
	vehicles     *repo.Vehicles
	pool         *DCCPoolService
	trainMembers *repo.TrainMembers
}

// NewVehicleService constructs a VehicleService.
func NewVehicleService(v *repo.Vehicles, pool *DCCPoolService, members *repo.TrainMembers) *VehicleService {
	return &VehicleService{vehicles: v, pool: pool, trainMembers: members}
}

// Get loads a vehicle by primary key.
func (s *VehicleService) Get(ctx context.Context, id uint) (domain.Vehicle, error) {
	row, err := s.vehicles.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrVehicleNotFound) {
			return domain.Vehicle{}, ErrVehicleNotFound
		}
		return domain.Vehicle{}, err
	}
	return row, nil
}

// ListOwned returns every vehicle owned by the user.
func (s *VehicleService) ListOwned(ctx context.Context, ownerID uint) ([]domain.Vehicle, error) {
	return s.vehicles.ListByOwner(ctx, ownerID)
}

// VehicleCreateInput is the validated payload of VehicleService.Create.
// DCCAddress is a pointer so the dummy case (no address) is
// representable end-to-end.
type VehicleCreateInput struct {
	OwnerUserID uint
	Name        string
	Kind        domain.VehicleKind
	Number      string
	DCCAddress  *uint16
}

// Create registers a new vehicle owned by the caller. The DCC
// address — when set — is validated against the owner's pool and
// against the global UNIQUE constraint; a nil address creates a
// dummy.
func (s *VehicleService) Create(ctx context.Context, in VehicleCreateInput) (domain.Vehicle, error) {
	name, err := sanitiseVehicleName(in.Name)
	if err != nil {
		return domain.Vehicle{}, err
	}
	if !in.Kind.IsValid() {
		return domain.Vehicle{}, ErrVehicleKindInvalid
	}

	number := strings.TrimSpace(in.Number)
	if len(number) > maxVehicleNumberLen {
		number = number[:maxVehicleNumberLen]
	}

	if in.DCCAddress != nil {
		if err := s.checkDCCAddress(ctx, in.OwnerUserID, *in.DCCAddress, 0); err != nil {
			return domain.Vehicle{}, err
		}
	}

	now := time.Now().UTC()
	row := domain.Vehicle{
		DCCAddress:  in.DCCAddress,
		OwnerUserID: in.OwnerUserID,
		Name:        name,
		Kind:        in.Kind,
		Number:      number,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.vehicles.Insert(ctx, &row); err != nil {
		return domain.Vehicle{}, err
	}
	return row, nil
}

// VehicleUpdateInput is the validated payload of VehicleService.Update.
// Every field is a pointer so the handler can distinguish "not
// provided" from "explicitly cleared".
type VehicleUpdateInput struct {
	Name   *string
	Kind   *domain.VehicleKind
	Number *string

	// DCCAddress carries a tri-state via two pointers:
	//   - DCCAddress.IsSet == false      → leave the column alone;
	//   - DCCAddress.IsSet == true,
	//     DCCAddress.Value == nil        → mark as dummy (NULL);
	//   - DCCAddress.IsSet == true,
	//     DCCAddress.Value != nil        → set / change the address.
	DCCAddress VehicleAddressPatch
}

// VehicleAddressPatch is the tri-state used by VehicleUpdateInput.
type VehicleAddressPatch struct {
	IsSet bool
	Value *uint16
}

// Update mutates an existing vehicle in place. Only the OWNER may
// call this; the HTTP layer is expected to enforce that via the
// VehicleSecurityContext before calling — Update double-checks
// anyway so a missing wiring fails closed.
func (s *VehicleService) Update(ctx context.Context, actorID, vehicleID uint, in VehicleUpdateInput) (domain.Vehicle, error) {
	row, err := s.Get(ctx, vehicleID)
	if err != nil {
		return domain.Vehicle{}, err
	}
	if row.OwnerUserID != actorID {
		return domain.Vehicle{}, ErrVehicleNotOwned
	}

	if in.Name != nil {
		name, err := sanitiseVehicleName(*in.Name)
		if err != nil {
			return domain.Vehicle{}, err
		}
		row.Name = name
	}
	if in.Kind != nil {
		if !in.Kind.IsValid() {
			return domain.Vehicle{}, ErrVehicleKindInvalid
		}
		row.Kind = *in.Kind
	}
	if in.Number != nil {
		num := strings.TrimSpace(*in.Number)
		if len(num) > maxVehicleNumberLen {
			num = num[:maxVehicleNumberLen]
		}
		row.Number = num
	}

	if in.DCCAddress.IsSet {
		if in.DCCAddress.Value == nil {
			row.DCCAddress = nil
		} else {
			newAddr := *in.DCCAddress.Value
			if err := s.checkDCCAddress(ctx, row.OwnerUserID, newAddr, row.ID); err != nil {
				return domain.Vehicle{}, err
			}
			row.DCCAddress = &newAddr
		}
	}

	row.UpdatedAt = time.Now().UTC()
	if err := s.vehicles.Update(ctx, &row); err != nil {
		return domain.Vehicle{}, err
	}
	return row, nil
}

// Delete removes the vehicle. Only the OWNER may call this. Refuses
// when the vehicle is still a member of any train (the user must
// detach it first) so the train_members FK never dangles.
func (s *VehicleService) Delete(ctx context.Context, actorID, vehicleID uint) (domain.Vehicle, error) {
	row, err := s.Get(ctx, vehicleID)
	if err != nil {
		return domain.Vehicle{}, err
	}
	if row.OwnerUserID != actorID {
		return domain.Vehicle{}, ErrVehicleNotOwned
	}
	n, err := s.trainMembers.CountReferencingVehicle(ctx, row.ID)
	if err != nil {
		return domain.Vehicle{}, err
	}
	if n > 0 {
		return domain.Vehicle{}, ErrVehicleInUse
	}
	if err := s.vehicles.Delete(ctx, &row); err != nil {
		return domain.Vehicle{}, err
	}
	return row, nil
}

// checkDCCAddress validates that addr is unused (or used only by
// `excludeID`, which is the vehicle being mutated) AND inside the
// owner's pool.
func (s *VehicleService) checkDCCAddress(ctx context.Context, ownerID uint, addr uint16, excludeID uint) error {
	allowed, err := s.pool.AllowsAddress(ctx, ownerID, addr)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrDCCAddressOutsidePool
	}
	existing, err := s.vehicles.FindByDCCAddress(ctx, addr)
	if err != nil {
		if errors.Is(err, repo.ErrVehicleNotFound) {
			return nil
		}
		return err
	}
	if existing.ID != excludeID {
		return ErrDCCAddressTaken
	}
	return nil
}

func sanitiseVehicleName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", ErrVehicleNameRequired
	}
	if len(name) > maxVehicleNameLen {
		name = name[:maxVehicleNameLen]
	}
	return name, nil
}
