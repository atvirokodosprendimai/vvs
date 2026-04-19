package xmltv_test

import (
	"strings"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/modules/iptv/adapters/xmltv"
)

const sampleXMLTV = `<?xml version="1.0" encoding="UTF-8"?>
<tv>
  <channel id="bbc.co.uk"><display-name>BBC One</display-name></channel>
  <programme start="20260419140000 +0000" stop="20260419150000 +0000" channel="bbc.co.uk">
    <title lang="en">News at One</title>
    <desc lang="en">Daily news programme.</desc>
    <category lang="en">News</category>
    <rating><value>G</value></rating>
  </programme>
  <programme start="20260419150000 +0200" stop="20260419160000 +0200" channel="bbc.co.uk">
    <title lang="en">Afternoon Show</title>
    <title lang="lt">Popietės laida</title>
  </programme>
  <programme start="badtime" stop="20260419170000 +0000" channel="bbc.co.uk">
    <title lang="en">Should be skipped</title>
  </programme>
</tv>`

func TestParse_BasicProgrammes(t *testing.T) {
	progs, err := xmltv.Parse(strings.NewReader(sampleXMLTV))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 3rd entry has malformed start time → skipped; only 2 valid
	if len(progs) != 2 {
		t.Fatalf("expected 2 programmes, got %d", len(progs))
	}
}

func TestParse_FieldMapping(t *testing.T) {
	progs, _ := xmltv.Parse(strings.NewReader(sampleXMLTV))
	p := progs[0]

	if p.ChannelEPGID != "bbc.co.uk" {
		t.Errorf("ChannelEPGID: want %q, got %q", "bbc.co.uk", p.ChannelEPGID)
	}
	if p.Title != "News at One" {
		t.Errorf("Title: want %q, got %q", "News at One", p.Title)
	}
	if p.Description != "Daily news programme." {
		t.Errorf("Description: want %q, got %q", "Daily news programme.", p.Description)
	}
	if p.Category != "News" {
		t.Errorf("Category: want %q, got %q", "News", p.Category)
	}
	if p.Rating != "G" {
		t.Errorf("Rating: want %q, got %q", "G", p.Rating)
	}
	if p.ID == "" {
		t.Error("ID should be generated")
	}
}

func TestParse_TimezoneOffset(t *testing.T) {
	progs, _ := xmltv.Parse(strings.NewReader(sampleXMLTV))
	// Second programme uses +0200 — should be normalised to UTC
	p := progs[1]
	// 15:00 +0200 = 13:00 UTC
	want := time.Date(2026, 4, 19, 13, 0, 0, 0, time.UTC)
	if !p.StartTime.Equal(want) {
		t.Errorf("StartTime: want %v, got %v", want, p.StartTime)
	}
}

func TestParse_LangFallback(t *testing.T) {
	progs, _ := xmltv.Parse(strings.NewReader(sampleXMLTV))
	// Second programme has lt + en titles — should pick en
	p := progs[1]
	if p.Title != "Afternoon Show" {
		t.Errorf("Title: want %q, got %q", "Afternoon Show", p.Title)
	}
}

func TestParse_MalformedTime_Skipped(t *testing.T) {
	progs, err := xmltv.Parse(strings.NewReader(sampleXMLTV))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range progs {
		if p.Title == "Should be skipped" {
			t.Error("malformed-time entry should have been skipped")
		}
	}
}

func TestParse_EmptyInput(t *testing.T) {
	_, err := xmltv.Parse(strings.NewReader(`<tv></tv>`))
	if err != nil {
		t.Fatalf("empty tv element should not error: %v", err)
	}
}
