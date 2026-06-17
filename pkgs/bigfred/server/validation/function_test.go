package validation_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/validation"
)

func TestValidateFunctionUpsert(t *testing.T) {
	got, err := validation.ValidateFunctionUpsert(validation.FunctionUpsertInput{
		Name:     "  Horn  ",
		Icon:     domain.FunctionIcon("horn_low"),
		Position: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "Horn" || got.Icon != domain.FunctionIcon("horn_low") || got.Position != 2 {
		t.Fatalf("got %+v", got)
	}
}

func TestValidateFunctionUpsertRejectsEmptyNameAndInvalidIcon(t *testing.T) {
	_, err := validation.ValidateFunctionUpsert(validation.FunctionUpsertInput{
		Name: " ",
		Icon: domain.FunctionIcon("horn_low"),
	})
	if !errors.Is(err, svcerrors.ErrFunctionNameRequired) {
		t.Fatalf("empty name: got %v", err)
	}
	_, err = validation.ValidateFunctionUpsert(validation.FunctionUpsertInput{
		Name: "Light",
		Icon: domain.FunctionIcon("not-in-catalogue"),
	})
	if !errors.Is(err, svcerrors.ErrFunctionIconInvalid) {
		t.Fatalf("invalid icon: got %v", err)
	}
}

func TestValidateFunctionUpsertTruncatesLongName(t *testing.T) {
	long := strings.Repeat("f", validation.MaxFunctionNameLen+8)
	got, err := validation.ValidateFunctionUpsert(validation.FunctionUpsertInput{
		Name: long,
		Icon: domain.FunctionIcon("light"),
	})
	if err != nil || len(got.Name) != validation.MaxFunctionNameLen {
		t.Fatalf("len=%d err=%v", len(got.Name), err)
	}
}
