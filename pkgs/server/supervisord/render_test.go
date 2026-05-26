package supervisord_test

import (
	"strings"
	"testing"

	"github.com/keskad/loco/pkgs/server/supervisord"
)

func TestDesiredStateValidate(t *testing.T) {
	valid := supervisord.DesiredState{
		Groups: []supervisord.GroupSpec{{
			Name: "loco",
			Programs: []supervisord.ProgramSpec{{
				Name: "scripts-executor", Command: "echo hi",
				Autostart: true, Autorestart: true,
			}},
		}},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid state: %v", err)
	}

	cases := []supervisord.DesiredState{
		{Groups: []supervisord.GroupSpec{{Name: "loco", Programs: []supervisord.ProgramSpec{
			{Name: "Bad Name", Command: "x"},
		}}}},
		{Groups: []supervisord.GroupSpec{{Name: "loco", Programs: []supervisord.ProgramSpec{
			{Name: "worker", Command: ""},
		}}}},
		{Groups: []supervisord.GroupSpec{
			{Name: "a", Programs: []supervisord.ProgramSpec{{Name: "worker", Command: "x"}}},
			{Name: "b", Programs: []supervisord.ProgramSpec{{Name: "worker", Command: "y"}}},
		}},
	}
	for i, st := range cases {
		if err := st.Validate(); err == nil {
			t.Fatalf("case %d: expected validation error", i)
		}
	}
}

func TestRenderSingleProgram(t *testing.T) {
	out, err := supervisord.Render(supervisord.RenderInput{
		RunAsUser:  "alice",
		ConfigDir:  "/run/loco/supervisord",
		SocketPath: "/run/loco/supervisord/supervisor.sock",
		PIDFile:    "/run/loco/supervisord/supervisord.pid",
		LogDir:     "/cache/loco/supervisord",
		Groups: []supervisord.GroupSpec{{
			Name: "loco",
			Programs: []supervisord.ProgramSpec{{
				Name:        "scripts-executor",
				Command:     "/usr/bin/loco scripts-executor --executor-socket /run/loco/exec.sock",
				Autostart:   true,
				Autorestart: true,
				StopWaitSecs: 5,
			}},
		}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	text := string(out)
	checks := []string{
		"file=/run/loco/supervisord/supervisor.sock",
		"user=alice",
		"[group:loco]",
		"programs=scripts-executor",
		"[program:scripts-executor]",
		"command=/bin/bash -c '/usr/bin/loco scripts-executor --executor-socket /run/loco/exec.sock'",
		"autostart=true",
		"autorestart=true",
		"stopwaitsecs=5",
		"/cache/loco/supervisord/scripts-executor.stdout.log",
	}
	for _, want := range checks {
		if !strings.Contains(text, want) {
			t.Errorf("render missing %q\n%s", want, text)
		}
	}
}

func TestRenderShellQuoteEscaping(t *testing.T) {
	out, err := supervisord.Render(supervisord.RenderInput{
		RunAsUser:  "bob",
		ConfigDir:  "/cfg",
		SocketPath: "/cfg/supervisor.sock",
		PIDFile:    "/cfg/supervisord.pid",
		LogDir:     "/log",
		Groups: []supervisord.GroupSpec{{
			Name: "loco",
			Programs: []supervisord.ProgramSpec{{
				Name: "worker", Command: "echo it's fine",
				Autostart: false, Autorestart: false,
			}},
		}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(string(out), `command=/bin/bash -c 'echo it'\''s fine'`) {
		t.Fatalf("unexpected quote escaping:\n%s", out)
	}
}

func TestGlobalFingerprintIgnoresPrograms(t *testing.T) {
	base, err := supervisord.Render(supervisord.RenderInput{
		RunAsUser: "u", ConfigDir: "/c", SocketPath: "/c/s.sock",
		PIDFile: "/c/s.pid", LogDir: "/l",
		Groups: []supervisord.GroupSpec{{
			Name: "loco",
			Programs: []supervisord.ProgramSpec{{
				Name: "a", Command: "one", Autostart: true, Autorestart: true,
			}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	changed, err := supervisord.Render(supervisord.RenderInput{
		RunAsUser: "u", ConfigDir: "/c", SocketPath: "/c/s.sock",
		PIDFile: "/c/s.pid", LogDir: "/l",
		Groups: []supervisord.GroupSpec{{
			Name: "loco",
			Programs: []supervisord.ProgramSpec{{
				Name: "a", Command: "two", Autostart: true, Autorestart: true,
			}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if supervisord.GlobalFingerprint(base) != supervisord.GlobalFingerprint(changed) {
		t.Fatal("program-only change should not alter global fingerprint")
	}

	globalChanged, err := supervisord.Render(supervisord.RenderInput{
		RunAsUser: "u", ConfigDir: "/c", SocketPath: "/c/other.sock",
		PIDFile: "/c/s.pid", LogDir: "/l",
	})
	if err != nil {
		t.Fatal(err)
	}
	if supervisord.GlobalFingerprint(base) == supervisord.GlobalFingerprint(globalChanged) {
		t.Fatal("socket path change should alter global fingerprint")
	}
}