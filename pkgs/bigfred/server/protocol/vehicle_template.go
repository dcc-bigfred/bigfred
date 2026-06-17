package protocol

import (
	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// VehicleTemplateFunctionResponse is one function slot on a template.
type VehicleTemplateFunctionResponse struct {
	Num      uint8              `json:"num"`
	Name     string             `json:"name"`
	Icon     domain.FunctionIcon `json:"icon"`
	Position int                `json:"position"`
}

// VehicleTemplateResponse is the REST wire shape for one template.
type VehicleTemplateResponse struct {
	ID          uint                              `json:"id"`
	Name        string                            `json:"name"`
	Description string                            `json:"description"`
	OwnerID     uint                              `json:"ownerId"`
	OwnerLogin  string                            `json:"ownerLogin"`
	Version     int                               `json:"version"`
	Functions   []VehicleTemplateFunctionResponse `json:"functions"`
}

// ToVehicleTemplateResponse maps a list entry to the REST wire shape.
func ToVehicleTemplateResponse(t cmd.VehicleTemplateListEntry) VehicleTemplateResponse {
	fns := make([]VehicleTemplateFunctionResponse, 0, len(t.Functions))
	for _, fn := range t.Functions {
		fns = append(fns, VehicleTemplateFunctionResponse{
			Num:      fn.Num,
			Name:     fn.Name,
			Icon:     fn.Icon,
			Position: fn.Position,
		})
	}
	return VehicleTemplateResponse{
		ID:          t.ID,
		Name:        t.Name,
		Description: t.Description,
		OwnerID:     t.OwnerUserID,
		OwnerLogin:  t.OwnerLogin,
		Version:     t.Version,
		Functions:   fns,
	}
}

// ToVehicleTemplateResponseFromDomain maps a bare domain row (no functions).
func ToVehicleTemplateResponseFromDomain(t domain.VehicleTemplate) VehicleTemplateResponse {
	return VehicleTemplateResponse{
		ID:          t.ID,
		Name:        t.Name,
		Description: t.Description,
		OwnerID:     t.OwnerUserID,
		Version:     t.Version,
		Functions:   nil,
	}
}

// VehicleTemplateCreateRequest is the POST body.
type VehicleTemplateCreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ToCreateInput maps the HTTP body to cmd input.
func (r VehicleTemplateCreateRequest) ToCreateInput(ownerUserID uint) cmd.VehicleTemplateCreateInput {
	return cmd.VehicleTemplateCreateInput{
		OwnerUserID: ownerUserID,
		Name:        r.Name,
		Description: r.Description,
	}
}

// VehicleTemplateUpdateRequest is the PUT body.
type VehicleTemplateUpdateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ToUpdateInput maps the HTTP body to cmd input.
func (r VehicleTemplateUpdateRequest) ToUpdateInput() cmd.VehicleTemplateUpdateInput {
	return cmd.VehicleTemplateUpdateInput{
		Name:        r.Name,
		Description: r.Description,
	}
}
