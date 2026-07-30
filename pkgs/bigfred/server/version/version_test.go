package version

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGet_defaultsWithoutSection(t *testing.T) {
	info := Get()
	require.Equal(t, "dev", info.Version)
	require.Empty(t, info.TagCommit)
	// buildCommit / buildTime may be empty in unit tests (no ldflags).
}

func TestReadSectionFrom_objcopyRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("objcopy"); err != nil {
		t.Skip("objcopy not on PATH")
	}

	src, err := os.Executable()
	require.NoError(t, err)

	dir := t.TempDir()
	dst := filepath.Join(dir, "binary")
	data, err := os.ReadFile(src)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(dst, data, 0o755))

	payload, err := json.Marshal(sectionPayload{Version: "v1.2.3", Commit: "abc1234"})
	require.NoError(t, err)
	sectionFile := filepath.Join(dir, "section.json")
	require.NoError(t, os.WriteFile(sectionFile, payload, 0o644))

	// Remove any existing section (no-op if absent), then add.
	_ = exec.Command("objcopy", "--remove-section", sectionName, dst).Run()
	out, err := exec.Command("objcopy", "--add-section", sectionName+"="+sectionFile, dst).CombinedOutput()
	require.NoError(t, err, string(out))

	v, c, ok := readSectionFrom(dst)
	require.True(t, ok)
	require.Equal(t, "v1.2.3", v)
	require.Equal(t, "abc1234", c)

	// Re-tag: replace with a different version.
	payload2, err := json.Marshal(sectionPayload{Version: "v9.9.9", Commit: "deadbeef"})
	require.NoError(t, err)
	sectionFile2 := filepath.Join(dir, "section2.json")
	require.NoError(t, os.WriteFile(sectionFile2, payload2, 0o644))
	_ = exec.Command("objcopy", "--remove-section", sectionName, dst).Run()
	out, err = exec.Command("objcopy", "--add-section", sectionName+"="+sectionFile2, dst).CombinedOutput()
	require.NoError(t, err, string(out))

	v, c, ok = readSectionFrom(dst)
	require.True(t, ok)
	require.Equal(t, "v9.9.9", v)
	require.Equal(t, "deadbeef", c)
}

func TestReadSectionFrom_missing(t *testing.T) {
	src, err := os.Executable()
	require.NoError(t, err)
	_, _, ok := readSectionFrom(src)
	// The test binary itself should not carry the release section.
	require.False(t, ok)
}

func TestString(t *testing.T) {
	s := String()
	require.Contains(t, s, "dev")
}
