package app

type Config struct {
	DatabasePath  string
	ListenAddr    string
	AdminUser     string
	AdminPassword string
	NetBoxURL     string // optional; NETBOX_URL env var
	NetBoxToken   string // optional; NETBOX_TOKEN env var
}
