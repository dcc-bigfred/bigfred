package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/keskad/loco/pkgs/server/supervisord"
)

const (
	supervisordReadyTimeout = 10 * time.Second
	supervisordApplyTimeout = 30 * time.Second
	supervisordStopTimeout  = 15 * time.Second
)

// SupervisordConfig configures SupervisordService paths and binaries.
type SupervisordConfig struct {
	SupervisordBin   string
	SupervisorctlBin string

	ConfigDir  string
	ConfigPath string
	SocketPath string
	PIDFile    string
	LogDir     string

	InitialState supervisord.DesiredState
}

// ProgramState is the observable status of one managed program.
type ProgramState struct {
	Name   string
	Group  string
	Status string
	PID    int
}

// SupervisordService owns declarative process groups and supervisord lifecycle.
type SupervisordService struct {
	mu sync.Mutex

	cfg    SupervisordConfig
	ctl    supervisord.Ctl
	daemon supervisord.Daemon

	state             supervisord.DesiredState
	configHash        string
	globalFingerprint string
	runAsUser         string

	healthCancel context.CancelFunc
}

// NewSupervisordService builds a service with default XDG paths when unset.
func NewSupervisordService(cfg SupervisordConfig) (*SupervisordService, error) {
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

	userName, err := supervisord.CurrentUserName()
	if err != nil {
		return nil, fmt.Errorf("supervisord user: %w", err)
	}

	s := &SupervisordService{
		cfg: cfg,
		ctl: supervisord.Ctl{
			Bin:        cfg.SupervisorctlBin,
			ConfigPath: cfg.ConfigPath,
		},
		daemon: supervisord.Daemon{
			Bin:        cfg.SupervisordBin,
			ConfigPath: cfg.ConfigPath,
			PIDFile:    cfg.PIDFile,
		},
		state:     cfg.InitialState,
		runAsUser: userName,
	}
	return s, nil
}

// Start ensures directories exist, renders config, and launches supervisord.
func (s *SupervisordService) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := supervisord.LookPath(s.cfg.SupervisordBin, s.cfg.SupervisorctlBin); err != nil {
		return err
	}
	if err := supervisord.EnsureDir(s.cfg.ConfigDir); err != nil {
		return err
	}
	if err := supervisord.EnsureDir(s.cfg.LogDir); err != nil {
		return err
	}
	if err := s.state.Validate(); err != nil {
		return err
	}

	content, hash, global, err := s.renderLocked()
	if err != nil {
		return err
	}

	running, _, err := s.daemon.IsRunning()
	if err != nil {
		return err
	}

	if !running {
		if err := supervisord.WriteConfigAtomically(s.cfg.ConfigPath, content); err != nil {
			return err
		}
		s.configHash = hash
		s.globalFingerprint = global
		if err := s.daemon.Start(ctx); err != nil {
			return err
		}
		return s.daemon.WaitUntilReady(ctx, &s.ctl, supervisordReadyTimeout)
	}

	onDisk, err := os.ReadFile(s.cfg.ConfigPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		return s.applyLocked(ctx, content, hash, global)
	}
	onDiskHash := hashBytes(onDisk)
	if onDiskHash != hash {
		return s.applyLocked(ctx, content, hash, global)
	}
	s.configHash = onDiskHash
	s.globalFingerprint = supervisord.GlobalFingerprint(onDisk)
	return s.daemon.WaitUntilReady(ctx, &s.ctl, supervisordReadyTimeout)
}

// Stop shuts down supervisord and managed programs.
func (s *SupervisordService) Stop(ctx context.Context) error {
	s.mu.Lock()
	if s.healthCancel != nil {
		s.healthCancel()
		s.healthCancel = nil
	}
	s.mu.Unlock()

	running, _, err := s.daemon.IsRunning()
	if err != nil {
		return err
	}
	if !running {
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, supervisordStopTimeout)
	defer cancel()
	if err := s.ctl.Shutdown(shutdownCtx); err != nil {
		_ = s.daemon.ForceStop()
		return err
	}
	if err := s.daemon.WaitUntilStopped(supervisordStopTimeout); err != nil {
		_ = s.daemon.ForceStop()
		return err
	}
	return nil
}

// Apply replaces desired state, re-renders config, and reloads supervisord.
func (s *SupervisordService) Apply(ctx context.Context, state supervisord.DesiredState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := state.Validate(); err != nil {
		return err
	}
	s.state = state

	content, hash, global, err := s.renderLocked()
	if err != nil {
		return err
	}
	if hash == s.configHash {
		return nil
	}
	return s.applyLocked(ctx, content, hash, global)
}

// SetGroups replaces all groups and applies the change.
func (s *SupervisordService) SetGroups(ctx context.Context, groups []supervisord.GroupSpec) error {
	return s.Apply(ctx, supervisord.DesiredState{Groups: groups})
}

// UpsertProgram adds or replaces a program inside a group.
func (s *SupervisordService) UpsertProgram(ctx context.Context, group string, prog supervisord.ProgramSpec) error {
	s.mu.Lock()
	groups := cloneGroups(s.state.Groups)
	s.mu.Unlock()

	idx := -1
	for i, g := range groups {
		if g.Name == group {
			idx = i
			break
		}
	}
	if idx < 0 {
		groups = append(groups, supervisord.GroupSpec{Name: group, Programs: []supervisord.ProgramSpec{prog}})
		return s.Apply(ctx, supervisord.DesiredState{Groups: groups})
	}

	progs := groups[idx].Programs
	found := false
	for i, p := range progs {
		if p.Name == prog.Name {
			progs[i] = prog
			found = true
			break
		}
	}
	if !found {
		progs = append(progs, prog)
	}
	groups[idx].Programs = progs
	return s.Apply(ctx, supervisord.DesiredState{Groups: groups})
}

// RemoveProgram removes a program from a group.
func (s *SupervisordService) RemoveProgram(ctx context.Context, group, name string) error {
	s.mu.Lock()
	groups := cloneGroups(s.state.Groups)
	s.mu.Unlock()

	idx := -1
	for i, g := range groups {
		if g.Name == group {
			idx = i
			break
		}
	}
	if idx < 0 {
		return supervisord.ErrGroupNotFound
	}

	progs := groups[idx].Programs
	out := progs[:0]
	found := false
	for _, p := range progs {
		if p.Name == name {
			found = true
			continue
		}
		out = append(out, p)
	}
	if !found {
		return supervisord.ErrProgramNotFound
	}
	groups[idx].Programs = out
	return s.Apply(ctx, supervisord.DesiredState{Groups: groups})
}

// ProgramStatus returns the status of one program.
func (s *SupervisordService) ProgramStatus(ctx context.Context, name string) (ProgramState, error) {
	all, err := s.AllStatus(ctx)
	if err != nil {
		return ProgramState{}, err
	}
	for _, row := range all {
		if row.Name == name {
			return row, nil
		}
	}
	return ProgramState{}, supervisord.ErrProgramNotFound
}

// GroupStatus returns statuses for programs in a group.
func (s *SupervisordService) GroupStatus(ctx context.Context, group string) ([]ProgramState, error) {
	all, err := s.AllStatus(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ProgramState, 0)
	for _, row := range all {
		if row.Group == group {
			out = append(out, row)
		}
	}
	return out, nil
}

// AllStatus returns every managed program status.
func (s *SupervisordService) AllStatus(ctx context.Context) ([]ProgramState, error) {
	rows, err := s.ctl.Status(ctx)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	state := s.state
	s.mu.Unlock()

	out := make([]ProgramState, 0, len(rows))
	for _, row := range rows {
		out = append(out, ProgramState{
			Name:   row.Name,
			Group:  state.ProgramGroup(row.Name),
			Status: row.Status,
			PID:    row.PID,
		})
	}
	return out, nil
}

// RestartProgram restarts one program without rewriting config.
func (s *SupervisordService) RestartProgram(ctx context.Context, name string) error {
	return s.ctl.RestartProgram(ctx, name)
}

// StopProgram stops one program without rewriting config.
func (s *SupervisordService) StopProgram(ctx context.Context, name string) error {
	return s.ctl.StopProgram(ctx, name)
}

// StartProgram starts one program without rewriting config.
func (s *SupervisordService) StartProgram(ctx context.Context, name string) error {
	return s.ctl.StartProgram(ctx, name)
}

// RunHealthLoop polls program status until ctx is cancelled.
func (s *SupervisordService) RunHealthLoop(ctx context.Context, interval time.Duration, onChange func([]ProgramState)) {
	loopCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.healthCancel = cancel
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		var prev map[string]string
		for {
			select {
			case <-loopCtx.Done():
				return
			case <-ticker.C:
				rows, err := s.AllStatus(loopCtx)
				if err != nil {
					s.tryRespawnDaemon(loopCtx)
					continue
				}
				next := make(map[string]string, len(rows))
				for _, row := range rows {
					next[row.Name] = row.Status
				}
				if statusMapChanged(prev, next) && onChange != nil {
					onChange(rows)
				}
				prev = next
			}
		}
	}()
}

// DefaultLocoProcesses returns the standard "loco" group for sibling processes.
// scripts-executor is included when executorSocket is non-empty.
func DefaultLocoProcesses(executable, executorSocket string) supervisord.DesiredState {
	if executorSocket == "" {
		return supervisord.DesiredState{}
	}
	return supervisord.DesiredState{
		Groups: []supervisord.GroupSpec{{
			Name: "loco",
			Programs: []supervisord.ProgramSpec{{
				Name:         "scripts-executor",
				Command:      fmt.Sprintf("%s scripts-executor --executor-socket %s", executable, executorSocket),
				Autostart:    true,
				Autorestart:  true,
				StopWaitSecs: 5,
			}},
		}},
	}
}

// RedisConfig collects the few knobs InfraProcesses exposes for the
// managed redis-server child. Redis is mandatory in BigFred (state
// cache + cross-process pub/sub between loco-server and dcc-bus); the
// only way to skip the managed instance is to point loco-server at an
// external one with --redis-external, which sets RedisConfig.Disable.
type RedisConfig struct {
	// Bin is the absolute or PATH-relative redis-server binary.
	// Defaults to "redis-server" when empty.
	Bin string
	// BindAddr is the interface redis-server binds on. Defaults to
	// "127.0.0.1" — Redis carries privileged state (sessions, port
	// allocations, pub/sub), so the daemon MUST stay on loopback
	// unless the operator explicitly widens it.
	BindAddr string
	// Port is the TCP port the managed redis-server listens on.
	// Defaults to 6379 (the upstream default) but operators commonly
	// pick a non-default port like 6380 to avoid colliding with a
	// pre-existing system Redis. Pass 0 for the default.
	Port uint16
	// DataDir is the working directory for redis-server. Defaults to
	// the supervisord log directory so dump.rdb / appendonly.aof
	// stay co-located with the loco-server runtime. Redis is run
	// with `--save ""` and `--appendonly no` by default (see
	// `EphemeralPersistence`), so this dir mostly stays empty in
	// practice.
	DataDir string
	// EphemeralPersistence, when true (default), disables RDB
	// snapshots and AOF — state is cheap to rebuild from SQLite +
	// re-issued daemon pulls on next boot, so persisting it adds
	// latency for no upside.
	EphemeralPersistence bool
	// Disable, when true, removes redis-server from the managed
	// process set. Used when the operator runs an external Redis
	// (e.g. on another host) and points loco-server at it via
	// --redis-addr.
	Disable bool
}

// DefaultInfraProcesses returns the "infra" group seeded with the
// shared Redis instance every BigFred component talks to. Returns an
// empty DesiredState when cfg.Disable is true so the caller may merge
// it unconditionally without re-checking.
func DefaultInfraProcesses(cfg RedisConfig) supervisord.DesiredState {
	if cfg.Disable {
		return supervisord.DesiredState{}
	}
	bin := cfg.Bin
	if bin == "" {
		bin = "redis-server"
	}
	bind := cfg.BindAddr
	if bind == "" {
		bind = "127.0.0.1"
	}
	port := cfg.Port
	if port == 0 {
		port = 6379
	}
	dataDir := cfg.DataDir
	if dataDir == "" {
		dataDir = "."
	}

	// `--save ""` disables RDB snapshots; `--appendonly no` disables
	// AOF. We also pass `--daemonize no` so supervisord (not redis)
	// owns the process. `--protected-mode no` is intentional: the
	// bind clause already pins us to loopback, and Redis's protected
	// mode complains the moment we touch it from a sibling process
	// (loco-server / dcc-bus dial in over TCP, not Unix socket).
	persistArgs := ""
	if cfg.EphemeralPersistence {
		persistArgs = `--save "" --appendonly no `
	}
	cmd := fmt.Sprintf(
		`%s --bind %s --port %d --dir %s --daemonize no --protected-mode no --logfile "" %s`,
		bin, bind, port, dataDir, persistArgs,
	)

	return supervisord.DesiredState{
		Groups: []supervisord.GroupSpec{{
			Name: "infra",
			Programs: []supervisord.ProgramSpec{{
				Name:         "redis",
				Command:      cmd,
				Autostart:    true,
				Autorestart:  true,
				StartSecs:    2,
				StopWaitSecs: 10,
			}},
		}},
	}
}

// MergeDesiredStates concatenates desired-state slices, deduplicating
// by group name (earlier groups win). It is a convenience for
// callers that compose multiple DefaultXxxProcesses helpers.
func MergeDesiredStates(states ...supervisord.DesiredState) supervisord.DesiredState {
	out := supervisord.DesiredState{}
	seen := make(map[string]struct{}, 4)
	for _, st := range states {
		for _, g := range st.Groups {
			if _, dup := seen[g.Name]; dup {
				continue
			}
			seen[g.Name] = struct{}{}
			out.Groups = append(out.Groups, g)
		}
	}
	return out
}

func (s *SupervisordService) renderLocked() ([]byte, string, string, error) {
	content, err := supervisord.Render(supervisord.RenderInput{
		RunAsUser:  s.runAsUser,
		ConfigDir:  s.cfg.ConfigDir,
		SocketPath: s.cfg.SocketPath,
		PIDFile:    s.cfg.PIDFile,
		LogDir:     s.cfg.LogDir,
		Groups:     s.state.Groups,
	})
	if err != nil {
		return nil, "", "", err
	}
	hash := hashBytes(content)
	global := supervisord.GlobalFingerprint(content)
	return content, hash, global, nil
}

func (s *SupervisordService) applyLocked(ctx context.Context, content []byte, hash, global string) error {
	prevGlobal := s.globalFingerprint

	if err := supervisord.WriteConfigAtomically(s.cfg.ConfigPath, content); err != nil {
		return err
	}

	applyCtx, cancel := context.WithTimeout(ctx, supervisordApplyTimeout)
	defer cancel()

	running, _, err := s.daemon.IsRunning()
	if err != nil {
		return err
	}
	if !running {
		if err := s.daemon.Start(applyCtx); err != nil {
			return s.rollbackApply(err)
		}
		if err := s.daemon.WaitUntilReady(applyCtx, &s.ctl, supervisordReadyTimeout); err != nil {
			return s.rollbackApply(err)
		}
		s.configHash = hash
		s.globalFingerprint = global
		return nil
	}

	var applyErr error
	if prevGlobal != "" && prevGlobal != global {
		applyErr = s.restartDaemonLocked(applyCtx)
	} else {
		if err := s.ctl.Reread(applyCtx); err != nil {
			applyErr = err
		} else if err := s.ctl.Update(applyCtx); err != nil {
			applyErr = err
		}
	}
	if applyErr != nil {
		return s.rollbackApply(applyErr)
	}

	s.configHash = hash
	s.globalFingerprint = global
	return nil
}

func (s *SupervisordService) restartDaemonLocked(ctx context.Context) error {
	if err := s.ctl.Shutdown(ctx); err != nil {
		_ = s.daemon.ForceStop()
	} else if err := s.daemon.WaitUntilStopped(supervisordStopTimeout); err != nil {
		_ = s.daemon.ForceStop()
	}
	if err := s.daemon.Start(ctx); err != nil {
		return fmt.Errorf("%w: %v", supervisord.ErrDaemonRestart, err)
	}
	if err := s.daemon.WaitUntilReady(ctx, &s.ctl, supervisordApplyTimeout); err != nil {
		return fmt.Errorf("%w: %v", supervisord.ErrDaemonRestart, err)
	}
	return nil
}

func (s *SupervisordService) rollbackApply(cause error) error {
	if err := supervisord.RestoreConfigPrev(s.cfg.ConfigPath); err != nil {
		return fmt.Errorf("%w (rollback failed: %v)", cause, err)
	}
	onDisk, err := os.ReadFile(s.cfg.ConfigPath)
	if err == nil {
		s.configHash = hashBytes(onDisk)
		s.globalFingerprint = supervisord.GlobalFingerprint(onDisk)
	}
	ctx, cancel := context.WithTimeout(context.Background(), supervisordApplyTimeout)
	defer cancel()
	_ = s.restartDaemonLocked(ctx)
	return fmt.Errorf("%w: %v", supervisord.ErrReloadFailed, cause)
}

func (s *SupervisordService) tryRespawnDaemon(ctx context.Context) {
	running, _, err := s.daemon.IsRunning()
	if err != nil || running {
		return
	}
	_ = s.daemon.Start(ctx)
	_ = s.daemon.WaitUntilReady(ctx, &s.ctl, supervisordReadyTimeout)
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func cloneGroups(in []supervisord.GroupSpec) []supervisord.GroupSpec {
	out := make([]supervisord.GroupSpec, len(in))
	for i, g := range in {
		out[i] = supervisord.GroupSpec{
			Name:     g.Name,
			Programs: append([]supervisord.ProgramSpec(nil), g.Programs...),
		}
	}
	return out
}

func statusMapChanged(prev, next map[string]string) bool {
	if len(prev) != len(next) {
		return true
	}
	for k, v := range next {
		if prev[k] != v {
			return true
		}
	}
	return false
}

// ExecutorSocketPath returns the default Unix socket for scripts-executor RPC.
func ExecutorSocketPath() (string, error) {
	runtime := os.Getenv("XDG_RUNTIME_DIR")
	if runtime == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		runtime = cache
	}
	return fmt.Sprintf("%s/loco/exec.sock", runtime), nil
}

// ConfigMatches renders state and compares to on-disk config without applying.
func (s *SupervisordService) ConfigMatches(state supervisord.DesiredState) (bool, error) {
	if err := state.Validate(); err != nil {
		return false, err
	}
	content, err := supervisord.Render(supervisord.RenderInput{
		RunAsUser:  s.runAsUser,
		ConfigDir:  s.cfg.ConfigDir,
		SocketPath: s.cfg.SocketPath,
		PIDFile:    s.cfg.PIDFile,
		LogDir:     s.cfg.LogDir,
		Groups:     state.Groups,
	})
	if err != nil {
		return false, err
	}
	onDisk, err := os.ReadFile(s.cfg.ConfigPath)
	if err != nil {
		return false, err
	}
	return bytes.Equal(content, onDisk), nil
}
