package errors

import "errors"

// Train catalogue error codes (REST + service sentinels).
const (
	CodeTrainNotFound                    = "train_not_found"
	CodeTrainNameRequired                = "train_name_required"
	CodeTrainNameTaken                   = "train_name_taken"
	CodeTrainNoMembers                   = "train_no_members"
	CodeTrainMemberNotOwned              = "train_member_not_owned"
	CodeTrainMemberMissing               = "train_member_missing"
	CodeTrainNotOwned                    = "train_not_owned"
	CodeTrainMemberMultiplierRange       = "train_member_multiplier_range"
	CodeTrainLeadingMultiplierImmutable  = "train_leading_multiplier_immutable"
	CodeTrainMemberPatchEmpty            = "train_member_patch_empty"
	CodeTrainLeadingSpeedControlImmutable = "train_leading_speed_control_immutable"
	CodeTrainMemberStartDelayRange        = "train_member_start_delay_range"
	CodeTrainLeadingStartDelayImmutable   = "train_leading_start_delay_immutable"
	CodeTrainMemberAccelRampRange         = "train_member_accel_ramp_range"
	CodeTrainMemberAccelRampStepsRange    = "train_member_accel_ramp_steps_range"
	CodeTrainLeadingAccelRampImmutable    = "train_leading_accel_ramp_immutable"
	CodeTrainMemberBrakeRampRange         = "train_member_brake_ramp_range"
	CodeTrainMemberBrakeRampStepsRange    = "train_member_brake_ramp_steps_range"
)

var (
	ErrTrainNotFound                   = errors.New(CodeTrainNotFound)
	ErrTrainNameRequired               = errors.New(CodeTrainNameRequired)
	ErrTrainNameTaken                  = errors.New(CodeTrainNameTaken)
	ErrTrainNoMembers                  = errors.New(CodeTrainNoMembers)
	ErrTrainMemberNotOwned             = errors.New(CodeTrainMemberNotOwned)
	ErrTrainMemberMissing              = errors.New(CodeTrainMemberMissing)
	ErrTrainNotOwned                   = errors.New(CodeTrainNotOwned)
	ErrTrainMemberMultiplierRange       = errors.New(CodeTrainMemberMultiplierRange)
	ErrTrainLeadingMultiplierImmutable  = errors.New(CodeTrainLeadingMultiplierImmutable)
	ErrTrainMemberPatchEmpty            = errors.New(CodeTrainMemberPatchEmpty)
	ErrTrainLeadingSpeedControlImmutable = errors.New(CodeTrainLeadingSpeedControlImmutable)
	ErrTrainMemberStartDelayRange        = errors.New(CodeTrainMemberStartDelayRange)
	ErrTrainLeadingStartDelayImmutable   = errors.New(CodeTrainLeadingStartDelayImmutable)
	ErrTrainMemberAccelRampRange         = errors.New(CodeTrainMemberAccelRampRange)
	ErrTrainMemberAccelRampStepsRange    = errors.New(CodeTrainMemberAccelRampStepsRange)
	ErrTrainLeadingAccelRampImmutable    = errors.New(CodeTrainLeadingAccelRampImmutable)
	ErrTrainMemberBrakeRampRange         = errors.New(CodeTrainMemberBrakeRampRange)
	ErrTrainMemberBrakeRampStepsRange    = errors.New(CodeTrainMemberBrakeRampStepsRange)
)
