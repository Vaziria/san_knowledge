package san_youtube

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	reVideoID   = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
	reChannelID = regexp.MustCompile(`^UC[A-Za-z0-9_-]{22}$`)
	reHandle    = regexp.MustCompile(`^@[\p{L}\p{N}._·-]{1,100}$`)
)

// target is what Open was asked for: a video, or a channel whose current
// stream is wanted.
type target struct {
	videoID   string
	handle    string // with its @
	channelID string
}

func (t target) String() string {
	switch {
	case t.videoID != "":
		return t.videoID
	case t.handle != "":
		return t.handle
	}
	return t.channelID
}

// parseInput accepts:
//
//	dQw4w9WgXcQ                                  a video ID
//	https://www.youtube.com/watch?v=dQw4w9WgXcQ  also m., music., no scheme
//	https://youtu.be/dQw4w9WgXcQ
//	https://www.youtube.com/live/dQw4w9WgXcQ     also /shorts/, /embed/, /v/
//	https://www.youtube.com/live_chat?v=dQw4w9WgXcQ
//	@name, https://www.youtube.com/@name[/live]  a handle
//	UC…, https://www.youtube.com/channel/UC…     a channel ID
func parseInput(s string) (target, error) {
	s = strings.TrimSpace(s)
	bad := fmt.Errorf("%w: %q", ErrInvalidInput, s)
	switch {
	case s == "":
		return target{}, bad
	case reVideoID.MatchString(s):
		return target{videoID: s}, nil
	case reHandle.MatchString(s):
		return target{handle: s}, nil
	case reChannelID.MatchString(s):
		return target{channelID: s}, nil
	}

	raw := s
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return target{}, bad
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	segs := strings.FieldsFunc(u.Path, func(r rune) bool { return r == '/' })

	switch host {
	case "youtu.be":
		if len(segs) > 0 && reVideoID.MatchString(segs[0]) {
			return target{videoID: segs[0]}, nil
		}
		return target{}, bad
	case "youtube.com", "m.youtube.com", "music.youtube.com", "gaming.youtube.com":
	default:
		return target{}, bad
	}

	if v := u.Query().Get("v"); reVideoID.MatchString(v) {
		return target{videoID: v}, nil
	}
	if len(segs) == 0 {
		return target{}, bad
	}
	switch first := segs[0]; {
	case strings.HasPrefix(first, "@"):
		// Paths are percent-encoded; handles may be written in any script.
		h, err := url.PathUnescape(first)
		if err == nil && reHandle.MatchString(h) {
			return target{handle: h}, nil
		}
	case first == "channel" && len(segs) > 1 && reChannelID.MatchString(segs[1]):
		return target{channelID: segs[1]}, nil
	case (first == "live" || first == "shorts" || first == "embed" || first == "v") && len(segs) > 1 && reVideoID.MatchString(segs[1]):
		return target{videoID: segs[1]}, nil
	}
	return target{}, bad
}
