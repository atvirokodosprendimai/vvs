package commands

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/vvs/isp/internal/modules/iptv/adapters/xmltv"
	"github.com/vvs/isp/internal/modules/iptv/domain"
)

// ImportEPGCommand triggers an EPG import from an XMLTV URL.
type ImportEPGCommand struct {
	URL      string
	DaysAhead int // how many future days to keep; 0 → default 14
}

// ImportEPGHandler fetches an XMLTV file from a URL, parses it, and bulk-saves programmes.
type ImportEPGHandler struct {
	epgRepo domain.EPGProgrammeRepository
	client  *http.Client
}

func NewImportEPGHandler(epgRepo domain.EPGProgrammeRepository) *ImportEPGHandler {
	return &ImportEPGHandler{
		epgRepo: epgRepo,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// ImportResult summarises an import run.
type ImportResult struct {
	Imported int
	Skipped  int // malformed entries skipped by parser
}

func (h *ImportEPGHandler) Handle(ctx context.Context, cmd ImportEPGCommand) (*ImportResult, error) {
	if cmd.URL == "" {
		return nil, fmt.Errorf("epg import: URL required")
	}
	days := cmd.DaysAhead
	if days <= 0 {
		days = 14
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cmd.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("epg import: build request: %w", err)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("epg import: fetch %s: %w", cmd.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("epg import: HTTP %d from %s", resp.StatusCode, cmd.URL)
	}

	progs, err := xmltv.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("epg import: parse: %w", err)
	}

	// Filter to programmes within the requested window.
	cutoff := time.Now().Add(time.Duration(days) * 24 * time.Hour)
	now := time.Now()
	filtered := progs[:0]
	skipped := 0
	for _, p := range progs {
		if p.StopTime.Before(now) || p.StartTime.After(cutoff) {
			skipped++
			continue
		}
		filtered = append(filtered, p)
	}

	if err := h.epgRepo.BulkSave(ctx, filtered); err != nil {
		return nil, fmt.Errorf("epg import: save: %w", err)
	}
	return &ImportResult{Imported: len(filtered), Skipped: skipped}, nil
}
