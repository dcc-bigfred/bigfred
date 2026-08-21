package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/coder/websocket"
	miclient "github.com/dcc-bigfred/microinit/go/client"
	"github.com/go-chi/chi/v5"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

const microinitLogHistoryLines = 300

type microinitWSMessage struct {
	Type  string   `json:"type"`
	Lines []string `json:"lines,omitempty"`
	Text  string   `json:"text,omitempty"`
	Error string   `json:"error,omitempty"`
}

// MicroinitHandler serves admin microinit service listing, info, and log WS.
type MicroinitHandler struct {
	svc  *service.MicroinitControl
	auth *cmd.Auth
}

// NewMicroinitHandler returns a MicroinitHandler. svc/auth may be nil (503/401).
func NewMicroinitHandler(svc *service.MicroinitControl, auth *cmd.Auth) *MicroinitHandler {
	return &MicroinitHandler{svc: svc, auth: auth}
}

// ListServices handles GET /api/v1/admin/microinit/services.
func (h *MicroinitHandler) ListServices(w http.ResponseWriter, _ *http.Request) {
	if h.svc == nil || !h.svc.Available() {
		writeJSONError(w, http.StatusServiceUnavailable, "service_unavailable")
		return
	}
	services, err := h.svc.ListServices()
	if err != nil {
		if errors.Is(err, service.ErrSystemUnavailable) {
			writeJSONError(w, http.StatusServiceUnavailable, "service_unavailable")
			return
		}
		writeJSONErrorCause(w, http.StatusInternalServerError, "internal_error", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"services": services})
}

// Info handles GET /api/v1/admin/microinit/info.
func (h *MicroinitHandler) Info(w http.ResponseWriter, _ *http.Request) {
	if h.svc == nil || !h.svc.Available() {
		writeJSONError(w, http.StatusServiceUnavailable, "service_unavailable")
		return
	}
	info, err := h.svc.Info()
	if err != nil {
		if errors.Is(err, service.ErrSystemUnavailable) {
			writeJSONError(w, http.StatusServiceUnavailable, "service_unavailable")
			return
		}
		writeJSONErrorCause(w, http.StatusInternalServerError, "internal_error", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(info)
}

// StreamLogs handles GET /api/v1/admin/microinit/services/{id}/logs/stream (WebSocket).
// Auth is verified in-handler (cookie / ?token=) like ScanWS — chi
// RequireRole does not reliably gate the WS upgrade itself. Uses
// auth.Effective so sudo-elevated admins match the rest of the admin UI.
func (h *MicroinitHandler) StreamLogs(w http.ResponseWriter, r *http.Request) {
	if h.auth == nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	token := readSessionToken(r)
	if token == "" {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := h.auth.VerifyToken(r.Context(), token)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	eff, err := h.auth.Effective(r.Context(), id.User, id.Layout.ID)
	if err != nil {
		writeJSONErrorCause(w, http.StatusInternalServerError, "internal_error", err)
		return
	}
	if !eff.Has(domain.RoleAdmin) {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}

	serviceID := chi.URLParam(r, "id")
	if err := miclient.ValidateName(serviceID); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	if h.svc == nil || !h.svc.Available() {
		writeJSONError(w, http.StatusServiceUnavailable, "service_unavailable")
		return
	}

	wsConn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}
	defer wsConn.Close(websocket.StatusNormalClosure, "done")

	ctx := r.Context()

	history, err := h.fetchLogHistory(serviceID, microinitLogHistoryLines)
	if err != nil {
		_ = writeMicroinitWS(wsConn, ctx, microinitWSMessage{Type: "error", Error: err.Error()})
		return
	}
	if err := writeMicroinitWS(wsConn, ctx, microinitWSMessage{Type: "history", Lines: history}); err != nil {
		return
	}

	unix, err := h.svc.FollowLogs(serviceID, 0, true)
	if err != nil {
		_ = writeMicroinitWS(wsConn, ctx, microinitWSMessage{Type: "error", Error: err.Error()})
		return
	}
	defer unix.Close()

	// Keepalive + disconnect detection: answer client pings, and when the
	// WebSocket read fails (client gone), close the microinit follow conn so
	// the blocking ReadFrame loop below unblocks instead of leaking.
	go func() {
		defer unix.Close()
		for {
			_, data, err := wsConn.Read(ctx)
			if err != nil {
				return
			}
			var msg struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(data, &msg) != nil {
				continue
			}
			if msg.Type == "ping" {
				_ = writeMicroinitWS(wsConn, ctx, microinitWSMessage{Type: "pong"})
			}
		}
	}()

	for {
		resp, err := h.svc.ReadFrame(unix)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || isClosedConn(err) {
				return
			}
			// Client disconnect closes unix from the ping goroutine; treat
			// resulting read errors as a clean end of stream.
			if ctx.Err() != nil {
				return
			}
			_ = writeMicroinitWS(wsConn, ctx, microinitWSMessage{Type: "error", Error: "stream_failed"})
			return
		}

		switch resp.Type {
		case "log":
			if resp.Line == nil {
				continue
			}
			text := miclient.FormatLogLine(*resp.Line)
			if err := writeMicroinitWS(wsConn, ctx, microinitWSMessage{Type: "line", Text: text}); err != nil {
				return
			}
		case "error":
			msg := resp.Message
			if msg == "" {
				msg = "stream_failed"
			}
			_ = writeMicroinitWS(wsConn, ctx, microinitWSMessage{Type: "error", Error: msg})
			return
		case "ok":
			return
		}
	}
}

func (h *MicroinitHandler) fetchLogHistory(name string, lines int) ([]string, error) {
	unix, err := h.svc.FollowLogs(name, lines, false)
	if err != nil {
		return nil, err
	}
	defer unix.Close()

	out := make([]string, 0, 64)
	for {
		resp, err := h.svc.ReadFrame(unix)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return out, nil
			}
			return nil, err
		}
		switch resp.Type {
		case "log":
			if resp.Line != nil {
				out = append(out, miclient.FormatLogLine(*resp.Line))
			}
		case "ok":
			return out, nil
		case "error":
			msg := resp.Message
			if msg == "" {
				msg = "read_failed"
			}
			return nil, errors.New(msg)
		}
	}
}

func writeMicroinitWS(conn *websocket.Conn, ctx context.Context, msg microinitWSMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, data)
}

func isClosedConn(err error) bool {
	if err == nil {
		return false
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "use of closed network connection") ||
		strings.Contains(msg, "closed")
}
