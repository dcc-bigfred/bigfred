package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/microinit"
)

func TestProgramNameDeterministic(t *testing.T) {
	if got := programName(1, 2); got != "dcc-bus-1-2" {
		t.Fatalf("programName(1,2) = %q", got)
	}
	if got := programName(99, 7); got != "dcc-bus-99-7" {
		t.Fatalf("programName(99,7) = %q", got)
	}
}

func TestProgramsForCommandStationNilSupervisor(t *testing.T) {
	d := NewDccBusService(DccBusConfig{}, nil, nil, nil, nil, nil)
	_, err := d.ProgramsForCommandStation(context.Background(), 2)
	if !errors.Is(err, ErrServicesNotWired) {
		t.Fatalf("ProgramsForCommandStation: got %v, want ErrServicesNotWired", err)
	}
}

func TestParseDccBusProgramLayoutID(t *testing.T) {
	if id, ok := parseDccBusProgramLayoutID("dcc-bus-7-3", 3); !ok || id != 7 {
		t.Fatalf("got %d ok=%v", id, ok)
	}
	if _, ok := parseDccBusProgramLayoutID("dcc-bus-7-3", 4); ok {
		t.Fatal("expected mismatch for wrong cs")
	}
	if _, ok := parseDccBusProgramLayoutID("redis", 3); ok {
		t.Fatal("expected non-match")
	}
}

func TestProgramsForCommandStationMergesPortsAndStatus(t *testing.T) {
	fake := &fakeSupervisor{
		status: []ServiceState{
			{Name: "dcc-bus-2-5", State: "running", PID: 42},
			{Name: "dcc-bus-9-5", State: "failed"},
			{Name: "redis", State: "running", PID: 1},
		},
	}
	d := NewDccBusService(DccBusConfig{PortMin: 9200, PortMax: 9209}, fake, nil, nil, nil, nil)
	// Port-only layout 3 (no supervisord row yet) → STOPPED.
	if _, err := d.allocatePortLocked(3, 5); err != nil {
		t.Fatalf("alloc: %v", err)
	}
	// Layout 2 already has a RUNNING row; also give it a port.
	if _, err := d.allocatePortLocked(2, 5); err != nil {
		t.Fatalf("alloc: %v", err)
	}

	got, err := d.ProgramsForCommandStation(context.Background(), 5)
	if err != nil {
		t.Fatalf("ProgramsForCommandStation: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len=%d want 3: %+v", len(got), got)
	}
	byLayout := map[uint]DccBusProgramStatus{}
	for _, p := range got {
		byLayout[p.LayoutID] = p
	}
	if p := byLayout[2]; !p.Running || p.Status != "running" || p.PID != 42 || p.Name != "dcc-bus-2-5" {
		t.Fatalf("layout 2: %+v", p)
	}
	if p := byLayout[3]; p.Running || p.Status != "stopped" || p.Name != "dcc-bus-3-5" {
		t.Fatalf("layout 3 (port-only): %+v", p)
	}
	if p := byLayout[9]; p.Running || p.Status != "failed" || p.Name != "dcc-bus-9-5" {
		t.Fatalf("layout 9 (status-only): %+v", p)
	}
}

func TestStartStopRestartNilSupervisor(t *testing.T) {
	d := NewDccBusService(DccBusConfig{}, nil, nil, nil, nil, nil)
	ctx := context.Background()
	if err := d.StartDccBus(ctx, 1, 2); !errors.Is(err, ErrServicesNotWired) {
		t.Fatalf("StartDccBus: got %v", err)
	}
	if err := d.StopDccBus(ctx, 1, 2); !errors.Is(err, ErrServicesNotWired) {
		t.Fatalf("StopDccBus: got %v", err)
	}
	if err := d.RestartDccBus(ctx, 1, 2); !errors.Is(err, ErrServicesNotWired) {
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

// fakeSupervisor is a minimal Supervisor stub for unit tests.
type fakeSupervisor struct {
	status      []ServiceState
	statusErr   error
	startErr    error
	lastStart   string
	lastStop    string
	lastRestart string
}

func (f *fakeSupervisor) Start(context.Context) error                                        { return nil }
func (f *fakeSupervisor) Stop(context.Context) error                                         { return nil }
func (f *fakeSupervisor) RunHealthLoop(context.Context, time.Duration, func([]ServiceState)) {}
func (f *fakeSupervisor) Paths() (string, string)                                            { return "", "" }
func (f *fakeSupervisor) UpsertService(context.Context, string, microinit.ServiceDef) error {
	return nil
}
func (f *fakeSupervisor) ReplaceServices(context.Context, string, []microinit.ServiceDef) error {
	return nil
}
func (f *fakeSupervisor) RemoveService(context.Context, string, string) error { return nil }
func (f *fakeSupervisor) HasService(context.Context, string) (bool, error)    { return false, nil }
func (f *fakeSupervisor) CanManage(context.Context, string) (bool, error)     { return true, nil }
func (f *fakeSupervisor) StartService(_ context.Context, name string) error {
	f.lastStart = name
	return f.startErr
}
func (f *fakeSupervisor) StopService(_ context.Context, name string) error {
	f.lastStop = name
	return nil
}
func (f *fakeSupervisor) RestartService(_ context.Context, name string) error {
	f.lastRestart = name
	return nil
}
func (f *fakeSupervisor) Status(context.Context) ([]ServiceState, error) {
	if f.statusErr != nil {
		return nil, f.statusErr
	}
	return f.status, nil
}
