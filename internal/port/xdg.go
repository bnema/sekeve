package port

// XDGPort provides XDG Base Directory paths for the application.
// Implementations must respect the XDG Base Directory Specification:
// - Use the corresponding XDG environment variable if set and absolute
// - Fall back to the spec-defined default otherwise
// - Return an error if the resolved path cannot be determined
type XDGPort interface {
	// ConfigDir returns the XDG config directory for the application.
	// ($XDG_CONFIG_HOME/<appname>, default ~/.config/<appname>)
	ConfigDir() (string, error)

	// DataDir returns the XDG data directory for the application.
	// ($XDG_DATA_HOME/<appname>, default ~/.local/share/<appname>)
	DataDir() (string, error)

	// CacheDir returns the XDG cache directory for the application.
	// ($XDG_CACHE_HOME/<appname>, default ~/.cache/<appname>)
	CacheDir() (string, error)

	// StateDir returns the XDG state directory for the application.
	// ($XDG_STATE_HOME/<appname>, default ~/.local/state/<appname>)
	StateDir() (string, error)
}
