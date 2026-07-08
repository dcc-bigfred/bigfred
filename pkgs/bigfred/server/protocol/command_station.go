package protocol

import (
	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

type CommandStationResponse struct {
	ID                      uint                      `json:"id"`
	Name                    string                    `json:"name"`
	Kind                    domain.CommandStationKind `json:"kind"`
	ConnectionURI           string                    `json:"connectionUri"`
	SpeedSteps              uint                      `json:"speedSteps"`
	HeartbeatSecs           float64                   `json:"heartbeatSecs"`
	DeadmanSecs             float64                   `json:"deadmanSecs"`
	PollIntervalMs          uint                      `json:"pollIntervalMs"`
	Z21ServerEnabled        bool                      `json:"z21ServerEnabled"`
	Z21IPStickiness         bool                      `json:"z21IpStickiness"`
	WithrottleServerEnabled bool                      `json:"withrottleServerEnabled"`
	MaxLoconetSlots         uint                      `json:"maxLoconetSlots,omitempty"`
	IdleTimeoutSecs         uint                      `json:"idleTimeoutSecs,omitempty"`
	BootStopEnabled         bool                      `json:"bootStopEnabled"`
}

func ToCommandStationResponse(cs domain.CommandStation) CommandStationResponse {
	resp := CommandStationResponse{
		ID:                      cs.ID,
		Name:                    cs.Name,
		Kind:                    cs.Kind,
		ConnectionURI:           cs.ConnectionURI,
		SpeedSteps:              cs.EffectiveSpeedSteps(),
		HeartbeatSecs:           cs.EffectiveHeartbeatSecs(),
		DeadmanSecs:             cs.EffectiveDeadmanSecs(),
		PollIntervalMs:          cs.EffectivePollIntervalMs(),
		Z21ServerEnabled:        cs.Z21ServerEnabled,
		Z21IPStickiness:         cs.Z21IPStickiness,
		WithrottleServerEnabled: cs.WithrottleServerEnabled,
		BootStopEnabled:         cs.BootStopEnabled,
	}
	if cs.Kind.IsLocoNet() {
		resp.MaxLoconetSlots = cs.EffectiveMaxLoconetSlots()
		resp.IdleTimeoutSecs = cs.IdleTimeoutSecs
	}
	return resp
}

type CommandStationCreateRequest struct {
	Name          string                    `json:"name"`
	Kind          domain.CommandStationKind `json:"kind"`
	ConnectionURI string                    `json:"connectionUri"`
	SpeedSteps     uint                      `json:"speedSteps"`
	HeartbeatSecs  float64                   `json:"heartbeatSecs"`
	DeadmanSecs    float64                   `json:"deadmanSecs"`
	PollIntervalMs   uint                      `json:"pollIntervalMs"`
	Z21ServerEnabled bool                      `json:"z21ServerEnabled"`
	Z21IPStickiness  bool                      `json:"z21IpStickiness"`
	WithrottleServerEnabled bool                      `json:"withrottleServerEnabled"`
	MaxLoconetSlots         uint                      `json:"maxLoconetSlots"`
	IdleTimeoutSecs         uint                      `json:"idleTimeoutSecs"`
	BootStopEnabled         bool                      `json:"bootStopEnabled"`
}

func (r CommandStationCreateRequest) ToCreateInput() cmd.CommandStationCreateInput {
	return cmd.CommandStationCreateInput{
		Name:             r.Name,
		Kind:             r.Kind,
		ConnectionURI:    r.ConnectionURI,
		SpeedSteps:       r.SpeedSteps,
		HeartbeatSecs:    r.HeartbeatSecs,
		DeadmanSecs:      r.DeadmanSecs,
		PollIntervalMs:   r.PollIntervalMs,
		Z21ServerEnabled: r.Z21ServerEnabled,
		Z21IPStickiness:  r.Z21IPStickiness,
		WithrottleServerEnabled: r.WithrottleServerEnabled,
		MaxLoconetSlots:         r.MaxLoconetSlots,
		IdleTimeoutSecs:         r.IdleTimeoutSecs,
		BootStopEnabled:         r.BootStopEnabled,
	}
}

type CommandStationUpdateRequest struct {
	Name          *string                    `json:"name"`
	Kind          *domain.CommandStationKind `json:"kind"`
	ConnectionURI *string                    `json:"connectionUri"`
	SpeedSteps     *uint                      `json:"speedSteps"`
	HeartbeatSecs  *float64                   `json:"heartbeatSecs"`
	DeadmanSecs    *float64                   `json:"deadmanSecs"`
	PollIntervalMs   *uint                      `json:"pollIntervalMs"`
	Z21ServerEnabled *bool                    `json:"z21ServerEnabled"`
	Z21IPStickiness  *bool                    `json:"z21IpStickiness"`
	WithrottleServerEnabled *bool                      `json:"withrottleServerEnabled"`
	MaxLoconetSlots         *uint                      `json:"maxLoconetSlots"`
	IdleTimeoutSecs         *uint                      `json:"idleTimeoutSecs"`
	BootStopEnabled         *bool                      `json:"bootStopEnabled"`
}

func (r CommandStationUpdateRequest) ToUpdateInput() cmd.CommandStationUpdateInput {
	return cmd.CommandStationUpdateInput{
		Name:             r.Name,
		Kind:             r.Kind,
		ConnectionURI:    r.ConnectionURI,
		SpeedSteps:       r.SpeedSteps,
		HeartbeatSecs:    r.HeartbeatSecs,
		DeadmanSecs:      r.DeadmanSecs,
		PollIntervalMs:   r.PollIntervalMs,
		Z21ServerEnabled: r.Z21ServerEnabled,
		Z21IPStickiness:  r.Z21IPStickiness,
		WithrottleServerEnabled: r.WithrottleServerEnabled,
		MaxLoconetSlots:         r.MaxLoconetSlots,
		IdleTimeoutSecs:         r.IdleTimeoutSecs,
		BootStopEnabled:         r.BootStopEnabled,
	}
}
