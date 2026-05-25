package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/keskad/loco/pkgs/server/domain"
	"github.com/keskad/loco/pkgs/server/service"
)

// RouterConfig collects everything the chi router needs at construction
// time. Keeping it as an explicit struct (rather than positional args)
// makes future additions (Hub, LocoService, …) source-compatible.
type RouterConfig struct {
	Auth          *service.AuthService
	Layouts       *service.LayoutService
	Interlockings *service.InterlockingService

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

	authH := NewAuthHandler(cfg.Auth, cfg.SecureCookie)
	layoutH := NewLayoutHandler(cfg.Layouts)
	interlockingH := NewInterlockingHandler(cfg.Interlockings)

	r.Route("/api/v1", func(r chi.Router) {
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

			r.Get("/interlockings", interlockingH.List)
			r.Get("/interlockings/{id}", interlockingH.Get)

			// Layouts: read endpoints are open to every
			// authenticated user; mutating endpoints require admin.
			r.Get("/layouts", layoutH.List)
			r.Get("/layouts/{id}", layoutH.Get)
			r.Get("/layouts/{id}/interlockings", layoutH.ListInterlockings)

			r.Group(func(r chi.Router) {
				r.Use(RequireRole(domain.RoleAdmin))
				r.Post("/layouts", layoutH.Create)
				r.Put("/layouts/{id}", layoutH.Update)
				r.Delete("/layouts/{id}", layoutH.Delete)
				r.Post("/layouts/{id}/lock", layoutH.Lock)
				r.Delete("/layouts/{id}/lock", layoutH.Unlock)
				r.Put("/layouts/{id}/interlockings", layoutH.SetInterlockings)

				r.Post("/interlockings", interlockingH.Create)
				r.Put("/interlockings/{id}", interlockingH.Update)
				r.Delete("/interlockings/{id}", interlockingH.Delete)
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
