// Package xmltv parses XMLTV EPG files into domain.EPGProgramme slices.
// Supports the standard XMLTV format used by most EPG providers.
// Reference: https://wiki.xmltv.org/index.php/XMLTVFormat
package xmltv

import (
	"encoding/xml"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/modules/iptv/domain"
)

// xmltvDoc is the top-level XML structure.
type xmltvDoc struct {
	XMLName    xml.Name      `xml:"tv"`
	Programmes []xmltvProg   `xml:"programme"`
}

type xmltvProg struct {
	Start   string        `xml:"start,attr"`
	Stop    string        `xml:"stop,attr"`
	Channel string        `xml:"channel,attr"`
	Titles  []xmltvLang   `xml:"title"`
	Descs   []xmltvLang   `xml:"desc"`
	Category []xmltvLang  `xml:"category"`
	Rating  *xmltvRating  `xml:"rating"`
}

type xmltvLang struct {
	Lang  string `xml:"lang,attr"`
	Value string `xml:",chardata"`
}

type xmltvRating struct {
	Value string `xml:"value"`
}

// Parse reads XMLTV content from r and returns domain programmes.
// Unknown channels are included — caller filters by channelEPGID if needed.
func Parse(r io.Reader) ([]*domain.EPGProgramme, error) {
	var doc xmltvDoc
	if err := xml.NewDecoder(r).Decode(&doc); err != nil {
		return nil, fmt.Errorf("xmltv: decode: %w", err)
	}

	out := make([]*domain.EPGProgramme, 0, len(doc.Programmes))
	for _, p := range doc.Programmes {
		start, err := parseXMLTVTime(p.Start)
		if err != nil {
			continue // skip malformed entries
		}
		stop, err := parseXMLTVTime(p.Stop)
		if err != nil {
			continue
		}

		prog := &domain.EPGProgramme{
			ID:           uuid.NewString(),
			ChannelEPGID: p.Channel,
			StartTime:    start,
			StopTime:     stop,
		}

		// Prefer English title; fall back to first available.
		prog.Title = pickLang(p.Titles, "en")
		prog.Description = pickLang(p.Descs, "en")
		prog.Category = pickLang(p.Category, "en")

		if p.Rating != nil {
			prog.Rating = p.Rating.Value
		}

		out = append(out, prog)
	}
	return out, nil
}

// parseXMLTVTime parses XMLTV timestamp format: "20060102150405 +0200" or "20060102150405 +0000".
func parseXMLTVTime(s string) (time.Time, error) {
	// Try with timezone offset first, then without.
	for _, layout := range []string{"20060102150405 -0700", "20060102150405"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("xmltv: cannot parse time %q", s)
}

// pickLang returns the value for the preferred language, falling back to the first entry.
func pickLang(items []xmltvLang, preferred string) string {
	if len(items) == 0 {
		return ""
	}
	for _, item := range items {
		if item.Lang == preferred {
			return item.Value
		}
	}
	return items[0].Value
}
