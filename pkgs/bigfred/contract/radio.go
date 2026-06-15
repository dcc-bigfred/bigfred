package contract

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

const (
	// RadioInterlockingStreamTmpl stores the signalman group-chat stream
	// for one interlocking on a layout (TTL 4h, §4.4.4).
	RadioInterlockingStreamTmpl = "bigfred:radio:layout:%d:interlocking:%d"

	// RadioUserStreamTmpl stores a driver's personal stream on a layout.
	RadioUserStreamTmpl = "bigfred:radio:layout:%d:user:%d"
)

// RadioInterlockingStreamKey is the Redis stream key for interlocking chat.
func RadioInterlockingStreamKey(layoutID, interlockingID uint) string {
	return fmt.Sprintf(RadioInterlockingStreamTmpl, layoutID, interlockingID)
}

// RadioUserStreamKey is the Redis stream key for a driver's personal chat.
func RadioUserStreamKey(layoutID, userID uint) string {
	return fmt.Sprintf(RadioUserStreamTmpl, layoutID, userID)
}

// WS action / event type strings (§4.2).
const (
	TypeRadioSend    = "radio.send"
	TypeRadioReplay  = "radio.replay"
	TypeRadioMessage = "radio.message"
	TypeRadioHistory = "radio.history"
)

// RadioUserWire identifies a user on the wire.
type RadioUserWire struct {
	UserID uint   `json:"userId"`
	Login  string `json:"login"`
}

// RadioTargetWire is exactly one of userId or interlockingId.
type RadioTargetWire struct {
	UserID         *uint `json:"userId,omitempty"`
	InterlockingID *uint `json:"interlockingId,omitempty"`
}

// RadioContextWire carries denormalized vehicle or train context.
type RadioContextWire struct {
	Vehicle *RadioContextEntityWire `json:"vehicle,omitempty"`
	Train   *RadioContextEntityWire `json:"train,omitempty"`
}

// RadioContextEntityWire is a context id + display name pair.
type RadioContextEntityWire struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// RadioInterlockingWire identifies the signal box a message was sent from.
type RadioInterlockingWire struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// RadioMessageWire is the server → client push / replay envelope (§4.2).
type RadioMessageWire struct {
	MessageID string            `json:"messageId"`
	From      RadioUserWire     `json:"from"`
	FromInterlocking *RadioInterlockingWire `json:"fromInterlocking,omitempty"`
	To        RadioTargetWire   `json:"to"`
	Context   RadioContextWire  `json:"context"`
	Phrase    domain.RadioPhrase `json:"phrase"`
	Note      string            `json:"note,omitempty"`
	SentAt    int64             `json:"sentAt"`
}

// RadioSendPayload is the client → server radio.send body.
type RadioSendPayload struct {
	To struct {
		UserID         *uint `json:"userId"`
		InterlockingID *uint `json:"interlockingId"`
	} `json:"to"`
	Context struct {
		VehicleID *uint `json:"vehicleId"`
		TrainID   *uint `json:"trainId"`
	} `json:"context"`
	Phrase domain.RadioPhrase `json:"phrase"`
	Note   string             `json:"note,omitempty"`
}

// RadioReplayPayload is the client → server radio.replay body.
type RadioReplayPayload struct {
	Scope          string `json:"scope"`
	InterlockingID uint   `json:"interlockingId,omitempty"`
	Limit          int    `json:"limit,omitempty"`
}

// RadioHistoryWire wraps a replay burst (§4.2).
type RadioHistoryWire struct {
	Messages []RadioMessageWire `json:"messages"`
}

// MessageWireFromDomain maps a domain message to the wire push shape.
func MessageWireFromDomain(msg domain.RadioMessage) RadioMessageWire {
	out := RadioMessageWire{
		MessageID: msg.ID,
		From: RadioUserWire{
			UserID: msg.FromUserID,
			Login:  msg.FromLogin,
		},
		Phrase: msg.Phrase,
		Note:   msg.Note,
		SentAt: msg.SentAt.UTC().UnixMilli(),
	}
	if msg.FromInterlockingID != nil && msg.FromInterlockingName != "" {
		out.FromInterlocking = &RadioInterlockingWire{
			ID:   *msg.FromInterlockingID,
			Name: msg.FromInterlockingName,
		}
	}
	if msg.ToUserID != nil {
		out.To.UserID = msg.ToUserID
	}
	if msg.ToInterlockingID != nil {
		out.To.InterlockingID = msg.ToInterlockingID
	}
	if msg.ContextVehicleID != nil {
		out.Context.Vehicle = &RadioContextEntityWire{
			ID:   *msg.ContextVehicleID,
			Name: msg.ContextName,
		}
	}
	if msg.ContextTrainID != nil {
		out.Context.Train = &RadioContextEntityWire{
			ID:   *msg.ContextTrainID,
			Name: msg.ContextName,
		}
	}
	return out
}

// MarshalRadioMessage serialises a domain message for Redis stream storage.
func MarshalRadioMessage(msg domain.RadioMessage) ([]byte, error) {
	return json.Marshal(msg)
}

// UnmarshalRadioMessage deserialises a Redis stream payload.
func UnmarshalRadioMessage(raw []byte) (domain.RadioMessage, error) {
	var msg domain.RadioMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return domain.RadioMessage{}, err
	}
	if msg.SentAt.IsZero() {
		msg.SentAt = time.Now().UTC()
	}
	return msg, nil
}
