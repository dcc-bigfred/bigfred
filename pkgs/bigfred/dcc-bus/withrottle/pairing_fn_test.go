package withrottle

import "testing"

func TestPairingFnAccept(t *testing.T) {
	if pairingFnAccept(true, true, false) {
		// force on without rising edge (Engine Driver repeat press)
	} else {
		t.Fatal("force on should accept repeat press")
	}
	if pairingFnAccept(true, false, false) {
		t.Fatal("momentary on without rising edge should reject")
	}
	if !pairingFnAccept(true, false, true) {
		t.Fatal("momentary on with rising edge should accept")
	}
	if pairingFnAccept(false, true, true) {
		t.Fatal("off should never accept")
	}
}

func TestBufferPairingFnDuplicateDigitForceKeys(t *testing.T) {
	var c wireClient
	if _, ready := c.BufferPairingFn(4); ready || c.pairingBufferDigits() != "4" {
		t.Fatalf("first F4: ready=%v buf=%q", ready, c.pairingBufferDigits())
	}
	c.pairingFnRisingEdge(4, true) // latched on
	if _, ready := c.BufferPairingFn(4); ready || c.pairingBufferDigits() != "44" {
		t.Fatalf("second F4 while latched: ready=%v buf=%q", ready, c.pairingBufferDigits())
	}
}
