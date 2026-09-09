package domain

import "testing"

func TestCommandStationKindWiThrottleIsValid(t *testing.T) {
	if !CommandStationKindWiThrottle.IsValid() {
		t.Fatal("withrottle must be a valid catalogue kind")
	}
	if CommandStationKindWiThrottle.IsLocoNet() {
		t.Fatal("withrottle is not a LocoNet kind")
	}
	found := false
	for _, k := range CommandStationKinds() {
		if k == CommandStationKindWiThrottle {
			found = true
		}
	}
	if !found {
		t.Fatal("CommandStationKinds must include withrottle")
	}
}
