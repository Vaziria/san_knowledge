package san_youtube

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"net/url"
	"time"
)

type chatOptions struct {
	pollDelay   time.Duration
	skipHistory bool
}

type ChatOption func(*chatOptions)

// WithPollDelay sets how long to wait before the next poll when YouTube's
// answer does not say. It normally does, and that is always what is waited.
// The default is 5 seconds.
func WithPollDelay(d time.Duration) ChatOption {
	return func(o *chatOptions) {
		if d > 0 {
			o.pollDelay = d
		}
	}
}

// WithoutHistory drops the recent messages a chat starts with, so that
// Messages returns only what is sent after Open. Open reads the first answer
// itself to do this, which costs one request more there and none later.
func WithoutHistory() ChatOption {
	return func(o *chatOptions) { o.skipHistory = true }
}

// Chat is one live stream's chat. It is not safe for concurrent use.
type Chat struct {
	c       *Client
	videoID string
	opts    chatOptions

	cfg     innertube
	referer string
	// continuation is the token for the next request; empty once the
	// stream is over.
	continuation string
	// wait is how long to wait before that request, as the last answer said.
	wait time.Duration
	// pending holds the messages of the last answer that the caller has not
	// taken yet, because it stopped ranging halfway through.
	pending []Message
	seen    seenSet
	// authorDeleted holds the channels whose messages were all deleted and
	// who have not written since: another such deletion for them is a repeat.
	authorDeleted map[string]bool
}

// VideoID is the ID of the stream whose chat this is, which is how a channel
// given to Open was resolved.
func (ch *Chat) VideoID() string { return ch.videoID }

// Messages returns the chat's messages as they arrive, starting with the
// recent history YouTube replays to a new reader:
//
//	for msg, err := range chat.Messages(ctx) {
//		if errors.Is(err, san_youtube.ErrEnded) {
//			break // the stream is over
//		}
//		if err != nil {
//			return err
//		}
//		fmt.Println(msg.Meta().Author.Name)
//	}
//
// A message is never returned twice, although YouTube repeats them. The
// sequence ends after its first error: ErrEnded when the stream is over,
// ctx.Err() once ctx is done, or a request that failed after its retries.
// Stopping the loop early is fine; nothing keeps running. Calling Messages
// again carries on exactly where the last call stopped, with no message lost.
func (ch *Chat) Messages(ctx context.Context) iter.Seq2[Message, error] {
	return func(yield func(Message, error) bool) {
		reloaded := false
		for {
			for len(ch.pending) > 0 {
				m := ch.pending[0]
				ch.pending[0] = nil
				ch.pending = ch.pending[1:]
				if !yield(m, nil) {
					return
				}
			}
			if ch.continuation == "" {
				yield(nil, fmt.Errorf("%w: %s", ErrEnded, ch.videoID))
				return
			}
			if ch.wait > 0 {
				if err := ch.c.opts.sleep(ctx, ch.wait); err != nil {
					yield(nil, err)
					return
				}
				ch.wait = 0
			}

			err := ch.fetch(ctx)
			if se, ok := errors.AsType[*StatusError](err); ok && se.Code < 500 && !reloaded {
				// A token that worked a moment ago is refused, most likely
				// because it expired while the machine slept. Starting again
				// from the page gets a fresh one, and the history it replays
				// is deduplicated like any other.
				reloaded = true
				if err = ch.load(ctx); err == nil {
					continue
				}
			}
			if err != nil {
				yield(nil, err)
				return
			}
			reloaded = false
		}
	}
}

// fetch makes one request and queues what it brings that is new, and the
// continuation and wait for the next one.
func (ch *Chat) fetch(ctx context.Context) error {
	batch, next, err := ch.poll(ctx)
	if err != nil {
		return err
	}
	for _, m := range batch {
		if ch.isNew(m) {
			ch.pending = append(ch.pending, m)
		}
	}
	if next == nil {
		ch.continuation = ""
		return nil
	}
	ch.continuation = next.Continuation
	ch.wait = time.Duration(next.TimeoutMs) * time.Millisecond
	if ch.wait <= 0 {
		ch.wait = ch.opts.pollDelay
	}
	return nil
}

// isNew records m and reports whether the reader has not had it yet.
//
// An author deletion has no ID to go by. It is a repeat if the same author's
// messages were deleted and they have written nothing since; once they write
// again, a new deletion (a ban after a timeout) is news.
func (ch *Chat) isNew(m Message) bool {
	channel := m.Meta().Author.ChannelID
	if _, ok := m.(*AuthorDeletion); ok {
		if channel == "" {
			return true
		}
		if ch.authorDeleted[channel] {
			return false
		}
		if ch.authorDeleted == nil {
			ch.authorDeleted = map[string]bool{}
		}
		ch.authorDeleted[channel] = true
		return true
	}
	if !ch.seen.add(dedupeKey(m)) {
		return false
	}
	if r, ok := m.(*Replacement); ok {
		channel = r.Message.Meta().Author.ChannelID
	}
	delete(ch.authorDeleted, channel)
	return true
}

// load fetches the live_chat page: the request config and the continuation
// that starts "Live chat". A page without one is explained through the watch
// page.
func (ch *Chat) load(ctx context.Context) error {
	u := ch.c.opts.baseURL + "/live_chat?is_popout=1&v=" + url.QueryEscape(ch.videoID)
	resp, err := ch.c.get(ctx, u)
	if err != nil {
		return err
	}
	if resp.status != http.StatusOK {
		return &StatusError{Code: resp.status, URL: stripQuery(u)}
	}
	page := parseChatPage(string(resp.body), ch.c.opts.lang)
	if page.continuation == "" {
		return ch.c.explain(ctx, ch.videoID, page)
	}
	ch.cfg = page.cfg
	ch.referer = u
	ch.continuation = page.continuation
	return nil
}

type rawChatResponse struct {
	ContinuationContents *struct {
		LiveChatContinuation *struct {
			Continuations []rawContinuation `json:"continuations"`
			Actions       []json.RawMessage `json:"actions"`
		} `json:"liveChatContinuation"`
	} `json:"continuationContents"`
}

// poll makes one get_live_chat request. A nil next means the chat gave no
// continuation, which is how it says the stream is over.
func (ch *Chat) poll(ctx context.Context) (batch []Message, next *continuationData, err error) {
	body, err := json.Marshal(struct {
		Context      json.RawMessage `json:"context"`
		Continuation string          `json:"continuation"`
	}{ch.cfg.context, ch.continuation})
	if err != nil {
		return nil, nil, err
	}

	u := ch.c.opts.baseURL + "/youtubei/v1/live_chat/get_live_chat?prettyPrint=false"
	if ch.cfg.apiKey != "" {
		u += "&key=" + url.QueryEscape(ch.cfg.apiKey)
	}
	hdr := http.Header{}
	hdr.Set("Content-Type", "application/json")
	hdr.Set("Origin", ch.c.opts.baseURL)
	hdr.Set("Referer", ch.referer)
	hdr.Set("X-Youtube-Client-Name", "1")
	hdr.Set("X-Youtube-Client-Version", ch.cfg.clientVersion)

	resp, err := ch.c.send(ctx, http.MethodPost, u, body, hdr)
	if err != nil {
		return nil, nil, err
	}
	if resp.status != http.StatusOK {
		return nil, nil, &StatusError{Code: resp.status, URL: stripQuery(u)}
	}

	var r rawChatResponse
	if err := decodeLenient(resp.body, &r); err != nil {
		return nil, nil, fmt.Errorf("san_youtube: reading chat response: %w", err)
	}
	if r.ContinuationContents == nil || r.ContinuationContents.LiveChatContinuation == nil {
		return nil, nil, nil
	}
	lc := r.ContinuationContents.LiveChatContinuation
	return parseActions(lc.Actions, ch.c.opts.now()), firstContinuation(lc.Continuations), nil
}

// dedupeKey says which messages are the same message. A deletion has no ID of
// its own and is keyed by what it deletes. A replacement is keyed by the
// message it brings: YouTube sends one to everyone when a message held for
// review is released, including readers who already got that message in the
// history, and to them it is a repeat. Author deletions are handled by isNew.
func dedupeKey(m Message) string {
	switch m := m.(type) {
	case *Deletion:
		return "deleted:" + m.TargetID
	case *Replacement:
		if id := m.Message.Meta().ID; id != "" {
			return "id:" + id
		}
		return "replaced:" + m.TargetID
	}
	if id := m.Meta().ID; id != "" {
		return "id:" + id
	}
	return ""
}

// seenLimit bounds the dedupe memory. YouTube replays a few hundred messages
// at most, so remembering the last ten thousand is plenty for a stream that
// runs for days.
const seenLimit = 10000

// seenSet remembers the most recent keys, forgetting the oldest first.
type seenSet struct {
	keys  map[string]struct{}
	order []string
	next  int
}

// add records k and reports whether it was new. The empty key is always new.
func (s *seenSet) add(k string) bool {
	if k == "" {
		return true
	}
	if s.keys == nil {
		s.keys = make(map[string]struct{})
	}
	if _, ok := s.keys[k]; ok {
		return false
	}
	if len(s.order) < seenLimit {
		s.order = append(s.order, k)
	} else {
		delete(s.keys, s.order[s.next])
		s.order[s.next] = k
		s.next = (s.next + 1) % seenLimit
	}
	s.keys[k] = struct{}{}
	return true
}
