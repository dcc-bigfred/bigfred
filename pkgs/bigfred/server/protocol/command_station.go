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
	SpeedSteps       uint                      `json:"speedSteps"`
	HeartbeatSecs    float64                   `json:"heartbeatSecs"`
	DeadmanSecs      float64                   `json:"deadmanSecs"`
	PollIntervalMs   uint                      `json:"pollIntervalMs"`
}

func ToCommandStationResponse(cs domain.CommandStation) CommandStationResponse {
	return CommandStationResponse{
		ID:             cs.ID,
		Name:           cs.Name,
		Kind:           cs.Kind,
		ConnectionURI:  cs.ConnectionURI,
		SpeedSteps:     cs.EffectiveSpeedSteps(),
		HeartbeatSecs:  cs.EffectiveHeartbeatSecs(),
		DeadmanSecs:    cs.EffectiveDeadmanSecs(),
		PollIntervalMs: cs.EffectivePollIntervalMs(),
	}
}

type CommandStationCreateRequest struct {
	Name          string                    `json:"name"`
	Kind          domain.CommandStationKind `json:"kind"`
	ConnectionURI string                    `json:"connectionUri"`
	SpeedSteps     uint                      `json:"speedSteps"`
	HeartbeatSecs  float64                   `json:"heartbeatSecs"`
	DeadmanSecs    float64                   `json:"deadmanSecs"`
	PollIntervalMs uint                      `json:"pollIntervalMs"`
}

func (r CommandStationCreateRequest) ToCreateInput() cmd.CommandStationCreateInput {
	return cmd.CommandStationCreateInput{
		Name:           r.Name,
		Kind:           r.Kind,
		ConnectionURI:  r.ConnectionURI,
		SpeedSteps:     r.SpeedSteps,
		HeartbeatSecs:  r.HeartbeatSecs,
		DeadmanSecs:    r.DeadmanSecs,
		PollIntervalMs: r.PollIntervalMs,
	}
}

type CommandStationUpdateRequest struct {
	Name          *string                    `json:"name"`
	Kind          *domain.CommandStationKind `json:"kind"`
	ConnectionURI *string                    `json:"connectionUri"`
	SpeedSteps     *uint                      `json:"speedSteps"`
	HeartbeatSecs  *float64                   `json:"heartbeatSecs"`
	DeadmanSecs    *float64                   `json:"deadmanSecs"`
	PollIntervalMs *uint                      `json:"pollIntervalMs"`
}

func (r CommandStationUpdateRequest) ToUpdateInput() cmd.CommandStationUpdateInput {
	return cmd.CommandStationUpdateInput{
		Name:           r.Name,
		Kind:           r.Kind,
		ConnectionURI:  r.ConnectionURI,
		SpeedSteps:     r.SpeedSteps,
		HeartbeatSecs:  r.HeartbeatSecs,
		DeadmanSecs:    r.DeadmanSecs,
		PollIntervalMs: r.PollIntervalMs,
	}
}
