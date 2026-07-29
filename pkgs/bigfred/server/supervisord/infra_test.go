package supervisord

import (
	"strings"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/server/datadir"
)

func TestDefaultInfraProcesses_includesAlloyWhenEnabled(t *testing.T) {
	cfgPath := datadir.Path("etc", "alloy.conf")
	st := DefaultInfraProcesses(InfraConfig{
		Redis: RedisConfig{Disable: true},
		Telemetry: TelemetryConfig{
			Enable:     true,
			ConfigPath: cfgPath,
		},
	})
	if len(st.Groups) != 1 || len(st.Groups[0].Programs) != 1 {
		t.Fatalf("groups: %+v", st.Groups)
	}
	prog := st.Groups[0].Programs[0]
	if prog.Name != "alloy" {
		t.Fatalf("name = %q", prog.Name)
	}
	wantFrag := "alloy run --storage.path=" + datadir.Path("alloy") + " " + cfgPath
	if !strings.Contains(prog.Command, wantFrag) {
		t.Fatalf("command = %q", prog.Command)
	}
}

func TestDefaultInfraProcesses_redisAndAlloy(t *testing.T) {
	st := DefaultInfraProcesses(InfraConfig{
		Redis: RedisConfig{Port: 6379},
		Telemetry: TelemetryConfig{
			Enable: true,
		},
	})
	if len(st.Groups[0].Programs) != 2 {
		t.Fatalf("programs = %d", len(st.Groups[0].Programs))
	}
	if st.Groups[0].Programs[0].Name != "alloy" || st.Groups[0].Programs[1].Name != "redis" {
		t.Fatalf("order: %s, %s", st.Groups[0].Programs[0].Name, st.Groups[0].Programs[1].Name)
	}
}
