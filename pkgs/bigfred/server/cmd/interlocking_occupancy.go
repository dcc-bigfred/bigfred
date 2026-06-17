package cmd

import (
	"context"
	"errors"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/security"
)

// InterlockingWithOccupant pairs a catalogue row with its current occupant.
type InterlockingWithOccupant struct {
	Interlocking domain.Interlocking
	Occupant     *OccupantInfo
}

// OccupantInfo identifies the signalman staffing a box.
type OccupantInfo struct {
	UserID uint
	Login  string
}

// JoinResult describes the outcome of occupying an interlocking.
type JoinResult struct {
	Interlocking domain.Interlocking
	Occupant     OccupantInfo
	Displaced    *OccupantInfo
}

// InterlockingOccupancy handles join/leave and occupant enrichment.
type InterlockingOccupancy struct {
	interlockings       *repo.Interlockings
	layoutInterlockings *repo.LayoutInterlockings
	sessions            *repo.InterlockingSessions
	users               *repo.Users
	auth                InterlockingOccupancyAuthPort
	hub                 InterlockingOccupancyHubPort
	presence            InterlockingOccupancyPresencePort
	takeover            InterlockingOccupancyTakeoverPort
	sec                 security.InterlockingSecurityContext
}

func NewInterlockingOccupancy(
	interlockings *repo.Interlockings,
	layoutInterlockings *repo.LayoutInterlockings,
	sessions *repo.InterlockingSessions,
	users *repo.Users,
	auth InterlockingOccupancyAuthPort,
	hub InterlockingOccupancyHubPort,
	presence InterlockingOccupancyPresencePort,
) *InterlockingOccupancy {
	return &InterlockingOccupancy{
		interlockings:       interlockings,
		layoutInterlockings: layoutInterlockings,
		sessions:            sessions,
		users:               users,
		auth:                auth,
		hub:                 hub,
		presence:            presence,
	}
}

// SetTakeover wires takeover release on leave/displace.
func (s *InterlockingOccupancy) SetTakeover(t InterlockingOccupancyTakeoverPort) {
	s.takeover = t
}

func (s *InterlockingOccupancy) GetForLayout(
	ctx context.Context,
	layoutID, interlockingID uint,
) (InterlockingWithOccupant, error) {
	whitelisted, err := s.layoutInterlockings.Exists(ctx, layoutID, interlockingID)
	if err != nil {
		return InterlockingWithOccupant{}, err
	}
	if !whitelisted {
		return InterlockingWithOccupant{}, svcerrors.ErrInterlockingNotInLayout
	}
	row, err := s.interlockings.FindByID(ctx, interlockingID)
	if err != nil {
		if errors.Is(err, repo.ErrInterlockingNotFound) {
			return InterlockingWithOccupant{}, svcerrors.ErrInterlockingNotFound
		}
		return InterlockingWithOccupant{}, err
	}
	out := InterlockingWithOccupant{Interlocking: row}
	if sess, err := s.sessions.FindActiveByInterlocking(ctx, interlockingID); err == nil {
		if user, err := s.users.FindByID(ctx, sess.SignalmanUserID); err == nil {
			out.Occupant = &OccupantInfo{UserID: user.ID, Login: user.Login}
		}
	} else if !errors.Is(err, repo.ErrInterlockingSessionNotFound) {
		return InterlockingWithOccupant{}, err
	}
	return out, nil
}

func (s *InterlockingOccupancy) ListForLayout(ctx context.Context, layoutID uint) ([]InterlockingWithOccupant, error) {
	ids, err := s.layoutInterlockings.InterlockingIDsForLayout(ctx, layoutID)
	if err != nil {
		return nil, err
	}
	rows, err := s.interlockings.ListByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	active, err := s.sessions.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	byInterlocking := make(map[uint]domain.InterlockingSession, len(active))
	for _, sess := range active {
		byInterlocking[sess.InterlockingID] = sess
	}

	out := make([]InterlockingWithOccupant, 0, len(rows))
	for _, row := range rows {
		item := InterlockingWithOccupant{Interlocking: row}
		if sess, ok := byInterlocking[row.ID]; ok {
			user, err := s.users.FindByID(ctx, sess.SignalmanUserID)
			if err == nil {
				item.Occupant = &OccupantInfo{UserID: user.ID, Login: user.Login}
			}
		}
		out = append(out, item)
	}
	return out, nil
}

// JoinInput is the validated payload for Join.
type JoinInput struct {
	InterlockingID uint
	LayoutID       uint
	Actor          domain.User
	Force          bool
}

func (s *InterlockingOccupancy) Join(ctx context.Context, in JoinInput) (JoinResult, error) {
	now := time.Now().UTC()

	ilk, err := s.interlockings.FindByID(ctx, in.InterlockingID)
	if err != nil {
		if errors.Is(err, repo.ErrInterlockingNotFound) {
			return JoinResult{}, svcerrors.ErrInterlockingNotFound
		}
		return JoinResult{}, err
	}

	var layoutRow *domain.LayoutInterlocking
	rows, err := s.layoutInterlockings.ListByLayoutID(ctx, in.LayoutID)
	if err != nil {
		return JoinResult{}, err
	}
	for i := range rows {
		if rows[i].InterlockingID == in.InterlockingID {
			layoutRow = &rows[i]
			break
		}
	}

	eff, err := s.auth.Effective(ctx, in.Actor, in.LayoutID)
	if err != nil {
		return JoinResult{}, err
	}

	var current *domain.InterlockingSession
	if sess, err := s.sessions.FindActiveByInterlocking(ctx, in.InterlockingID); err == nil {
		current = &sess
	} else if !errors.Is(err, repo.ErrInterlockingSessionNotFound) {
		return JoinResult{}, err
	}

	if current != nil && current.SignalmanUserID == in.Actor.ID {
		return JoinResult{
			Interlocking: ilk,
			Occupant:     OccupantInfo{UserID: in.Actor.ID, Login: in.Actor.Login},
		}, nil
	}

	decision := s.sec.CanOccupy(eff, in.Actor.ID, layoutRow, current)
	if !decision.Allowed {
		switch decision.Reason {
		case "interlocking_not_in_layout":
			return JoinResult{}, svcerrors.ErrInterlockingNotInLayout
		case "not_signalman":
			return JoinResult{}, svcerrors.ErrNotSignalman
		case "interlocking_occupied":
			if !in.Force {
				return JoinResult{}, svcerrors.ErrInterlockingOccupied
			}
		default:
			return JoinResult{}, errors.New(decision.Reason)
		}
	}

	if current != nil && current.SignalmanUserID != in.Actor.ID {
		if in.Force {
			if d := s.sec.CanDisplace(eff, current, in.Actor.ID); !d.Allowed {
				return JoinResult{}, svcerrors.ErrNotSignalman
			}
		}
	}

	var displaced *OccupantInfo
	if current != nil && current.SignalmanUserID != in.Actor.ID {
		if user, err := s.users.FindByID(ctx, current.SignalmanUserID); err == nil {
			displaced = &OccupantInfo{UserID: user.ID, Login: user.Login}
		}
		if err := s.sessions.End(ctx, current, now); err != nil {
			return JoinResult{}, err
		}
		if s.takeover != nil {
			_ = s.takeover.ReleaseAllForSignalman(ctx, current.SignalmanUserID, "signalman_left")
		}
		s.broadcastOccupant(in.LayoutID, in.InterlockingID, nil, "displaced")
	}

	if err := s.sessions.EndAllForUser(ctx, in.Actor.ID, now); err != nil {
		return JoinResult{}, err
	}

	sess := domain.InterlockingSession{
		InterlockingID:  in.InterlockingID,
		SignalmanUserID: in.Actor.ID,
		StartedAt:       now,
	}
	if err := s.sessions.Insert(ctx, &sess); err != nil {
		return JoinResult{}, err
	}

	occupant := OccupantInfo{UserID: in.Actor.ID, Login: in.Actor.Login}
	s.broadcastOccupant(in.LayoutID, in.InterlockingID, &occupant, "joined")
	s.refreshPresence(ctx, in.LayoutID)

	return JoinResult{
		Interlocking: ilk,
		Occupant:     occupant,
		Displaced:    displaced,
	}, nil
}

func (s *InterlockingOccupancy) Leave(ctx context.Context, interlockingID, layoutID uint, actor domain.User) error {
	now := time.Now().UTC()
	sess, err := s.sessions.FindActiveByInterlocking(ctx, interlockingID)
	if err != nil {
		if errors.Is(err, repo.ErrInterlockingSessionNotFound) {
			return nil
		}
		return err
	}
	if sess.SignalmanUserID != actor.ID {
		return nil
	}
	if err := s.sessions.End(ctx, &sess, now); err != nil {
		return err
	}
	if s.takeover != nil {
		_ = s.takeover.ReleaseAllForSignalman(ctx, actor.ID, "signalman_left")
	}
	s.broadcastOccupant(layoutID, interlockingID, nil, "left")
	s.refreshPresence(ctx, layoutID)
	return nil
}

func (s *InterlockingOccupancy) broadcastOccupant(layoutID, interlockingID uint, occupant *OccupantInfo, reason string) {
	if s.hub != nil {
		s.hub.BroadcastOccupantChanged(layoutID, interlockingID, occupant, reason)
	}
}

func (s *InterlockingOccupancy) refreshPresence(ctx context.Context, layoutID uint) {
	if s.presence != nil {
		s.presence.RefreshAndBroadcast(ctx, layoutID)
	}
}
