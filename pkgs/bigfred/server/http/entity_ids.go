package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// parseVehicleIDParam pulls a path parameter and validates the V- prefix.
func parseVehicleIDParam(r *http.Request, name string) (domain.VehicleID, bool) {
	raw := chi.URLParam(r, name)
	id, ok := domain.ParseVehicleID(raw)
	return id, ok
}

// parseTrainIDParam pulls a path parameter and validates the T- prefix.
func parseTrainIDParam(r *http.Request, name string) (domain.TrainID, bool) {
	raw := chi.URLParam(r, name)
	id, ok := domain.ParseTrainID(raw)
	return id, ok
}
