package security_test

import (
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/security"
)

func TestEStopTargetCanStopOwner(t *testing.T) {
	sec := security.EStopTargetSecurityContext{}
	eff := domain.NewEffectiveRoles(domain.RoleDriver)
	if d := sec.CanStop(eff, 10, false, 10, nil); !d.Allowed {
		t.Fatalf("owner should stop, got %q", d.Reason)
	}
}

func TestEStopTargetCanStopController(t *testing.T) {
	sec := security.EStopTargetSecurityContext{}
	eff := domain.NewEffectiveRoles(domain.RoleDriver)
	if d := sec.CanStop(eff, 42, false, 10, []uint{10, 42}); !d.Allowed {
		t.Fatalf("controller should stop, got %q", d.Reason)
	}
}

func TestEStopTargetCanStopOccupyingSignalman(t *testing.T) {
	sec := security.EStopTargetSecurityContext{}
	eff := domain.NewEffectiveRoles(domain.RoleSignalman)
	if d := sec.CanStop(eff, 7, true, 99, []uint{99}); !d.Allowed {
		t.Fatalf("occupying signalman should stop, got %q", d.Reason)
	}
}

func TestEStopTargetDeniesIdleSignalman(t *testing.T) {
	sec := security.EStopTargetSecurityContext{}
	eff := domain.NewEffectiveRoles(domain.RoleSignalman)
	if d := sec.CanStop(eff, 7, false, 99, []uint{99}); d.Allowed {
		t.Fatal("idle signalman should not stop unrelated target")
	}
}

func TestEStopTargetDeniesUnrelatedDriver(t *testing.T) {
	sec := security.EStopTargetSecurityContext{}
	eff := domain.NewEffectiveRoles(domain.RoleDriver)
	if d := sec.CanStop(eff, 5, false, 99, []uint{99}); d.Allowed {
		t.Fatal("unrelated driver should be denied")
	}
}
