package supervisord

import "fmt"

// InfraConfig configures managed infra processes started by loco-server.
type InfraConfig struct {
	Redis     RedisConfig
	Telemetry TelemetryConfig
}

// DefaultInfraProcesses returns the "infra" supervisord group with Redis
// and, when enabled, Grafana Alloy. Returns an empty DesiredState when
// every component is disabled.
func DefaultInfraProcesses(cfg InfraConfig) DesiredState {
	var programs []ProgramSpec
	if cfg.Telemetry.Enable {
		programs = append(programs, alloyProgramSpec(cfg.Telemetry))
	}
	if !cfg.Redis.Disable {
		programs = append(programs, redisProgramSpec(cfg.Redis))
	}
	if len(programs) == 0 {
		return DesiredState{}
	}
	return DesiredState{
		Groups: []GroupSpec{{
			Name:     "infra",
			Programs: programs,
		}},
	}
}

func alloyProgramSpec(cfg TelemetryConfig) ProgramSpec {
	bin := cfg.AlloyBin
	if bin == "" {
		bin = "alloy"
	}
	storagePath := cfg.StoragePath
	if storagePath == "" {
		storagePath = DefaultAlloyStoragePath
	}
	cmd := fmt.Sprintf("%s run --storage.path=%s %s", bin, storagePath, alloyRunConfigPath(cfg))
	return ProgramSpec{
		Name:         "alloy",
		Command:      cmd,
		Autostart:    true,
		Autorestart:  true,
		StartSecs:    2,
		StopWaitSecs: 10,
	}
}
