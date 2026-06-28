package contract

import "testing"

func TestValidWithrottleCode(t *testing.T) {
	if !ValidWithrottleCode("122145") {
		t.Fatal("expected valid")
	}
	if ValidWithrottleCode("12214") {
		t.Fatal("too short")
	}
	if ValidWithrottleCode("1221456") {
		t.Fatal("too long")
	}
	if ValidWithrottleCode("12a145") {
		t.Fatal("non-digit")
	}
}

func TestWithrottlePairReqID(t *testing.T) {
	if got := WithrottlePairReqID("122145"); got != "withrottle:122145" {
		t.Fatalf("got %q", got)
	}
}

func TestRandomPairingCode(t *testing.T) {
	code := RandomPairingCode(nil)
	if !ValidWithrottleCode(code) {
		t.Fatalf("invalid code %q", code)
	}
}

func TestWithrottlePairingDisplayLabel(t *testing.T) {
	if got := WithrottlePairingDisplayLabel("122145"); got != "1 2 2 1 4 5" {
		t.Fatalf("got %q", got)
	}
}
