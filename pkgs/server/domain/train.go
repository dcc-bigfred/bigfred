package domain

import "time"

// Train (Polish: skład) is an ordered group of 1+ Vehicles owned by
// one user and addressed and driven as a single unit (§3a.1).
//
// Members live in TrainMember rows; the relation is deliberately
// LEFT IMPLICIT here (no inline `Members []TrainMember` slice) so the
// service layer can fetch them with a single dedicated query and the
// REL Insert/Update calls on Train stay simple.
type Train struct {
	ID          uint
	OwnerUserID uint   `db:"owner_user_id"`
	Name        string
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// Table tells REL which physical table backs this struct.
func (Train) Table() string { return "trains" }

// TrainMember binds one Vehicle to a Train with the per-consist
// `Position` (ordering) and `Reversed` (coupled the other way around
// — drives in the opposite DCC direction when train.setSpeed fans
// out to its members). A member may be a DUMMY vehicle: when its
// vehicle's DCCAddress is nil, the train slider simply skips DCC for
// that row.
type TrainMember struct {
	ID        uint
	TrainID   uint `db:"train_id"`
	VehicleID uint `db:"vehicle_id"`
	Position  int
	Reversed  bool
}

// Table tells REL which physical table backs this struct.
func (TrainMember) Table() string { return "train_members" }
