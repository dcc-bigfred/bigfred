package service

import (
	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/ws"
)

type RadioConfig struct {
	Store         *RadioStore
	Hub           *ws.Hub
	Auth          *cmd.Auth
	Layouts       *repo.Layouts
	Vehicles      *repo.Vehicles
	Trains        *repo.Trains
	IlkSessions   *repo.InterlockingSessions
	LayoutIlks    *repo.LayoutInterlockings
	Interlockings *repo.Interlockings
}

type SendInput = cmd.RadioSendInput

// RadioService is the legacy name for cmd.Radio.
type RadioService = cmd.Radio

func NewRadioService(cfg RadioConfig) *RadioService {
	var hub cmd.RadioHubPort
	if cfg.Hub != nil {
		hub = radioHub{hub: cfg.Hub}
	}
	return cmd.NewRadio(cmd.RadioConfig{
		Store:         cfg.Store,
		Hub:           hub,
		Auth:          cfg.Auth,
		Layouts:       cfg.Layouts,
		Vehicles:      cfg.Vehicles,
		Trains:        cfg.Trains,
		IlkSessions:   cfg.IlkSessions,
		LayoutIlks:    cfg.LayoutIlks,
		Interlockings: cfg.Interlockings,
	})
}

func RadioDeniedCode(err error) string { return cmd.RadioDeniedCode(err) }

type radioHub struct {
	hub *ws.Hub
}

func (h radioHub) BroadcastRadioMessage(layoutID, userID uint, msg domain.RadioMessage) {
	h.hub.BroadcastToUserInLayout(layoutID, userID, contract.TypeRadioMessage, contract.MessageWireFromDomain(msg))
}
