package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

func TestHandsetPairingLimiterKeysByIPAndLogin(t *testing.T) {
	now := time.Unix(100, 0)
	limiter := newHandsetPairingLimiter()
	for i := 0; i < handsetPairingAttempts; i++ {
		if limiter.blocked("10.0.0.1", "alice", now) {
			t.Fatalf("attempt %d unexpectedly blocked", i)
		}
		limiter.recordFailure("10.0.0.1", "alice", now)
	}
	if !limiter.blocked("10.0.0.1", "bob", now) {
		t.Fatal("IP limit should reject a different login")
	}
	if !limiter.blocked("10.0.0.2", "ALICE", now) {
		t.Fatal("login limit should be case-insensitive across IPs")
	}
	if limiter.blocked("10.0.0.1", "alice", now.Add(handsetPairingWindow)) {
		t.Fatal("window should reset")
	}
}

func TestHandsetPairingLimiterIgnoresSuccess(t *testing.T) {
	now := time.Unix(200, 0)
	limiter := newHandsetPairingLimiter()
	for i := 0; i < handsetPairingAttempts*3; i++ {
		if limiter.blocked("10.0.0.1", "alice", now) {
			t.Fatalf("successes must not count toward the limit (i=%d)", i)
		}
	}
}

func TestHandsetPairingLimiterDoesNotFailClosedOnNewKeys(t *testing.T) {
	now := time.Unix(300, 0)
	limiter := newHandsetPairingLimiter()
	for i := 0; i < maxHandsetRateKeys; i++ {
		limiter.recordFailure("10.0.0.1", fmt.Sprintf("user-%d", i), now)
	}
	if limiter.blocked("10.0.0.2", "victim", now) {
		t.Fatal("a new key must not be fail-closed when the map is full")
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
	h := NewHandsetPairingHandler(nil, nil, nil, nil, nil)
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
	h := NewHandsetSessionHandler(nil, nil, nil, nil, nil)
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

type recordingAudit struct {
	n int
}

func (a *recordingAudit) Publish(context.Context, uint, cmd.AuditActor, string, map[string]string) error {
	a.n++
	return nil
}

func TestHandsetPairingSkipsAuditWhenRateLimited(t *testing.T) {
	audit := &recordingAudit{}
	limiter := newHandsetPairingLimiter()
	now := time.Now()
	for i := 0; i < handsetPairingAttempts; i++ {
		limiter.recordFailure("10.0.0.1", "ops", now)
	}
	h := NewHandsetPairingHandler(nil, nil, nil, audit, limiter)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/remotes/handset-pairing",
		strings.NewReader(`{"login":"ops","pin":"1234","deviceId":"4242"}`),
	)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	h.Start(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if audit.n != 0 {
		t.Fatalf("rate-limited requests must not write audit events, got %d", audit.n)
	}
}

func TestHandsetSessionSkipsAuditWhenRateLimited(t *testing.T) {
	audit := &recordingAudit{}
	limiter := newHandsetPairingLimiter()
	now := time.Now()
	for i := 0; i < handsetPairingAttempts; i++ {
		limiter.recordFailure("10.0.0.1", "ops", now)
	}
	h := NewHandsetSessionHandler(nil, nil, nil, audit, limiter)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/remotes/handset-session",
		strings.NewReader(`{"login":"ops","pin":"1234","deviceId":"4242"}`),
	)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	h.Status(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if audit.n != 0 {
		t.Fatalf("rate-limited requests must not write audit events, got %d", audit.n)
	}
}

func TestSelectHandsetWithrottleStationRejectsUnlistedLayout(t *testing.T) {
	listed := []domain.Layout{{ID: 2, Name: "club"}}
	_, _, err := selectHandsetWithrottleStation(context.Background(), nil, listed, 99)
	if !errors.Is(err, errHandsetLayoutUnavailable) {
		t.Fatalf("err=%v", err)
	}
}

func TestWriteHandsetSelectErrorHidesUnlistedLayout(t *testing.T) {
	rec := httptest.NewRecorder()
	writeHandsetSelectError(rec, errHandsetLayoutUnavailable)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "invalid_request") {
		t.Fatalf("body=%s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "layout") {
		t.Fatalf("response leaked layout detail: %s", rec.Body.String())
	}
}
