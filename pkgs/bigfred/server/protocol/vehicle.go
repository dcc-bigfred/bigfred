package protocol

import (
	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/validation"
)

// VehicleResponse is the JSON shape the frontend consumes for one
// catalogue vehicle. DCCAddress is a pointer so the dummy case
// ("DCC: —") is representable.
type VehicleResponse struct {
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	Kind       domain.VehicleKind `json:"kind"`
	Number     string             `json:"number"`
	DCCAddress *uint16            `json:"dccAddress"`
	IsDummy    bool               `json:"isDummy"`
	OwnerID    uint               `json:"ownerId"`

	Rp1Function             uint8                      `json:"rp1Function"`
	EmergencyLightsFunction uint8                      `json:"emergencyLightsFunction"`
	DeadManSwitchOption     domain.DeadManSwitchOption `json:"deadManSwitchOption"`

	Carrier      string  `json:"carrier"`
	Assignment   string  `json:"assignment"`
	RevisionDate *string `json:"revisionDate"`
	Epoch        string  `json:"epoch"`
}

// ToVehicleResponse maps a domain row to the REST wire shape.
func ToVehicleResponse(v domain.Vehicle) VehicleResponse {
	return VehicleResponse{
		ID:                      v.ID.String(),
		Name:                    v.Name,
		Kind:                    v.Kind,
		Number:                  v.Number,
		DCCAddress:              v.DCCAddress,
		IsDummy:                 v.IsDummy(),
		OwnerID:                 v.OwnerUserID,
		Rp1Function:             v.Rp1Function,
		EmergencyLightsFunction: v.EmergencyLightsFunction,
		DeadManSwitchOption:     v.DeadManSwitchOption,
		Carrier:                 v.Carrier,
		Assignment:              v.Assignment,
		RevisionDate:            validation.FormatVehicleRevisionDate(v.RevisionDate),
		Epoch:                   string(v.Epoch),
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

	Carrier      string  `json:"carrier"`
	Assignment   string  `json:"assignment"`
	RevisionDate *string `json:"revisionDate"`
	Epoch        string  `json:"epoch"`
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
		Carrier:                 r.Carrier,
		Assignment:              r.Assignment,
		RevisionDate:            r.RevisionDate,
		Epoch:                   r.Epoch,
	}
}

// VehicleUpdateRequest mirrors the tri-state in cmd.VehicleUpdateInput.
// DCCAddressSet is true when the client wants to mutate the column.
// Carrier/Assignment/Epoch/RevisionDate are always applied by the dialog.
type VehicleUpdateRequest struct {
	Name          *string             `json:"name"`
	Kind          *domain.VehicleKind `json:"kind"`
	Number        *string             `json:"number"`
	DCCAddress    *uint16             `json:"dccAddress"`
	DCCAddressSet bool                `json:"dccAddressSet"`

	Rp1Function             *uint8                      `json:"rp1Function"`
	EmergencyLightsFunction *uint8                      `json:"emergencyLightsFunction"`
	DeadManSwitchOption     *domain.DeadManSwitchOption `json:"deadManSwitchOption"`

	Carrier      string  `json:"carrier"`
	Assignment   string  `json:"assignment"`
	RevisionDate *string `json:"revisionDate"`
	Epoch        string  `json:"epoch"`
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
		Carrier:                 r.Carrier,
		Assignment:              r.Assignment,
		RevisionDate:            r.RevisionDate,
		Epoch:                   r.Epoch,
	}
	if r.DCCAddressSet {
		in.DCCAddress = cmd.VehicleAddressPatch{IsSet: true, Value: r.DCCAddress}
	}
	return in
}

// VehicleCatalogueResponse is one row of GET /api/v1/vehicles/catalogue.
type VehicleCatalogueResponse struct {
	VehicleResponse
	OwnerLogin        string `json:"ownerLogin"`
	OwnerOrganization string `json:"ownerOrganization"`
	OnLayout          bool   `json:"onLayout"`
}

// ToVehicleCatalogueResponse maps a catalogue entry to the REST wire shape.
func ToVehicleCatalogueResponse(e cmd.VehicleCatalogueEntry) VehicleCatalogueResponse {
	return VehicleCatalogueResponse{
		VehicleResponse:   ToVehicleResponse(e.Vehicle),
		OwnerLogin:        e.OwnerLogin,
		OwnerOrganization: e.OwnerOrganization,
		OnLayout:          e.OnLayout,
	}
}
