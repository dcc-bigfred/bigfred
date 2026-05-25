package domain

// DCCAddressRange is a contiguous DCC address window granted to a
// user by an admin (§3a.1, goal 3). A user may have any number of
// rows (e.g. 100..199 and 3001..3010); a vehicle registration is
// accepted iff the requested DCC address lies inside at least one of
// the user's ranges. Dummies (DCCAddress = nil on Vehicle) skip the
// pool check entirely.
//
// FromAddr / ToAddr are inclusive bounds. The service layer rejects
// `FromAddr > ToAddr`. There is no end date on a pool row — admins
// revoke pool by deleting / re-issuing the ranges.
type DCCAddressRange struct {
	ID       uint
	UserID   uint   `db:"user_id"`
	FromAddr uint16 `db:"from_addr"`
	ToAddr   uint16 `db:"to_addr"`
}

// Table tells REL which physical table backs this struct.
func (DCCAddressRange) Table() string { return "dcc_address_ranges" }

// Contains reports whether addr lies inside the inclusive range.
func (r DCCAddressRange) Contains(addr uint16) bool {
	return addr >= r.FromAddr && addr <= r.ToAddr
}
