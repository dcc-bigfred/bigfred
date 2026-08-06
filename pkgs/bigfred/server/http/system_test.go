package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	miclient "github.com/dcc-bigfred/microinit/go/client"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

func mountSystem(h *SystemHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/api/v1/admin/system", h.Get)
	r.Post("/api/v1/admin/system/shutdown", h.Shutdown)
	return r
}

type fakePower struct {
	info    *miclient.DaemonInfo
	infoErr error
}

func (f *fakePower) Info() (*miclient.DaemonInfo, error) {
	if f.infoErr != nil {
		return nil, f.infoErr
	}
	return f.info, nil
}

func (f *fakePower) ShutdownMode(string) error { return nil }

func TestSystemGetUnavailable(t *testing.T) {
	h := NewSystemHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/system", nil)
	rec := httptest.NewRecorder()
	mountSystem(h).ServeHTTP(rec, req)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestSystemShutdownInvalidMode(t *testing.T) {
	ctl := service.NewSystemControl(nil)
	h := NewSystemHandler(ctl)
	body, _ := json.Marshal(map[string]string{"mode": "halt"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/system/shutdown", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mountSystem(h).ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSystemShutdownModeValidationOrder(t *testing.T) {
	err := service.NewSystemControl(nil).RequestShutdown("halt")
	require.True(t, errors.Is(err, service.ErrInvalidShutdownMode))
	err = service.NewSystemControl(nil).RequestShutdown("poweroff")
	require.True(t, errors.Is(err, service.ErrSystemUnavailable))
}

func TestSystemShutdownNotInit(t *testing.T) {
	ctl := service.NewSystemControlWithPower(&fakePower{
		info: &miclient.DaemonInfo{Mode: "supervise"},
	})
	h := NewSystemHandler(ctl)
	body, _ := json.Marshal(map[string]string{"mode": "poweroff"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/system/shutdown", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mountSystem(h).ServeHTTP(rec, req)
	require.Equal(t, http.StatusConflict, rec.Code)
	var env map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&env))
	require.Equal(t, "system_not_init", env["error"])
}

func TestSystemGetInit(t *testing.T) {
	ctl := service.NewSystemControlWithPower(&fakePower{
		info: &miclient.DaemonInfo{Mode: "init"},
	})
	h := NewSystemHandler(ctl)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/system", nil)
	rec := httptest.NewRecorder()
	mountSystem(h).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var info service.SystemInfo
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&info))
	require.Equal(t, "init", info.Mode)
	require.True(t, info.CanShutdown)
}
