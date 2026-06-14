package supervisord

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed templates/supervisord.conf.tmpl
var templateFS embed.FS

// RenderInput carries everything needed to render supervisord.conf.
type RenderInput struct {
	RunAsUser  string
	ConfigDir  string
	SocketPath string
	PIDFile    string
	LogDir     string
	Groups     []GroupSpec
}

// Render executes the embedded template for the given input.
func Render(in RenderInput) ([]byte, error) {
	tmpl, err := template.New("supervisord.conf.tmpl").Funcs(template.FuncMap{
		"joinProgramNames": joinProgramNames,
		"shellQuote":       shellQuote,
		"orInt": func(v, def int) int {
			if v == 0 {
				return def
			}
			return v
		},
	}).ParseFS(templateFS, "templates/supervisord.conf.tmpl")
	if err != nil {
		return nil, fmt.Errorf("parse supervisord template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, in); err != nil {
		return nil, fmt.Errorf("execute supervisord template: %w", err)
	}
	return buf.Bytes(), nil
}

// GlobalFingerprint returns a stable hash of everything before the first
// [group:…] or [program:…] section — used to decide hot reload vs restart.
func GlobalFingerprint(content []byte) string {
	global := string(content)
	if idx := strings.Index(global, "\n[group:"); idx >= 0 {
		global = global[:idx]
	} else if idx := strings.Index(global, "\n[program:"); idx >= 0 {
		global = global[:idx]
	}
	sum := sha256.Sum256([]byte(global))
	return hex.EncodeToString(sum[:])
}

// WriteConfigAtomically renders and writes configPath, keeping a .prev backup.
func WriteConfigAtomically(path string, content []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir config dir: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, content, 0o600); err != nil {
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := syncFile(tmp); err != nil {
		_ = os.Remove(tmp)
		return err
	}

	if _, err := os.Stat(path); err == nil {
		prev := path + ".prev"
		if err := copyFile(path, prev); err != nil {
			_ = os.Remove(tmp)
			return fmt.Errorf("backup previous config: %w", err)
		}
	}

	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename config: %w", err)
	}
	return nil
}

// RestoreConfigPrev swaps supervisord.conf.prev back into place.
func RestoreConfigPrev(path string) error {
	prev := path + ".prev"
	if _, err := os.Stat(prev); err != nil {
		return fmt.Errorf("no previous config to restore: %w", err)
	}
	tmp := path + ".restore"
	if err := copyFile(prev, tmp); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func joinProgramNames(programs []ProgramSpec) string {
	names := make([]string, len(programs))
	for i, p := range programs {
		names[i] = p.Name
	}
	return strings.Join(names, ",")
}

// shellQuote wraps s for safe use inside /bin/bash -c '…'.
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func syncFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open for fsync: %w", err)
	}
	defer f.Close()
	return f.Sync()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
	}()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
