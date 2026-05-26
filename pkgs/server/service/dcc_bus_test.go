package service

import (
	"errors"
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

func TestPortAllocationExhausts(t *testing.T) {
	d := NewDccBusService(DccBusConfig{PortMin: 9200, PortMax: 9201}, nil, nil, nil)

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
	d := NewDccBusService(DccBusConfig{PortMin: 9300, PortMax: 9309}, nil, nil, nil)
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
