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

func TestResolveRelativeUnderRoot(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BIGFRED_DATA_DIR", dir)
	t.Setenv("DATA_DIR", "")
	want := filepath.Join(dir, "bigfred.db")
	if got := Resolve("bigfred.db"); got != want {
		t.Fatalf("Resolve(relative) = %q, want %q", got, want)
	}
}

func TestResolveAbsoluteUnchanged(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BIGFRED_DATA_DIR", dir)
	abs := filepath.Join(dir, "elsewhere", "custom.db")
	if got := Resolve(abs); got != filepath.Clean(abs) {
		t.Fatalf("Resolve(abs) = %q, want %q", got, filepath.Clean(abs))
	}
}
