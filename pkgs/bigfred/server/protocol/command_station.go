package protocol

import (
	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

type CommandStationResponse struct {
	ID            uint                      `json:"id"`
	Name          string                    `json:"name"`
	Kind          domain.CommandStationKind `json:"kind"`
	ConnectionURI string                    `json:"connectionUri"`
	SpeedSteps    uint                      `json:"speedSteps"`
}

func ToCommandStationResponse(cs domain.CommandStation) CommandStationResponse {
	return CommandStationResponse{
		ID:            cs.ID,
		Name:          cs.Name,
		Kind:          cs.Kind,
		ConnectionURI: cs.ConnectionURI,
		SpeedSteps:    cs.SpeedSteps,
	}
}

type CommandStationCreateRequest struct {
	Name          string                    `json:"name"`
	Kind          domain.CommandStationKind `json:"kind"`
	ConnectionURI string                    `json:"connectionUri"`
	SpeedSteps    uint                      `json:"speedSteps"`
}

func (r CommandStationCreateRequest) ToCreateInput() cmd.CommandStationCreateInput {
	return cmd.CommandStationCreateInput{
		Name:          r.Name,
		Kind:          r.Kind,
		ConnectionURI: r.ConnectionURI,
		SpeedSteps:    r.SpeedSteps,
	}
}

type CommandStationUpdateRequest struct {
	Name          *string                    `json:"name"`
	Kind          *domain.CommandStationKind `json:"kind"`
	ConnectionURI *string                    `json:"connectionUri"`
	SpeedSteps    *uint                      `json:"speedSteps"`
}

func (r CommandStationUpdateRequest) ToUpdateInput() cmd.CommandStationUpdateInput {
	return cmd.CommandStationUpdateInput{
		Name:          r.Name,
		Kind:          r.Kind,
		ConnectionURI: r.ConnectionURI,
		SpeedSteps:    r.SpeedSteps,
	}
}
