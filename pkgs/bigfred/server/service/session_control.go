package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/ws"
)

// SessionControlService adapts ws.Client to cmd.SessionControl.
type SessionControlService struct {
	core *cmd.SessionControl

	mu      sync.Mutex
	clients map[*ws.Client]*controlClient
}

// SessionControlConfig groups dependencies.
type SessionControlConfig struct {
	Log         *logrus.Logger
	DccBus      *DccBusService
	RadioStop   *RadioStopService
	EStopTarget *EStopTargetService
	CommandStns *repo.CommandStations
	LayoutCS    *repo.LayoutCommandStations
	Layouts     *repo.Layouts
}

// NewSessionControlService returns a ready WS adapter.
func NewSessionControlService(cfg SessionControlConfig) *SessionControlService {
	var dccBus cmd.DccBusControlPort
	if cfg.DccBus != nil {
		dccBus = dccBusControlPort{svc: cfg.DccBus}
	}
	var radioStop cmd.RadioStopControlPort
	if cfg.RadioStop != nil {
		radioStop = radioStopControlPort{svc: cfg.RadioStop}
	}
	var estopTarget cmd.EStopTargetControlPort
	if cfg.EStopTarget != nil {
		estopTarget = eStopTargetControlPort{svc: cfg.EStopTarget}
	}
	return &SessionControlService{
		core: cmd.NewSessionControl(cmd.SessionControlConfig{
			Log:         cfg.Log,
			DccBus:      dccBus,
			RadioStop:   radioStop,
			EStopTarget: estopTarget,
			CommandStns: cfg.CommandStns,
			LayoutCS:    cfg.LayoutCS,
			Layouts:     cfg.Layouts,
		}),
		clients: make(map[*ws.Client]*controlClient, 8),
	}
}

// Sessions returns a snapshot of every live control-plane client.
func (s *SessionControlService) Sessions() []*ws.Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*ws.Client, 0, len(s.clients))
	for c := range s.clients {
		out = append(out, c)
	}
	return out
}

func (s *SessionControlService) HandleOpened(ctx context.Context, c *ws.Client) {
	client := wrapControlClient(c)
	s.mu.Lock()
	s.clients[c] = client
	s.mu.Unlock()
	s.core.HandleOpened(ctx, client)
}

func (s *SessionControlService) HandleClosed(ctx context.Context, c *ws.Client) {
	s.mu.Lock()
	client := s.clients[c]
	delete(s.clients, c)
	s.mu.Unlock()
	if client == nil {
		client = wrapControlClient(c)
	}
	s.core.HandleClosed(ctx, client)
}

func (s *SessionControlService) HandleEnvelope(ctx context.Context, c *ws.Client, env ws.Envelope) {
	s.mu.Lock()
	client := s.clients[c]
	if client == nil {
		client = wrapControlClient(c)
		s.clients[c] = client
	}
	s.mu.Unlock()
	s.core.HandleEnvelope(ctx, client, cmd.ControlEnvelope{
		Type:    env.Type,
		ID:      env.ID,
		Payload: json.RawMessage(env.Payload),
	})
}

func (s *SessionControlService) BroadcastCommandStationCatalogChanged(ctx context.Context, cs domain.CommandStation) {
	s.core.BroadcastCommandStationCatalogChanged(ctx, cs)
}

type controlClient struct {
	client  *ws.Client
	session controlSession
}

func wrapControlClient(c *ws.Client) *controlClient {
	return &controlClient{client: c, session: controlSession{session: c.Session()}}
}

func (c *controlClient) Session() cmd.ControlSession { return c.session }
func (c *controlClient) SendTyped(eventType string, payload any) {
	c.client.SendTyped(eventType, payload)
}
func (c *controlClient) SendAck(requestID string, ok bool, errCode string) {
	c.client.SendAck(requestID, ok, errCode)
}

type controlSession struct {
	session *ws.DriveSession
}

func (s controlSession) SessionID() string           { return s.session.ID }
func (s controlSession) UserID() uint                { return s.session.UserID }
func (s controlSession) Login() string               { return s.session.Login }
func (s controlSession) LayoutID() uint              { return s.session.LayoutID }
func (s controlSession) CurrentCommandStation() uint { return s.session.CurrentCommandStation() }
func (s controlSession) SetCommandStation(commandStationID uint) uint {
	return s.session.SetCommandStation(commandStationID)
}

type dccBusControlPort struct {
	svc *DccBusService
}

func (p dccBusControlPort) EnsureRunning(ctx context.Context, layoutID, commandStationID uint) (uint16, string, error) {
	port, name, err := p.svc.EnsureRunning(ctx, layoutID, commandStationID)
	if errors.Is(err, ErrNoDccBusPortsAvailable) {
		err = svcerrors.ErrNoDCCBusPortsAvailable
	}
	return port, name, err
}

func (p dccBusControlPort) PortFor(layoutID, commandStationID uint) uint16 {
	return p.svc.PortFor(layoutID, commandStationID)
}

func (p dccBusControlPort) ProxyEnabled() bool { return p.svc.ProxyEnabled() }

type radioStopControlPort struct {
	svc *RadioStopService
}

func (p radioStopControlPort) Trigger(ctx context.Context, sess cmd.ControlSession) (bool, string) {
	wrapped, ok := sess.(controlSession)
	if !ok {
		return false, "dcc_bus_not_configured"
	}
	return p.svc.Trigger(ctx, wrapped.session)
}

type eStopTargetControlPort struct {
	svc *EStopTargetService
}

func (p eStopTargetControlPort) Trigger(
	ctx context.Context,
	sess cmd.ControlSession,
	target domain.TakeoverTarget,
	targetID uint,
) (bool, string) {
	wrapped, ok := sess.(controlSession)
	if !ok {
		return false, "dcc_bus_not_configured"
	}
	return p.svc.Trigger(ctx, wrapped.session, target, targetID)
}
