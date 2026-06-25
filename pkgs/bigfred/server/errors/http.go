package errors

import (
	stderrors "errors"
	"net/http"
)

// AuthHTTPStatus maps auth flow errors to HTTP status and code.
func AuthHTTPStatus(err error) (status int, code string) {
	switch {
	case stderrors.Is(err, ErrInvalidCredentials):
		return http.StatusUnauthorized, CodeInvalidCredentials
	case stderrors.Is(err, ErrAccountDeactivated):
		return http.StatusForbidden, CodeAccountDeactivated
	case stderrors.Is(err, ErrLayoutNotFound):
		return http.StatusUnprocessableEntity, CodeLayoutNotFound
	case stderrors.Is(err, ErrLayoutLocked):
		return http.StatusUnprocessableEntity, CodeLayoutLocked
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}

// SudoHTTPStatus maps sudo / signalman elevation errors to HTTP status and code.
func SudoHTTPStatus(err error) (status int, code string) {
	switch {
	case stderrors.Is(err, ErrSudoInvalidInput):
		return http.StatusUnprocessableEntity, CodeSudoLayoutMismatch
	case stderrors.Is(err, ErrSudoInvalidPIN):
		return http.StatusUnauthorized, CodeSudoInvalidPIN
	case stderrors.Is(err, ErrSudoLocked):
		return http.StatusTooManyRequests, CodeSudoLocked
	case stderrors.Is(err, ErrLayoutAdminPINUnset):
		return http.StatusUnprocessableEntity, CodeLayoutAdminPINUnset
	case stderrors.Is(err, ErrLayoutNotFound):
		return http.StatusNotFound, CodeLayoutNotFound
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}

// VehicleHTTPStatus maps vehicle catalogue sentinel errors to an HTTP
// status and machine-readable code for JSON error bodies.
func VehicleHTTPStatus(err error) (status int, code string) {
	switch {
	case stderrors.Is(err, ErrVehicleNotFound):
		return http.StatusNotFound, CodeVehicleNotFound
	case stderrors.Is(err, ErrVehicleNameRequired):
		return http.StatusUnprocessableEntity, CodeVehicleNameRequired
	case stderrors.Is(err, ErrVehicleKindInvalid):
		return http.StatusUnprocessableEntity, CodeVehicleKindInvalid
	case stderrors.Is(err, ErrDCCAddressTaken):
		return http.StatusConflict, CodeDCCAddressTaken
	case stderrors.Is(err, ErrDCCAddressOutsidePool):
		return http.StatusUnprocessableEntity, CodeDCCAddressOutsidePool
	case stderrors.Is(err, ErrVehicleNotOwned):
		return http.StatusForbidden, CodeVehicleNotOwned
	case stderrors.Is(err, ErrVehicleInUse):
		return http.StatusConflict, CodeVehicleInUse
	case stderrors.Is(err, ErrVehicleDccFunctionInvalid):
		return http.StatusUnprocessableEntity, CodeVehicleDccFunctionInvalid
	case stderrors.Is(err, ErrVehicleDeadManSwitchInvalid):
		return http.StatusUnprocessableEntity, CodeVehicleDeadManSwitchInvalid
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}

// TrainHTTPStatus maps train catalogue sentinel errors to an HTTP
// status and machine-readable code for JSON error bodies.
func TrainHTTPStatus(err error) (status int, code string) {
	switch {
	case stderrors.Is(err, ErrTrainNotFound):
		return http.StatusNotFound, CodeTrainNotFound
	case stderrors.Is(err, ErrTrainNameRequired):
		return http.StatusUnprocessableEntity, CodeTrainNameRequired
	case stderrors.Is(err, ErrTrainNameTaken):
		return http.StatusConflict, CodeTrainNameTaken
	case stderrors.Is(err, ErrTrainNoMembers):
		return http.StatusUnprocessableEntity, CodeTrainNoMembers
	case stderrors.Is(err, ErrTrainMemberNotOwned):
		return http.StatusForbidden, CodeTrainMemberNotOwned
	case stderrors.Is(err, ErrTrainMemberMissing):
		return http.StatusUnprocessableEntity, CodeTrainMemberMissing
	case stderrors.Is(err, ErrTrainNotOwned):
		return http.StatusForbidden, CodeTrainNotOwned
	case stderrors.Is(err, ErrTrainMemberMultiplierRange):
		return http.StatusUnprocessableEntity, CodeTrainMemberMultiplierRange
	case stderrors.Is(err, ErrTrainLeadingMultiplierImmutable):
		return http.StatusUnprocessableEntity, CodeTrainLeadingMultiplierImmutable
	case stderrors.Is(err, ErrTrainMemberPatchEmpty):
		return http.StatusBadRequest, CodeTrainMemberPatchEmpty
	case stderrors.Is(err, ErrTrainLeadingSpeedControlImmutable):
		return http.StatusUnprocessableEntity, CodeTrainLeadingSpeedControlImmutable
	case stderrors.Is(err, ErrTrainMemberStartDelayRange):
		return http.StatusUnprocessableEntity, CodeTrainMemberStartDelayRange
	case stderrors.Is(err, ErrTrainLeadingStartDelayImmutable):
		return http.StatusUnprocessableEntity, CodeTrainLeadingStartDelayImmutable
	case stderrors.Is(err, ErrTrainMemberAccelRampRange):
		return http.StatusUnprocessableEntity, CodeTrainMemberAccelRampRange
	case stderrors.Is(err, ErrTrainMemberAccelRampStepsRange):
		return http.StatusUnprocessableEntity, CodeTrainMemberAccelRampStepsRange
	case stderrors.Is(err, ErrTrainLeadingAccelRampImmutable):
		return http.StatusUnprocessableEntity, CodeTrainLeadingAccelRampImmutable
	case stderrors.Is(err, ErrTrainMemberBrakeRampRange):
		return http.StatusUnprocessableEntity, CodeTrainMemberBrakeRampRange
	case stderrors.Is(err, ErrTrainMemberBrakeRampStepsRange):
		return http.StatusUnprocessableEntity, CodeTrainMemberBrakeRampStepsRange
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}

// UserHTTPStatus maps user catalogue errors to HTTP status and code.
func UserHTTPStatus(err error) (status int, code string) {
	switch {
	case stderrors.Is(err, ErrUserNotFound):
		return http.StatusNotFound, CodeUserNotFound
	case stderrors.Is(err, ErrUserLoginRequired):
		return http.StatusUnprocessableEntity, CodeUserLoginRequired
	case stderrors.Is(err, ErrUserLoginInvalid):
		return http.StatusUnprocessableEntity, CodeUserLoginInvalid
	case stderrors.Is(err, ErrUserLoginTaken):
		return http.StatusConflict, CodeUserLoginTaken
	case stderrors.Is(err, ErrUserPINRequired):
		return http.StatusUnprocessableEntity, CodeUserPINRequired
	case stderrors.Is(err, ErrUserPINInvalid):
		return http.StatusUnprocessableEntity, CodeUserPINInvalid
	case stderrors.Is(err, ErrUserRoleInvalid):
		return http.StatusUnprocessableEntity, CodeUserRoleInvalid
	case stderrors.Is(err, ErrUserHasVehicles):
		return http.StatusConflict, CodeUserHasVehicles
	case stderrors.Is(err, ErrUserHasTrains):
		return http.StatusConflict, CodeUserHasTrains
	case stderrors.Is(err, ErrCannotDeactivateSelf):
		return http.StatusUnprocessableEntity, CodeCannotDeactivateSelf
	case stderrors.Is(err, ErrCannotDeleteSelf):
		return http.StatusUnprocessableEntity, CodeCannotDeleteSelf
	case stderrors.Is(err, ErrDCCPoolEmpty):
		return http.StatusUnprocessableEntity, CodeDCCPoolEmpty
	case stderrors.Is(err, ErrDCCPoolRangeInvalid):
		return http.StatusUnprocessableEntity, CodeDCCPoolRangeInvalid
	case stderrors.Is(err, ErrDCCPoolOverlap):
		return http.StatusConflict, CodeDCCPoolOverlap
	case stderrors.Is(err, ErrDCCPoolForbidden), stderrors.Is(err, ErrUserForbidden):
		return http.StatusForbidden, CodeUserForbidden
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}

// FunctionHTTPStatus maps function catalogue sentinel errors to HTTP status and code.
func FunctionHTTPStatus(err error) (status int, code string) {
	switch {
	case stderrors.Is(err, ErrVehicleNotFound), stderrors.Is(err, ErrVehicleTemplateNotFound):
		return http.StatusNotFound, "not_found"
	case stderrors.Is(err, ErrFunctionNotFound):
		return http.StatusNotFound, CodeFunctionNotFound
	case stderrors.Is(err, ErrOnlyOwnerCanEdit):
		return http.StatusForbidden, CodeOnlyOwnerCanEdit
	case stderrors.Is(err, ErrTemplateNotOwned):
		return http.StatusForbidden, CodeTemplateNotOwned
	case stderrors.Is(err, ErrFunctionReplaceSourceInvalid):
		return http.StatusUnprocessableEntity, CodeFunctionReplaceSourceInvalid
	case stderrors.Is(err, ErrFunctionNumInvalid):
		return http.StatusUnprocessableEntity, CodeFunctionNumInvalid
	case stderrors.Is(err, ErrFunctionIconInvalid):
		return http.StatusUnprocessableEntity, CodeFunctionIconInvalid
	case stderrors.Is(err, ErrFunctionNameRequired):
		return http.StatusUnprocessableEntity, CodeFunctionNameRequired
	case stderrors.Is(err, ErrFunctionNumTaken):
		return http.StatusConflict, CodeFunctionNumTaken
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}

// LayoutRosterHTTPStatus maps layout roster sentinel errors to HTTP status and code.
func LayoutRosterHTTPStatus(err error) (status int, code string) {
	switch {
	case stderrors.Is(err, ErrVehicleNotFound):
		return http.StatusNotFound, CodeVehicleNotFound
	case stderrors.Is(err, ErrTrainNotFound):
		return http.StatusNotFound, CodeTrainNotFound
	case stderrors.Is(err, ErrVehicleNotOwned):
		return http.StatusForbidden, CodeVehicleNotOwned
	case stderrors.Is(err, ErrTrainNotOwned):
		return http.StatusForbidden, CodeTrainNotOwned
	case stderrors.Is(err, ErrLayoutVehicleAlreadyOnRoster):
		return http.StatusConflict, CodeLayoutVehicleAlreadyOnRoster
	case stderrors.Is(err, ErrLayoutVehicleNotOnRoster):
		return http.StatusNotFound, CodeLayoutVehicleNotOnRoster
	case stderrors.Is(err, ErrLayoutTrainAlreadyOnRoster):
		return http.StatusConflict, CodeLayoutTrainAlreadyOnRoster
	case stderrors.Is(err, ErrLayoutTrainNotOnRoster):
		return http.StatusNotFound, CodeLayoutTrainNotOnRoster
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}

// VehicleTemplateHTTPStatus maps template catalogue errors to HTTP status and code.
func VehicleTemplateHTTPStatus(err error) (status int, code string) {
	switch {
	case stderrors.Is(err, ErrVehicleTemplateNotFound):
		return http.StatusNotFound, CodeVehicleTemplateNotFound
	case stderrors.Is(err, ErrVehicleTemplateNameRequired):
		return http.StatusUnprocessableEntity, CodeVehicleTemplateNameRequired
	case stderrors.Is(err, ErrVehicleTemplateNameTaken):
		return http.StatusConflict, CodeVehicleTemplateNameTaken
	case stderrors.Is(err, ErrTemplateNotOwned):
		return http.StatusForbidden, CodeTemplateNotOwned
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}

// LayoutHTTPStatus maps layout catalogue errors to HTTP status and code.
func LayoutHTTPStatus(err error) (status int, code string) {
	switch {
	case stderrors.Is(err, ErrLayoutNotFound):
		return http.StatusNotFound, CodeLayoutNotFound
	case stderrors.Is(err, ErrLayoutNameTaken):
		return http.StatusConflict, CodeLayoutNameTaken
	case stderrors.Is(err, ErrLayoutNameRequired):
		return http.StatusUnprocessableEntity, CodeLayoutNameRequired
	case stderrors.Is(err, ErrSystemLayoutImmutable):
		return http.StatusUnprocessableEntity, CodeSystemLayoutImmutable
	case stderrors.Is(err, ErrSystemLayoutUndeletable):
		return http.StatusUnprocessableEntity, CodeSystemLayoutUndeletable
	case stderrors.Is(err, ErrLayoutAdminPINInvalid):
		return http.StatusUnprocessableEntity, CodeLayoutAdminPINInvalid
	case stderrors.Is(err, ErrLayoutForbidden):
		return http.StatusForbidden, CodeLayoutForbidden
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}

// LayoutInterlockingHTTPStatus maps layout+interlocking errors to HTTP status and code.
func LayoutInterlockingHTTPStatus(err error) (status int, code string) {
	switch {
	case stderrors.Is(err, ErrLayoutNotFound):
		return http.StatusNotFound, CodeLayoutNotFound
	case stderrors.Is(err, ErrInterlockingNotFound):
		return http.StatusNotFound, CodeInterlockingNotFound
	case stderrors.Is(err, ErrLayoutForbidden):
		return http.StatusForbidden, CodeLayoutForbidden
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}

// InterlockingHTTPStatus maps interlocking catalogue errors to HTTP status and code.
func InterlockingHTTPStatus(err error) (status int, code string) {
	switch {
	case stderrors.Is(err, ErrInterlockingNotFound):
		return http.StatusNotFound, CodeInterlockingNotFound
	case stderrors.Is(err, ErrInterlockingNameTaken):
		return http.StatusConflict, CodeInterlockingNameTaken
	case stderrors.Is(err, ErrInterlockingNameRequired):
		return http.StatusUnprocessableEntity, CodeInterlockingNameRequired
	case stderrors.Is(err, ErrInterlockingForbidden):
		return http.StatusForbidden, CodeInterlockingForbidden
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}

// LayoutCommandStationHTTPStatus maps layout+command-station errors to HTTP status and code.
func LayoutCommandStationHTTPStatus(err error) (status int, code string) {
	switch {
	case stderrors.Is(err, ErrLayoutNotFound):
		return http.StatusNotFound, CodeLayoutNotFound
	case stderrors.Is(err, ErrCommandStationNotFound):
		return http.StatusNotFound, CodeCommandStationNotFound
	case stderrors.Is(err, ErrLayoutNeedsAtLeastOneCommandStation):
		return http.StatusUnprocessableEntity, CodeLayoutNeedsAtLeastOneCommandStation
	case stderrors.Is(err, ErrSystemLayoutCommandStationsImmutable):
		return http.StatusUnprocessableEntity, CodeSystemLayoutCommandStationsImmutable
	case stderrors.Is(err, ErrLayoutForbidden):
		return http.StatusForbidden, CodeLayoutForbidden
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}

// CommandStationHTTPStatus maps command-station catalogue errors to HTTP status and code.
func CommandStationHTTPStatus(err error) (status int, code string) {
	switch {
	case stderrors.Is(err, ErrCommandStationNotFound):
		return http.StatusNotFound, CodeCommandStationNotFound
	case stderrors.Is(err, ErrCommandStationNameTaken):
		return http.StatusConflict, CodeCommandStationNameTaken
	case stderrors.Is(err, ErrCommandStationNameRequired):
		return http.StatusUnprocessableEntity, CodeCommandStationNameRequired
	case stderrors.Is(err, ErrCommandStationKindInvalid):
		return http.StatusUnprocessableEntity, CodeCommandStationKindInvalid
	case stderrors.Is(err, ErrCommandStationSpeedInvalid):
		return http.StatusUnprocessableEntity, CodeCommandStationSpeedInvalid
	case stderrors.Is(err, ErrCommandStationHeartbeatInvalid):
		return http.StatusUnprocessableEntity, CodeCommandStationHeartbeatInvalid
	case stderrors.Is(err, ErrCommandStationDeadmanInvalid):
		return http.StatusUnprocessableEntity, CodeCommandStationDeadmanInvalid
	case stderrors.Is(err, ErrCommandStationDeadmanTooShort):
		return http.StatusUnprocessableEntity, CodeCommandStationDeadmanTooShort
	case stderrors.Is(err, ErrCommandStationPollIntervalInvalid):
		return http.StatusUnprocessableEntity, CodeCommandStationPollIntervalInvalid
	case stderrors.Is(err, ErrLayoutNeedsAtLeastOneCommandStation):
		return http.StatusConflict, CodeLayoutNeedsAtLeastOneCommandStation
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}

// LeaseHTTPStatus maps lease errors to HTTP status and code.
func LeaseHTTPStatus(err error) (status int, code string) {
	switch {
	case stderrors.Is(err, ErrLeaseNotFound):
		return http.StatusNotFound, CodeLeaseNotFound
	case stderrors.Is(err, ErrLeaseConflict):
		return http.StatusConflict, CodeLeaseConflict
	case stderrors.Is(err, ErrLeaseNotOwner):
		return http.StatusForbidden, CodeLeaseNotOwner
	case stderrors.Is(err, ErrLeaseNotParty):
		return http.StatusForbidden, CodeLeaseNotParty
	case stderrors.Is(err, ErrLeaseSelf):
		return http.StatusUnprocessableEntity, CodeLeaseSelf
	case stderrors.Is(err, ErrLeaseTargetNotOnLayout):
		return http.StatusUnprocessableEntity, CodeLeaseTargetNotOnLayout
	case stderrors.Is(err, ErrLeaseInvalidSpeedLimit):
		return http.StatusUnprocessableEntity, CodeLeaseInvalidSpeedLimit
	case stderrors.Is(err, ErrLeaseInvalidDuration):
		return http.StatusUnprocessableEntity, CodeLeaseInvalidDuration
	case stderrors.Is(err, ErrLeaseTargetNotDrivable):
		return http.StatusUnprocessableEntity, CodeLeaseTargetNotDrivable
	case stderrors.Is(err, ErrLeaseStoreUnavailable):
		return http.StatusServiceUnavailable, CodeLeaseStoreUnavailable
	case stderrors.Is(err, ErrUserNotFound):
		return http.StatusNotFound, CodeUserNotFound
	case stderrors.Is(err, ErrAccountDeactivated):
		return http.StatusForbidden, CodeAccountDeactivated
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}

// Z21RemoteHTTPStatus maps z21-remote errors to HTTP status codes.
func Z21RemoteHTTPStatus(err error) (status int, code string) {
	switch {
	case stderrors.Is(err, ErrZ21ServerDisabled):
		return http.StatusConflict, CodeZ21ServerDisabled
	case stderrors.Is(err, ErrZ21CommandStationNotOnLayout):
		return http.StatusNotFound, CodeZ21CommandStationNotOnLayout
	case stderrors.Is(err, ErrCommandStationNotFound):
		return http.StatusNotFound, CodeCommandStationNotFound
	case stderrors.Is(err, ErrZ21VehicleNotOnRoster):
		return http.StatusUnprocessableEntity, CodeZ21VehicleNotOnRoster
	case stderrors.Is(err, ErrZ21VehicleNotDrivable):
		return http.StatusForbidden, CodeZ21VehicleNotDrivable
	case stderrors.Is(err, ErrZ21VehicleNoDCCAddress):
		return http.StatusUnprocessableEntity, CodeZ21VehicleNoDCCAddress
	case stderrors.Is(err, ErrZ21SessionNotFound):
		return http.StatusNotFound, CodeZ21SessionNotFound
	case stderrors.Is(err, ErrZ21PairingScopeInvalid):
		return http.StatusUnprocessableEntity, CodeZ21PairingScopeInvalid
	case stderrors.Is(err, ErrZ21HandsetBrakeSecsInvalid):
		return http.StatusUnprocessableEntity, CodeZ21HandsetBrakeSecsInvalid
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}
