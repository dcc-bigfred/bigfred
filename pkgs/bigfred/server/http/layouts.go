package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/protocol"
)

// LayoutHandler bundles the endpoints documented under `/api/v1/layouts*` in §4.1.
type LayoutHandler struct {
	svc   *cmd.Layout
	auth  *cmd.Auth
	audit cmd.AuditPublisher
}

// NewLayoutHandler returns a LayoutHandler.
func NewLayoutHandler(svc *cmd.Layout, auth *cmd.Auth, audit cmd.AuditPublisher) *LayoutHandler {
	return &LayoutHandler{svc: svc, auth: auth, audit: audit}
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
	out := make([]protocol.LoginLayoutResponse, 0, len(rows))
	for _, l := range rows {
		out = append(out, protocol.ToLoginLayoutResponse(l))
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
	out := make([]protocol.LayoutResponse, 0, len(rows))
	for _, l := range rows {
		out = append(out, protocol.ToLayoutResponse(l))
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
	_ = json.NewEncoder(w).Encode(protocol.ToLayoutResponse(layout))
}

// Create handles `POST /api/v1/layouts` (admin only).
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
	var req protocol.LayoutCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	layout, err := h.svc.Create(r.Context(), eff, req.ToCreateInput(actor.User.ID))
	if err != nil {
		if errors.Is(err, svcerrors.ErrInterlockingNotFound) {
			writeLayoutInterlockingError(w, err)
			return
		}
		if errors.Is(err, svcerrors.ErrCommandStationNotFound) ||
			errors.Is(err, svcerrors.ErrLayoutNeedsAtLeastOneCommandStation) {
			writeLayoutCommandStationError(w, err)
			return
		}
		writeLayoutError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(protocol.ToLayoutResponse(layout))
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
	out := make([]protocol.CommandStationResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, protocol.ToCommandStationResponse(row))
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
	out := make([]protocol.CommandStationResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, protocol.ToCommandStationResponse(row))
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
	out := make([]protocol.InterlockingResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, protocol.ToInterlockingResponse(row))
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
	out := make([]protocol.InterlockingResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, protocol.ToInterlockingResponse(row))
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
	if h.audit != nil {
		_ = h.audit.Publish(r.Context(), id,
			cmd.AuditActor{UserID: actor.User.ID, Login: actor.User.Login},
			"audit_layout_updated", map[string]string{"name": layout.Name})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(protocol.ToLayoutResponse(layout))
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
	actor, _ := IdentityFromContext(r.Context())
	layout, err := h.svc.SetLocked(r.Context(), id, true)
	if err != nil {
		writeLayoutError(w, err)
		return
	}
	if h.audit != nil {
		_ = h.audit.Publish(r.Context(), id,
			cmd.AuditActor{UserID: actor.User.ID, Login: actor.User.Login},
			"audit_layout_locked", map[string]string{"name": layout.Name})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(protocol.ToLayoutResponse(layout))
}

// Unlock handles `DELETE /api/v1/layouts/{id}/lock` (admin only).
func (h *LayoutHandler) Unlock(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	actor, _ := IdentityFromContext(r.Context())
	layout, err := h.svc.SetLocked(r.Context(), id, false)
	if err != nil {
		writeLayoutError(w, err)
		return
	}
	if h.audit != nil {
		_ = h.audit.Publish(r.Context(), id,
			cmd.AuditActor{UserID: actor.User.ID, Login: actor.User.Login},
			"audit_layout_unlocked", map[string]string{"name": layout.Name})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(protocol.ToLayoutResponse(layout))
}

func writeLayoutError(w http.ResponseWriter, err error) {
	status, code := svcerrors.LayoutHTTPStatus(err)
	writeJSONError(w, status, code)
}

func writeLayoutInterlockingError(w http.ResponseWriter, err error) {
	status, code := svcerrors.LayoutInterlockingHTTPStatus(err)
	writeJSONError(w, status, code)
}

func writeLayoutCommandStationError(w http.ResponseWriter, err error) {
	status, code := svcerrors.LayoutCommandStationHTTPStatus(err)
	writeJSONError(w, status, code)
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
