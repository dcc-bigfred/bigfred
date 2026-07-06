package withrottle

import (
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/service"
)

func TestDccSpeedFromWire_wire1IsEstopPayload(t *testing.T) {
	if got := dccSpeedFromWire(1, 128); got != 0 {
		t.Fatalf("dccSpeedFromWire(1) = %d, want 0 (e-stop / stop payload)", got)
	}
}

func TestDccSpeedFromWire_roundTripsThroughWireSpeedFromPayload(t *testing.T) {
	for wire := 2; wire <= 126; wire += 17 {
		payload := dccSpeedFromWire(wire, 128)
		if payload == 0 && wire > 1 {
			t.Fatalf("wire %d -> payload 0", wire)
		}
		back := int(service.WireSpeedFromPayload(payload, false))
		if back < 2 {
			t.Fatalf("wire %d -> payload %d -> wire %d", wire, payload, back)
		}
		// WiThrottle wire encoding is lossy; re-encode must land on a moving step.
		if back == 1 {
			t.Fatalf("WireSpeedFromPayload returned emergency wire 1 for payload %d", payload)
		}
	}
}

func TestDccSpeedFromWire_firstNotchPayload(t *testing.T) {
	payload := dccSpeedFromWire(2, 128)
	if payload < 1 {
		t.Fatalf("first moving WiThrottle wire 2 -> payload %d, want >= 1", payload)
	}
	if wire := service.WireSpeedFromPayload(payload, false); wire != 2 && wire != 3 {
		t.Fatalf("payload %d -> wire %d, want first notch (2 or 3)", payload, wire)
	}
}
