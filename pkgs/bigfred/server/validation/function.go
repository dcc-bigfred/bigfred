package validation

import (
	"strings"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
)

const MaxFunctionNameLen = 64

// MaxMomentaryDurationMs caps the auto-off duration of a momentary function.
const MaxMomentaryDurationMs = 300_000

// FunctionUpsertInput is the validated payload for upserting one slot.
type FunctionUpsertInput struct {
	Name                string
	Icon                domain.FunctionIcon
	Position            int
	Momentary           bool
	MomentaryDurationMs int
}

type ValidatedFunction struct {
	Name                string
	Icon                domain.FunctionIcon
	Position            int
	Momentary           bool
	MomentaryDurationMs int
}

// ValidateFunctionUpsert trims and validates one function slot payload.
func ValidateFunctionUpsert(in FunctionUpsertInput) (ValidatedFunction, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return ValidatedFunction{}, svcerrors.ErrFunctionNameRequired
	}
	if len(name) > MaxFunctionNameLen {
		name = name[:MaxFunctionNameLen]
	}
	if !in.Icon.IsValid() {
		return ValidatedFunction{}, svcerrors.ErrFunctionIconInvalid
	}
	if in.MomentaryDurationMs < 0 || in.MomentaryDurationMs > MaxMomentaryDurationMs {
		return ValidatedFunction{}, svcerrors.ErrFunctionDurationInvalid
	}
	return ValidatedFunction{
		Name:                name,
		Icon:                in.Icon,
		Position:            in.Position,
		Momentary:           in.Momentary,
		MomentaryDurationMs: in.MomentaryDurationMs,
	}, nil
}
