package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/server/domain"
	"github.com/keskad/loco/pkgs/server/repo"
	"github.com/keskad/loco/pkgs/server/ws"
)

// SessionControlService implements ws.ControlHandler. It owns the
// control-plane logic that used to be inlined in Client.readLoop:
// session-scoped command-station selection, the lazy-spawn handshake
// with DccBusService and the session.opened payload (§7e.6).
type SessionControlService struct {
	log        *logrus.Logger
	dccBus     *DccBusService
	cs         *repo.CommandStations
	layoutCS   *repo.LayoutCommandStations
	layoutRows *repo.Layouts

	// proxyPathFn turns (csID) into the URL the SPA should dial.
	// Default returns "/api/v1/dcc-bus/<csId>/ws"; tests may override.
	proxyPathFn func(csID uint) string

	mu       sync.Mutex
	sessions map[*ws.Client]struct{}
}

// SessionControlConfig groups dependencies.
type SessionControlConfig struct {
	Log         *logrus.Logger
	DccBus      *DccBusService
	CommandStns *repo.CommandStations
	LayoutCS    *repo.LayoutCommandStations
	Layouts     *repo.Layouts
}

// NewSessionControlService returns a ready handler. dccBus may be
// nil at construction time (e.g. --no-supervisor); EnsureRunning
// calls then fail fast with `dcc_bus_not_configured`.
func NewSessionControlService(cfg SessionControlConfig) *SessionControlService {
	log := cfg.Log
	if log == nil {
		log = logrus.New()
	}
	return &SessionControlService{
		log:        log,
		dccBus:     cfg.DccBus,
		cs:         cfg.CommandStns,
		layoutCS:   cfg.LayoutCS,
		layoutRows: cfg.Layouts,
		proxyPathFn: defaultProxyPath,
		sessions:   make(map[*ws.Client]struct{}, 8),
	}
}

func defaultProxyPath(csID uint) string {
	return fmt.Sprintf("/api/v1/dcc-bus/%d/ws", csID)
}

// Sessions returns a snapshot of every live control-plane client.
// Used by the dcc-bus event consumer to fan messages out (§7e.5).
func (s *SessionControlService) Sessions() []*ws.Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*ws.Client, 0, len(s.sessions))
	for c := range s.sessions {
		out = append(out, c)
	}
	return out
}

// HandleOpened sends the session.opened welcome envelope with the
// list of available command stations on the user's layout.
func (s *SessionControlService) HandleOpened(ctx context.Context, c *ws.Client) {
	s.mu.Lock()
	s.sessions[c] = struct{}{}
	s.mu.Unlock()

	c.SendTyped("session.opened", s.openedPayload(ctx, c))
}

// HandleClosed drops the client from the live set.
func (s *SessionControlService) HandleClosed(_ context.Context, c *ws.Client) {
	s.mu.Lock()
	delete(s.sessions, c)
	s.mu.Unlock()
}

// HandleEnvelope dispatches inbound frames to the right action.
// Unknown types are dropped silently — the hub already ack'd them
// implicitly by accepting the WS upgrade.
func (s *SessionControlService) HandleEnvelope(ctx context.Context, c *ws.Client, env ws.Envelope) {
	switch env.Type {
	case "session.setCommandStation":
		var p sessionSetCSPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			c.SendAck(env.ID, false, "bad_payload")
			return
		}
		s.handleSetCS(ctx, c, p, env.ID)
	}
}

type sessionSetCSPayload struct {
	CommandStationID uint `json:"commandStationId"`
}

// sessionCommandStationChangedPayload is the broadcast event the
// server emits on every session of the user when their cs selection
// changes. `wsUrl` is nil during the spawning phase; the second
// emission carries the final URL once the daemon is dial-able.
type sessionCommandStationChangedPayload struct {
	CommandStationID uint    `json:"commandStationId"`
	WSURL            *string `json:"wsUrl"`
	Status           string  `json:"status"`
	Reason           string  `json:"reason,omitempty"`
}

func ptr(s string) *string { return &s }

// handleSetCS validates the requested cs against the layout roster,
// orchestrates the lazy spawn with DccBusService, and broadcasts the
// intermediate "spawning" + final "running" events on the user's
// other tabs.
func (s *SessionControlService) handleSetCS(ctx context.Context, c *ws.Client, p sessionSetCSPayload, requestID string) {
	sess := c.Session()
	if s.layoutCS == nil || s.cs == nil || s.layoutRows == nil {
		c.SendAck(requestID, false, "dcc_bus_not_configured")
		return
	}
	if p.CommandStationID == 0 {
		sess.SetCommandStation(0)
		c.SendAck(requestID, true, "")
		s.broadcastChangeForUser(sess.LayoutID, sess.UserID, sessionCommandStationChangedPayload{
			CommandStationID: 0,
			WSURL:            nil,
			Status:           "stopped",
		})
		return
	}

	if !s.commandStationAttached(ctx, sess.LayoutID, p.CommandStationID) {
		c.SendAck(requestID, false, "command_station_not_attached")
		return
	}

	if s.dccBus == nil {
		c.SendAck(requestID, false, "dcc_bus_not_configured")
		return
	}

	// Lazy spawn UX (§7e.6): emit the interim "spawning" event so the
	// SPA can paint the placeholder before EnsureRunning blocks.
	s.broadcastChangeForUser(sess.LayoutID, sess.UserID, sessionCommandStationChangedPayload{
		CommandStationID: p.CommandStationID,
		WSURL:            nil,
		Status:           "starting",
		Reason:           "spawning",
	})

	port, _, err := s.dccBus.EnsureRunning(ctx, sess.LayoutID, p.CommandStationID)
	if err != nil {
		s.log.WithError(err).Warn("dcc-bus ensure running")
		code := "dcc_bus_unavailable"
		if errors.Is(err, ErrNoDccBusPortsAvailable) {
			code = "no_dcc_bus_ports_available"
		}
		c.SendAck(requestID, false, code)
		s.broadcastChangeForUser(sess.LayoutID, sess.UserID, sessionCommandStationChangedPayload{
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
		// Direct mode (dev / cross-host): the SPA dials the port itself.
		wsURL = fmt.Sprintf("ws://127.0.0.1:%d/ws", port)
	}

	c.SendAck(requestID, true, "")
	s.broadcastChangeForUser(sess.LayoutID, sess.UserID, sessionCommandStationChangedPayload{
		CommandStationID: p.CommandStationID,
		WSURL:            ptr(wsURL),
		Status:           "running",
	})
}

type commandStationCatalogChangedPayload struct {
	CommandStationID uint                      `json:"commandStationId"`
	Name             string                    `json:"name"`
	Kind             domain.CommandStationKind `json:"kind"`
	SpeedSteps       uint                      `json:"speedSteps"`
}

// BroadcastCommandStationCatalogChanged notifies every live control-
// plane session whose layout exposes the updated command station.
func (s *SessionControlService) BroadcastCommandStationCatalogChanged(ctx context.Context, cs domain.CommandStation) {
	if s.layoutCS == nil || s.layoutRows == nil || s.cs == nil {
		return
	}
	payload := commandStationCatalogChangedPayload{
		CommandStationID: cs.ID,
		Name:             cs.Name,
		Kind:             cs.Kind,
		SpeedSteps:       cs.SpeedSteps,
	}
	for _, c := range s.Sessions() {
		sess := c.Session()
		if !s.commandStationAttached(ctx, sess.LayoutID, cs.ID) {
			continue
		}
		c.SendTyped("session.commandStationCatalogChanged", payload)
	}
}

// broadcastChangeForUser fans the change event out to every session
// of the user inside the layout. Other users see a separate event
// from the dcc-bus consumer (presence-style).
func (s *SessionControlService) broadcastChangeForUser(layoutID, userID uint, payload sessionCommandStationChangedPayload) {
	for _, c := range s.Sessions() {
		sess := c.Session()
		if sess.LayoutID != layoutID || sess.UserID != userID {
			continue
		}
		c.SendTyped("session.commandStationChanged", payload)
	}
}

// openedSessionPayload is the JSON shape sent on `session.opened`.
// `availableCommandStations` is computed from layout_command_stations;
// `currentSession` is the throttle handoff hint for re-connects (a
// browser that refreshes mid-session can keep the same cs).
type openedSessionPayload struct {
	SessionID                string                    `json:"sessionId"`
	LayoutID                 uint                      `json:"layoutId"`
	AvailableCommandStations []availableCSPayload      `json:"availableCommandStations"`
	CurrentSession           *currentSessionPayload    `json:"currentSession,omitempty"`
}

type availableCSPayload struct {
	ID         uint                     `json:"id"`
	Name       string                   `json:"name"`
	Kind       domain.CommandStationKind `json:"kind"`
	SpeedSteps uint                     `json:"speedSteps"`
	WSURL      *string                  `json:"wsUrl"`
}

type currentSessionPayload struct {
	CommandStationID uint `json:"commandStationId"`
}

func (s *SessionControlService) openedPayload(ctx context.Context, c *ws.Client) openedSessionPayload {
	sess := c.Session()
	out := openedSessionPayload{
		SessionID: sess.ID,
		LayoutID:  sess.LayoutID,
	}
	if s.layoutCS == nil || s.cs == nil || s.layoutRows == nil {
		return out
	}
	stations, err := s.listAvailableStations(ctx, sess.LayoutID)
	if err != nil {
		s.log.WithError(err).Warn("session.opened: list cs by ids")
		return out
	}
	out.AvailableCommandStations = make([]availableCSPayload, 0, len(stations))
	for _, st := range stations {
		var wsURL *string
		if s.dccBus != nil && s.dccBus.PortFor(sess.LayoutID, st.ID) != 0 {
			if s.dccBus.ProxyEnabled() {
				url := s.proxyPathFn(st.ID)
				wsURL = &url
			} else {
				url := fmt.Sprintf("ws://127.0.0.1:%d/ws", s.dccBus.PortFor(sess.LayoutID, st.ID))
				wsURL = &url
			}
		}
		out.AvailableCommandStations = append(out.AvailableCommandStations, availableCSPayload{
			ID:         st.ID,
			Name:       st.Name,
			Kind:       st.Kind,
			SpeedSteps: st.SpeedSteps,
			WSURL:      wsURL,
		})
	}
	if cur := sess.CurrentCommandStation(); cur != 0 {
		out.CurrentSession = &currentSessionPayload{CommandStationID: cur}
	}
	return out
}

func (s *SessionControlService) listAvailableStations(ctx context.Context, layoutID uint) ([]domain.CommandStation, error) {
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

func (s *SessionControlService) commandStationAttached(ctx context.Context, layoutID, commandStationID uint) bool {
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
