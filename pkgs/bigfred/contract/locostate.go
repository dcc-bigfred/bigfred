package contract

import (
	"encoding/json"
	"fmt"
)

// Per-locomotive state cache (dcc-bus writes; dcc-bus and loco-server read).

const (
	// LocoStateKeyTmpl is the Redis STRING key holding the last-known
	// state snapshot for one locomotive on a layout. dcc-bus applies a
	// TTL so stale rows evict after a roster removal. Verbs: layoutID,
	// addr.
	LocoStateKeyTmpl = "loco:state:%d:%d"
)

// LocoStateKey is the Redis STRING key for one locomotive's snapshot.
func LocoStateKey(layoutID uint, addr uint16) string {
	return fmt.Sprintf(LocoStateKeyTmpl, layoutID, addr)
}

// LocoStateWire is the authoritative state snapshot for one locomotive:
// the JSON stored at LocoStateKey and mirrored on the event channel. The
// daemon emits one frame per loco whenever it observes a change (its own
// poller, an external throttle, or a script). ControlledByUserID is 0 when
// nobody is driving; Source is a free-form origin hint ("throttle" |
// "script" | "external" | "poller"); At is a unix-ms stamp (see NowMS).
type LocoStateWire struct {
	Address            uint16 `json:"address"`
	Speed              uint8  `json:"speed"`
	Forward            bool   `json:"forward"`
	Functions          []bool `json:"functions"`
	ControlledByUserID uint   `json:"controlledByUserId,omitempty"`
	Source             string `json:"source,omitempty"`
	At                 int64  `json:"at"`
}

// BuildLocoStatePayload marshals the per-loco snapshot stored at
// LocoStateKey(layoutID, address) from primitive inputs.
func BuildLocoStatePayload(address uint16, speed uint8, forward bool, functions []bool, controlledByUserID uint, source string, at int64) ([]byte, error) {
	return json.Marshal(LocoStateWire{
		Address:            address,
		Speed:              speed,
		Forward:            forward,
		Functions:          functions,
		ControlledByUserID: controlledByUserID,
		Source:             source,
		At:                 at,
	})
}
