// Package cli wires the cobra command that the `loco-server` binary
// runs. Keeping the cobra wiring out of `main` makes the command
// testable in isolation and mirrors the layout of `pkgs/loco/cli`.
package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	httpapi "github.com/keskad/loco/pkgs/server/http"
	"github.com/keskad/loco/pkgs/server/repo"
	"github.com/keskad/loco/pkgs/server/repo/migrations"
	"github.com/keskad/loco/pkgs/server/service"
)

// Flags collects every command-line knob exposed by `loco-server`.
// Defaults are tuned for local development: SQLite file lives next to
// the binary, the API listens on :8080 and CORS allows the Vite dev
// server on :5173.
type Flags struct {
	HTTPAddr       string
	DBPath         string
	JWTSecret      string
	AllowedOrigins []string
	SecureCookie   bool
}

// NewRootCommand returns the top-level cobra command. It is invoked
// from `main()` of the standalone `loco-server` binary.
func NewRootCommand(log *logrus.Logger) *cobra.Command {
	var f Flags

	cmd := &cobra.Command{
		Use:   "loco-server",
		Short: "BigFred web application — Go backend (REST + WebSocket).",
		Long: `loco-server is the HTTP/WebSocket facade in front of the existing
LocoApp controller layer. It owns user authentication, session
management and (in later milestones) the WebSocket fan-out for
real-time throttle commands.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd.Context(), log, f)
		},
	}

	cmd.Flags().StringVar(&f.HTTPAddr, "http", ":8080", "address the HTTP server listens on")
	cmd.Flags().StringVar(&f.DBPath, "db", "bigfred.db", "path to the SQLite database file")
	cmd.Flags().StringVar(&f.JWTSecret, "jwt-secret", "",
		"hex/base64 secret used to sign session JWTs. Falls back to BIGFRED_JWT_SECRET "+
			"env var; a random per-run secret is generated when empty (sessions don't survive restarts).")
	cmd.Flags().StringSliceVar(&f.AllowedOrigins, "cors-origin",
		[]string{"http://localhost:5173", "http://127.0.0.1:5173"},
		"CORS allowed origins (Vite dev server on :5173 by default)")
	cmd.Flags().BoolVar(&f.SecureCookie, "secure-cookie", false,
		"set the Secure flag on the session cookie (REQUIRED in production, off for local http://)")

	return cmd
}

func run(ctx context.Context, log *logrus.Logger, f Flags) error {
	if absPath, err := filepath.Abs(f.DBPath); err == nil {
		f.DBPath = absPath
	}

	secret, err := resolveJWTSecret(f.JWTSecret, log)
	if err != nil {
		return err
	}

	repository, sqlDB, err := repo.Open(f.DBPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer sqlDB.Close()

	log.WithField("path", f.DBPath).Info("database opened, applying migrations")
	migrations.MigrateUp(ctx, repository)

	users := repo.NewUsers(repository)

	authSvc := service.NewAuthService(users, service.AuthConfig{JWTSecret: secret})

	seeded, err := service.SeedAdmin(ctx, users, service.SeedDefaults)
	if err != nil {
		return fmt.Errorf("seed admin: %w", err)
	}
	if seeded {
		log.WithFields(logrus.Fields{
			"login": service.SeedDefaults.Login,
			"pin":   service.SeedDefaults.PIN,
		}).Warn("bootstrap admin account created — CHANGE THE PIN AFTER FIRST LOGIN")
	}

	router := httpapi.NewRouter(httpapi.RouterConfig{
		Auth:           authSvc,
		AllowedOrigins: f.AllowedOrigins,
		SecureCookie:   f.SecureCookie,
	})

	srv := &http.Server{
		Addr:              f.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		log.WithField("addr", f.HTTPAddr).Info("listening")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
		close(serveErr)
	}()

	// Cooperative shutdown on SIGINT/SIGTERM. We give in-flight
	// requests a brief grace period before forcing the server down.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.WithField("signal", sig.String()).Info("shutdown requested")
	case err := <-serveErr:
		if err != nil {
			return err
		}
		return nil
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}

// resolveJWTSecret picks the JWT signing key in the documented
// precedence order: explicit --jwt-secret > BIGFRED_JWT_SECRET env >
// random per-run secret (development only).
func resolveJWTSecret(flag string, log *logrus.Logger) ([]byte, error) {
	if flag != "" {
		return []byte(flag), nil
	}
	if env := os.Getenv("BIGFRED_JWT_SECRET"); env != "" {
		return []byte(env), nil
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("generate random jwt secret: %w", err)
	}
	log.Warn("no JWT secret configured — generated a random one. Existing sessions will not survive a restart. " +
		"Set --jwt-secret or BIGFRED_JWT_SECRET in production.")
	// Use the raw bytes (not hex) — the encoding doesn't matter for
	// HMAC, but the log message above is a strong hint that this is
	// development-only behaviour.
	_ = hex.EncodeToString(buf)
	return buf, nil
}
