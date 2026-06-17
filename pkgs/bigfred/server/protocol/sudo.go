package protocol

import "time"

// PinRequest mirrors the JSON body for sudo and self-signalman requests.
type PinRequest struct {
	PIN string `json:"pin"`
}

// GrantSignalmanRequest is the admin grant body for layout signalmen.
type GrantSignalmanRequest struct {
	UserID uint `json:"userId"`
}

// SudoResponse echoes the persisted admin grant.
type SudoResponse struct {
	GrantedAt time.Time `json:"grantedAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// ElevationChangedPayload is the auth.elevationChanged WS event body.
type ElevationChangedPayload struct {
	LayoutID uint `json:"layoutId"`
	UserID   uint `json:"userId"`
}
