package service

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/keskad/loco/pkgs/server/supervisord"
)

var (
	// ErrDiagnosticsForbidden is returned when a file id does not map
	// to a whitelisted path.
	ErrDiagnosticsForbidden = errors.New("diagnostics: file not allowed")
	// ErrDiagnosticsNotFound is returned when the whitelisted file
	// does not exist on disk.
	ErrDiagnosticsNotFound = errors.New("diagnostics: file not found")
)

const (
	diagnosticsMaxReadBytes = 512 * 1024
	diagnosticsMaxTailLines = 10_000
)

// DiagnosticEntry is one readable file exposed to admins.
type DiagnosticEntry struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// DiagnosticGroup groups related log/config files for the UI select.
type DiagnosticGroup struct {
	ID      string            `json:"id"`
	Label   string            `json:"label"`
	Entries []DiagnosticEntry `json:"entries"`
}

// DiagnosticSources is the whitelist catalogue returned to the SPA.
type DiagnosticSources struct {
	Groups []DiagnosticGroup `json:"groups"`
}

// DiagnosticContent is a slice of a whitelisted text file.
type DiagnosticContent struct {
	FileID    string `json:"fileId"`
	FileName  string `json:"fileName"`
	Size      int64  `json:"size"`
	Truncated bool   `json:"truncated"`
	Content   string `json:"content"`
}

// DiagnosticsService exposes read-only access to a fixed set of
// supervisord/redis/dcc-bus log paths under the active LogDir.
type DiagnosticsService struct {
	configPath string
	logDir     string
}

// NewDiagnosticsService builds a catalogue from supervisord paths.
// When sup is nil, default XDG paths are used (files may be absent).
func NewDiagnosticsService(sup *SupervisordService) (*DiagnosticsService, error) {
	var configPath, logDir string
	if sup != nil {
		configPath, logDir = sup.Paths()
	} else {
		paths, err := supervisord.DefaultPaths()
		if err != nil {
			return nil, err
		}
		configPath = paths.ConfigPath
		logDir = paths.LogDir
	}
	return &DiagnosticsService{
		configPath: configPath,
		logDir:     logDir,
	}, nil
}

// Sources returns the admin-visible whitelist (no filesystem paths).
func (d *DiagnosticsService) Sources() (DiagnosticSources, error) {
	dccEntries, err := d.listDccBusEntries()
	if err != nil {
		return DiagnosticSources{}, err
	}

	groups := []DiagnosticGroup{
		{
			ID:    "supervisord",
			Label: "Supervisord",
			Entries: []DiagnosticEntry{
				{ID: "supervisord.log", Label: "supervisord.log"},
				{ID: "supervisord.config", Label: filepath.Base(d.configPath)},
			},
		},
		{
			ID:    "redis",
			Label: "Redis",
			Entries: []DiagnosticEntry{
				{ID: "redis.stdout", Label: "redis.stdout.log"},
				{ID: "redis.stderr", Label: "redis.stderr.log"},
			},
		},
		{
			ID:      "dcc-bus",
			Label:   "dcc-bus",
			Entries: dccEntries,
		},
	}
	return DiagnosticSources{Groups: groups}, nil
}

// Read returns up to the last tailLines of a whitelisted file.
func (d *DiagnosticsService) Read(fileID string, tailLines int) (DiagnosticContent, error) {
	if tailLines <= 0 {
		tailLines = 500
	}
	if tailLines > diagnosticsMaxTailLines {
		tailLines = diagnosticsMaxTailLines
	}

	abs, fileName, err := d.resolveFileID(fileID)
	if err != nil {
		return DiagnosticContent{}, err
	}

	st, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return DiagnosticContent{}, ErrDiagnosticsNotFound
		}
		return DiagnosticContent{}, err
	}
	if st.IsDir() {
		return DiagnosticContent{}, ErrDiagnosticsForbidden
	}

	f, err := os.Open(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return DiagnosticContent{}, ErrDiagnosticsNotFound
		}
		return DiagnosticContent{}, err
	}
	defer f.Close()

	content, truncated, err := readTailText(f, st.Size(), tailLines)
	if err != nil {
		return DiagnosticContent{}, err
	}

	return DiagnosticContent{
		FileID:    fileID,
		FileName:  fileName,
		Size:      st.Size(),
		Truncated: truncated,
		Content:   content,
	}, nil
}

func (d *DiagnosticsService) listDccBusEntries() ([]DiagnosticEntry, error) {
	ents, err := os.ReadDir(d.logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []DiagnosticEntry
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !isDccBusLogFile(name) {
			continue
		}
		out = append(out, DiagnosticEntry{
			ID:    dccBusFileID(name),
			Label: name,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out, nil
}

func isDccBusLogFile(name string) bool {
	if !strings.HasPrefix(name, "dcc-bus") {
		return false
	}
	return strings.HasSuffix(name, ".stdout.log") || strings.HasSuffix(name, ".stderr.log")
}

func dccBusFileID(fileName string) string {
	return "dcc-bus." + fileName
}

func (d *DiagnosticsService) resolveFileID(fileID string) (absPath, displayName string, err error) {
	switch fileID {
	case "supervisord.log":
		return filepath.Join(d.logDir, "supervisord.log"), "supervisord.log", nil
	case "supervisord.config":
		return d.configPath, filepath.Base(d.configPath), nil
	case "redis.stdout":
		return filepath.Join(d.logDir, "redis.stdout.log"), "redis.stdout.log", nil
	case "redis.stderr":
		return filepath.Join(d.logDir, "redis.stderr.log"), "redis.stderr.log", nil
	default:
		if strings.HasPrefix(fileID, "dcc-bus.") {
			name := strings.TrimPrefix(fileID, "dcc-bus.")
			if name == "" || name != filepath.Base(name) || !isDccBusLogFile(name) {
				return "", "", ErrDiagnosticsForbidden
			}
			abs := filepath.Join(d.logDir, name)
			if err := d.validateUnderLogDir(abs); err != nil {
				return "", "", err
			}
			return abs, name, nil
		}
		return "", "", ErrDiagnosticsForbidden
	}
}

func (d *DiagnosticsService) validateUnderLogDir(abs string) error {
	logDir, err := filepath.Abs(d.logDir)
	if err != nil {
		return err
	}
	clean, err := filepath.Abs(abs)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(logDir, clean)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ErrDiagnosticsForbidden
	}
	return nil
}

func readTailText(f *os.File, size int64, tailLines int) (string, bool, error) {
	if size == 0 {
		return "", false, nil
	}

	chunk := int64(diagnosticsMaxReadBytes)
	if chunk > size {
		chunk = size
	}
	if _, err := f.Seek(-chunk, io.SeekEnd); err != nil {
		return "", false, err
	}
	buf := make([]byte, chunk)
	n, err := io.ReadFull(f, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", false, err
	}
	buf = buf[:n]
	text := string(buf)
	truncated := chunk < size

	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > tailLines {
		lines = lines[len(lines)-tailLines:]
		truncated = true
	}
	out := strings.Join(lines, "\n")
	if !utf8.ValidString(out) {
		return "", false, fmt.Errorf("diagnostics: file is not valid UTF-8 text")
	}
	return out, truncated, nil
}
