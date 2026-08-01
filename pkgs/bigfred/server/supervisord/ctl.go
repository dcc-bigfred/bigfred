package supervisord

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Ctl wraps supervisorctl for a single config file.
type Ctl struct {
	Bin        string
	ConfigPath string
}

// ProgramStatus is one row from supervisorctl status.
type ProgramStatus struct {
	Name   string
	Status string
	PID    int
}

var statusLinePattern = regexp.MustCompile(`^(\S+)\s+(\S+)(?:\s+pid\s+(\d+))?`)

// Status runs supervisorctl status and parses the output.
// supervisorctl exits with code 3 when any program is not RUNNING; that is
// still a successful status listing and must not be treated as failure.
func (c *Ctl) Status(ctx context.Context) ([]ProgramStatus, error) {
	out, err := c.runStatus(ctx)
	if err != nil {
		return nil, err
	}
	return parseStatusOutput(out), nil
}

// Reread re-parses the on-disk config file.
func (c *Ctl) Reread(ctx context.Context) error {
	_, err := c.run(ctx, "reread")
	return err
}

// Update applies config diffs to running programs.
func (c *Ctl) Update(ctx context.Context) error {
	_, err := c.run(ctx, "update")
	return err
}

// Shutdown stops supervisord and all managed programs.
func (c *Ctl) Shutdown(ctx context.Context) error {
	_, err := c.run(ctx, "shutdown")
	return err
}

// StartProgram starts a single program.
// name may be bare ("dcc-bus-1-2") or group-qualified ("dcc-bus:dcc-bus-1-2").
func (c *Ctl) StartProgram(ctx context.Context, name string) error {
	_, err := c.run(ctx, "start", name)
	return err
}

// StopProgram stops a single program.
func (c *Ctl) StopProgram(ctx context.Context, name string) error {
	_, err := c.run(ctx, "stop", name)
	return err
}

// RestartProgram restarts a single program via supervisorctl restart.
func (c *Ctl) RestartProgram(ctx context.Context, name string) error {
	_, err := c.run(ctx, "restart", name)
	return err
}

// Ping checks whether supervisord responds via supervisorctl (HTTP ctl).
func (c *Ctl) Ping(ctx context.Context) error {
	_, err := c.run(ctx, "pid")
	return err
}

func (c *Ctl) run(ctx context.Context, args ...string) (string, error) {
	out, err := c.runRaw(ctx, args...)
	if err != nil {
		return "", err
	}
	return out, nil
}

// runStatus is like run("status") but treats exit code 3 as success.
func (c *Ctl) runStatus(ctx context.Context) (string, error) {
	out, err := c.runRaw(ctx, "status")
	if err == nil {
		return out, nil
	}
	if isSupervisorStatusPartialExit(err) && strings.TrimSpace(out) != "" {
		return out, nil
	}
	return "", err
}

func (c *Ctl) runRaw(ctx context.Context, args ...string) (string, error) {
	bin := c.Bin
	if bin == "" {
		bin = "supervisorctl"
	}
	cmdArgs := append([]string{"-c", c.ConfigPath}, args...)
	cmd := exec.CommandContext(ctx, bin, cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := stdout.String()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(out)
		}
		if msg == "" {
			msg = err.Error()
		}
		return out, &supervisorctlError{
			args:    args,
			exitErr: err,
			msg:     msg,
		}
	}
	return out, nil
}

type supervisorctlError struct {
	args    []string
	exitErr error
	msg     string
}

func (e *supervisorctlError) Error() string {
	return fmt.Sprintf("supervisorctl %s: %s", strings.Join(e.args, " "), e.msg)
}

func (e *supervisorctlError) Unwrap() error { return e.exitErr }

// isSupervisorStatusPartialExit reports whether err comes from
// supervisorctl status exiting 3 (some programs not RUNNING).
func isSupervisorStatusPartialExit(err error) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	return exitErr.ExitCode() == 3
}

func parseStatusOutput(out string) []ProgramStatus {
	var rows []ProgramStatus
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := statusLinePattern.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		row := ProgramStatus{Name: m[1], Status: m[2]}
		if len(m) >= 4 && m[3] != "" {
			row.PID, _ = strconv.Atoi(m[3])
		}
		rows = append(rows, row)
	}
	return rows
}

// LookPath verifies that supervisord and supervisorctl exist.
func LookPath(supervisordBin, supervisorctlBin string) error {
	if supervisordBin == "" {
		supervisordBin = "supervisord"
	}
	if supervisorctlBin == "" {
		supervisorctlBin = "supervisorctl"
	}
	if _, err := exec.LookPath(supervisordBin); err != nil {
		return fmt.Errorf("%w: supervisord", ErrBinaryNotFound)
	}
	if _, err := exec.LookPath(supervisorctlBin); err != nil {
		return fmt.Errorf("%w: supervisorctl", ErrBinaryNotFound)
	}
	return nil
}
