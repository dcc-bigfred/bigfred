//go:build !prod

package web

import "io/fs"

// Dist reports that no frontend bundle is embedded. Development builds
// (no `prod` build tag) deliberately skip the go:embed so `web/dist`
// need not exist; serve the SPA via the Vite dev server (`make web-dev`)
// instead.
func Dist() (fs.FS, bool) {
	return nil, false
}
