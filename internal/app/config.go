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
	NATSAuthToken  string // optional auth token required by remote clients connecting to embedded NATS

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

	// SessionLifetimeSecs is the session cookie MaxAge in seconds (default 86400 = 1 day).
	SessionLifetimeSecs int // VVS_SESSION_LIFETIME env var

	// SecureCookie sets the Secure flag on the session cookie. Enable in production (HTTPS only).
	SecureCookie bool // VVS_SECURE_COOKIE env var

	// BaseURL is the public base URL used in generated links (e.g. portal access links).
	// Example: "https://isp.example.com". Defaults to http://host when empty.
	BaseURL string // VVS_BASE_URL env var

	// Debug enables verbose debug logging (slog DEBUG level).
	Debug bool
}

// SessionLifetime returns SessionLifetimeSecs, defaulting to 86400 when unset.
func (c Config) SessionLifetime() int {
	if c.SessionLifetimeSecs > 0 {
		return c.SessionLifetimeSecs
	}
	return 86400
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
