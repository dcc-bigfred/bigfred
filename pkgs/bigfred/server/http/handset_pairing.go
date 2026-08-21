package httpapi

import (
	"context"
	"encoding/json"
	"errors"
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
	handsetPairingAttempts = 30
	maxHandsetPairingBody  = 4096
	maxHandsetDeviceIDLen  = 32
	maxHandsetRateKeys     = 1024
)

type handsetPairingRequest struct {
	Login    string `json:"login"`
	PIN      string `json:"pin"`
	DeviceID string `json:"deviceId"`
	LayoutID uint   `json:"layoutId"`
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

func (l *handsetPairingLimiter) blocked(ip, login string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	pruneHandsetBuckets(l.byIP, now)
	pruneHandsetBuckets(l.byLogin, now)
	ipBucket := currentHandsetBucket(l.byIP[ip], now)
	loginKey := strings.ToLower(login)
	loginBucket := currentHandsetBucket(l.byLogin[loginKey], now)
	return ipBucket.count >= handsetPairingAttempts || loginBucket.count >= handsetPairingAttempts
}

// recordFailure counts one failed auth attempt. Successful logins are not
// counted so a reconnecting handset does not lock itself out. New keys are
// skipped when the map is full so a flood of distinct IPs/logins cannot
// fail-close pairing for everyone.
func (l *handsetPairingLimiter) recordFailure(ip, login string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	pruneHandsetBuckets(l.byIP, now)
	pruneHandsetBuckets(l.byLogin, now)
	loginKey := strings.ToLower(login)
	recordHandsetFailure(l.byIP, ip, now)
	recordHandsetFailure(l.byLogin, loginKey, now)
}

func recordHandsetFailure(buckets map[string]handsetRateBucket, key string, now time.Time) {
	if key == "" {
		return
	}
	bucket := currentHandsetBucket(buckets[key], now)
	if _, ok := buckets[key]; !ok && len(buckets) >= maxHandsetRateKeys {
		return
	}
	if bucket.count >= handsetPairingAttempts {
		buckets[key] = bucket
		return
	}
	bucket.count++
	buckets[key] = bucket
}

func pruneHandsetBuckets(buckets map[string]handsetRateBucket, now time.Time) {
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

func NewHandsetPairingHandler(auth *cmd.Auth, layouts *cmd.Layout, remote *cmd.Remote, audit cmd.AuditPublisher, limiter *handsetPairingLimiter) *HandsetPairingHandler {
	if limiter == nil {
		limiter = newHandsetPairingLimiter()
	}
	return &HandsetPairingHandler{
		auth: auth, layouts: layouts, remote: remote, audit: audit,
		limiter: limiter,
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
	now := time.Now()
	if h.limiter.blocked(ip, req.Login, now) {
		writeJSONError(w, http.StatusTooManyRequests, "rate_limited")
		return
	}
	if req.Login == "" || !validHandsetDeviceID(req.DeviceID) {
		h.limiter.recordFailure(ip, req.Login, now)
		h.auditResult(r, 0, 0, req.Login, req.DeviceID, ip, "invalid_request")
		writeJSONError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	if err := validation.ValidateUserPIN(req.PIN); err != nil {
		h.limiter.recordFailure(ip, req.Login, now)
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
	layoutID, commandStationID, err := selectHandsetWithrottleStation(r.Context(), h.layouts, layouts, req.LayoutID)
	if err != nil {
		h.auditResult(r, 0, 0, req.Login, req.DeviceID, ip, handsetSelectAuditResult(err))
		writeHandsetSelectError(w, err)
		return
	}
	if commandStationID == 0 {
		h.auditResult(r, 0, 0, req.Login, req.DeviceID, ip, "server_disabled")
		writeRemoteError(w, svcerrors.ErrWithrottleServerDisabled)
		return
	}
	identity, err := h.auth.Login(r.Context(), req.Login, req.PIN, layoutID)
	if err != nil {
		h.limiter.recordFailure(ip, req.Login, now)
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

var errHandsetLayoutUnavailable = errors.New("handset layout unavailable")

func selectHandsetWithrottleStation(ctx context.Context, layouts *cmd.Layout, listed []domain.Layout, preferredLayoutID uint) (layoutID, commandStationID uint, err error) {
	if preferredLayoutID != 0 {
		layout, found := layoutByID(listed, preferredLayoutID)
		if !found {
			return 0, 0, errHandsetLayoutUnavailable
		}
		csID, pickErr := pickHandsetWithrottleStation(ctx, layouts, layout)
		return layout.ID, csID, pickErr
	}
	for _, layout := range listed {
		if layout.IsSystem {
			continue
		}
		csID, pickErr := pickHandsetWithrottleStation(ctx, layouts, layout)
		if pickErr != nil {
			return 0, 0, pickErr
		}
		if csID != 0 {
			return layout.ID, csID, nil
		}
	}
	return 0, 0, nil
}

func layoutByID(listed []domain.Layout, id uint) (domain.Layout, bool) {
	for _, layout := range listed {
		if layout.ID == id {
			return layout, true
		}
	}
	return domain.Layout{}, false
}

func pickHandsetWithrottleStation(ctx context.Context, layouts *cmd.Layout, layout domain.Layout) (uint, error) {
	stations, err := layouts.ListCommandStations(ctx, layout.ID)
	if err != nil {
		return 0, err
	}
	for _, station := range stations {
		if station.WithrottleServerEnabled && !station.HideInThrottle {
			return station.ID, nil
		}
	}
	return 0, nil
}

func writeHandsetSelectError(w http.ResponseWriter, err error) {
	if errors.Is(err, errHandsetLayoutUnavailable) {
		writeJSONError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	status, code := svcerrors.LayoutHTTPStatus(err)
	writeJSONErrorCause(w, status, code, err)
}

func handsetSelectAuditResult(err error) string {
	if errors.Is(err, errHandsetLayoutUnavailable) {
		return "invalid_request"
	}
	return "internal_error"
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
