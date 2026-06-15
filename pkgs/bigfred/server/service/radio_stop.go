package service

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/security"
	"github.com/keskad/loco/pkgs/bigfred/server/ws"
)

const radioStopDebounce = 2 * time.Second

// RadioStopService orchestrates layout-wide Radio Stop (§4.6).
type RadioStopService struct {
	hub    *ws.Hub
	redis  *RedisService
	roster *LayoutVehicleService
	auth   *AuthService
	log    *logrus.Logger
	sec    security.RadioStopSecurityContext

	mu          sync.Mutex
	lastTrigger map[uint]time.Time
}

// RadioStopConfig wires RadioStopService dependencies.
type RadioStopConfig struct {
	Hub    *ws.Hub
	Redis  *RedisService
	Roster *LayoutVehicleService
	Auth   *AuthService
	Log    *logrus.Logger
}

// NewRadioStopService returns a ready orchestrator. hub and redis may
// be nil in tests that only exercise authorization.
func NewRadioStopService(cfg RadioStopConfig) *RadioStopService {
	log := cfg.Log
	if log == nil {
		log = logrus.New()
	}
	return &RadioStopService{
		hub:         cfg.Hub,
		redis:       cfg.Redis,
		roster:      cfg.Roster,
		auth:        cfg.Auth,
		log:         log,
		lastTrigger: make(map[uint]time.Time, 4),
	}
}

// Trigger runs the layout-wide halt for the calling session. Returns
// (true, "") on success, (false, machine-readable code) on failure.
func (s *RadioStopService) Trigger(ctx context.Context, sess *ws.DriveSession) (bool, string) {
	if s.redis == nil || s.hub == nil {
		return false, "dcc_bus_not_configured"
	}

	roster, err := s.loadRoster(ctx, sess.LayoutID)
	if err != nil {
		s.log.WithError(err).Warn("radio stop: load roster")
		return false, "dcc_bus_unavailable"
	}

	eff, err := s.effectiveRoles(ctx, sess.UserID, sess.LayoutID)
	if err != nil {
		s.log.WithError(err).Warn("radio stop: effective roles")
		return false, "not_authorized_to_drive"
	}
	if d := s.sec.CanTrigger(eff, sess.UserID, roster); !d.Allowed {
		return false, d.Reason
	}

	if s.debounced(sess.LayoutID) {
		s.log.WithField("layoutId", sess.LayoutID).Debug("radio stop: debounced")
		return true, ""
	}

	cmd := contract.RadioStopCommandWire{
		TriggeredByUserID: sess.UserID,
		TriggeredByLogin:  sess.Login,
		At:                time.Now().UTC().UnixMilli(),
	}
	if err := s.redis.PublishLayoutRadioStop(ctx, sess.LayoutID, cmd); err != nil {
		s.log.WithError(err).Warn("radio stop: publish")
		return false, "dcc_bus_unavailable"
	}

	push := contract.RadioStopPushWire{At: cmd.At}
	push.TriggeredBy.UserID = sess.UserID
	push.TriggeredBy.Login = sess.Login
	s.hub.BroadcastToLayout(sess.LayoutID, ws.TypeSystemRadioStop, push)

	s.log.WithFields(logrus.Fields{
		"layoutId":    sess.LayoutID,
		"triggeredBy": sess.Login,
		"userId":      sess.UserID,
	}).Info("radio stop triggered")

	return true, ""
}

func (s *RadioStopService) effectiveRoles(ctx context.Context, userID, layoutID uint) (domain.EffectiveRoles, error) {
	if s.auth == nil {
		return domain.NewEffectiveRoles(), nil
	}
	return s.auth.EffectiveForUserID(ctx, userID, layoutID)
}

func (s *RadioStopService) loadRoster(ctx context.Context, layoutID uint) (contract.AllowedVehicles, error) {
	if snap, err := s.redis.GetAllowedVehicles(ctx, layoutID); err == nil && len(snap.Vehicles) > 0 {
		return snap, nil
	}
	if s.roster == nil {
		return contract.AllowedVehicles{}, nil
	}
	return s.roster.buildAllowedVehiclesSnapshot(ctx, layoutID)
}

func (s *RadioStopService) debounced(layoutID uint) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if last, ok := s.lastTrigger[layoutID]; ok && now.Sub(last) < radioStopDebounce {
		return true
	}
	s.lastTrigger[layoutID] = now
	return false
}
