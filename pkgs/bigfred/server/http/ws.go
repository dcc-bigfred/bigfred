package httpapi

import (
	"net/http"

	"github.com/coder/websocket"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"

	"github.com/keskad/loco/pkgs/bigfred/server/ws"
)

// ServeWS upgrades an authenticated HTTP request to a WebSocket and
// registers a drive session in the Hub (§4.2).
func ServeWS(hub *ws.Hub, auth *cmd.Auth, w http.ResponseWriter, r *http.Request) {
	token := readSessionToken(r)
	if token == "" {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := auth.VerifyToken(r.Context(), token)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}

	session := ws.NewDriveSession(id.User.ID, id.User.Login, id.Layout.ID)
	client := ws.NewClient(conn, hub, session)
	hub.Register(client)
	client.Serve(r.Context())
}
