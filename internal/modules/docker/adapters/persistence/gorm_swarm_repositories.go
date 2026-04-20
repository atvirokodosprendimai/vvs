package persistence

import (
	"context"
	"fmt"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
	"github.com/atvirokodosprendimai/vvs/internal/modules/email/emailcrypto"
	"gorm.io/gorm"
)

// ── GormSwarmClusterRepository ────────────────────────────────────────────────

type GormSwarmClusterRepository struct {
	db     *gormsqlite.DB
	encKey []byte
}

func NewGormSwarmClusterRepository(db *gormsqlite.DB, encKey []byte) *GormSwarmClusterRepository {
	return &GormSwarmClusterRepository{db: db, encKey: encKey}
}

func (r *GormSwarmClusterRepository) Save(ctx context.Context, c *domain.SwarmCluster) error {
	model := toSwarmClusterModel(c)
	var err error
	if c.WgmeshKey != "" {
		model.WgmeshKey, err = emailcrypto.EncryptPassword(r.encKey, []byte(c.WgmeshKey))
		if err != nil {
			return fmt.Errorf("encrypt wgmesh_key: %w", err)
		}
	}
	if c.ManagerToken != "" {
		model.ManagerToken, err = emailcrypto.EncryptPassword(r.encKey, []byte(c.ManagerToken))
		if err != nil {
			return fmt.Errorf("encrypt manager_token: %w", err)
		}
	}
	if c.WorkerToken != "" {
		model.WorkerToken, err = emailcrypto.EncryptPassword(r.encKey, []byte(c.WorkerToken))
		if err != nil {
			return fmt.Errorf("encrypt worker_token: %w", err)
		}
	}
	if c.HetznerAPIKey != "" {
		model.HetznerAPIKey, err = emailcrypto.EncryptPassword(r.encKey, []byte(c.HetznerAPIKey))
		if err != nil {
			return fmt.Errorf("encrypt hetzner_api_key: %w", err)
		}
	}
	if len(c.SSHPrivateKey) > 0 {
		model.SSHPrivateKey, err = emailcrypto.EncryptPassword(r.encKey, c.SSHPrivateKey)
		if err != nil {
			return fmt.Errorf("encrypt ssh_private_key: %w", err)
		}
	}
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(model).Error
	})
}

func (r *GormSwarmClusterRepository) FindByID(ctx context.Context, id string) (*domain.SwarmCluster, error) {
	var m SwarmClusterModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&m).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrClusterNotFound
		}
		return nil, err
	}
	return r.decryptCluster(&m)
}

func (r *GormSwarmClusterRepository) FindAll(ctx context.Context) ([]*domain.SwarmCluster, error) {
	var result []*domain.SwarmCluster
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		var models []SwarmClusterModel
		if err := tx.Order("name ASC").Find(&models).Error; err != nil {
			return err
		}
		result = make([]*domain.SwarmCluster, len(models))
		for i := range models {
			c, err := r.decryptCluster(&models[i])
			if err != nil {
				return err
			}
			result[i] = c
		}
		return nil
	})
	return result, err
}

func (r *GormSwarmClusterRepository) Delete(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).Delete(&SwarmClusterModel{}).Error
	})
}

func (r *GormSwarmClusterRepository) decryptCluster(m *SwarmClusterModel) (*domain.SwarmCluster, error) {
	c := toSwarmClusterDomain(m)
	if len(m.WgmeshKey) > 0 {
		dec, err := emailcrypto.DecryptPassword(r.encKey, m.WgmeshKey)
		if err != nil {
			return nil, fmt.Errorf("decrypt wgmesh_key for cluster %s: %w", m.ID, err)
		}
		c.WgmeshKey = string(dec)
	}
	if len(m.ManagerToken) > 0 {
		dec, err := emailcrypto.DecryptPassword(r.encKey, m.ManagerToken)
		if err != nil {
			return nil, fmt.Errorf("decrypt manager_token for cluster %s: %w", m.ID, err)
		}
		c.ManagerToken = string(dec)
	}
	if len(m.WorkerToken) > 0 {
		dec, err := emailcrypto.DecryptPassword(r.encKey, m.WorkerToken)
		if err != nil {
			return nil, fmt.Errorf("decrypt worker_token for cluster %s: %w", m.ID, err)
		}
		c.WorkerToken = string(dec)
	}
	if len(m.HetznerAPIKey) > 0 {
		dec, err := emailcrypto.DecryptPassword(r.encKey, m.HetznerAPIKey)
		if err != nil {
			return nil, fmt.Errorf("decrypt hetzner_api_key for cluster %s: %w", m.ID, err)
		}
		c.HetznerAPIKey = string(dec)
	}
	if len(m.SSHPrivateKey) > 0 {
		dec, err := emailcrypto.DecryptPassword(r.encKey, m.SSHPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("decrypt ssh_private_key for cluster %s: %w", m.ID, err)
		}
		c.SSHPrivateKey = dec
	}
	return c, nil
}

// ── GormSwarmNodeRepository ───────────────────────────────────────────────────

type GormSwarmNodeRepository struct {
	db     *gormsqlite.DB
	encKey []byte
}

func NewGormSwarmNodeRepository(db *gormsqlite.DB, encKey []byte) *GormSwarmNodeRepository {
	return &GormSwarmNodeRepository{db: db, encKey: encKey}
}

func (r *GormSwarmNodeRepository) Save(ctx context.Context, n *domain.SwarmNode) error {
	model := toSwarmNodeModel(n)
	if len(n.SshKey) > 0 {
		enc, err := emailcrypto.EncryptPassword(r.encKey, n.SshKey)
		if err != nil {
			return fmt.Errorf("encrypt ssh_key: %w", err)
		}
		model.SshKey = enc
	}
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(model).Error
	})
}

func (r *GormSwarmNodeRepository) FindByID(ctx context.Context, id string) (*domain.SwarmNode, error) {
	var m SwarmNodeModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&m).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrSwarmNodeNotFound
		}
		return nil, err
	}
	return r.decryptNode(&m)
}

func (r *GormSwarmNodeRepository) FindByClusterID(ctx context.Context, clusterID string) ([]*domain.SwarmNode, error) {
	var result []*domain.SwarmNode
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		var models []SwarmNodeModel
		if err := tx.Where("cluster_id = ?", clusterID).Order("name ASC").Find(&models).Error; err != nil {
			return err
		}
		result = make([]*domain.SwarmNode, len(models))
		for i := range models {
			n, err := r.decryptNode(&models[i])
			if err != nil {
				return err
			}
			result[i] = n
		}
		return nil
	})
	return result, err
}

func (r *GormSwarmNodeRepository) FindAll(ctx context.Context) ([]*domain.SwarmNode, error) {
	var result []*domain.SwarmNode
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		var models []SwarmNodeModel
		if err := tx.Order("name ASC").Find(&models).Error; err != nil {
			return err
		}
		result = make([]*domain.SwarmNode, len(models))
		for i := range models {
			n, err := r.decryptNode(&models[i])
			if err != nil {
				return err
			}
			result[i] = n
		}
		return nil
	})
	return result, err
}

func (r *GormSwarmNodeRepository) Delete(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).Delete(&SwarmNodeModel{}).Error
	})
}

func (r *GormSwarmNodeRepository) decryptNode(m *SwarmNodeModel) (*domain.SwarmNode, error) {
	n := toSwarmNodeDomain(m)
	if len(m.SshKey) > 0 {
		dec, err := emailcrypto.DecryptPassword(r.encKey, m.SshKey)
		if err != nil {
			return nil, fmt.Errorf("decrypt ssh_key for node %s: %w", m.ID, err)
		}
		n.SshKey = dec
	}
	return n, nil
}

// ── GormSwarmNetworkRepository ────────────────────────────────────────────────

type GormSwarmNetworkRepository struct {
	db *gormsqlite.DB
}

func NewGormSwarmNetworkRepository(db *gormsqlite.DB) *GormSwarmNetworkRepository {
	return &GormSwarmNetworkRepository{db: db}
}

func (r *GormSwarmNetworkRepository) Save(ctx context.Context, n *domain.SwarmNetwork) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(toSwarmNetworkModel(n)).Error
	})
}

func (r *GormSwarmNetworkRepository) FindByID(ctx context.Context, id string) (*domain.SwarmNetwork, error) {
	var m SwarmNetworkModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&m).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNetworkNotFound
		}
		return nil, err
	}
	return toSwarmNetworkDomain(&m), nil
}

func (r *GormSwarmNetworkRepository) FindByClusterID(ctx context.Context, clusterID string) ([]*domain.SwarmNetwork, error) {
	var result []*domain.SwarmNetwork
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		var models []SwarmNetworkModel
		if err := tx.Where("cluster_id = ?", clusterID).Order("name ASC").Find(&models).Error; err != nil {
			return err
		}
		result = make([]*domain.SwarmNetwork, len(models))
		for i := range models {
			result[i] = toSwarmNetworkDomain(&models[i])
		}
		return nil
	})
	return result, err
}

func (r *GormSwarmNetworkRepository) FindAll(ctx context.Context) ([]*domain.SwarmNetwork, error) {
	var result []*domain.SwarmNetwork
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		var models []SwarmNetworkModel
		if err := tx.Order("name ASC").Find(&models).Error; err != nil {
			return err
		}
		result = make([]*domain.SwarmNetwork, len(models))
		for i := range models {
			result[i] = toSwarmNetworkDomain(&models[i])
		}
		return nil
	})
	return result, err
}

func (r *GormSwarmNetworkRepository) Delete(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).Delete(&SwarmNetworkModel{}).Error
	})
}

// ── GormSwarmStackRepository ──────────────────────────────────────────────────

type GormSwarmStackRepository struct {
	db *gormsqlite.DB
}

func NewGormSwarmStackRepository(db *gormsqlite.DB) *GormSwarmStackRepository {
	return &GormSwarmStackRepository{db: db}
}

func (r *GormSwarmStackRepository) Save(ctx context.Context, s *domain.SwarmStack) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(toSwarmStackModel(s)).Error
	})
}

func (r *GormSwarmStackRepository) FindByID(ctx context.Context, id string) (*domain.SwarmStack, error) {
	var m SwarmStackModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&m).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrStackNotFound
		}
		return nil, err
	}
	return toSwarmStackDomain(&m), nil
}

func (r *GormSwarmStackRepository) FindByClusterID(ctx context.Context, clusterID string) ([]*domain.SwarmStack, error) {
	var result []*domain.SwarmStack
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		var models []SwarmStackModel
		if err := tx.Where("cluster_id = ?", clusterID).Order("name ASC").Find(&models).Error; err != nil {
			return err
		}
		result = make([]*domain.SwarmStack, len(models))
		for i := range models {
			result[i] = toSwarmStackDomain(&models[i])
		}
		return nil
	})
	return result, err
}

func (r *GormSwarmStackRepository) Delete(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).Delete(&SwarmStackModel{}).Error
	})
}

func (r *GormSwarmStackRepository) SaveRoute(ctx context.Context, route *domain.SwarmRoute) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(toSwarmRouteModel(route)).Error
	})
}

func (r *GormSwarmStackRepository) FindRoutesByStackID(ctx context.Context, stackID string) ([]*domain.SwarmRoute, error) {
	var result []*domain.SwarmRoute
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		var models []SwarmRouteModel
		if err := tx.Where("stack_id = ?", stackID).Order("hostname ASC").Find(&models).Error; err != nil {
			return err
		}
		result = make([]*domain.SwarmRoute, len(models))
		for i := range models {
			result[i] = toSwarmRouteDomain(&models[i])
		}
		return nil
	})
	return result, err
}

func (r *GormSwarmStackRepository) DeleteRoute(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).Delete(&SwarmRouteModel{}).Error
	})
}
