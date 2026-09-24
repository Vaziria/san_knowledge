package san_youtube

import (
	"errors"
	"fmt"
)

var (
	// ErrChatDisabled reports a stream whose chat is turned off. The stream
	// itself may be live; there is simply nothing to read.
	ErrChatDisabled = errors.New("san_youtube: chat is disabled")

	// ErrMembersOnly reports a video, or a chat, that only the channel's
	// members may see. Reading it would need a logged-in member, which this
	// package deliberately does not do.
	ErrMembersOnly = errors.New("san_youtube: members only")

	// ErrNotLive reports a video that is not a live stream at all, or one that
	// is scheduled and has not opened its chat yet.
	ErrNotLive = errors.New("san_youtube: not a live stream")

	// ErrEnded reports a live stream that is over. Messages ends with it when
	// the chat stops handing out continuations, the way a reader ends with
	// io.EOF, so a caller can tell a finished stream from a failure.
	ErrEnded = errors.New("san_youtube: live stream has ended")

	// ErrVideoUnavailable reports a video that does not exist, is private, or
	// was removed.
	ErrVideoUnavailable = errors.New("san_youtube: video not found or private")

	// ErrNoLiveStream reports a channel that is not live right now. A channel
	// with a scheduled stream gets this too; the error names the scheduled
	// video, which can be opened by its ID.
	ErrNoLiveStream = errors.New("san_youtube: channel has no live stream")

	// ErrChannelNotFound reports a handle or channel ID YouTube does not know.
	ErrChannelNotFound = errors.New("san_youtube: channel not found")

	// ErrInvalidInput is returned by Open for something that is not a video
	// URL, a video ID, a channel URL or an @handle.
	ErrInvalidInput = errors.New("san_youtube: not a YouTube video or channel")

	// ErrConsent reports that YouTube kept showing its cookie consent page
	// after the consent form was submitted.
	ErrConsent = errors.New("san_youtube: stuck on the cookie consent page")

	// ErrRateLimited reports that YouTube kept answering 429, usually by
	// redirecting to google.com/sorry, after every retry. Waiting is the only
	// fix.
	ErrRateLimited = errors.New("san_youtube: rate limited by YouTube")
)

// StatusError is an HTTP status that retrying did not fix, or that is not
// worth retrying: any 4xx other than 429, or a 5xx that outlasted the retries.
type StatusError struct {
	Code int
	// URL is the request URL without its query, which for the chat endpoint
	// would only repeat the API key.
	URL string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("san_youtube: %s: HTTP %d", e.URL, e.Code)
}
