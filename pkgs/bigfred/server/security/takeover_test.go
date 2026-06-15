package security

import (
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

func TestTakeoverCanRequest(t *testing.T) {
	sec := TakeoverSecurityContext{}
	signalman := domain.NewEffectiveRoles(domain.RoleSignalman)
	driver := domain.NewEffectiveRoles(domain.RoleDriver)

	if d := sec.CanRequest(signalman, 5, 5); !d.Allowed {
		t.Fatalf("occupant signalman: %s", d.Reason)
	}
	if d := sec.CanRequest(signalman, 5, 6); d.Allowed {
		t.Fatal("non-occupant should be denied")
	}
	if d := sec.CanRequest(driver, 5, 5); d.Allowed {
		t.Fatal("driver should be denied")
	}
}
