package persistence

import (
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/modules/iptv/domain"
)

// ── EPGProgramme ──────────────────────────────────────────────────────────────

type EPGProgrammeModel struct {
	ID           string `gorm:"primaryKey;type:text"`
	ChannelEPGID string `gorm:"column:channel_epg_id;type:text;not null;index"`
	Title        string `gorm:"type:text;not null;default:''"`
	Description  string `gorm:"type:text;not null;default:''"`
	StartTime    int64  `gorm:"column:start_time;not null;index"`
	StopTime     int64  `gorm:"column:stop_time;not null;index"`
	Category     string `gorm:"type:text;not null;default:''"`
	Rating       string `gorm:"type:text;not null;default:''"`
}

func (EPGProgrammeModel) TableName() string { return "iptv_epg_programmes" }

func toEPGModel(p *domain.EPGProgramme) EPGProgrammeModel {
	return EPGProgrammeModel{
		ID:           p.ID,
		ChannelEPGID: p.ChannelEPGID,
		Title:        p.Title,
		Description:  p.Description,
		StartTime:    p.StartTime.Unix(),
		StopTime:     p.StopTime.Unix(),
		Category:     p.Category,
		Rating:       p.Rating,
	}
}

func (m *EPGProgrammeModel) toDomain() *domain.EPGProgramme {
	return &domain.EPGProgramme{
		ID:           m.ID,
		ChannelEPGID: m.ChannelEPGID,
		Title:        m.Title,
		Description:  m.Description,
		StartTime:    time.Unix(m.StartTime, 0).UTC(),
		StopTime:     time.Unix(m.StopTime, 0).UTC(),
		Category:     m.Category,
		Rating:       m.Rating,
	}
}

// ── Channel ───────────────────────────────────────────────────────────────────

type ChannelModel struct {
	ID        string    `gorm:"primaryKey;type:text"`
	Name      string    `gorm:"type:text;not null"`
	Slug      string    `gorm:"type:text;not null;default:''"`
	LogoURL   string    `gorm:"type:text;not null;default:''"`
	StreamURL string    `gorm:"type:text;not null;default:''"`
	DVRUrl    string    `gorm:"column:dvr_url;type:text;not null;default:''"`
	Category  string    `gorm:"type:text;not null;default:''"`
	EPGSource string    `gorm:"type:text;not null;default:''"`
	Active    bool      `gorm:"not null;default:1"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (ChannelModel) TableName() string { return "iptv_channels" }

func toChannelModel(c *domain.Channel) ChannelModel {
	return ChannelModel{
		ID: c.ID, Name: c.Name, Slug: c.Slug, LogoURL: c.LogoURL, StreamURL: c.StreamURL,
		DVRUrl: c.DVRUrl, Category: c.Category, EPGSource: c.EPGSource, Active: c.Active,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}

func (m *ChannelModel) toDomain() *domain.Channel {
	return &domain.Channel{
		ID: m.ID, Name: m.Name, Slug: m.Slug, LogoURL: m.LogoURL, StreamURL: m.StreamURL,
		DVRUrl: m.DVRUrl, Category: m.Category, EPGSource: m.EPGSource, Active: m.Active,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

// ── ChannelProvider ───────────────────────────────────────────────────────────

type ChannelProviderModel struct {
	ID          string    `gorm:"primaryKey;type:text"`
	ChannelID   string    `gorm:"column:channel_id;type:text;not null;index"`
	Name        string    `gorm:"type:text;not null;default:''"`
	URLTemplate string    `gorm:"column:url_template;type:text;not null;default:''"`
	Token       string    `gorm:"type:text;not null;default:''"`
	Type        string    `gorm:"type:text;not null;default:'external'"`
	Priority    int       `gorm:"not null;default:0"`
	Active      bool      `gorm:"not null;default:1"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (ChannelProviderModel) TableName() string { return "iptv_channel_providers" }

func toChannelProviderModel(p *domain.ChannelProvider) ChannelProviderModel {
	return ChannelProviderModel{
		ID: p.ID, ChannelID: p.ChannelID, Name: p.Name,
		URLTemplate: p.URLTemplate, Token: p.Token, Type: string(p.Type),
		Priority: p.Priority, Active: p.Active,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func (m *ChannelProviderModel) toDomain() *domain.ChannelProvider {
	return &domain.ChannelProvider{
		ID: m.ID, ChannelID: m.ChannelID, Name: m.Name,
		URLTemplate: m.URLTemplate, Token: m.Token, Type: domain.ProviderType(m.Type),
		Priority: m.Priority, Active: m.Active,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

// ── IPTVStack ─────────────────────────────────────────────────────────────────

type IPTVStackModel struct {
	ID                 string     `gorm:"primaryKey;type:text"`
	Name               string     `gorm:"type:text;not null;default:''"`
	ClusterID          string     `gorm:"column:cluster_id;type:text;not null;default:''"`
	NodeID             string     `gorm:"column:node_id;type:text;not null;default:''"`
	WANNetworkID       string     `gorm:"column:wan_network_id;type:text;not null;default:''"`
	OverlayNetworkID   string     `gorm:"column:overlay_network_id;type:text;not null;default:''"`
	WANNetworkName     string     `gorm:"column:wan_network_name;type:text;not null;default:''"`
	OverlayNetworkName string     `gorm:"column:overlay_network_name;type:text;not null;default:''"`
	WanIP              string     `gorm:"column:wan_ip;type:text;not null;default:''"`
	Status             string     `gorm:"type:text;not null;default:'pending'"`
	LastDeployedAt     *time.Time `gorm:"column:last_deployed_at;type:datetime"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (IPTVStackModel) TableName() string { return "iptv_stacks" }

func toIPTVStackModel(s *domain.IPTVStack) IPTVStackModel {
	return IPTVStackModel{
		ID: s.ID, Name: s.Name, ClusterID: s.ClusterID, NodeID: s.NodeID,
		WANNetworkID: s.WANNetworkID, OverlayNetworkID: s.OverlayNetworkID,
		WANNetworkName: s.WANNetworkName, OverlayNetworkName: s.OverlayNetworkName,
		WanIP: s.WanIP, Status: string(s.Status),
		LastDeployedAt: s.LastDeployedAt, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

func (m *IPTVStackModel) toDomain() *domain.IPTVStack {
	return &domain.IPTVStack{
		ID: m.ID, Name: m.Name, ClusterID: m.ClusterID, NodeID: m.NodeID,
		WANNetworkID: m.WANNetworkID, OverlayNetworkID: m.OverlayNetworkID,
		WANNetworkName: m.WANNetworkName, OverlayNetworkName: m.OverlayNetworkName,
		WanIP: m.WanIP, Status: domain.IPTVStackStatus(m.Status),
		LastDeployedAt: m.LastDeployedAt, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

// ── IPTVStackChannel ──────────────────────────────────────────────────────────

type IPTVStackChannelModel struct {
	ID         string `gorm:"primaryKey;type:text"`
	StackID    string `gorm:"column:stack_id;type:text;not null;index"`
	ChannelID  string `gorm:"column:channel_id;type:text;not null"`
	ProviderID string `gorm:"column:provider_id;type:text;not null;default:''"`
}

func (IPTVStackChannelModel) TableName() string { return "iptv_stack_channels" }

func toIPTVStackChannelModel(sc *domain.IPTVStackChannel) IPTVStackChannelModel {
	return IPTVStackChannelModel{
		ID: sc.ID, StackID: sc.StackID, ChannelID: sc.ChannelID, ProviderID: sc.ProviderID,
	}
}

func (m *IPTVStackChannelModel) toDomain() *domain.IPTVStackChannel {
	return &domain.IPTVStackChannel{
		ID: m.ID, StackID: m.StackID, ChannelID: m.ChannelID, ProviderID: m.ProviderID,
	}
}

// ── Package ───────────────────────────────────────────────────────────────────

type PackageModel struct {
	ID          string    `gorm:"primaryKey;type:text"`
	Name        string    `gorm:"type:text;not null"`
	PriceCents  int64     `gorm:"not null;default:0"`
	Description string    `gorm:"type:text;not null;default:''"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (PackageModel) TableName() string { return "iptv_packages" }

func toPackageModel(p *domain.Package) PackageModel {
	return PackageModel{
		ID: p.ID, Name: p.Name, PriceCents: p.PriceCents,
		Description: p.Description, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func (m *PackageModel) toDomain() *domain.Package {
	return &domain.Package{
		ID: m.ID, Name: m.Name, PriceCents: m.PriceCents,
		Description: m.Description, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

// ── Package-Channel join ──────────────────────────────────────────────────────

type PackageChannelModel struct {
	PackageID string `gorm:"primaryKey;type:text"`
	ChannelID string `gorm:"primaryKey;type:text"`
	Position  int    `gorm:"not null;default:0"`
}

func (PackageChannelModel) TableName() string { return "iptv_package_channels" }

// ── Subscription ──────────────────────────────────────────────────────────────

type SubscriptionModel struct {
	ID         string     `gorm:"primaryKey;type:text"`
	CustomerID string     `gorm:"type:text;not null"`
	PackageID  string     `gorm:"type:text;not null"`
	Status     string     `gorm:"type:text;not null;default:'active'"`
	StartsAt   time.Time  `gorm:"not null"`
	EndsAt     *time.Time `gorm:"type:datetime"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (SubscriptionModel) TableName() string { return "iptv_subscriptions" }

func toSubscriptionModel(s *domain.Subscription) SubscriptionModel {
	return SubscriptionModel{
		ID: s.ID, CustomerID: s.CustomerID, PackageID: s.PackageID,
		Status: s.Status, StartsAt: s.StartsAt, EndsAt: s.EndsAt,
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

func (m *SubscriptionModel) toDomain() *domain.Subscription {
	return &domain.Subscription{
		ID: m.ID, CustomerID: m.CustomerID, PackageID: m.PackageID,
		Status: m.Status, StartsAt: m.StartsAt, EndsAt: m.EndsAt,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

// ── STB ───────────────────────────────────────────────────────────────────────

type STBModel struct {
	ID         string    `gorm:"primaryKey;type:text"`
	MAC        string    `gorm:"type:text;not null;uniqueIndex"`
	Model      string    `gorm:"type:text;not null;default:''"`
	Firmware   string    `gorm:"type:text;not null;default:''"`
	Serial     string    `gorm:"type:text;not null;default:''"`
	CustomerID string    `gorm:"type:text;not null"`
	AssignedAt time.Time `gorm:"not null"`
	Notes      string    `gorm:"type:text;not null;default:''"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (STBModel) TableName() string { return "iptv_stbs" }

func toSTBModel(s *domain.STB) STBModel {
	return STBModel{
		ID: s.ID, MAC: s.MAC, Model: s.Model, Firmware: s.Firmware, Serial: s.Serial,
		CustomerID: s.CustomerID, AssignedAt: s.AssignedAt, Notes: s.Notes,
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

func (m *STBModel) toDomain() *domain.STB {
	return &domain.STB{
		ID: m.ID, MAC: m.MAC, Model: m.Model, Firmware: m.Firmware, Serial: m.Serial,
		CustomerID: m.CustomerID, AssignedAt: m.AssignedAt, Notes: m.Notes,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

// ── SubscriptionKey ───────────────────────────────────────────────────────────

type SubscriptionKeyModel struct {
	ID             string     `gorm:"primaryKey;type:text"`
	SubscriptionID string     `gorm:"type:text;not null"`
	CustomerID     string     `gorm:"type:text;not null"`
	PackageID      string     `gorm:"type:text;not null"`
	Token          string     `gorm:"type:text;not null;uniqueIndex"`
	CreatedAt      time.Time
	RevokedAt      *time.Time `gorm:"type:datetime"`
}

func (SubscriptionKeyModel) TableName() string { return "iptv_subscription_keys" }

func toKeyModel(k *domain.SubscriptionKey) SubscriptionKeyModel {
	return SubscriptionKeyModel{
		ID: k.ID, SubscriptionID: k.SubscriptionID, CustomerID: k.CustomerID,
		PackageID: k.PackageID, Token: k.Token, CreatedAt: k.CreatedAt, RevokedAt: k.RevokedAt,
	}
}

func (m *SubscriptionKeyModel) toDomain() *domain.SubscriptionKey {
	return &domain.SubscriptionKey{
		ID: m.ID, SubscriptionID: m.SubscriptionID, CustomerID: m.CustomerID,
		PackageID: m.PackageID, Token: m.Token, CreatedAt: m.CreatedAt, RevokedAt: m.RevokedAt,
	}
}
