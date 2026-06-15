package supervisord

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	readyTimeout = 10 * time.Second
	applyTimeout = 30 * time.Second
	stopTimeout  = 15 * time.Second
)

// Config configures Manager paths, binaries and initial desired state.
type Config struct {
	SupervisordBin   string
	SupervisorctlBin string

	ConfigDir  string
	ConfigPath string
	SocketPath string
	PIDFile    string
	LogDir     string

	InitialState DesiredState

	// Telemetry, when Enable is true, triggers templating of the BigFred
	// Alloy OTLP receiver config into ConfigDir before supervisord starts.
	Telemetry TelemetryConfig

	// Log receives lifecycle / apply messages. Nil disables logging (tests).
	Log *logrus.Logger
}

// ProgramState is the observable status of one managed program.
type ProgramState struct {
	Name   string
	Group  string
	Status string
	PID    int
}

// Manager owns declarative process groups and the supervisord lifecycle.
type Manager struct {
	mu sync.Mutex

	cfg    Config
	ctl    Ctl
	daemon Daemon

	state             DesiredState
	configHash        string
	globalFingerprint string
	runAsUser         string
	log               *logrus.Logger

	healthCancel context.CancelFunc
}

// NewManager builds a manager, filling in default XDG paths when unset.
func NewManager(cfg Config) (*Manager, error) {
	if cfg.ConfigPath == "" {
		paths, err := DefaultPaths()
		if err != nil {
			return nil, err
		}
		cfg.ConfigDir = paths.ConfigDir
		cfg.ConfigPath = paths.ConfigPath
		cfg.SocketPath = paths.SocketPath
		cfg.PIDFile = paths.PIDFile
		cfg.LogDir = paths.LogDir
	} else if cfg.ConfigDir == "" {
		cfg.ConfigDir = ConfigDirFromPath(cfg.ConfigPath)
	}

	userName, err := CurrentUserName()
	if err != nil {
		return nil, fmt.Errorf("supervisord user: %w", err)
	}

	return &Manager{
		cfg: cfg,
		ctl: Ctl{
			Bin:        cfg.SupervisorctlBin,
			ConfigPath: cfg.ConfigPath,
		},
		daemon: Daemon{
			Bin:        cfg.SupervisordBin,
			ConfigPath: cfg.ConfigPath,
			PIDFile:    cfg.PIDFile,
		},
		state:     cfg.InitialState,
		runAsUser: userName,
		log:       cfg.Log,
	}, nil
}

// Paths returns the active supervisord config file and log directory.
func (s *Manager) Paths() (configPath, logDir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.ConfigPath, s.cfg.LogDir
}

// Start ensures directories exist, renders config, and launches supervisord.
func (s *Manager) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := LookPath(s.cfg.SupervisordBin, s.cfg.SupervisorctlBin); err != nil {
		return err
	}
	if err := EnsureDir(s.cfg.ConfigDir); err != nil {
		return err
	}
	if err := EnsureDir(s.cfg.LogDir); err != nil {
		return err
	}
	if s.cfg.Telemetry.Enable {
		if err := PrepareAlloyTelemetry(s.cfg.ConfigDir, s.cfg.Telemetry); err != nil {
			return fmt.Errorf("prepare alloy telemetry config: %w", err)
		}
	}
	if err := s.state.Validate(); err != nil {
		return err
	}

	content, hash, global, err := s.renderLocked()
	if err != nil {
		return err
	}

	s.logInfo(logrus.Fields{
		"configPath": s.cfg.ConfigPath,
		"programs":   programSummary(s.state.Groups),
		"configHash": hashPrefix(hash),
	}, "supervisord bootstrap: rendered initial config")

	running, _, err := s.daemon.IsRunning()
	if err != nil {
		return err
	}

	if !running {
		s.logInfo(logrus.Fields{"configPath": s.cfg.ConfigPath}, "supervisord bootstrap: daemon not running, writing config and starting")
		if err := WriteConfigAtomically(s.cfg.ConfigPath, content); err != nil {
			return err
		}
		s.configHash = hash
		s.globalFingerprint = global
		if err := s.daemon.Start(ctx); err != nil {
			s.logError(err, logrus.Fields{"configPath": s.cfg.ConfigPath}, "supervisord bootstrap: start failed")
			return err
		}
		if err := s.daemon.WaitUntilReady(ctx, &s.ctl, readyTimeout); err != nil {
			s.logError(err, nil, "supervisord bootstrap: wait ready failed")
			return err
		}
		s.logInfo(nil, "supervisord bootstrap: daemon ready")
		return nil
	}

	s.logInfo(nil, "supervisord bootstrap: daemon already running, reconciling config")
	onDisk, err := os.ReadFile(s.cfg.ConfigPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		s.logInfo(nil, "supervisord bootstrap: config file missing, applying desired state")
		return s.applyLocked(ctx, content, hash, global, "bootstrap")
	}
	onDiskHash := hashBytes(onDisk)
	if onDiskHash != hash {
		s.logInfo(logrus.Fields{
			"onDiskHash": hashPrefix(onDiskHash),
			"wantHash":   hashPrefix(hash),
		}, "supervisord bootstrap: on-disk config stale, applying")
		return s.applyLocked(ctx, content, hash, global, "bootstrap")
	}
	s.configHash = onDiskHash
	s.globalFingerprint = GlobalFingerprint(onDisk)
	if err := s.daemon.WaitUntilReady(ctx, &s.ctl, readyTimeout); err != nil {
		s.logError(err, nil, "supervisord bootstrap: wait ready on existing daemon failed")
		return err
	}
	s.logInfo(logrus.Fields{"configHash": hashPrefix(onDiskHash)}, "supervisord bootstrap: config already current")
	return nil
}

// Stop shuts down supervisord and managed programs.
func (s *Manager) Stop(ctx context.Context) error {
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
		s.logInfo(nil, "supervisord shutdown: daemon not running")
		return nil
	}

	s.logInfo(logrus.Fields{"configPath": s.cfg.ConfigPath}, "supervisord shutdown: requesting graceful shutdown")
	shutdownCtx, cancel := context.WithTimeout(ctx, stopTimeout)
	defer cancel()
	if err := s.ctl.Shutdown(shutdownCtx); err != nil {
		s.logError(err, nil, "supervisord shutdown: supervisorctl shutdown failed, forcing stop")
		_ = s.daemon.ForceStop()
		return err
	}
	if err := s.daemon.WaitUntilStopped(stopTimeout); err != nil {
		s.logError(err, nil, "supervisord shutdown: wait stopped failed, forcing stop")
		_ = s.daemon.ForceStop()
		return err
	}
	s.logInfo(nil, "supervisord shutdown: complete")
	return nil
}

// Apply replaces desired state, re-renders config, and reloads supervisord.
func (s *Manager) Apply(ctx context.Context, state DesiredState) error {
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
		s.logDebug(logrus.Fields{
			"configHash": hashPrefix(hash),
			"programs":   programSummary(s.state.Groups),
		}, "supervisord apply: config unchanged, skipping")
		return nil
	}
	s.logInfo(logrus.Fields{
		"prevHash":   hashPrefix(s.configHash),
		"configHash": hashPrefix(hash),
		"programs":   programSummary(s.state.Groups),
	}, "supervisord apply: config changed")
	return s.applyLocked(ctx, content, hash, global, "apply")
}

// UpsertProgram adds or replaces a program inside a group.
func (s *Manager) UpsertProgram(ctx context.Context, group string, prog ProgramSpec) error {
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
		s.logInfo(logrus.Fields{"group": group, "program": prog.Name}, "supervisord upsert: creating group")
		groups = append(groups, GroupSpec{Name: group, Programs: []ProgramSpec{prog}})
		return s.Apply(ctx, DesiredState{Groups: groups})
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
	action := "update"
	if !found {
		action = "insert"
	}
	s.logInfo(logrus.Fields{"group": group, "program": prog.Name, "action": action}, "supervisord upsert program")
	return s.Apply(ctx, DesiredState{Groups: groups})
}

// ReplaceGroupPrograms swaps every program in a group in one Apply.
// Other groups are left untouched. Used when rebuilding the entire
// dcc-bus catalogue for a set of layouts.
func (s *Manager) ReplaceGroupPrograms(ctx context.Context, group string, programs []ProgramSpec) error {
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
	names := make([]string, len(programs))
	for i, p := range programs {
		names[i] = p.Name
	}
	if idx < 0 {
		s.logInfo(logrus.Fields{"group": group, "programs": names}, "supervisord replace group: creating group")
		groups = append(groups, GroupSpec{Name: group, Programs: programs})
	} else {
		s.logInfo(logrus.Fields{"group": group, "programs": names, "count": len(programs)}, "supervisord replace group")
		groups[idx].Programs = append([]ProgramSpec(nil), programs...)
	}
	return s.Apply(ctx, DesiredState{Groups: groups})
}

// RemoveProgram removes a program from a group.
func (s *Manager) RemoveProgram(ctx context.Context, group, name string) error {
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
		return ErrGroupNotFound
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
		return ErrProgramNotFound
	}
	s.logInfo(logrus.Fields{"group": group, "program": name}, "supervisord remove program")
	groups[idx].Programs = out
	return s.Apply(ctx, DesiredState{Groups: groups})
}

// StopProgram stops one program without rewriting config.
func (s *Manager) StopProgram(ctx context.Context, name string) error {
	return s.ctl.StopProgram(ctx, name)
}

// StartProgram starts one program without rewriting config.
func (s *Manager) StartProgram(ctx context.Context, name string) error {
	return s.ctl.StartProgram(ctx, name)
}

// RunHealthLoop polls program status until ctx is cancelled.
func (s *Manager) RunHealthLoop(ctx context.Context, interval time.Duration, onChange func([]ProgramState)) {
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
				rows, err := s.allStatus(loopCtx)
				if err != nil {
					s.logWarn(err, nil, "supervisord health: status poll failed, attempting respawn")
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

// allStatus returns every managed program status.
func (s *Manager) allStatus(ctx context.Context) ([]ProgramState, error) {
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

func (s *Manager) renderLocked() ([]byte, string, string, error) {
	content, err := Render(RenderInput{
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
	global := GlobalFingerprint(content)
	return content, hash, global, nil
}

func (s *Manager) applyLocked(ctx context.Context, content []byte, hash, global, reason string) error {
	prevGlobal := s.globalFingerprint

	s.logInfo(logrus.Fields{
		"reason":     reason,
		"configPath": s.cfg.ConfigPath,
		"configHash": hashPrefix(hash),
		"programs":   programSummary(s.state.Groups),
	}, "supervisord apply: writing config")

	if err := WriteConfigAtomically(s.cfg.ConfigPath, content); err != nil {
		s.logError(err, logrus.Fields{"reason": reason}, "supervisord apply: write config failed")
		return err
	}

	applyCtx, cancel := context.WithTimeout(ctx, applyTimeout)
	defer cancel()

	running, _, err := s.daemon.IsRunning()
	if err != nil {
		return err
	}
	if !running {
		s.logInfo(logrus.Fields{"reason": reason}, "supervisord apply: daemon down, starting")
		if err := s.daemon.Start(applyCtx); err != nil {
			return s.rollbackApply(err, reason)
		}
		if err := s.daemon.WaitUntilReady(applyCtx, &s.ctl, readyTimeout); err != nil {
			return s.rollbackApply(err, reason)
		}
		s.configHash = hash
		s.globalFingerprint = global
		s.logInfo(logrus.Fields{"reason": reason, "configHash": hashPrefix(hash)}, "supervisord apply: daemon started")
		return nil
	}

	var applyErr error
	reloadMode := "reread+update"
	if prevGlobal != "" && prevGlobal != global {
		reloadMode = "daemon-restart"
		s.logInfo(logrus.Fields{
			"reason":     reason,
			"prevGlobal": prevGlobal[:min(12, len(prevGlobal))],
			"newGlobal":  global[:min(12, len(global))],
		}, "supervisord apply: global section changed, restarting daemon")
		applyErr = s.restartDaemonLocked(applyCtx, reason)
	} else {
		s.logInfo(logrus.Fields{"reason": reason}, "supervisord apply: reloading programs (supervisorctl reread + update)")
		if err := s.ctl.Reread(applyCtx); err != nil {
			applyErr = err
		} else if err := s.ctl.Update(applyCtx); err != nil {
			applyErr = err
		}
	}
	if applyErr != nil {
		s.logError(applyErr, logrus.Fields{"reason": reason, "reloadMode": reloadMode}, "supervisord apply: reload failed")
		return s.rollbackApply(applyErr, reason)
	}

	s.configHash = hash
	s.globalFingerprint = global
	s.logInfo(logrus.Fields{"reason": reason, "reloadMode": reloadMode, "configHash": hashPrefix(hash)}, "supervisord apply: success")
	return nil
}

func (s *Manager) restartDaemonLocked(ctx context.Context, reason string) error {
	s.logInfo(logrus.Fields{"reason": reason}, "supervisord restart: shutting down daemon")
	if err := s.ctl.Shutdown(ctx); err != nil {
		s.logWarn(err, logrus.Fields{"reason": reason}, "supervisord restart: shutdown failed, forcing stop")
		_ = s.daemon.ForceStop()
	} else if err := s.daemon.WaitUntilStopped(stopTimeout); err != nil {
		s.logWarn(err, logrus.Fields{"reason": reason}, "supervisord restart: wait stopped failed, forcing stop")
		_ = s.daemon.ForceStop()
	}
	if err := s.daemon.Start(ctx); err != nil {
		s.logError(err, logrus.Fields{"reason": reason}, "supervisord restart: start failed")
		return fmt.Errorf("%w: %v", ErrDaemonRestart, err)
	}
	if err := s.daemon.WaitUntilReady(ctx, &s.ctl, applyTimeout); err != nil {
		s.logError(err, logrus.Fields{"reason": reason}, "supervisord restart: wait ready failed")
		return fmt.Errorf("%w: %v", ErrDaemonRestart, err)
	}
	s.logInfo(logrus.Fields{"reason": reason}, "supervisord restart: daemon ready")
	return nil
}

func (s *Manager) rollbackApply(cause error, reason string) error {
	s.logWarn(cause, logrus.Fields{"reason": reason}, "supervisord apply: rolling back config")
	if err := RestoreConfigPrev(s.cfg.ConfigPath); err != nil {
		s.logError(err, logrus.Fields{"reason": reason}, "supervisord apply: rollback restore failed")
		return fmt.Errorf("%w (rollback failed: %v)", cause, err)
	}
	onDisk, err := os.ReadFile(s.cfg.ConfigPath)
	if err == nil {
		s.configHash = hashBytes(onDisk)
		s.globalFingerprint = GlobalFingerprint(onDisk)
	}
	ctx, cancel := context.WithTimeout(context.Background(), applyTimeout)
	defer cancel()
	_ = s.restartDaemonLocked(ctx, reason+":rollback")
	return fmt.Errorf("%w: %v", ErrReloadFailed, cause)
}

func (s *Manager) tryRespawnDaemon(ctx context.Context) {
	running, _, err := s.daemon.IsRunning()
	if err != nil || running {
		return
	}
	s.logInfo(nil, "supervisord health: daemon not running, respawning")
	if err := s.daemon.Start(ctx); err != nil {
		s.logError(err, nil, "supervisord health: respawn start failed")
		return
	}
	if err := s.daemon.WaitUntilReady(ctx, &s.ctl, readyTimeout); err != nil {
		s.logError(err, nil, "supervisord health: respawn wait ready failed")
		return
	}
	s.logInfo(nil, "supervisord health: daemon respawned")
}

func (s *Manager) logInfo(fields logrus.Fields, msg string) {
	if s.log == nil {
		return
	}
	if fields == nil {
		s.log.Info(msg)
		return
	}
	s.log.WithFields(fields).Info(msg)
}

func (s *Manager) logDebug(fields logrus.Fields, msg string) {
	if s.log == nil {
		return
	}
	s.log.WithFields(fields).Debug(msg)
}

func (s *Manager) logWarn(err error, fields logrus.Fields, msg string) {
	if s.log == nil {
		return
	}
	entry := s.log.WithError(err)
	if fields != nil {
		entry = entry.WithFields(fields)
	}
	entry.Warn(msg)
}

func (s *Manager) logError(err error, fields logrus.Fields, msg string) {
	if s.log == nil {
		return
	}
	entry := s.log.WithError(err)
	if fields != nil {
		entry = entry.WithFields(fields)
	}
	entry.Error(msg)
}

func hashPrefix(hash string) string {
	if len(hash) <= 12 {
		return hash
	}
	return hash[:12]
}

// programSummary returns group → program names for logs.
func programSummary(groups []GroupSpec) map[string][]string {
	out := make(map[string][]string, len(groups))
	for _, g := range groups {
		names := make([]string, len(g.Programs))
		for i, p := range g.Programs {
			names[i] = p.Name
		}
		out[g.Name] = names
	}
	return out
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func cloneGroups(in []GroupSpec) []GroupSpec {
	out := make([]GroupSpec, len(in))
	for i, g := range in {
		out[i] = GroupSpec{
			Name:     g.Name,
			Programs: append([]ProgramSpec(nil), g.Programs...),
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
