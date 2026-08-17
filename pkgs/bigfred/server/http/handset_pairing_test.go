package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandsetPairingLimiterKeysByIPAndLogin(t *testing.T) {
	now := time.Unix(100, 0)
	limiter := newHandsetPairingLimiter()
	for i := 0; i < handsetPairingAttempts; i++ {
		if !limiter.allow("10.0.0.1", "alice", now) {
			t.Fatalf("attempt %d unexpectedly rejected", i)
		}
	}
	if limiter.allow("10.0.0.1", "bob", now) {
		t.Fatal("IP limit should reject a different login")
	}
	if limiter.allow("10.0.0.2", "ALICE", now) {
		t.Fatal("login limit should be case-insensitive across IPs")
	}
	if !limiter.allow("10.0.0.1", "alice", now.Add(handsetPairingWindow)) {
		t.Fatal("window should reset")
	}
}

func TestValidHandsetDeviceID(t *testing.T) {
	for _, valid := range []string{"1", "4242", "000042"} {
		if !validHandsetDeviceID(valid) {
			t.Fatalf("%q should be valid", valid)
		}
	}
	for _, invalid := range []string{"", "device", "42-42"} {
		if validHandsetDeviceID(invalid) {
			t.Fatalf("%q should be invalid", invalid)
		}
	}
}

func TestHandsetPairingRejectsInvalidPINWithoutEcho(t *testing.T) {
	h := NewHandsetPairingHandler(nil, nil, nil, nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/remotes/handset-pairing",
		strings.NewReader(`{"login":"ops","pin":"12x","deviceId":"4242"}`),
	)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	h.Start(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "user_pin_invalid") {
		t.Fatalf("body=%s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "12x") {
		t.Fatal("response echoed PIN")
	}
}

func TestHandsetPairingRouteIsPublic(t *testing.T) {
	router := NewRouter(RouterConfig{})
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/remotes/handset-pairing",
		strings.NewReader(`{"login":"ops","pin":"1234","deviceId":"4242"}`),
	)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "remote_not_configured") {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestHandsetSessionRouteIsPublic(t *testing.T) {
	router := NewRouter(RouterConfig{})
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/remotes/handset-session",
		strings.NewReader(`{"login":"ops","pin":"1234","deviceId":"4242"}`),
	)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "remote_not_configured") {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestHandsetSessionRejectsInvalidPINWithoutEcho(t *testing.T) {
	h := NewHandsetSessionHandler(nil, nil, nil, nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/remotes/handset-session",
		strings.NewReader(`{"login":"ops","pin":"12x","deviceId":"4242"}`),
	)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	h.Status(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "user_pin_invalid") {
		t.Fatalf("body=%s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "12x") {
		t.Fatal("response echoed PIN")
	}
}
