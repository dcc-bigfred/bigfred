package protocol

import "github.com/keskad/loco/pkgs/bigfred/server/domain"

const (
	TypeSessionSetCommandStation            = "session.setCommandStation"
	TypeSessionOpened                       = "session.opened"
	TypeSessionCommandStationChanged        = "session.commandStationChanged"
	TypeSessionCommandStationCatalogChanged = "session.commandStationCatalogChanged"
	TypeSystemRadioStop                     = "system.radioStop"
	TypeSystemEStopTarget                   = "system.estopTarget"
)

type SessionSetCommandStationRequest struct {
	CommandStationID uint `json:"commandStationId"`
}

type SessionCommandStationChangedPayload struct {
	CommandStationID uint    `json:"commandStationId"`
	WSURL            *string `json:"wsUrl"`
	Status           string  `json:"status"`
	Reason           string  `json:"reason,omitempty"`
}

type CommandStationCatalogChangedPayload struct {
	CommandStationID uint                      `json:"commandStationId"`
	Name             string                    `json:"name"`
	Kind             domain.CommandStationKind `json:"kind"`
	SpeedSteps       uint                      `json:"speedSteps"`
}

type OpenedSessionPayload struct {
	SessionID                string                           `json:"sessionId"`
	LayoutID                 uint                             `json:"layoutId"`
	AvailableCommandStations []AvailableCommandStationPayload `json:"availableCommandStations"`
	CurrentSession           *CurrentSessionPayload           `json:"currentSession,omitempty"`
}

type AvailableCommandStationPayload struct {
	ID         uint                      `json:"id"`
	Name       string                    `json:"name"`
	Kind       domain.CommandStationKind `json:"kind"`
	SpeedSteps uint                      `json:"speedSteps"`
	WSURL      *string                   `json:"wsUrl"`
}

type CurrentSessionPayload struct {
	CommandStationID uint `json:"commandStationId"`
}
