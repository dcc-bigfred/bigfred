//go:build prod

// Package web carries the production frontend bundle. With `-tags prod`
// the compiled SPA in `web/dist` is embedded into the loco-server binary
// (see §7b.1) so a single binary serves both the API and the UI at "/".
//
// `web/dist` must exist at build time — run `make web-build` first. The
// Makefile's `build-prod` / `run-prod` targets enforce this ordering.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// Dist returns the embedded production frontend (the contents of
// web/dist, with the leading "dist/" stripped) and true. It is only
// compiled into `-tags prod` builds.
func Dist() (fs.FS, bool) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil, false
	}
	return sub, true
}
