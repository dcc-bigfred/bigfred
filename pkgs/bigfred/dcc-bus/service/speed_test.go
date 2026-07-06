package service

import (
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

type recordingStation struct {
	lastSpeed uint8
}

func (s *recordingStation) WriteCV(commandstation.Mode, commandstation.LocoCV, ...commandstation.Option) error {
	return nil
}
func (s *recordingStation) ReadCV(commandstation.Mode, commandstation.LocoCV, ...commandstation.Option) (int, error) {
	return 0, nil
}
func (s *recordingStation) SendFn(commandstation.Mode, commandstation.LocoAddr, commandstation.FuncNum, bool) error {
	return nil
}
func (s *recordingStation) ListFunctions(commandstation.LocoAddr) ([]int, error) {
	return nil, nil
}
func (s *recordingStation) SetSpeed(_ commandstation.LocoAddr, speed uint8, _ bool, _ uint8) error {
	s.lastSpeed = speed
	return nil
}
func (s *recordingStation) GetSpeed(commandstation.LocoAddr) (uint8, bool, error) {
	return 0, true, nil
}
func (s *recordingStation) CleanUp() error { return nil }
func (s *recordingStation) ObserveStates() <-chan commandstation.LocoObservation {
	ch := make(chan commandstation.LocoObservation)
	close(ch)
	return ch
}

func TestDCCWriterSetSpeed(t *testing.T) {
	tests := []struct {
		name        string
		payload     uint8
		emergency   bool
		wantWire    uint8
	}{
		{"normal stop", 0, false, 0},
		{"payload 1 maps to first step (wire 2)", 1, false, 2},
		{"drive speed passes through", 10, false, 10},
		{"emergency forces wire 1", 10, true, 1},
		{"emergency overrides payload 0", 0, true, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			station := &recordingStation{}
			writer := &DCCWriter{Station: station, SpeedSteps: 128}
			if err := writer.SetSpeed(3, tc.payload, true, tc.emergency); err != nil {
				t.Fatal(err)
			}
			if station.lastSpeed != tc.wantWire {
				t.Fatalf("wire speed = %d, want %d", station.lastSpeed, tc.wantWire)
			}
		})
	}
}

func TestUISpeedFromWire(t *testing.T) {
	tests := []struct {
		wire, want uint8
	}{
		{0, 0},
		{1, 0},
		{2, 2},
		{127, 127},
	}
	for _, tc := range tests {
		got := contract.UISpeedFromWire(tc.wire)
		if got != tc.want {
			t.Fatalf("UISpeedFromWire(%d) = %d, want %d", tc.wire, got, tc.want)
		}
	}
}

func TestWireSpeedFromPayload(t *testing.T) {
	tests := []struct {
		payload, want uint8
		emergency     bool
	}{
		{0, 0, false},
		{1, 2, false},
		{10, 10, false},
		{1, 1, true},
		{0, 1, true},
	}
	for _, tc := range tests {
		got := WireSpeedFromPayload(tc.payload, tc.emergency)
		if got != tc.want {
			t.Fatalf("WireSpeedFromPayload(%d, emergency=%v) = %d, want %d",
				tc.payload, tc.emergency, got, tc.want)
		}
	}
}

func TestWireSpeedPayloadToUISnapshot(t *testing.T) {
	tests := []struct {
		name      string
		payload   uint8
		emergency bool
		wantWire  uint8
		wantUI    uint8
	}{
		{"first notch", 1, false, 2, 2},
		{"emergency from payload 1", 1, true, 1, 0},
		{"stop", 0, false, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wire := WireSpeedFromPayload(tc.payload, tc.emergency)
			if wire != tc.wantWire {
				t.Fatalf("wire = %d, want %d", wire, tc.wantWire)
			}
			ui := contract.UISpeedFromWire(wire)
			if ui != tc.wantUI {
				t.Fatalf("UI snapshot = %d, want %d", ui, tc.wantUI)
			}
		})
	}
}

// TestNoNonEmergencyWireSpeed1 fails if any non-emergency SetSpeed records
// wire speed 1 on the command station (DCC wire 1 is emergency brake).
func TestNoNonEmergencyWireSpeed1(t *testing.T) {
	station := &recordingStation{}
	writer := &DCCWriter{Station: station, SpeedSteps: 128}
	payloads := []uint8{0, 1, 2, 10, 50, 127}
	for _, payload := range payloads {
		station.lastSpeed = 0
		if err := writer.SetSpeed(3, payload, true, false); err != nil {
			t.Fatalf("SetSpeed(payload=%d): %v", payload, err)
		}
		if station.lastSpeed == 1 {
			t.Fatalf("non-emergency SetSpeed(payload=%d) sent wire speed 1", payload)
		}
	}
}
