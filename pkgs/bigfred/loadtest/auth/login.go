// Package auth performs loco-server HTTP login and extracts the session JWT.
package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"

	"github.com/keskad/loco/pkgs/bigfred/server/protocol"
)

const sessionCookieName = "bigfred_session"

// Session carries the authenticated HTTP client and JWT token extracted
// after a successful login.
type Session struct {
	HTTP   *http.Client
	Token  string
	UserID uint
}

// Login authenticates against POST /api/v1/auth/login and returns a cookie
// jar-backed HTTP client plus the session JWT for WebSocket upgrades.
func Login(ctx context.Context, baseURL, login, pin string, layoutID uint) (*Session, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("cookie jar: %w", err)
	}
	client := &http.Client{Jar: jar}

	apiBase, err := apiV1Base(baseURL)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(protocol.LoginRequest{
		Login:    strings.TrimSpace(login),
		PIN:      pin,
		LayoutID: layoutID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal login: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+"/auth/login", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("login failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	var me protocol.MeResponse
	if err := json.NewDecoder(resp.Body).Decode(&me); err != nil {
		return nil, fmt.Errorf("decode login response: %w", err)
	}

	token, err := tokenFromJar(jar, apiBase)
	if err != nil {
		return nil, err
	}

	return &Session{
		HTTP:   client,
		Token:  token,
		UserID: me.ID,
	}, nil
}

func apiV1Base(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("http-addr is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse http-addr: %w", err)
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/api/v1"
	return strings.TrimSuffix(u.String(), "/"), nil
}

func tokenFromJar(jar *cookiejar.Jar, apiBase string) (string, error) {
	u, err := url.Parse(apiBase)
	if err != nil {
		return "", fmt.Errorf("parse api base: %w", err)
	}
	for _, c := range jar.Cookies(u) {
		if c.Name == sessionCookieName && c.Value != "" {
			return c.Value, nil
		}
	}
	return "", fmt.Errorf("session cookie %q not found after login", sessionCookieName)
}
