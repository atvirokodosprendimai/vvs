package app

import "strings"

type Config struct {
	DatabasePath  string
	ListenAddr    string
	AdminUser     string
	AdminPassword string
	NetBoxURL   string // optional; NETBOX_URL env var
	NetBoxToken string // optional; NETBOX_TOKEN env var

	// NATS options — mutually exclusive
	NATSUrl        string // if set, connect to external NATS instead of starting embedded
	NATSListenAddr string // if set (and NATSUrl empty), embedded NATS exposes TCP on this addr

	// EmailEncKey is 32 bytes (hex or raw) used to AES-256-GCM encrypt IMAP passwords.
	// Empty = dev mode (passwords stored in plaintext — not for production).
	EmailEncKey string // VVS_EMAIL_ENC_KEY env var

	// APIToken is the bearer token required for /api/v1/* requests.
	// Empty string disables the REST API.
	APIToken string // VVS_API_TOKEN env var

	// EnabledModules lists which modules to start. Empty = all enabled.
	EnabledModules []string // e.g. ["auth","customer"]

	// Debug enables verbose debug logging (slog DEBUG level).
	Debug bool
}

// IsEnabled reports whether module name is enabled.
// Returns true when EnabledModules is empty (all enabled) or name is in the list.
func (c Config) IsEnabled(name string) bool {
	if len(c.EnabledModules) == 0 {
		return true
	}
	for _, m := range c.EnabledModules {
		if strings.EqualFold(m, name) {
			return true
		}
	}
	return false
}
