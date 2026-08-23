package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/metrics"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

// dccBusProxySpawnTimeout is the budget the WS proxy spends spawning a
// missing daemon before answering 503. Shorter than DccBusConfig.SpawnTimeout
// so a hung microinit start cannot stall the upgrade indefinitely.
const dccBusProxySpawnTimeout = 5 * time.Second

// DccBusProxy reverse-proxies the dcc-bus daemon's WebSocket endpoint
// so the SPA only ever talks to loco-server. JWT is verified before
// forwarding; the layout pinning makes sure a session for layout L
// cannot reach a daemon serving layout L'.
//
// When an admin sends ImpersonateAsHeader, the proxy mints a subject
// JWT (same layout) for the daemon so drive commands run as the
// participant — matching MaybeImpersonate on REST.
type DccBusProxy struct {
	auth    *cmd.Auth
	dccBus  *service.DccBusService
	metrics *metrics.Metrics
}

// NewDccBusProxy returns a handler that accepts WS upgrades on
// `/api/v1/dcc-bus/{commandStationId}/ws` and forwards them to the
// matching daemon on the loopback interface.
func NewDccBusProxy(auth *cmd.Auth, dccBus *service.DccBusService, m *metrics.Metrics) *DccBusProxy {
	return &DccBusProxy{auth: auth, dccBus: dccBus, metrics: m}
}

// ServeHTTP handles one upgrade attempt.
func (p *DccBusProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	layoutID := uint(0)
	csID := uint(0)
	start := time.Now()
	recordSession := false
	defer func() {
		if recordSession && p.metrics != nil {
			p.metrics.RecordDccBusProxySessionClosed(layoutID, csID, time.Since(start))
		}
	}()

	token := readSessionToken(r)
	if token == "" {
		if p.metrics != nil {
			p.metrics.RecordAuthUnauthorized("/api/v1/dcc-bus/ws")
		}
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := p.auth.VerifyToken(r.Context(), token)
	if err != nil {
		if p.metrics != nil {
			p.metrics.RecordAuthTokenVerifyError("verify_failed")
			p.metrics.RecordAuthUnauthorized("/api/v1/dcc-bus/ws")
		}
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	layoutID = id.Layout.ID

	daemonToken := token
	if login := strings.TrimSpace(r.Header.Get(ImpersonateAsHeader)); login != "" {
		minted, _, err := p.auth.IssueImpersonatedToken(r.Context(), id, login)
		if err != nil {
			if errors.Is(err, svcerrors.ErrImpersonationForbidden) {
				writeJSONError(w, http.StatusForbidden, svcerrors.CodeImpersonationForbidden)
				return
			}
			if errors.Is(err, svcerrors.ErrAccountDeactivated) {
				writeJSONError(w, http.StatusForbidden, svcerrors.CodeAccountDeactivated)
				return
			}
			status, code := svcerrors.UserHTTPStatus(err)
			writeJSONErrorCause(w, status, code, err)
			return
		}
		daemonToken = minted
	}

	csIDStr := chi.URLParam(r, "commandStationId")
	csID64, err := strconv.ParseUint(csIDStr, 10, 64)
	if err != nil || csID64 == 0 {
		if p.metrics != nil {
			p.metrics.RecordDccBusProxyUpgrade(layoutID, 0, false, "bad_command_station_id")
		}
		writeJSONError(w, http.StatusBadRequest, "bad_command_station_id")
		return
	}
	csID = uint(csID64)

	port, code, err := resolveDccBusPort(r.Context(), p.dccBus, layoutID, csID)
	if err != nil {
		if p.metrics != nil {
			p.metrics.RecordDccBusProxyUpgrade(layoutID, csID, false, code)
		}
		logDccBusProxyFail(w, r, layoutID, csID, 0, http.StatusServiceUnavailable, code, err)
		writeJSONErrorBody(w, http.StatusServiceUnavailable, code)
		return
	}

	if p.metrics != nil {
		p.metrics.RecordDccBusProxyUpgrade(layoutID, csID, true, "_")
		p.metrics.RecordDccBusProxySessionOpened(layoutID, csID)
	}
	recordSession = true

	target := &url.URL{
		Scheme: "http",
		Host:   "127.0.0.1:" + strconv.Itoa(int(port)),
	}
	rp := httputil.NewSingleHostReverseProxy(target)
	// Force the daemon's `/ws` path regardless of how chi parsed the
	// inbound URL — the daemon only understands that single path.
	rp.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = "/ws"
		// Forward the (possibly impersonated) JWT as a query param so
		// the daemon's authenticator can verify the same token.
		q := req.URL.Query()
		q.Set("token", daemonToken)
		req.URL.RawQuery = q.Encode()
		req.Host = target.Host
	}
	rp.ErrorHandler = dccBusProxyErrorHandler(layoutID, csID, port)
	rp.ServeHTTP(w, r)
}

// resolveDccBusPort returns the loopback port for (layout, CS). A miss
// triggers EnsureRunning so a wizard (or any client) that never opened a
// layout session can still spawn the daemon.
func resolveDccBusPort(ctx context.Context, dccBus *service.DccBusService, layoutID, csID uint) (uint16, string, error) {
	if port := dccBus.PortFor(layoutID, csID); port != 0 {
		return port, "", nil
	}
	spawnCtx, cancel := context.WithTimeout(ctx, dccBusProxySpawnTimeout)
	defer cancel()
	port, _, err := dccBus.EnsureRunning(spawnCtx, layoutID, csID)
	if err != nil {
		return 0, dccBusEnsureErrorCode(err), err
	}
	return port, "", nil
}

func dccBusEnsureErrorCode(err error) string {
	switch {
	case errors.Is(err, service.ErrCommandStationNotAttached):
		return "command_station_not_attached"
	case errors.Is(err, svcerrors.ErrNoDCCBusPortsAvailable):
		return svcerrors.CodeNoDCCBusPortsAvailable
	default:
		return "dcc_bus_unavailable"
	}
}

func dccBusProxyErrorHandler(layoutID, csID uint, port uint16) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		logDccBusProxyFail(w, r, layoutID, csID, port, http.StatusBadGateway, "dcc_bus_unreachable", err)
		writeJSONErrorBody(w, http.StatusBadGateway, "dcc_bus_unreachable")
	}
}

func logDccBusProxyFail(w http.ResponseWriter, r *http.Request, layoutID, csID uint, port uint16, status int, code string, err error) {
	log, req := loggerAndRequest(w)
	if log == nil && r != nil {
		if l, ok := r.Context().Value(requestLogCtxKey{}).(*logrus.Logger); ok {
			log = l
		}
	}
	if log == nil {
		return
	}
	if req == nil {
		req = r
	}
	fields := logrus.Fields{
		"status":           status,
		"code":             code,
		"layoutId":         layoutID,
		"commandStationId": csID,
		"program":          service.ProgramName(layoutID, csID),
		"port":             port,
		"portAllocated":    port != 0,
	}
	if req != nil {
		fields["method"] = req.Method
		fields["path"] = req.URL.Path
		fields["request_id"] = chimiddleware.GetReqID(req.Context())
	}
	entry := log.WithFields(fields)
	if err != nil {
		entry = entry.WithError(err)
	}
	entry.Warn("dcc-bus proxy 5xx")
}
