package persistence

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"github.com/atvirokodosprendimai/vvs/internal/modules/iptv/domain"
	"gorm.io/gorm"
)

// ── ChannelRepository ─────────────────────────────────────────────────────────

type ChannelRepository struct{ db *gormsqlite.DB }

func NewChannelRepository(db *gormsqlite.DB) *ChannelRepository { return &ChannelRepository{db: db} }

func (r *ChannelRepository) Save(ctx context.Context, ch *domain.Channel) error {
	m := toChannelModel(ch)
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error { return tx.Save(&m).Error })
}

func (r *ChannelRepository) FindByID(ctx context.Context, id string) (*domain.Channel, error) {
	var m ChannelModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&m).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return m.toDomain(), nil
}

func (r *ChannelRepository) FindAll(ctx context.Context) ([]*domain.Channel, error) {
	var ms []ChannelModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Order("name ASC").Find(&ms).Error
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.Channel, len(ms))
	for i := range ms {
		out[i] = ms[i].toDomain()
	}
	return out, nil
}

func (r *ChannelRepository) FindByPackage(ctx context.Context, packageID string) ([]*domain.Channel, error) {
	var ms []ChannelModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.
			Joins("JOIN iptv_package_channels pc ON pc.channel_id = iptv_channels.id").
			Where("pc.package_id = ?", packageID).
			Order("pc.position ASC, iptv_channels.name ASC").
			Find(&ms).Error
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.Channel, len(ms))
	for i := range ms {
		out[i] = ms[i].toDomain()
	}
	return out, nil
}

func (r *ChannelRepository) Delete(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).Delete(&ChannelModel{}).Error
	})
}

// ── PackageRepository ─────────────────────────────────────────────────────────

type PackageRepository struct{ db *gormsqlite.DB }

func NewPackageRepository(db *gormsqlite.DB) *PackageRepository { return &PackageRepository{db: db} }

func (r *PackageRepository) Save(ctx context.Context, p *domain.Package) error {
	m := toPackageModel(p)
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error { return tx.Save(&m).Error })
}

func (r *PackageRepository) FindByID(ctx context.Context, id string) (*domain.Package, error) {
	var m PackageModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&m).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return m.toDomain(), nil
}

func (r *PackageRepository) FindAll(ctx context.Context) ([]*domain.Package, error) {
	var ms []PackageModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Order("name ASC").Find(&ms).Error
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.Package, len(ms))
	for i := range ms {
		out[i] = ms[i].toDomain()
	}
	return out, nil
}

func (r *PackageRepository) Delete(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).Delete(&PackageModel{}).Error
	})
}

func (r *PackageRepository) AddChannel(ctx context.Context, packageID, channelID string) error {
	m := PackageChannelModel{PackageID: packageID, ChannelID: channelID}
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(&m).Error
	})
}

func (r *PackageRepository) RemoveChannel(ctx context.Context, packageID, channelID string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("package_id = ? AND channel_id = ?", packageID, channelID).
			Delete(&PackageChannelModel{}).Error
	})
}

func (r *PackageRepository) ListChannelIDs(ctx context.Context, packageID string) ([]string, error) {
	var ms []PackageChannelModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("package_id = ?", packageID).Find(&ms).Error
	})
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(ms))
	for i, m := range ms {
		ids[i] = m.ChannelID
	}
	return ids, nil
}

func (r *PackageRepository) SetChannelOrder(ctx context.Context, packageID string, channelIDs []string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		for pos, chID := range channelIDs {
			res := tx.Model(&PackageChannelModel{}).
				Where("package_id = ? AND channel_id = ?", packageID, chID).
				Update("position", pos)
			if res.Error != nil {
				return res.Error
			}
			// RowsAffected == 0 means channel not in this package — skip silently.
		}
		return nil
	})
}

// ── SubscriptionRepository ────────────────────────────────────────────────────

type SubscriptionRepository struct{ db *gormsqlite.DB }

func NewSubscriptionRepository(db *gormsqlite.DB) *SubscriptionRepository {
	return &SubscriptionRepository{db: db}
}

func (r *SubscriptionRepository) Save(ctx context.Context, s *domain.Subscription) error {
	m := toSubscriptionModel(s)
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error { return tx.Save(&m).Error })
}

func (r *SubscriptionRepository) FindByID(ctx context.Context, id string) (*domain.Subscription, error) {
	var m SubscriptionModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&m).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return m.toDomain(), nil
}

func (r *SubscriptionRepository) ListForCustomer(ctx context.Context, customerID string) ([]*domain.Subscription, error) {
	var ms []SubscriptionModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("customer_id = ?", customerID).Order("created_at DESC").Find(&ms).Error
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.Subscription, len(ms))
	for i := range ms {
		out[i] = ms[i].toDomain()
	}
	return out, nil
}

func (r *SubscriptionRepository) ListActive(ctx context.Context) ([]*domain.Subscription, error) {
	var ms []SubscriptionModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("status = ?", domain.SubscriptionActive).Order("created_at DESC").Find(&ms).Error
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.Subscription, len(ms))
	for i := range ms {
		out[i] = ms[i].toDomain()
	}
	return out, nil
}

func (r *SubscriptionRepository) Delete(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).Delete(&SubscriptionModel{}).Error
	})
}

// ── STBRepository ─────────────────────────────────────────────────────────────

type STBRepository struct{ db *gormsqlite.DB }

func NewSTBRepository(db *gormsqlite.DB) *STBRepository { return &STBRepository{db: db} }

func (r *STBRepository) Save(ctx context.Context, stb *domain.STB) error {
	m := toSTBModel(stb)
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error { return tx.Save(&m).Error })
}

func (r *STBRepository) FindByID(ctx context.Context, id string) (*domain.STB, error) {
	var m STBModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&m).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return m.toDomain(), nil
}

func (r *STBRepository) FindByMAC(ctx context.Context, mac string) (*domain.STB, error) {
	var m STBModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("mac = ?", mac).First(&m).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return m.toDomain(), nil
}

func (r *STBRepository) ListForCustomer(ctx context.Context, customerID string) ([]*domain.STB, error) {
	var ms []STBModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("customer_id = ?", customerID).Order("assigned_at DESC").Find(&ms).Error
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.STB, len(ms))
	for i := range ms {
		out[i] = ms[i].toDomain()
	}
	return out, nil
}

func (r *STBRepository) ListAll(ctx context.Context) ([]*domain.STB, error) {
	var ms []STBModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Order("assigned_at DESC").Find(&ms).Error
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.STB, len(ms))
	for i := range ms {
		out[i] = ms[i].toDomain()
	}
	return out, nil
}

func (r *STBRepository) Delete(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).Delete(&STBModel{}).Error
	})
}

// ── SubscriptionKeyRepository ─────────────────────────────────────────────────

type SubscriptionKeyRepository struct{ db *gormsqlite.DB }

func NewSubscriptionKeyRepository(db *gormsqlite.DB) *SubscriptionKeyRepository {
	return &SubscriptionKeyRepository{db: db}
}

func (r *SubscriptionKeyRepository) Save(ctx context.Context, k *domain.SubscriptionKey) error {
	m := toKeyModel(k)
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error { return tx.Save(&m).Error })
}

func (r *SubscriptionKeyRepository) FindByID(ctx context.Context, id string) (*domain.SubscriptionKey, error) {
	var m SubscriptionKeyModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&m).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return m.toDomain(), nil
}

func (r *SubscriptionKeyRepository) FindByToken(ctx context.Context, token string) (*domain.SubscriptionKey, error) {
	var m SubscriptionKeyModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("token = ?", token).First(&m).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return m.toDomain(), nil
}

func (r *SubscriptionKeyRepository) FindBySubscriptionID(ctx context.Context, subscriptionID string) ([]*domain.SubscriptionKey, error) {
	var ms []SubscriptionKeyModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("subscription_id = ?", subscriptionID).Order("created_at DESC").Find(&ms).Error
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.SubscriptionKey, len(ms))
	for i := range ms {
		out[i] = ms[i].toDomain()
	}
	return out, nil
}

func (r *SubscriptionKeyRepository) Delete(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).Delete(&SubscriptionKeyModel{}).Error
	})
}
