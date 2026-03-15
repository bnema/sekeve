package xdg

import (
	"fmt"
	"os"
	"path/filepath"
)

// Adapter implements port.XDGPort using the XDG Base Directory Specification.
type Adapter struct {
	appName string
}

// NewAdapter creates an XDG adapter for the given application name.
// The appName is appended to each XDG base directory.
func NewAdapter(appName string) *Adapter {
	return &Adapter{appName: appName}
}

// ConfigDir returns $XDG_CONFIG_HOME/<appName>, defaulting to ~/.config/<appName>.
func (a *Adapter) ConfigDir() (string, error) {
	return a.resolve("XDG_CONFIG_HOME", ".config")
}

// DataDir returns $XDG_DATA_HOME/<appName>, defaulting to ~/.local/share/<appName>.
func (a *Adapter) DataDir() (string, error) {
	return a.resolve("XDG_DATA_HOME", filepath.Join(".local", "share"))
}

// CacheDir returns $XDG_CACHE_HOME/<appName>, defaulting to ~/.cache/<appName>.
func (a *Adapter) CacheDir() (string, error) {
	return a.resolve("XDG_CACHE_HOME", ".cache")
}

// StateDir returns $XDG_STATE_HOME/<appName>, defaulting to ~/.local/state/<appName>.
func (a *Adapter) StateDir() (string, error) {
	return a.resolve("XDG_STATE_HOME", filepath.Join(".local", "state"))
}

// resolve checks the environment variable first; if set and absolute, uses it.
// Otherwise falls back to $HOME/<defaultRel>. Returns error if neither works.
func (a *Adapter) resolve(envVar, defaultRel string) (string, error) {
	if dir := os.Getenv(envVar); dir != "" {
		if !filepath.IsAbs(dir) {
			return "", fmt.Errorf("%s must be an absolute path, got %q", envVar, dir)
		}
		return filepath.Join(dir, a.appName), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}

	return filepath.Join(home, defaultRel, a.appName), nil
}
