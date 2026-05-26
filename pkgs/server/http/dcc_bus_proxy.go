package httpapi

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/keskad/loco/pkgs/server/service"
)

// DccBusProxy reverse-proxies the dcc-bus daemon's WebSocket endpoint
// so the SPA only ever talks to loco-server. JWT is verified before
// forwarding; the layout pinning makes sure a session for layout L
// cannot reach a daemon serving layout L'.
type DccBusProxy struct {
	auth   *service.AuthService
	dccBus *service.DccBusService
}

// NewDccBusProxy returns a handler that accepts WS upgrades on
// `/api/v1/dcc-bus/{commandStationId}/ws` and forwards them to the
// matching daemon on the loopback interface.
func NewDccBusProxy(auth *service.AuthService, dccBus *service.DccBusService) *DccBusProxy {
	return &DccBusProxy{auth: auth, dccBus: dccBus}
}

// ServeHTTP handles one upgrade attempt.
func (p *DccBusProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := readSessionToken(r)
	if token == "" {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := p.auth.VerifyToken(r.Context(), token)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	csIDStr := chi.URLParam(r, "commandStationId")
	csID, err := strconv.ParseUint(csIDStr, 10, 64)
	if err != nil || csID == 0 {
		writeJSONError(w, http.StatusBadRequest, "bad_command_station_id")
		return
	}

	port := p.dccBus.PortFor(id.Layout.ID, uint(csID))
	if port == 0 {
		writeJSONError(w, http.StatusServiceUnavailable, "dcc_bus_unavailable")
		return
	}

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
