package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/moviemov"
	"github.com/wargasipil/san_tower/internal/moviemov/moviemovtest"
	"github.com/wargasipil/san_tower/internal/store"
)

// movieMovEnv is a MovieMov connection (1) to a fake web_movie site whose
// admin is admin@moviemov.test, with the tower's kept tokens; connection 2
// is a Telegram bot. The data folder is a temporary one.
func movieMovEnv(t *testing.T) (*moviemovtest.Server, Env) {
	t.Helper()
	site := moviemovtest.New("admin@moviemov.test", "rahasia")
	t.Cleanup(site.Close)
	return site, Env{
		MovieMov: moviemov.NewSessions(),
		DataDir:  filepath.ToSlash(t.TempDir()),
		Connection: func(id uint32) (store.Connection, error) {
			switch id {
			case 1:
				return store.Connection{ID: 1, Name: "MovieMov", Type: store.MovieMov, Settings: &v1.ConnectionSettings{
					Type: &v1.ConnectionSettings_Moviemov{Moviemov: &v1.MovieMovSettings{BaseUri: site.API(), Username: "admin@moviemov.test", Password: "rahasia"}}}}, nil
			case 2:
				return store.Connection{ID: 2, Name: "Bot", Type: store.TelegramBot}, nil
			}
			return store.Connection{}, store.ErrNotFound
		},
	}
}

// movieMovJPEG writes a JPEG-looking file of size bytes in dir.
func movieMovJPEG(t *testing.T, dir, name string, size int) string {
	t.Helper()
	data := make([]byte, size)
	copy(data, []byte{0xFF, 0xD8, 0xFF, 0xE0})
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(p)
}

func jsonOf(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// Every field shows an example in its empty box, but the time picker,
// which shows none.
func TestMovieMovHints(t *testing.T) {
	for _, k := range movieMovKinds {
		if k.Category != "Web" || k.Group != "MovieMov" {
			t.Errorf("%s: %s › %s", k.Kind, k.Category, k.Group)
		}
		for _, in := range k.Inputs {
			if in.Field && in.Placeholder == "" && in.Picker == "" {
				t.Errorf("%s: field %s has no placeholder", k.Kind, in.Name)
			}
		}
	}
}

// Upload without a file sends JSON: the fields typed, lists as arrays,
// Draft by default, and gives the movie, its page and its slug. Three runs
// log in once.
func TestMovieMovUpload(t *testing.T) {
	site, env := movieMovEnv(t)
	s, err := runOne(t, env, "moviemov-upload", "Title", " Night Train ", "Embed URL", `<iframe src="https://vidara.to/e/abc"></iframe>`,
		"Backup Embed URLs", "https://vidara.to/e/b1\n\nhttps://vidara.to/e/b2", "Categories", "Drama, Thriller,", "Tags", `["Korea", "Tom Hanks"]`,
		"Synopsis", "A long ride.", "Release Year", "2024", "Duration", "104", "Thumbnail URL", "https://example.com/t.jpg")
	if err != nil {
		t.Fatal(err)
	}
	r := site.Last()
	if r.Method != "POST" || r.Path != "/api/movies" || r.ContentType != "application/json" {
		t.Fatalf("request = %+v", r)
	}
	if got := jsonOf(r.JSON); got != `{"backup_embed_urls":["https://vidara.to/e/b1","https://vidara.to/e/b2"],"categories":["Drama","Thriller"],"duration_minutes":104,"embed_url":"\u003ciframe src=\"https://vidara.to/e/abc\"\u003e\u003c/iframe\u003e","release_year":2024,"status":"draft","synopsis":"A long ride.","tags":["Korea","Tom Hanks"],"thumbnail_url":"https://example.com/t.jpg","title":"Night Train"}` {
		t.Errorf("sent %s", got)
	}
	if strings.Join(s.Fired, ",") != "Done" || textOut(t, s, "Slug") != "night-train" || textOut(t, s, "URL") != site.URL+"/movies/night-train" {
		t.Errorf("fired %v, slug %q, url %q", s.Fired, textOut(t, s, "Slug"), textOut(t, s, "URL"))
	}
	m := objectOut(t, s, "Movie").GetStructValue().GetFields()
	if m["status"].GetStringValue() != "draft" || m["categories"].GetListValue().GetValues()[1].GetStructValue().GetFields()["slug"].GetStringValue() != "thriller" {
		t.Errorf("Movie = %v", m)
	}
	logs := logsOf(s)
	if !strings.Contains(logs, "logged in to "+site.API()+" as admin@moviemov.test") ||
		!strings.Contains(logs, `uploaded "Night Train" (night-train, draft); sent title, embed url, backup embed urls, categories, tags, thumbnail url, synopsis, release year, duration, status draft: `+site.URL+"/movies/night-train") {
		t.Errorf("logs = %s", logs)
	}
	for _, title := range []string{"Second", "Third"} {
		if _, err := runOne(t, env, "moviemov-upload", "Title", title, "Embed URL", "https://vidara.to/e/x", "Status", "Published"); err != nil {
			t.Fatal(err)
		}
	}
	if n := site.Logins(); n != 1 {
		t.Errorf("logins = %d, want 1", n)
	}
	if got := jsonOf(site.Last().JSON); got != `{"embed_url":"https://vidara.to/e/x","status":"published","title":"Third"}` {
		t.Errorf("published: sent %s", got)
	}
}

// A thumbnail file goes as a form with the file, lists as name[]; one too
// big or not a picture is refused before anything is sent.
func TestMovieMovUploadThumbnail(t *testing.T) {
	site, env := movieMovEnv(t)
	dir := t.TempDir()
	thumb := movieMovJPEG(t, dir, "clip_thumb_2s.jpg", 4096)
	at := time.Now().Add(48 * time.Hour).In(wib).Format("2006-01-02T15:04")
	s, err := runOne(t, env, "moviemov-upload", "Title", "Night Train", "Embed URL", "https://vidara.to/e/abc", "Thumbnail", thumb,
		"Categories", "Drama", "Status", "Scheduled", "Publish At", at, "Slug", "kereta-malam")
	if err != nil {
		t.Fatal(err)
	}
	r := site.Last()
	if r.Method != "POST" || !strings.HasPrefix(r.ContentType, "multipart/form-data") || r.File != "clip_thumb_2s.jpg" || r.FileSize != 4096 {
		t.Fatalf("request = %+v", r)
	}
	want, _ := time.ParseInLocation("2006-01-02T15:04", at, wib)
	if f := r.Form; f.Get("title") != "Night Train" || f["categories[]"][0] != "Drama" || f.Get("status") != "scheduled" || f.Get("published_at") != want.Format(time.RFC3339) || f.Get("slug") != "kereta-malam" {
		t.Errorf("form = %v", f)
	}
	if !strings.HasSuffix(r.Form.Get("published_at"), "+07:00") {
		t.Errorf("published_at = %q, want WIB", r.Form.Get("published_at"))
	}
	if textOut(t, s, "Slug") != "kereta-malam" || objectOut(t, s, "Movie").GetStructValue().GetFields()["thumbnail_url"].GetStringValue() != site.URL+"/uploads/thumbnails/clip_thumb_2s.jpg" {
		t.Errorf("Movie = %v", objectOut(t, s, "Movie"))
	}

	big := movieMovJPEG(t, dir, "big.jpg", moviemov.MaxThumbnail+1)
	gif := filepath.ToSlash(filepath.Join(dir, "a.gif"))
	os.WriteFile(gif, []byte("GIF89a......"), 0o644)
	before := len(site.Requests())
	for file, want := range map[string]string{
		big:                  "big.jpg is 2.0 MB: MovieMov takes 2 MB at most",
		gif:                  "a.gif is not a JPEG, PNG or WebP picture",
		dir + "/missing.jpg": "missing.jpg is not there",
	} {
		if _, err := runOne(t, env, "moviemov-upload", "Title", "x", "Embed URL", "https://vidara.to/e/x", "Thumbnail", file); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", file, err, want)
		}
	}
	if len(site.Requests()) != before {
		t.Error("a bad thumbnail reached the site")
	}
}

// What is wrong is said before the site is called; what the site refuses
// names its fields.
func TestMovieMovUploadRefused(t *testing.T) {
	site, env := movieMovEnv(t)
	past := time.Now().Add(-time.Hour).In(wib).Format("2006-01-02T15:04")
	soon := time.Now().Add(time.Hour).Format(time.RFC3339)
	ok := []string{"Title", "x", "Embed URL", "https://vidara.to/e/x"}
	for _, c := range []struct {
		values []string
		want   string
	}{
		{[]string{"Title", "x", "Embed URL", "https://vidara.to/e/x", "Connection", ""}, "Connection: pick a MovieMov connection"},
		{append([]string{"Connection", "2"}, ok...), "connection Bot is not a MovieMov connection"},
		{[]string{"Embed URL", "https://vidara.to/e/x"}, "Title is empty"},
		{[]string{"Title", "x"}, "Embed URL is empty: give the player's https link or its <iframe> code"},
		{[]string{"Title", "x", "Embed URL", "http://vidara.to/e/x"}, `Embed URL "http://vidara.to/e/x": give the player's https link`},
		{append(ok, "Backup Embed URLs", "https://a/1\nhttps://a/2\nhttps://a/3\nhttps://a/4\nhttps://a/5\nhttps://a/6"), "Backup Embed URLs has 6 players: MovieMov takes 5 at most"},
		{append(ok, "Categories", "a,b,c,d,e,f,g,h,i,j,k"), "Categories has 11: MovieMov takes 10 at most"},
		{append(ok, "Tags", strings.Repeat("x", 51)), "is longer than 50 characters"},
		{append(ok, "Tags", "[1, 2"), `Tags "[1, 2" is not a JSON list`},
		{append(ok, "Release Year", "99"), `Release Year "99": give a year, a whole number from 1870 to 2100`},
		{append(ok, "Duration", "1h"), `Duration "1h": give minutes, a whole number from 1 to 1440`},
		{append(ok, "Thumbnail", "a.jpg", "Thumbnail URL", "https://x/a.jpg"), "give a file or a link, not both"},
		{append(ok, "Thumbnail URL", "ftp://x/a.jpg"), "give the picture's http or https link"},
		{append(ok, "Status", "Scheduled"), "Publish At is empty: a scheduled movie needs the time it goes live (WIB)"},
		{append(ok, "Status", "Scheduled", "Publish At", past), "has passed: a scheduled movie goes live later"},
		{append(ok, "Status", "Published", "Publish At", soon), "is still to come: pick Status Scheduled"},
		{append(ok, "Status", "Scheduled", "Publish At", "besok"), `Publish At "besok" is not a time`},
		{append(ok, "Slug", "night train"), `Slug "night train": letters, digits, - and _ only`},
	} {
		values := c.values
		if !slices.Contains(values, "Connection") {
			values = append([]string{"Connection", "1"}, values...)
		}
		_, err := run(t, env, "moviemov-upload", values...)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v: %v, want %q", c.values, err, c.want)
		}
	}
	if n := len(site.Requests()); n != 0 {
		t.Errorf("%d requests reached the site", n)
	}
	_, err := runOne(t, env, "moviemov-upload", "Title", "x", "Embed URL", "https://bad.example/e/x")
	if err == nil || !strings.HasSuffix(err.Error(), "MovieMov: embed_url: The embed url field must be a valid URL. (422)") {
		t.Errorf("422: %v", err)
	}
}

// Update sends only what is given: as JSON (PATCH) without a file, as a
// form (POST) with one; [] empties a list; Keep sends no status.
func TestMovieMovUpdate(t *testing.T) {
	site, env := movieMovEnv(t)
	site.AddMovie("night-train", "Night Train", "draft")
	s, err := runOne(t, env, "moviemov-update", "Slug", "https://moviemov.id/movies/night-train", "Title", "Night Train (2024)", "Tags", "[]")
	if err != nil {
		t.Fatal(err)
	}
	r := site.Last()
	if r.Method != "PATCH" || r.Path != "/api/movies/night-train" || jsonOf(r.JSON) != `{"tags":[],"title":"Night Train (2024)"}` {
		t.Errorf("request = %s %s %s", r.Method, r.Path, jsonOf(r.JSON))
	}
	if textOut(t, s, "Slug") != "night-train" || !strings.Contains(logsOf(s), `updated "Night Train (2024)" (night-train, draft); sent title, tags`) {
		t.Errorf("logs = %s", logsOf(s))
	}

	thumb := movieMovJPEG(t, t.TempDir(), "new.jpg", 2048)
	at := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	if _, err := runOne(t, env, "moviemov-update", "Slug", "night-train", "Thumbnail", thumb, "Status", "Scheduled", "Publish At", at, "New Slug", "night-train-2"); err != nil {
		t.Fatal(err)
	}
	r = site.Last()
	if r.Method != "POST" || r.Path != "/api/movies/night-train" || r.File != "new.jpg" || r.Form.Get("status") != "scheduled" || r.Form.Get("slug") != "night-train-2" || r.Form.Get("title") != "" {
		t.Errorf("request = %+v", r)
	}
	if m := site.Movie("night-train-2"); m == nil || m["status"] != "scheduled" {
		t.Errorf("movie = %v", m)
	}

	if _, err := runOne(t, env, "moviemov-update", "Slug", "night-train-2", "Remove Thumbnail", "Yes"); err != nil {
		t.Fatal(err)
	}
	if r := site.Last(); jsonOf(r.JSON) != `{"remove_thumbnail":true}` {
		t.Errorf("remove: %s", jsonOf(r.JSON))
	}
	before := len(site.Requests())
	for _, c := range []struct {
		values []string
		want   string
	}{
		{[]string{"Slug", "night-train-2"}, "nothing to change"},
		{[]string{"Slug", ""}, "Slug: \"\" is not a movie"},
		{[]string{"Slug", "night-train-2", "Remove Thumbnail", "Yes", "Thumbnail URL", "https://x/a.jpg"}, "Remove Thumbnail Yes, with a Thumbnail or Thumbnail URL"},
		{[]string{"Slug", "night-train-2", "Publish At", at}, "Publish At: pick Status Published or Scheduled"},
	} {
		if _, err := runOne(t, env, "moviemov-update", c.values...); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v: %v, want %q", c.values, err, c.want)
		}
	}
	if len(site.Requests()) != before {
		t.Error("a bad update reached the site")
	}
	if _, err := runOne(t, env, "moviemov-update", "Slug", "gone", "Title", "x"); err == nil || !strings.Contains(err.Error(), site.API()+" has no movie gone: MovieMov: No query results") {
		t.Errorf("404: %v", err)
	}
}

// Movies reads one page by Page and Per Page, or with All every page up to
// Limit; Each fires once per movie with Item and Slug.
func TestMovieMovMovies(t *testing.T) {
	site, env := movieMovEnv(t)
	for i := range 250 {
		site.AddMovie("m"+string(rune('a'+i/26))+string(rune('a'+i%26)), "Movie", "published")
	}
	site.AddMovie("d1", "Draft", "draft")
	s, err := runOne(t, env, "moviemov-movies", "Search", " Movie ", "Category", "Sci Fi", "Tags", "Korea, Tom Hanks", "Sort", "Rating", "Direction", "Ascending", "Per Page", "10", "Page", "3")
	if err != nil {
		t.Fatal(err)
	}
	if q := site.Last().Query; q != "category=sci-fi&dir=asc&page=3&per_page=10&q=Movie&sort=rating&tags=korea%2Ctom-hanks" {
		t.Errorf("query = %s", q)
	}
	if textOut(t, s, "Count") != "10" || textOut(t, s, "Total") != "250" || len(objectOut(t, s, "Movies").GetListValue().GetValues()) != 10 {
		t.Errorf("Count %s, Total %s", textOut(t, s, "Count"), textOut(t, s, "Total"))
	}

	before := len(site.Requests())
	s, err = runOne(t, env, "moviemov-movies", "Pages", "All", "Limit", "230")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(site.Requests()) - before; n != 3 || textOut(t, s, "Count") != "230" || textOut(t, s, "Total") != "250" {
		t.Errorf("%d requests, Count %s", n, textOut(t, s, "Count"))
	}
	if q := site.Last().Query; q != "page=3&per_page=100" {
		t.Errorf("query = %s", q)
	}
	if !strings.Contains(logsOf(s), "read 230 of 250 movies (3 pages)") {
		t.Errorf("logs = %s", logsOf(s))
	}

	// Each, over the drafts.
	g := &v1.Graph{
		Steps: []*v1.Step{step("ev", TopicEvent), step("s", "moviemov-movies", "Connection", "1", "Status", "Draft"), step("log", "log")},
		Links: []*v1.Link{link("ev", "Event", "s", "Event"), link("s", "Each", "log", "Event"), link("s", "Slug", "log", "Message")},
	}
	steps, err := Execute(context.Background(), g, "ev", nil, env)
	if err != nil {
		t.Fatal(err)
	}
	if got := ran(steps); !strings.Contains(got, "log::d1") {
		t.Errorf("ran %s", got)
	}
	if q := site.Last().Query; q != "page=1&per_page=24&status=draft" {
		t.Errorf("query = %s", q)
	}
}

// Delete fires Deleted, or Not Found for a movie the site does not have,
// and Done either way.
func TestMovieMovDelete(t *testing.T) {
	site, env := movieMovEnv(t)
	site.AddMovie("night-train", "Night Train", "published")
	s, err := runOne(t, env, "moviemov-delete", "Slug", "night-train")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(s.Fired, ",") != "Deleted,Done" || site.Movie("night-train") != nil || site.Last().Method != "DELETE" {
		t.Errorf("fired %v", s.Fired)
	}
	s, err = runOne(t, env, "moviemov-delete", "Slug", "night-train")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(s.Fired, ",") != "Not Found,Done" || logsOf(s) != site.API()+" has no movie night-train: nothing deleted" {
		t.Errorf("fired %v, logs %q", s.Fired, logsOf(s))
	}
}

// Reports lists the open reports with each movie's slug; Resolve Report
// resolves a movie's and says how many.
func TestMovieMovReports(t *testing.T) {
	site, env := movieMovEnv(t)
	site.AddMovie("night-train", "Night Train", "published")
	site.AddMovie("starfall", "Starfall", "published")
	site.Report("night-train", 3)
	g := &v1.Graph{
		Steps: []*v1.Step{step("ev", TopicEvent), step("r", "moviemov-reports", "Connection", "1"), step("fix", "moviemov-resolve-report", "Connection", "1")},
		Links: []*v1.Link{link("ev", "Event", "r", "Event"), link("r", "Each", "fix", "Event"), link("r", "Slug", "fix", "Slug")},
	}
	steps, err := Execute(context.Background(), g, "ev", nil, env)
	if err != nil {
		t.Fatal(err)
	}
	var r, fix *v1.StepRun
	for _, s := range steps {
		switch s.StepId {
		case "r":
			r = s
		case "fix":
			fix = s
		}
	}
	if textOut(t, r, "Count") != "1" || objectOut(t, r, "Reports").GetListValue().GetValues()[0].GetStructValue().GetFields()["open_reports"].GetNumberValue() != 3 {
		t.Errorf("Reports = %v", objectOut(t, r, "Reports"))
	}
	if textOut(t, fix, "Resolved") != "3" || site.Last().Path != "/api/reports/night-train/resolve" {
		t.Errorf("Resolved = %s", textOut(t, fix, "Resolved"))
	}
	s, err := runOne(t, env, "moviemov-resolve-report", "Slug", "starfall")
	if err != nil || textOut(t, s, "Resolved") != "0" {
		t.Errorf("none open: %v", err)
	}
	if _, err := runOne(t, env, "moviemov-resolve-report", "Slug", "gone"); err == nil || !strings.Contains(err.Error(), "has no movie gone") {
		t.Errorf("404: %v", err)
	}
}

// A list input takes a linked list (a movie's categories by name), typed
// items, or JSON.
func TestMovieMovList(t *testing.T) {
	cats, _ := structpb.NewList([]any{map[string]any{"name": "Sci-Fi", "slug": "sci-fi"}, "Drama", 2024.0})
	for _, c := range []struct {
		in    any
		lines bool
		want  string
		ok    bool
	}{
		{wrapperspb.String(" Drama, Thriller ,\nNoir "), false, `["Drama","Thriller","Noir"]`, true},
		{wrapperspb.String("<iframe src=\"https://a/1\" width=\"640\"></iframe>\nhttps://a/2"), true, `["\u003ciframe src=\"https://a/1\" width=\"640\"\u003e\u003c/iframe\u003e","https://a/2"]`, true},
		{wrapperspb.String(`["A, B"]`), false, `["A, B"]`, true},
		{wrapperspb.String("[]"), false, `[]`, true},
		{wrapperspb.String("  "), false, `null`, false},
		{structpb.NewListValue(cats), false, `["Sci-Fi","Drama","2024"]`, true},
		{structpb.NewNullValue(), false, `null`, false},
	} {
		cl := &call{inputs: map[string]proto.Message{"L": c.in.(proto.Message)}}
		got, ok, err := cl.list("L", c.lines)
		if err != nil || ok != c.ok || jsonOf(got) != c.want {
			t.Errorf("%v: %s %v %v, want %s", c.in, jsonOf(got), ok, err, c.want)
		}
	}
}

func TestMovieMovSlugify(t *testing.T) {
	for in, want := range map[string]string{"Sci-Fi": "sci-fi", " Tom  Hanks ": "tom-hanks", "korea": "korea", "Drama & Noir!": "drama-noir"} {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}
