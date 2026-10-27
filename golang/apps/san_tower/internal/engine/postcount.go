package engine

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// countSites are the sites whose posts Post Count can count: those whose
// lists tell each post's time.
var countSites = []string{"X", "Threads"}

// countPublished is how many published posts Post Count reads: San Remote
// scrolls the profile for them, the latest first.
const countPublished = 100

// postCount runs Post Count: the site's scheduled and/or published posts
// whose time falls on Date's day, in local time.
func postCount(c *call) error {
	site := c.setting("Site")
	if !slices.Contains(countSites, site) {
		return fmt.Errorf("Site %q: give one of %s", site, strings.Join(countSites, ", "))
	}
	list := cmp.Or(c.setting("List"), "Both")
	var lists []string
	switch list {
	case "Both":
		lists = []string{"Scheduled", "Published"}
	case "Scheduled", "Published":
		lists = []string{list}
	default:
		return fmt.Errorf("List %q: give Both, Scheduled or Published", list)
	}
	day, err := countDay(strings.TrimSpace(c.text("Date")))
	if err != nil {
		return err
	}
	next := day.AddDate(0, 0, 1)
	counted := &structpb.ListValue{}
	counts := map[string]string{"Scheduled": "", "Published": ""}
	total := 0
	for _, kind := range lists {
		items, at, err := c.listPostsUpTo(site, kind, countPublished)
		if err != nil {
			return err
		}
		n := 0
		oldest := time.Time{}
		for i, item := range items {
			t := at(i)
			if oldest.IsZero() || t.Before(oldest) {
				oldest = t
			}
			if t.Before(day) || !t.Before(next) {
				continue
			}
			n++
			v := toValue(item)
			if s := v.GetStructValue(); s != nil {
				s.Fields["list"] = structpb.NewStringValue(strings.ToLower(kind))
			}
			counted.Values = append(counted.Values, v)
		}
		if kind == "Published" && len(items) >= countPublished && !oldest.Before(day) {
			c.logs = append(c.logs, fmt.Sprintf("the latest %d published posts go back only to %s: older ones on %s are not counted",
				countPublished, oldest.Local().Format("2006-01-02 15:04"), day.Format(time.DateOnly)))
			c.levels = append(c.levels, v1.LogLevel_LOG_LEVEL_WARNING)
		}
		counts[kind] = strconv.Itoa(n)
		total += n
	}
	c.outputs["Count"] = stringValue(strconv.Itoa(total))
	c.outputs["Scheduled"] = stringValue(counts["Scheduled"])
	c.outputs["Published"] = stringValue(counts["Published"])
	c.outputs["Posts"] = structpb.NewListValue(counted)
	c.outputs["Date"] = stringValue(day.Format(time.DateOnly))
	c.fire("Done")
	return nil
}

// countDay is the local day a Date input names: a date, a datetime's day,
// or today when it is empty.
func countDay(s string) (time.Time, error) {
	t := now()
	if s != "" {
		var err error
		if t, err = parseDatetime(s); err != nil {
			return time.Time{}, fmt.Errorf("Date: %w", err)
		}
	}
	y, m, d := t.Local().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local), nil
}
