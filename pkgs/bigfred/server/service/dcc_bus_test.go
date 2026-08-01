package service

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestProgramNameDeterministic(t *testing.T) {
	if got := programName(1, 2); got != "dcc-bus-1-2" {
		t.Fatalf("programName(1,2) = %q", got)
	}
	if got := programName(99, 7); got != "dcc-bus-99-7" {
		t.Fatalf("programName(99,7) = %q", got)
	}
}

func TestCtlProgramName(t *testing.T) {
	if got := ctlProgramName(1, 2); got != "dcc-bus:dcc-bus-1-2" {
		t.Fatalf("ctlProgramName(1,2) = %q", got)
	}
}

func TestMatchSupervisordProgram(t *testing.T) {
	cases := []struct {
		status, program string
		want            bool
	}{
		{"dcc-bus-1-2", "dcc-bus-1-2", true},
		{"dcc-bus:dcc-bus-1-2", "dcc-bus-1-2", true},
		{"dcc-bus:dcc-bus-1-3", "dcc-bus-1-2", false},
		{"redis", "dcc-bus-1-2", false},
		{"", "dcc-bus-1-2", false},
	}
	for _, tc := range cases {
		if got := matchSupervisordProgram(tc.status, tc.program); got != tc.want {
			t.Fatalf("matchSupervisordProgram(%q, %q) = %v, want %v", tc.status, tc.program, got, tc.want)
		}
	}
}

func TestProgramStatusNilSupervisor(t *testing.T) {
	d := NewDccBusService(DccBusConfig{}, nil, nil, nil, nil, nil)
	_, err := d.ProgramStatus(context.Background(), 1, 2)
	if !errors.Is(err, ErrSupervisordNotWired) {
		t.Fatalf("ProgramStatus: got %v, want ErrSupervisordNotWired", err)
	}
}

func TestStartStopRestartNilSupervisor(t *testing.T) {
	d := NewDccBusService(DccBusConfig{}, nil, nil, nil, nil, nil)
	ctx := context.Background()
	if err := d.StartDccBus(ctx, 1, 2); !errors.Is(err, ErrSupervisordNotWired) {
		t.Fatalf("StartDccBus: got %v", err)
	}
	if err := d.StopDccBus(ctx, 1, 2); !errors.Is(err, ErrSupervisordNotWired) {
		t.Fatalf("StopDccBus: got %v", err)
	}
	if err := d.RestartDccBus(ctx, 1, 2); !errors.Is(err, ErrSupervisordNotWired) {
		t.Fatalf("RestartDccBus: got %v", err)
	}
}

func TestPortAllocationExhausts(t *testing.T) {
	d := NewDccBusService(DccBusConfig{PortMin: 9200, PortMax: 9201}, nil, nil, nil, nil, nil)

	p1, err := d.allocatePortLocked(1, 1)
	if err != nil {
		t.Fatalf("first alloc: %v", err)
	}
	if p1 != 9200 {
		t.Fatalf("first port = %d", p1)
	}
	p2, err := d.allocatePortLocked(2, 1)
	if err != nil {
		t.Fatalf("second alloc: %v", err)
	}
	if p2 != 9201 {
		t.Fatalf("second port = %d", p2)
	}
	if _, err := d.allocatePortLocked(3, 1); !errors.Is(err, ErrNoDccBusPortsAvailable) {
		t.Fatalf("expected ErrNoDccBusPortsAvailable, got %v", err)
	}
}

func TestPortForReportsCachedAllocation(t *testing.T) {
	d := NewDccBusService(DccBusConfig{PortMin: 9300, PortMax: 9309}, nil, nil, nil, nil, nil)
	if _, err := d.allocatePortLocked(7, 3); err != nil {
		t.Fatalf("alloc: %v", err)
	}
	if got := d.PortFor(7, 3); got != 9300 {
		t.Fatalf("PortFor = %d", got)
	}
	if got := d.PortFor(7, 4); got != 0 {
		t.Fatalf("PortFor unknown = %d", got)
	}
}

func TestAppendDccBusTelemetryArgs(t *testing.T) {
	args := appendDccBusTelemetryArgs([]string{"loco-server", "dcc-bus"}, DccBusConfig{
		EnableTelemetry: true,
		OTLPEndpoint:    "127.0.0.1:4317",
	})
	got := strings.Join(args, " ")
	if !strings.Contains(got, "--enable-telemetry") || !strings.Contains(got, "--otel-endpoint 127.0.0.1:4317") {
		t.Fatalf("args = %q", got)
	}

	unchanged := appendDccBusTelemetryArgs([]string{"x"}, DccBusConfig{EnableTelemetry: true})
	if len(unchanged) != 1 {
		t.Fatalf("expected no flags without endpoint, got %v", unchanged)
	}
}
