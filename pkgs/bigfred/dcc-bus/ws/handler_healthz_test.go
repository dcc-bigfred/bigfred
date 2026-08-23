package ws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeStationHealth struct{ ok bool }

func (f fakeStationHealth) Reachable() bool { return f.ok }

func TestHealthzOKWhenNoStationHealth(t *testing.T) {
	s := NewServer(ServerConfig{})
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHealthzOKWhenStationReachable(t *testing.T) {
	s := NewServer(ServerConfig{StationHealth: fakeStationHealth{ok: true}})
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHealthzUnavailableWhenStationUnreachable(t *testing.T) {
	s := NewServer(ServerConfig{StationHealth: fakeStationHealth{ok: false}})
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	var body struct {
		Status string `json:"status"`
		Code   string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	if body.Status != "unhealthy" || body.Code != "station_unreachable" {
		t.Fatalf("body = %+v", body)
	}
}
