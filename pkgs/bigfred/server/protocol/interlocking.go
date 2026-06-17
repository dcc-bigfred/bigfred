package protocol

import (
	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

type InterlockingResponse struct {
	ID       uint              `json:"id"`
	Name     string            `json:"name"`
	Location string            `json:"location"`
	Occupant *OccupantResponse `json:"occupant,omitempty"`
}

type OccupantResponse struct {
	UserID uint   `json:"userId"`
	Login  string `json:"login"`
}

func ToInterlockingResponse(i domain.Interlocking) InterlockingResponse {
	return InterlockingResponse{
		ID:       i.ID,
		Name:     i.Name,
		Location: i.Location,
	}
}

type InterlockingCreateRequest struct {
	Name     string `json:"name"`
	Location string `json:"location"`
}

func (r InterlockingCreateRequest) ToCreateInput() cmd.InterlockingCreateInput {
	return cmd.InterlockingCreateInput{
		Name:     r.Name,
		Location: r.Location,
	}
}

type InterlockingUpdateRequest struct {
	Name     *string `json:"name"`
	Location *string `json:"location"`
}

func (r InterlockingUpdateRequest) ToUpdateInput() cmd.InterlockingUpdateInput {
	return cmd.InterlockingUpdateInput{
		Name:     r.Name,
		Location: r.Location,
	}
}
