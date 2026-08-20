package microinit

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/dcc-bigfred/microinit/go/config"

	"github.com/keskad/loco/pkgs/bigfred/platform"
	"github.com/keskad/loco/pkgs/bigfred/server/datadir"
)

// defaultMicrodnsConfig is the seed for $DATA_DIR/etc/microdns.json when the
// file is absent. On BigFred OS the image overlay overwrites this file every
// boot; the static bigfred row here is a fallback for non-OS / supervise hosts
// that do not watch microinit labels.
const defaultMicrodnsConfig = `{
  "services": [
    { "name": "bigfred", "type": "_http._tcp", "protocol": "tcp", "port": 8080, "host": "bigfred", "txt": { "path": "/" } }
  ],
  "bigfred": { "enabled": true },
  "microinit": { "enabled": true },
  "dccBus": { "beacon": true },
  "retry": { "bigfredMs": 45000, "pollMs": 25000, "mdnsMs": 3000, "ifaceMs": 5000, "microinitReconnectMs": 3000 }
}
`

// EnsureMicrodnsConfig writes $DATA_DIR/etc/microdns.json with the default
// template only when the file does not already exist. Existing operator
// edits are left untouched. No-op on Android (phone builds do not run microdns).
func EnsureMicrodnsConfig() error {
	if !platform.SupportsMicrodns() {
		return nil
	}
	return ensureMicrodnsConfigFile(datadir.Path("etc", "microdns.json"))
}

func ensureMicrodnsConfigFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if !json.Valid([]byte(defaultMicrodnsConfig)) {
		return errors.New("microdns: default config is not valid JSON")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return config.WriteFileAtomically(path, []byte(defaultMicrodnsConfig))
}
