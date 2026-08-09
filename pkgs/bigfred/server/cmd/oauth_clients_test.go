package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOAuthClientsRegistryLoadAndCors(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	write("wizard.json", `{
		"clientId": "bigfred-wizard",
		"clientSecret": "secret",
		"redirectUris": ["http://localhost:8091/auth/callback"],
		"corsEnabled": false,
		"corsOrigins": ["http://localhost:8091"],
		"enabled": true
	}`)
	write("other.json", `{
		"clientId": "other-app",
		"clientSecret": "x",
		"redirectUris": ["http://localhost:3000/cb"],
		"corsEnabled": true,
		"corsOrigins": ["http://localhost:3000"],
		"enabled": true
	}`)
	write("disabled.json", `{
		"clientId": "disabled",
		"clientSecret": "x",
		"redirectUris": ["http://x/cb"],
		"enabled": false
	}`)

	reg, err := NewOAuthClientsRegistry(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reg.Get("disabled"); ok {
		t.Fatal("disabled client must be absent")
	}
	c, ok := reg.Get("bigfred-wizard")
	if !ok {
		t.Fatal("wizard client missing")
	}
	if !c.RedirectURIAllowed("http://localhost:8091/auth/callback") {
		t.Fatal("redirect URI should match")
	}
	if c.RedirectURIAllowed("http://evil/") {
		t.Fatal("evil redirect must fail")
	}
	origins := reg.CorsOrigins()
	if len(origins) != 1 || origins[0] != "http://localhost:3000" {
		t.Fatalf("cors origins = %#v, want only other-app", origins)
	}
}
