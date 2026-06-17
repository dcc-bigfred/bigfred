package protocol

import (
	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// VehicleResponse is the JSON shape the frontend consumes for one
// catalogue vehicle. DCCAddress is a pointer so the dummy case
// ("DCC: —") is representable.
type VehicleResponse struct {
	ID         uint               `json:"id"`
	Name       string             `json:"name"`
	Kind       domain.VehicleKind `json:"kind"`
	Number     string             `json:"number"`
	DCCAddress *uint16            `json:"dccAddress"`
	IsDummy    bool               `json:"isDummy"`
	OwnerID    uint               `json:"ownerId"`

	Rp1Function             uint8                      `json:"rp1Function"`
	EmergencyLightsFunction uint8                      `json:"emergencyLightsFunction"`
	DeadManSwitchOption     domain.DeadManSwitchOption `json:"deadManSwitchOption"`
}

// ToVehicleResponse maps a domain row to the REST wire shape.
func ToVehicleResponse(v domain.Vehicle) VehicleResponse {
	return VehicleResponse{
		ID:                      v.ID,
		Name:                    v.Name,
		Kind:                    v.Kind,
		Number:                  v.Number,
		DCCAddress:              v.DCCAddress,
		IsDummy:                 v.IsDummy(),
		OwnerID:                 v.OwnerUserID,
		Rp1Function:             v.Rp1Function,
		EmergencyLightsFunction: v.EmergencyLightsFunction,
		DeadManSwitchOption:     v.DeadManSwitchOption,
	}
}

// VehicleCreateRequest is the POST /api/v1/vehicles body.
type VehicleCreateRequest struct {
	Name       string             `json:"name"`
	Kind       domain.VehicleKind `json:"kind"`
	Number     string             `json:"number"`
	DCCAddress *uint16            `json:"dccAddress"`

	Rp1Function             *uint8                      `json:"rp1Function"`
	EmergencyLightsFunction *uint8                      `json:"emergencyLightsFunction"`
	DeadManSwitchOption     *domain.DeadManSwitchOption `json:"deadManSwitchOption"`
}

// ToCreateInput maps the HTTP body to the cmd use-case input.
func (r VehicleCreateRequest) ToCreateInput(ownerUserID uint) cmd.VehicleCreateInput {
	return cmd.VehicleCreateInput{
		OwnerUserID:             ownerUserID,
		Name:                    r.Name,
		Kind:                    r.Kind,
		Number:                  r.Number,
		DCCAddress:              r.DCCAddress,
		Rp1Function:             r.Rp1Function,
		EmergencyLightsFunction: r.EmergencyLightsFunction,
		DeadManSwitchOption:     r.DeadManSwitchOption,
	}
}

// VehicleUpdateRequest mirrors the tri-state in cmd.VehicleUpdateInput.
// DCCAddressSet is true when the client wants to mutate the column.
type VehicleUpdateRequest struct {
	Name          *string             `json:"name"`
	Kind          *domain.VehicleKind `json:"kind"`
	Number        *string             `json:"number"`
	DCCAddress    *uint16             `json:"dccAddress"`
	DCCAddressSet bool                `json:"dccAddressSet"`

	Rp1Function             *uint8                      `json:"rp1Function"`
	EmergencyLightsFunction *uint8                      `json:"emergencyLightsFunction"`
	DeadManSwitchOption     *domain.DeadManSwitchOption `json:"deadManSwitchOption"`
}

// ToUpdateInput maps the HTTP body to the cmd use-case input.
func (r VehicleUpdateRequest) ToUpdateInput() cmd.VehicleUpdateInput {
	in := cmd.VehicleUpdateInput{
		Name:                    r.Name,
		Kind:                    r.Kind,
		Number:                  r.Number,
		Rp1Function:             r.Rp1Function,
		EmergencyLightsFunction: r.EmergencyLightsFunction,
		DeadManSwitchOption:     r.DeadManSwitchOption,
	}
	if r.DCCAddressSet {
		in.DCCAddress = cmd.VehicleAddressPatch{IsSet: true, Value: r.DCCAddress}
	}
	return in
}
