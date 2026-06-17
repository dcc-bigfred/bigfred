package cmd

import (
	"context"
	"errors"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
)

// Presence builds and broadcasts layout dashboard online-user snapshots.
type Presence struct {
	hub                 PresenceHubPort
	auth                PresenceAuthPort
	users               *repo.Users
	sessions            *repo.InterlockingSessions
	interlockings       *repo.Interlockings
	layoutInterlockings *repo.LayoutInterlockings
}

func NewPresence(
	hub PresenceHubPort,
	auth PresenceAuthPort,
	users *repo.Users,
	sessions *repo.InterlockingSessions,
	interlockings *repo.Interlockings,
	layoutInterlockings *repo.LayoutInterlockings,
) *Presence {
	return &Presence{
		hub:                 hub,
		auth:                auth,
		users:               users,
		sessions:            sessions,
		interlockings:       interlockings,
		layoutInterlockings: layoutInterlockings,
	}
}

func (s *Presence) ListForLayout(ctx context.Context, layoutID uint) ([]domain.PresenceUser, error) {
	if s.hub == nil {
		return nil, nil
	}
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

func (s *Presence) ListForLayoutEnsuringCaller(ctx context.Context, layoutID uint, caller domain.User) ([]domain.PresenceUser, error) {
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

func (s *Presence) buildEntry(ctx context.Context, layoutID, userID uint, login string) (domain.PresenceUser, error) {
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

func (s *Presence) RefreshAndBroadcast(ctx context.Context, layoutID uint) {
	if s.hub == nil {
		return
	}
	users, err := s.ListForLayout(ctx, layoutID)
	if err != nil {
		return
	}
	s.hub.BroadcastPresenceChanged(layoutID, users)
}
