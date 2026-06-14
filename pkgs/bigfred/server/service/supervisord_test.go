package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/server/service"
	"github.com/keskad/loco/pkgs/bigfred/server/supervisord"
)

func TestDefaultLocoProcessesEmptyWithoutSocket(t *testing.T) {
	st := service.DefaultLocoProcesses("/usr/bin/loco", "")
	if len(st.Groups) != 0 {
		t.Fatalf("expected empty groups, got %+v", st.Groups)
	}
}

func TestDefaultLocoProcessesIncludesExecutor(t *testing.T) {
	st := service.DefaultLocoProcesses("/usr/bin/loco", "/run/loco/exec.sock")
	if err := st.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(st.Groups) != 1 || st.Groups[0].Name != "loco" {
		t.Fatalf("groups: %+v", st.Groups)
	}
	if len(st.Groups[0].Programs) != 1 {
		t.Fatalf("programs: %+v", st.Groups[0].Programs)
	}
	p := st.Groups[0].Programs[0]
	if p.Name != "scripts-executor" || !p.Autostart || !p.Autorestart {
		t.Fatalf("program: %+v", p)
	}
}

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
