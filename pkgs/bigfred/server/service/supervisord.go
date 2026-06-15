package service

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/server/supervisord"
)

// ProgramState is the observable status of one managed program. It is an
// alias for the supervisord package type so callers can keep referring to
// service.ProgramState while the implementation lives behind the facade.
type ProgramState = supervisord.ProgramState

// TelemetryConfig controls the managed Grafana Alloy process.
type TelemetryConfig = supervisord.TelemetryConfig

// RedisConfig configures the managed redis-server child process.
type RedisConfig = supervisord.RedisConfig

// InfraConfig configures managed infra processes started by loco-server.
type InfraConfig = supervisord.InfraConfig

const (
	// DefaultTelemetryConfigPath is the Alloy config file on hub images.
	DefaultTelemetryConfigPath = supervisord.DefaultTelemetryConfigPath
	// DefaultOTLPEndpoint is the loopback gRPC receiver dcc-bus exports to.
	DefaultOTLPEndpoint = supervisord.DefaultOTLPEndpoint
)

// DefaultInfraProcesses returns the "infra" supervisord group with Redis
// and, when enabled, Grafana Alloy.
func DefaultInfraProcesses(cfg InfraConfig) supervisord.DesiredState {
	return supervisord.DefaultInfraProcesses(cfg)
}

// AlloyRunConfigPath returns the single path passed to `alloy run`.
func AlloyRunConfigPath(cfg TelemetryConfig) string {
	return supervisord.AlloyRunConfigPath(cfg)
}

// BigFredAlloyGeneratedPath returns the path of the templated BigFred OTLP
// receiver block after alloy telemetry is prepared.
func BigFredAlloyGeneratedPath(configDir string, cfg TelemetryConfig) string {
	return supervisord.BigFredAlloyGeneratedPath(configDir, cfg)
}

// SupervisordConfig configures the managed supervisord instance plus the
// BigFred-specific telemetry side-car. Paths default to XDG locations when
// left empty.
type SupervisordConfig struct {
	SupervisordBin   string
	SupervisorctlBin string

	ConfigDir  string
	ConfigPath string
	SocketPath string
	PIDFile    string
	LogDir     string

	InitialState supervisord.DesiredState

	// Telemetry, when Enable is true, triggers templating of the BigFred
	// Alloy OTLP receiver config into ConfigDir before supervisord starts.
	Telemetry TelemetryConfig

	// Log receives lifecycle / apply messages. Nil disables supervisord
	// logging (tests).
	Log *logrus.Logger
}

// Supervisor is the facade the rest of loco-server uses to drive the
// supervisord lifecycle. The concrete implementation lives in
// pkgs/bigfred/server/supervisord.
type Supervisor interface {
	// Start ensures directories exist, renders config, and launches supervisord.
	Start(ctx context.Context) error
	// Stop shuts down supervisord and managed programs.
	Stop(ctx context.Context) error
	// Apply replaces desired state, re-renders config, and reloads supervisord.
	Apply(ctx context.Context, state supervisord.DesiredState) error
	// RunHealthLoop polls program status until ctx is cancelled.
	RunHealthLoop(ctx context.Context, interval time.Duration, onChange func([]ProgramState))
	// Paths returns the active supervisord config file and log directory.
	Paths() (configPath, logDir string)
	// UpsertProgram adds or replaces a program inside a group.
	UpsertProgram(ctx context.Context, group string, prog supervisord.ProgramSpec) error
	// ReplaceGroupPrograms swaps every program in a group in one Apply.
	ReplaceGroupPrograms(ctx context.Context, group string, programs []supervisord.ProgramSpec) error
	// RemoveProgram removes a program from a group.
	RemoveProgram(ctx context.Context, group, name string) error
	// StartProgram starts one program without rewriting config.
	StartProgram(ctx context.Context, name string) error
	// StopProgram stops one program without rewriting config.
	StopProgram(ctx context.Context, name string) error
}

// NewSupervisordService builds the supervisord facade. Default XDG paths
// are filled in when unset.
func NewSupervisordService(cfg SupervisordConfig) (Supervisor, error) {
	if cfg.ConfigPath == "" {
		paths, err := supervisord.DefaultPaths()
		if err != nil {
			return nil, err
		}
		cfg.ConfigDir = paths.ConfigDir
		cfg.ConfigPath = paths.ConfigPath
		cfg.SocketPath = paths.SocketPath
		cfg.PIDFile = paths.PIDFile
		cfg.LogDir = paths.LogDir
	} else if cfg.ConfigDir == "" {
		cfg.ConfigDir = supervisord.ConfigDirFromPath(cfg.ConfigPath)
	}

	return supervisord.NewManager(supervisord.Config{
		SupervisordBin:   cfg.SupervisordBin,
		SupervisorctlBin: cfg.SupervisorctlBin,
		ConfigDir:        cfg.ConfigDir,
		ConfigPath:       cfg.ConfigPath,
		SocketPath:       cfg.SocketPath,
		PIDFile:          cfg.PIDFile,
		LogDir:           cfg.LogDir,
		InitialState:     cfg.InitialState,
		Telemetry:        cfg.Telemetry,
		Log:              cfg.Log,
	})
}
