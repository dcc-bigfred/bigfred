package security

import (
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

func TestRadioCanSend(t *testing.T) {
	sec := RadioSecurityContext{}
	signalman := domain.NewEffectiveRoles(domain.RoleSignalman)
	driver := domain.NewEffectiveRoles(domain.RoleDriver)

	if d := sec.CanSend(signalman, 42, 0); !d.Allowed {
		t.Fatalf("signalman → user: %s", d.Reason)
	}
	if d := sec.CanSend(driver, 42, 0); d.Allowed {
		t.Fatal("driver → user should be denied")
	}
	if d := sec.CanSend(driver, 0, 7); !d.Allowed {
		t.Fatalf("driver → interlocking: %s", d.Reason)
	}
}

func TestRadioCanReplayInterlocking(t *testing.T) {
	sec := RadioSecurityContext{}
	signalman := domain.NewEffectiveRoles(domain.RoleSignalman)
	driver := domain.NewEffectiveRoles(domain.RoleDriver)

	if d := sec.CanReplayInterlocking(signalman, 3, 10, 10); !d.Allowed {
		t.Fatalf("occupant signalman: %s", d.Reason)
	}
	if d := sec.CanReplayInterlocking(signalman, 3, 10, 11); d.Allowed {
		t.Fatal("non-occupant should be denied")
	}
	if d := sec.CanReplayInterlocking(driver, 3, 10, 10); d.Allowed {
		t.Fatal("driver should be denied interlocking replay")
	}
}

func TestRadioCanReplayUser(t *testing.T) {
	sec := RadioSecurityContext{}
	if d := sec.CanReplayUser(); !d.Allowed {
		t.Fatalf("user replay: %s", d.Reason)
	}
}
