package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"github.com/keskad/loco/pkgs/bigfred/server/version"
)

func TestVersionHandler_publicJSON(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/api/v1/version", VersionHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var info version.Info
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&info))
	require.Equal(t, "dev", info.Version)
	// TagCommit empty without ELF section; BuildCommit may be empty without ldflags.
	require.Empty(t, info.TagCommit)
}

func TestNewRouter_versionUnauthenticated(t *testing.T) {
	h := NewRouter(RouterConfig{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var info version.Info
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&info))
	require.Equal(t, "dev", info.Version)
}
