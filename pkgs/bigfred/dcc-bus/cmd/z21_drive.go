package cmd

// AuthorizeZ21Drive checks handset vehicle scope and roster drive policy
// before forwarding Z21 LAN drive commands to the shared Router.
func (r *Router) AuthorizeZ21Drive(userID uint, addr uint16, allowedAddrs []uint16, allowAll bool) bool {
	vehicle, onLayout := r.roster.AllowedVehicle(addr)
	if !onLayout {
		return false
	}
	if allowAll {
		return r.drive.CanDrive(userID, vehicle, true).Allowed
	}
	for _, a := range allowedAddrs {
		if a == addr {
			return r.drive.CanDrive(userID, vehicle, true).Allowed
		}
	}
	return false
}
