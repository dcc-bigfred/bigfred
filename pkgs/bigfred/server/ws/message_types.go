package ws

// Control-plane WebSocket frame types (§4.2, §4.6).

const (
	TypeSessionSetCommandStation = "session.setCommandStation"
	TypeSystemRadioStop          = "system.radioStop"
	TypeSystemEStopTarget        = "system.estopTarget"
)
