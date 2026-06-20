package contract

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	// AuditStreamKey is the global Redis Stream holding audit entries.
	AuditStreamKey = "bigfred:audit"

	// AuditStreamMaxLen is the approximate MAXLEN passed to XADD so the
	// stream trims automatically without an explicit janitor.
	AuditStreamMaxLen int64 = 5000
)

// AuditEntryWire is the JSON payload stored in each Redis Stream entry.
// StreamID is populated when reading (it's the Redis entry id) and
// intentionally omitted when writing.
type AuditEntryWire struct {
	StreamID   string            `json:"streamId,omitempty"`
	LayoutID   uint              `json:"layoutId"`
	ActorID    uint              `json:"actorId"`
	ActorLogin string            `json:"actorLogin"`
	Msg        string            `json:"msg"`
	Vars       map[string]string `json:"vars,omitempty"`
	OccurredAt int64             `json:"occurredAt"` // unix ms
}

// AuditStreamKey is also used as a format template for potential future
// per-layout streams. AuditLayoutStreamKey returns the key for a specific
// layout and is kept for forward compatibility.
func AuditLayoutStreamKey(layoutID uint) string {
	if layoutID == 0 {
		return AuditStreamKey
	}
	return fmt.Sprintf("bigfred:audit:layout:%d", layoutID)
}

// MarshalAuditEntry serialises an entry for Redis Stream storage.
// StreamID is zeroed before marshalling so it is never persisted inside
// the payload (Redis provides it as the entry key).
func MarshalAuditEntry(e AuditEntryWire) ([]byte, error) {
	e.StreamID = ""
	return json.Marshal(e)
}

// UnmarshalAuditEntry deserialises a stored payload.
func UnmarshalAuditEntry(raw []byte) (AuditEntryWire, error) {
	var e AuditEntryWire
	if err := json.Unmarshal(raw, &e); err != nil {
		return AuditEntryWire{}, err
	}
	if e.OccurredAt == 0 {
		e.OccurredAt = time.Now().UTC().UnixMilli()
	}
	return e, nil
}
