package contract

import "testing"

func TestFormatVehicleDisplayName(t *testing.T) {
	if got := FormatVehicleDisplayName("ET22", "1175", 42); got != "ET22" {
		t.Fatalf("name: %q", got)
	}
	if got := FormatVehicleDisplayName("", "1175", 42); got != "1175" {
		t.Fatalf("number: %q", got)
	}
	if got := FormatVehicleDisplayName("", "", 42); got != "Loco 42" {
		t.Fatalf("addr: %q", got)
	}
}
