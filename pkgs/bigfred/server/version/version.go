// Package version exposes build and release metadata for loco-server.
//
// buildCommit and buildTime are injected at compile time via -ldflags.
// version and tagCommit come from the optional ELF section
// ".bigfred.version" (JSON {"version":"v1.2.3","commit":"abc1234"}),
// which release tooling appends with objcopy after the binary is built.
package version

import (
	"debug/elf"
	"encoding/json"
	"os"
	"strings"
	"sync"
)

const sectionName = ".bigfred.version"

// Set by -ldflags at build time.
var (
	buildCommit = ""
	buildTime   = ""
)

// Info is the public version payload returned by GET /api/v1/version.
type Info struct {
	Product     string `json:"product"`
	Version     string `json:"version"`
	TagCommit   string `json:"tagCommit"`
	BuildCommit string `json:"buildCommit"`
	BuildTime   string `json:"buildTime"`
}

type sectionPayload struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

var (
	once sync.Once
	info Info
)

// Get returns the process version info. The ELF section is read once.
func Get() Info {
	once.Do(func() {
		info = Info{
			Product:     "bigfred",
			Version:     "dev",
			BuildCommit: buildCommit,
			BuildTime:   buildTime,
		}
		v, c, ok := readSection()
		if ok {
			if v != "" {
				info.Version = v
			}
			info.TagCommit = c
		}
	})
	return info
}

// String returns a human-readable summary for logs.
func String() string {
	i := Get()
	parts := []string{i.Version}
	if i.TagCommit != "" {
		parts = append(parts, "tag "+i.TagCommit)
	}
	if i.BuildCommit != "" {
		parts = append(parts, "build "+i.BuildCommit)
	}
	if i.BuildTime != "" {
		parts = append(parts, i.BuildTime)
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return parts[0] + " (" + strings.Join(parts[1:], ", ") + ")"
}

func readSection() (version, commit string, ok bool) {
	path, err := os.Executable()
	if err != nil {
		return "", "", false
	}
	return readSectionFrom(path)
}

func readSectionFrom(path string) (version, commit string, ok bool) {
	f, err := elf.Open(path)
	if err != nil {
		return "", "", false
	}
	defer f.Close()

	sec := f.Section(sectionName)
	if sec == nil {
		return "", "", false
	}
	data, err := sec.Data()
	if err != nil || len(data) == 0 {
		return "", "", false
	}

	var payload sectionPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		// Fall back to plain-text section contents (version only).
		v := strings.TrimSpace(string(data))
		if v == "" {
			return "", "", false
		}
		return v, "", true
	}
	return strings.TrimSpace(payload.Version), strings.TrimSpace(payload.Commit), true
}
