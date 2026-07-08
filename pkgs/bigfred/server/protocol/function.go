package protocol

import (
	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// FunctionResponse is one resolved function slot on the wire.
type FunctionResponse struct {
	Num        uint8               `json:"num"`
	Name       string              `json:"name"`
	Icon       domain.FunctionIcon `json:"icon"`
	Position   int                 `json:"position"`
	Momentary  bool                `json:"momentary"`
	DurationMs int                 `json:"durationMs"`
	Source     string              `json:"source,omitempty"`
}

// ToFunctionResponses maps cmd rows to REST wire shape.
func ToFunctionResponses(rows []cmd.ResolvedFunction) []FunctionResponse {
	out := make([]FunctionResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, FunctionResponse{
			Num:        r.Num,
			Name:       r.Name,
			Icon:       r.Icon,
			Position:   r.Position,
			Momentary:  r.Momentary,
			DurationMs: r.MomentaryDurationMs,
			Source:     r.Source,
		})
	}
	return out
}

// ToFunctionResponse maps one domain row to the wire shape.
func ToFunctionResponse(row domain.DccFunction, source string) FunctionResponse {
	return FunctionResponse{
		Num:        row.Num,
		Name:       row.Name,
		Icon:       row.Icon,
		Position:   row.Position,
		Momentary:  row.Momentary,
		DurationMs: row.MomentaryDurationMs,
		Source:     source,
	}
}

// FunctionUpsertRequest is the PUT body for one function slot.
type FunctionUpsertRequest struct {
	Name       string              `json:"name"`
	Icon       domain.FunctionIcon `json:"icon"`
	Position   int                 `json:"position"`
	Momentary  bool                `json:"momentary"`
	DurationMs int                 `json:"durationMs"`
}

// ToUpsertInput maps the HTTP body to cmd input.
func (r FunctionUpsertRequest) ToUpsertInput() cmd.FunctionUpsertInput {
	return cmd.FunctionUpsertInput{
		Name:                r.Name,
		Icon:                r.Icon,
		Position:            r.Position,
		Momentary:           r.Momentary,
		MomentaryDurationMs: r.DurationMs,
	}
}

// FunctionReorderEntry maps a function number to display position.
type FunctionReorderEntry struct {
	Num      uint8 `json:"num"`
	Position int   `json:"position"`
}

// FunctionReorderRequest is the POST body for reorder endpoints.
type FunctionReorderRequest struct {
	Positions []FunctionReorderEntry `json:"positions"`
}

// ToReorderEntries maps the HTTP body to cmd input.
func (r FunctionReorderRequest) ToReorderEntries() []cmd.FunctionReorderEntry {
	out := make([]cmd.FunctionReorderEntry, 0, len(r.Positions))
	for _, p := range r.Positions {
		out = append(out, cmd.FunctionReorderEntry{Num: p.Num, Position: p.Position})
	}
	return out
}

// FunctionIconResponse is one icon catalogue entry.
type FunctionIconResponse struct {
	Icon string `json:"icon"`
}

// FunctionCatalogueEntryResponse is one row in the function catalogue.
type FunctionCatalogueEntryResponse struct {
	VehicleID   string             `json:"vehicleId"`
	VehicleName string             `json:"vehicleName"`
	OwnerID     uint               `json:"ownerId"`
	OwnerLogin        string             `json:"ownerLogin"`
	OwnerOrganization string             `json:"ownerOrganization"`
	DCCAddress        *uint16            `json:"dccAddress"`
	Kind        domain.VehicleKind `json:"kind"`
	Functions   []FunctionResponse `json:"functions"`
}

// ToFunctionCatalogueEntry maps a cmd catalogue row to REST wire shape.
func ToFunctionCatalogueEntry(row cmd.VehicleFunctionCatalogueEntry) FunctionCatalogueEntryResponse {
	return FunctionCatalogueEntryResponse{
		VehicleID:   row.VehicleID.String(),
		VehicleName: row.VehicleName,
		OwnerID:     row.OwnerID,
		OwnerLogin:        row.OwnerLogin,
		OwnerOrganization: row.OwnerOrganization,
		DCCAddress:        row.DCCAddress,
		Kind:        row.Kind,
		Functions:   ToFunctionResponses(row.Functions),
	}
}

// FunctionReplaceFromRequest is the POST body for attach/copy endpoints.
type FunctionReplaceFromRequest struct {
	TemplateID      uint `json:"templateId"`
	SourceVehicleID string `json:"sourceVehicleId"`
}
