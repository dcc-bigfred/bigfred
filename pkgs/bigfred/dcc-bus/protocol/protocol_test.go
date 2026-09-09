package protocol

import (
	"encoding/json"
	"testing"

	"github.com/dcc-bigfred/bigfred/pkgs/bigfred/contract"
)

func TestFrameRoundTrip(t *testing.T) {
	env, err := Frame(TypeLocoState, contract.LocoStateWire{Address: 3, Speed: 42, Forward: true, At: 123})
	if err != nil {
		t.Fatalf("frame: %v", err)
	}
	if env.Type != TypeLocoState {
		t.Fatalf("type = %q", env.Type)
	}
	var snap contract.LocoStateWire
	if err := json.Unmarshal(env.Payload, &snap); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if snap.Address != 3 || snap.Speed != 42 || !snap.Forward {
		t.Fatalf("unexpected payload: %#v", snap)
	}
}

func TestFrameWithID(t *testing.T) {
	env, err := FrameWithID(TypeAck, "req-1", AckPayload{OK: true})
	if err != nil {
		t.Fatalf("frame: %v", err)
	}
	if env.ID != "req-1" {
		t.Fatalf("id = %q", env.ID)
	}
	var ack AckPayload
	if err := json.Unmarshal(env.Payload, &ack); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !ack.OK || ack.Error != "" {
		t.Fatalf("ack = %#v", ack)
	}
}

func TestAckPayloadErrorsRoundTrip(t *testing.T) {
	raw, err := json.Marshal(AckPayload{OK: true, Errors: []uint16{2, 500}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("map: %v", err)
	}
	if _, ok := m["errors"]; !ok {
		t.Fatalf("errors missing: %s", raw)
	}
	empty, err := json.Marshal(AckPayload{OK: true})
	if err != nil {
		t.Fatalf("marshal empty: %v", err)
	}
	var emptyMap map[string]any
	if err := json.Unmarshal(empty, &emptyMap); err != nil {
		t.Fatalf("empty map: %v", err)
	}
	if _, ok := emptyMap["errors"]; ok {
		t.Fatalf("empty errors should omit: %s", empty)
	}
}
