package security

import "github.com/keskad/loco/pkgs/bigfred/server/domain"

// DriveSecurityContext evaluates driving authority from an owner plus
// active-lessee projection (§4.3). Callers construct it with a zero
// value.
type DriveSecurityContext struct{}

// CanDrive reports whether actor may drive when ownerID owns the
// target and lessees lists active lease holders. The owner may drive
// only while no one else holds a lease; a lessee may drive while
// their lease is active.
func (DriveSecurityContext) CanDrive(actor domain.User, ownerID uint, lessees []uint) Decision {
	if actor.ID == ownerID {
		if len(lessees) == 0 {
			return Allow
		}
		return Deny("not_authorized_to_drive")
	}
	for _, l := range lessees {
		if l == actor.ID {
			return Allow
		}
	}
	return Deny("not_authorized_to_drive")
}
