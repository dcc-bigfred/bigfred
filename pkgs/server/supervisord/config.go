package supervisord

import (
	"fmt"
	"regexp"
)

var programNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// ProgramSpec is one supervisord [program:NAME] entry.
type ProgramSpec struct {
	Name         string
	Command      string
	Autostart    bool
	Autorestart  bool
	StopWaitSecs int
	StartSecs    int
}

// GroupSpec groups programs under [group:NAME].
type GroupSpec struct {
	Name     string
	Programs []ProgramSpec
}

// DesiredState is the full desired supervisord configuration.
type DesiredState struct {
	Groups []GroupSpec
}

// Validate checks desired state invariants before rendering.
func (s DesiredState) Validate() error {
	seen := make(map[string]string, 16)
	for _, g := range s.Groups {
		if g.Name == "" {
			return fmt.Errorf("supervisord: group name is required")
		}
		for _, p := range g.Programs {
			if !programNamePattern.MatchString(p.Name) {
				return fmt.Errorf("%w: %q", ErrInvalidProgramName, p.Name)
			}
			if p.Command == "" {
				return fmt.Errorf("%w: %q", ErrInvalidCommand, p.Name)
			}
			if prev, ok := seen[p.Name]; ok {
				if prev != g.Name {
					return fmt.Errorf("%w: %q", ErrProgramInMultiGroup, p.Name)
				}
				return fmt.Errorf("%w: %q", ErrDuplicateProgram, p.Name)
			}
			seen[p.Name] = g.Name
		}
	}
	return nil
}

// ProgramGroup returns the group name for a program, or "" if unknown.
func (s DesiredState) ProgramGroup(name string) string {
	for _, g := range s.Groups {
		for _, p := range g.Programs {
			if p.Name == name {
				return g.Name
			}
		}
	}
	return ""
}
