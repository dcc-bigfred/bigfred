package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/keskad/loco/pkgs/server/domain"
	"github.com/keskad/loco/pkgs/server/service"
)

// LayoutHandler bundles the endpoints documented under
// `/api/v1/layouts*` in §4.1. The public dropdown endpoint
// (`/layouts/login`) lives here too because it is logically part of
// the layouts surface — wiring it next to the admin endpoints keeps
// the JSON shape in one place.
type LayoutHandler struct {
	svc *service.LayoutService
}

// NewLayoutHandler returns a LayoutHandler bound to a LayoutService.
func NewLayoutHandler(svc *service.LayoutService) *LayoutHandler {
	return &LayoutHandler{svc: svc}
}

// layoutResponse is the canonical JSON shape of a Layout row. The
// `commandStations` slice promised by §4.1 will be added together
// with the command-station catalogue — for now it is omitted so the
// payload only carries fields backed by real data.
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
// endpoint. It returns every non-locked layout (the system layout is
// always present because it can never be locked). The frontend reads
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

// createRequest models the JSON body of POST /api/v1/layouts. The
// spec also lists `commandStationIds:[id,...]` — see the note on
// service.LayoutService.Create for why it is deferred.
type createRequest struct {
	Name string `json:"name"`
}

// Create handles `POST /api/v1/layouts` (admin only). The HTTP wiring
// adds the auth middleware; this handler trusts the identity in
// context.
func (h *LayoutHandler) Create(w http.ResponseWriter, r *http.Request) {
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	layout, err := h.svc.Create(r.Context(), service.CreateInput{
		Name:      req.Name,
		CreatedBy: id.User.ID,
	})
	if err != nil {
		writeLayoutError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toLayoutResponse(layout))
}

// updateRequest models the JSON body of PUT /api/v1/layouts/{id}.
// Today the only mutable field is `name` (§4.1: "rename only").
type updateRequest struct {
	Name string `json:"name"`
}

// Update handles `PUT /api/v1/layouts/{id}` (admin only).
func (h *LayoutHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	layout, err := h.svc.Rename(r.Context(), id, req.Name)
	if err != nil {
		writeLayoutError(w, err)
		return
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
	if err := h.svc.Delete(r.Context(), id); err != nil {
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
	case errors.Is(err, service.ErrSystemLayoutCannotBeLocked):
		writeJSONError(w, http.StatusUnprocessableEntity, "default_layout_cannot_be_locked")
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
