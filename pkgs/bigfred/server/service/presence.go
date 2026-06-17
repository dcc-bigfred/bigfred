package service

import (
	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/ws"
)

// PresenceService is the legacy name for cmd.Presence.
type PresenceService = cmd.Presence

func NewPresenceService(
	hub *ws.Hub,
	auth *cmd.Auth,
	users *repo.Users,
	sessions *repo.InterlockingSessions,
	interlockings *repo.Interlockings,
	layoutInterlockings *repo.LayoutInterlockings,
) *cmd.Presence {
	var hubPort cmd.PresenceHubPort
	if hub != nil {
		hubPort = presenceHub{hub: hub}
	}
	return cmd.NewPresence(hubPort, auth, users, sessions, interlockings, layoutInterlockings)
}

type presenceHub struct {
	hub *ws.Hub
}

func (h presenceHub) OnlineUsers(layoutID uint) []cmd.PresenceOnlineUser {
	rows := h.hub.OnlineUsers(layoutID)
	out := make([]cmd.PresenceOnlineUser, 0, len(rows))
	for _, row := range rows {
		out = append(out, cmd.PresenceOnlineUser{UserID: row.UserID, Login: row.Login})
	}
	return out
}

func (h presenceHub) BroadcastPresenceChanged(layoutID uint, users []domain.PresenceUser) {
	h.hub.BroadcastToLayout(layoutID, "layout.presenceChanged", ws.PresenceChangedPayload{
		LayoutID: layoutID,
		Users:    users,
	})
}
