package engine

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"html"
	"image"
	_ "image/jpeg" // Width and Height of a JPEG thumbnail
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/moviemov"
)

// Vidara Thumbnail (the user, 2026-11-08: "can we have get thumbnail url
// from vidara video ?, use this sample https://vidara.to/v/8SdwOuKAAk92t",
// then "Yes" to: Mode Download, the picture saved and uploaded to MovieMov
// as a file, so it outlives Vidara's domain changes, or External Link; and
// the Telegram form's thumbnail question skipped means Vidara's thumbnail).
// A video's page, /v/<code> and the player's /e/<code>, carries its
// thumbnail as og:image, with no login, so the tower reads it over plain
// HTTP, without San Remote or a browser. The picture is on an image server
// whose random-looking name changes as Vidara moves domains: a kept link may
// break, a downloaded file does not. Vidara's thumbnails are small, 320x132
// for the sample, not 16:9.

// The Mode setting's options; Download is the default.
const (
	vidaraDownload = "Download"
	vidaraExternal = "External Link"
)

var vidaraThumbnailKind = &kind{
	StepKind: &v1.StepKind{
		Kind: "vidara-thumbnail", Title: "Vidara Thumbnail", Category: "Social",
		Group:       "Vidara",
		Description: "Reads a Vidara video's thumbnail from its page (og:image), without a browser or San Remote. Link is the video's link (https://vidara.to/v/<code>, the player's /e/<code>, on vidara.so or another of Vidara's domains), its <iframe> code, or its code; when the link's own site does not answer, https://vidara.to/v/<code> is read. Mode Download (the default) saves the picture in the run's own folder, removed when the run ends: Thumbnail is its file, which links straight into MovieMov Upload Movie's Thumbnail, so the site keeps its own copy. External Link gives only Thumbnail URL, the picture on Vidara's image server, whose name changes when Vidara moves domains. Vidara's thumbnails are small (320x132), not 16:9. Found fires when the video has one, Not Found when it has none (a link that is not Vidara's, a video that is gone, a page without og:image: logged as a warning), then Done either way. Own Thumbnail, when given (a Telegram Form's photo answer), is passed on as Thumbnail and Vidara is not read: only Done fires. Title is the video's title, Video ID its code, Embed URL https://vidara.to/e/<code>, Width and Height the downloaded picture's size in pixels.",
		Outputs: []*v1.Socket{sock("Done", Event), sock("Found", Event), sock("Not Found", Event),
			sock("Thumbnail", Text), sock("Thumbnail URL", Text), sock("Title", Text), sock("Video ID", Text), sock("Embed URL", Text),
			sock("Width", Text), sock("Height", Text)},
		Settings: []*v1.Setting{{Name: "Mode", Options: []string{vidaraDownload, vidaraExternal}}},
		Inputs: []*v1.Socket{sock("Event", Event),
			hint("https://vidara.to/v/8SdwOuKAAk92t, the <iframe> code or 8SdwOuKAAk92t", field("Link")),
			hint("Empty: Vidara's; a file of your own (a Telegram Form's photo) is used instead", field("Own Thumbnail"))},
	},
	run: vidaraThumbnail,
}

// vidaraClient reads Vidara's pages and pictures: each answer within 20
// seconds, as a page is some 20-50 KB and a thumbnail some 4 KB. Tests
// point its transport at a fake.
var vidaraClient = &http.Client{Timeout: 20 * time.Second}

// vidaraHome is Vidara's main site, whose /v/<code> is read when the
// link's own site does not answer; vidaraEmbed is the player's link the
// step gives, as MovieMov's Embed URL takes it.
const (
	vidaraHome  = "https://vidara.to"
	vidaraEmbed = "https://vidara.to/e/"
)

// vidaraUA is a browser's User-Agent: the pages are a browser's.
const vidaraUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36"

// The most read of a page and of a picture: a page is far less; the
// picture is held to what MovieMov takes (2 MB), Vidara's being some 4 KB.
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
)

// errNotVidara is a link that is no Vidara video's, such as another
// player's: there is no Vidara thumbnail (Not Found), rather than a mistake.
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
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) == 2 && (parts[0] == "v" || parts[0] == "e" || parts[0] == "d") && vidaraCode.MatchString(parts[1]) {
		return parts[1], (&url.URL{Scheme: u.Scheme, Host: u.Host, Path: "/" + parts[0] + "/" + parts[1]}).String(), nil
	}
	return "", "", fmt.Errorf("Link %s %w", u.Redacted(), errNotVidara)
}

func vidaraThumbnail(c *call) error {
	mode := cmp.Or(c.setting("Mode"), vidaraDownload)
	if mode != vidaraDownload && mode != vidaraExternal {
		return fmt.Errorf("Mode %q: pick Download or External Link", mode)
	}
	for _, o := range []string{"Thumbnail", "Thumbnail URL", "Title", "Video ID", "Embed URL", "Width", "Height"} {
		c.outputs[o] = stringValue("")
	}
	raw := strings.TrimSpace(c.text("Link"))
	var code, page string
	var linkErr error
	if raw != "" {
		code, page, linkErr = vidaraVideo(raw)
	}
	if code != "" {
		c.outputs["Video ID"] = stringValue(code)
		c.outputs["Embed URL"] = stringValue(vidaraEmbed + code)
	}
	// A picture of the user's own wins, and Vidara is not asked.
	if own := strings.TrimSpace(c.text("Own Thumbnail")); own != "" {
		c.outputs["Thumbnail"] = stringValue(own)
		c.logf(v1.LogLevel_LOG_LEVEL_INFO, "Own Thumbnail %s is used: Vidara is not read", path.Base(filepath.ToSlash(own)))
		c.fire(Done)
		return nil
	}
	switch {
	case raw == "":
		return fmt.Errorf("Link %w: give a Vidara video's link (https://vidara.to/v/...), its <iframe> code or its code", errEmpty)
	case errors.Is(linkErr, errNotVidara):
		return vidaraNotFound(c, linkErr.Error()+": no Vidara thumbnail")
	case linkErr != nil:
		return linkErr
	}

	th, why, err := vidaraFind(c, code, page)
	if err != nil {
		return err
	}
	c.outputs["Title"] = stringValue(th.title)
	if th.image == "" {
		return vidaraNotFound(c, why)
	}
	c.outputs["Thumbnail URL"] = stringValue(th.image)
	name := code
	if th.title != "" {
		name = fmt.Sprintf("%s %q", code, th.title)
	}
	if mode == vidaraExternal {
		c.logf(v1.LogLevel_LOG_LEVEL_INFO, "%s: thumbnail %s, kept as a link", name, th.image)
		c.fire("Found")
		c.fire(Done)
		return nil
	}

	data, status, _, err := vidaraGet(c.ctx, th.image, "image/avif,image/webp,image/apng,image/*,*/*;q=0.8", th.page, vidaraImageMax)
	switch {
	case errors.Is(err, errTooLarge):
		return fmt.Errorf("the thumbnail %s is over 2 MB, more than MovieMov takes", th.image)
	case err != nil:
		return fmt.Errorf("could not load the thumbnail: %w", err)
	case status == http.StatusNotFound || status == http.StatusGone:
		return vidaraNotFound(c, fmt.Sprintf("its page names the thumbnail %s, which is not there (%d)", th.image, status))
	case status != http.StatusOK:
		return fmt.Errorf("the thumbnail %s: Vidara's image server answered %d %s", th.image, status, http.StatusText(status))
	}
	ext := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}[moviemov.ImageType(data)]
	if ext == "" {
		return fmt.Errorf("the thumbnail %s is not a JPEG, PNG or WebP picture (%s)", th.image, http.DetectContentType(data))
	}
	if c.tempDir == nil {
		return errors.New("this run has no folder of its own for the picture")
	}
	dir, err := c.tempDir()
	if err != nil {
		return err
	}
	file := filepath.Join(dir, "vidara-"+code+ext)
	if err := os.WriteFile(file, data, 0o644); err != nil {
		return err
	}
	size := ""
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		c.outputs["Width"] = stringValue(strconv.Itoa(cfg.Width))
		c.outputs["Height"] = stringValue(strconv.Itoa(cfg.Height))
		size = fmt.Sprintf("%dx%d, ", cfg.Width, cfg.Height)
	}
	c.outputs["Thumbnail"] = stringValue(filepath.ToSlash(file))
	c.logf(v1.LogLevel_LOG_LEVEL_INFO, "%s: thumbnail %s (%s%s), saved for this run as %s", name, th.image, size, sizeText(int64(len(data))), filepath.Base(file))
	c.fire("Found")
	c.fire(Done)
	return nil
}

// vidaraNotFound fires Not Found and Done, with no thumbnail, saying why as
// a warning.
func vidaraNotFound(c *call, why string) error {
	c.outputs["Thumbnail"] = stringValue("")
	c.outputs["Thumbnail URL"] = stringValue("")
	c.logf(v1.LogLevel_LOG_LEVEL_WARNING, "no thumbnail: %s", why)
	c.fire("Not Found")
	c.fire(Done)
	return nil
}

// vidaraThumb is what a video's page says: the thumbnail (og:image, "" when
// it has none), the video's title and the page read.
type vidaraThumb struct{ image, title, page string }

// vidaraFind reads the video's page, the link's own first, then Vidara's
// main site's /v/<code>, until one names a thumbnail. Without one, why says
// what each page answered; err is when no page answered at all (each failed
// to load, or answered with another error than 404).
func vidaraFind(c *call, code, page string) (th vidaraThumb, why string, err error) {
	pages := []string{}
	if page != "" {
		pages = append(pages, page)
	}
	if home := vidaraHome + "/v/" + code; page != home {
		pages = append(pages, home)
	}
	var said []string
	answered := false
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
		answered = true
		image, title := pageMeta(body)
		if strings.HasPrefix(final.Path, "/v/") {
			title = strings.TrimPrefix(title, "Watch ") // the /v/ page's <title> is "Watch <title>"
		}
		th.title = cmp.Or(th.title, title)
		if image == "" {
			said = append(said, p+" has no og:image")
			continue
		}
		u, err := final.Parse(image)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			said = append(said, fmt.Sprintf("%s names a thumbnail %q that is not an http link", p, image))
			continue
		}
		if i > 0 {
			c.logf(v1.LogLevel_LOG_LEVEL_INFO, "read %s instead: %s", p, strings.Join(said, "; "))
		}
		th.image, th.page = u.String(), final.String()
		return th, "", nil
	}
	if !answered {
		return th, "", fmt.Errorf("Vidara's page of %s did not load: %s", code, strings.Join(said, "; "))
	}
	return th, strings.Join(said, "; "), nil
}

// errTooLarge is an answer over the most vidaraGet reads.
var errTooLarge = errors.New("too large")

// vidaraGet GETs raw as a browser would, with accept and the referer (when
// given), and reads up to max bytes of a 200 answer (more is errTooLarge).
// Another status is given back with no body; final is the link the answer
// came from, after redirects.
func vidaraGet(ctx context.Context, raw, accept, referer string, max int64) (body []byte, status int, final *url.URL, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, 0, nil, err
	}
	req.Header.Set("User-Agent", vidaraUA)
	req.Header.Set("Accept", accept)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,id;q=0.8")
	if referer != "" {
		req.Header.Set("Referer", referer)
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
	body, err = io.ReadAll(io.LimitReader(res.Body, max+1))
	if err != nil {
		return nil, res.StatusCode, final, fmt.Errorf("GET %s: %w", raw, err)
	}
	if int64(len(body)) > max {
		return nil, res.StatusCode, final, errTooLarge
	}
	return body, res.StatusCode, final, nil
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
