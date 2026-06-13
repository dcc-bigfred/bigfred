package errors

// WebSocket-layer error codes returned before a frame reaches the
// router (malformed envelope, unknown type, invalid JSON payload).
const (
	WsCodeBadEnvelope  = "bad_envelope"
	WsCodeUnknownFrame = "unknown_frame"
	WsCodeBadPayload   = "bad_payload"
)

// Session close reasons passed to Session.Close and HandleSessionClose.
const (
	WsCodeSessionSendFailed    = "opened_send_failed"
	WsCodeSessionWsClosed      = "ws_closed"
	WsCodeSessionReadLoopDone  = "read_loop_done"
	WsCodeSessionDeadman       = "deadman"
)
