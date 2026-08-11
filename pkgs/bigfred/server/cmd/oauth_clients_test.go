package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"
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

func TestShouldReloadOnOAuthClientEvent(t *testing.T) {
	t.Parallel()
	jsonPath := filepath.Join("tmp", "oauth-clients", "wizard.json")
	cases := []struct {
		name string
		ev   fsnotify.Event
		want bool
	}{
		{"json write reloads", fsnotify.Event{Name: jsonPath, Op: fsnotify.Write}, true},
		{"json create reloads", fsnotify.Event{Name: jsonPath, Op: fsnotify.Create}, true},
		{"json remove reloads", fsnotify.Event{Name: jsonPath, Op: fsnotify.Remove}, true},
		{"json rename reloads", fsnotify.Event{Name: jsonPath, Op: fsnotify.Rename}, true},
		{"txt write skipped", fsnotify.Event{Name: "README.txt", Op: fsnotify.Write}, false},
		{"swap file skipped", fsnotify.Event{Name: ".wizard.json.swp", Op: fsnotify.Write}, false},
		{"bak file skipped", fsnotify.Event{Name: "wizard.json.bak", Op: fsnotify.Write}, false},
		{"dir event reloads", fsnotify.Event{Name: "", Op: fsnotify.Remove}, true},
		{"chmod only skipped", fsnotify.Event{Name: jsonPath, Op: fsnotify.Chmod}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldReloadOnOAuthClientEvent(tc.ev); got != tc.want {
				t.Fatalf("shouldReloadOnOAuthClientEvent(%+v) = %v, want %v", tc.ev, got, tc.want)
			}
		})
	}
}
