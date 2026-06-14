package service

import (
	"context"
	"errors"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/ws"
)

// PresenceService builds the layout dashboard "online users" snapshot
// from live WS sessions enriched with roles and interlocking
// occupation (§6.3c).
type PresenceService struct {
	hub                 *ws.Hub
	auth                *AuthService
	users               *repo.Users
	sessions            *repo.InterlockingSessions
	interlockings       *repo.Interlockings
	layoutInterlockings *repo.LayoutInterlockings
}

// NewPresenceService constructs a PresenceService.
func NewPresenceService(
	hub *ws.Hub,
	auth *AuthService,
	users *repo.Users,
	sessions *repo.InterlockingSessions,
	interlockings *repo.Interlockings,
	layoutInterlockings *repo.LayoutInterlockings,
) *PresenceService {
	return &PresenceService{
		hub:                 hub,
		auth:                auth,
		users:               users,
		sessions:            sessions,
		interlockings:       interlockings,
		layoutInterlockings: layoutInterlockings,
	}
}

// ListForLayout returns online users for the layout.
func (s *PresenceService) ListForLayout(ctx context.Context, layoutID uint) ([]domain.PresenceUser, error) {
	online := s.hub.OnlineUsers(layoutID)
	out := make([]domain.PresenceUser, 0, len(online))
	for _, u := range online {
		entry, err := s.buildEntry(ctx, layoutID, u.UserID, u.Login)
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, nil
}

// ListForLayoutEnsuringCaller is the HTTP dashboard snapshot. The hub
// only learns about a user after the WebSocket upgrade registers; the
// first GET /presence on page load often races ahead of that, so the
// authenticated caller is appended when missing from the hub set.
func (s *PresenceService) ListForLayoutEnsuringCaller(ctx context.Context, layoutID uint, caller domain.User) ([]domain.PresenceUser, error) {
	out, err := s.ListForLayout(ctx, layoutID)
	if err != nil {
		return nil, err
	}
	for _, u := range out {
		if u.UserID == caller.ID {
			return out, nil
		}
	}
	entry, err := s.buildEntry(ctx, layoutID, caller.ID, caller.Login)
	if err != nil {
		return nil, err
	}
	return append(out, entry), nil
}

func (s *PresenceService) buildEntry(ctx context.Context, layoutID, userID uint, login string) (domain.PresenceUser, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repo.ErrUserNotFound) {
			return domain.PresenceUser{UserID: userID, Login: login, Role: domain.RoleDriver}, nil
		}
		return domain.PresenceUser{}, err
	}

	role, err := s.auth.EffectiveDisplayRole(ctx, user, layoutID)
	if err != nil {
		return domain.PresenceUser{}, err
	}

	entry := domain.PresenceUser{
		UserID: userID,
		Login:  login,
		Role:   role,
	}

	sess, err := s.sessions.FindActiveByUser(ctx, userID)
	if err != nil {
		if errors.Is(err, repo.ErrInterlockingSessionNotFound) {
			return entry, nil
		}
		return domain.PresenceUser{}, err
	}

	ilk, err := s.interlockings.FindByID(ctx, sess.InterlockingID)
	if err != nil {
		return entry, nil
	}

	whitelisted, err := s.layoutInterlockings.Exists(ctx, layoutID, ilk.ID)
	if err != nil || !whitelisted {
		return entry, nil
	}

	entry.OccupiedInterlocking = &domain.OccupiedInterlocking{
		ID:   ilk.ID,
		Name: ilk.Name,
	}
	return entry, nil
}

// RefreshAndBroadcast rebuilds presence for a layout and fan-outs
// layout.presenceChanged to every live session there.
func (s *PresenceService) RefreshAndBroadcast(ctx context.Context, layoutID uint) {
	users, err := s.ListForLayout(ctx, layoutID)
	if err != nil {
		return
	}
	s.hub.BroadcastToLayout(layoutID, "layout.presenceChanged", ws.PresenceChangedPayload{
		LayoutID: layoutID,
		Users:    users,
	})
}
