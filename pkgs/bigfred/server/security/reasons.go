package security

// Machine-readable denial reasons returned in Decision.Reason. HTTP and
// cmd layers map these to sentinel errors or status codes.
const (
	ReasonVehicleNotOwned         = "vehicle_not_owned"
	ReasonTrainNotOwned           = "train_not_owned"
	ReasonOnlyOwnerCanEdit        = "only_owner_can_edit"
	ReasonTemplateNotOwned        = "template_not_owned"
	ReasonCannotDeactivateSelf    = "cannot_deactivate_self"
	ReasonCannotDeleteSelf        = "cannot_delete_self"
	ReasonForbidden               = "forbidden"
	ReasonInterlockingNotInLayout = "interlocking_not_in_layout"
	ReasonInterlockingOccupied    = "interlocking_occupied"
	ReasonNotSignalman            = "not_signalman"
	ReasonNotInterlockingOccupant = "not_interlocking_occupant"
	ReasonNotAuthorizedToDrive    = "not_authorized_to_drive"
	ReasonNotAuthorizedToStop     = "not_authorized_to_stop"
	ReasonRadioInvalidTarget      = "radio_invalid_target"
	ReasonInvalidInterlocking     = "invalid_interlocking"
)
