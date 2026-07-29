package supervisord_test

import (
	"os"
	"strings"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/server/supervisord"
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
		RunAsUser:    "alice",
		ConfigDir:    "/run/loco/supervisord",
		InetHTTPAddr: "127.0.0.1:9001",
		PIDFile:      "/run/loco/supervisord/supervisord.pid",
		LogDir:       "/cache/loco/supervisord",
		Shell:        "/bin/bash",
		Groups: []supervisord.GroupSpec{{
			Name: "loco",
			Programs: []supervisord.ProgramSpec{{
				Name:         "scripts-executor",
				Command:      "/usr/bin/loco scripts-executor --executor-socket /run/loco/exec.sock",
				Autostart:    true,
				Autorestart:  true,
				StopWaitSecs: 5,
			}},
		}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	text := string(out)
	checks := []string{
		"[inet_http_server]",
		"port=127.0.0.1:9001",
		"serverurl=http://127.0.0.1:9001",
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
	if strings.Contains(text, "[unix_http_server]") || strings.Contains(text, "unix://") {
		t.Fatalf("render still uses unix socket:\n%s", text)
	}
}

func TestRenderShellQuoteEscaping(t *testing.T) {
	out, err := supervisord.Render(supervisord.RenderInput{
		RunAsUser:    "bob",
		ConfigDir:    "/cfg",
		InetHTTPAddr: "127.0.0.1:9001",
		PIDFile:      "/cfg/supervisord.pid",
		LogDir:       "/log",
		Shell:        "/bin/bash",
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

func TestRenderUsesAndroidShell(t *testing.T) {
	out, err := supervisord.Render(supervisord.RenderInput{
		RunAsUser: "u", ConfigDir: "/c", InetHTTPAddr: "127.0.0.1:9001",
		PIDFile: "/c/s.pid", LogDir: "/l", Shell: "/system/bin/sh",
		Groups: []supervisord.GroupSpec{{
			Name: "loco",
			Programs: []supervisord.ProgramSpec{{
				Name: "a", Command: "echo hi", Autostart: true, Autorestart: true,
			}},
		}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(string(out), "command=/system/bin/sh -c 'echo hi'") {
		t.Fatalf("expected Android shell wrapper:\n%s", out)
	}
}

func TestDefaultShellResolvesExisting(t *testing.T) {
	sh := supervisord.DefaultShell()
	if sh == "" {
		t.Fatal("DefaultShell returned empty")
	}
	if _, err := os.Stat(sh); err != nil {
		// Last-resort fallback may not exist in minimal containers; allow /bin/sh.
		if sh != "/bin/sh" {
			t.Fatalf("DefaultShell %q is not usable: %v", sh, err)
		}
	}
}

func TestGlobalFingerprintIgnoresPrograms(t *testing.T) {
	base, err := supervisord.Render(supervisord.RenderInput{
		RunAsUser: "u", ConfigDir: "/c", InetHTTPAddr: "127.0.0.1:9001",
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
		RunAsUser: "u", ConfigDir: "/c", InetHTTPAddr: "127.0.0.1:9001",
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
		RunAsUser: "u", ConfigDir: "/c", InetHTTPAddr: "127.0.0.1:9002",
		PIDFile: "/c/s.pid", LogDir: "/l",
	})
	if err != nil {
		t.Fatal(err)
	}
	if supervisord.GlobalFingerprint(base) == supervisord.GlobalFingerprint(globalChanged) {
		t.Fatal("HTTP addr change should alter global fingerprint")
	}
}
