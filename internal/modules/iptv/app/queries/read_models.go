package queries

import "time"

// ChannelReadModel is the flattened read model for channel list/detail views.
type ChannelReadModel struct {
	ID        string
	Name      string
	Slug      string
	LogoURL   string
	StreamURL string
	Category  string
	EPGSource string
	Active    bool
	CreatedAt time.Time
}

// PackageReadModel is the flattened read model for package list/detail views.
type PackageReadModel struct {
	ID           string
	Name         string
	PriceCents   int64
	Description  string
	ChannelCount int
	SubCount     int
	CreatedAt    time.Time
}

// SubscriptionReadModel is the flattened read model for subscription list views.
type SubscriptionReadModel struct {
	ID          string
	CustomerID  string
	PackageID   string
	PackageName string
	Status      string
	StartsAt    time.Time
	EndsAt      *time.Time
	CreatedAt   time.Time
}

// STBReadModel is the flattened read model for STB list views.
type STBReadModel struct {
	ID           string
	MAC          string
	Model        string
	CustomerID   string
	CustomerName string
	AssignedAt   time.Time
	Notes        string
}

// SubscriptionKeyReadModel is the flattened read model for key management.
type SubscriptionKeyReadModel struct {
	ID             string
	SubscriptionID string
	CustomerID     string
	Token          string
	CreatedAt      time.Time
	RevokedAt      *time.Time
}

// ChannelProviderReadModel is the flattened read model for provider list/detail views.
type ChannelProviderReadModel struct {
	ID          string
	ChannelID   string
	Name        string
	URLTemplate string
	Token       string
	Type        string
	Priority    int
	Active      bool
	CreatedAt   time.Time
}

// IPTVStackReadModel is the flattened read model for IPTV stack list/detail views.
type IPTVStackReadModel struct {
	ID                 string
	Name               string
	ClusterID          string
	NodeID             string
	WANNetworkID       string
	OverlayNetworkID   string
	WANNetworkName     string
	OverlayNetworkName string
	WanIP              string
	WANInterface       string
	Status             string
	LastDeployedAt     *time.Time
	ChannelCount       int
	CreatedAt          time.Time
}

// IPTVStackChannelReadModel is one channel+provider assignment within a stack.
type IPTVStackChannelReadModel struct {
	ID           string
	StackID      string
	ChannelID    string
	ChannelName  string
	ChannelSlug  string
	ProviderID   string
	ProviderName string
	ProviderType string
}
