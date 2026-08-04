package microinit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func BoolPtr(v bool) *bool { return &v }
func IntPtr(v int) *int    { return &v }

func WriteDropin(dir, group, name string, svc ServiceDef) error {
	if err := validateName(group); err != nil {
		return fmt.Errorf("drop-in group: %w", err)
	}
	if err := validateName(name); err != nil {
		return fmt.Errorf("drop-in name: %w", err)
	}
	if svc.Name == "" {
		svc.Name = name
	}
	if svc.Name != name {
		return fmt.Errorf("drop-in name %q does not match service %q", name, svc.Name)
	}
	content, err := json.MarshalIndent(DropinFile{Services: []ServiceDef{svc}}, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomically(filepath.Join(dir, group, name+".json"), append(content, '\n'))
}

func RemoveDropin(dir, group, name string) error {
	if err := validateName(group); err != nil {
		return err
	}
	if err := validateName(name); err != nil {
		return err
	}
	err := os.Remove(filepath.Join(dir, group, name+".json"))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func SyncGroup(dir, group string, desired map[string]ServiceDef) error {
	if err := validateName(group); err != nil {
		return err
	}
	groupDir := filepath.Join(dir, group)
	if err := os.MkdirAll(groupDir, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(groupDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		name := entry.Name()[:len(entry.Name())-len(".json")]
		if _, ok := desired[name]; !ok {
			if err := os.Remove(filepath.Join(groupDir, entry.Name())); err != nil {
				return err
			}
		}
	}
	names := make([]string, 0, len(desired))
	for name := range desired {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := WriteDropin(dir, group, name, desired[name]); err != nil {
			return err
		}
	}
	return nil
}

func ListGroup(dir, group string) (map[string]ServiceDef, error) {
	groupDir := filepath.Join(dir, group)
	entries, err := os.ReadDir(groupDir)
	if os.IsNotExist(err) {
		return map[string]ServiceDef{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := make(map[string]ServiceDef)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(groupDir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var dropin DropinFile
		if err := json.Unmarshal(data, &dropin); err != nil {
			return nil, err
		}
		for _, svc := range dropin.Services {
			out[svc.Name] = svc
		}
	}
	return out, nil
}

func writeFileAtomically(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
