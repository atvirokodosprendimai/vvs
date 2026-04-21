package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
	"github.com/atvirokodosprendimai/vvs/internal/modules/email/emailcrypto"
	"gorm.io/gorm"
)

// ── ContainerRegistry model ───────────────────────────────────────────────────

type ContainerRegistryModel struct {
	ID        string    `gorm:"primaryKey;type:text"`
	Name      string    `gorm:"type:text;not null"`
	URL       string    `gorm:"column:url;type:text;not null;default:''"`
	Username  string    `gorm:"column:username;type:text;not null;default:''"`
	Password  []byte    `gorm:"column:password"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (ContainerRegistryModel) TableName() string { return "container_registries" }

func toRegistryModel(r *domain.ContainerRegistry) *ContainerRegistryModel {
	return &ContainerRegistryModel{
		ID:        r.ID,
		Name:      r.Name,
		URL:       r.URL,
		Username:  r.Username,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

func toRegistryDomain(m *ContainerRegistryModel) *domain.ContainerRegistry {
	return &domain.ContainerRegistry{
		ID:        m.ID,
		Name:      m.Name,
		URL:       m.URL,
		Username:  m.Username,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

// ── GormContainerRegistryRepository ──────────────────────────────────────────

type GormContainerRegistryRepository struct {
	db     *gormsqlite.DB
	encKey []byte
}

func NewGormContainerRegistryRepository(db *gormsqlite.DB, encKey []byte) *GormContainerRegistryRepository {
	return &GormContainerRegistryRepository{db: db, encKey: encKey}
}

func (r *GormContainerRegistryRepository) Save(ctx context.Context, reg *domain.ContainerRegistry) error {
	model := toRegistryModel(reg)
	if reg.Password != "" {
		enc, err := emailcrypto.EncryptPassword(r.encKey, []byte(reg.Password))
		if err != nil {
			return fmt.Errorf("encrypt registry password: %w", err)
		}
		model.Password = enc
	}
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(model).Error
	})
}

func (r *GormContainerRegistryRepository) FindByID(ctx context.Context, id string) (*domain.ContainerRegistry, error) {
	var m ContainerRegistryModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&m).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrRegistryNotFound
		}
		return nil, err
	}
	return r.decryptRegistry(&m)
}

func (r *GormContainerRegistryRepository) FindAll(ctx context.Context) ([]*domain.ContainerRegistry, error) {
	var result []*domain.ContainerRegistry
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		var models []ContainerRegistryModel
		if err := tx.Order("name ASC").Find(&models).Error; err != nil {
			return err
		}
		result = make([]*domain.ContainerRegistry, len(models))
		for i := range models {
			reg, err := r.decryptRegistry(&models[i])
			if err != nil {
				return err
			}
			result[i] = reg
		}
		return nil
	})
	return result, err
}

func (r *GormContainerRegistryRepository) Delete(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).Delete(&ContainerRegistryModel{}).Error
	})
}

func (r *GormContainerRegistryRepository) decryptRegistry(m *ContainerRegistryModel) (*domain.ContainerRegistry, error) {
	reg := toRegistryDomain(m)
	if len(m.Password) > 0 {
		dec, err := emailcrypto.DecryptPassword(r.encKey, m.Password)
		if err != nil {
			return nil, fmt.Errorf("decrypt registry password for %s: %w", m.ID, err)
		}
		reg.Password = string(dec)
	}
	return reg, nil
}

// ── VVSDeployment model ───────────────────────────────────────────────────────

type VVSDeploymentModel struct {
	ID             string     `gorm:"primaryKey;type:text"`
	ClusterID      string     `gorm:"column:cluster_id;type:text;not null;default:''"`
	NodeID         string     `gorm:"column:node_id;type:text;not null;default:''"`
	Component      string     `gorm:"column:component;type:text;not null;default:''"`
	Source         string     `gorm:"column:source;type:text;not null;default:'image'"`
	ImageURL       string     `gorm:"column:image_url;type:text;not null;default:''"`
	RegistryID     string     `gorm:"column:registry_id;type:text;not null;default:''"`
	GitURL         string     `gorm:"column:git_url;type:text;not null;default:''"`
	GitRef         string     `gorm:"column:git_ref;type:text;not null;default:'main'"`
	NATSUrl        string     `gorm:"column:nats_url;type:text;not null;default:''"`
	Port           int        `gorm:"column:port;not null;default:8080"`
	EnvVars        string     `gorm:"column:env_vars;type:text;not null;default:'{}'"`
	Status         string     `gorm:"column:status;type:text;not null;default:'pending'"`
	ErrorMsg       string     `gorm:"column:error_msg;type:text;not null;default:''"`
	LastDeployedAt *time.Time `gorm:"column:last_deployed_at"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (VVSDeploymentModel) TableName() string { return "vvs_deployments" }

func toDeploymentModel(d *domain.VVSDeployment) *VVSDeploymentModel {
	envJSON, _ := json.Marshal(d.EnvVars)
	return &VVSDeploymentModel{
		ID:             d.ID,
		ClusterID:      d.ClusterID,
		NodeID:         d.NodeID,
		Component:      string(d.Component),
		Source:         string(d.Source),
		ImageURL:       d.ImageURL,
		RegistryID:     d.RegistryID,
		GitURL:         d.GitURL,
		GitRef:         d.GitRef,
		NATSUrl:        d.NATSUrl,
		Port:           d.Port,
		EnvVars:        string(envJSON),
		Status:         string(d.Status),
		ErrorMsg:       d.ErrorMsg,
		LastDeployedAt: d.LastDeployedAt,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
	}
}

func toDeploymentDomain(m *VVSDeploymentModel) *domain.VVSDeployment {
	d := &domain.VVSDeployment{
		ID:             m.ID,
		ClusterID:      m.ClusterID,
		NodeID:         m.NodeID,
		Component:      domain.VVSComponentType(m.Component),
		Source:         domain.VVSDeploySource(m.Source),
		ImageURL:       m.ImageURL,
		RegistryID:     m.RegistryID,
		GitURL:         m.GitURL,
		GitRef:         m.GitRef,
		NATSUrl:        m.NATSUrl,
		Port:           m.Port,
		EnvVars:        make(map[string]string),
		Status:         domain.VVSDeploymentStatus(m.Status),
		ErrorMsg:       m.ErrorMsg,
		LastDeployedAt: m.LastDeployedAt,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
	_ = json.Unmarshal([]byte(m.EnvVars), &d.EnvVars)
	return d
}

// ── GormVVSDeploymentRepository ───────────────────────────────────────────────

type GormVVSDeploymentRepository struct {
	db *gormsqlite.DB
}

func NewGormVVSDeploymentRepository(db *gormsqlite.DB) *GormVVSDeploymentRepository {
	return &GormVVSDeploymentRepository{db: db}
}

func (r *GormVVSDeploymentRepository) Save(ctx context.Context, d *domain.VVSDeployment) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(toDeploymentModel(d)).Error
	})
}

func (r *GormVVSDeploymentRepository) FindByID(ctx context.Context, id string) (*domain.VVSDeployment, error) {
	var m VVSDeploymentModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&m).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrDeploymentNotFound
		}
		return nil, err
	}
	return toDeploymentDomain(&m), nil
}

func (r *GormVVSDeploymentRepository) FindAll(ctx context.Context) ([]*domain.VVSDeployment, error) {
	var result []*domain.VVSDeployment
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		var models []VVSDeploymentModel
		if err := tx.Order("created_at DESC").Find(&models).Error; err != nil {
			return err
		}
		result = make([]*domain.VVSDeployment, len(models))
		for i := range models {
			result[i] = toDeploymentDomain(&models[i])
		}
		return nil
	})
	return result, err
}

func (r *GormVVSDeploymentRepository) Delete(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).Delete(&VVSDeploymentModel{}).Error
	})
}
