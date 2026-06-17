package protocol

import (
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// LoginRequest mirrors POST /api/v1/auth/login.
type LoginRequest struct {
	Login    string `json:"login"`
	PIN      string `json:"pin"`
	LayoutID uint   `json:"layoutId"`
}

// SudoElevationResponse carries the active sudo grant, or is absent when nil.
type SudoElevationResponse struct {
	GrantedAt time.Time `json:"grantedAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// MeResponse is returned by GET /api/v1/auth/me and POST /auth/login.
type MeResponse struct {
	ID             uint                   `json:"id"`
	Login          string                 `json:"login"`
	Role           domain.Role            `json:"role"`
	EffectiveRole  domain.Role            `json:"effectiveRole"`
	IsSignalman    bool                   `json:"isSignalman"`
	LayoutID       uint                   `json:"layoutId"`
	LayoutName     string                 `json:"layoutName"`
	LayoutIsSystem bool                   `json:"layoutIsSystem"`
	Sudo           *SudoElevationResponse `json:"sudo"`
}
