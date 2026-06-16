package security

import "github.com/keskad/loco/pkgs/bigfred/contract"

// DrivePolicy evaluates throttle authority against one roster vehicle row.
type DrivePolicy struct{}

// CanDrive reports whether userID may issue drive commands for addr using
// the loaded roster metadata.
func (DrivePolicy) CanDrive(userID uint, vehicle contract.AllowedVehicle, onLayout bool) Decision {
	if !onLayout {
		return Deny(ReasonVehicleNotOnLayout)
	}
	for _, id := range vehicle.ControllerUserIDs {
		if id == userID {
			return Allow
		}
	}
	return Deny(ReasonNotAuthorized)
}

// TrainPolicy evaluates train-level drive authority.
type TrainPolicy struct{}

// CanDriveTrain reports whether userID may drive the given train definition.
func (TrainPolicy) CanDriveTrain(userID uint, train contract.DefinedTrain, known bool) Decision {
	if !known {
		return Deny(ReasonTrainNotOnLayout)
	}
	if !train.CanDrive(userID) {
		return Deny(ReasonNotAuthorizedToDrive)
	}
	return Allow
}

// CanDriveTrainMembers reports whether the train has at least one powered
// DCC member to receive throttle commands.
func (TrainPolicy) CanDriveTrainMembers(train contract.DefinedTrain) Decision {
	if _, ok := train.LeadingMember(); !ok {
		return Deny(ReasonTrainNoPoweredMembers)
	}
	return Allow
}
