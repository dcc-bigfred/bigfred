// Package datadir resolves the BigFred persistent data root.
//
// Priority: BIGFRED_DATA_DIR, then DATA_DIR, then /data (hub image default).
// Only absolute paths are accepted from the environment; relative values are
// ignored so misconfiguration cannot silently redirect data under the process
// working directory.
package datadir

import (
	"os"
	"path/filepath"
)

// Root returns the persistent data directory.
func Root() string {
	if v, ok := rootFromEnv("BIGFRED_DATA_DIR"); ok {
		return v
	}
	if v, ok := rootFromEnv("DATA_DIR"); ok {
		return v
	}
	return "/data"
}

func rootFromEnv(name string) (string, bool) {
	v := os.Getenv(name)
	if v == "" || !filepath.IsAbs(v) {
		return "", false
	}
	return v, true
}

// Path joins parts under Root().
func Path(parts ...string) string {
	return filepath.Join(append([]string{Root()}, parts...)...)
}
