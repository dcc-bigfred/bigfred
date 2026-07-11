package commandstation

import "testing"

func TestResolveSerialDevice(t *testing.T) {
	orig := listSerialPorts
	t.Cleanup(func() { listSerialPorts = orig })

	listSerialPorts = func() ([]string, error) {
		return []string{"/dev/ttyS0", "/dev/ttyUSB1", "/dev/ttyACM0"}, nil
	}
	got, err := resolveSerialDevice()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != "/dev/ttyUSB1" {
		t.Fatalf("got %q, want /dev/ttyUSB1 (USB preferred)", got)
	}

	listSerialPorts = func() ([]string, error) { return nil, nil }
	if _, err := resolveSerialDevice(); err == nil {
		t.Fatalf("expected error on empty port list")
	}
}
