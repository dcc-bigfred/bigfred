// Package wsutil builds WebSocket URLs for loco-server load testing.
package wsutil

import (
	"fmt"
	"net/url"
	"strings"
)

// WithToken appends ?token= when the URL does not already carry one.
func WithToken(wsURL, token string) (string, error) {
	wsURL = strings.TrimSpace(wsURL)
	if wsURL == "" {
		return "", fmt.Errorf("websocket url is required")
	}
	u, err := url.Parse(wsURL)
	if err != nil {
		return "", fmt.Errorf("parse websocket url: %w", err)
	}
	q := u.Query()
	if q.Get("token") == "" && token != "" {
		q.Set("token", token)
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
}

// HTTPToWS converts an http(s) base URL to ws(s) with the same host.
func HTTPToWS(httpBase string) (*url.URL, error) {
	httpBase = strings.TrimSpace(httpBase)
	if httpBase == "" {
		return nil, fmt.Errorf("http-addr is required")
	}
	if !strings.Contains(httpBase, "://") {
		httpBase = "http://" + httpBase
	}
	u, err := url.Parse(httpBase)
	if err != nil {
		return nil, fmt.Errorf("parse http-addr: %w", err)
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	default:
		return nil, fmt.Errorf("unsupported http-addr scheme %q", u.Scheme)
	}
	return u, nil
}

// ControlWS returns the loco-server control-plane WebSocket URL.
func ControlWS(httpBase string) (string, error) {
	u, err := HTTPToWS(httpBase)
	if err != nil {
		return "", err
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/api/v1/ws"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

// DccBusProxyWS returns the reverse-proxy dcc-bus WebSocket URL on loco-server.
func DccBusProxyWS(httpBase string, commandStationID uint) (string, error) {
	if commandStationID == 0 {
		return "", fmt.Errorf("command-station-id is required")
	}
	u, err := HTTPToWS(httpBase)
	if err != nil {
		return "", err
	}
	u.Path = fmt.Sprintf("%s/api/v1/dcc-bus/%d/ws",
		strings.TrimSuffix(u.Path, "/"), commandStationID)
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}
