// Package otelenv loads optional OpenTelemetry settings from
// $DATA_DIR/etc/otel.env into the process environment.
//
// Existing process environment variables win: the file only sets keys
// that are not already present. A missing file is a no-op.
package otelenv

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/keskad/loco/pkgs/bigfred/server/datadir"
)

// DefaultPath is the live dotenv under the persistent data root.
func DefaultPath() string { return datadir.Path("etc", "otel.env") }

// DefaultsReferencePath is rewritten on loco-server start with commented
// example keys. It is not read at runtime.
func DefaultsReferencePath() string { return datadir.Path("etc", "otel.env.defaults") }

// Load reads path as KEY=value dotenv and sets missing process env vars.
// Missing path returns nil. Malformed lines are skipped.
func Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read %s: %w", path, err)
	}
	for k, v := range Parse(string(data)) {
		if _, ok := os.LookupEnv(k); ok {
			continue
		}
		if err := os.Setenv(k, v); err != nil {
			return fmt.Errorf("set %s from %s: %w", k, path, err)
		}
	}
	return nil
}

// Parse reads KEY=value lines (dotenv). Comments (#) and blank lines are ignored.
// Keys are uppercased. Returns a map of key → value (last wins on duplicates).
func Parse(text string) map[string]string {
	out := make(map[string]string)
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.ToUpper(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" {
			continue
		}
		out[key] = value
	}
	return out
}

// WriteDefaultsReference writes a commented example file for operators.
func WriteDefaultsReference(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(defaultsReference), 0o644)
}

// EnvBool reports whether name is a truthy env value (1/true/yes/on).
func EnvBool(name string) (value bool, set bool) {
	v, ok := os.LookupEnv(name)
	if !ok {
		return false, false
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off", "":
		return false, true
	default:
		return false, true
	}
}

const defaultsReference = `# otel.env.defaults — OpenTelemetry settings reference (not read at runtime)
# Auto-generated on loco-server start. Copy keys into otel.env to enable.
# Process environment wins over otel.env. CLI flags override both.
#
# Path: $BIGFRED_DATA_DIR/etc/otel.env (or $DATA_DIR/etc/otel.env)

# Copy to otel.env and uncomment to enable.
# ENABLE_TELEMETRY=false
# OTEL_EXPORTER_OTLP_ENDPOINT=127.0.0.1:4317
# # microinit HTTP sibling (Alloy listens on :4318 when telemetry enabled):
# # OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318
# OTEL_EXPORTER_OTLP_PROTOCOL=http
# OTEL_SERVICE_NAME=microinit
# OTEL_METRIC_EXPORT_INTERVAL=15000
# OTEL_EXPORTER_OTLP_HEADERS=
`
