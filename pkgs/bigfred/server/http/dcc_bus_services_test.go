package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/microinit"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

func TestActionRequiresLayoutId(t *testing.T) {
	fake := &actionFakeSup{}
	dcc := service.NewDccBusService(service.DccBusConfig{}, fake, nil, nil, nil, nil)
	h := NewDccBusServicesHandler(dcc)

	r := chi.NewRouter()
	r.Post("/admin/dcc-bus/{commandStationId}/services/{action}", h.Action)

	req := httptest.NewRequest(http.MethodPost, "/admin/dcc-bus/5/services/start", nil)
	req = req.WithContext(WithIdentity(req.Context(), cmd.Identity{
		User:   domain.User{ID: 1},
		Layout: domain.Layout{ID: 1},
	}))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "invalid_layout" {
		t.Fatalf("error=%q", body["error"])
	}
}

func TestActionStartInvokesService(t *testing.T) {
	fake := &actionFakeSup{}
	dcc := service.NewDccBusService(service.DccBusConfig{}, fake, nil, nil, nil, nil)
	h := NewDccBusServicesHandler(dcc)

	r := chi.NewRouter()
	r.Post("/admin/dcc-bus/{commandStationId}/services/{action}", h.Action)

	req := httptest.NewRequest(http.MethodPost, "/admin/dcc-bus/5/services/start?layoutId=2", nil)
	req = req.WithContext(WithIdentity(req.Context(), cmd.Identity{
		User:   domain.User{ID: 1},
		Layout: domain.Layout{ID: 1},
	}))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fake.lastStart != "dcc-bus-2-5" {
		t.Fatalf("lastStart=%q", fake.lastStart)
	}
}

func TestActionStartSurfacesMessage(t *testing.T) {
	fake := &actionFakeSup{startErr: errors.New(`supervisorctl start: ERROR (no such process)`)}
	dcc := service.NewDccBusService(service.DccBusConfig{}, fake, nil, nil, nil, nil)
	h := NewDccBusServicesHandler(dcc)

	r := chi.NewRouter()
	r.Post("/admin/dcc-bus/{commandStationId}/services/{action}", h.Action)

	req := httptest.NewRequest(http.MethodPost, "/admin/dcc-bus/5/services/start?layoutId=2", nil)
	req = req.WithContext(WithIdentity(req.Context(), cmd.Identity{
		User:   domain.User{ID: 1},
		Layout: domain.Layout{ID: 1},
	}))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "action_failed" {
		t.Fatalf("error=%q", body["error"])
	}
	if body["message"] == "" || body["message"] != fake.startErr.Error() {
		t.Fatalf("message=%q", body["message"])
	}
}

func TestGetStatusListsPrograms(t *testing.T) {
	fake := &actionFakeSup{
		status: []service.ServiceState{
			{Name: "dcc-bus-2-5", State: "running", PID: 11},
		},
	}
	dcc := service.NewDccBusService(service.DccBusConfig{}, fake, nil, nil, nil, nil)
	h := NewDccBusServicesHandler(dcc)

	r := chi.NewRouter()
	r.Get("/admin/dcc-bus/{commandStationId}/services", h.GetStatus)

	req := httptest.NewRequest(http.MethodGet, "/admin/dcc-bus/5/services", nil)
	req = req.WithContext(WithIdentity(req.Context(), cmd.Identity{
		User:   domain.User{ID: 1},
		Layout: domain.Layout{ID: 99},
	}))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Programs []service.DccBusProgramStatus `json:"programs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Programs) != 1 || !body.Programs[0].Running || body.Programs[0].LayoutID != 2 {
		t.Fatalf("programs=%+v", body.Programs)
	}
}

// actionFakeSup implements service.Supervisor for HTTP handler tests.
type actionFakeSup struct {
	status    []service.ServiceState
	startErr  error
	lastStart string
}

func (f *actionFakeSup) Start(context.Context) error                                                { return nil }
func (f *actionFakeSup) Stop(context.Context) error                                                 { return nil }
func (f *actionFakeSup) RunHealthLoop(context.Context, time.Duration, func([]service.ServiceState)) {}
func (f *actionFakeSup) Paths() (string, string)                                                    { return "", "" }
func (f *actionFakeSup) UpsertService(context.Context, string, microinit.ServiceDef) error {
	return nil
}
func (f *actionFakeSup) ReplaceServices(context.Context, string, []microinit.ServiceDef) error {
	return nil
}
func (f *actionFakeSup) RemoveService(context.Context, string, string) error { return nil }
func (f *actionFakeSup) HasService(context.Context, string) (bool, error)    { return false, nil }
func (f *actionFakeSup) CanManage(context.Context, string) (bool, error)     { return true, nil }
func (f *actionFakeSup) StartService(_ context.Context, name string) error {
	f.lastStart = name
	return f.startErr
}
func (f *actionFakeSup) StopService(context.Context, string) error    { return nil }
func (f *actionFakeSup) RestartService(context.Context, string) error { return nil }
func (f *actionFakeSup) Status(context.Context) ([]service.ServiceState, error) {
	return f.status, nil
}
