package httpapi

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

// DccBusSlotsProxy reverse-proxies the dcc-bus admin slot-diagnostic
// WebSocket and release action. Admin role is required; daemon endpoints are
// loopback-only.
type DccBusSlotsProxy struct {
	auth   *cmd.Auth
	dccBus *service.DccBusService
}

// NewDccBusSlotsProxy returns a handler for
// `/api/v1/admin/dcc-bus/{commandStationId}/slots/ws` and `/slots/release`.
func NewDccBusSlotsProxy(auth *cmd.Auth, dccBus *service.DccBusService) *DccBusSlotsProxy {
	return &DccBusSlotsProxy{auth: auth, dccBus: dccBus}
}

// ServeHTTP handles one WebSocket upgrade attempt.
func (p *DccBusSlotsProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.proxy(w, r, "/admin/slots/ws")
}

// ServeRelease reverse-proxies POST /admin/dcc-bus/{commandStationId}/slots/release.
func (p *DccBusSlotsProxy) ServeRelease(w http.ResponseWriter, r *http.Request) {
	p.proxy(w, r, "/admin/slots/release")
}

func (p *DccBusSlotsProxy) proxy(w http.ResponseWriter, r *http.Request, daemonPath string) {
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
	if !id.HasRole(domain.RoleAdmin) {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}

	csIDStr := chi.URLParam(r, "commandStationId")
	csID64, err := strconv.ParseUint(csIDStr, 10, 64)
	if err != nil || csID64 == 0 {
		writeJSONError(w, http.StatusBadRequest, "bad_command_station_id")
		return
	}
	csID := uint(csID64)

	port := p.dccBus.PortFor(id.Layout.ID, csID)
	if port == 0 {
		writeJSONError(w, http.StatusServiceUnavailable, "dcc_bus_unavailable")
		return
	}

	target := &url.URL{
		Scheme: "http",
		Host:   "127.0.0.1:" + strconv.Itoa(int(port)),
	}
	rp := httputil.NewSingleHostReverseProxy(target)
	rp.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = daemonPath
		q := req.URL.Query()
		q.Set("token", token)
		req.URL.RawQuery = q.Encode()
		req.Host = target.Host
	}
	rp.ServeHTTP(w, r)
}
