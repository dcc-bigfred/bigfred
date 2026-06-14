package domain

// PresenceUser is one row in the layout dashboard "online users"
// table (§6.3c). One entry per user regardless of how many WS tabs
// they have open.
type PresenceUser struct {
	UserID               uint                  `json:"userId"`
	Login                string                `json:"login"`
	Role                 Role                  `json:"role"`
	OccupiedInterlocking *OccupiedInterlocking `json:"occupiedInterlocking,omitempty"`
}

// OccupiedInterlocking is the signal box a user currently staffs.
type OccupiedInterlocking struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}
