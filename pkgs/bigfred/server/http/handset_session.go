package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/validation"
)

type handsetSessionResponse struct {
	Paired           bool  `json:"paired"`
	ExpiresAt        int64 `json:"expiresAt,omitempty"`
	LastSeenAt       int64 `json:"lastSeenAt,omitempty"`
	LayoutID         uint  `json:"layoutId"`
	CommandStationID uint  `json:"commandStationId"`
	UserID           uint  `json:"userId,omitempty"`
}

// HandsetSessionHandler reports whether a no_std handset still has a paired
// Redis session, using the same login+PIN+deviceId auth as pairing.
type HandsetSessionHandler struct {
	auth    *cmd.Auth
	layouts *cmd.Layout
	remote  *cmd.Remote
	audit   cmd.AuditPublisher
	limiter *handsetPairingLimiter
}

func NewHandsetSessionHandler(auth *cmd.Auth, layouts *cmd.Layout, remote *cmd.Remote, audit cmd.AuditPublisher) *HandsetSessionHandler {
	return &HandsetSessionHandler{
		auth: auth, layouts: layouts, remote: remote, audit: audit,
		limiter: newHandsetPairingLimiter(),
	}
}

func (h *HandsetSessionHandler) Status(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxHandsetPairingBody)
	var req handsetPairingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	req.Login = strings.TrimSpace(req.Login)
	req.DeviceID = strings.TrimSpace(req.DeviceID)
	ip := handsetClientIP(r)
	if !h.limiter.allowAt(ip, req.Login, time.Now(), handsetSessionAttempts) {
		h.auditResult(r, 0, 0, req.Login, req.DeviceID, ip, "rate_limited")
		writeJSONError(w, http.StatusTooManyRequests, "rate_limited")
		return
	}
	if req.Login == "" || !validHandsetDeviceID(req.DeviceID) {
		h.auditResult(r, 0, 0, req.Login, req.DeviceID, ip, "invalid_request")
		writeJSONError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	if err := validation.ValidateUserPIN(req.PIN); err != nil {
		h.auditResult(r, 0, 0, req.Login, req.DeviceID, ip, "invalid_pin")
		status, code := svcerrors.UserHTTPStatus(err)
		writeJSONError(w, status, code)
		return
	}
	if h.auth == nil || h.layouts == nil || h.remote == nil {
		h.auditResult(r, 0, 0, req.Login, req.DeviceID, ip, "not_configured")
		writeJSONError(w, http.StatusServiceUnavailable, "remote_not_configured")
		return
	}

	layouts, err := h.layouts.ListSelectable(r.Context())
	if err != nil {
		h.auditResult(r, 0, 0, req.Login, req.DeviceID, ip, "internal_error")
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	layoutID, commandStationID, err := selectHandsetWithrottleStation(r.Context(), h.layouts, layouts)
	if err != nil {
		h.auditResult(r, 0, 0, req.Login, req.DeviceID, ip, "internal_error")
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if commandStationID == 0 {
		h.auditResult(r, 0, 0, req.Login, req.DeviceID, ip, "server_disabled")
		writeRemoteError(w, svcerrors.ErrWithrottleServerDisabled)
		return
	}
	identity, err := h.auth.Login(r.Context(), req.Login, req.PIN, layoutID)
	if err != nil {
		h.auditResult(r, layoutID, 0, req.Login, req.DeviceID, ip, "invalid_credentials")
		status, code := svcerrors.AuthHTTPStatus(err)
		writeJSONError(w, status, code)
		return
	}

	active, ttl, ok, err := h.remote.HandsetSessionByDevice(r.Context(), layoutID, commandStationID, req.DeviceID)
	if err != nil {
		h.auditResult(r, layoutID, identity.User.ID, identity.User.Login, req.DeviceID, ip, "internal_error")
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	h.auditResult(r, layoutID, identity.User.ID, identity.User.Login, req.DeviceID, ip, "success")
	resp := handsetSessionResponse{
		Paired:           ok,
		LayoutID:         layoutID,
		CommandStationID: commandStationID,
	}
	if ok {
		resp.UserID = active.UserID
		resp.LastSeenAt = active.LastSeenAt
		if ttl > 0 {
			resp.ExpiresAt = time.Now().Add(ttl).UnixMilli()
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *HandsetSessionHandler) auditResult(r *http.Request, layoutID, userID uint, login, deviceID, ip, result string) {
	if h.audit == nil {
		return
	}
	_ = h.audit.Publish(
		r.Context(),
		layoutID,
		cmd.AuditActor{UserID: userID, Login: login},
		"audit_handset_session",
		map[string]string{"deviceId": deviceID, "ip": ip, "result": result},
	)
}
