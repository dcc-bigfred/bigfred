package supervisord

import (
	"strings"
	"testing"
)

func TestDefaultInfraProcesses_includesAlloyWhenEnabled(t *testing.T) {
	t.Setenv("BIGFRED_DATA_DIR", "")
	t.Setenv("DATA_DIR", "")
	st := DefaultInfraProcesses(InfraConfig{
		Redis: RedisConfig{Disable: true},
		Telemetry: TelemetryConfig{
			Enable: true,
		},
	})
	if len(st.Groups) != 1 || len(st.Groups[0].Programs) != 1 {
		t.Fatalf("groups: %+v", st.Groups)
	}
	prog := st.Groups[0].Programs[0]
	if prog.Name != "alloy" {
		t.Fatalf("name = %q", prog.Name)
	}
	wantFrag := "alloy run --storage.path=/data/alloy /data/etc/alloy.conf"
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
