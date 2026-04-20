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
	NATSAuthToken  string // deprecated: use NATSCorePassword + NATSPortalPassword instead

	// Per-user NATS credentials (preferred over NATSAuthToken).
	// When both are set, embedded NATS uses per-user permissions:
	//   core   → full access (">")
	//   portal → isp.portal.rpc.> and _INBOX.> only
	NATSCorePassword   string // VVS_NATS_CORE_PASSWORD env var
	NATSPortalPassword string // VVS_NATS_PORTAL_PASSWORD env var

	// EmailEncKey is 32 bytes (hex or raw) used to AES-256-GCM encrypt IMAP passwords.
	// Empty = dev mode (passwords stored in plaintext — not for production).
	EmailEncKey string // VVS_EMAIL_ENC_KEY env var

	// RouterEncKey is 32 bytes (hex or raw) used to AES-256-GCM encrypt router passwords.
	// Empty = dev mode (passwords stored in plaintext — not for production).
	RouterEncKey string // VVS_ROUTER_ENC_KEY env var

	// ProxmoxEncKey is 32 bytes (hex or raw) used to AES-256-GCM encrypt Proxmox node token secrets.
	// Empty = dev mode (tokens stored in plaintext — not for production).
	ProxmoxEncKey string // VVS_PROXMOX_ENC_KEY env var

	// Stripe keys — required for portal VM purchase and balance top-up flows.
	// Empty = Stripe integration disabled (portal checkout endpoints return 503).
	StripeSecretKey      string // VVS_STRIPE_SECRET_KEY env var
	StripeWebhookSecret  string // VVS_STRIPE_WEBHOOK_SECRET env var
	StripePublishableKey string // VVS_STRIPE_PUBLISHABLE_KEY env var

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

	// MetricsAddr is the address for the Prometheus /metrics endpoint (e.g. ":9091").
	// When empty, the metrics server is not started.
	MetricsAddr string // VVS_METRICS_ADDR env var

	// OllamaURL is the base URL for the Ollama API used by the portal chat bot.
	// Defaults to http://localhost:11434 when empty.
	OllamaURL string // VVS_OLLAMA_URL env var

	// BotModel is the Ollama model name for the portal chat bot (default: llama3.2).
	BotModel string // VVS_BOT_MODEL env var

	// DemoMode disables risky cron job types (shell, url) for public demo environments.
	// When true: shell and url job types are rejected at the HTTP layer and skipped at execution.
	DemoMode bool // VVS_DEMO_MODE env var

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
