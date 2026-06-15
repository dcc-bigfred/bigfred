package service

import (
	"context"
	"errors"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/security"
	"github.com/keskad/loco/pkgs/bigfred/server/ws"
)

// RadioService orchestrates walkie-talkie send, replay and fan-out (§4.4).
type RadioService struct {
	store      *RadioStore
	hub        *ws.Hub
	auth       *AuthService
	vehicles   *repo.Vehicles
	trains     *repo.Trains
	ilkSessions *repo.InterlockingSessions
	layoutIlks  *repo.LayoutInterlockings
	interlockings *repo.Interlockings
	sec        security.RadioSecurityContext
}

// RadioConfig wires RadioService dependencies.
type RadioConfig struct {
	Store       *RadioStore
	Hub         *ws.Hub
	Auth        *AuthService
	Vehicles    *repo.Vehicles
	Trains      *repo.Trains
	IlkSessions   *repo.InterlockingSessions
	LayoutIlks    *repo.LayoutInterlockings
	Interlockings *repo.Interlockings
}

// NewRadioService returns a ready orchestrator.
func NewRadioService(cfg RadioConfig) *RadioService {
	return &RadioService{
		store:       cfg.Store,
		hub:         cfg.Hub,
		auth:        cfg.Auth,
		vehicles:    cfg.Vehicles,
		trains:      cfg.Trains,
		ilkSessions: cfg.IlkSessions,
		layoutIlks:  cfg.LayoutIlks,
		interlockings: cfg.Interlockings,
	}
}

// SendInput is the validated send request from WS or REST shim.
type SendInput struct {
	LayoutID         uint
	FromUserID       uint
	FromLogin        string
	ToUserID         uint
	ToInterlockingID uint
	ContextVehicleID uint
	ContextTrainID   uint
	Phrase           domain.RadioPhrase
	Note             string
}

// ReplayLimit returns the configured replay cap.
func (s *RadioService) ReplayLimit() int {
	if s == nil || s.store == nil {
		return defaultRadioReplayLimit
	}
	return s.store.ReplayLimit()
}

// Send validates, persists and fans out a radio message.
func (s *RadioService) Send(ctx context.Context, in SendInput) (domain.RadioMessage, error) {
	if s.store == nil {
		return domain.RadioMessage{}, errors.New("radio_not_configured")
	}
	if err := domain.ValidateTarget(in.ToUserID, in.ToInterlockingID); err != nil {
		return domain.RadioMessage{}, err
	}
	if err := domain.ValidateContext(in.ContextVehicleID, in.ContextTrainID); err != nil {
		return domain.RadioMessage{}, err
	}
	if !domain.IsValidRadioPhrase(in.Phrase) {
		return domain.RadioMessage{}, domain.ErrRadioInvalidPhrase
	}
	note, err := domain.ValidateNote(in.Note)
	if err != nil {
		return domain.RadioMessage{}, err
	}

	eff, err := s.effectiveRoles(ctx, in.FromUserID, in.LayoutID)
	if err != nil {
		return domain.RadioMessage{}, err
	}
	if d := s.sec.CanSend(eff, in.ToUserID, in.ToInterlockingID); !d.Allowed {
		return domain.RadioMessage{}, errRadioDenied(d.Reason)
	}

	contextName, err := s.resolveContextName(ctx, in.ContextVehicleID, in.ContextTrainID)
	if err != nil {
		return domain.RadioMessage{}, err
	}

	msg := domain.RadioMessage{
		LayoutID:    in.LayoutID,
		FromUserID:  in.FromUserID,
		FromLogin:   in.FromLogin,
		Phrase:      in.Phrase,
		Note:        note,
		ContextName: contextName,
		SentAt:      time.Now().UTC(),
	}
	if in.ToUserID != 0 {
		id := in.ToUserID
		msg.ToUserID = &id
	}
	if in.ToInterlockingID != 0 {
		id := in.ToInterlockingID
		msg.ToInterlockingID = &id
	}
	if in.ContextVehicleID != 0 {
		id := in.ContextVehicleID
		msg.ContextVehicleID = &id
	}
	if in.ContextTrainID != 0 {
		id := in.ContextTrainID
		msg.ContextTrainID = &id
	}

	senderIlk := s.activeInterlockingForUser(ctx, in.FromUserID)
	if senderIlk != 0 {
		msg.FromInterlockingID = &senderIlk
		msg.FromInterlockingName = s.interlockingName(ctx, senderIlk)
	}
	keys := StreamKeysForSend(msg, senderIlk)
	streamID, err := s.store.Append(ctx, msg, keys)
	if err != nil {
		return domain.RadioMessage{}, err
	}
	msg.ID = streamID

	s.fanOut(msg)
	return msg, nil
}

// ReplayInterlocking returns the merged group-chat history visible to
// the active occupant of interlockingID (§4.4.3).
func (s *RadioService) ReplayInterlocking(
	ctx context.Context,
	layoutID, interlockingID, callerUserID uint,
	limit int,
) ([]domain.RadioMessage, error) {
	if s.store == nil {
		return nil, errors.New("radio_not_configured")
	}
	eff, err := s.effectiveRoles(ctx, callerUserID, layoutID)
	if err != nil {
		return nil, err
	}
	occupantID := uint(0)
	if sess, err := s.ilkSessions.FindActiveByInterlocking(ctx, interlockingID); err == nil {
		occupantID = sess.SignalmanUserID
	}
	if d := s.sec.CanReplayInterlocking(eff, interlockingID, occupantID, callerUserID); !d.Allowed {
		return nil, errRadioDenied(d.Reason)
	}
	keys, err := s.interlockingStreamKeys(ctx, layoutID)
	if err != nil {
		return nil, err
	}
	return s.store.Replay(ctx, keys, limit)
}

// ReplayUser returns the caller's personal radio history (§4.4.3).
func (s *RadioService) ReplayUser(
	ctx context.Context,
	layoutID, userID uint,
	limit int,
) ([]domain.RadioMessage, error) {
	if s.store == nil {
		return nil, errors.New("radio_not_configured")
	}
	if d := s.sec.CanReplayUser(); !d.Allowed {
		return nil, errRadioDenied(d.Reason)
	}
	key := contract.RadioUserStreamKey(layoutID, userID)
	return s.store.Replay(ctx, []string{key}, limit)
}

func (s *RadioService) fanOut(msg domain.RadioMessage) {
	if s.hub == nil {
		return
	}
	wire := contract.MessageWireFromDomain(msg)
	s.hub.BroadcastToUserInLayout(msg.LayoutID, msg.FromUserID, contract.TypeRadioMessage, wire)

	if msg.ToUserID != nil {
		s.hub.BroadcastToUserInLayout(msg.LayoutID, *msg.ToUserID, contract.TypeRadioMessage, wire)
	}
	if msg.ToInterlockingID != nil {
		if occ := s.occupantUserID(context.Background(), *msg.ToInterlockingID); occ != 0 {
			s.hub.BroadcastToUserInLayout(msg.LayoutID, occ, contract.TypeRadioMessage, wire)
		}
	}
}

func (s *RadioService) occupantUserID(ctx context.Context, interlockingID uint) uint {
	if s.ilkSessions == nil {
		return 0
	}
	sess, err := s.ilkSessions.FindActiveByInterlocking(ctx, interlockingID)
	if err != nil {
		return 0
	}
	return sess.SignalmanUserID
}

func (s *RadioService) activeInterlockingForUser(ctx context.Context, userID uint) uint {
	if s.ilkSessions == nil || userID == 0 {
		return 0
	}
	sess, err := s.ilkSessions.FindActiveByUser(ctx, userID)
	if err != nil {
		return 0
	}
	return sess.InterlockingID
}

func (s *RadioService) interlockingName(ctx context.Context, interlockingID uint) string {
	if s.interlockings == nil || interlockingID == 0 {
		return ""
	}
	row, err := s.interlockings.FindByID(ctx, interlockingID)
	if err != nil {
		return ""
	}
	return row.Name
}

func (s *RadioService) interlockingStreamKeys(ctx context.Context, layoutID uint) ([]string, error) {
	if s.layoutIlks == nil {
		return nil, nil
	}
	ids, err := s.layoutIlks.InterlockingIDsForLayout(ctx, layoutID)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(ids))
	for _, id := range ids {
		keys = append(keys, contract.RadioInterlockingStreamKey(layoutID, id))
	}
	return keys, nil
}

func (s *RadioService) resolveContextName(ctx context.Context, vehicleID, trainID uint) (string, error) {
	if vehicleID != 0 {
		if s.vehicles == nil {
			return "", errors.New("radio_context_unavailable")
		}
		v, err := s.vehicles.FindByID(ctx, vehicleID)
		if err != nil {
			return "", err
		}
		return v.Name, nil
	}
	if trainID != 0 {
		if s.trains == nil {
			return "", errors.New("radio_context_unavailable")
		}
		t, err := s.trains.FindByID(ctx, trainID)
		if err != nil {
			return "", err
		}
		return t.Name, nil
	}
	return "", domain.ErrRadioInvalidContext
}

func (s *RadioService) effectiveRoles(ctx context.Context, userID, layoutID uint) (domain.EffectiveRoles, error) {
	if s.auth == nil {
		return domain.NewEffectiveRoles(), nil
	}
	return s.auth.EffectiveForUserID(ctx, userID, layoutID)
}

type radioDeniedError struct{ code string }

func (e radioDeniedError) Error() string { return e.code }

func errRadioDenied(code string) error {
	return radioDeniedError{code: code}
}

// RadioDeniedCode extracts the machine-readable denial reason.
func RadioDeniedCode(err error) string {
	var d radioDeniedError
	if errors.As(err, &d) {
		return d.code
	}
	switch {
	case errors.Is(err, domain.ErrRadioInvalidTarget):
		return "radio_invalid_target"
	case errors.Is(err, domain.ErrRadioInvalidContext):
		return "radio_invalid_context"
	case errors.Is(err, domain.ErrRadioInvalidPhrase):
		return "radio_invalid_phrase"
	case errors.Is(err, domain.ErrRadioNoteTooLong):
		return "radio_note_too_long"
	}
	if err != nil {
		return err.Error()
	}
	return "radio_send_failed"
}
