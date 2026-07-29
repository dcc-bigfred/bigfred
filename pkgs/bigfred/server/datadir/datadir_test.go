package datadir

import (
	"path/filepath"
	"testing"
)

func TestRootEmptyEnvDefaultsToData(t *testing.T) {
	t.Setenv("BIGFRED_DATA_DIR", "")
	t.Setenv("DATA_DIR", "")
	if got := Root(); got != "/data" {
		t.Fatalf("Root() = %q, want /data", got)
	}
}

func TestRootBIGFREDDataDirTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BIGFRED_DATA_DIR", dir)
	t.Setenv("DATA_DIR", "/other")
	if got := Root(); got != dir {
		t.Fatalf("Root() = %q, want %q", got, dir)
	}
}

func TestRootDATADirFallback(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BIGFRED_DATA_DIR", "")
	t.Setenv("DATA_DIR", dir)
	if got := Root(); got != dir {
		t.Fatalf("Root() = %q, want %q", got, dir)
	}
}

func TestPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BIGFRED_DATA_DIR", dir)
	want := filepath.Join(dir, "etc", "loco-server.conf")
	if got := Path("etc", "loco-server.conf"); got != want {
		t.Fatalf("Path(...) = %q, want %q", got, want)
	}
}

func TestRootIgnoresRelativeEnv(t *testing.T) {
	t.Setenv("BIGFRED_DATA_DIR", "relative/path")
	t.Setenv("DATA_DIR", "")
	if got := Root(); got != "/data" {
		t.Fatalf("Root() = %q, want /data", got)
	}
}
