package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/server/datadir"
)

// OAuthClient is one drop-in OAuth client registration.
type OAuthClient struct {
	ClientID     string   `json:"clientId"`
	ClientSecret string   `json:"clientSecret"`
	DisplayName  string   `json:"displayName"`
	RedirectURIs []string `json:"redirectUris"`
	CorsEnabled  bool     `json:"corsEnabled"`
	CorsOrigins  []string `json:"corsOrigins"`
	Enabled      bool     `json:"enabled"`
}

// OAuthClientsRegistry loads `$DATA_DIR/etc/bigfred/oauth-clients/*.json`.
type OAuthClientsRegistry struct {
	dir  string
	log  *logrus.Logger
	mu   sync.RWMutex
	byID map[string]OAuthClient
}

// DefaultOAuthClientsDir is the drop-in directory under DATA_DIR.
func DefaultOAuthClientsDir() string {
	return datadir.Path("etc", "bigfred", "oauth-clients")
}

// NewOAuthClientsRegistry loads clients from dir (created if missing).
func NewOAuthClientsRegistry(dir string, log *logrus.Logger) (*OAuthClientsRegistry, error) {
	if dir == "" {
		dir = DefaultOAuthClientsDir()
	}
	if log == nil {
		log = logrus.New()
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	r := &OAuthClientsRegistry{dir: dir, log: log, byID: map[string]OAuthClient{}}
	if err := r.reload(); err != nil {
		return nil, err
	}
	return r, nil
}

// StartWatch watches the drop-in directory and reloads on change.
func (r *OAuthClientsRegistry) StartWatch(stop <-chan struct{}) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err := w.Add(r.dir); err != nil {
		_ = w.Close()
		return err
	}
	go func() {
		defer w.Close()
		debounce := time.NewTimer(0)
		if !debounce.Stop() {
			<-debounce.C
		}
		for {
			select {
			case <-stop:
				return
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
					continue
				}
				if !strings.HasSuffix(strings.ToLower(ev.Name), ".json") &&
					filepath.Ext(ev.Name) != "" {
					// still reload on dir events that may affect json files
				}
				debounce.Reset(300 * time.Millisecond)
			case <-debounce.C:
				if err := r.reload(); err != nil {
					r.log.WithError(err).Warn("oauth-clients: reload failed")
				}
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				r.log.WithError(err).Warn("oauth-clients: watch error")
			}
		}
	}()
	return nil
}

func (r *OAuthClientsRegistry) reload() error {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return err
	}
	next := make(map[string]OAuthClient)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		path := filepath.Join(r.dir, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			r.log.WithError(err).WithField("path", path).Warn("oauth-clients: skip unreadable")
			continue
		}
		var c OAuthClient
		if err := json.Unmarshal(raw, &c); err != nil {
			r.log.WithError(err).WithField("path", path).Warn("oauth-clients: skip invalid JSON")
			continue
		}
		c.ClientID = strings.TrimSpace(c.ClientID)
		if c.ClientID == "" {
			r.log.WithField("path", path).Warn("oauth-clients: skip missing clientId")
			continue
		}
		if !c.Enabled {
			continue
		}
		next[c.ClientID] = c
	}
	r.mu.Lock()
	r.byID = next
	r.mu.Unlock()
	r.log.WithField("count", len(next)).Debug("oauth-clients: reloaded")
	return nil
}

// Get returns an enabled client by id.
func (r *OAuthClientsRegistry) Get(clientID string) (OAuthClient, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.byID[strings.TrimSpace(clientID)]
	return c, ok
}

// CorsOrigins returns origins from clients with corsEnabled=true.
func (r *OAuthClientsRegistry) CorsOrigins() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []string
	seen := map[string]struct{}{}
	for _, c := range r.byID {
		if !c.CorsEnabled {
			continue
		}
		for _, o := range c.CorsOrigins {
			o = strings.TrimSpace(o)
			if o == "" {
				continue
			}
			if _, ok := seen[o]; ok {
				continue
			}
			seen[o] = struct{}{}
			out = append(out, o)
		}
	}
	return out
}

// RedirectURIAllowed reports exact-match allowlist membership.
func (c OAuthClient) RedirectURIAllowed(uri string) bool {
	uri = strings.TrimSpace(uri)
	for _, u := range c.RedirectURIs {
		if strings.TrimSpace(u) == uri {
			return true
		}
	}
	return false
}