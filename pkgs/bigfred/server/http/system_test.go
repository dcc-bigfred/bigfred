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

	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

func mountSystem(h *SystemHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/api/v1/admin/system", h.Get)
	r.Post("/api/v1/admin/system/shutdown", h.Shutdown)
	return r
}

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
