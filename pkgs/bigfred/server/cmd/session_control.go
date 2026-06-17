package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
)

const (
	TypeSessionSetCommandStation            = "session.setCommandStation"
	TypeSessionOpened                       = "session.opened"
	TypeSessionCommandStationChanged        = "session.commandStationChanged"
	TypeSessionCommandStationCatalogChanged = "session.commandStationCatalogChanged"
	TypeSystemRadioStop                     = "system.radioStop"
	TypeSystemEStopTarget                   = "system.estopTarget"
)

type SessionSetCommandStationRequest struct {
	CommandStationID uint `json:"commandStationId"`
}

type SessionCommandStationChangedPayload struct {
	CommandStationID uint    `json:"commandStationId"`
	WSURL            *string `json:"wsUrl"`
	Status           string  `json:"status"`
	Reason           string  `json:"reason,omitempty"`
}

type CommandStationCatalogChangedPayload struct {
	CommandStationID uint                      `json:"commandStationId"`
	Name             string                    `json:"name"`
	Kind             domain.CommandStationKind `json:"kind"`
	SpeedSteps       uint                      `json:"speedSteps"`
}

type OpenedSessionPayload struct {
	SessionID                string                           `json:"sessionId"`
	LayoutID                 uint                             `json:"layoutId"`
	AvailableCommandStations []AvailableCommandStationPayload `json:"availableCommandStations"`
	CurrentSession           *CurrentSessionPayload           `json:"currentSession,omitempty"`
}

type AvailableCommandStationPayload struct {
	ID         uint                      `json:"id"`
	Name       string                    `json:"name"`
	Kind       domain.CommandStationKind `json:"kind"`
	SpeedSteps uint                      `json:"speedSteps"`
	WSURL      *string                   `json:"wsUrl"`
}

type CurrentSessionPayload struct {
	CommandStationID uint `json:"commandStationId"`
}

// ControlEnvelope is the transport-neutral WS envelope shape.
type ControlEnvelope struct {
	Type    string
	ID      string
	Payload json.RawMessage
}

// ControlSession exposes the mutable session state SessionControl needs.
type ControlSession interface {
	SessionID() string
	UserID() uint
	Login() string
	LayoutID() uint
	CurrentCommandStation() uint
	SetCommandStation(commandStationID uint) uint
}

// ControlClient is the outbound surface SessionControl needs from ws.Client.
type ControlClient interface {
	Session() ControlSession
	SendTyped(eventType string, payload any)
	SendAck(requestID string, ok bool, errCode string)
}

// SessionControl owns control-plane session actions.
type SessionControl struct {
	log         *logrus.Logger
	dccBus      DccBusControlPort
	radioStop   RadioStopControlPort
	estopTarget EStopTargetControlPort
	cs          *repo.CommandStations
	layoutCS    *repo.LayoutCommandStations
	layoutRows  *repo.Layouts

	proxyPathFn func(csID uint) string

	mu       sync.Mutex
	sessions map[ControlClient]struct{}
}

// SessionControlConfig groups dependencies.
type SessionControlConfig struct {
	Log         *logrus.Logger
	DccBus      DccBusControlPort
	RadioStop   RadioStopControlPort
	EStopTarget EStopTargetControlPort
	CommandStns *repo.CommandStations
	LayoutCS    *repo.LayoutCommandStations
	Layouts     *repo.Layouts
}

// NewSessionControl returns a ready control-plane use case.
func NewSessionControl(cfg SessionControlConfig) *SessionControl {
	log := cfg.Log
	if log == nil {
		log = logrus.New()
	}
	return &SessionControl{
		log:         log,
		dccBus:      cfg.DccBus,
		radioStop:   cfg.RadioStop,
		estopTarget: cfg.EStopTarget,
		cs:          cfg.CommandStns,
		layoutCS:    cfg.LayoutCS,
		layoutRows:  cfg.Layouts,
		proxyPathFn: defaultSessionProxyPath,
		sessions:    make(map[ControlClient]struct{}, 8),
	}
}

func defaultSessionProxyPath(csID uint) string {
	return fmt.Sprintf("/api/v1/dcc-bus/%d/ws", csID)
}

// Sessions returns a snapshot of every live control-plane client.
func (s *SessionControl) Sessions() []ControlClient {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ControlClient, 0, len(s.sessions))
	for c := range s.sessions {
		out = append(out, c)
	}
	return out
}

// HandleOpened registers a live control-plane client and sends session.opened.
func (s *SessionControl) HandleOpened(ctx context.Context, c ControlClient) {
	s.mu.Lock()
	s.sessions[c] = struct{}{}
	s.mu.Unlock()

	c.SendTyped(TypeSessionOpened, s.openedPayload(ctx, c))
}

// HandleClosed drops a live control-plane client.
func (s *SessionControl) HandleClosed(_ context.Context, c ControlClient) {
	s.mu.Lock()
	delete(s.sessions, c)
	s.mu.Unlock()
}

// HandleEnvelope dispatches inbound control-plane frames.
func (s *SessionControl) HandleEnvelope(ctx context.Context, c ControlClient, env ControlEnvelope) {
	switch env.Type {
	case TypeSessionSetCommandStation:
		var p SessionSetCommandStationRequest
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			c.SendAck(env.ID, false, "bad_payload")
			return
		}
		s.handleSetCS(ctx, c, p, env.ID)

	case TypeSystemRadioStop:
		if s.radioStop == nil {
			c.SendAck(env.ID, false, "dcc_bus_not_configured")
			return
		}
		ok, code := s.radioStop.Trigger(ctx, c.Session())
		c.SendAck(env.ID, ok, code)

	case TypeSystemEStopTarget:
		if s.estopTarget == nil {
			c.SendAck(env.ID, false, "dcc_bus_not_configured")
			return
		}
		var p contract.EStopTargetPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			c.SendAck(env.ID, false, "bad_payload")
			return
		}
		ok, code := s.estopTarget.Trigger(ctx, c.Session(), p.Target, p.TargetID)
		c.SendAck(env.ID, ok, code)
	}
}

func ptr(s string) *string { return &s }

func (s *SessionControl) handleSetCS(
	ctx context.Context,
	c ControlClient,
	p SessionSetCommandStationRequest,
	requestID string,
) {
	sess := c.Session()
	if s.layoutCS == nil || s.cs == nil || s.layoutRows == nil {
		c.SendAck(requestID, false, "dcc_bus_not_configured")
		return
	}
	if p.CommandStationID == 0 {
		sess.SetCommandStation(0)
		c.SendAck(requestID, true, "")
		s.broadcastChangeForUser(sess.LayoutID(), sess.UserID(), SessionCommandStationChangedPayload{
			CommandStationID: 0,
			WSURL:            nil,
			Status:           "stopped",
		})
		return
	}

	if !s.commandStationAttached(ctx, sess.LayoutID(), p.CommandStationID) {
		c.SendAck(requestID, false, "command_station_not_attached")
		return
	}
	if s.dccBus == nil {
		c.SendAck(requestID, false, "dcc_bus_not_configured")
		return
	}

	s.broadcastChangeForUser(sess.LayoutID(), sess.UserID(), SessionCommandStationChangedPayload{
		CommandStationID: p.CommandStationID,
		WSURL:            nil,
		Status:           "starting",
		Reason:           "spawning",
	})

	port, _, err := s.dccBus.EnsureRunning(ctx, sess.LayoutID(), p.CommandStationID)
	if err != nil {
		s.log.WithError(err).Warn("dcc-bus ensure running")
		code := "dcc_bus_unavailable"
		if errors.Is(err, svcerrors.ErrNoDCCBusPortsAvailable) {
			code = svcerrors.CodeNoDCCBusPortsAvailable
		}
		c.SendAck(requestID, false, code)
		s.broadcastChangeForUser(sess.LayoutID(), sess.UserID(), SessionCommandStationChangedPayload{
			CommandStationID: p.CommandStationID,
			WSURL:            nil,
			Status:           "degraded",
			Reason:           code,
		})
		return
	}

	sess.SetCommandStation(p.CommandStationID)
	wsURL := s.proxyPathFn(p.CommandStationID)
	if !s.dccBus.ProxyEnabled() {
		wsURL = fmt.Sprintf("ws://127.0.0.1:%d/ws", port)
	}

	c.SendAck(requestID, true, "")
	s.broadcastChangeForUser(sess.LayoutID(), sess.UserID(), SessionCommandStationChangedPayload{
		CommandStationID: p.CommandStationID,
		WSURL:            ptr(wsURL),
		Status:           "running",
	})
}

// BroadcastCommandStationCatalogChanged notifies live sessions that expose cs.
func (s *SessionControl) BroadcastCommandStationCatalogChanged(ctx context.Context, cs domain.CommandStation) {
	if s.layoutCS == nil || s.layoutRows == nil || s.cs == nil {
		return
	}
	payload := CommandStationCatalogChangedPayload{
		CommandStationID: cs.ID,
		Name:             cs.Name,
		Kind:             cs.Kind,
		SpeedSteps:       cs.SpeedSteps,
	}
	for _, c := range s.Sessions() {
		sess := c.Session()
		if !s.commandStationAttached(ctx, sess.LayoutID(), cs.ID) {
			continue
		}
		c.SendTyped(TypeSessionCommandStationCatalogChanged, payload)
	}
}

func (s *SessionControl) broadcastChangeForUser(layoutID, userID uint, payload SessionCommandStationChangedPayload) {
	for _, c := range s.Sessions() {
		sess := c.Session()
		if sess.LayoutID() != layoutID || sess.UserID() != userID {
			continue
		}
		c.SendTyped(TypeSessionCommandStationChanged, payload)
	}
}

func (s *SessionControl) openedPayload(ctx context.Context, c ControlClient) OpenedSessionPayload {
	sess := c.Session()
	out := OpenedSessionPayload{
		SessionID: sess.SessionID(),
		LayoutID:  sess.LayoutID(),
	}
	if s.layoutCS == nil || s.cs == nil || s.layoutRows == nil {
		return out
	}
	stations, err := s.listAvailableStations(ctx, sess.LayoutID())
	if err != nil {
		s.log.WithError(err).Warn("session.opened: list cs by ids")
		return out
	}
	out.AvailableCommandStations = make([]AvailableCommandStationPayload, 0, len(stations))
	for _, st := range stations {
		var wsURL *string
		if s.dccBus != nil && s.dccBus.PortFor(sess.LayoutID(), st.ID) != 0 {
			port := s.dccBus.PortFor(sess.LayoutID(), st.ID)
			if s.dccBus.ProxyEnabled() {
				url := s.proxyPathFn(st.ID)
				wsURL = &url
			} else {
				url := fmt.Sprintf("ws://127.0.0.1:%d/ws", port)
				wsURL = &url
			}
		}
		out.AvailableCommandStations = append(out.AvailableCommandStations, AvailableCommandStationPayload{
			ID:         st.ID,
			Name:       st.Name,
			Kind:       st.Kind,
			SpeedSteps: st.SpeedSteps,
			WSURL:      wsURL,
		})
	}
	if cur := sess.CurrentCommandStation(); cur != 0 {
		out.CurrentSession = &CurrentSessionPayload{CommandStationID: cur}
	}
	return out
}

func (s *SessionControl) listAvailableStations(ctx context.Context, layoutID uint) ([]domain.CommandStation, error) {
	layout, err := s.layoutRows.FindByID(ctx, layoutID)
	if err != nil {
		return nil, err
	}
	if layout.IsSystem {
		return s.cs.ListAll(ctx)
	}
	rows, err := s.layoutCS.ListByLayout(ctx, layoutID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	ids := make([]uint, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.CommandStationID)
	}
	return s.cs.ListByIDs(ctx, ids)
}

func (s *SessionControl) commandStationAttached(ctx context.Context, layoutID, commandStationID uint) bool {
	layout, err := s.layoutRows.FindByID(ctx, layoutID)
	if err != nil {
		return false
	}
	if layout.IsSystem {
		_, err := s.cs.FindByID(ctx, commandStationID)
		return err == nil
	}
	_, err = s.layoutCS.Find(ctx, layoutID, commandStationID)
	return err == nil
}
