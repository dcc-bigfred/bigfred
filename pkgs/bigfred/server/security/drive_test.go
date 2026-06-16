package security

import (
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

func TestDriveSecurityContext_CanDrive(t *testing.T) {
	sec := DriveSecurityContext{}
	owner := domain.User{ID: 1}
	lessee := domain.User{ID: 2}
	if !sec.CanDrive(owner, owner.ID, nil).Allowed {
		t.Fatal("owner without lease should drive")
	}
	if sec.CanDrive(owner, owner.ID, []uint{lessee.ID}).Allowed {
		t.Fatal("owner with lease to other should not drive")
	}
	if !sec.CanDrive(lessee, owner.ID, []uint{lessee.ID}).Allowed {
		t.Fatal("lessee should drive")
	}
}
