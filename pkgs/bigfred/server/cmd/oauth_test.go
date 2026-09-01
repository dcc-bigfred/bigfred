package cmd_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/dcc-bigfred/bigfred/pkgs/bigfred/server/cmd"
	"github.com/dcc-bigfred/bigfred/pkgs/bigfred/server/domain"
	svcerrors "github.com/dcc-bigfred/bigfred/pkgs/bigfred/server/errors"
	httpapi "github.com/dcc-bigfred/bigfred/pkgs/bigfred/server/http"
	"github.com/dcc-bigfred/bigfred/pkgs/bigfred/server/repo"
)

const oauthTestRedirect = "http://localhost:8091/auth/callback"

type oauthTestEnv struct {
	ctx      context.Context
	oauth    *cmd.OAuth
	auth     *cmd.Auth
	layout   *cmd.Layout
	system   domain.Layout
	user     domain.User
	mr       *miniredis.Miniredis
	bundle   repo.UsersBundle
	cleanup  func()
}

func freshOAuthEnv(t *testing.T) *oauthTestEnv {
	t.Helper()
	bundle, cleanup := freshRepo(t)
	ctx := context.Background()
	layoutSvc := freshLayoutSvc(t, ctx, bundle)
	if _, err := layoutSvc.EnsureSystemLayout(ctx); err != nil {
		t.Fatalf("EnsureSystemLayout: %v", err)
	}
	system, err := layoutSvc.GetSystem(ctx)
	if err != nil {
		t.Fatalf("GetSystem: %v", err)
	}
	user := insertUserWithPIN(t, ctx, bundle, "alice", "1234", domain.RoleAdmin)
	auth := cmd.NewAuth(bundle.Users, layoutSvc, bundle.LayoutSignalmen, bundle.SudoElevations,
		cmd.AuthConfig{JWTSecret: []byte("test-secret-test-secret-test-aaaa")})

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	dir := t.TempDir()
	writeOAuthClient(t, dir, "bigfred-wizard", false)
	writeOAuthClient(t, dir, "shared-app", true)
	reg, err := cmd.NewOAuthClientsRegistry(dir, nil)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	oauth := cmd.NewOAuth(auth, reg, rdb)
	return &oauthTestEnv{
		ctx:     ctx,
		oauth:   oauth,
		auth:    auth,
		layout:  layoutSvc,
		system:  system,
		user:    user,
		mr:      mr,
		bundle:  bundle,
		cleanup: func() { mr.Close(); cleanup() },
	}
}

func writeOAuthClient(t *testing.T, dir, id string, share bool) {
	t.Helper()
	body := `{
		"clientId": "` + id + `",
		"clientSecret": "secret",
		"redirectUris": ["` + oauthTestRedirect + `"],
		"enabled": true,
		"shareSession": ` + boolJSON(share) + `
	}`
	if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(body), 0o640); err != nil {
		t.Fatal(err)
	}
}

func boolJSON(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func TestLoginTicketIsOneTime(t *testing.T) {
	env := freshOAuthEnv(t)
	defer env.cleanup()

	id := cmd.Identity{User: env.user, Layout: env.system}
	ticket, err := env.oauth.IssueLoginTicket(env.ctx, id)
	if err != nil {
		t.Fatalf("IssueLoginTicket: %v", err)
	}
	got, err := env.oauth.ConsumeLoginTicket(env.ctx, ticket)
	if err != nil {
		t.Fatalf("ConsumeLoginTicket: %v", err)
	}
	if got.User.ID != env.user.ID || got.Layout.ID != env.system.ID {
		t.Fatalf("identity = user %d layout %d, want %d %d", got.User.ID, got.Layout.ID, env.user.ID, env.system.ID)
	}
	if _, err := env.oauth.ConsumeLoginTicket(env.ctx, ticket); !errors.Is(err, svcerrors.ErrInvalidCredentials) {
		t.Fatalf("second consume: %v, want ErrInvalidCredentials", err)
	}
}

func TestLoginTicketExpires(t *testing.T) {
	env := freshOAuthEnv(t)
	defer env.cleanup()

	id := cmd.Identity{User: env.user, Layout: env.system}
	ticket, err := env.oauth.IssueLoginTicket(env.ctx, id)
	if err != nil {
		t.Fatalf("IssueLoginTicket: %v", err)
	}
	env.mr.FastForward(61 * time.Second)
	if _, err := env.oauth.ConsumeLoginTicket(env.ctx, ticket); !errors.Is(err, svcerrors.ErrInvalidCredentials) {
		t.Fatalf("expired ticket: %v, want ErrInvalidCredentials", err)
	}
}

func TestLoginTicketRejectsInactiveUser(t *testing.T) {
	env := freshOAuthEnv(t)
	defer env.cleanup()

	id := cmd.Identity{User: env.user, Layout: env.system}
	ticket, err := env.oauth.IssueLoginTicket(env.ctx, id)
	if err != nil {
		t.Fatalf("IssueLoginTicket: %v", err)
	}
	env.user.Active = false
	if err := env.bundle.Users.Update(env.ctx, &env.user); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if _, err := env.oauth.ConsumeLoginTicket(env.ctx, ticket); !errors.Is(err, svcerrors.ErrInvalidCredentials) {
		t.Fatalf("inactive user: %v, want ErrInvalidCredentials", err)
	}
}

func TestAuthorizeRedirectsEphemeralWhenShareSessionFalse(t *testing.T) {
	env := freshOAuthEnv(t)
	defer env.cleanup()

	h := httpapi.NewOAuthHandler(env.oauth, env.auth)
	req := httptest.NewRequest(http.MethodGet, authorizeURL("bigfred-wizard"), nil)
	rec := httptest.NewRecorder()
	h.Authorize(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "ephemeral=1") {
		t.Fatalf("Location=%s, want ephemeral=1", loc)
	}
	if !strings.Contains(loc, "return_to=") {
		t.Fatalf("Location=%s, want return_to", loc)
	}
}

func TestAuthorizeOmitsEphemeralWhenShareSessionTrue(t *testing.T) {
	env := freshOAuthEnv(t)
	defer env.cleanup()

	h := httpapi.NewOAuthHandler(env.oauth, env.auth)
	req := httptest.NewRequest(http.MethodGet, authorizeURL("shared-app"), nil)
	rec := httptest.NewRecorder()
	h.Authorize(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if strings.Contains(loc, "ephemeral=") {
		t.Fatalf("Location=%s, did not want ephemeral", loc)
	}
}

func TestAuthorizeLoginTicketIssuesCodeWithoutCookie(t *testing.T) {
	env := freshOAuthEnv(t)
	defer env.cleanup()

	id := cmd.Identity{User: env.user, Layout: env.system}
	ticket, err := env.oauth.IssueLoginTicket(env.ctx, id)
	if err != nil {
		t.Fatalf("IssueLoginTicket: %v", err)
	}
	h := httpapi.NewOAuthHandler(env.oauth, env.auth)
	req := httptest.NewRequest(http.MethodGet, authorizeURL("bigfred-wizard")+"&login_ticket="+ticket, nil)
	rec := httptest.NewRecorder()
	h.Authorize(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, oauthTestRedirect+"?") && !strings.Contains(loc, "code=") {
		t.Fatalf("Location=%s, want redirect with code", loc)
	}
	if rec.Header().Get("Set-Cookie") != "" {
		t.Fatalf("Set-Cookie=%s, want none", rec.Header().Get("Set-Cookie"))
	}
}

func TestAuthorizeInvalidTicketStripsTicketFromReturnTo(t *testing.T) {
	env := freshOAuthEnv(t)
	defer env.cleanup()

	h := httpapi.NewOAuthHandler(env.oauth, env.auth)
	req := httptest.NewRequest(http.MethodGet, authorizeURL("bigfred-wizard")+"&login_ticket=deadbeef", nil)
	rec := httptest.NewRecorder()
	h.Authorize(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if strings.Contains(loc, "login_ticket") {
		t.Fatalf("Location=%s, login_ticket must be stripped from return_to", loc)
	}
	if !strings.Contains(loc, "ephemeral=1") {
		t.Fatalf("Location=%s, want ephemeral=1", loc)
	}
}

func TestLoginEphemeralReturnsTicketWithoutCookie(t *testing.T) {
	env := freshOAuthEnv(t)
	defer env.cleanup()

	h := httpapi.NewAuthHandler(env.auth, env.layout, nil, nil, false, nil, env.oauth)
	body := map[string]any{
		"login":     "alice",
		"pin":       "1234",
		"layoutId":  env.system.ID,
		"ephemeral": true,
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(string(raw)))
	rec := httptest.NewRecorder()
	h.Login(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Set-Cookie") != "" {
		t.Fatalf("Set-Cookie=%s, want none", rec.Header().Get("Set-Cookie"))
	}
	var res map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("json: %v", err)
	}
	ticket, _ := res["loginTicket"].(string)
	if ticket == "" {
		t.Fatalf("body=%s, want loginTicket", rec.Body.String())
	}
}

func TestLoginNonEphemeralSetsCookie(t *testing.T) {
	env := freshOAuthEnv(t)
	defer env.cleanup()

	h := httpapi.NewAuthHandler(env.auth, env.layout, nil, nil, false, nil, env.oauth)
	body := map[string]any{
		"login":    "alice",
		"pin":      "1234",
		"layoutId": env.system.ID,
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(string(raw)))
	rec := httptest.NewRecorder()
	h.Login(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	cookie := rec.Header().Get("Set-Cookie")
	if !strings.Contains(cookie, httpapi.SessionCookieName+"=") {
		t.Fatalf("Set-Cookie=%s, want %s", cookie, httpapi.SessionCookieName)
	}
}

func authorizeURL(clientID string) string {
	return "/api/v1/auth/oauth/authorize?client_id=" + clientID +
		"&redirect_uri=" + oauthTestRedirect + "&response_type=code&state=abc"
}

func TestLoginEphemeralWithoutOAuthReturns503(t *testing.T) {
	env := freshOAuthEnv(t)
	defer env.cleanup()

	h := httpapi.NewAuthHandler(env.auth, env.layout, nil, nil, false, nil, nil)
	body := map[string]any{
		"login":     "alice",
		"pin":       "1234",
		"layoutId":  env.system.ID,
		"ephemeral": true,
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(string(raw)))
	rec := httptest.NewRecorder()
	h.Login(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
