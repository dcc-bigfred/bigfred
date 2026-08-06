package microinit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureMicrodnsConfigWritesDefaultWhenMissing(t *testing.T) {
	root := t.TempDir()
	t.Setenv("BIGFRED_DATA_DIR", root)
	t.Setenv("DATA_DIR", "")

	path := filepath.Join(root, "etc", "microdns.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected missing config before Ensure: %v", err)
	}

	if err := EnsureMicrodnsConfig(); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, raw)
	}
	dccBus, ok := cfg["dccBus"].(map[string]any)
	if !ok {
		t.Fatalf("missing dccBus: %#v", cfg)
	}
	if enabled, _ := dccBus["enabled"].(bool); !enabled {
		t.Fatalf("dccBus.enabled = %#v, want true", dccBus["enabled"])
	}
	services, ok := cfg["services"].([]any)
	if !ok || len(services) != 1 {
		t.Fatalf("services = %#v", cfg["services"])
	}
}

func TestEnsureMicrodnsConfigPreservesExisting(t *testing.T) {
	root := t.TempDir()
	t.Setenv("BIGFRED_DATA_DIR", root)
	t.Setenv("DATA_DIR", "")

	path := filepath.Join(root, "etc", "microdns.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	const existing = `{"services":[],"dccBus":{"enabled":false}}`
	if err := os.WriteFile(path, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := EnsureMicrodnsConfig(); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != existing {
		t.Fatalf("existing config was overwritten:\n%s", raw)
	}
}
