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
)

var (
	ErrTrainNotFound                   = errors.New(CodeTrainNotFound)
	ErrTrainNameRequired               = errors.New(CodeTrainNameRequired)
	ErrTrainNameTaken                  = errors.New(CodeTrainNameTaken)
	ErrTrainNoMembers                  = errors.New(CodeTrainNoMembers)
	ErrTrainMemberNotOwned             = errors.New(CodeTrainMemberNotOwned)
	ErrTrainMemberMissing              = errors.New(CodeTrainMemberMissing)
	ErrTrainNotOwned                   = errors.New(CodeTrainNotOwned)
	ErrTrainMemberMultiplierRange      = errors.New(CodeTrainMemberMultiplierRange)
	ErrTrainLeadingMultiplierImmutable = errors.New(CodeTrainLeadingMultiplierImmutable)
)
