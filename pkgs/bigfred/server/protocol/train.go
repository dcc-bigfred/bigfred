package protocol

import (
	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// TrainMemberResponse is one row in a train catalogue response.
type TrainMemberResponse struct {
	ID              uint    `json:"id"`
	VehicleID       uint    `json:"vehicleId"`
	Position        int     `json:"position"`
	Reversed        bool    `json:"reversed"`
	SpeedMultiplier float64 `json:"speedMultiplier"`
}

// ToTrainMemberResponse maps a domain member to the REST wire shape.
func ToTrainMemberResponse(m domain.TrainMember) TrainMemberResponse {
	mult := m.SpeedMultiplier
	if mult <= 0 {
		mult = 1.0
	}
	return TrainMemberResponse{
		ID:              m.ID,
		VehicleID:       m.VehicleID,
		Position:        m.Position,
		Reversed:        m.Reversed,
		SpeedMultiplier: mult,
	}
}

// TrainResponse is the JSON shape for one catalogue train.
type TrainResponse struct {
	ID      uint                  `json:"id"`
	Name    string                `json:"name"`
	OwnerID uint                  `json:"ownerId"`
	Members []TrainMemberResponse `json:"members"`
}

// ToTrainResponse maps a cmd detail bundle to the REST wire shape.
func ToTrainResponse(d cmd.TrainDetail) TrainResponse {
	members := make([]TrainMemberResponse, 0, len(d.Members))
	for _, m := range d.Members {
		members = append(members, ToTrainMemberResponse(m))
	}
	return TrainResponse{
		ID:      d.Train.ID,
		Name:    d.Train.Name,
		OwnerID: d.Train.OwnerUserID,
		Members: members,
	}
}

// TrainMemberRequest is one member in a create/update body.
type TrainMemberRequest struct {
	VehicleID       uint    `json:"vehicleId"`
	Reversed        bool    `json:"reversed"`
	SpeedMultiplier float64 `json:"speedMultiplier,omitempty"`
}

// ToMemberInput maps the HTTP member row to cmd input.
func (r TrainMemberRequest) ToMemberInput() cmd.TrainMemberInput {
	return cmd.TrainMemberInput{
		VehicleID:       r.VehicleID,
		Reversed:        r.Reversed,
		SpeedMultiplier: r.SpeedMultiplier,
	}
}

// TrainCreateRequest is the POST /api/v1/trains body.
type TrainCreateRequest struct {
	Name    string               `json:"name"`
	Members []TrainMemberRequest `json:"members"`
}

// ToCreateInput maps the HTTP body to the cmd use-case input.
func (r TrainCreateRequest) ToCreateInput(ownerUserID uint) cmd.TrainCreateInput {
	members := make([]cmd.TrainMemberInput, 0, len(r.Members))
	for _, m := range r.Members {
		members = append(members, m.ToMemberInput())
	}
	return cmd.TrainCreateInput{
		OwnerUserID: ownerUserID,
		Name:        r.Name,
		Members:     members,
	}
}

// TrainUpdateRequest mirrors the tri-state in cmd.TrainUpdateInput.
type TrainUpdateRequest struct {
	Name       *string              `json:"name"`
	Members    []TrainMemberRequest `json:"members"`
	MembersSet bool                 `json:"membersSet"`
}

// ToUpdateInput maps the HTTP body to the cmd use-case input.
func (r TrainUpdateRequest) ToUpdateInput() cmd.TrainUpdateInput {
	in := cmd.TrainUpdateInput{Name: r.Name}
	if r.MembersSet {
		members := make([]cmd.TrainMemberInput, 0, len(r.Members))
		for _, m := range r.Members {
			members = append(members, m.ToMemberInput())
		}
		in.Members = &members
	}
	return in
}

// TrainMemberPatchRequest is the PATCH body for one member multiplier.
type TrainMemberPatchRequest struct {
	SpeedMultiplier float64 `json:"speedMultiplier"`
}
