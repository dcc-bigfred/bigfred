package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"time"

	"github.com/coder/websocket"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

type scanWSFrame struct {
	Type   string `json:"type"`
	Name   string `json:"name,omitempty"`
	URI    string `json:"uri,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// ScanWS handles GET /api/v1/command-stations/scan/ws (admin). Streams
// dcc-bus scan hits as WebSocket JSON frames.
func (h *CommandStationHandler) ScanWS(w http.ResponseWriter, r *http.Request) {
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
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if !eff.Has(domain.RoleAdmin) {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns:     h.allowedOrigins,
		InsecureSkipVerify: len(h.allowedOrigins) == 0,
	})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	ctx := conn.CloseRead(r.Context())

	release, err := service.BeginCommandStationScan()
	if err != nil {
		_ = writeScanWSFrame(ctx, conn, scanWSFrame{
			Type:   "error",
			Detail: "scan already running",
		})
		_ = writeScanWSFrame(ctx, conn, scanWSFrame{Type: "done"})
		return
	}
	defer release()

	stderr, runErr := service.StreamScanCommandStations(ctx, h.executable, func(c commandstation.DetectedConnection) error {
		return writeScanWSFrame(ctx, conn, scanWSFrame{
			Type: "connection",
			Name: c.Name,
			URI:  c.URI,
		})
	})

	if runErr != nil && !errors.Is(runErr, context.Canceled) && !errors.Is(runErr, context.DeadlineExceeded) {
		detail := formatScanErrorDetail(runErr, stderr)
		_ = writeScanWSFrame(ctx, conn, scanWSFrame{Type: "error", Detail: detail})
	}
	_ = writeScanWSFrame(ctx, conn, scanWSFrame{Type: "done"})
}

func writeScanWSFrame(ctx context.Context, conn *websocket.Conn, frame scanWSFrame) error {
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	data, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	return conn.Write(writeCtx, websocket.MessageText, data)
}

func formatScanErrorDetail(runErr error, stderr string) string {
	var b string
	if runErr != nil {
		b = runErr.Error()
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			b = fmt.Sprintf("exit status %d", exitErr.ExitCode())
		}
	}
	if stderr != "" {
		if b != "" {
			b += "\n"
		}
		b += stderr
	}
	if b == "" {
		b = "scan_failed"
	}
	return truncateScanDetail(b, 4096)
}

func truncateScanDetail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
