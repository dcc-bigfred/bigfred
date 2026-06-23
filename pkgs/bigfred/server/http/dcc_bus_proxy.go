package httpapi

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/metrics"

	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

// DccBusProxy reverse-proxies the dcc-bus daemon's WebSocket endpoint
// so the SPA only ever talks to loco-server. JWT is verified before
// forwarding; the layout pinning makes sure a session for layout L
// cannot reach a daemon serving layout L'.
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

	port := p.dccBus.PortFor(layoutID, csID)
	if port == 0 {
		if p.metrics != nil {
			p.metrics.RecordDccBusProxyUpgrade(layoutID, csID, false, "dcc_bus_unavailable")
		}
		writeJSONError(w, http.StatusServiceUnavailable, "dcc_bus_unavailable")
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
		// Forward the JWT as a query param so the daemon's
		// authenticator (which doesn't share AuthService's cookie
		// jar) can verify the same token.
		q := req.URL.Query()
		q.Set("token", token)
		req.URL.RawQuery = q.Encode()
		req.Host = target.Host
	}
	rp.ServeHTTP(w, r)
}
