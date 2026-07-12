package cmd

import (
	"context"
	"errors"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/security"
)

const defaultRadioReplayLimit = 200

type RadioStorePort interface {
	Append(ctx context.Context, msg domain.RadioMessage, keys []string) (string, error)
	Replay(ctx context.Context, keys []string, limit int) ([]domain.RadioMessage, error)
	ReplayLimit() int
}

type RadioAuthPort interface {
	EffectiveForUserID(ctx context.Context, userID, layoutID uint) (domain.EffectiveRoles, error)
}

type RadioHubPort interface {
	BroadcastRadioMessage(layoutID, userID uint, msg domain.RadioMessage)
}

type RadioLayoutsPort interface {
	FindByID(ctx context.Context, id uint) (domain.Layout, error)
}

// Radio orchestrates walkie-talkie send, replay and fan-out.
type Radio struct {
	store         RadioStorePort
	hub           RadioHubPort
	auth          RadioAuthPort
	layouts       RadioLayoutsPort
	vehicles      *repo.Vehicles
	trains        *repo.Trains
	ilkSessions   *repo.InterlockingSessions
	layoutIlks    *repo.LayoutInterlockings
	interlockings *repo.Interlockings
	sec           security.RadioSecurityContext
}

type RadioConfig struct {
	Store         RadioStorePort
	Hub           RadioHubPort
	Auth          RadioAuthPort
	Layouts       RadioLayoutsPort
	Vehicles      *repo.Vehicles
	Trains        *repo.Trains
	IlkSessions   *repo.InterlockingSessions
	LayoutIlks    *repo.LayoutInterlockings
	Interlockings *repo.Interlockings
}

func NewRadio(cfg RadioConfig) *Radio {
	return &Radio{
		store:         cfg.Store,
		hub:           cfg.Hub,
		auth:          cfg.Auth,
		layouts:       cfg.Layouts,
		vehicles:      cfg.Vehicles,
		trains:        cfg.Trains,
		ilkSessions:   cfg.IlkSessions,
		layoutIlks:    cfg.LayoutIlks,
		interlockings: cfg.Interlockings,
	}
}

func (s *Radio) ReplayLimit() int {
	if s == nil || s.store == nil {
		return defaultRadioReplayLimit
	}
	return s.store.ReplayLimit()
}

func (s *Radio) Send(ctx context.Context, in RadioSendInput) (domain.RadioMessage, error) {
	if s.store == nil {
		return domain.RadioMessage{}, svcerrors.ErrRadioNotConfigured
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
	if err := s.ensureRadioChatEnabled(ctx, in.LayoutID); err != nil {
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
		LayoutID:         in.LayoutID,
		FromUserID:       in.FromUserID,
		FromLogin:        in.FromLogin,
		FromOrganization: in.FromOrganization,
		Phrase:           in.Phrase,
		Note:             note,
		ContextName:      contextName,
		SentAt:           time.Now().UTC(),
	}
	if in.ToUserID != 0 {
		id := in.ToUserID
		msg.ToUserID = &id
	}
	if in.ToInterlockingID != 0 {
		id := in.ToInterlockingID
		msg.ToInterlockingID = &id
	}
	if !in.ContextVehicleID.IsZero() {
		id := in.ContextVehicleID
		msg.ContextVehicleID = &id
	}
	if !in.ContextTrainID.IsZero() {
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

func (s *Radio) ReplayInterlocking(
	ctx context.Context,
	layoutID, interlockingID, callerUserID uint,
	limit int,
) ([]domain.RadioMessage, error) {
	if s.store == nil {
		return nil, svcerrors.ErrRadioNotConfigured
	}
	if err := s.ensureRadioChatEnabled(ctx, layoutID); err != nil {
		return nil, err
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

func (s *Radio) ReplayUser(
	ctx context.Context,
	layoutID, userID uint,
	limit int,
) ([]domain.RadioMessage, error) {
	if s.store == nil {
		return nil, svcerrors.ErrRadioNotConfigured
	}
	if err := s.ensureRadioChatEnabled(ctx, layoutID); err != nil {
		return nil, err
	}
	if d := s.sec.CanReplayUser(); !d.Allowed {
		return nil, errRadioDenied(d.Reason)
	}
	key := contract.RadioUserStreamKey(layoutID, userID)
	return s.store.Replay(ctx, []string{key}, limit)
}

func (s *Radio) fanOut(msg domain.RadioMessage) {
	if s.hub == nil {
		return
	}
	s.hub.BroadcastRadioMessage(msg.LayoutID, msg.FromUserID, msg)

	if msg.ToUserID != nil {
		s.hub.BroadcastRadioMessage(msg.LayoutID, *msg.ToUserID, msg)
	}
	if msg.ToInterlockingID != nil {
		if occ := s.occupantUserID(context.Background(), *msg.ToInterlockingID); occ != 0 {
			s.hub.BroadcastRadioMessage(msg.LayoutID, occ, msg)
		}
	}
}

func (s *Radio) occupantUserID(ctx context.Context, interlockingID uint) uint {
	if s.ilkSessions == nil {
		return 0
	}
	sess, err := s.ilkSessions.FindActiveByInterlocking(ctx, interlockingID)
	if err != nil {
		return 0
	}
	return sess.SignalmanUserID
}

func (s *Radio) activeInterlockingForUser(ctx context.Context, userID uint) uint {
	if s.ilkSessions == nil || userID == 0 {
		return 0
	}
	sess, err := s.ilkSessions.FindActiveByUser(ctx, userID)
	if err != nil {
		return 0
	}
	return sess.InterlockingID
}

func (s *Radio) interlockingName(ctx context.Context, interlockingID uint) string {
	if s.interlockings == nil || interlockingID == 0 {
		return ""
	}
	row, err := s.interlockings.FindByID(ctx, interlockingID)
	if err != nil {
		return ""
	}
	return row.Name
}

func (s *Radio) interlockingStreamKeys(ctx context.Context, layoutID uint) ([]string, error) {
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

func (s *Radio) resolveContextName(ctx context.Context, vehicleID domain.VehicleID, trainID domain.TrainID) (string, error) {
	if !vehicleID.IsZero() {
		if s.vehicles == nil {
			return "", svcerrors.ErrRadioContextUnavailable
		}
		v, err := s.vehicles.FindByID(ctx, vehicleID)
		if err != nil {
			return "", err
		}
		return v.Name, nil
	}
	if !trainID.IsZero() {
		if s.trains == nil {
			return "", svcerrors.ErrRadioContextUnavailable
		}
		t, err := s.trains.FindByID(ctx, trainID)
		if err != nil {
			return "", err
		}
		return t.Name, nil
	}
	return "", domain.ErrRadioInvalidContext
}

func (s *Radio) effectiveRoles(ctx context.Context, userID, layoutID uint) (domain.EffectiveRoles, error) {
	if s.auth == nil {
		return domain.NewEffectiveRoles(), nil
	}
	return s.auth.EffectiveForUserID(ctx, userID, layoutID)
}

func (s *Radio) ensureRadioChatEnabled(ctx context.Context, layoutID uint) error {
	if s.layouts == nil {
		return nil
	}
	layout, err := s.layouts.FindByID(ctx, layoutID)
	if err != nil {
		return err
	}
	if !layout.EffectiveRadioChatEnabled() {
		return svcerrors.ErrRadioChatDisabled
	}
	return nil
}

type radioDeniedError struct{ code string }

func (e radioDeniedError) Error() string { return e.code }

func errRadioDenied(code string) error {
	return radioDeniedError{code: code}
}

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
	case errors.Is(err, svcerrors.ErrRadioChatDisabled):
		return "radio_chat_disabled"
	}
	if err != nil {
		return err.Error()
	}
	return "radio_send_failed"
}

func StreamKeysForSend(msg domain.RadioMessage, senderInterlockingID uint) []string {
	keys := make([]string, 0, 3)
	keys = append(keys, contract.RadioUserStreamKey(msg.LayoutID, msg.FromUserID))

	if msg.ToUserID != nil {
		keys = append(keys, contract.RadioUserStreamKey(msg.LayoutID, *msg.ToUserID))
		if senderInterlockingID != 0 {
			keys = append(keys, contract.RadioInterlockingStreamKey(msg.LayoutID, senderInterlockingID))
		}
	}
	if msg.ToInterlockingID != nil {
		keys = append(keys, contract.RadioInterlockingStreamKey(msg.LayoutID, *msg.ToInterlockingID))
	}
	return dedupeStrings(keys)
}

func dedupeStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
