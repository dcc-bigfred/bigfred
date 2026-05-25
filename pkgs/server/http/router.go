package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/keskad/loco/pkgs/server/domain"
	"github.com/keskad/loco/pkgs/server/service"
	"github.com/keskad/loco/pkgs/server/ws"
)

// RouterConfig collects everything the chi router needs at construction
// time. Keeping it as an explicit struct (rather than positional args)
// makes future additions (Hub, LocoService, …) source-compatible.
type RouterConfig struct {
	Auth           *service.AuthService
	Users          *service.UserService
	Layouts        *service.LayoutService
	Interlockings  *service.InterlockingService
	Occupancy      *service.InterlockingOccupancyService
	Presence       *service.PresenceService
	Vehicles       *service.VehicleService
	Trains         *service.TrainService
	LayoutVehicles *service.LayoutVehicleService
	DCCPool        *service.DCCPoolService
	Sudo           *service.SudoService
	Hub            *ws.Hub

	// AllowedOrigins is forwarded verbatim to the CORS middleware.
	// In development the Vite dev server lives on a different port
	// (5173) than the API, so cookies must be allowed cross-origin.
	AllowedOrigins []string

	// SecureCookie controls the `Secure` flag on the session cookie.
	// Set to false ONLY when the server is reachable over http://
	// (i.e. local development).
	SecureCookie bool
}

// NewRouter wires every HTTP route currently shipped by the bootstrap.
// It returns a http.Handler that the caller mounts on a net/http
// Server (see pkgs/server/main.go).
func NewRouter(cfg RouterConfig) http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
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

	authH := NewAuthHandler(cfg.Auth, cfg.Sudo, cfg.SecureCookie)
	layoutH := NewLayoutHandler(cfg.Layouts)
	interlockingH := NewInterlockingHandler(cfg.Interlockings, cfg.Occupancy, cfg.Auth)
	presenceH := NewPresenceHandler(cfg.Presence)
	vehicleH := NewVehicleHandler(cfg.Vehicles, cfg.LayoutVehicles, cfg.DCCPool)
	trainH := NewTrainHandler(cfg.Trains, cfg.LayoutVehicles)
	rosterH := NewLayoutRosterHandler(cfg.LayoutVehicles, cfg.Auth)
	userH := NewUserHandler(cfg.Users)
	sudoH := NewSudoHandler(cfg.Sudo)

	r.Route("/api/v1", func(r chi.Router) {
		// WebSocket upgrade — auth reads cookie / ?token= inline.
		r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
			ServeWS(cfg.Hub, cfg.Auth, w, r)
		})

		// Public auth endpoints (login does its own credential check).
		r.Post("/auth/login", authH.Login)
		r.Post("/auth/logout", authH.Logout)

		// Public layout dropdown for the login form (§7a.1). Lives
		// outside RequireAuth so an unauthenticated client can fetch
		// the list before submitting credentials.
		r.Get("/layouts/login", layoutH.ListForLogin)

		// Authenticated routes share the RequireAuth middleware.
		r.Group(func(r chi.Router) {
			r.Use(RequireAuth(cfg.Auth))

			r.Get("/auth/me", authH.Me)
			r.Get("/auth/me/dcc-pool", vehicleH.ListPool)

			// Vehicle catalogue (own only for now).
			r.Get("/vehicles", vehicleH.List)
			r.Post("/vehicles", vehicleH.Create)
			r.Put("/vehicles/{id}", vehicleH.Update)
			r.Delete("/vehicles/{id}", vehicleH.Delete)

			// Train catalogue.
			r.Get("/trains", trainH.List)
			r.Post("/trains", trainH.Create)
			r.Put("/trains/{id}", trainH.Update)
			r.Delete("/trains/{id}", trainH.Delete)

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

			r.Get("/layouts/{id}/presence", presenceH.List)

			// Layouts: read endpoints are open to every
			// authenticated user; mutating endpoints require admin.
			r.Get("/layouts", layoutH.List)
			r.Get("/layouts/{id}", layoutH.Get)
			r.Get("/layouts/{id}/interlockings", layoutH.ListInterlockings)

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
				r.Get("/interlockings/catalogue", interlockingH.ListCatalogue)
				r.Post("/layouts", layoutH.Create)
				r.Put("/layouts/{id}", layoutH.Update)
				r.Delete("/layouts/{id}", layoutH.Delete)
				r.Post("/layouts/{id}/lock", layoutH.Lock)
				r.Delete("/layouts/{id}/lock", layoutH.Unlock)
				r.Put("/layouts/{id}/interlockings", layoutH.SetInterlockings)

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
			})
		})
	})

	// Liveness probe sitting OUTSIDE /api/v1 so deployment plumbing
	// (load balancers, k8s) doesn't need to know the API prefix.
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	return r
}
