package gmaps

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Social is one social media profile of a place.
type Social struct {
	// Site is instagram, facebook, tiktok, x, youtube, linkedin, whatsapp,
	// telegram or threads.
	Site string `json:"site"`
	URL  string `json:"url"`
}

// socialHosts are the sites a link is a profile of, by host without www.
// or m.
var socialHosts = map[string]string{
	"instagram.com": "instagram",
	"facebook.com":  "facebook", "fb.com": "facebook", "web.facebook.com": "facebook",
	"tiktok.com": "tiktok",
	"x.com":      "x", "twitter.com": "x",
	"youtube.com":  "youtube",
	"linkedin.com": "linkedin", "id.linkedin.com": "linkedin",
	"wa.me": "whatsapp", "api.whatsapp.com": "whatsapp", "wa.link": "whatsapp",
	"t.me":        "telegram",
	"threads.net": "threads", "threads.com": "threads",
}

// notProfiles are the first path parts of a site's links that are not a
// profile: share buttons, posts, the site's own pages.
var notProfiles = map[string][]string{
	"instagram": {"p", "reel", "reels", "explore", "accounts", "stories", "about", "developer", "legal", "direct"},
	"facebook":  {"sharer", "sharer.php", "share", "share.php", "dialog", "plugins", "tr", "login", "help", "policies", "privacy", "events", "watch", "photo", "photo.php", "story.php", "permalink.php", "hashtag", "groups"},
	"tiktok":    {"embed", "share", "tag", "music", "discover", "legal", "about", "login", "video"},
	"x":         {"intent", "share", "home", "hashtag", "search", "i", "login", "tos", "privacy", "explore"},
	"youtube":   {"watch", "embed", "shorts", "results", "playlist", "redirect", "feed", "t", "about", "iframe_api"},
	"linkedin":  {"shareArticle", "sharing", "feed", "posts", "pulse", "legal", "help", "login"},
	"telegram":  {"share", "joinchat", "s", "iv"},
	"threads":   {"intent", "t", "login"},
}

// socialOf says which site link is a profile on, and the link without what
// does not name the profile. ok is false for any other link.
func socialOf(link string) (s Social, ok bool) {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return Social{}, false
	}
	host := strings.ToLower(u.Hostname())
	host = strings.TrimPrefix(strings.TrimPrefix(host, "www."), "m.")
	site, ok := socialHosts[host]
	if !ok {
		return Social{}, false
	}
	parts := strings.FieldsFunc(u.Path, func(r rune) bool { return r == '/' })
	switch site {
	case "whatsapp":
		if host == "api.whatsapp.com" {
			// Only its digits: a "+" in a query reads as a space.
			phone := strings.Map(func(r rune) rune {
				if r >= '0' && r <= '9' {
					return r
				}
				return -1
			}, u.Query().Get("phone"))
			if phone == "" {
				return Social{}, false
			}
			return Social{"whatsapp", "https://wa.me/" + phone}, true
		}
		if len(parts) == 0 {
			return Social{}, false
		}
		return Social{"whatsapp", "https://" + host + "/" + parts[0]}, true
	case "youtube":
		// A channel: /@name, /channel/<id>, /c/<name>, /user/<name>.
		if len(parts) == 0 {
			return Social{}, false
		}
		switch {
		case strings.HasPrefix(parts[0], "@"):
			parts = parts[:1]
		case (parts[0] == "channel" || parts[0] == "c" || parts[0] == "user") && len(parts) > 1:
			parts = parts[:2]
		default:
			return Social{}, false
		}
	case "linkedin":
		// /company/<name>, /in/<name>, /school/<name>.
		if len(parts) < 2 || (parts[0] != "company" && parts[0] != "in" && parts[0] != "school" && parts[0] != "showcase") {
			return Social{}, false
		}
		parts = parts[:2]
	case "facebook":
		if len(parts) == 0 {
			return Social{}, false
		}
		if parts[0] == "profile.php" {
			id := u.Query().Get("id")
			if id == "" {
				return Social{}, false
			}
			return Social{"facebook", "https://www.facebook.com/profile.php?id=" + id}, true
		}
		// A page's link may be /pages/<name>/<id> or /people/<name>/<id>.
		if parts[0] == "pages" || parts[0] == "people" {
			if len(parts) < 2 {
				return Social{}, false
			}
			parts = parts[:min(len(parts), 3)]
			break
		}
		parts = parts[:1]
	case "tiktok", "threads":
		// /@name.
		if len(parts) == 0 || !strings.HasPrefix(parts[0], "@") {
			return Social{}, false
		}
		parts = parts[:1]
	default:
		if len(parts) == 0 {
			return Social{}, false
		}
		parts = parts[:1]
	}
	for _, n := range notProfiles[site] {
		if strings.EqualFold(parts[0], n) {
			return Social{}, false
		}
	}
	canon := map[string]string{
		"instagram": "www.instagram.com", "facebook": "www.facebook.com", "tiktok": "www.tiktok.com", "x": "x.com",
		"youtube": "www.youtube.com", "linkedin": "www.linkedin.com", "telegram": "t.me", "threads": "www.threads.net",
	}[site]
	return Social{site, "https://" + canon + "/" + strings.Join(parts, "/")}, true
}

// linkIn finds the links in a page, in its markup and in the JSON of its
// scripts (a link page such as Linktree keeps them there, with escaped
// slashes).
var linkIn = regexp.MustCompile(`https?://[^\s"'<>\\)\]]+`)

// socialIn lists the social profiles a page links to, each once, in the
// order the page has them.
func socialIn(page string) []Social {
	page = strings.NewReplacer(`\/`, `/`, `\u002F`, `/`, `\u002f`, `/`, `&amp;`, `&`).Replace(page)
	var out []Social
	seen := map[string]bool{}
	for _, l := range linkIn.FindAllString(page, -1) {
		if s, ok := socialOf(l); ok && !seen[strings.ToLower(s.URL)] {
			seen[strings.ToLower(s.URL)] = true
			out = append(out, s)
		}
	}
	return out
}

// maxPage is how much of a website is read for its links.
const maxPage = 2 << 20

// fetchSocial reads a website's page, and lists the social profiles it
// links to.
func fetchSocial(ctx context.Context, hc *http.Client, site string) ([]Social, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, site, nil)
	if err != nil {
		return nil, err
	}
	// As a browser asks: some sites answer a client they do not know with
	// nothing, or a 403.
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,*/*;q=0.8")
	req.Header.Set("Accept-Language", "id,en;q=0.8")
	res, err := hc.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("no answer in 20s")
		}
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		return nil, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, maxPage))
	if err != nil {
		return nil, err
	}
	return socialIn(string(b)), nil
}

// addSocial adds the profiles of add that list does not have yet.
func addSocial(list []Social, add ...Social) []Social {
	for _, s := range add {
		dup := false
		for _, t := range list {
			if strings.EqualFold(t.URL, s.URL) {
				dup = true
				break
			}
		}
		if !dup {
			list = append(list, s)
		}
	}
	return list
}
