package protocol

import (
	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// LayoutResponse is the canonical JSON shape of a Layout row.
type LayoutResponse struct {
	ID                 uint   `json:"id"`
	Name               string `json:"name"`
	IsSystem           bool   `json:"isSystem"`
	Locked             bool   `json:"locked"`
	MaxVehiclesPerUser uint   `json:"maxVehiclesPerUser"`
}

// LoginLayoutResponse is the trimmed shape for GET /layouts/login.
type LoginLayoutResponse struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	IsSystem bool   `json:"isSystem"`
}

// ToLayoutResponse maps a domain row to the REST wire shape.
func ToLayoutResponse(l domain.Layout) LayoutResponse {
	return LayoutResponse{
		ID:                 l.ID,
		Name:               l.Name,
		IsSystem:           l.IsSystem,
		Locked:             l.Locked,
		MaxVehiclesPerUser: l.EffectiveMaxVehiclesPerUser(),
	}
}

// ToLoginLayoutResponse maps a domain row to the login dropdown shape.
func ToLoginLayoutResponse(l domain.Layout) LoginLayoutResponse {
	return LoginLayoutResponse{
		ID:       l.ID,
		Name:     l.Name,
		IsSystem: l.IsSystem,
	}
}

// LayoutCreateRequest is the POST /api/v1/layouts body.
type LayoutCreateRequest struct {
	Name                 string `json:"name"`
	InterlockingIDs      []uint `json:"interlockingIds"`
	CommandStationIDs    []uint `json:"commandStationIds"`
	AdminPIN             string `json:"adminPin"`
	MaxVehiclesPerUser   uint   `json:"maxVehiclesPerUser"`
}

// ToCreateInput maps the HTTP body to cmd input.
func (r LayoutCreateRequest) ToCreateInput(createdBy uint) cmd.LayoutCreateInput {
	return cmd.LayoutCreateInput{
		Name:               r.Name,
		CreatedBy:          createdBy,
		InterlockingIDs:    r.InterlockingIDs,
		CommandStationIDs:  r.CommandStationIDs,
		AdminPIN:           r.AdminPIN,
		MaxVehiclesPerUser: r.MaxVehiclesPerUser,
	}
}

// LayoutUpdateRequest is the PUT /api/v1/layouts/{id} body.
type LayoutUpdateRequest struct {
	Name                 string `json:"name"`
	InterlockingIDs      []uint `json:"interlockingIds"`
	CommandStationIDs    []uint `json:"commandStationIds"`
	AdminPIN             string `json:"adminPin"`
	MaxVehiclesPerUser   *uint  `json:"maxVehiclesPerUser"`
}

// SetLayoutCommandStationsRequest is the PUT body for command-station attachments.
type SetLayoutCommandStationsRequest struct {
	CommandStationIDs []uint `json:"commandStationIds"`
}

// SetLayoutInterlockingsRequest is the PUT body for interlocking whitelist.
type SetLayoutInterlockingsRequest struct {
	InterlockingIDs []uint `json:"interlockingIds"`
}
