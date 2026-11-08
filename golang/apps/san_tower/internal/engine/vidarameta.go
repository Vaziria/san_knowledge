package engine

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"image"
	_ "image/jpeg" // the size of a JPEG thumbnail
	_ "image/png"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/moviemov"
)

// Vidara Metadata (the user, 2026-11-08: "Can we have vidara metadata node
// video, it contain thumbnail and duration, remove node thumbnail"). It took
// the place of Vidara Thumbnail (the user, 2026-11-08: "can we have get
// thumbnail url from vidara video ?, use this sample
// https://vidara.to/v/8SdwOuKAAk92t", then "Yes" to downloading the picture
// by default, so MovieMov keeps a copy that outlives Vidara's domain
// changes). It reads over plain HTTP, without San Remote or a browser, the
// way Vidara's player does:
//
//  1. the video's page, /v/<code> or the player's /e/<code>: og:image, the
//     title, and the player's own host, which Vidara rotates
//     (as9u7bgh9.com for the sample): the /v/ page's <iframe>, the /e/
//     page's MIRROR;
//  2. POST <player host>/api/stream {"filecode", "device", "codecs"}: the
//     title, thumbnail, subtitles and streaming_url, an HLS master
//     playlist. Its token is bound to the IP that asked: vidara.to's own
//     /api/stream answers too, but from behind Vidara's front proxy, so its
//     token is the proxy's and its playlist answers 404. Hence the player's
//     host first, and vidara.to only as the last resort;
//  3. the master playlist: the best stream (the most pixels, then the most
//     bits a second), its size, frame rate, bitrate and codecs;
//  4. that stream's playlist: Duration is the sum of its #EXTINF.
//
// What comes after the page only adds to it: when a later part fails, the
// step still gives what it found, with a warning saying which part.

// The Thumbnail setting's options; Download is the default.
const (
	vidaraDownload = "Download"
	vidaraLinkOnly = "Link Only"
)

var vidaraMetadataKind = &kind{
	StepKind: &v1.StepKind{
		Kind: "vidara-metadata", Title: "Vidara Metadata", Category: "Social",
		Group:       "Vidara",
		Description: "Reads a Vidara video's details over plain HTTP, without a browser or San Remote: its title, its thumbnail, and from its HLS stream its duration, picture size, frame rate, bitrate and codecs. Link is the video's link (https://vidara.to/v/<code>, the player's /e/<code>, on vidara.so or another of Vidara's domains), its <iframe> code, or its code. The video's page is read first (its og:image, its title, the player's host), then the player's /api/stream, the stream's master playlist (its best stream) and that stream's playlist, whose segments add up to Duration. Thumbnail Download (the default) saves the picture in the run's own folder, removed when the run ends: Thumbnail is its file and Thumbnail URL its link. Link Only downloads nothing and gives only Thumbnail URL. Link one of the two into MovieMov Upload Movie, Thumbnail or Thumbnail URL, never both: given both it refuses. Vidara's thumbnails are small (320x132), not 16:9. Duration is in seconds (224.84), Duration Minutes in whole minutes, at least 1, for MovieMov Upload Movie's Duration (4), Duration Text 3:45 or 1:02:03. Width, Height, Frame Rate, Bitrate (bits a second) and Codecs are the best stream's, Vidara's own encoding of the video; Subtitles is the list of its subtitles; Metadata holds all of it as one object. Found fires when Vidara has the video (a thumbnail or a stream), Not Found when it has not (a link that is not Vidara's, a video that is gone: a warning), then Done either way. When the stream cannot be read, Found still fires with the rest and the stream's outputs empty, and a warning says which part failed. Own Thumbnail, when given (a Telegram Form's photo answer), is passed on as Thumbnail and Vidara's picture is not downloaded; the rest is still read, and Vidara not answering at all is then a warning (only Done fires) instead of a failure.",
		Outputs: []*v1.Socket{sock("Done", Event), sock("Found", Event), sock("Not Found", Event),
			sock("Title", Text), sock("Video ID", Text), sock("Embed URL", Text), sock("Thumbnail", Text), sock("Thumbnail URL", Text),
			sock("Duration", Text), sock("Duration Minutes", Text), sock("Duration Text", Text),
			sock("Width", Text), sock("Height", Text), sock("Frame Rate", Text), sock("Bitrate", Text), sock("Codecs", Text),
			sock("Subtitles", Object), sock("Metadata", Object)},
		Settings: []*v1.Setting{{Name: "Thumbnail", Options: []string{vidaraDownload, vidaraLinkOnly}}},
		Inputs: []*v1.Socket{sock("Event", Event),
			hint("https://vidara.to/v/8SdwOuKAAk92t, the <iframe> code or 8SdwOuKAAk92t", field("Link")),
			hint("Empty: Vidara's; a file of your own (a Telegram Form's photo) is used instead", field("Own Thumbnail"))},
	},
	run: vidaraMetadata,
}

// vidaraClient reads Vidara's pages, its API, playlists and pictures: each
// answer within 20 seconds, as a page is some 20-50 KB, a playlist 2 KB and
// a thumbnail 4 KB. Tests point its transport at a fake.
var vidaraClient = &http.Client{Timeout: 20 * time.Second}

// vidaraHome is Vidara's main site, whose /v/<code> is read when the
// link's own site does not answer, and whose /api/stream is the last one
// asked; vidaraEmbed is the player's link the step gives, as MovieMov's
// Embed URL takes it.
const (
	vidaraHome  = "https://vidara.to"
	vidaraEmbed = "https://vidara.to/e/"
)

// vidaraUA is a browser's User-Agent: the pages are a browser's.
const vidaraUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36"

// The most read of a page, an API answer or a playlist, and of a picture:
// those are far less; the picture is held to what MovieMov takes (2 MB),
// Vidara's being some 4 KB.
const (
	vidaraPageMax  = 2 << 20
	vidaraImageMax = moviemov.MaxThumbnail
)

// vidaraCode is what Vidara's video codes look like ("8SdwOuKAAk92t"), as
// San Remote's vidara.Code reads them.
var vidaraCode = regexp.MustCompile(`^[A-Za-z0-9]{6,24}$`)

var (
	iframeSrc = regexp.MustCompile(`(?is)<iframe\b[^>]*?\ssrc\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'>]+))`)
	metaTag   = regexp.MustCompile(`(?is)<meta\b[^>]*>`)
	tagAttr   = regexp.MustCompile(`(?is)([a-z][a-z0-9:_-]*)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'>]+))`)
	titleTag  = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	// mirrorVar is the player's link in the /e/ page's script, where a
	// browser is sent when it can reach it: var MIRROR = 'https://<host>/e/<code>'.
	mirrorVar = regexp.MustCompile(`\bMIRROR(?:_ORIGIN)?\s*=\s*['"]([^'"]+)['"]`)
)

// errNotVidara is a link that is no Vidara video's, such as another
// player's: there is no Vidara video (Not Found), rather than a mistake.
var errNotVidara = errors.New("is not a Vidara video's link (/v/<code> or /e/<code>)")

// vidaraVideo reads a video from Link: its link on any of Vidara's domains
// (/v/<code>, /e/<code>, /d/<code>), the src of its <iframe> code, or its
// bare code. page is the link's own page to read first, without its query
// ("" for a bare code).
func vidaraVideo(s string) (code, page string, err error) {
	s = strings.TrimSpace(s)
	if vidaraCode.MatchString(s) {
		return s, "", nil
	}
	link := s
	if strings.Contains(strings.ToLower(s), "<iframe") {
		m := iframeSrc.FindStringSubmatch(s)
		if m == nil {
			return "", "", errors.New("Link: the <iframe> code has no src link")
		}
		link = html.UnescapeString(strings.TrimSpace(m[1] + m[2] + m[3]))
		if strings.HasPrefix(link, "//") {
			link = "https:" + link
		}
	}
	u, err := url.Parse(link)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", "", fmt.Errorf("Link %q: give a Vidara video's link (https://vidara.to/v/8SdwOuKAAk92t), its <iframe> code or its code", s)
	}
	if typ, c := videoPath(u); c != "" {
		return c, (&url.URL{Scheme: u.Scheme, Host: u.Host, Path: "/" + typ + "/" + c}).String(), nil
	}
	return "", "", fmt.Errorf("Link %s %w", u.Redacted(), errNotVidara)
}

// videoPath reads a video's /v/, /e/ or /d/ link: the kind of page (v, e
// or d) and the code, "" for another link.
func videoPath(u *url.URL) (typ, code string) {
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) == 2 && (parts[0] == "v" || parts[0] == "e" || parts[0] == "d") && vidaraCode.MatchString(parts[1]) {
		return parts[0], parts[1]
	}
	return "", ""
}

// vidaraMeta is what the step found of a video; a zero number is not known.
type vidaraMeta struct {
	code, title, embed, thumbnail, thumbnailURL string
	thumbWidth, thumbHeight                     int
	duration                                    float64 // seconds, to the millisecond
	width, height, bitrate, averageBitrate      int
	frameRate                                   float64
	codecs, subtitleLang                        string
	subtitles                                   []any
}

func vidaraMetadata(c *call) error {
	mode := cmp.Or(c.setting("Thumbnail"), vidaraDownload)
	if mode != vidaraDownload && mode != vidaraLinkOnly {
		return fmt.Errorf("Thumbnail %q: pick Download or Link Only", mode)
	}
	m := &vidaraMeta{}
	raw := strings.TrimSpace(c.text("Link"))
	own := strings.TrimSpace(c.text("Own Thumbnail"))
	var page string
	var linkErr error
	if raw != "" {
		m.code, page, linkErr = vidaraVideo(raw)
	}
	if m.code != "" {
		m.embed = vidaraEmbed + m.code
	}
	m.thumbnail = own
	switch {
	case raw == "":
		return fmt.Errorf("Link %w: give a Vidara video's link (https://vidara.to/v/...), its <iframe> code or its code", errEmpty)
	case errors.Is(linkErr, errNotVidara):
		return m.notFound(c, linkErr.Error())
	case linkErr != nil:
		return linkErr
	}
	if own != "" {
		c.logf(v1.LogLevel_LOG_LEVEL_INFO, "Own Thumbnail %s is used: Vidara's picture is not downloaded", path.Base(filepath.ToSlash(own)))
	}

	pg, err := vidaraPages(c, m.code, page)
	if err != nil {
		if own == "" {
			return err
		}
		// The picture is had: the rest is extra, so no failure.
		c.logf(v1.LogLevel_LOG_LEVEL_WARNING, "no details from Vidara, only Own Thumbnail: %v", err)
		m.put(c)
		c.fire(Done)
		return nil
	}
	m.title = pg.title
	if pg.gone {
		return m.notFound(c, pg.why)
	}

	st, streamErr := vidaraStream(c, m.code, pg.players)
	if streamErr != nil {
		c.logf(v1.LogLevel_LOG_LEVEL_WARNING, "no stream details (duration, size): %v", streamErr)
	} else {
		m.title = cmp.Or(st.Title, m.title)
		m.subtitles, _ = st.Subtitles.([]any)
		if len(m.subtitles) > 0 {
			m.subtitleLang = st.DefaultSubLang
		}
		if err := vidaraPlaylists(c, m, st.StreamingURL, st.player); err != nil {
			c.logf(v1.LogLevel_LOG_LEVEL_WARNING, "no duration: %v", err)
		}
	}
	pic, why := pg.image, pg.why
	if pic == "" && st.Thumbnail != "" {
		if u, err := url.Parse(st.Thumbnail); err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
			pic, why = u.String(), ""
		}
	}
	stream := streamErr == nil
	if pic == "" && !stream {
		return m.notFound(c, fmt.Sprintf("no thumbnail (%s) and no stream (%v)", why, streamErr))
	}

	name := m.code
	if m.title != "" {
		name = fmt.Sprintf("%s %q", m.code, m.title)
	}
	thumb := ""
	switch {
	case own != "":
		thumb = "Own Thumbnail " + path.Base(filepath.ToSlash(own))
	case pic == "":
		thumb = "no thumbnail"
		c.logf(v1.LogLevel_LOG_LEVEL_WARNING, "no thumbnail: %s", why)
	case mode == vidaraLinkOnly:
		m.thumbnailURL = pic
		thumb = "thumbnail " + pic + ", kept as a link"
	default:
		m.thumbnailURL = pic
		referer := pg.page
		if referer == "" && st.player != "" {
			referer = st.player + "/" // the API's picture, as its player shows it
		}
		size, gone, err := vidaraPicture(c, m, pic, referer)
		switch {
		case err != nil:
			return err
		case gone != "" && !stream:
			return m.notFound(c, gone)
		case gone != "":
			m.thumbnailURL = ""
			c.logf(v1.LogLevel_LOG_LEVEL_WARNING, "no thumbnail: %s", gone)
			thumb = "no thumbnail"
		default:
			thumb = fmt.Sprintf("thumbnail %s (%s), saved for this run as %s", pic, size, path.Base(m.thumbnail))
		}
	}
	c.logf(v1.LogLevel_LOG_LEVEL_INFO, "%s: %s; %s", name, m.streamText(), thumb)
	m.put(c)
	c.fire("Found")
	c.fire(Done)
	return nil
}

// notFound fires Not Found and Done, with no Vidara thumbnail (Own
// Thumbnail stays), saying why as a warning.
func (m *vidaraMeta) notFound(c *call, why string) error {
	m.thumbnailURL = ""
	c.logf(v1.LogLevel_LOG_LEVEL_WARNING, "no Vidara video: %s", why)
	m.put(c)
	c.fire("Not Found")
	c.fire(Done)
	return nil
}

// streamText is the stream's part of the log line: "3:45 (224.84 s),
// 1920x792, 30 fps, 3.0 Mbps, avc1.640028,mp4a.40.2".
func (m *vidaraMeta) streamText() string {
	var parts []string
	if m.duration > 0 {
		parts = append(parts, fmt.Sprintf("%s (%s s)", durationText(m.duration), numberText(m.duration)))
	}
	if m.width > 0 && m.height > 0 {
		parts = append(parts, fmt.Sprintf("%dx%d", m.width, m.height))
	}
	if m.frameRate > 0 {
		parts = append(parts, numberText(m.frameRate)+" fps")
	}
	if m.bitrate > 0 {
		parts = append(parts, fmt.Sprintf("%.1f Mbps", float64(m.bitrate)/1e6))
	}
	if m.codecs != "" {
		parts = append(parts, m.codecs)
	}
	if len(m.subtitles) > 0 {
		parts = append(parts, fmt.Sprintf("%d subtitles", len(m.subtitles)))
	}
	if len(parts) == 0 {
		return "no stream details"
	}
	return strings.Join(parts, ", ")
}

// put sets every output from what was found: a text not known is "", and
// left out of Metadata (the outputs' values under snake_case keys, numbers
// as numbers, plus the thumbnail's size, the average bitrate and the
// subtitles' default language); Subtitles is a list, empty when there are
// none.
func (m *vidaraMeta) put(c *call) {
	meta := map[string]any{}
	add := func(out, key string, v any) {
		s := ""
		switch x := v.(type) {
		case string:
			s = x
		case float64:
			if x > 0 {
				s = numberText(x)
			}
		}
		if out != "" {
			c.outputs[out] = stringValue(s)
		}
		if s != "" {
			meta[key] = v
		}
	}
	minutes, clock := 0.0, ""
	if m.duration > 0 {
		minutes, clock = float64(durationMinutes(m.duration)), durationText(m.duration)
	}
	add("Video ID", "video_id", m.code)
	add("Title", "title", m.title)
	add("Embed URL", "embed_url", m.embed)
	add("Thumbnail", "thumbnail", m.thumbnail)
	add("Thumbnail URL", "thumbnail_url", m.thumbnailURL)
	add("", "thumbnail_width", float64(m.thumbWidth))
	add("", "thumbnail_height", float64(m.thumbHeight))
	add("Duration", "duration", m.duration)
	add("Duration Minutes", "duration_minutes", minutes)
	add("Duration Text", "duration_text", clock)
	add("Width", "width", float64(m.width))
	add("Height", "height", float64(m.height))
	add("Frame Rate", "frame_rate", m.frameRate)
	add("Bitrate", "bitrate", float64(m.bitrate))
	add("", "average_bitrate", float64(m.averageBitrate))
	add("Codecs", "codecs", m.codecs)
	add("", "default_subtitle", m.subtitleLang)
	subs := m.subtitles
	if subs == nil {
		subs = []any{}
	}
	meta["subtitles"] = subs
	if v, err := structpb.NewValue(subs); err == nil {
		c.outputs["Subtitles"] = v
	} else {
		delete(meta, "subtitles") // a value JSON would not hold
		c.outputs["Subtitles"] = structpb.NewListValue(&structpb.ListValue{})
	}
	if v, err := structpb.NewValue(meta); err == nil {
		c.outputs["Metadata"] = v
	}
}

// durationMinutes is a duration in whole minutes as MovieMov's
// duration_minutes takes it: rounded, and at least 1 (29.5 s is 1 minute,
// 224.84 s 4).
func durationMinutes(sec float64) int { return max(1, int(math.Round(sec/60))) }

// durationText is a duration as a player shows it, to the second: 0:30,
// 3:45, 1:02:03.
func durationText(sec float64) string {
	s := int(math.Round(sec))
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// numberText writes n to the millisecond without trailing zeros: 224.84,
// 30, 23.976.
func numberText(n float64) string {
	return strconv.FormatFloat(math.Round(n*1000)/1000, 'f', -1, 64)
}

// vidaraPage is what the video's pages said: the thumbnail (og:image, ""
// when none had one) and the page that named it, the title, the player
// hosts to ask for the stream (vidara.to's last), gone when no page has
// the video (404), and why, what each page lacked.
type vidaraPage struct {
	image, page, title, why string
	players                 []string
	gone                    bool
}

// vidaraPages reads the video's page, the link's own first, then Vidara's
// main site's /v/<code>, until one gives both a thumbnail and the player's
// host. err is when no page answered at all (each failed to load, or
// answered another error than 404).
func vidaraPages(c *call, code, page string) (vidaraPage, error) {
	var pg vidaraPage
	pages := []string{}
	if page != "" {
		pages = append(pages, page)
	}
	if home := vidaraHome + "/v/" + code; page != home {
		pages = append(pages, home)
	}
	var said, players []string
	answered, found := false, false
	for i, p := range pages {
		body, status, final, err := vidaraGet(c.ctx, p, "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8", "", vidaraPageMax)
		switch {
		case errors.Is(err, errTooLarge):
			said = append(said, p+" is over 2 MB, more than a video's page")
			continue
		case err != nil:
			said = append(said, err.Error())
			continue
		case status == http.StatusNotFound || status == http.StatusGone:
			answered = true
			said = append(said, fmt.Sprintf("%s is not there (%d)", p, status))
			continue
		case status != http.StatusOK:
			said = append(said, fmt.Sprintf("%s answered %d %s", p, status, http.StatusText(status)))
			continue
		}
		answered, found = true, true
		image, title := pageMeta(body)
		if strings.HasPrefix(final.Path, "/v/") {
			title = strings.TrimPrefix(title, "Watch ") // the /v/ page's <title> is "Watch <title>"
		}
		pg.title = cmp.Or(pg.title, title)
		players = append(players, pagePlayers(body, final, code)...)
		if image != "" && pg.image == "" {
			if u, err := final.Parse(image); err != nil || (u.Scheme != "http" && u.Scheme != "https") {
				said = append(said, fmt.Sprintf("%s names a thumbnail %q that is not an http link", p, image))
			} else {
				pg.image, pg.page = u.String(), final.String()
				if i > 0 {
					c.logf(v1.LogLevel_LOG_LEVEL_INFO, "read %s instead: %s", p, strings.Join(said, "; "))
				}
			}
		} else if pg.image == "" {
			said = append(said, p+" has no og:image")
		}
		if pg.image != "" && slices.ContainsFunc(players, func(s string) bool { return !homeHost(s) }) {
			break
		}
	}
	if !answered {
		return pg, fmt.Errorf("Vidara's page of %s did not load: %s", code, strings.Join(said, "; "))
	}
	pg.gone = !found
	if pg.image == "" {
		pg.why = strings.Join(said, "; ")
	}
	// The player's hosts, vidara.to's last as its token is its proxy's.
	var away []string
	for _, p := range players {
		if !homeHost(p) && !slices.Contains(away, p) {
			away = append(away, p)
		}
	}
	pg.players = append(away, vidaraHome)
	return pg, nil
}

// homeHost says whether a player's origin is Vidara's main site.
func homeHost(origin string) bool { return origin == vidaraHome || origin == "http://vidara.to" }

// pagePlayers are the hosts (https://<host>) of the video's player that a
// page names: the /v/ page's <iframe> of /e/<code>, the /e/ page's MIRROR,
// and the page's own host when it is the player itself (its script asks
// /api/stream).
func pagePlayers(body []byte, page *url.URL, code string) []string {
	var out []string
	add := func(raw string, needCode bool) {
		raw = html.UnescapeString(strings.TrimSpace(raw))
		if strings.HasPrefix(raw, "//") {
			raw = page.Scheme + ":" + raw
		}
		u, err := page.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return
		}
		if typ, c := videoPath(u); needCode && (typ != "e" || c != code) {
			return
		}
		if o := u.Scheme + "://" + u.Host; !slices.Contains(out, o) {
			out = append(out, o)
		}
	}
	for _, m := range iframeSrc.FindAllSubmatch(body, -1) {
		add(string(m[1])+string(m[2])+string(m[3]), true)
	}
	for _, m := range mirrorVar.FindAllSubmatch(body, -1) {
		add(string(m[1]), false)
	}
	if bytes.Contains(body, []byte("/api/stream")) {
		add("/", false)
	}
	return out
}

// vidaraStreamInfo is the player's /api/stream answer; player is the host
// that gave it.
type vidaraStreamInfo struct {
	Title          string `json:"title"`
	Thumbnail      string `json:"thumbnail"`
	StreamingURL   string `json:"streaming_url"`
	Subtitles      any    `json:"subtitles"`
	DefaultSubLang string `json:"default_sub_lang"`
	Error          string `json:"error"`
	player         string
}

// vidaraStream asks each player host's /api/stream in turn, as the player
// does (a web browser that decodes neither AV1 nor H.265, so the stream is
// H.264), until one gives the stream's link.
func vidaraStream(c *call, code string, players []string) (vidaraStreamInfo, error) {
	req, _ := json.Marshal(map[string]any{"filecode": code, "device": "web", "codecs": []string{}})
	var said []string
	for _, p := range players {
		api := p + "/api/stream"
		body, status, _, err := vidaraDo(c.ctx, http.MethodPost, api, req, map[string]string{
			"Accept": "*/*", "Content-Type": "application/json", "Origin": p, "Referer": p + "/e/" + code}, vidaraPageMax)
		var st vidaraStreamInfo
		switch {
		case errors.Is(err, errTooLarge):
			said = append(said, api+" answered over 2 MB")
			continue
		case err != nil:
			said = append(said, err.Error())
			continue
		case status != http.StatusOK:
			said = append(said, fmt.Sprintf("%s answered %d %s", api, status, http.StatusText(status)))
			continue
		case json.Unmarshal(body, &st) != nil:
			said = append(said, api+" answered no JSON")
			continue
		}
		u, err := url.Parse(st.StreamingURL)
		if st.StreamingURL == "" || err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			why := api + " gave no stream link"
			if st.Error != "" {
				why += " (" + st.Error + ")"
			}
			said = append(said, why)
			continue
		}
		st.player = p
		if len(said) > 0 {
			c.logf(v1.LogLevel_LOG_LEVEL_INFO, "asked %s instead: %s", api, strings.Join(said, "; "))
		}
		return st, nil
	}
	return vidaraStreamInfo{}, errors.New(strings.Join(said, "; "))
}

// hlsStream is one stream of an HLS master playlist.
type hlsStream struct {
	uri                string
	bandwidth, average int
	width, height      int
	frameRate          float64
	codecs             string
}

// vidaraPlaylists reads the master playlist at master, as the player on
// host player would, then its best stream's playlist, into m: the stream's
// details first, so they stay when its playlist fails.
func vidaraPlaylists(c *call, m *vidaraMeta, master, player string) error {
	hdr := map[string]string{"Accept": "*/*", "Origin": player, "Referer": player + "/"}
	text, final, err := hlsGet(c.ctx, master, hdr)
	if err != nil {
		return err
	}
	media := final
	if streams := hlsStreams(final, text); len(streams) > 0 {
		best := streams[0]
		m.width, m.height, m.frameRate = best.width, best.height, best.frameRate
		m.bitrate, m.averageBitrate, m.codecs = best.bandwidth, best.average, best.codecs
		u, err := url.Parse(best.uri)
		if err != nil {
			return fmt.Errorf("the stream's playlist %q is not a link", best.uri)
		}
		if text, media, err = hlsGet(c.ctx, u.String(), hdr); err != nil {
			return err
		}
	}
	sec, whole := hlsDuration(text)
	switch {
	case sec <= 0:
		return fmt.Errorf("the stream's playlist %s has no segments", noQuery(media))
	case !whole:
		return fmt.Errorf("the stream's playlist %s is not a whole video (no #EXT-X-ENDLIST)", noQuery(media))
	}
	m.duration = math.Round(sec*1000) / 1000
	return nil
}

// hlsGet reads an HLS playlist; final is its link after redirects.
func hlsGet(ctx context.Context, raw string, hdr map[string]string) (string, *url.URL, error) {
	body, status, final, err := vidaraDo(ctx, http.MethodGet, raw, nil, hdr, vidaraPageMax)
	shown := raw
	if u, e := url.Parse(raw); e == nil {
		shown = noQuery(u)
	}
	switch {
	case errors.Is(err, errTooLarge):
		return "", nil, fmt.Errorf("the stream's playlist %s is over 2 MB", shown)
	case err != nil:
		return "", nil, fmt.Errorf("the stream's playlist %s did not load: %s", shown, strings.ReplaceAll(err.Error(), raw, shown))
	case status == http.StatusNotFound || status == http.StatusGone:
		return "", nil, fmt.Errorf("the stream's playlist %s is not there (%d)", shown, status)
	case status != http.StatusOK:
		return "", nil, fmt.Errorf("the stream's playlist %s answered %d %s", shown, status, http.StatusText(status))
	case !strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(string(body), "\ufeff")), "#EXTM3U"):
		return "", nil, fmt.Errorf("the stream's playlist %s is not an HLS playlist", shown)
	}
	return string(body), final, nil
}

// noQuery is a link without its query, which holds the stream's token.
func noQuery(u *url.URL) string {
	v := *u
	v.RawQuery, v.Fragment = "", ""
	return v.String()
}

// hlsStreams reads the streams of an HLS master playlist, the best first:
// the most pixels, then the most bits a second. Each stream's link is
// resolved against base; one on base's host without a query of its own
// gets base's (Vidara's token). A media playlist has none.
func hlsStreams(base *url.URL, text string) []hlsStream {
	var out []hlsStream
	var next *hlsStream
	for _, l := range hlsLines(text) {
		switch {
		case strings.HasPrefix(l, "#EXT-X-STREAM-INF:"):
			a := hlsAttrs(strings.TrimPrefix(l, "#EXT-X-STREAM-INF:"))
			s := hlsStream{codecs: a["CODECS"]}
			s.bandwidth, _ = strconv.Atoi(a["BANDWIDTH"])
			s.average, _ = strconv.Atoi(a["AVERAGE-BANDWIDTH"])
			if w, h, ok := strings.Cut(strings.ToLower(a["RESOLUTION"]), "x"); ok {
				s.width, _ = strconv.Atoi(w)
				s.height, _ = strconv.Atoi(h)
			}
			s.frameRate, _ = strconv.ParseFloat(a["FRAME-RATE"], 64)
			next = &s
		case strings.HasPrefix(l, "#"):
		case next != nil:
			u, err := base.Parse(l)
			if err == nil {
				if u.RawQuery == "" && u.Host == base.Host {
					u.RawQuery = base.RawQuery
				}
				next.uri = u.String()
				out = append(out, *next)
			}
			next = nil
		}
	}
	slices.SortStableFunc(out, func(a, b hlsStream) int {
		return cmp.Or(cmp.Compare(b.width*b.height, a.width*a.height), cmp.Compare(b.bandwidth, a.bandwidth))
	})
	return out
}

// hlsDuration adds up a media playlist's #EXTINF, in seconds; whole is
// whether it ends (#EXT-X-ENDLIST), as a video's does, unlike a live one.
func hlsDuration(text string) (sec float64, whole bool) {
	for _, l := range hlsLines(text) {
		switch {
		case strings.HasPrefix(l, "#EXTINF:"):
			d, _, _ := strings.Cut(strings.TrimPrefix(l, "#EXTINF:"), ",")
			if f, err := strconv.ParseFloat(strings.TrimSpace(d), 64); err == nil && f > 0 {
				sec += f
			}
		case l == "#EXT-X-ENDLIST":
			whole = true
		}
	}
	return sec, whole
}

func hlsLines(text string) []string {
	var out []string
	for l := range strings.SplitSeq(strings.ReplaceAll(text, "\r", ""), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// hlsAttrs reads an HLS attribute list: BANDWIDTH=3000000,CODECS="avc1.640028,mp4a.40.2".
func hlsAttrs(s string) map[string]string {
	m := map[string]string{}
	for s != "" {
		k, rest, ok := strings.Cut(s, "=")
		if !ok {
			break
		}
		var v string
		if strings.HasPrefix(rest, `"`) {
			v, rest, _ = strings.Cut(rest[1:], `"`)
			_, rest, _ = strings.Cut(rest, ",")
		} else {
			v, rest, _ = strings.Cut(rest, ",")
		}
		m[strings.ToUpper(strings.TrimSpace(k))] = v
		s = rest
	}
	return m
}

// vidaraPicture downloads the thumbnail at link into the run's folder, with
// its page as the referer, as m's Thumbnail, and gives its size ("320x132,
// 3.8 KB"). A picture that is not there gives gone, why, instead; one that
// is not a JPEG, PNG or WebP, or is over 2 MB, is an error.
func vidaraPicture(c *call, m *vidaraMeta, link, page string) (size, gone string, err error) {
	data, status, _, err := vidaraGet(c.ctx, link, "image/avif,image/webp,image/apng,image/*,*/*;q=0.8", page, vidaraImageMax)
	switch {
	case errors.Is(err, errTooLarge):
		return "", "", fmt.Errorf("the thumbnail %s is over 2 MB, more than MovieMov takes", link)
	case err != nil:
		return "", "", fmt.Errorf("could not load the thumbnail: %w", err)
	case status == http.StatusNotFound || status == http.StatusGone:
		return "", fmt.Sprintf("Vidara names the thumbnail %s, which is not there (%d)", link, status), nil
	case status != http.StatusOK:
		return "", "", fmt.Errorf("the thumbnail %s: Vidara's image server answered %d %s", link, status, http.StatusText(status))
	}
	ext := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}[moviemov.ImageType(data)]
	if ext == "" {
		return "", "", fmt.Errorf("the thumbnail %s is not a JPEG, PNG or WebP picture (%s)", link, http.DetectContentType(data))
	}
	if c.tempDir == nil {
		return "", "", errors.New("this run has no folder of its own for the picture")
	}
	dir, err := c.tempDir()
	if err != nil {
		return "", "", err
	}
	file := filepath.Join(dir, "vidara-"+m.code+ext)
	if err := os.WriteFile(file, data, 0o644); err != nil {
		return "", "", err
	}
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		m.thumbWidth, m.thumbHeight = cfg.Width, cfg.Height
		size = fmt.Sprintf("%dx%d, ", cfg.Width, cfg.Height)
	}
	m.thumbnail = filepath.ToSlash(file)
	return size + sizeText(int64(len(data))), "", nil
}

// errTooLarge is an answer over the most vidaraDo reads.
var errTooLarge = errors.New("too large")

// vidaraGet GETs raw as a browser would, with accept and the referer (when
// given).
func vidaraGet(ctx context.Context, raw, accept, referer string, max int64) (body []byte, status int, final *url.URL, err error) {
	hdr := map[string]string{"Accept": accept}
	if referer != "" {
		hdr["Referer"] = referer
	}
	return vidaraDo(ctx, http.MethodGet, raw, nil, hdr, max)
}

// vidaraDo sends a request as a browser would, with the headers in hdr
// and body (when given), and reads up to max bytes of a 200 answer (more
// is errTooLarge). Another status is given back with no body; final is the
// link the answer came from, after redirects.
func vidaraDo(ctx context.Context, method, raw string, body []byte, hdr map[string]string, max int64) (data []byte, status int, final *url.URL, err error) {
	var in io.Reader
	if body != nil {
		in = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, raw, in)
	if err != nil {
		return nil, 0, nil, err
	}
	req.Header.Set("User-Agent", vidaraUA)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,id;q=0.8")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	res, err := vidaraClient.Do(req)
	if err != nil {
		return nil, 0, nil, err
	}
	defer res.Body.Close()
	final = req.URL
	if res.Request != nil {
		final = res.Request.URL
	}
	if res.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 64<<10))
		return nil, res.StatusCode, final, nil
	}
	if res.ContentLength > max {
		return nil, res.StatusCode, final, errTooLarge
	}
	data, err = io.ReadAll(io.LimitReader(res.Body, max+1))
	if err != nil {
		return nil, res.StatusCode, final, fmt.Errorf("%s %s: %w", method, raw, err)
	}
	if int64(len(data)) > max {
		return nil, res.StatusCode, final, errTooLarge
	}
	return data, res.StatusCode, final, nil
}

// pageMeta reads a page's thumbnail (og:image, else Twitter's) and title
// (og:title, else Twitter's, else <title>), entities decoded.
func pageMeta(body []byte) (image, title string) {
	meta := map[string]string{}
	for _, tag := range metaTag.FindAll(body, -1) {
		attrs := map[string]string{}
		for _, m := range tagAttr.FindAllSubmatch(tag, -1) {
			attrs[strings.ToLower(string(m[1]))] = string(m[2]) + string(m[3]) + string(m[4])
		}
		key := strings.ToLower(cmp.Or(attrs["property"], attrs["name"]))
		content := strings.TrimSpace(html.UnescapeString(attrs["content"]))
		if _, seen := meta[key]; key != "" && content != "" && !seen {
			meta[key] = content
		}
	}
	image = cmp.Or(meta["og:image"], meta["og:image:secure_url"], meta["og:image:url"], meta["twitter:image"], meta["twitter:image:src"])
	title = cmp.Or(meta["og:title"], meta["twitter:title"])
	if title == "" {
		if m := titleTag.FindSubmatch(body); m != nil {
			title = html.UnescapeString(string(m[1]))
		}
	}
	return image, strings.Join(strings.Fields(title), " ")
}
