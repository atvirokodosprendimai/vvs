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

	// RouterEncKey is 32 bytes (hex or raw) used to AES-256-GCM encrypt router passwords.
	// Empty = dev mode (passwords stored in plaintext — not for production).
	RouterEncKey string // VVS_ROUTER_ENC_KEY env var

	// APIToken is the bearer token required for /api/v1/* requests.
	// Empty string disables the REST API.
	APIToken string // VVS_API_TOKEN env var

	// EnabledModules lists which modules to start. Empty = all enabled.
	EnabledModules []string // e.g. ["auth","customer"]

	// EmailSyncIntervalSecs is the email sync polling interval in seconds (default 300).
	EmailSyncIntervalSecs int // VVS_EMAIL_SYNC_INTERVAL env var

	// EmailPageSize is the number of email threads per page (default 50).
	EmailPageSize int // VVS_EMAIL_PAGE_SIZE env var

	// DefaultVATRate is the default VAT percentage for new invoice line items (default 21).
	DefaultVATRate int // VVS_DEFAULT_VAT_RATE env var

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
