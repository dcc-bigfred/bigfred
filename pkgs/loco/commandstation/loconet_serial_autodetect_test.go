package commandstation

import "testing"

func TestResolveSerialDevice(t *testing.T) {
	origList := listSerialPorts
	origExists := serialDeviceExists
	t.Cleanup(func() {
		listSerialPorts = origList
		serialDeviceExists = origExists
	})

	listSerialPorts = func() ([]string, error) {
		return []string{"/dev/ttyS0", "/dev/ttyUSB1", "/dev/ttyACM0"}, nil
	}
	serialDeviceExists = func(path string) bool { return path == "/dev/loconet-63120" }
	got, err := resolveSerialDevice()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != "/dev/loconet-63120" {
		t.Fatalf("got %q, want /dev/loconet-63120 (udev symlink preferred)", got)
	}

	serialDeviceExists = func(string) bool { return false }
	got, err = resolveSerialDevice()
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
