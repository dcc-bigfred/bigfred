package validation

import (
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
)

const maxLayoutMaxVehiclesPerUser = 120

// SanitiseLayoutMaxVehiclesPerUser normalises the per-user driven-vehicle cap.
// Zero selects the catalogue default (8).
func SanitiseLayoutMaxVehiclesPerUser(max uint) (uint, error) {
	if max == 0 {
		return 0, nil
	}
	if max > maxLayoutMaxVehiclesPerUser {
		return 0, svcerrors.ErrLayoutMaxVehiclesInvalid
	}
	return max, nil
}
