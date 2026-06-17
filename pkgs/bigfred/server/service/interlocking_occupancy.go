package service

import (
	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/ws"
)

var (
	ErrInterlockingOccupied    = svcerrors.ErrInterlockingOccupied
	ErrInterlockingNotInLayout = svcerrors.ErrInterlockingNotInLayout
	ErrNotSignalman            = svcerrors.ErrNotSignalman
)

type InterlockingWithOccupant = cmd.InterlockingWithOccupant
type OccupantInfo = cmd.OccupantInfo
type JoinResult = cmd.JoinResult
type JoinInput = cmd.JoinInput

// InterlockingOccupancyService is the legacy facade for cmd.InterlockingOccupancy.
type InterlockingOccupancyService struct {
	*cmd.InterlockingOccupancy
}

func NewInterlockingOccupancyService(
	interlockings *repo.Interlockings,
	layoutInterlockings *repo.LayoutInterlockings,
	sessions *repo.InterlockingSessions,
	users *repo.Users,
	auth *cmd.Auth,
	hub *ws.Hub,
	presence *cmd.Presence,
) *InterlockingOccupancyService {
	var hubPort cmd.InterlockingOccupancyHubPort
	if hub != nil {
		hubPort = interlockingOccupancyHub{hub: hub}
	}
	return &InterlockingOccupancyService{InterlockingOccupancy: cmd.NewInterlockingOccupancy(
		interlockings, layoutInterlockings, sessions, users, auth, hubPort, presence,
	)}
}

// SetTakeoverService wires takeover release on leave/displace.
func (s *InterlockingOccupancyService) SetTakeoverService(t *TakeoverService) {
	s.InterlockingOccupancy.SetTakeover(t)
}

type interlockingOccupancyHub struct {
	hub *ws.Hub
}

func (h interlockingOccupancyHub) BroadcastOccupantChanged(layoutID, interlockingID uint, occupant *cmd.OccupantInfo, reason string) {
	payload := ws.OccupantChangedPayload{
		InterlockingID: interlockingID,
		Reason:         reason,
	}
	if occupant != nil {
		payload.Occupant = &ws.OccupantPayload{
			UserID: occupant.UserID,
			Login:  occupant.Login,
		}
	}
	h.hub.BroadcastToLayout(layoutID, "interlocking.occupantChanged", payload)
}
