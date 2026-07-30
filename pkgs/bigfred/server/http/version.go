package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/keskad/loco/pkgs/bigfred/server/version"
)

// VersionHandler serves GET /api/v1/version (public, no auth).
func VersionHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(version.Get())
}
