package security

import (
	"github.com/keskad/loco/pkgs/server/domain"
)

// TrainSecurityContext gates catalogue mutations on domain.Train
// (§7a.3). Callers construct it with a zero value.
type TrainSecurityContext struct{}

// CanMutateTrain decides whether actor may update or delete a train
// row. The owner may always mutate their own catalogue entry; an
// effective admin (permanent or sudo in the active layout, §7a.7) may
// mutate any train.
func (TrainSecurityContext) CanMutateTrain(
	eff domain.EffectiveRoles,
	actorID, ownerUserID uint,
) Decision {
	if eff.Has(domain.RoleAdmin) {
		return Allow
	}
	if actorID == ownerUserID {
		return Allow
	}
	return Deny("train_not_owned")
}
