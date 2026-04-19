package persistence

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"github.com/atvirokodosprendimai/vvs/internal/modules/iptv/domain"
	"gorm.io/gorm/clause"
)

// EPGProgrammeRepository persists XMLTV programme data.
type EPGProgrammeRepository struct{ db *gormsqlite.DB }

func NewEPGProgrammeRepository(db *gormsqlite.DB) *EPGProgrammeRepository {
	return &EPGProgrammeRepository{db: db}
}

func (r *EPGProgrammeRepository) Save(ctx context.Context, p *domain.EPGProgramme) error {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	m := toEPGModel(p)
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "channel_epg_id"}, {Name: "start_time"}},
			DoUpdates: clause.AssignmentColumns([]string{"title", "description", "stop_time", "category", "rating"}),
		}).Create(&m).Error
	})
}

func (r *EPGProgrammeRepository) BulkSave(ctx context.Context, ps []*domain.EPGProgramme) error {
	if len(ps) == 0 {
		return nil
	}
	ms := make([]EPGProgrammeModel, len(ps))
	for i, p := range ps {
		if p.ID == "" {
			p.ID = uuid.NewString()
		}
		ms[i] = toEPGModel(p)
	}
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "channel_epg_id"}, {Name: "start_time"}},
			DoUpdates: clause.AssignmentColumns([]string{"title", "description", "stop_time", "category", "rating"}),
		}).CreateInBatches(ms, 500).Error
	})
}

func (r *EPGProgrammeRepository) ListForChannel(ctx context.Context, channelEPGID string, from, to time.Time) ([]*domain.EPGProgramme, error) {
	var ms []EPGProgrammeModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("channel_epg_id = ? AND start_time >= ? AND stop_time <= ?",
			channelEPGID, from.Unix(), to.Unix()).
			Order("start_time ASC").
			Find(&ms).Error
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.EPGProgramme, len(ms))
	for i := range ms {
		out[i] = ms[i].toDomain()
	}
	return out, nil
}

func (r *EPGProgrammeRepository) ListCurrentAndNext(ctx context.Context, channelEPGIDs []string) (map[string][2]*domain.EPGProgramme, error) {
	if len(channelEPGIDs) == 0 {
		return nil, nil
	}
	now := time.Now().Unix()
	// Fetch the 2 nearest programmes per channel that haven't ended yet.
	var ms []EPGProgrammeModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("channel_epg_id IN ? AND stop_time > ?", channelEPGIDs, now).
			Order("channel_epg_id ASC, start_time ASC").
			Find(&ms).Error
	})
	if err != nil {
		return nil, err
	}
	// Group: first entry = current (or next if nothing started), second = next.
	result := make(map[string][2]*domain.EPGProgramme, len(channelEPGIDs))
	seen := make(map[string]int) // channel → count added
	for i := range ms {
		ch := ms[i].ChannelEPGID
		if seen[ch] >= 2 {
			continue
		}
		p := ms[i].toDomain()
		slot := result[ch]
		slot[seen[ch]] = p
		result[ch] = slot
		seen[ch]++
	}
	return result, nil
}

func (r *EPGProgrammeRepository) DeleteBefore(ctx context.Context, before time.Time) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("stop_time < ?", before.Unix()).Delete(&EPGProgrammeModel{}).Error
	})
}
