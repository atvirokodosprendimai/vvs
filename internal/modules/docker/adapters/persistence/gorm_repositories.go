package persistence

import (
	"context"
	"fmt"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
	"github.com/atvirokodosprendimai/vvs/internal/modules/email/emailcrypto"
	"gorm.io/gorm"
)

// ── GormDockerNodeRepository ─────────────────────────────────────────────────

type GormDockerNodeRepository struct {
	db     *gormsqlite.DB
	encKey []byte
}

// NewGormDockerNodeRepository creates a node repository.
// Pass a 32-byte encKey to encrypt TLS credentials at rest; omit for dev mode (plaintext).
func NewGormDockerNodeRepository(db *gormsqlite.DB, encKey ...[]byte) *GormDockerNodeRepository {
	var key []byte
	if len(encKey) > 0 {
		key = encKey[0]
	}
	return &GormDockerNodeRepository{db: db, encKey: key}
}

func (r *GormDockerNodeRepository) Save(ctx context.Context, node *domain.DockerNode) error {
	model := toNodeModel(node)

	// Encrypt TLS credentials if present
	if len(node.TLSCert) > 0 {
		enc, err := emailcrypto.EncryptPassword(r.encKey, node.TLSCert)
		if err != nil {
			return fmt.Errorf("encrypt docker tls_cert: %w", err)
		}
		model.TLSCert = enc
	}
	if len(node.TLSKey) > 0 {
		enc, err := emailcrypto.EncryptPassword(r.encKey, node.TLSKey)
		if err != nil {
			return fmt.Errorf("encrypt docker tls_key: %w", err)
		}
		model.TLSKey = enc
	}
	if len(node.TLSCA) > 0 {
		enc, err := emailcrypto.EncryptPassword(r.encKey, node.TLSCA)
		if err != nil {
			return fmt.Errorf("encrypt docker tls_ca: %w", err)
		}
		model.TLSCA = enc
	}

	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(model).Error
	})
}

func (r *GormDockerNodeRepository) FindByID(ctx context.Context, id string) (*domain.DockerNode, error) {
	var model DockerNodeModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&model).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNodeNotFound
		}
		return nil, err
	}
	return r.decryptNode(&model)
}

func (r *GormDockerNodeRepository) FindAll(ctx context.Context) ([]*domain.DockerNode, error) {
	var nodes []*domain.DockerNode
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		var models []DockerNodeModel
		if err := tx.Order("name ASC").Find(&models).Error; err != nil {
			return err
		}
		nodes = make([]*domain.DockerNode, len(models))
		for i := range models {
			n, err := r.decryptNode(&models[i])
			if err != nil {
				return err
			}
			nodes[i] = n
		}
		return nil
	})
	return nodes, err
}

func (r *GormDockerNodeRepository) Delete(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).Delete(&DockerNodeModel{}).Error
	})
}

func (r *GormDockerNodeRepository) decryptNode(m *DockerNodeModel) (*domain.DockerNode, error) {
	node := toNodeDomain(m)
	var err error
	if len(m.TLSCert) > 0 {
		node.TLSCert, err = emailcrypto.DecryptPassword(r.encKey, m.TLSCert)
		if err != nil {
			return nil, fmt.Errorf("decrypt docker tls_cert for node %s: %w", m.ID, err)
		}
	}
	if len(m.TLSKey) > 0 {
		node.TLSKey, err = emailcrypto.DecryptPassword(r.encKey, m.TLSKey)
		if err != nil {
			return nil, fmt.Errorf("decrypt docker tls_key for node %s: %w", m.ID, err)
		}
	}
	if len(m.TLSCA) > 0 {
		node.TLSCA, err = emailcrypto.DecryptPassword(r.encKey, m.TLSCA)
		if err != nil {
			return nil, fmt.Errorf("decrypt docker tls_ca for node %s: %w", m.ID, err)
		}
	}
	return node, nil
}

// ── GormDockerServiceRepository ──────────────────────────────────────────────

type GormDockerServiceRepository struct {
	db *gormsqlite.DB
}

func NewGormDockerServiceRepository(db *gormsqlite.DB) *GormDockerServiceRepository {
	return &GormDockerServiceRepository{db: db}
}

func (r *GormDockerServiceRepository) Save(ctx context.Context, svc *domain.DockerService) error {
	model := toServiceModel(svc)
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(model).Error
	})
}

func (r *GormDockerServiceRepository) FindByID(ctx context.Context, id string) (*domain.DockerService, error) {
	var model DockerServiceModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&model).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrServiceNotFound
		}
		return nil, err
	}
	return toServiceDomain(&model), nil
}

func (r *GormDockerServiceRepository) FindAll(ctx context.Context) ([]*domain.DockerService, error) {
	var services []*domain.DockerService
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		var models []DockerServiceModel
		if err := tx.Order("created_at DESC").Find(&models).Error; err != nil {
			return err
		}
		services = make([]*domain.DockerService, len(models))
		for i := range models {
			services[i] = toServiceDomain(&models[i])
		}
		return nil
	})
	return services, err
}

func (r *GormDockerServiceRepository) FindByNodeID(ctx context.Context, nodeID string) ([]*domain.DockerService, error) {
	var services []*domain.DockerService
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		var models []DockerServiceModel
		if err := tx.Where("node_id = ?", nodeID).Order("created_at DESC").Find(&models).Error; err != nil {
			return err
		}
		services = make([]*domain.DockerService, len(models))
		for i := range models {
			services[i] = toServiceDomain(&models[i])
		}
		return nil
	})
	return services, err
}

func (r *GormDockerServiceRepository) Delete(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).Delete(&DockerServiceModel{}).Error
	})
}
