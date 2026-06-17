package validation

import (
	"strings"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
)

const MaxFunctionNameLen = 64

// FunctionUpsertInput is the validated payload for upserting one slot.
type FunctionUpsertInput struct {
	Name     string
	Icon     domain.FunctionIcon
	Position int
}

type ValidatedFunction struct {
	Name     string
	Icon     domain.FunctionIcon
	Position int
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
	return ValidatedFunction{
		Name:     name,
		Icon:     in.Icon,
		Position: in.Position,
	}, nil
}
