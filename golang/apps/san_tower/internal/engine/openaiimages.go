package engine

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// Images in (the user, 2026-11-03: "Can we add tool call and image ?", then
// "Yea I follow your recommendation, go ahead": images a request sends go
// to the chat; images the chat makes do not come back). A request's
// image_url parts, a data: URL with the image in base64 or an http(s) link,
// are saved as files in a folder of the request's own, removed when its run
// ends. Each message of Messages keeps its text, with [image n] where an
// image was, and the files of its images as images; a browser Ask sends the
// images of the messages it sends, through San Remote, which attaches them
// in the chat's composer before the prompt.

const (
	// maxImage is the largest image a request may send, as San Remote's
	// attach takes it (ChatGPT's limit).
	maxImage = 20 << 20
	// maxAskImages is the most images an Ask attaches at once, as San
	// Remote takes them.
	maxAskImages = 10
)

// imageFetchTimeout bounds the download of an image a request links.
var imageFetchTimeout = 30 * time.Second

// imageExts are the images the chats take, by type, with their files'
// extension.
var imageExts = map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp", "image/gif": ".gif"}

// imageRef is an image a request's message brings, until it is saved: its
// number in the conversation, the n of [image n], and its URL.
type imageRef struct {
	N   int
	URL string
}

// partImage reads an image part's image_url: {"url": ...} as Chat
// Completions has it, or the URL alone, as the Responses API's input_image.
func partImage(raw json.RawMessage) (string, error) {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s), nil
	}
	var o struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(raw, &o); err != nil || strings.TrimSpace(o.URL) == "" {
		return "", errors.New(`give image_url as {"url": "data:image/png;base64,..."} or {"url": "https://..."}`)
	}
	return strings.TrimSpace(o.URL), nil
}

// imageCount is how many image files a conversation has.
func imageCount(turns []chatTurn) int {
	n := 0
	for _, t := range turns {
		n += len(t.Images)
	}
	return n
}

// hasImages says whether a request's conversation brings images.
func hasImages(turns []chatTurn) bool {
	for _, t := range turns {
		if len(t.refs) > 0 {
			return true
		}
	}
	return false
}

// saveImages saves the images of a request's conversation as files in a new
// folder, image-<n>.<ext>, and gives each message the files of its own:
// a data: URL decoded, an http(s) link downloaded (imageFetchTimeout). Each
// must be a png, jpeg, webp or gif image of at most maxImage. remove takes
// the folder away; it is a no-op when there were no images.
func saveImages(ctx context.Context, turns []chatTurn) (remove func(), err error) {
	remove = func() {}
	if !hasImages(turns) {
		return remove, nil
	}
	dir, err := os.MkdirTemp("", "san_tower-openai-")
	if err != nil {
		return remove, err
	}
	remove = func() { _ = os.RemoveAll(dir) }
	for i := range turns {
		for _, ref := range turns[i].refs {
			data, err := imageData(ctx, ref.URL)
			if err != nil {
				remove()
				return func() {}, fmt.Errorf("image %d: %w", ref.N, err)
			}
			typ := http.DetectContentType(data)
			ext, ok := imageExts[typ]
			if !ok {
				remove()
				return func() {}, fmt.Errorf("image %d is %s: give a png, jpeg, webp or gif image", ref.N, typ)
			}
			path := filepath.Join(dir, fmt.Sprintf("image-%d%s", ref.N, ext))
			if err := os.WriteFile(path, data, 0o600); err != nil {
				remove()
				return func() {}, err
			}
			turns[i].Images = append(turns[i].Images, path)
		}
	}
	return remove, nil
}

// imageData is an image's bytes, from a data: URL or an http(s) link.
func imageData(ctx context.Context, link string) ([]byte, error) {
	if strings.HasPrefix(strings.ToLower(link), "data:") {
		return dataURL(link)
	}
	u, err := url.Parse(link)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("%s: give a data: URL (data:image/png;base64,...) or an http(s) link", clip(link, 60))
	}
	ctx, cancel := context.WithTimeout(ctx, imageFetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", clip(link, 100), res.Status)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxImage+1))
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	if len(data) > maxImage {
		return nil, fmt.Errorf("%s is more than %d MB", clip(link, 100), maxImage>>20)
	}
	return data, nil
}

// dataURL decodes a data: URL's image, which is in base64.
func dataURL(link string) ([]byte, error) {
	head, b64, ok := strings.Cut(link, ",")
	if !ok || !strings.HasSuffix(strings.ToLower(head), ";base64") {
		return nil, errors.New("a data: URL must hold the image in base64: data:image/png;base64,...")
	}
	b64 = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' {
			return -1
		}
		return r
	}, b64)
	if base64.StdEncoding.DecodedLen(len(b64)) > maxImage+3 {
		return nil, fmt.Errorf("it is more than %d MB", maxImage>>20)
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		if data, err = base64.RawStdEncoding.DecodeString(strings.TrimRight(b64, "=")); err != nil {
			return nil, fmt.Errorf("its base64 does not decode: %v", err)
		}
	}
	if len(data) == 0 {
		return nil, errors.New("it is empty")
	}
	if len(data) > maxImage {
		return nil, fmt.Errorf("it is more than %d MB", maxImage>>20)
	}
	return data, nil
}

// withoutImageData is a request's body with the base64 of its data: URLs
// left out ("data:image/png;base64,… (812345 characters)"), as the run's
// Request keeps it: the images are files, and a run's data need not hold
// megabytes of them.
func withoutImageData(body []byte) []byte {
	if !bytes.Contains(body, []byte(`"data:`)) {
		return body
	}
	var v any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if dec.Decode(&v) != nil {
		return body
	}
	var walk func(v any) any
	walk = func(v any) any {
		switch x := v.(type) {
		case map[string]any:
			for k, e := range x {
				x[k] = walk(e)
			}
		case []any:
			for i, e := range x {
				x[i] = walk(e)
			}
		case string:
			if head, rest, ok := strings.Cut(x, ","); ok && strings.HasPrefix(strings.ToLower(head), "data:") && len(rest) > 64 {
				return fmt.Sprintf("%s,… (%d characters)", head, len(rest))
			}
		}
		return v
	}
	b, err := json.Marshal(walk(v))
	if err != nil {
		return body
	}
	return b
}

// askImages are the images an Ask sends with what it asks: those of every
// message into a new chat, which gets the whole conversation (method 1),
// else those of the messages after the last assistant one, the newest turn
// (method 2). At most maxAskImages go, the newest.
func (c *call) askImages(turns []chatTurn, whole bool) []string {
	from := 0
	if !whole {
		for i, t := range turns {
			if t.Role == "assistant" {
				from = i + 1
			}
		}
	}
	var images []string
	for _, t := range turns[from:] {
		images = append(images, t.Images...)
	}
	if len(images) > maxAskImages {
		c.logf(v1.LogLevel_LOG_LEVEL_WARNING, "Images: the conversation has %d: only the newest %d go, as many as the chat takes at once", len(images), maxAskImages)
		images = images[len(images)-maxAskImages:]
	}
	if len(images) > 0 {
		c.logf(v1.LogLevel_LOG_LEVEL_INFO, "Images: %s, attached to the prompt", plural(len(images), "image", "images"))
	}
	return images
}
