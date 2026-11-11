package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wargasipil/san_remote/internal/trends"
)

// The trends_* tools (2026-11-11).

const trendsInstructions = `
trends_related reads a keyword's related queries from Google Trends' new Explore page (trends.google.com/explore) in a tab of that browser, as a person does there (no API); it only reads. It gives the Top queries (by search interest, 0-100) and the Rising queries (by change in percent; Breakout past 5000%), each with both numbers and its own Explore link, for a location (a country "ID", a region "ID-JK", or "" for Worldwide), a time range ("today 12-m" Past year by default, "now 7-d", "today 5-y", "all", or "YYYY-MM-DD YYYY-MM-DD"), a category id (0: all) and a search type ("", images, news, froogle, youtube). The page loads its related queries only while they are on screen, so each lookup shows its tab for a few seconds. Google limits how often a browser may look up: one lookup at a time, with a pause of several seconds between them; a "too many requests" error means waiting minutes before the next. trends_pickers reads the page's own lists: locations (Worldwide and the countries; a country's regions with country), categories with their ids, time ranges and search types.`

func trendsToolDefs() []any {
	return []any{
		tool("trends_related", "Google Trends: related queries",
			"A keyword's related queries on Google Trends' new Explore: Top (by search interest) and Rising (by change), each with its search interest, change and Explore link. A keyword with too little data gives empty lists.",
			schema(map[string]any{
				"keyword":  str("The search term, as typed in Explore's search box"),
				"geo":      str("The location: a country's code (ID), a region's (ID-JK), or empty for Worldwide (default ID)"),
				"time":     str("The time range as the page writes it: now 1-H, now 4-H, now 1-d, now 7-d, today 1-m, today 3-m, today 12-m (default), today 5-y, all, or YYYY-MM-DD YYYY-MM-DD"),
				"category": intg("The category's id from trends_pickers; 0 (default) is All Categories"),
				"property": enum("The search type (default web)", "web", "images", "news", "froogle", "youtube"),
				"browser":  browserProp,
			}, "keyword"), true, false),
		tool("trends_pickers", "Google Trends: the Explore page's filters",
			"The new Explore page's own filter lists: Worldwide and the countries (with country, that country's regions too), the categories with their ids, the time ranges and the search types.",
			schema(map[string]any{
				"country": str("Also read this country's regions (ID)"),
				"browser": browserProp,
			}), true, false),
	}
}

func (s *Server) trends(ctx context.Context, name string, a args) (string, error) {
	var res any
	var err error
	switch name {
	case "trends_related":
		geo := a.Geo
		if geo == "" {
			geo = "ID"
		}
		res, err = s.Trends.Related(ctx, trends.Query{
			Browser: a.Browser, Keyword: a.Keyword, Geo: geo, Time: a.Time, Category: a.Category, Property: a.Property,
		})
	case "trends_pickers":
		res, err = s.Trends.Pickers(ctx, a.Browser, a.Country)
	default:
		return "", fmt.Errorf("no tool %s", name)
	}
	if err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(res, "", "  ")
	return string(b), err
}
