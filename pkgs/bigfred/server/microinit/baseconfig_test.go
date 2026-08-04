package microinit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBaseConfigServiceNames(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "microinit.json")
	if err := os.WriteFile(path, []byte(`{
  "services": [
    {"name": "redis", "cmd": "/etc/init.d/redis"},
    {"name": "alloy", "cmd": "/etc/init.d/alloy"},
    {"name": ""}
  ]
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	names, err := BaseConfigServiceNames(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := names["redis"]; !ok {
		t.Fatal("expected redis")
	}
	if _, ok := names["alloy"]; !ok {
		t.Fatal("expected alloy")
	}
	if len(names) != 2 {
		t.Fatalf("len=%d want 2", len(names))
	}
}

func TestBaseConfigServiceNamesMissingFile(t *testing.T) {
	names, err := BaseConfigServiceNames(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 0 {
		t.Fatalf("len=%d", len(names))
	}
}
