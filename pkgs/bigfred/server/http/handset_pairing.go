package httpapi

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/remotepairing"
	"github.com/keskad/loco/pkgs/bigfred/remotes/inbound"
	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/validation"
)

const (
	handsetPairingWindow   = time.Minute
	handsetPairingAttempts = 5
	handsetSessionAttempts = 30
	maxHandsetPairingBody  = 4096
	maxHandsetDeviceIDLen  = 32
	maxHandsetRateKeys     = 1024
)

type handsetPairingRequest struct {
	Login    string `json:"login"`
	PIN      string `json:"pin"`
	DeviceID string `json:"deviceId"`
}

type handsetPairingResponse struct {
	PairingCode      string `json:"pairingCode"`
	ExpiresAt        int64  `json:"expiresAt"`
	LayoutID         uint   `json:"layoutId"`
	CommandStationID uint   `json:"commandStationId"`
}

type handsetRateBucket struct {
	start time.Time
	count int
}

type handsetPairingLimiter struct {
	mu      sync.Mutex
	byIP    map[string]handsetRateBucket
	byLogin map[string]handsetRateBucket
}

func newHandsetPairingLimiter() *handsetPairingLimiter {
	return &handsetPairingLimiter{
		byIP:    make(map[string]handsetRateBucket),
		byLogin: make(map[string]handsetRateBucket),
	}
}

func (l *handsetPairingLimiter) allow(ip, login string, now time.Time) bool {
	return l.allowAt(ip, login, now, handsetPairingAttempts)
}

func (l *handsetPairingLimiter) allowAt(ip, login string, now time.Time, max int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	ipBucket := currentHandsetBucket(l.byIP[ip], now)
	loginKey := strings.ToLower(login)
	loginBucket := currentHandsetBucket(l.byLogin[loginKey], now)
	pruneHandsetBuckets(l.byIP, now)
	pruneHandsetBuckets(l.byLogin, now)
	if _, ok := l.byIP[ip]; !ok && len(l.byIP) >= maxHandsetRateKeys {
		return false
	}
	if _, ok := l.byLogin[loginKey]; !ok && len(l.byLogin) >= maxHandsetRateKeys {
		return false
	}
	if ipBucket.count >= max || loginBucket.count >= max {
		return false
	}
	ipBucket.count++
	loginBucket.count++
	l.byIP[ip] = ipBucket
	l.byLogin[loginKey] = loginBucket
	return true
}

func pruneHandsetBuckets(buckets map[string]handsetRateBucket, now time.Time) {
	if len(buckets) < maxHandsetRateKeys {
		return
	}
	for key, bucket := range buckets {
		if now.Sub(bucket.start) >= handsetPairingWindow {
			delete(buckets, key)
		}
	}
}

func currentHandsetBucket(bucket handsetRateBucket, now time.Time) handsetRateBucket {
	if bucket.start.IsZero() || now.Sub(bucket.start) >= handsetPairingWindow {
		return handsetRateBucket{start: now}
	}
	return bucket
}

// HandsetPairingHandler authenticates a no_std handset and starts the existing
// WiThrottle pairing flow without issuing a browser session cookie.
type HandsetPairingHandler struct {
	auth    *cmd.Auth
	layouts *cmd.Layout
	remote  *cmd.Remote
	audit   cmd.AuditPublisher
	limiter *handsetPairingLimiter
}

func NewHandsetPairingHandler(auth *cmd.Auth, layouts *cmd.Layout, remote *cmd.Remote, audit cmd.AuditPublisher) *HandsetPairingHandler {
	return &HandsetPairingHandler{
		auth: auth, layouts: layouts, remote: remote, audit: audit,
		limiter: newHandsetPairingLimiter(),
	}
}

func (h *HandsetPairingHandler) Start(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxHandsetPairingBody)
	var req handsetPairingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	req.Login = strings.TrimSpace(req.Login)
	req.DeviceID = strings.TrimSpace(req.DeviceID)
	ip := handsetClientIP(r)
	if !h.limiter.allow(ip, req.Login, time.Now()) {
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
		writeJSONErrorCause(w, status, code, err)
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
		writeJSONErrorCause(w, http.StatusInternalServerError, "internal_error", err)
		return
	}
	layoutID, commandStationID, err := selectHandsetWithrottleStation(r.Context(), h.layouts, layouts)
	if err != nil {
		h.auditResult(r, 0, 0, req.Login, req.DeviceID, ip, "internal_error")
		writeJSONErrorCause(w, http.StatusInternalServerError, "internal_error", err)
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
		writeJSONErrorCause(w, status, code, err)
		return
	}

	pending, err := h.remote.StartPairing(
		r.Context(),
		layoutID,
		commandStationID,
		identity.User.ID,
		contract.RemoteProtocolWithrottle,
		cmd.RemoteStartPairingInput{
			UserLogin:         identity.User.Login,
			AllowAllVehicles:  true,
			ExpectedClientKey: inbound.ClientKey(contract.RemoteProtocolWithrottle, req.DeviceID),
		},
	)
	if err != nil {
		h.auditResult(r, layoutID, identity.User.ID, identity.User.Login, req.DeviceID, ip, "failed")
		writeRemoteError(w, err)
		return
	}
	h.auditResult(r, layoutID, identity.User.ID, identity.User.Login, req.DeviceID, ip, "success")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(handsetPairingResponse{
		PairingCode:      pending.PairingCode,
		ExpiresAt:        remotepairing.PendingExpiresAt(pending).UnixMilli(),
		LayoutID:         layoutID,
		CommandStationID: commandStationID,
	})
}

func selectHandsetWithrottleStation(ctx context.Context, layouts *cmd.Layout, listed []domain.Layout) (layoutID, commandStationID uint, err error) {
	for _, layout := range listed {
		if layout.IsSystem {
			continue
		}
		stations, listErr := layouts.ListCommandStations(ctx, layout.ID)
		if listErr != nil {
			return 0, 0, listErr
		}
		for _, station := range stations {
			if station.WithrottleServerEnabled && !station.HideInThrottle {
				return layout.ID, station.ID, nil
			}
		}
	}
	return 0, 0, nil
}

func validHandsetDeviceID(id string) bool {
	if id == "" || len(id) > maxHandsetDeviceIDLen {
		return false
	}
	for _, ch := range id {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func handsetClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func (h *HandsetPairingHandler) auditResult(r *http.Request, layoutID, userID uint, login, deviceID, ip, result string) {
	if h.audit == nil {
		return
	}
	_ = h.audit.Publish(
		r.Context(),
		layoutID,
		cmd.AuditActor{UserID: userID, Login: login},
		"audit_handset_pairing",
		map[string]string{"deviceId": deviceID, "ip": ip, "result": result},
	)
}
