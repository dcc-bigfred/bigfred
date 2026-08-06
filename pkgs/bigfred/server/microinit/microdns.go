package microinit

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/dcc-bigfred/microinit/go/config"

	"github.com/keskad/loco/pkgs/bigfred/platform"
	"github.com/keskad/loco/pkgs/bigfred/server/datadir"
)

// defaultMicrodnsConfig is the seed for $DATA_DIR/etc/microdns.json when the
// file is absent. microinit / microdns owns LAN advertisement; loco-server
// only ensures a usable default exists.
const defaultMicrodnsConfig = `{
  "services": [
    { "name": "bigfred", "type": "_http._tcp", "protocol": "tcp", "port": 8080, "host": "bigfred", "txt": { "path": "/" } }
  ],
  "dccBus": { "enabled": true, "z21Port": 21105, "withrottlePort": 12090, "beacon": true },
  "retry": { "microinitMs": 2000, "procMs": 2000, "mdnsMs": 3000, "ifaceMs": 5000 }
}
`

// EnsureMicrodnsConfig writes $DATA_DIR/etc/microdns.json with the default
// template only when the file does not already exist. Existing operator
// edits are left untouched. No-op on Android (phone builds do not run microdns).
func EnsureMicrodnsConfig() error {
	if !platform.SupportsMicrodns() {
		return nil
	}
	path := datadir.Path("etc", "microdns.json")
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// Validate the default so a typo fails tests / startup seed, not microdns.
	if !json.Valid([]byte(defaultMicrodnsConfig)) {
		return errors.New("microdns: default config is not valid JSON")
	}
	return config.WriteFileAtomically(path, []byte(defaultMicrodnsConfig))
}
