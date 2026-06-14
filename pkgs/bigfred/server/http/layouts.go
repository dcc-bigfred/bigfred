package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

// LayoutHandler bundles the endpoints documented under
// `/api/v1/layouts*` in §4.1. The public dropdown endpoint
// (`/layouts/login`) lives here too because it is logically part of
// the layouts surface — wiring it next to the admin endpoints keeps
// the JSON shape in one place.
type LayoutHandler struct {
	svc  *service.LayoutService
	auth *service.AuthService
}

// NewLayoutHandler returns a LayoutHandler bound to a LayoutService.
func NewLayoutHandler(svc *service.LayoutService, auth *service.AuthService) *LayoutHandler {
	return &LayoutHandler{svc: svc, auth: auth}
}

// layoutResponse is the canonical JSON shape of a Layout row.
type layoutResponse struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	IsSystem bool   `json:"isSystem"`
	Locked   bool   `json:"locked"`
}

// loginLayoutResponse is the trimmed shape returned by the public
// `/layouts/login` endpoint (§4.1). The UI substitutes the i18n key
// `layout:system_default_label` for rows where `isSystem == true`.
type loginLayoutResponse struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	IsSystem bool   `json:"isSystem"`
}

func toLayoutResponse(l domain.Layout) layoutResponse {
	return layoutResponse{
		ID:       l.ID,
		Name:     l.Name,
		IsSystem: l.IsSystem,
		Locked:   l.Locked,
	}
}

// ListForLogin handles the unauthenticated `GET /api/v1/layouts/login`
// endpoint. It returns every non-locked layout. The frontend reads
// the response BEFORE the user submits the login form to populate the
// layout dropdown.
func (h *LayoutHandler) ListForLogin(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.ListSelectable(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]loginLayoutResponse, 0, len(rows))
	for _, l := range rows {
		out = append(out, loginLayoutResponse{
			ID:       l.ID,
			Name:     l.Name,
			IsSystem: l.IsSystem,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// List handles `GET /api/v1/layouts`. Available to any authenticated
// user (§4.1). Returns the full list — including locked rows — so an
// admin's table view can render the lock state next to the name.
func (h *LayoutHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.ListAll(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]layoutResponse, 0, len(rows))
	for _, l := range rows {
		out = append(out, toLayoutResponse(l))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// Get handles `GET /api/v1/layouts/{id}`.
func (h *LayoutHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	layout, err := h.svc.Get(r.Context(), id)
	if err != nil {
		writeLayoutError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toLayoutResponse(layout))
}

// createRequest models the JSON body of POST /api/v1/layouts.
type createRequest struct {
	Name              string `json:"name"`
	InterlockingIDs   []uint `json:"interlockingIds"`
	CommandStationIDs []uint `json:"commandStationIds"`
	// AdminPIN is the initial layout admin PIN (§7a.7). Empty
	// means "default" — the service falls back to the well-known
	// SystemLayoutDefaultAdminPIN ("0000"), mirroring the
	// bootstrap UX of the system layout.
	AdminPIN string `json:"adminPin"`
}

// Create handles `POST /api/v1/layouts` (admin only). The HTTP wiring
// adds the auth middleware; this handler trusts the identity in
// context.
func (h *LayoutHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	eff, err := h.auth.Effective(r.Context(), actor.User, actor.Layout.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	layout, err := h.svc.Create(r.Context(), eff, service.CreateInput{
		Name:              req.Name,
		CreatedBy:         actor.User.ID,
		InterlockingIDs:   req.InterlockingIDs,
		CommandStationIDs: req.CommandStationIDs,
		AdminPIN:          req.AdminPIN,
	})
	if err != nil {
		if errors.Is(err, service.ErrInterlockingNotFound) {
			writeLayoutInterlockingError(w, err)
			return
		}
		if errors.Is(err, service.ErrCommandStationNotFound) ||
			errors.Is(err, service.ErrLayoutNeedsAtLeastOneCommandStation) {
			writeLayoutCommandStationError(w, err)
			return
		}
		writeLayoutError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toLayoutResponse(layout))
}

// ListCommandStations handles GET /api/v1/layouts/{id}/command-stations.
func (h *LayoutHandler) ListCommandStations(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	rows, err := h.svc.ListCommandStations(r.Context(), id)
	if err != nil {
		writeLayoutCommandStationError(w, err)
		return
	}
	out := make([]commandStationResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toCommandStationResponse(row))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

type setLayoutCommandStationsRequest struct {
	CommandStationIDs []uint `json:"commandStationIds"`
}

// SetCommandStations handles PUT /api/v1/layouts/{id}/command-stations
// (admin only). Replaces the entire attachment set in one shot.
func (h *LayoutHandler) SetCommandStations(w http.ResponseWriter, r *http.Request) {
	layoutID, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	eff, err := h.auth.Effective(r.Context(), actor.User, actor.Layout.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	var req setLayoutCommandStationsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	rows, err := h.svc.SetCommandStations(r.Context(), eff, layoutID, actor.User.ID, req.CommandStationIDs)
	if err != nil {
		writeLayoutCommandStationError(w, err)
		return
	}
	out := make([]commandStationResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toCommandStationResponse(row))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// ListInterlockings handles GET /api/v1/layouts/{id}/interlockings.
func (h *LayoutHandler) ListInterlockings(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	rows, err := h.svc.ListInterlockings(r.Context(), id)
	if err != nil {
		writeLayoutInterlockingError(w, err)
		return
	}
	out := make([]interlockingResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toInterlockingResponse(row))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

type setLayoutInterlockingsRequest struct {
	InterlockingIDs []uint `json:"interlockingIds"`
}

// SetInterlockings handles PUT /api/v1/layouts/{id}/interlockings
// (admin only). Replaces the entire whitelist in one shot — used by
// the multi-select in the layout edit dialog.
func (h *LayoutHandler) SetInterlockings(w http.ResponseWriter, r *http.Request) {
	layoutID, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	eff, err := h.auth.Effective(r.Context(), actor.User, actor.Layout.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	var req setLayoutInterlockingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	rows, err := h.svc.SetInterlockings(r.Context(), eff, layoutID, actor.User.ID, req.InterlockingIDs)
	if err != nil {
		writeLayoutInterlockingError(w, err)
		return
	}
	out := make([]interlockingResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toInterlockingResponse(row))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// updateRequest models the JSON body of PUT /api/v1/layouts/{id}.
//
// `adminPin` carries the layout admin PIN rotation (§7a.7). The
// empty string means "no change" — leaving the dialog field blank
// MUST keep the existing digest. A non-empty value replaces the
// digest after passing the digit / length policy.
type updateRequest struct {
	Name              string `json:"name"`
	InterlockingIDs   []uint `json:"interlockingIds"`
	CommandStationIDs []uint `json:"commandStationIds"`
	AdminPIN          string `json:"adminPin"`
}

// Update handles `PUT /api/v1/layouts/{id}` (admin only). Renames
// non-system layouts, replaces the interlocking whitelist when
// `interlockingIds` is present (including an empty slice), and
// rotates the admin PIN when `adminPin` is non-empty. Every branch
// is open to every effective admin (permanent or sudo) — sudo grants
// the same authority as a permanent admin everywhere (§7a.7).
func (h *LayoutHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	eff, err := h.auth.Effective(r.Context(), actor.User, actor.Layout.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	layout, err := h.svc.Rename(r.Context(), eff, id, req.Name)
	if err != nil {
		writeLayoutError(w, err)
		return
	}
	if req.InterlockingIDs != nil {
		if _, err := h.svc.SetInterlockings(r.Context(), eff, id, actor.User.ID, req.InterlockingIDs); err != nil {
			writeLayoutInterlockingError(w, err)
			return
		}
	}
	if req.CommandStationIDs != nil {
		if _, err := h.svc.SetCommandStations(r.Context(), eff, id, actor.User.ID, req.CommandStationIDs); err != nil {
			writeLayoutCommandStationError(w, err)
			return
		}
	}
	if req.AdminPIN != "" {
		updated, err := h.svc.UpdateAdminPIN(r.Context(), eff, id, req.AdminPIN)
		if err != nil {
			writeLayoutError(w, err)
			return
		}
		layout = updated
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toLayoutResponse(layout))
}

// Delete handles `DELETE /api/v1/layouts/{id}` (admin only).
func (h *LayoutHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	eff, err := h.auth.Effective(r.Context(), actor.User, actor.Layout.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if err := h.svc.Delete(r.Context(), eff, id); err != nil {
		writeLayoutError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Lock handles `POST /api/v1/layouts/{id}/lock` (admin only).
func (h *LayoutHandler) Lock(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	layout, err := h.svc.SetLocked(r.Context(), id, true)
	if err != nil {
		writeLayoutError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toLayoutResponse(layout))
}

// Unlock handles `DELETE /api/v1/layouts/{id}/lock` (admin only).
func (h *LayoutHandler) Unlock(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	layout, err := h.svc.SetLocked(r.Context(), id, false)
	if err != nil {
		writeLayoutError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toLayoutResponse(layout))
}

// writeLayoutError maps every service-level sentinel to the matching
// status + machine-readable code. Unknown errors fall through to
// `internal_error` so the response stays JSON-shaped.
func writeLayoutError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrLayoutNotFound):
		writeJSONError(w, http.StatusNotFound, "layout_not_found")
	case errors.Is(err, service.ErrLayoutNameTaken):
		writeJSONError(w, http.StatusConflict, "layout_name_taken")
	case errors.Is(err, service.ErrLayoutNameRequired):
		writeJSONError(w, http.StatusUnprocessableEntity, "layout_name_required")
	case errors.Is(err, service.ErrSystemLayoutImmutable):
		writeJSONError(w, http.StatusUnprocessableEntity, "default_layout_immutable")
	case errors.Is(err, service.ErrSystemLayoutUndeletable):
		writeJSONError(w, http.StatusUnprocessableEntity, "default_layout_undeletable")
	case errors.Is(err, service.ErrLayoutAdminPINInvalid):
		writeJSONError(w, http.StatusUnprocessableEntity, "layout_admin_pin_invalid")
	case errors.Is(err, service.ErrLayoutForbidden):
		writeJSONError(w, http.StatusForbidden, "forbidden")
	default:
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
	}
}

func writeLayoutInterlockingError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrLayoutNotFound):
		writeJSONError(w, http.StatusNotFound, "layout_not_found")
	case errors.Is(err, service.ErrInterlockingNotFound):
		writeJSONError(w, http.StatusNotFound, "interlocking_not_found")
	case errors.Is(err, service.ErrLayoutForbidden):
		writeJSONError(w, http.StatusForbidden, "forbidden")
	default:
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
	}
}

func writeLayoutCommandStationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrLayoutNotFound):
		writeJSONError(w, http.StatusNotFound, "layout_not_found")
	case errors.Is(err, service.ErrCommandStationNotFound):
		writeJSONError(w, http.StatusNotFound, "command_station_not_found")
	case errors.Is(err, service.ErrLayoutNeedsAtLeastOneCommandStation):
		writeJSONError(w, http.StatusUnprocessableEntity, "layout_needs_at_least_one_command_station")
	case errors.Is(err, service.ErrSystemLayoutCommandStationsImmutable):
		writeJSONError(w, http.StatusUnprocessableEntity, "default_layout_command_stations_immutable")
	case errors.Is(err, service.ErrLayoutForbidden):
		writeJSONError(w, http.StatusForbidden, "forbidden")
	default:
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
	}
}

// parseUintParam pulls a path parameter from chi and parses it as a
// non-zero uint. Returns (0, false) when missing or unparseable so
// the caller can return the right 4xx.
func parseUintParam(r *http.Request, name string) (uint, bool) {
	raw := chi.URLParam(r, name)
	if raw == "" {
		return 0, false
	}
	n, err := strconv.ParseUint(raw, 10, 32)
	if err != nil || n == 0 {
		return 0, false
	}
	return uint(n), true
}
