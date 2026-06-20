package cmd

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/security"
)

const radioStopDebounce = 2 * time.Second

type RadioStopRedisPort interface {
	PublishLayoutRadioStop(ctx context.Context, layoutID uint, cmd contract.RadioStopCommandWire) error
	GetAllowedVehicles(ctx context.Context, layoutID uint) (contract.AllowedVehicles, error)
}

type RadioStopRosterPort interface {
	BuildAllowedVehiclesSnapshot(ctx context.Context, layoutID uint) (contract.AllowedVehicles, error)
}

type RadioStopHubPort interface {
	BroadcastRadioStop(layoutID uint, push contract.RadioStopPushWire)
}

// RadioStop orchestrates layout-wide Radio Stop.
type RadioStop struct {
	hub    RadioStopHubPort
	redis  RadioStopRedisPort
	roster RadioStopRosterPort
	auth   RadioAuthPort
	audit  AuditPublisher
	log    *logrus.Logger
	sec    security.RadioStopSecurityContext

	mu          sync.Mutex
	lastTrigger map[uint]time.Time
}

type RadioStopConfig struct {
	Hub    RadioStopHubPort
	Redis  RadioStopRedisPort
	Roster RadioStopRosterPort
	Auth   RadioAuthPort
	Audit  AuditPublisher
	Log    *logrus.Logger
}

func NewRadioStop(cfg RadioStopConfig) *RadioStop {
	log := cfg.Log
	if log == nil {
		log = logrus.New()
	}
	return &RadioStop{
		hub:         cfg.Hub,
		redis:       cfg.Redis,
		roster:      cfg.Roster,
		auth:        cfg.Auth,
		audit:       cfg.Audit,
		log:         log,
		lastTrigger: make(map[uint]time.Time, 4),
	}
}

func (s *RadioStop) Trigger(ctx context.Context, sess ControlSession) (bool, string) {
	if s.redis == nil || s.hub == nil {
		return false, "dcc_bus_not_configured"
	}

	roster, err := s.loadRoster(ctx, sess.LayoutID())
	if err != nil {
		s.log.WithError(err).Warn("radio stop: load roster")
		return false, "dcc_bus_unavailable"
	}

	eff, err := s.effectiveRoles(ctx, sess.UserID(), sess.LayoutID())
	if err != nil {
		s.log.WithError(err).Warn("radio stop: effective roles")
		return false, "not_authorized_to_drive"
	}
	if d := s.sec.CanTrigger(eff, sess.UserID(), roster); !d.Allowed {
		return false, d.Reason
	}

	if s.debounced(sess.LayoutID()) {
		s.log.WithField("layoutId", sess.LayoutID()).Debug("radio stop: debounced")
		return true, ""
	}

	cmd := contract.RadioStopCommandWire{
		TriggeredByUserID: sess.UserID(),
		TriggeredByLogin:  sess.Login(),
		At:                time.Now().UTC().UnixMilli(),
	}
	if err := s.redis.PublishLayoutRadioStop(ctx, sess.LayoutID(), cmd); err != nil {
		s.log.WithError(err).Warn("radio stop: publish")
		return false, "dcc_bus_unavailable"
	}

	push := contract.RadioStopPushWire{At: cmd.At}
	push.TriggeredBy.UserID = sess.UserID()
	push.TriggeredBy.Login = sess.Login()
	s.hub.BroadcastRadioStop(sess.LayoutID(), push)

	s.log.WithFields(logrus.Fields{
		"layoutId":    sess.LayoutID(),
		"triggeredBy": sess.Login(),
		"userId":      sess.UserID(),
	}).Info("radio stop triggered")

	if s.audit != nil {
		_ = s.audit.Publish(ctx, sess.LayoutID(), AuditActor{UserID: sess.UserID(), Login: sess.Login()},
			"audit_radio_stop", nil)
	}

	return true, ""
}

func (s *RadioStop) effectiveRoles(ctx context.Context, userID, layoutID uint) (domain.EffectiveRoles, error) {
	if s.auth == nil {
		return domain.NewEffectiveRoles(), nil
	}
	return s.auth.EffectiveForUserID(ctx, userID, layoutID)
}

func (s *RadioStop) loadRoster(ctx context.Context, layoutID uint) (contract.AllowedVehicles, error) {
	if snap, err := s.redis.GetAllowedVehicles(ctx, layoutID); err == nil && len(snap.Vehicles) > 0 {
		return snap, nil
	}
	if s.roster == nil {
		return contract.AllowedVehicles{}, nil
	}
	return s.roster.BuildAllowedVehiclesSnapshot(ctx, layoutID)
}

func (s *RadioStop) debounced(layoutID uint) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if last, ok := s.lastTrigger[layoutID]; ok && now.Sub(last) < radioStopDebounce {
		return true
	}
	s.lastTrigger[layoutID] = now
	return false
}
