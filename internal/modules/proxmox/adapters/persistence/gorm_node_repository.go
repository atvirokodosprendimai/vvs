package persistence

import (
	"context"
	"fmt"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"github.com/atvirokodosprendimai/vvs/internal/modules/email/emailcrypto"
	"github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/domain"
	"gorm.io/gorm"
)

// GormNodeRepository persists Proxmox nodes with optional AES-256-GCM encryption for token secrets.
type GormNodeRepository struct {
	db     *gormsqlite.DB
	encKey []byte
}

// NewGormNodeRepository creates a node repository.
// Pass a 32-byte encKey to encrypt token secrets at rest; omit or pass nil for dev mode (plaintext).
func NewGormNodeRepository(db *gormsqlite.DB, encKey ...[]byte) *GormNodeRepository {
	var key []byte
	if len(encKey) > 0 {
		key = encKey[0]
	}
	return &GormNodeRepository{db: db, encKey: key}
}

func (r *GormNodeRepository) Save(ctx context.Context, node *domain.ProxmoxNode) error {
	enc, err := emailcrypto.EncryptPassword(r.encKey, []byte(node.TokenSecret))
	if err != nil {
		return fmt.Errorf("encrypt proxmox token secret: %w", err)
	}
	model := toNodeModel(node)
	model.TokenSecret = enc
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(model).Error
	})
}

func (r *GormNodeRepository) FindByID(ctx context.Context, id string) (*domain.ProxmoxNode, error) {
	var model NodeModel
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

func (r *GormNodeRepository) FindAll(ctx context.Context) ([]*domain.ProxmoxNode, error) {
	var nodes []*domain.ProxmoxNode
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		var models []NodeModel
		if err := tx.Order("name ASC").Find(&models).Error; err != nil {
			return err
		}
		nodes = make([]*domain.ProxmoxNode, len(models))
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

func (r *GormNodeRepository) Delete(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).Delete(&NodeModel{}).Error
	})
}

func (r *GormNodeRepository) decryptNode(m *NodeModel) (*domain.ProxmoxNode, error) {
	node := toNodeDomain(m)
	if len(m.TokenSecret) > 0 {
		plain, err := emailcrypto.DecryptPassword(r.encKey, m.TokenSecret)
		if err != nil {
			return nil, fmt.Errorf("decrypt proxmox token secret for node %s: %w", m.ID, err)
		}
		node.TokenSecret = string(plain)
	}
	return node, nil
}
