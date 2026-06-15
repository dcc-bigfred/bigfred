package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/server/service"
	"github.com/keskad/loco/pkgs/bigfred/server/supervisord"
)

func TestSupervisordServiceNewUsesDefaultPaths(t *testing.T) {
	svc, err := service.NewSupervisordService(service.SupervisordConfig{})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if svc == nil {
		t.Fatal("nil service")
	}
}

func TestSupervisordServiceApplyValidation(t *testing.T) {
	svc, err := service.NewSupervisordService(service.SupervisordConfig{})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	err = svc.Apply(context.Background(), supervisord.DesiredState{
		Groups: []supervisord.GroupSpec{{Name: "loco", Programs: []supervisord.ProgramSpec{
			{Name: "bad name", Command: "x"},
		}}},
	})
	if !errors.Is(err, supervisord.ErrInvalidProgramName) {
		t.Fatalf("expected ErrInvalidProgramName, got %v", err)
	}
}

func TestSupervisordServiceRemoveProgramMissing(t *testing.T) {
	svc, err := service.NewSupervisordService(service.SupervisordConfig{})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	err = svc.RemoveProgram(context.Background(), "loco", "missing")
	if !errors.Is(err, supervisord.ErrGroupNotFound) {
		t.Fatalf("expected ErrGroupNotFound, got %v", err)
	}
}
