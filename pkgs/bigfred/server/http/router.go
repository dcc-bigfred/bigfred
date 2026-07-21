package httpapi

import (
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/metrics"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
	"github.com/keskad/loco/pkgs/bigfred/server/ws"
)

// RouterConfig collects everything the chi router needs at construction
// time. Keeping it as an explicit struct (rather than positional args)
// makes future additions (Hub, LocoService, …) source-compatible.
type RouterConfig struct {
	Auth             *cmd.Auth
	Users            *cmd.User
	Layouts          *cmd.Layout
	Interlockings    *cmd.Interlocking
	Occupancy        *service.InterlockingOccupancyService
	Presence         *cmd.Presence
	DccBusLayoutSync *service.DccBusLayoutSync
	Vehicles         *cmd.Vehicle
	Functions        *cmd.Function
	VehicleTemplates *cmd.VehicleTemplate
	Trains           *cmd.Train
	LayoutVehicles   *service.LayoutVehicleService
	DCCPool          *cmd.DCCPool
	Sudo             *cmd.Sudo
	CommandStations  *cmd.CommandStation
	Diagnostics      *service.DiagnosticsService
	Hub              *ws.Hub
	DccBus           *service.DccBusService
	Radio            *service.RadioService
	Audit            *service.AuditService
	Leases           *service.LeaseService
	Remote           *cmd.Remote

	// AllowedOrigins is forwarded verbatim to the CORS middleware.
	// In development the Vite dev server lives on a different port
	// (5173) than the API, so cookies must be allowed cross-origin.
	AllowedOrigins []string

	// SecureCookie controls the `Secure` flag on the session cookie.
	// Set to false ONLY when the server is reachable over http://
	// (i.e. local development).
	SecureCookie bool

	// StaticFS, when non-nil, is the embedded production frontend
	// bundle (web/dist). It is served as a single-page application at
	// "/" with an index.html fallback. nil in development builds, where
	// the SPA is served by the Vite dev server instead.
	StaticFS fs.FS

	// Metrics, when non-nil, records OpenTelemetry signals for HTTP/WS.
	Metrics *metrics.Metrics
}

// NewRouter wires every HTTP route currently shipped by the bootstrap.
// It returns a http.Handler that the caller mounts on a net/http
// Server (see pkgs/server/main.go).
func NewRouter(cfg RouterConfig) http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(MetricsMiddleware(cfg.Metrics))
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		ExposedHeaders:   []string{"Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	authH := NewAuthHandler(cfg.Auth, cfg.Layouts, cfg.Sudo, cfg.Audit, cfg.SecureCookie, cfg.Metrics)
	layoutH := NewLayoutHandler(cfg.Layouts, cfg.Auth, cfg.Audit)
	interlockingH := NewInterlockingHandler(cfg.Interlockings, cfg.Occupancy, cfg.Auth)
	presenceH := NewPresenceHandler(cfg.Presence, cfg.DccBusLayoutSync)
	vehicleH := NewVehicleHandler(cfg.Vehicles, cfg.LayoutVehicles, cfg.DCCPool, cfg.Auth)
	functionH := NewFunctionHandler(cfg.Functions, cfg.Auth)
	functionH.SetVehicleFunctionSync(cfg.LayoutVehicles)
	templateH := NewVehicleTemplateHandler(cfg.VehicleTemplates, cfg.Auth)
	trainH := NewTrainHandler(cfg.Trains, cfg.LayoutVehicles, cfg.Auth)
	rosterH := NewLayoutRosterHandler(cfg.LayoutVehicles, cfg.Auth, cfg.Audit)
	userH := NewUserHandler(cfg.Users, cfg.Auth, cfg.Audit)
	sudoH := NewSudoHandler(cfg.Sudo, cfg.Auth, cfg.Users, cfg.Presence)
	commandStationH := NewCommandStationHandler(cfg.CommandStations, cfg.Auth, cfg.Audit)
	diagnosticsH := NewDiagnosticsHandler(cfg.Diagnostics)
	radioH := NewRadioHandler(cfg.Radio)
	auditH := NewAuditHandler(cfg.Audit)
	leaseH := NewLeaseHandler(cfg.Leases, cfg.Auth)
	remoteH := NewRemoteHandler(cfg.Remote, cfg.Auth)

	r.Route("/api/v1", func(r chi.Router) {
		// WebSocket upgrade — auth reads cookie / ?token= inline.
		r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
			ServeWS(cfg.Hub, cfg.Auth, cfg.Metrics, w, r)
		})

		// dcc-bus data-plane reverse proxy (§7e.6). Verifies the
		// session JWT, looks up the daemon's loopback port and
		// forwards the WebSocket upgrade. The proxy is the default
		// path the SPA dials in production; --redis-external /
		// --dcc-bus-proxy=false setups bypass it.
		if cfg.DccBus != nil {
			proxy := NewDccBusProxy(cfg.Auth, cfg.DccBus, cfg.Metrics)
			r.Get("/dcc-bus/{commandStationId}/ws", proxy.ServeHTTP)
		}

		// Public auth endpoints (login does its own credential check).
		r.Post("/auth/login", authH.Login)
		r.Post("/auth/logout", authH.Logout)

		// Public layout dropdown for the login form (§7a.1). Lives
		// outside RequireAuth so an unauthenticated client can fetch
		// the list before submitting credentials.
		r.Get("/layouts/login", layoutH.ListForLogin)

		// Authenticated routes share the RequireAuth middleware.
		r.Group(func(r chi.Router) {
			r.Use(RequireAuth(cfg.Auth, cfg.Metrics))

			r.Get("/audit-log", auditH.List)

			r.Get("/auth/me", authH.Me)
			r.Put("/auth/me/pin", authH.ChangePIN)
			r.Put("/auth/me/profile", authH.UpdateProfile)
			r.Get("/auth/me/dcc-pool", vehicleH.ListPool)

			// Vehicle catalogue (own only for now).
			r.Get("/vehicles", vehicleH.List)
			r.Get("/vehicles/catalogue", vehicleH.ListCatalogue)
			r.Post("/vehicles", vehicleH.Create)
			r.Put("/vehicles/by-external-id/{externalId}", vehicleH.UpsertByExternalID)
			r.Delete("/vehicles/by-external-id/{externalId}", vehicleH.DeleteByExternalID)
			r.Put("/vehicles/{id}", vehicleH.Update)
			r.Delete("/vehicles/{id}", vehicleH.Delete)

			r.Get("/function-icons", functionH.ListIcons)
			r.Get("/vehicles/function-catalogue", functionH.ListCatalogue)
			r.Get("/vehicles/{id}/functions", functionH.ListVehicle)
			r.Post("/vehicles/{id}/functions/attach", functionH.AttachVehicle)
			r.Post("/vehicles/{id}/functions/reorder", functionH.ReorderVehicle)
			r.Put("/vehicles/{id}/functions/{num}", functionH.UpsertVehicle)
			r.Delete("/vehicles/{id}/functions/{num}", functionH.DeleteVehicle)

			r.Get("/vehicle-templates", templateH.List)
			r.Post("/vehicle-templates", templateH.Create)
			r.Get("/vehicle-templates/{id}", templateH.Get)
			r.Put("/vehicle-templates/{id}", templateH.Update)
			r.Get("/vehicle-templates/{id}/functions", functionH.ListTemplate)
			r.Post("/vehicle-templates/{id}/functions/reorder", functionH.ReorderTemplate)
			r.Put("/vehicle-templates/{id}/functions/{num}", functionH.UpsertTemplate)
			r.Delete("/vehicle-templates/{id}/functions/{num}", functionH.DeleteTemplate)

			// Train catalogue.
			r.Get("/trains", trainH.List)
			r.Get("/trains/catalogue", trainH.ListCatalogue)
			r.Post("/trains", trainH.Create)
			r.Put("/trains/{id}", trainH.Update)
			r.Patch("/trains/{id}/members/{memberId}", trainH.PatchMember)
			r.Delete("/trains/{id}", trainH.Delete)

			r.Get("/leases/received", leaseH.ListReceived)
			r.Get("/leases/granted", leaseH.ListGranted)
			r.Get("/leases/lendable", leaseH.Lendable)
			r.Post("/leases", leaseH.Create)
			r.Patch("/leases/{kind}/{id}", leaseH.Patch)
			r.Delete("/leases/{kind}/{id}", leaseH.Delete)

			// Layout vehicle / train roster (dashboard data sources).
			r.Get("/layouts/{id}/vehicles", rosterH.ListVehicles)
			r.Post("/layouts/{id}/vehicles", rosterH.AddVehicle)
			r.Delete("/layouts/{id}/vehicles/{vehicleId}", rosterH.RemoveVehicle)
			r.Get("/layouts/{id}/trains", rosterH.ListTrains)
			r.Post("/layouts/{id}/trains", rosterH.AddTrain)
			r.Delete("/layouts/{id}/trains/{trainId}", rosterH.RemoveTrain)

			r.Get("/interlockings", interlockingH.List)
			r.Get("/interlockings/{id}", interlockingH.Get)
			r.Post("/interlockings/{id}/join", interlockingH.Join)
			r.Post("/interlockings/{id}/leave", interlockingH.Leave)

			r.Get("/interlockings/{id}/radio", radioH.ReplayInterlocking)
			r.Get("/radio/mine", radioH.ReplayMine)

			r.Get("/layouts/{id}/presence", presenceH.List)

			// Layouts: read endpoints are open to every
			// authenticated user; mutating endpoints require admin.
			r.Get("/layouts", layoutH.List)
			r.Get("/layouts/{id}", layoutH.Get)
			r.Get("/layouts/{id}/interlockings", layoutH.ListInterlockings)
			r.Get("/layouts/{id}/command-stations", layoutH.ListCommandStations)
			r.Get("/layouts/{id}/command-stations/{csid}/remotes/status", remoteH.GetStatus)
			r.Get("/layouts/{id}/command-stations/{csid}/remotes/clients", remoteH.ListClients)
			r.Post("/layouts/{id}/command-stations/{csid}/remotes/{protocol}/pairing", remoteH.StartPairing)
			r.Delete("/layouts/{id}/command-stations/{csid}/remotes/pairing", remoteH.CancelPairing)
			r.Patch("/layouts/{id}/command-stations/{csid}/remotes/session", remoteH.UpdateSession)
			r.Delete("/layouts/{id}/command-stations/{csid}/remotes/session", remoteH.Unpair)

			// Sudo elevation (§7a.7) — open to every authenticated
			// caller; the service guards entry with the layout
			// admin PIN. Two flavours:
			//   /sudo       → 2-min temporary admin elevation
			//   /signalman  → permanent self-grant of the
			//                 layout-scoped signalman role
			r.Post("/layouts/{id}/sudo", sudoH.RequestSudo)
			r.Delete("/layouts/{id}/sudo", sudoH.RevokeSudo)
			r.Post("/layouts/{id}/signalman", sudoH.RequestSignalman)
			r.Delete("/layouts/{id}/signalman", sudoH.RevokeSignalman)

			r.Group(func(r chi.Router) {
				r.Use(RequireRole(cfg.Auth, domain.RoleAdmin))
				r.Post("/layouts/{id}/signalmen", sudoH.GrantSignalmanToUser)
				r.Delete("/layouts/{id}/signalmen/{userId}", sudoH.RevokeSignalmanFromUser)
				r.Get("/interlockings/catalogue", interlockingH.ListCatalogue)
				r.Post("/layouts", layoutH.Create)
				r.Put("/layouts/{id}", layoutH.Update)
				r.Delete("/layouts/{id}", layoutH.Delete)
				r.Post("/layouts/{id}/lock", layoutH.Lock)
				r.Delete("/layouts/{id}/lock", layoutH.Unlock)
				r.Put("/layouts/{id}/interlockings", layoutH.SetInterlockings)
				r.Put("/layouts/{id}/command-stations", layoutH.SetCommandStations)

				r.Get("/command-stations/catalogue", commandStationH.ListCatalogue)
				r.Post("/command-stations", commandStationH.Create)
				r.Put("/command-stations/{id}", commandStationH.Update)
				r.Delete("/command-stations/{id}", commandStationH.Delete)

				r.Post("/interlockings", interlockingH.Create)
				r.Put("/interlockings/{id}", interlockingH.Update)
				r.Delete("/interlockings/{id}", interlockingH.Delete)

				// User management (§4.1 / §7a.5). Permanent admin
				// is the only role allowed to mutate the user
				// catalogue; the dedicated security policy guards
				// the self-action edge cases inside the handler.
				r.Get("/users", userH.List)
				r.Post("/users", userH.Create)
				r.Put("/users/{id}", userH.Update)
				r.Delete("/users/{id}", userH.Delete)
				r.Post("/users/{id}/activate", userH.Activate)
				r.Post("/users/{id}/deactivate", userH.Deactivate)

				r.Get("/diagnostics/sources", diagnosticsH.ListSources)
				r.Get("/diagnostics/content", diagnosticsH.ReadContent)

				if cfg.DccBus != nil {
					slotsProxy := NewDccBusSlotsProxy(cfg.Auth, cfg.DccBus)
					r.Get("/admin/dcc-bus/{commandStationId}/slots/ws", slotsProxy.ServeHTTP)
					r.Post("/admin/dcc-bus/{commandStationId}/slots/release", slotsProxy.ServeRelease)
				}
			})
		})
	})

	// Liveness probe sitting OUTSIDE /api/v1 so deployment plumbing
	// (load balancers, k8s) doesn't need to know the API prefix.
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Embedded SPA (production builds). Registered last as a catch-all:
	// chi prefers the more specific /api/v1 and /healthz routes, so this
	// only handles browser navigations and static asset requests.
	if cfg.StaticFS != nil {
		spa := NewSPAHandler(cfg.StaticFS)
		r.Handle("/*", spa)
	}

	return r
}
