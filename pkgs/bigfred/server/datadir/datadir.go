// Package datadir resolves the BigFred persistent data root.
//
// Priority: BIGFRED_DATA_DIR, then DATA_DIR, then /data (hub image default).
package datadir

import (
	"os"
	"path/filepath"
)

// Root returns the persistent data directory.
func Root() string {
	if v := os.Getenv("BIGFRED_DATA_DIR"); v != "" {
		return v
	}
	if v := os.Getenv("DATA_DIR"); v != "" {
		return v
	}
	return "/data"
}

// Path joins parts under Root().
func Path(parts ...string) string {
	return filepath.Join(append([]string{Root()}, parts...)...)
}
