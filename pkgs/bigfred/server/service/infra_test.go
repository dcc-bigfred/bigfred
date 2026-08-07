package service

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/server/microinit"
)

type alloySetupStub struct {
	hasService bool
}

func (s *alloySetupStub) Start(context.Context) error { return nil }
func (s *alloySetupStub) Stop(context.Context) error  { return nil }
func (s *alloySetupStub) UpsertService(context.Context, string, microinit.ServiceDef) error {
	return nil
}
func (s *alloySetupStub) ReplaceServices(context.Context, string, []microinit.ServiceDef) error {
	return nil
}
func (s *alloySetupStub) RemoveService(context.Context, string, string) error { return nil }
func (s *alloySetupStub) StartService(context.Context, string) error         { return nil }
func (s *alloySetupStub) StopService(context.Context, string) error          { return nil }
func (s *alloySetupStub) RestartService(context.Context, string) error       { return nil }
func (s *alloySetupStub) Status(context.Context) ([]ServiceState, error)     { return nil, nil }
func (s *alloySetupStub) RunHealthLoop(context.Context, time.Duration, func([]ServiceState)) {
}
func (s *alloySetupStub) Paths() (string, string) { return "", "" }
func (s *alloySetupStub) HasService(context.Context, string) (bool, error) {
	return s.hasService, nil
}
func (s *alloySetupStub) CanManage(context.Context, string) (bool, error) { return true, nil }

func TestEnsureInfraAlloySetupFailureIsNonFatal(t *testing.T) {
	t.Setenv("BIGFRED_DATA_DIR", t.TempDir())
	t.Setenv("DATA_DIR", "")

	log := logrus.New()
	log.SetLevel(logrus.WarnLevel)
	var buf bytes.Buffer
	log.SetOutput(&buf)

	mgr := &alloySetupStub{hasService: false}
	err := EnsureInfra(context.Background(), mgr, log, InfraConfig{
		Redis:     RedisConfig{Disable: true},
		Telemetry: TelemetryConfig{Enable: true},
	})
	if err != nil {
		t.Fatalf("EnsureInfra: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "alloy") || !strings.Contains(out, "warning") {
		t.Fatalf("expected alloy warning log, got:\n%s", out)
	}
}

func TestEnsureInfraMicrodnsSetupFailureIsNonFatal(t *testing.T) {
	t.Setenv("BIGFRED_DATA_DIR", t.TempDir())
	t.Setenv("DATA_DIR", "")

	log := logrus.New()
	log.SetLevel(logrus.WarnLevel)
	var buf bytes.Buffer
	log.SetOutput(&buf)

	mgr := &alloySetupStub{hasService: false}
	err := EnsureInfra(context.Background(), mgr, log, InfraConfig{
		Redis:    RedisConfig{Disable: true},
		Microdns: MicrodnsConfig{Bin: "microdns"},
	})
	if err != nil {
		t.Fatalf("EnsureInfra: %v", err)
	}
}

func TestEnsureInfraRedisSetupFailureIsFatal(t *testing.T) {
	t.Setenv("BIGFRED_DATA_DIR", t.TempDir())
	t.Setenv("DATA_DIR", "")

	log := logrus.New()
	mgr := &alloySetupStub{hasService: false}
	err := EnsureInfra(context.Background(), mgr, log, InfraConfig{
		Redis: RedisConfig{Bin: "redis-server"},
	})
	if err == nil {
		t.Fatal("expected redis setup error")
	}
	if !strings.Contains(err.Error(), "redis") {
		t.Fatalf("unexpected error: %v", err)
	}
}
