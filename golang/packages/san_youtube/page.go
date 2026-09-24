package san_youtube

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Fallbacks for when a page does not carry ytcfg. The key is the public one
// every youtube.com visitor's browser sends; the version is what the site
// served on 2026-09-24. Both are read from the page whenever it has them.
const (
	fallbackAPIKey        = "AIzaSyAO_FJ2SlqU8Q4STEHLGCilw_Y9_11qcW8"
	fallbackClientVersion = "2.20260922.01.00"
)

var (
	reAPIKey    = regexp.MustCompile(`"INNERTUBE_API_KEY"\s*:\s*"([^"]+)"`)
	reVersion   = regexp.MustCompile(`"INNERTUBE_CLIENT_VERSION"\s*:\s*"([^"]+)"`)
	reCanonical = regexp.MustCompile(`<link rel="canonical" href="([^"]+)"`)
	reOGURL     = regexp.MustCompile(`<meta property="og:url" content="([^"]+)"`)
)

// innertube is what a chat request needs from the page that preceded it.
type innertube struct {
	apiKey        string
	clientVersion string
	// context is the page's INNERTUBE_CONTEXT, sent back verbatim the way the
	// web client does. It carries the visitor id, language and region the
	// page was served with.
	context json.RawMessage
}

func readInnertube(page string, lang string) innertube {
	it := innertube{apiKey: fallbackAPIKey, clientVersion: fallbackClientVersion}
	if m := reAPIKey.FindStringSubmatch(page); m != nil {
		it.apiKey = m[1]
	}
	if m := reVersion.FindStringSubmatch(page); m != nil {
		it.clientVersion = m[1]
	}
	if ctx := jsonAfter(page, `"INNERTUBE_CONTEXT":`); ctx != nil {
		it.context = ctx
	} else {
		it.context, _ = json.Marshal(map[string]any{
			"client": map[string]string{"clientName": "WEB", "clientVersion": it.clientVersion, "hl": lang},
		})
	}
	return it
}

// jsonAfter decodes the JSON object that follows a marker such as
// `window["ytInitialData"] =`. Decoding one value from a stream stops at the
// object's closing brace, so the script around it does not matter. An error
// page can carry the marker followed by HTML, which is why each occurrence is
// checked for an object.
func jsonAfter(page string, markers ...string) json.RawMessage {
	for _, marker := range markers {
		rest := page
		for {
			i := strings.Index(rest, marker)
			if i < 0 {
				break
			}
			rest = rest[i+len(marker):]
			tail := strings.TrimLeft(rest, " \t\r\n")
			if !strings.HasPrefix(tail, "{") {
				continue
			}
			var raw json.RawMessage
			if json.NewDecoder(strings.NewReader(tail)).Decode(&raw) == nil {
				return raw
			}
		}
	}
	return nil
}

func initialData(page string) json.RawMessage {
	return jsonAfter(page, `window["ytInitialData"] =`, `var ytInitialData =`, `window['ytInitialData'] =`)
}

// rawContinuation is one entry of a "continuations" list. The web client
// knows more kinds; the replay ones mean the stream is over, and like any
// other unknown kind they leave every field here nil.
type rawContinuation struct {
	Invalidation *continuationData `json:"invalidationContinuationData"`
	Timed        *continuationData `json:"timedContinuationData"`
	Reload       *continuationData `json:"reloadContinuationData"`
}

type continuationData struct {
	Continuation string `json:"continuation"`
	TimeoutMs    int    `json:"timeoutMs"`
}

func (c rawContinuation) data() *continuationData {
	for _, d := range []*continuationData{c.Invalidation, c.Timed, c.Reload} {
		if d != nil && d.Continuation != "" {
			return d
		}
	}
	return nil
}

func firstContinuation(list []rawContinuation) *continuationData {
	for _, c := range list {
		if d := c.data(); d != nil {
			return d
		}
	}
	return nil
}

// chatPage is what the live_chat page says.
type chatPage struct {
	cfg innertube
	// continuation starts "Live chat" rather than the filtered "Top chat" the
	// page opens with. Empty when the page has no chat.
	continuation string
	// notice is what YouTube shows instead of a chat, e.g. "Chat is disabled
	// for this live stream."
	notice string
	// hasData is false for the error page an unknown video gets.
	hasData bool
}

type rawChatPage struct {
	Contents struct {
		LiveChatRenderer *struct {
			Continuations []rawContinuation `json:"continuations"`
			Header        struct {
				LiveChatHeaderRenderer struct {
					ViewSelector struct {
						SortFilterSubMenuRenderer struct {
							SubMenuItems []struct {
								Title        string          `json:"title"`
								Continuation rawContinuation `json:"continuation"`
							} `json:"subMenuItems"`
						} `json:"sortFilterSubMenuRenderer"`
					} `json:"viewSelector"`
				} `json:"liveChatHeaderRenderer"`
			} `json:"header"`
		} `json:"liveChatRenderer"`
		MessageRenderer *struct {
			Text rawText `json:"text"`
		} `json:"messageRenderer"`
	} `json:"contents"`
}

func parseChatPage(page string, lang string) chatPage {
	p := chatPage{cfg: readInnertube(page, lang)}
	data := initialData(page)
	if data == nil {
		return p
	}
	var d rawChatPage
	if decodeLenient(data, &d) != nil {
		return p
	}
	p.hasData = true
	if m := d.Contents.MessageRenderer; m != nil {
		p.notice = m.Text.String()
	}
	lc := d.Contents.LiveChatRenderer
	if lc == nil {
		return p
	}

	// The view selector offers "Top chat" then "Live chat". The title is
	// matched first because the page is requested in English; the position
	// is the fallback for any other language.
	items := lc.Header.LiveChatHeaderRenderer.ViewSelector.SortFilterSubMenuRenderer.SubMenuItems
	for _, it := range items {
		if strings.EqualFold(it.Title, "Live chat") {
			if d := it.Continuation.data(); d != nil {
				p.continuation = d.Continuation
				return p
			}
		}
	}
	if len(items) >= 2 {
		if d := items[1].Continuation.data(); d != nil {
			p.continuation = d.Continuation
			return p
		}
	}
	// A chat with no selector has only one view, and its own continuation
	// is the one to follow.
	if d := firstContinuation(lc.Continuations); d != nil {
		p.continuation = d.Continuation
	}
	return p
}

// playerInfo is the part of a watch page's ytInitialPlayerResponse that says
// what kind of video it is.
type playerInfo struct {
	PlayabilityStatus struct {
		Status      string          `json:"status"`
		Reason      string          `json:"reason"`
		Messages    []string        `json:"messages"`
		ErrorScreen json.RawMessage `json:"errorScreen"`
	} `json:"playabilityStatus"`
	VideoDetails struct {
		VideoID       string `json:"videoId"`
		IsLive        bool   `json:"isLive"`
		IsUpcoming    bool   `json:"isUpcoming"`
		IsLiveContent bool   `json:"isLiveContent"`
	} `json:"videoDetails"`
	Microformat struct {
		PlayerMicroformatRenderer struct {
			LiveBroadcastDetails *struct {
				IsLiveNow      bool   `json:"isLiveNow"`
				StartTimestamp string `json:"startTimestamp"`
				EndTimestamp   string `json:"endTimestamp"`
			} `json:"liveBroadcastDetails"`
		} `json:"playerMicroformatRenderer"`
	} `json:"microformat"`
}

func parsePlayer(page string) *playerInfo {
	data := jsonAfter(page, `var ytInitialPlayerResponse =`, `window["ytInitialPlayerResponse"] =`, `ytInitialPlayerResponse =`)
	if data == nil {
		return nil
	}
	var p playerInfo
	if decodeLenient(data, &p) != nil {
		return nil
	}
	return &p
}

func (p *playerInfo) liveNow() bool {
	lbd := p.Microformat.PlayerMicroformatRenderer.LiveBroadcastDetails
	return p.VideoDetails.IsLive || (lbd != nil && lbd.IsLiveNow)
}

// why is YouTube's own explanation of the playability status.
func (p *playerInfo) why() string {
	parts := []string{p.PlayabilityStatus.Reason}
	parts = append(parts, p.PlayabilityStatus.Messages...)
	var out []string
	for _, s := range parts {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, " ")
}

// membersOnly recognises a members-only video. The purchase offer renderer
// in the error screen is the structural sign; the wording is the fallback.
func (p *playerInfo) membersOnly() bool {
	if strings.Contains(string(p.PlayabilityStatus.ErrorScreen), "YpcOfferRenderer") {
		return true
	}
	why := strings.ToLower(p.why())
	return strings.Contains(why, "members-only") || strings.Contains(why, "join this channel")
}

// classify explains why a video's chat page had no chat, using the watch
// page's player response. It is only reached on that failure path, so the
// happy path costs one page fetch, not two.
func classify(videoID string, page chatPage, p *playerInfo) error {
	notice := page.notice
	if notice == "" {
		notice = "no chat on the live_chat page"
	}
	if p == nil {
		if !page.hasData {
			// Neither page said anything recognisable, which is what a bot
			// check looks like. Better no sentinel than a wrong one.
			return fmt.Errorf("san_youtube: %s: YouTube sent neither a chat nor video details", videoID)
		}
		return fmt.Errorf("%w: %s: %s", ErrChatDisabled, videoID, notice)
	}

	status := p.PlayabilityStatus.Status
	lbd := p.Microformat.PlayerMicroformatRenderer.LiveBroadcastDetails
	why := p.why()
	switch {
	case p.membersOnly():
		return fmt.Errorf("%w: %s: %s", ErrMembersOnly, videoID, why)
	case status == "ERROR",
		status == "LOGIN_REQUIRED" && strings.Contains(strings.ToLower(why), "private"),
		p.VideoDetails.VideoID == "" && status != "OK":
		return fmt.Errorf("%w: %s: %s", ErrVideoUnavailable, videoID, why)
	case !p.VideoDetails.IsLiveContent && lbd == nil:
		return fmt.Errorf("%w: %s is a regular video", ErrNotLive, videoID)
	case lbd != nil && lbd.EndTimestamp != "",
		!p.liveNow() && !p.VideoDetails.IsUpcoming:
		return fmt.Errorf("%w: %s", ErrEnded, videoID)
	case p.VideoDetails.IsUpcoming && !p.liveNow():
		return fmt.Errorf("%w: %s has not started: %s", ErrNotLive, videoID, notice)
	}
	return fmt.Errorf("%w: %s: %s", ErrChatDisabled, videoID, notice)
}

// liveFromChannelPage reads a channel's /live page. YouTube serves the watch
// page of the stream that is on, with a canonical link to it, or the channel
// page when nothing is.
func liveFromChannelPage(page string) (videoID string, p *playerInfo) {
	link := ""
	if m := reCanonical.FindStringSubmatch(page); m != nil {
		link = m[1]
	} else if m := reOGURL.FindStringSubmatch(page); m != nil {
		link = m[1]
	}
	if t, err := parseInput(link); err == nil {
		videoID = t.videoID
	}
	if videoID == "" {
		return "", nil
	}
	return videoID, parsePlayer(page)
}
