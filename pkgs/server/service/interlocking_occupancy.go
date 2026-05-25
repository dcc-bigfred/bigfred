package service

import (
	"context"
	"errors"
	"time"

	"github.com/keskad/loco/pkgs/server/domain"
	"github.com/keskad/loco/pkgs/server/repo"
	"github.com/keskad/loco/pkgs/server/security"
	"github.com/keskad/loco/pkgs/server/ws"
)

var (
	ErrInterlockingOccupied     = errors.New("interlocking_occupied")
	ErrInterlockingNotInLayout  = errors.New("interlocking_not_in_layout")
	ErrNotSignalman             = errors.New("not_signalman")
)

// InterlockingWithOccupant pairs a catalogue row with its current
// occupant, if any.
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

// InterlockingOccupancyService handles join/leave and occupant
// enrichment for the layout dashboard.
type InterlockingOccupancyService struct {
	interlockings       *repo.Interlockings
	layoutInterlockings *repo.LayoutInterlockings
	sessions            *repo.InterlockingSessions
	users               *repo.Users
	auth                *AuthService
	hub                 *ws.Hub
	presence            *PresenceService
	sec                 security.InterlockingSecurityContext
}

// NewInterlockingOccupancyService constructs the service.
func NewInterlockingOccupancyService(
	interlockings *repo.Interlockings,
	layoutInterlockings *repo.LayoutInterlockings,
	sessions *repo.InterlockingSessions,
	users *repo.Users,
	auth *AuthService,
	hub *ws.Hub,
	presence *PresenceService,
) *InterlockingOccupancyService {
	return &InterlockingOccupancyService{
		interlockings:       interlockings,
		layoutInterlockings: layoutInterlockings,
		sessions:            sessions,
		users:               users,
		auth:                auth,
		hub:                 hub,
		presence:            presence,
	}
}

// GetForLayout returns one whitelisted interlocking enriched with
// occupant login. Returns ErrInterlockingNotInLayout when the row is
// not whitelisted for the caller's layout (the dashboard / details
// page only show layout-scoped boxes).
func (s *InterlockingOccupancyService) GetForLayout(
	ctx context.Context,
	layoutID, interlockingID uint,
) (InterlockingWithOccupant, error) {
	whitelisted, err := s.layoutInterlockings.Exists(ctx, layoutID, interlockingID)
	if err != nil {
		return InterlockingWithOccupant{}, err
	}
	if !whitelisted {
		return InterlockingWithOccupant{}, ErrInterlockingNotInLayout
	}
	row, err := s.interlockings.FindByID(ctx, interlockingID)
	if err != nil {
		if errors.Is(err, repo.ErrInterlockingNotFound) {
			return InterlockingWithOccupant{}, ErrInterlockingNotFound
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

// ListForLayout returns whitelisted interlockings enriched with
// occupant login.
func (s *InterlockingOccupancyService) ListForLayout(ctx context.Context, layoutID uint) ([]InterlockingWithOccupant, error) {
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

// Join staffs an interlocking for the caller.
func (s *InterlockingOccupancyService) Join(ctx context.Context, in JoinInput) (JoinResult, error) {
	now := time.Now().UTC()

	ilk, err := s.interlockings.FindByID(ctx, in.InterlockingID)
	if err != nil {
		if errors.Is(err, repo.ErrInterlockingNotFound) {
			return JoinResult{}, ErrInterlockingNotFound
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
			return JoinResult{}, ErrInterlockingNotInLayout
		case "not_signalman":
			return JoinResult{}, ErrNotSignalman
		case "interlocking_occupied":
			if !in.Force {
				return JoinResult{}, ErrInterlockingOccupied
			}
		default:
			return JoinResult{}, errors.New(decision.Reason)
		}
	}

	if current != nil && current.SignalmanUserID != in.Actor.ID {
		if in.Force {
			if d := s.sec.CanDisplace(eff, current, in.Actor.ID); !d.Allowed {
				return JoinResult{}, ErrNotSignalman
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
	s.presence.RefreshAndBroadcast(ctx, in.LayoutID)

	return JoinResult{
		Interlocking: ilk,
		Occupant:     occupant,
		Displaced:    displaced,
	}, nil
}

// Leave ends the caller's active session for an interlocking.
func (s *InterlockingOccupancyService) Leave(ctx context.Context, interlockingID, layoutID uint, actor domain.User) error {
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
	s.broadcastOccupant(layoutID, interlockingID, nil, "left")
	s.presence.RefreshAndBroadcast(ctx, layoutID)
	return nil
}

func (s *InterlockingOccupancyService) broadcastOccupant(layoutID, interlockingID uint, occupant *OccupantInfo, reason string) {
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
	s.hub.BroadcastToLayout(layoutID, "interlocking.occupantChanged", payload)
}
