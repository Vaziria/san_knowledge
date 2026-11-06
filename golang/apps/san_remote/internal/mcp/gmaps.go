package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wargasipil/san_remote/internal/gmaps"
)

// The gmaps_* tool (2026-11-06).

const gmapsInstructions = `
gmaps_search reads Google Maps (in English) in a background tab of that browser, as a person does there (no Places API); it only reads. It searches for a keyword around a point (latitude and longitude, or a full plus code) at a zoom, reads the list to its end (Maps lists about 120 places at most) or limit places, then opens each place for its name, category, address, phone, website, plus code, point and rating. visit_websites also fetches each place's website for its social media links. A long list takes minutes: give a limit when the task needs only a few.`

func gmapsToolDefs() []any {
	num := func(desc string) map[string]any { return map[string]any{"type": "number", "description": desc} }
	return []any{
		tool("gmaps_search", "Google Maps: places around a point",
			"Search Google Maps for a keyword around a point, and read each place it lists: name, category, address, phone, website, social media links (from the website when it is a profile; with visit_websites, also the links on the website), plus code (full, and as Maps shows it), latitude and longitude, rating and reviews, place id and its Maps link. Says whether the list was read to its end.",
			schema(map[string]any{
				"keyword":        str("What to search for, as typed in Maps' search box: \"coffee\", \"bengkel motor\""),
				"latitude":       num("The point to search around: its latitude"),
				"longitude":      num("The point to search around: its longitude"),
				"plus_code":      str("The point as a full plus code, such as 6P58RR48+P4, instead of latitude and longitude"),
				"zoom":           num("The map's zoom, 3 to 21 (default 15, a few streets; 12 is a city)"),
				"limit":          intg("At most this many places (default: all the list holds)"),
				"visit_websites": flag("Fetch each place's website for the social media links on it (slower)"),
				"browser":        browserProp,
			}, "keyword"), true, false),
	}
}

func (s *Server) gmaps(ctx context.Context, name string, a args) (string, error) {
	if name != "gmaps_search" {
		return "", fmt.Errorf("no tool %s", name)
	}
	res, err := s.GoogleMaps.Search(ctx, gmaps.Search{
		Browser: a.Browser, Lat: a.Latitude, Lng: a.Longitude, PlusCode: a.PlusCode, Zoom: a.Zoom,
		Keyword: a.Keyword, Limit: int(a.Limit), VisitWebsites: a.VisitWebsites,
	})
	if err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(res, "", "  ")
	return string(b), err
}
