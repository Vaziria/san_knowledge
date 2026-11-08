package engine

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/moviemov"
	"github.com/wargasipil/san_tower/internal/store"
)

// The MovieMov steps (the user, 2026-11-08: "In tower can we have node to
// upload to this web that deployed with web_movie ?", then "For 1 yes. For
// login, we create new connection type and paste username and password and
// base uri"): a site made from web_movie (php/apps/web_movie, MovieMov at
// https://moviemov.id), through its API (internal/moviemov), as the admin of
// a MovieMov connection. The tower logs in itself and keeps the token
// (Env.MovieMov), so a run does not log in. Category Web, group MovieMov.
//
// A list input (Backup Embed URLs, Categories, Tags) is an object socket
// that is also a field: a list linked to it (Read JSONL's Records, a
// movie's categories), or typed as "Drama, Thriller" (embed URLs a line
// each), or JSON such as ["Drama"]; [] is an empty list, which on an update
// removes them all.

var movieMovStatuses = []string{"Draft", "Published", "Scheduled"}

// statusFromInput is the Status that takes the status from the Status
// Value input (2026-11-08: a Telegram Form's status answer).
const statusFromInput = "From Input"

// refusedNote is how Upload and Update say what a refusal does, in their
// descriptions.
const refusedNote = " With Refused linked, a refusal of the fields (422) fires Refused instead, with Reason and Errors, the refused fields' messages by field ({\"slug\": \"The slug has already been taken.\"}), which a Telegram Form's Fix and Errors take to ask those questions again."

var movieMovKinds = []*kind{
	{
		StepKind: &v1.StepKind{
			Kind: "moviemov-upload", Title: "MovieMov Upload Movie", Category: "Web", Group: "MovieMov",
			Description: "Creates a movie on the MovieMov connection's site (a site made from web_movie) with Title and Embed URL (the player's https link or its <iframe> code), and optional Backup Embed URLs (up to 5, a line each), Thumbnail (a jpg, png or webp file of 2 MB at most, such as Video Thumbnail's File) or Thumbnail URL, Categories (up to 10) and Tags (up to 20), separated by commas or a list, Synopsis, Release Year, Duration (minutes) and Slug (empty: made from the title). Status Draft (the default) keeps it hidden; Published puts it live now, or at a past Publish At; Scheduled puts it live by itself at Publish At (WIB unless it says otherwise). Status From Input takes it from Status Value (Draft, Published or Scheduled, any case). Movie is the movie as the site gives it, URL its page and Slug its slug." + refusedNote,
			Outputs: []*v1.Socket{sock("Done", Event), sock("Refused", Event), sock("Movie", Object), sock("URL", Text), sock("Slug", Text),
				sock("Reason", Text), sock("Errors", Object)},
			Settings: []*v1.Setting{movieMovSetting(), {Name: "Status", Options: append(slices.Clone(movieMovStatuses), statusFromInput)}},
			Inputs:   append([]*v1.Socket{sock("Event", Event)}, movieMovInputs(false)...),
		},
		run: movieMovUpload,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "moviemov-update", Title: "MovieMov Update Movie", Category: "Web", Group: "MovieMov",
			Description: "Changes the movie Slug (its slug, or its page's link) on the MovieMov connection's site: only the fields that are linked or typed are sent, the others stay as they are. Fields are as MovieMov Upload Movie's; New Slug changes its page's address; a list typed as [] empties it. A Thumbnail file replaces the picture (sent as a form, POST), else the change goes as JSON (PATCH); Remove Thumbnail Yes takes the picture off. Status Keep leaves it as it is; Draft hides it; Published puts it live (one already live keeps its date); Scheduled, at Publish At; From Input takes it from Status Value (empty: Keep). Movie is the movie as it is now, URL its page and Slug its slug." + refusedNote,
			Outputs: []*v1.Socket{sock("Done", Event), sock("Refused", Event), sock("Movie", Object), sock("URL", Text), sock("Slug", Text),
				sock("Reason", Text), sock("Errors", Object)},
			Settings: []*v1.Setting{movieMovSetting(), {Name: "Status", Options: append(append([]string{"Keep"}, movieMovStatuses...), statusFromInput)},
				{Name: "Remove Thumbnail", Options: []string{"No", "Yes"}}},
			Inputs: append([]*v1.Socket{sock("Event", Event), hint("night-train, or its page's link", field("Slug"))}, movieMovInputs(true)...),
		},
		run: movieMovUpdate,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "moviemov-movies", Title: "MovieMov Movies", Category: "Web", Group: "MovieMov",
			Description: "Lists the movies on the MovieMov connection's site of Status (Published, Draft, Scheduled or All), matching Search (in titles and synopses), Category and Tags (all of up to 5, by name or slug), by Sort (Created: newest first, Trending: most views in 7 days, Rating, Views or Duration) and Direction. Pages One reads Page (default 1) of Per Page movies (1 to 100, default 24); All reads every page, 100 a page, up to Limit movies (default 1000). Each fires once per movie with Item it and Slug its slug, then Done; Movies is the list, Count how many came and Total how many the site has.",
			Outputs: []*v1.Socket{sock("Done", Event), sock("Each", Event), sock("Item", Object), sock("Slug", Text),
				sock("Movies", Object), sock("Count", Text), sock("Total", Text)},
			Settings: []*v1.Setting{movieMovSetting(),
				{Name: "Status", Options: []string{"Published", "Draft", "Scheduled", "All"}},
				{Name: "Sort", Options: []string{"Created", "Trending", "Rating", "Views", "Duration"}},
				{Name: "Direction", Options: []string{"Default", "Descending", "Ascending"}},
				{Name: "Pages", Options: []string{"One", "All"}}},
			Inputs: []*v1.Socket{sock("Event", Event),
				hint("night train (titles and synopses)", field("Search")),
				hint("sci-fi, or Sci-Fi", field("Category")),
				hint("korea, zombie (all of them, up to 5)", field("Tags")),
				only("Pages=One", hint("1 to 100, default 24", field("Per Page"))),
				only("Pages=One", hint("1", field("Page"))),
				only("Pages=All", hint("1000 (movies at most)", field("Limit")))},
		},
		run: movieMovMovies,
	},
	{
		StepKind: &v1.StepKind{
			Kind: movieMovTermsKind, Title: "MovieMov Categories and Tags", Category: "Web", Group: "MovieMov",
			Description: "Reads the categories and the tags on the MovieMov connection's site, from its public lists (no login): every category, and the tags of published movies. Order Most Used (the default) puts those with the most published movies first, Name orders them by name; Limit (default 10, 0: all) keeps that many of each. Categories and Tags are their names, as lists, which a Telegram Form's kategori Choices and tag Choices take as buttons; Category Items and Tag Items are the site's objects (name, slug, movies_count, url).",
			Outputs: []*v1.Socket{sock("Done", Event), sock("Categories", Object), sock("Tags", Object),
				sock("Category Items", Object), sock("Tag Items", Object)},
			Settings: []*v1.Setting{movieMovSetting(), {Name: "Order", Options: []string{"Most Used", "Name"}},
				{Name: "Limit", Placeholder: "10 (0: all)"}},
			Inputs: []*v1.Socket{sock("Event", Event)},
		},
		run: movieMovTerms,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "moviemov-delete", Title: "MovieMov Delete Movie", Category: "Web", Group: "MovieMov",
			Description: "Deletes the movie Slug (its slug, or its page's link) from the MovieMov connection's site, with its ratings and its uploaded thumbnail. Deleted fires when it was deleted, Not Found when the site has no such movie (already gone, or a wrong slug: logged as a warning), then Done either way.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Deleted", Event), sock("Not Found", Event)},
			Settings:    []*v1.Setting{movieMovSetting()},
			Inputs:      []*v1.Socket{sock("Event", Event), hint("night-train, or its page's link", field("Slug"))},
		},
		run: movieMovDelete,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "moviemov-reports", Title: "MovieMov Reports", Category: "Web", Group: "MovieMov",
			Description: "Lists the movies whose players visitors reported broken on the MovieMov connection's site (\"Video not working?\"), most reported first: each with its movie (slug and title), open_reports, servers (reports per player, \"1\" being Embed URL), reasons (not_loading, wrong_video, bad_quality, other) and latest_at. Each fires once per movie with Item it and Slug its movie's slug, then Done; Reports is the list and Count how many movies.",
			Outputs: []*v1.Socket{sock("Done", Event), sock("Each", Event), sock("Item", Object), sock("Slug", Text),
				sock("Reports", Object), sock("Count", Text)},
			Settings: []*v1.Setting{movieMovSetting()},
			Inputs:   []*v1.Socket{sock("Event", Event)},
		},
		run: movieMovReports,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "moviemov-resolve-report", Title: "MovieMov Resolve Report", Category: "Web", Group: "MovieMov",
			Description: "Marks every open broken-video report of the movie Slug (its slug, its page's link, or MovieMov Reports' Slug) fixed on the MovieMov connection's site; Resolved is how many there were (0 when none was open).",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Resolved", Text)},
			Settings:    []*v1.Setting{movieMovSetting()},
			Inputs:      []*v1.Socket{sock("Event", Event), hint("night-train, or MovieMov Reports' Slug", field("Slug"))},
		},
		run: movieMovResolve,
	},
}

func movieMovSetting() *v1.Setting {
	return &v1.Setting{Name: "Connection", ConnectionType: store.MovieMov}
}

// listField is a list input: an object socket that is a field too, so a
// list can be linked to it or typed in it.
func listField(name string) *v1.Socket { return &v1.Socket{Name: name, Type: Object, Field: true} }

// movieMovInputs are a movie's fields, Upload's and Update's; Update's
// slug change is New Slug, as its Slug is the movie to change.
func movieMovInputs(update bool) []*v1.Socket {
	slug := hint("Empty: made from the title (night-train)", field("Slug"))
	if update {
		slug = hint("Empty: keep it; night-train-2 changes the page's address", field("New Slug"))
	}
	publish := only("Status!=Draft", picker("Publish At", "datetime"))
	if update {
		publish = only("Status!=Keep,Status!=Draft", picker("Publish At", "datetime"))
	}
	return []*v1.Socket{
		only("Status="+statusFromInput, hint("Draft, Published or Scheduled (a Telegram Form's answer)", field("Status Value"))),
		hint("Night Train", field("Title")),
		hint("https://vidara.to/e/8SdwOuKAAk92t, or the <iframe> code", field("Embed URL")),
		hint("Up to 5, a line each (Server 2, 3, ...)", listField("Backup Embed URLs")),
		hint("Video Thumbnail's File: jpg, png or webp, 2 MB at most", field("Thumbnail")),
		hint("https://example.com/night-train.jpg", field("Thumbnail URL")),
		hint("Drama, Thriller (up to 10)", listField("Categories")),
		hint("Korea, Tom Hanks (up to 20)", listField("Tags")),
		hint("A long ride through the night.", field("Synopsis")),
		hint("2024", field("Release Year")),
		hint("104 (minutes)", field("Duration")),
		publish,
		slug,
	}
}

// movieMov is the client of the step's MovieMov connection, logged in as
// its admin with the tower's kept token (Env.MovieMov). Its logins are
// logged.
func (c *call) movieMov() (*moviemov.Client, error) {
	id, err := c.connectionID("MovieMov")
	if err != nil {
		return nil, err
	}
	if c.env.Connection == nil {
		return nil, errors.New("this run has no connections")
	}
	conn, err := c.env.Connection(id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, fmt.Errorf("connection %d is gone: pick another", id)
	}
	if err != nil {
		return nil, err
	}
	if conn.Type != store.MovieMov {
		return nil, fmt.Errorf("connection %s is not a MovieMov connection", conn.Name)
	}
	m := conn.Settings.GetMoviemov()
	sessions := c.env.MovieMov
	if sessions == nil {
		sessions = moviemov.NewSessions() // a login for this step alone
	}
	return sessions.Client(id, moviemov.Login{Base: m.GetBaseUri(), Email: m.GetUsername(), Password: m.GetPassword()},
		func(line string) { c.log(v1.LogLevel_LOG_LEVEL_INFO, line) }), nil
}

// wib is the site's time zone for a time without an offset (WIB,
// Asia/Jakarta, which has no daylight saving).
var wib = time.FixedZone("WIB", 7*60*60)

// movieMovInput reads a movie's fields from the step: for an upload, Title
// and Embed URL are required; for an update every field is optional, and
// one left empty is not sent. what lists what is sent, for the log. Nothing
// is sent from here: a bad field fails the step before the site is called.
func movieMovInput(c *call, update bool) (m moviemov.Movie, what []string, err error) {
	m.Fields = map[string]any{}
	set := func(name, field string, v any) {
		m.Fields[field] = v
		what = append(what, strings.ToLower(name))
	}
	text := func(name string, max int, required bool) (string, error) {
		s := strings.TrimSpace(c.text(name))
		switch n := utf8.RuneCountInString(s); {
		case s == "" && required:
			return "", fmt.Errorf("%s %w", name, errEmpty)
		case n > max:
			return "", fmt.Errorf("%s has %d characters: MovieMov takes %d at most", name, n, max)
		}
		return s, nil
	}

	title, err := text("Title", 255, !update)
	if err != nil {
		return m, nil, err
	}
	if title != "" {
		set("Title", "title", title)
	}
	embed, err := text("Embed URL", 20000, !update)
	if err != nil {
		return m, nil, fmt.Errorf("%w: give the player's https link or its <iframe> code", err)
	}
	if embed != "" {
		if err := checkEmbed("Embed URL", embed); err != nil {
			return m, nil, err
		}
		set("Embed URL", "embed_url", embed)
	}
	if list, ok, err := c.list("Backup Embed URLs", true); err != nil {
		return m, nil, err
	} else if ok {
		if len(list) > 5 {
			return m, nil, fmt.Errorf("Backup Embed URLs has %d players: MovieMov takes 5 at most", len(list))
		}
		for _, e := range list {
			if err := checkEmbed("Backup Embed URLs", e); err != nil {
				return m, nil, err
			}
		}
		set("Backup Embed URLs", "backup_embed_urls", list)
	}
	for _, l := range []struct {
		name, field string
		most, long  int
	}{{"Categories", "categories", 10, 100}, {"Tags", "tags", 20, 50}} {
		list, ok, err := c.list(l.name, false)
		if err != nil {
			return m, nil, err
		}
		if !ok {
			continue
		}
		if len(list) > l.most {
			return m, nil, fmt.Errorf("%s has %d: MovieMov takes %d at most", l.name, len(list), l.most)
		}
		for _, item := range list {
			if utf8.RuneCountInString(item) > l.long {
				return m, nil, fmt.Errorf("%s: %q is longer than %d characters", l.name, item, l.long)
			}
		}
		set(l.name, l.field, list)
	}

	file := strings.TrimSpace(c.text("Thumbnail"))
	thumbURL := strings.TrimSpace(c.text("Thumbnail URL"))
	switch {
	case file != "" && thumbURL != "":
		return m, nil, errors.New("Thumbnail and Thumbnail URL: give a file or a link, not both")
	case file != "":
		if m.Thumbnail, err = thumbnailFile(c, file); err != nil {
			return m, nil, err
		}
		what = append(what, "thumbnail "+path.Base(m.Thumbnail.Name))
	case thumbURL != "":
		if u, err := url.Parse(thumbURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return m, nil, fmt.Errorf("Thumbnail URL %q: give the picture's http or https link", thumbURL)
		}
		set("Thumbnail URL", "thumbnail_url", thumbURL)
	}

	synopsis, err := text("Synopsis", 10000, false)
	if err != nil {
		return m, nil, err
	}
	if synopsis != "" {
		set("Synopsis", "synopsis", synopsis)
	}
	for _, n := range []struct {
		name, field, unit string
		lo, hi            int
	}{{"Release Year", "release_year", "a year", 1870, 2100}, {"Duration", "duration_minutes", "minutes", 1, 1440}} {
		s := strings.TrimSpace(c.text(n.name))
		if s == "" {
			continue
		}
		v, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSuffix(s, "m"), " min"))
		if err != nil || v < n.lo || v > n.hi {
			return m, nil, fmt.Errorf("%s %q: give %s, a whole number from %d to %d", n.name, s, n.unit, n.lo, n.hi)
		}
		set(n.name, n.field, v)
	}
	slugName := "Slug"
	if update {
		slugName = "New Slug"
	}
	if s := strings.TrimSpace(c.text(slugName)); s != "" {
		slug, err := moviemov.Slug(s)
		if err != nil || slug != s {
			return m, nil, fmt.Errorf("%s %q: letters, digits, - and _ only, such as night-train", slugName, s)
		}
		set(slugName, "slug", slug)
	}

	status := c.setting("Status")
	if status == statusFromInput {
		v := strings.TrimSpace(c.text("Status Value"))
		status = ""
		for _, s := range append([]string{"Keep"}, movieMovStatuses...) {
			if strings.EqualFold(v, s) && (update || s != "Keep") {
				status = s
			}
		}
		if status == "" && v != "" {
			return m, nil, fmt.Errorf("Status Value %q: give Draft, Published or Scheduled", v)
		}
	}
	if status == "" {
		status = "Draft"
		if update {
			status = "Keep"
		}
	}
	at := strings.TrimSpace(c.text("Publish At"))
	switch status {
	case "Keep":
		if update && at != "" {
			return m, nil, errors.New("Publish At: pick Status Published or Scheduled to change when the movie goes live")
		}
	case "Draft":
		m.Fields["status"] = "draft"
		what = append(what, "status draft")
		if at != "" {
			c.log(v1.LogLevel_LOG_LEVEL_INFO, "Publish At is left out: a draft has no publish time")
		}
	case "Published", "Scheduled":
		m.Fields["status"] = strings.ToLower(status)
		if at == "" && status == "Scheduled" {
			return m, nil, fmt.Errorf("Publish At %w: a scheduled movie needs the time it goes live (WIB)", errEmpty)
		}
		if at == "" {
			what = append(what, "status published")
			break
		}
		t, err := publishAt(at)
		if err != nil {
			return m, nil, err
		}
		future := t.After(now())
		switch {
		case status == "Scheduled" && !future:
			return m, nil, fmt.Errorf("Publish At %s has passed: a scheduled movie goes live later (or pick Published)", t.In(wib).Format("2006-01-02 15:04 MST"))
		case status == "Published" && future:
			return m, nil, fmt.Errorf("Publish At %s is still to come: pick Status Scheduled for a movie that goes live later", t.In(wib).Format("2006-01-02 15:04 MST"))
		}
		m.Fields["published_at"] = t.Format(time.RFC3339)
		what = append(what, fmt.Sprintf("status %s at %s", strings.ToLower(status), t.In(wib).Format("2006-01-02 15:04 MST")))
	default:
		return m, nil, fmt.Errorf("Status %q: give Draft, Published or Scheduled", status)
	}
	return m, what, nil
}

// publishAt reads Publish At: RFC 3339 as given (the Datetime steps'), or
// the picker's 2026-11-09T20:00 (with a space or seconds too) in WIB, as the
// site reads a time without an offset.
func publishAt(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	for _, l := range datetimeLayouts {
		if t, err := time.ParseInLocation(l, s, wib); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("Publish At %q is not a time: give one such as 2026-11-09T20:00 (WIB) or 2026-11-09T20:00:00+07:00", s)
}

// checkEmbed checks a player is an https link or an <iframe> code, which
// the site reads the link from.
func checkEmbed(input, s string) error {
	if strings.Contains(strings.ToLower(s), "<iframe") {
		return nil
	}
	if u, err := url.Parse(s); err == nil && u.Scheme == "https" && u.Host != "" {
		return nil
	}
	return fmt.Errorf("%s %q: give the player's https link, such as https://vidara.to/e/8SdwOuKAAk92t, or its <iframe> code", input, s)
}

// thumbnailFile reads a thumbnail file, a relative path in the tower's data
// folder, and checks it is a picture the site takes: JPEG, PNG or WebP, of
// 2 MB at most.
func thumbnailFile(c *call, raw string) (*moviemov.File, error) {
	full, err := c.localFile("Thumbnail", raw)
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(full)
	if err != nil {
		return nil, err
	}
	name := filepath.ToSlash(full)
	if fi.Size() > moviemov.MaxThumbnail {
		return nil, fmt.Errorf("Thumbnail %s is %.1f MB: MovieMov takes 2 MB at most (a 1280-wide JPEG, such as Video Thumbnail's with Width 1280, is far less)", name, float64(fi.Size())/(1<<20))
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, fmt.Errorf("Thumbnail: %w", err)
	}
	if moviemov.ImageType(data) == "" {
		return nil, fmt.Errorf("Thumbnail %s is not a JPEG, PNG or WebP picture, the ones MovieMov takes", name)
	}
	return &moviemov.File{Name: name, Data: data}, nil
}

// list reads a list input: a list linked to it, a text of items (separated
// by commas and lines, or by lines alone with lines), or JSON such as
// ["Drama"]. ok is false when nothing is given; [] gives an empty list.
func (c *call) list(input string, lines bool) (items []string, ok bool, err error) {
	switch v := c.inputs[input].(type) {
	case *wrapperspb.StringValue:
		return listText(input, v.GetValue(), lines)
	case *structpb.Value:
		switch k := v.GetKind().(type) {
		case *structpb.Value_StringValue:
			return listText(input, k.StringValue, lines)
		case *structpb.Value_ListValue:
			items = []string{}
			for _, item := range k.ListValue.GetValues() {
				if s := strings.TrimSpace(itemText(item)); s != "" {
					items = append(items, s)
				}
			}
			return items, true, nil
		case nil, *structpb.Value_NullValue:
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("%s: link a list, or type the items", input)
	}
	return nil, false, nil
}

func listText(input, s string, lines bool) ([]string, bool, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, false, nil
	}
	if strings.HasPrefix(s, "[") {
		var v structpb.Value
		if err := protojson.Unmarshal([]byte(s), &v); err != nil || v.GetListValue() == nil {
			return nil, false, fmt.Errorf("%s %q is not a JSON list, such as [\"Drama\", \"Thriller\"]", input, s)
		}
		items := []string{}
		for _, item := range v.GetListValue().GetValues() {
			if t := strings.TrimSpace(itemText(item)); t != "" {
				items = append(items, t)
			}
		}
		return items, true, nil
	}
	sep := func(r rune) bool { return r == '\n' || r == '\r' || (!lines && r == ',') }
	items := []string{}
	for _, item := range strings.FieldsFunc(s, sep) {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items, true, nil
}

// itemText is a list item as text: a string as it is, a number as written,
// an object by its name (a movie's categories are {name, slug}), anything
// else as JSON.
func itemText(v *structpb.Value) string {
	switch k := v.GetKind().(type) {
	case *structpb.Value_StringValue:
		return k.StringValue
	case *structpb.Value_NumberValue:
		return strconv.FormatFloat(k.NumberValue, 'f', -1, 64)
	case *structpb.Value_BoolValue:
		return strconv.FormatBool(k.BoolValue)
	case *structpb.Value_StructValue:
		if name := k.StructValue.GetFields()["name"].GetStringValue(); name != "" {
			return name
		}
	}
	b, _ := json.Marshal(v.AsInterface())
	return string(b)
}

// movieMovErr says what the site refusing a call on movie slug means.
func movieMovErr(cl *moviemov.Client, slug string, err error) error {
	switch {
	case slug != "" && moviemov.IsStatus(err, 404):
		return fmt.Errorf("%s has no movie %s: %w", cl.Base(), slug, err)
	case moviemov.IsStatus(err, 403):
		return fmt.Errorf("%w: the connection's user may not do this (only admins can)", err)
	}
	return err
}

// movieOut gives a step the movie the site answered: Movie, URL and Slug,
// and logs what was done.
func movieOut(c *call, raw json.RawMessage, done string, what []string) error {
	var v structpb.Value
	if err := protojson.Unmarshal(raw, &v); err != nil || v.GetStructValue() == nil {
		return fmt.Errorf("MovieMov answered something else than a movie: %s", raw)
	}
	f := v.GetStructValue().GetFields()
	c.outputs["Movie"] = &v
	c.outputs["URL"] = stringValue(f["url"].GetStringValue())
	c.outputs["Slug"] = stringValue(f["slug"].GetStringValue())
	line := fmt.Sprintf("%s %q (%s, %s)", done, f["title"].GetStringValue(), f["slug"].GetStringValue(), f["status"].GetStringValue())
	if at := f["published_at"].GetStringValue(); at != "" {
		if t, err := time.Parse(time.RFC3339, at); err == nil {
			line += " at " + t.In(wib).Format("2006-01-02 15:04 MST")
		}
	}
	if len(what) > 0 {
		line += "; sent " + strings.Join(what, ", ")
	}
	c.log(v1.LogLevel_LOG_LEVEL_SUCCESS, line+": "+f["url"].GetStringValue())
	c.fire("Done")
	return nil
}

func movieMovUpload(c *call) error {
	m, what, err := movieMovInput(c, false)
	if err != nil {
		return err
	}
	cl, err := c.movieMov()
	if err != nil {
		return err
	}
	raw, err := cl.CreateMovie(c.ctx, m)
	if refused(c, err) {
		return nil
	}
	if err != nil {
		return movieMovErr(cl, "", err)
	}
	return movieOut(c, raw, "uploaded", what)
}

// refused fires Refused, with Reason and Errors, when the site refused the
// fields (422) and Refused is linked, and says whether it did: a refusal a
// workflow handles, such as by asking a Telegram Form's questions again.
func refused(c *call, err error) bool {
	var e *moviemov.Error
	if !errors.As(err, &e) || e.Status != 422 || c.linked == nil || !c.linked("Refused") {
		return false
	}
	fields := &structpb.Struct{Fields: map[string]*structpb.Value{}}
	for _, f := range e.Fields {
		fields.Fields[f.Field] = structpb.NewStringValue(strings.Join(f.Messages, " "))
	}
	reason := strings.TrimSuffix(strings.TrimPrefix(e.Error(), "MovieMov: "), " (422)")
	c.outputs["Errors"] = structpb.NewStructValue(fields)
	c.outputs["Reason"] = stringValue(reason)
	c.logf(v1.LogLevel_LOG_LEVEL_WARNING, "the site refused it: %s", reason)
	c.fire("Refused")
	return true
}

func movieMovUpdate(c *call) error {
	slug, err := moviemov.Slug(c.text("Slug"))
	if err != nil {
		return fmt.Errorf("Slug: %w", err)
	}
	m, what, err := movieMovInput(c, true)
	if err != nil {
		return err
	}
	if c.setting("Remove Thumbnail") == "Yes" {
		if m.Thumbnail != nil || m.Fields["thumbnail_url"] != nil {
			return errors.New("Remove Thumbnail Yes, with a Thumbnail or Thumbnail URL: give one or the other")
		}
		m.Fields["remove_thumbnail"] = true
		what = append(what, "thumbnail removed")
	}
	if len(m.Fields) == 0 && m.Thumbnail == nil {
		return errors.New("nothing to change: give a field, a Status or Remove Thumbnail Yes")
	}
	cl, err := c.movieMov()
	if err != nil {
		return err
	}
	raw, err := cl.UpdateMovie(c.ctx, slug, m)
	if refused(c, err) {
		return nil
	}
	if err != nil {
		return movieMovErr(cl, slug, err)
	}
	return movieOut(c, raw, "updated", what)
}

// movieMovPages are the most pages Movies' All reads (100 a page).
const movieMovPages = 100

func movieMovMovies(c *call) error {
	q := url.Values{}
	if s := strings.Join(strings.Fields(c.text("Search")), " "); s != "" {
		if utf8.RuneCountInString(s) > 100 {
			return errors.New("Search: MovieMov searches for 100 characters at most")
		}
		q.Set("q", s)
	}
	if s := strings.TrimSpace(c.text("Category")); s != "" {
		q.Set("category", slugify(s))
	}
	if s := strings.TrimSpace(c.text("Tags")); s != "" {
		var tags []string
		for _, t := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '\n' }) {
			if t = slugify(t); t != "" {
				tags = append(tags, t)
			}
		}
		if len(tags) > 5 {
			return fmt.Errorf("Tags has %d: MovieMov filters by 5 at most", len(tags))
		}
		q.Set("tags", strings.Join(tags, ","))
	}
	for _, s := range []struct{ setting, param, def string }{{"Status", "status", "Published"}, {"Sort", "sort", "Created"}} {
		if v := c.setting(s.setting); v != "" && v != s.def {
			q.Set(s.param, strings.ToLower(v))
		}
	}
	switch d := c.setting("Direction"); d {
	case "", "Default":
	case "Descending":
		q.Set("dir", "desc")
	case "Ascending":
		q.Set("dir", "asc")
	default:
		return fmt.Errorf("Direction %q: give Default, Descending or Ascending", d)
	}
	number := func(name string, lo, hi, def int) (int, error) {
		s := strings.TrimSpace(c.text(name))
		if s == "" {
			return def, nil
		}
		n, err := strconv.Atoi(s)
		if err != nil || n < lo || n > hi {
			return 0, fmt.Errorf("%s %q: give a whole number from %d to %d", name, s, lo, hi)
		}
		return n, nil
	}
	all := c.setting("Pages") == "All"
	var perPage, page, limit int
	var err error
	if all {
		perPage, page = 100, 1
		if limit, err = number("Limit", 1, movieMovPages*100, 1000); err != nil {
			return err
		}
	} else {
		if perPage, err = number("Per Page", 1, 100, 24); err != nil {
			return err
		}
		if page, err = number("Page", 1, 1_000_000, 1); err != nil {
			return err
		}
	}
	cl, err := c.movieMov()
	if err != nil {
		return err
	}
	list := &structpb.ListValue{}
	total, pages := 0, 0
	for {
		q.Set("per_page", strconv.Itoa(perPage))
		q.Set("page", strconv.Itoa(page))
		p, err := cl.Movies(c.ctx, q)
		if err != nil {
			return movieMovErr(cl, "", err)
		}
		pages++
		total = p.Meta.Total
		for _, raw := range p.Data {
			if all && len(list.Values) >= limit {
				break
			}
			var v structpb.Value
			if err := protojson.Unmarshal(raw, &v); err != nil {
				return fmt.Errorf("MovieMov answered something else than a movie: %s", raw)
			}
			list.Values = append(list.Values, &v)
		}
		if !all || page >= p.Meta.LastPage || len(list.Values) >= limit || len(p.Data) == 0 || pages >= movieMovPages {
			break
		}
		page++
	}
	c.outputs["Total"] = stringValue(strconv.Itoa(total))
	c.logf(v1.LogLevel_LOG_LEVEL_INFO, "read %d of %d movies (%d pages) from %s", len(list.Values), total, pages, cl.Base())
	return eachSlugged(c, "Movies", list, func(v *structpb.Value) string {
		return v.GetStructValue().GetFields()["slug"].GetStringValue()
	})
}

// eachSlugged gives a list step's list (as name) and Count, then fires Each
// once per item with Item it and Slug its slug, then Done.
func eachSlugged(c *call, name string, list *structpb.ListValue, slug func(*structpb.Value) string) error {
	c.outputs[name] = structpb.NewListValue(list)
	c.outputs["Count"] = stringValue(strconv.Itoa(len(list.Values)))
	c.outputs["Item"] = structpb.NewNullValue()
	c.outputs["Slug"] = stringValue("")
	for _, v := range list.Values {
		if c.each == nil {
			break
		}
		c.outputs["Item"] = v
		c.outputs["Slug"] = stringValue(slug(v))
		if err := c.each("Each"); err != nil {
			return err
		}
	}
	c.fire("Done")
	return nil
}

// slugify is a category or tag name as its slug, the site's filters' key:
// lower case, other runs than letters and digits as one dash (Sci Fi is
// sci-fi). Names with accents keep them; give the slug for those.
func slugify(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			dash = false
		} else {
			dash = true
		}
	}
	return b.String()
}

// movieMovTermsKind is MovieMov Categories and Tags (the user, 2026-11-08:
// the tag question's buttons from the tags already on moviemov.id).
const movieMovTermsKind = "moviemov-categories-tags"

func movieMovTerms(c *call) error {
	limit := 10
	if s := c.setting("Limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			return fmt.Errorf("Limit %q: how many of each to keep, such as 10 (0: all)", s)
		}
		limit = n
	}
	order := c.setting("Order")
	switch order {
	case "":
		order = "Most Used"
	case "Most Used", "Name":
	default:
		return fmt.Errorf("Order %q: give Most Used or Name", order)
	}
	cl, err := c.movieMov()
	if err != nil {
		return err
	}
	var counts []string
	for _, l := range []struct {
		what, names, items string
		read               func(context.Context) ([]json.RawMessage, error)
	}{{"categories", "Categories", "Category Items", cl.Categories}, {"tags", "Tags", "Tag Items", cl.Tags}} {
		raws, err := l.read(c.ctx)
		if err != nil {
			return movieMovErr(cl, "", err)
		}
		var items []*structpb.Value
		for _, raw := range raws {
			var v structpb.Value
			if err := protojson.Unmarshal(raw, &v); err != nil || strings.TrimSpace(v.GetStructValue().GetFields()["name"].GetStringValue()) == "" {
				return fmt.Errorf("MovieMov answered something else than its %s: %s", l.what, raw)
			}
			items = append(items, &v)
		}
		name := func(v *structpb.Value) string { return v.GetStructValue().GetFields()["name"].GetStringValue() }
		slices.SortStableFunc(items, func(a, b *structpb.Value) int {
			if order == "Most Used" {
				na, nb := a.GetStructValue().GetFields()["movies_count"].GetNumberValue(), b.GetStructValue().GetFields()["movies_count"].GetNumberValue()
				if na != nb {
					return cmp.Compare(nb, na)
				}
			}
			return cmp.Or(strings.Compare(strings.ToLower(name(a)), strings.ToLower(name(b))), strings.Compare(name(a), name(b)))
		})
		all := len(items)
		if limit > 0 && len(items) > limit {
			items = items[:limit]
		}
		names, objects := &structpb.ListValue{}, &structpb.ListValue{Values: items}
		for _, v := range items {
			names.Values = append(names.Values, structpb.NewStringValue(name(v)))
		}
		c.outputs[l.names] = structpb.NewListValue(names)
		c.outputs[l.items] = structpb.NewListValue(objects)
		counts = append(counts, fmt.Sprintf("%d of %d %s", len(items), all, l.what))
	}
	how := "the most used first"
	if order == "Name" {
		how = "by name"
	}
	c.logf(v1.LogLevel_LOG_LEVEL_INFO, "read %s from %s, %s", strings.Join(counts, " and "), cl.Base(), how)
	c.fire("Done")
	return nil
}

func movieMovDelete(c *call) error {
	slug, err := moviemov.Slug(c.text("Slug"))
	if err != nil {
		return fmt.Errorf("Slug: %w", err)
	}
	cl, err := c.movieMov()
	if err != nil {
		return err
	}
	switch err := cl.DeleteMovie(c.ctx, slug); {
	case moviemov.IsStatus(err, 404):
		c.logf(v1.LogLevel_LOG_LEVEL_WARNING, "%s has no movie %s: nothing deleted", cl.Base(), slug)
		c.fire("Not Found")
	case err != nil:
		return movieMovErr(cl, slug, err)
	default:
		c.logf(v1.LogLevel_LOG_LEVEL_SUCCESS, "deleted %s from %s", slug, cl.Base())
		c.fire("Deleted")
	}
	c.fire("Done")
	return nil
}

func movieMovReports(c *call) error {
	cl, err := c.movieMov()
	if err != nil {
		return err
	}
	reports, err := cl.Reports(c.ctx)
	if err != nil {
		return movieMovErr(cl, "", err)
	}
	list := &structpb.ListValue{}
	open := 0
	for _, raw := range reports {
		var v structpb.Value
		if err := protojson.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("MovieMov answered something else than a report: %s", raw)
		}
		open += int(v.GetStructValue().GetFields()["open_reports"].GetNumberValue())
		list.Values = append(list.Values, &v)
	}
	c.logf(v1.LogLevel_LOG_LEVEL_INFO, "%d open reports on %d movies at %s", open, len(list.Values), cl.Base())
	return eachSlugged(c, "Reports", list, func(v *structpb.Value) string {
		return v.GetStructValue().GetFields()["movie"].GetStructValue().GetFields()["slug"].GetStringValue()
	})
}

func movieMovResolve(c *call) error {
	slug, err := moviemov.Slug(c.text("Slug"))
	if err != nil {
		return fmt.Errorf("Slug: %w", err)
	}
	cl, err := c.movieMov()
	if err != nil {
		return err
	}
	n, err := cl.ResolveReport(c.ctx, slug)
	if err != nil {
		return movieMovErr(cl, slug, err)
	}
	c.outputs["Resolved"] = stringValue(strconv.Itoa(n))
	c.logf(v1.LogLevel_LOG_LEVEL_SUCCESS, "resolved %d open reports of %s at %s", n, slug, cl.Base())
	c.fire("Done")
	return nil
}
