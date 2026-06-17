package contract

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	// SudoElevationKeyTmpl stores one active sudo grant for (layout, user).
	// The key TTL matches the remaining elevation lifetime.
	SudoElevationKeyTmpl = "bigfred:sudo:%d:%d"
)

// SudoElevationKey returns the Redis key for one (layoutID, userID) pair.
func SudoElevationKey(layoutID, userID uint) string {
	return fmt.Sprintf(SudoElevationKeyTmpl, layoutID, userID)
}

// SudoElevationWire is the JSON payload stored at SudoElevationKey.
type SudoElevationWire struct {
	GrantedAt time.Time `json:"grantedAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// MarshalSudoElevation encodes a sudo grant for Redis SET.
func MarshalSudoElevation(w SudoElevationWire) ([]byte, error) {
	return json.Marshal(w)
}

// UnmarshalSudoElevation decodes a sudo grant from Redis GET.
func UnmarshalSudoElevation(raw []byte) (SudoElevationWire, error) {
	var w SudoElevationWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return SudoElevationWire{}, err
	}
	return w, nil
}
