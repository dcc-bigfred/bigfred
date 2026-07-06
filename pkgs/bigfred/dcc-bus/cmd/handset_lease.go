package cmd

import (
	"github.com/keskad/loco/pkgs/bigfred/remotes"
)

// prepareHandsetLease acquires (or refreshes) a slot lease for the first
// drive action on a remote handset and resets the remote idle timer.
func (r *Router) prepareHandsetLease(actor remotes.ThrottleActor, addr uint16) Result {
	if r == nil || r.leaser == nil || actor.Source == "" {
		return OKResult()
	}
	if _, err := r.leaser.Select(actor.UserID, actor.SessionID, actor.Source, addr); err != nil {
		return leaseErrorResult(actor.UserID, r.leaser, err)
	}
	r.leaser.Touch(actor.UserID, actor.SessionID, addr)
	return OKResult()
}
