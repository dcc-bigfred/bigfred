package microinit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// baseConfigFile is the subset of microinit.json needed to detect
// system-declared services (not drop-ins).
type baseConfigFile struct {
	Services []struct {
		Name string `json:"name"`
	} `json:"services"`
}

// BaseConfigServiceNames returns service names declared in the main
// microinit.json (configPath). These are treated as system-owned: bigfred
// must not create or overwrite drop-ins for them.
func BaseConfigServiceNames(configPath string) (map[string]struct{}, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]struct{}{}, nil
		}
		return nil, fmt.Errorf("read microinit config %s: %w", configPath, err)
	}
	var cfg baseConfigFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse microinit config %s: %w", configPath, err)
	}
	out := make(map[string]struct{}, len(cfg.Services))
	for _, svc := range cfg.Services {
		if svc.Name == "" {
			continue
		}
		out[svc.Name] = struct{}{}
	}
	return out, nil
}

// DropinExists reports whether a drop-in file is present for group/name.
func DropinExists(dir, group, name string) bool {
	if err := validateName(group); err != nil {
		return false
	}
	if err := validateName(name); err != nil {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, group, name+".json"))
	return err == nil
}
