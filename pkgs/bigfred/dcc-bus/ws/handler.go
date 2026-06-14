package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/auth"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/errors"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
)

// Router is the abstraction the WS handler relies on to dispatch a
// client envelope to the right business logic. It lives in
// `pkgs/dcc-bus/cmd` and is wired by the daemon assembly. Keeping
// the interface here avoids a cyclic import between ws and cmd.
type Router interface {
	// HandleSubscribe is called when the client requests state
	// updates for a set of locomotive addresses. The daemon
	// validates membership in the layout roster and pushes a
	// `loco.state` snapshot for each accepted address.
	HandleSubscribe(ctx context.Context, sess *Session, payload protocol.LocoSubscribePayload, requestID string)
	// HandleSetSpeed handles a single throttle move.
	HandleSetSpeed(ctx context.Context, sess *Session, payload protocol.LocoSetSpeedPayload, requestID string)
	// HandleSetFunction toggles one locomotive function.
	HandleSetFunction(ctx context.Context, sess *Session, payload protocol.LocoSetFunctionPayload, requestID string)
	// HandleEStop fires the data-plane emergency stop scoped to
	// this daemon's command station.
	HandleEStop(ctx context.Context, sess *Session, payload protocol.SystemEStopPayload, requestID string)
	// HandleSessionClose is called once when the session goes away
	// (any reason: WS close, error, ctx cancellation). It is the
	// router's chance to fire the dead-man's plan and drop user-
	// scoped subscriptions.
	HandleSessionClose(ctx context.Context, sess *Session, reason string)
}

// Server is the HTTP endpoint the daemon exposes on its `--port`.
// It accepts JWT-authenticated WebSocket upgrades and routes inbound
// frames to the Router.
type Server struct {
	verifier      *auth.Verifier
	hub           *Hub
	router        Router
	log           *logrus.Logger
	heartbeatSecs float64
	deadmanSecs   float64
	speedSteps    uint
	layoutID      uint
	csID          uint

	// AllowedOrigins is forwarded verbatim to websocket.AcceptOptions.
	// Empty slice means InsecureSkipVerify = true (acceptable when
	// the daemon binds to loopback because the reverse proxy on
	// loco-server already validates Origin).
	AllowedOrigins []string
}

// ServerConfig collects the few knobs Server takes at construction.
type ServerConfig struct {
	Verifier       *auth.Verifier
	Hub            *Hub
	Router         Router
	Log            *logrus.Logger
	LayoutID       uint
	CommandStation uint
	SpeedSteps     uint
	HeartbeatSecs  float64
	DeadmanSecs    float64
	AllowedOrigins []string
}

// NewServer returns a ready-to-mount Server. Heartbeat and dead-man
// defaults match §7e.5 (5s / 6s).
func NewServer(cfg ServerConfig) *Server {
	hb := cfg.HeartbeatSecs
	if hb <= 0 {
		hb = 5
	}
	dms := cfg.DeadmanSecs
	if dms <= 0 {
		dms = 6
	}
	steps := cfg.SpeedSteps
	if steps == 0 {
		steps = 128
	}
	log := cfg.Log
	if log == nil {
		log = logrus.New()
	}
	return &Server{
		verifier:       cfg.Verifier,
		hub:            cfg.Hub,
		router:         cfg.Router,
		log:            log,
		layoutID:       cfg.LayoutID,
		csID:           cfg.CommandStation,
		speedSteps:     steps,
		heartbeatSecs:  hb,
		deadmanSecs:    dms,
		AllowedOrigins: cfg.AllowedOrigins,
	}
}

// ServeHTTP implements http.Handler. It accepts WS upgrades on any
// path and rejects everything else with 404 — there's only one
// endpoint per daemon.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/ws":
		s.handleWS(w, r)
	case "/healthz":
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	default:
		http.NotFound(w, r)
	}
}

// handleWS authenticates, upgrades, registers the session and runs
// the read loop until ctx ends or the client disconnects.
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		if c, err := r.Cookie("bigfred_session"); err == nil {
			token = c.Value
		}
	}
	id, err := s.verifier.Verify(token)
	if err != nil {
		s.log.WithError(err).Debug("dcc-bus reject upgrade")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	accept := &websocket.AcceptOptions{
		OriginPatterns:     s.AllowedOrigins,
		InsecureSkipVerify: len(s.AllowedOrigins) == 0,
	}
	conn, err := websocket.Accept(w, r, accept)
	if err != nil {
		s.log.WithError(err).Debug("dcc-bus upgrade failed")
		return
	}

	sess := NewSession(id, conn)
	s.hub.Register(sess)

	openPayload := protocol.DccBusOpenedPayload{
		LayoutID:         s.layoutID,
		CommandStationID: s.csID,
		SpeedSteps:       s.speedSteps,
		HeartbeatSecs:    s.heartbeatSecs,
		DeadmanSecs:      s.deadmanSecs,
		SessionID:        sess.ID,
	}
	if err := sess.SendTyped(r.Context(), protocol.TypeDccBusOpened, openPayload); err != nil {
		s.log.WithError(err).Debug("dcc-bus opened frame failed")
		sess.Close(errors.WsCodeSessionSendFailed)
		s.hub.Unregister(sess)
		return
	}

	s.log.WithFields(logrus.Fields{
		"sessionId":        sess.ID,
		"userId":           sess.UserID,
		"login":            sess.Login,
		"layoutId":         s.layoutID,
		"commandStationId": s.csID,
		"speedSteps":       s.speedSteps,
	}).Info("dcc-bus browser session opened (command station driver already bound at daemon start)")

	s.readLoop(r.Context(), sess)
}

// readLoop pumps client frames into the Router until EOF or ctx
// cancellation. It also installs the per-session dead-man's switch
// watchdog.
func (s *Server) readLoop(ctx context.Context, sess *Session) {
	dmsCtx, cancelDMS := context.WithCancel(ctx)
	defer cancelDMS()

	go s.watchDeadman(dmsCtx, sess)

	defer func() {
		s.router.HandleSessionClose(context.Background(), sess, errors.WsCodeSessionWsClosed)
		s.hub.Unregister(sess)
		sess.Close(errors.WsCodeSessionReadLoopDone)
		s.log.WithFields(logrus.Fields{
			"sessionId":              sess.ID,
			"userId":                 sess.UserID,
			"userSessionsRemaining": len(s.hub.SessionsForUser(sess.UserID)),
		}).Info("dcc-bus session closed")
	}()

	for {
		_, raw, err := sess.conn.Read(ctx)
		if err != nil {
			s.log.WithError(err).WithFields(logrus.Fields{
				"sessionId":        sess.ID,
				"layoutId":         s.layoutID,
				"commandStationId": s.csID,
			}).Info("dcc-bus browser WebSocket read ended")
			return
		}
		sess.Touch()

		var env contract.EnvelopeWire
		if err := json.Unmarshal(raw, &env); err != nil {
			_ = sess.SendTyped(ctx, protocol.TypeLocoError, protocol.LocoErrorPayload{
				Code: errors.WsCodeBadEnvelope,
			})
			continue
		}
		s.dispatch(ctx, sess, env)
	}
}

// dispatch decodes the inbound envelope into a typed payload and
// delegates to the Router. Unknown types are answered with a
// `loco.error{code:errors.WsCodeUnknownFrame}` so the client surfaces a
// debuggable error instead of silently dropping the request.
func (s *Server) dispatch(ctx context.Context, sess *Session, env contract.EnvelopeWire) {
	switch env.Type {
	case protocol.TypePing:
		_ = sess.SendTyped(ctx, protocol.TypePong, nil)

	case protocol.TypeLocoSubscribe:
		var p protocol.LocoSubscribePayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			_ = sess.SendAck(ctx, env.ID, false, errors.WsCodeBadPayload)
			return
		}
		s.router.HandleSubscribe(ctx, sess, p, env.ID)

	case protocol.TypeLocoSetSpeed:
		var p protocol.LocoSetSpeedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			_ = sess.SendAck(ctx, env.ID, false, errors.WsCodeBadPayload)
			return
		}
		s.router.HandleSetSpeed(ctx, sess, p, env.ID)

	case protocol.TypeLocoSetFunction:
		var p protocol.LocoSetFunctionPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			_ = sess.SendAck(ctx, env.ID, false, errors.WsCodeBadPayload)
			return
		}
		s.router.HandleSetFunction(ctx, sess, p, env.ID)

	case protocol.TypeSystemEStop:
		var p protocol.SystemEStopPayload
		if env.Payload != nil {
			_ = json.Unmarshal(env.Payload, &p)
		}
		s.router.HandleEStop(ctx, sess, p, env.ID)

	default:
		_ = sess.SendTyped(ctx, protocol.TypeLocoError, protocol.LocoErrorPayload{
			Code:   errors.WsCodeUnknownFrame,
			Detail: env.Type,
		})
	}
}

// watchDeadman fires HandleSessionClose(WsCodeSessionDeadman) when the session
// stays silent past its budget. The Router is responsible for the
// actual DCC actions (emergency stop on every loco the session owns).
func (s *Server) watchDeadman(ctx context.Context, sess *Session) {
	tick := time.NewTicker(time.Duration(s.deadmanSecs) * time.Second / 2)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if sess.IsClosed() {
				return
			}
			if sess.IdleFor() > time.Duration(s.deadmanSecs)*time.Second {
				s.log.WithField("sessionId", sess.ID).Warn("dcc-bus dead-man triggered")
				s.router.HandleSessionClose(context.Background(), sess, errors.WsCodeSessionDeadman)
				sess.Close(errors.WsCodeSessionDeadman)
				return
			}
		}
	}
}
