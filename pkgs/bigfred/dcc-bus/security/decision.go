// Package security holds the dcc-bus policy layer: drive-authority and
// roster membership checks against the allowed_vehicles snapshot from
// Redis. Policies never touch the command station or WebSocket layer.
package security

// Decision is the return type of drive-authority checks. Reason is
// machine-readable so the WS handler can forward it in ack payloads.
type Decision struct {
	Allowed bool
	Reason  string
}

// Allow is the canonical positive Decision.
var Allow = Decision{Allowed: true}

// Machine-readable denial reasons forwarded in WS ack and loco.error
// payloads. Keep in sync with web/src/i18n throttle error keys.
const (
	ReasonVehicleNotOnLayout    = "vehicle_not_on_layout"
	ReasonNotAuthorized         = "not_authorized"
	ReasonTrainNotOnLayout      = "train_not_on_layout"
	ReasonTrainNoPoweredMembers = "train_no_powered_members"
	ReasonNotAuthorizedToDrive  = "not_authorized_to_drive"
)

// Deny constructs a negative Decision with a machine-readable reason.
func Deny(reason string) Decision {
	return Decision{Allowed: false, Reason: reason}
}
