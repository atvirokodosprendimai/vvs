package domain

import (
	"context"
	"time"
)

// EPGProgramme is a single EPG entry for a channel.
// ChannelEPGID matches Channel.EPGSource (the tvg-id used in XMLTV).
type EPGProgramme struct {
	ID           string
	ChannelEPGID string
	Title        string
	Description  string
	StartTime    time.Time
	StopTime     time.Time
	Category     string
	Rating       string
}

// EPGProgrammeRepository is the port for EPG programme persistence.
type EPGProgrammeRepository interface {
	// Save upserts a single programme (on conflict channel_epg_id+start_time: replace).
	Save(ctx context.Context, p *EPGProgramme) error

	// BulkSave upserts a batch of programmes.
	BulkSave(ctx context.Context, ps []*EPGProgramme) error

	// ListForChannel returns programmes for a channel within [from, to].
	ListForChannel(ctx context.Context, channelEPGID string, from, to time.Time) ([]*EPGProgramme, error)

	// ListCurrentAndNext returns at most the current + next programme per channel.
	// The returned map key is ChannelEPGID; value is [current, next] (either may be nil).
	ListCurrentAndNext(ctx context.Context, channelEPGIDs []string) (map[string][2]*EPGProgramme, error)

	// DeleteBefore removes all programmes whose StopTime is before the given cutoff.
	// Used for periodic cleanup of stale EPG data.
	DeleteBefore(ctx context.Context, before time.Time) error
}
