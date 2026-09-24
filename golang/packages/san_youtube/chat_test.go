package san_youtube

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeYouTube serves testdata pages and scripted chat answers, and records
// what it was asked. Pages are keyed by path, plus "?v=" for video pages;
// chat answers by continuation. Each key holds a queue of replies whose last
// one repeats.
type fakeYouTube struct {
	t *testing.T

	mu    sync.Mutex
	pages map[string][]reply
	polls map[string][]reply
	gets  []recordedGet
	posts []pollRequest
	// hang makes a chat request for that continuation wait until the client
	// gives up on it.
	hang map[string]bool

	// consent puts the consent page in front of every page until the
	// consent form has been posted; consentForever never lets it go.
	consent        bool
	consentForever bool
	consentForm    url.Values
}

type reply struct {
	status int
	file   string
	body   string
}

func page(file string) reply { return reply{status: http.StatusOK, file: file} }
func status(code int) reply  { return reply{status: code} }

type recordedGet struct {
	key    string
	header http.Header
}

type pollRequest struct {
	continuation string
	key          string
	context      map[string]any
	header       http.Header
}

func newFake(t *testing.T) *fakeYouTube {
	return &fakeYouTube{
		t:     t,
		pages: map[string][]reply{},
		polls: map[string][]reply{},
		hang:  map[string]bool{},
	}
}

func (f *fakeYouTube) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/consent/m":
		b := strings.ReplaceAll(readFile(f.t, "consent.html"), "https://consent.youtube.com", "http://"+r.Host+"/consent")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(b))
		return

	case r.URL.Path == "/consent/save" && r.Method == http.MethodPost:
		r.ParseForm()
		f.mu.Lock()
		f.consentForm = r.PostForm
		f.mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: "SOCS", Value: "CAESfake-reject", Path: "/"})
		http.SetCookie(w, &http.Cookie{Name: "CONSENT_DONE", Value: "1", Path: "/"})
		http.Redirect(w, r, r.PostForm.Get("continue"), http.StatusSeeOther)
		return

	case r.Method == http.MethodGet:
		if f.consent {
			ck, err := r.Cookie("CONSENT_DONE")
			if err != nil || ck.Value != "1" || f.consentForever {
				http.Redirect(w, r, "/consent/m?continue="+url.QueryEscape(r.URL.String()), http.StatusFound)
				return
			}
		}
		key := r.URL.Path
		if v := r.URL.Query().Get("v"); v != "" {
			key += "?v=" + v
		}
		f.mu.Lock()
		f.gets = append(f.gets, recordedGet{key: key, header: r.Header.Clone()})
		f.mu.Unlock()
		f.write(w, f.next(f.pages, key))

	case r.Method == http.MethodPost && r.URL.Path == "/youtubei/v1/live_chat/get_live_chat":
		var body struct {
			Context      map[string]any `json:"context"`
			Continuation string         `json:"continuation"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			f.t.Errorf("chat request body: %v", err)
		}
		f.mu.Lock()
		f.posts = append(f.posts, pollRequest{
			continuation: body.Continuation,
			key:          r.URL.Query().Get("key"),
			context:      body.Context,
			header:       r.Header.Clone(),
		})
		hang := f.hang[body.Continuation]
		f.mu.Unlock()
		if hang {
			<-r.Context().Done()
			return
		}
		f.write(w, f.next(f.polls, body.Continuation))

	default:
		http.NotFound(w, r)
	}
}

func (f *fakeYouTube) next(m map[string][]reply, key string) reply {
	f.mu.Lock()
	defer f.mu.Unlock()
	rs := m[key]
	if len(rs) == 0 {
		return reply{status: http.StatusNotFound, body: "no fake for " + key}
	}
	if len(rs) > 1 {
		m[key] = rs[1:]
	}
	return rs[0]
}

func (f *fakeYouTube) write(w http.ResponseWriter, rep reply) {
	body := rep.body
	if rep.file != "" {
		body = readFile(f.t, rep.file)
	}
	if strings.HasSuffix(rep.file, ".json") || strings.HasPrefix(body, "{") {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	} else {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
	w.WriteHeader(rep.status)
	w.Write([]byte(body))
}

func (f *fakeYouTube) getKeys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var keys []string
	for _, g := range f.gets {
		keys = append(keys, g.key)
	}
	return keys
}

func (f *fakeYouTube) pollRequests() []pollRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.posts)
}

func (f *fakeYouTube) continuations() []string {
	var out []string
	for _, p := range f.pollRequests() {
		out = append(out, p.continuation)
	}
	return out
}

func readFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// newTestClient points a client at f. Waits are recorded rather than waited,
// so a test runs in milliseconds whatever the poll intervals and backoffs.
func newTestClient(t *testing.T, f *fakeYouTube, opt ...Option) (*Client, *[]time.Duration) {
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	sleeps := &[]time.Duration{}
	opts := []Option{
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
		func(o *options) {
			o.sleep = func(ctx context.Context, d time.Duration) error {
				*sleeps = append(*sleeps, d)
				return ctx.Err()
			}
			o.now = func() time.Time { return testNow }
		},
	}
	return New(append(opts, opt...)...), sleeps
}

// liveFake serves a chat whose polls walk through every continuation kind:
// history, a timed continuation that repeats a message, a reload, the history
// the reload replays, and the end.
func liveFake(t *testing.T) *fakeYouTube {
	f := newFake(t)
	f.pages["/live_chat?v=LIVEVIDEO01"] = []reply{page("live_chat.html")}
	f.polls["live-chat-reload"] = []reply{page("get_live_chat_1.json")}
	f.polls["live-chat-2"] = []reply{page("get_live_chat_2.json")}
	f.polls["live-chat-3"] = []reply{page("get_live_chat_3.json")}
	f.polls["live-chat-reload-2"] = []reply{page("get_live_chat_4.json")}
	f.polls["live-chat-5"] = []reply{page("get_live_chat_ended.json")}
	return f
}

// readAll ranges over the chat until the sequence ends, which it always does
// with an error.
func readAll(t *testing.T, ctx context.Context, ch *Chat) ([]Message, error) {
	t.Helper()
	var msgs []Message
	for m, err := range ch.Messages(ctx) {
		if err != nil {
			return msgs, err
		}
		if m == nil {
			t.Fatal("nil message with nil error")
		}
		msgs = append(msgs, m)
	}
	t.Fatal("Messages ended without an error")
	return nil, nil
}

func idsOf(msgs []Message) []string {
	var out []string
	for _, m := range msgs {
		switch m := m.(type) {
		case *Deletion:
			out = append(out, "del:"+m.TargetID)
		case *Replacement:
			out = append(out, "rep:"+m.Message.Meta().ID)
		default:
			out = append(out, m.Meta().ID)
		}
	}
	return out
}

func wantStrings(t *testing.T, what string, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("%s:\n got %q\nwant %q", what, got, want)
	}
}

func wantDurations(t *testing.T, got, want []time.Duration) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("waits = %v, want %v", got, want)
	}
}

func TestReadsLiveChatToTheEnd(t *testing.T) {
	f := liveFake(t)
	c, sleeps := newTestClient(t, f)

	ch, err := c.Open(t.Context(), "https://www.youtube.com/watch?v=LIVEVIDEO01")
	if err != nil {
		t.Fatalf("Open = %v", err)
	}
	if ch.VideoID() != "LIVEVIDEO01" {
		t.Errorf("VideoID = %q", ch.VideoID())
	}

	msgs, err := readAll(t, t.Context(), ch)
	if !errors.Is(err, ErrEnded) {
		t.Fatalf("final error = %v, want ErrEnded", err)
	}

	// History, then new messages. h-3 comes again in the second answer and
	// everything comes again after the reload; each is returned once. The
	// second answer also replaces h-2 with itself, which is how YouTube
	// releases a held message to everyone, and is dropped; its release of
	// the placeholder as r-1 is new and kept, and not repeated when the
	// reload replays r-1 as an ordinary message. The Top chat message on the
	// page, the placeholder and the system notice are never returned.
	wantStrings(t, "messages", idsOf(msgs), []string{
		"del:h-0", "h-1", "h-2", "h-3", "h-4",
		"n-1", "n-2", "rep:r-1",
		"n-3", "del:n-1",
		"n-4",
	})
	// "Live chat", not "Top chat", and each answer's continuation after it.
	wantStrings(t, "continuations", f.continuations(), []string{
		"live-chat-reload", "live-chat-2", "live-chat-3", "live-chat-reload-2", "live-chat-5",
	})
	// Each answer's timeoutMs; the reload has none and gets the fallback.
	wantDurations(t, *sleeps, []time.Duration{10 * time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second})
	// A chat that works costs one page, not the watch page as well.
	wantStrings(t, "pages", f.getKeys(), []string{"/live_chat?v=LIVEVIDEO01"})

	if sc, ok := msgs[4].(*SuperChat); !ok || sc.Amount.Value != 50000 || sc.Amount.Currency != "IDR" {
		t.Errorf("msgs[4] = %#v, want the IDR 50,000 Super Chat", msgs[4])
	}
	if d := msgs[0].(*Deletion); !d.Time.Equal(testNow) {
		t.Errorf("deletion time = %v, want when the answer arrived", d.Time)
	}
}

// The chat request is built from the page, the way the browser builds it.
func TestChatRequestsCarryThePageConfig(t *testing.T) {
	f := liveFake(t)
	c, _ := newTestClient(t, f)
	ch, err := c.Open(t.Context(), "LIVEVIDEO01")
	if err != nil {
		t.Fatal(err)
	}
	readAll(t, t.Context(), ch)

	get := f.gets[0].header
	if ua := get.Get("User-Agent"); !strings.Contains(ua, "Chrome/") {
		t.Errorf("page User-Agent = %q, want a browser's", ua)
	}
	if al := get.Get("Accept-Language"); !strings.HasPrefix(al, "en-US") {
		t.Errorf("page Accept-Language = %q", al)
	}
	if ck := get.Get("Cookie"); !strings.Contains(ck, "SOCS=CAI") {
		t.Errorf("page Cookie = %q, want the consent answer", ck)
	}

	p := f.pollRequests()[0]
	if p.key != "AIzaSyFAKE-key-read-from-the-page" {
		t.Errorf("key = %q, want the page's", p.key)
	}
	h := p.header
	for name, want := range map[string]string{
		"Content-Type":             "application/json",
		"X-Youtube-Client-Name":    "1",
		"X-Youtube-Client-Version": "2.20990101.00.00",
	} {
		if got := h.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if ref := h.Get("Referer"); !strings.HasSuffix(ref, "/live_chat?is_popout=1&v=LIVEVIDEO01") {
		t.Errorf("Referer = %q", ref)
	}
	if !strings.Contains(h.Get("User-Agent"), "Chrome/") || !strings.Contains(h.Get("Cookie"), "SOCS=CAI") {
		t.Errorf("chat request headers = %v", h)
	}
	client, _ := p.context["client"].(map[string]any)
	if client["visitorData"] != "CgtGYWtlVmlzaXRvcg%3D%3D" || client["clientVersion"] != "2.20990101.00.00" || client["hl"] != "en" {
		t.Errorf("context = %v, want the page's INNERTUBE_CONTEXT", p.context)
	}
}

// Without ytcfg on the page, the fallback key and version are used and a
// minimal context is built.
func TestFallbackConfig(t *testing.T) {
	f := newFake(t)
	f.pages["/live_chat?v=LIVEVIDEO01"] = []reply{{status: 200, body: `<html><body><script>window["ytInitialData"] = {"contents":{"liveChatRenderer":{"header":{"liveChatHeaderRenderer":{"viewSelector":{"sortFilterSubMenuRenderer":{"subMenuItems":[{"title":"Top chat","continuation":{"reloadContinuationData":{"continuation":"top"}}},{"title":"Live chat","continuation":{"reloadContinuationData":{"continuation":"live-chat-reload"}}}]}}}}}}};</script></body></html>`}}
	f.polls["live-chat-reload"] = []reply{page("get_live_chat_ended.json")}
	c, _ := newTestClient(t, f, WithLanguage("id"))

	ch, err := c.Open(t.Context(), "LIVEVIDEO01")
	if err != nil {
		t.Fatal(err)
	}
	readAll(t, t.Context(), ch)

	p := f.pollRequests()[0]
	if p.key != fallbackAPIKey || p.header.Get("X-Youtube-Client-Version") != fallbackClientVersion {
		t.Errorf("key %q version %q, want the fallbacks", p.key, p.header.Get("X-Youtube-Client-Version"))
	}
	client, _ := p.context["client"].(map[string]any)
	if client["clientName"] != "WEB" || client["clientVersion"] != fallbackClientVersion || client["hl"] != "id" {
		t.Errorf("context = %v", p.context)
	}
	if al := p.header.Get("Accept-Language"); al != "id,en;q=0.8" {
		t.Errorf("Accept-Language = %q", al)
	}
}

func TestWithoutHistory(t *testing.T) {
	f := liveFake(t)
	c, _ := newTestClient(t, f)
	ch, err := c.Open(t.Context(), "LIVEVIDEO01", WithoutHistory())
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := readAll(t, t.Context(), ch)
	if !errors.Is(err, ErrEnded) {
		t.Fatalf("final error = %v", err)
	}
	// The replay after the reload is still history the reader has seen, and
	// n-4 is still new.
	wantStrings(t, "messages", idsOf(msgs), []string{"n-1", "n-2", "rep:r-1", "n-3", "del:n-1", "n-4"})
}

func TestPollDelayFallback(t *testing.T) {
	f := liveFake(t)
	c, sleeps := newTestClient(t, f)
	ch, err := c.Open(t.Context(), "LIVEVIDEO01", WithPollDelay(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	readAll(t, t.Context(), ch)
	wantDurations(t, *sleeps, []time.Duration{10 * time.Second, 2 * time.Second, 3 * time.Second, 10 * time.Second})
}

// An answer with messages and no continuation delivers the messages, then
// ends. So does every later call.
func TestEndsAfterTheLastMessages(t *testing.T) {
	f := newFake(t)
	f.pages["/live_chat?v=LIVEVIDEO01"] = []reply{page("live_chat.html")}
	f.polls["live-chat-reload"] = []reply{{status: 200, body: `{"continuationContents":{"liveChatContinuation":{"actions":[` +
		`{"addChatItemAction":{"item":{"liveChatTextMessageRenderer":{"message":{"runs":[{"text":"terima kasih"}]},"authorName":{"simpleText":"@fakechannel"},"id":"last-1","timestampUsec":"1790188500000000"}}}}` +
		`]}}}`}}
	c, _ := newTestClient(t, f)
	ch, err := c.Open(t.Context(), "LIVEVIDEO01")
	if err != nil {
		t.Fatal(err)
	}

	msgs, err := readAll(t, t.Context(), ch)
	if !errors.Is(err, ErrEnded) {
		t.Fatalf("final error = %v", err)
	}
	wantStrings(t, "messages", idsOf(msgs), []string{"last-1"})

	msgs, err = readAll(t, t.Context(), ch)
	if len(msgs) != 0 || !errors.Is(err, ErrEnded) {
		t.Errorf("second read = %v, %v; want nothing and ErrEnded", msgs, err)
	}
	if n := len(f.pollRequests()); n != 1 {
		t.Errorf("%d chat requests, want 1", n)
	}
}

func TestOpenExplainsMissingChat(t *testing.T) {
	tests := []struct {
		name  string
		chat  string
		watch reply
		want  error
		text  string
	}{
		{"chat disabled on a live stream", "live_chat_disabled.html", page("watch_live.html"), ErrChatDisabled, "Chat is disabled for this live stream."},
		{"a regular video", "live_chat_disabled.html", page("watch_regular.html"), ErrNotLive, "regular video"},
		{"a stream that ended", "live_chat_disabled.html", page("watch_ended.html"), ErrEnded, "LIVEVIDEO01"},
		{"a stream that has not started", "live_chat_disabled.html", page("watch_upcoming.html"), ErrNotLive, "has not started"},
		{"no such video", "live_chat_unavailable.html", page("watch_unavailable.html"), ErrVideoUnavailable, "This video is unavailable"},
		{"a private video", "live_chat_unavailable.html", page("watch_private.html"), ErrVideoUnavailable, "Private video"},
		{"a members-only stream", "live_chat_disabled.html", page("watch_members_only.html"), ErrMembersOnly, "members-only"},
		{"watch page failing leaves the chat page's word", "live_chat_disabled.html", status(500), ErrChatDisabled, "Chat is disabled"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			f.pages["/live_chat?v=LIVEVIDEO01"] = []reply{page(tt.chat)}
			f.pages["/watch?v=LIVEVIDEO01"] = []reply{tt.watch}
			c, _ := newTestClient(t, f, WithRetries(1))

			ch, err := c.Open(t.Context(), "LIVEVIDEO01")
			if !errors.Is(err, tt.want) {
				t.Fatalf("Open = %v, %v; want %v", ch, err, tt.want)
			}
			if !strings.Contains(err.Error(), tt.text) {
				t.Errorf("error %q does not say %q", err, tt.text)
			}
			if n := len(f.pollRequests()); n != 0 {
				t.Errorf("%d chat requests for a chat that is not there", n)
			}
		})
	}
}

func TestOpenChannel(t *testing.T) {
	tests := []struct {
		name  string
		input string
		path  string
		live  reply
		want  error
		text  string
	}{
		{"handle that is live", "@fakechannel", "/@fakechannel/live", page("watch_live.html"), nil, ""},
		{"channel URL that is live", "https://www.youtube.com/channel/UCfakeChannel00000000001", "/channel/UCfakeChannel00000000001/live", page("watch_live.html"), nil, ""},
		{"handle that is not live", "@fakeoffline", "/@fakeoffline/live", page("channel_offline.html"), ErrNoLiveStream, "@fakeoffline"},
		{"handle with a scheduled stream", "@fakechannel", "/@fakechannel/live", page("watch_upcoming.html"), ErrNoLiveStream, "UPCOMING001"},
		{"handle whose last stream ended", "@fakechannel", "/@fakechannel/live", page("watch_ended.html"), ErrNoLiveStream, "ENDEDVIDEO1"},
		{"handle nobody has", "@nobody", "/@nobody/live", status(404), ErrChannelNotFound, "@nobody"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			f.pages[tt.path] = []reply{tt.live}
			f.pages["/live_chat?v=LIVEVIDEO01"] = []reply{page("live_chat.html")}
			c, _ := newTestClient(t, f)

			ch, err := c.Open(t.Context(), tt.input)
			if tt.want == nil {
				if err != nil {
					t.Fatalf("Open = %v", err)
				}
				if ch.VideoID() != "LIVEVIDEO01" {
					t.Errorf("VideoID = %q", ch.VideoID())
				}
				wantStrings(t, "pages", f.getKeys(), []string{tt.path, "/live_chat?v=LIVEVIDEO01"})
				return
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("Open = %v, want %v", err, tt.want)
			}
			if !strings.Contains(err.Error(), tt.text) {
				t.Errorf("error %q does not say %q", err, tt.text)
			}
		})
	}
}

func TestOpenRejectsInput(t *testing.T) {
	f := newFake(t)
	c, _ := newTestClient(t, f)
	if _, err := c.Open(t.Context(), "https://example.com/watch?v=LIVEVIDEO01"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("Open = %v, want ErrInvalidInput", err)
	}
	if len(f.getKeys()) != 0 {
		t.Errorf("requests were made for bad input: %v", f.getKeys())
	}
}

func TestRetriesServerErrors(t *testing.T) {
	f := liveFake(t)
	f.polls["live-chat-reload"] = []reply{status(503), status(502), page("get_live_chat_1.json")}
	f.polls["live-chat-2"] = []reply{page("get_live_chat_ended.json")}
	c, sleeps := newTestClient(t, f)

	ch, err := c.Open(t.Context(), "LIVEVIDEO01")
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := readAll(t, t.Context(), ch)
	if !errors.Is(err, ErrEnded) {
		t.Fatalf("final error = %v", err)
	}
	if len(msgs) != 5 {
		t.Errorf("got %d messages, want the 5 of the first answer", len(msgs))
	}
	wantDurations(t, *sleeps, []time.Duration{1 * time.Second, 2 * time.Second, 10 * time.Second})
}

// flakyTransport fails the first chat requests the way a dropped connection
// does.
type flakyTransport struct {
	rt    http.RoundTripper
	mu    sync.Mutex
	fails int
}

func (ft *flakyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	ft.mu.Lock()
	fail := r.Method == http.MethodPost && ft.fails > 0
	if fail {
		ft.fails--
	}
	ft.mu.Unlock()
	if fail {
		return nil, errors.New("connection reset by fake peer")
	}
	return ft.rt.RoundTrip(r)
}

func TestRetriesNetworkErrors(t *testing.T) {
	f := liveFake(t)
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	var sleeps []time.Duration
	c := New(
		WithBaseURL(srv.URL),
		WithHTTPClient(&http.Client{Transport: &flakyTransport{rt: srv.Client().Transport, fails: 3}}),
		func(o *options) {
			o.sleep = func(ctx context.Context, d time.Duration) error { sleeps = append(sleeps, d); return nil }
		},
	)

	ch, err := c.Open(t.Context(), "LIVEVIDEO01")
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := readAll(t, t.Context(), ch)
	if !errors.Is(err, ErrEnded) || len(msgs) != 11 {
		t.Fatalf("got %d messages and %v, want 11 and ErrEnded", len(msgs), err)
	}
	wantDurations(t, sleeps[:3], []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second})
}

func TestRetriesGiveUp(t *testing.T) {
	f := liveFake(t)
	f.polls["live-chat-reload"] = []reply{status(503)}
	c, sleeps := newTestClient(t, f, WithRetries(2))
	ch, err := c.Open(t.Context(), "LIVEVIDEO01")
	if err != nil {
		t.Fatal(err)
	}

	_, err = readAll(t, t.Context(), ch)
	se, ok := errors.AsType[*StatusError](err)
	if !ok || se.Code != 503 || strings.Contains(se.URL, "key=") {
		t.Fatalf("error = %v, want a StatusError 503 without the query", err)
	}
	if n := len(f.pollRequests()); n != 3 {
		t.Errorf("%d requests, want 1 and 2 retries", n)
	}
	wantDurations(t, *sleeps, []time.Duration{1 * time.Second, 2 * time.Second})
}

func TestRateLimited(t *testing.T) {
	f := newFake(t)
	f.pages["/live_chat?v=LIVEVIDEO01"] = []reply{status(429)}
	c, _ := newTestClient(t, f, WithRetries(1))
	if _, err := c.Open(t.Context(), "LIVEVIDEO01"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("Open = %v, want ErrRateLimited", err)
	}
	if n := len(f.getKeys()); n != 2 {
		t.Errorf("%d requests, want 2", n)
	}
}

// A 4xx is an answer, not a hiccup: it is not retried.
func TestClientErrorsAreNotRetried(t *testing.T) {
	f := newFake(t)
	f.pages["/live_chat?v=LIVEVIDEO01"] = []reply{status(403)}
	c, sleeps := newTestClient(t, f)
	_, err := c.Open(t.Context(), "LIVEVIDEO01")
	if se, ok := errors.AsType[*StatusError](err); !ok || se.Code != 403 {
		t.Fatalf("Open = %v, want a StatusError 403", err)
	}
	if len(f.getKeys()) != 1 || len(*sleeps) != 0 {
		t.Errorf("%d requests and waits %v, want one request and no waits", len(f.getKeys()), *sleeps)
	}
}

// A continuation that stops working, as it does after a laptop sleeps, is
// replaced by starting over from the page once.
func TestReloadsAfterARefusedContinuation(t *testing.T) {
	f := liveFake(t)
	f.pages["/live_chat?v=LIVEVIDEO01"] = []reply{page("live_chat.html"), page("live_chat.html")}
	f.polls["live-chat-reload"] = []reply{page("get_live_chat_1.json"), page("get_live_chat_4.json")}
	f.polls["live-chat-2"] = []reply{status(400)}
	c, sleeps := newTestClient(t, f)

	ch, err := c.Open(t.Context(), "LIVEVIDEO01")
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := readAll(t, t.Context(), ch)
	if !errors.Is(err, ErrEnded) {
		t.Fatalf("final error = %v", err)
	}
	// After the reload, the replayed history is deduplicated; what was
	// missed while the token was refused is not.
	wantStrings(t, "messages", idsOf(msgs), []string{
		"del:h-0", "h-1", "h-2", "h-3", "h-4",
		"n-2", "n-3", "del:n-1", "r-1", "n-4",
	})
	wantStrings(t, "pages", f.getKeys(), []string{"/live_chat?v=LIVEVIDEO01", "/live_chat?v=LIVEVIDEO01"})
	wantStrings(t, "continuations", f.continuations(), []string{"live-chat-reload", "live-chat-2", "live-chat-reload", "live-chat-5"})
	wantDurations(t, *sleeps, []time.Duration{10 * time.Second, 10 * time.Second})
}

func TestReloadIsTriedOnce(t *testing.T) {
	f := liveFake(t)
	f.pages["/live_chat?v=LIVEVIDEO01"] = []reply{page("live_chat.html")}
	f.polls["live-chat-reload"] = []reply{page("get_live_chat_1.json"), status(404)}
	f.polls["live-chat-2"] = []reply{status(400)}
	c, _ := newTestClient(t, f)

	ch, err := c.Open(t.Context(), "LIVEVIDEO01")
	if err != nil {
		t.Fatal(err)
	}
	_, err = readAll(t, t.Context(), ch)
	if se, ok := errors.AsType[*StatusError](err); !ok || se.Code != 404 {
		t.Fatalf("final error = %v, want the StatusError 404 after one reload", err)
	}
	if n := len(f.getKeys()); n != 2 {
		t.Errorf("%d page loads, want 2", n)
	}
}

// When the reload finds the stream over, that is what the reader hears.
func TestReloadFindsTheStreamEnded(t *testing.T) {
	f := liveFake(t)
	f.pages["/live_chat?v=LIVEVIDEO01"] = []reply{page("live_chat.html"), page("live_chat_disabled.html")}
	f.pages["/watch?v=LIVEVIDEO01"] = []reply{page("watch_ended.html")}
	f.polls["live-chat-2"] = []reply{status(403)}
	c, _ := newTestClient(t, f)

	ch, err := c.Open(t.Context(), "LIVEVIDEO01")
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := readAll(t, t.Context(), ch)
	if !errors.Is(err, ErrEnded) || len(msgs) != 5 {
		t.Fatalf("got %d messages and %v, want 5 and ErrEnded", len(msgs), err)
	}
}

func TestConsentPage(t *testing.T) {
	f := liveFake(t)
	f.consent = true
	c, _ := newTestClient(t, f)

	ch, err := c.Open(t.Context(), "LIVEVIDEO01")
	if err != nil {
		t.Fatalf("Open = %v", err)
	}
	if _, err := readAll(t, t.Context(), ch); !errors.Is(err, ErrEnded) {
		t.Fatalf("final error = %v", err)
	}

	f.mu.Lock()
	form := f.consentForm
	f.mu.Unlock()
	if form.Get("set_eom") != "true" || form.Get("set_ytc") != "" || form.Get("pc") != "yt" {
		t.Errorf("posted %v, want the Reject all form", form)
	}
	ck := f.pollRequests()[0].header.Get("Cookie")
	if !strings.Contains(ck, "CONSENT_DONE=1") || !strings.Contains(ck, "SOCS=CAESfake-reject") {
		t.Errorf("chat request Cookie = %q, want the cookies the form set", ck)
	}
}

func TestConsentPageThatStays(t *testing.T) {
	f := liveFake(t)
	f.consent, f.consentForever = true, true
	c, _ := newTestClient(t, f)
	if _, err := c.Open(t.Context(), "LIVEVIDEO01"); !errors.Is(err, ErrConsent) {
		t.Fatalf("Open = %v, want ErrConsent", err)
	}
}

// Cancelling while waiting between polls ends the sequence with the
// context's error, at once.
func TestCancelWhileWaiting(t *testing.T) {
	f := newFake(t)
	f.pages["/live_chat?v=LIVEVIDEO01"] = []reply{page("live_chat.html")}
	f.polls["live-chat-reload"] = []reply{{status: 200, body: `{"continuationContents":{"liveChatContinuation":{"continuations":[{"timedContinuationData":{"timeoutMs":60000,"continuation":"later"}}],"actions":[` +
		`{"addChatItemAction":{"item":{"liveChatTextMessageRenderer":{"message":{"runs":[{"text":"halo"}]},"authorName":{"simpleText":"@viewer-one"},"id":"only-1","timestampUsec":"1790188500000000"}}}}` +
		`]}}}`}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	// The real sleep this time.
	c := New(WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ch, err := c.Open(ctx, "LIVEVIDEO01")
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	var got []string
	var final error
	for m, err := range ch.Messages(ctx) {
		if err != nil {
			final = err
			break
		}
		got = append(got, m.Meta().ID)
		cancel()
	}
	if !errors.Is(final, context.Canceled) {
		t.Fatalf("final error = %v, want context.Canceled", final)
	}
	if time.Since(start) > 5*time.Second {
		t.Errorf("took %v to stop", time.Since(start))
	}
	wantStrings(t, "messages", got, []string{"only-1"})
}

// Cancelling during a request aborts the request.
func TestCancelDuringRequest(t *testing.T) {
	f := liveFake(t)
	f.hang["live-chat-reload"] = true
	c, _ := newTestClient(t, f)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ch, err := c.Open(ctx, "LIVEVIDEO01")
	if err != nil {
		t.Fatal(err)
	}
	time.AfterFunc(100*time.Millisecond, cancel)

	start := time.Now()
	_, err = readAll(t, ctx, ch)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("final error = %v, want context.Canceled", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Errorf("took %v to stop", time.Since(start))
	}
}

func TestCancelBeforeOpen(t *testing.T) {
	f := liveFake(t)
	c, _ := newTestClient(t, f)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := c.Open(ctx, "LIVEVIDEO01"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Open = %v, want context.Canceled", err)
	}
}

// Leaving the loop leaves nothing running and asks for nothing more.
func TestBreakStopsPolling(t *testing.T) {
	f := liveFake(t)
	c, _ := newTestClient(t, f)
	ch, err := c.Open(t.Context(), "LIVEVIDEO01")
	if err != nil {
		t.Fatal(err)
	}
	for _, err := range ch.Messages(t.Context()) {
		if err != nil {
			t.Fatal(err)
		}
		break
	}
	time.Sleep(50 * time.Millisecond)
	if n := len(f.pollRequests()); n != 1 {
		t.Errorf("%d chat requests after breaking out of the first answer, want 1", n)
	}

	// The next call carries on with the rest of that answer: nothing is
	// lost and nothing is repeated.
	msgs, err := readAll(t, t.Context(), ch)
	if !errors.Is(err, ErrEnded) {
		t.Fatalf("final error = %v", err)
	}
	wantStrings(t, "messages after the break", idsOf(msgs), []string{
		"h-1", "h-2", "h-3", "h-4",
		"n-1", "n-2", "rep:r-1",
		"n-3", "del:n-1",
		"n-4",
	})
}

func TestParseChatPageSelectors(t *testing.T) {
	wrap := func(renderer string) string {
		return `<script>window["ytInitialData"] = {"contents":{"liveChatRenderer":` + renderer + `}};</script>`
	}
	tests := []struct {
		name, page, want string
	}{
		{"by title", wrap(`{"header":{"liveChatHeaderRenderer":{"viewSelector":{"sortFilterSubMenuRenderer":{"subMenuItems":[{"title":"Live chat","continuation":{"reloadContinuationData":{"continuation":"live"}}},{"title":"Top chat","continuation":{"reloadContinuationData":{"continuation":"top"}}}]}}}}}`), "live"},
		{"by position in another language", wrap(`{"header":{"liveChatHeaderRenderer":{"viewSelector":{"sortFilterSubMenuRenderer":{"subMenuItems":[{"title":"Chat teratas","continuation":{"reloadContinuationData":{"continuation":"top"}}},{"title":"Chat langsung","continuation":{"reloadContinuationData":{"continuation":"live"}}}]}}}}}`), "live"},
		{"no selector", wrap(`{"continuations":[{"invalidationContinuationData":{"timeoutMs":10000,"continuation":"only"}}]}`), "only"},
		{"replay only", wrap(`{"continuations":[{"liveChatReplayContinuationData":{"continuation":"replay"}}]}`), ""},
	}
	for _, tt := range tests {
		if got := parseChatPage(tt.page, "en").continuation; got != tt.want {
			t.Errorf("%s: continuation = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestSeenSetIsBounded(t *testing.T) {
	var s seenSet
	for i := range seenLimit + 10 {
		if !s.add("k" + strconv.Itoa(i)) {
			t.Fatalf("key %d reported as seen", i)
		}
	}
	if len(s.keys) != seenLimit {
		t.Errorf("remembers %d keys, want %d", len(s.keys), seenLimit)
	}
	if !s.add("k0") {
		t.Error("the oldest key is still remembered")
	}
	if s.add("k" + strconv.Itoa(seenLimit+9)) {
		t.Error("the newest key was forgotten")
	}
	if !s.add("") || !s.add("") {
		t.Error("the empty key must always be new")
	}
}

// When the chat page shows nothing and the watch page cannot be read either,
// the failure is reported, not a guess at why.
func TestOpenReportsWatchPageFailure(t *testing.T) {
	f := newFake(t)
	f.pages["/live_chat?v=LIVEVIDEO01"] = []reply{page("live_chat_unavailable.html")}
	f.pages["/watch?v=LIVEVIDEO01"] = []reply{status(500)}
	c, _ := newTestClient(t, f, WithRetries(1))

	_, err := c.Open(t.Context(), "LIVEVIDEO01")
	if se, ok := errors.AsType[*StatusError](err); !ok || se.Code != 500 {
		t.Fatalf("Open = %v, want the watch page's StatusError 500", err)
	}
	if errors.Is(err, ErrVideoUnavailable) {
		t.Errorf("Open = %v, which guesses the video is unavailable", err)
	}
}

// With WithoutHistory, Open reads the history itself, so what is sent between
// Open and the first Messages call comes from the next answer and is kept.
func TestWithoutHistoryReadsHistoryInOpen(t *testing.T) {
	f := liveFake(t)
	c, sleeps := newTestClient(t, f)
	ch, err := c.Open(t.Context(), "LIVEVIDEO01", WithoutHistory())
	if err != nil {
		t.Fatal(err)
	}
	wantStrings(t, "requests made by Open", f.continuations(), []string{"live-chat-reload"})

	msgs, err := readAll(t, t.Context(), ch)
	if !errors.Is(err, ErrEnded) {
		t.Fatalf("final error = %v", err)
	}
	if len(msgs) == 0 || msgs[0].Meta().ID != "n-1" {
		t.Errorf("first message = %v, want n-1 from the answer after the history", idsOf(msgs))
	}
	wantDurations(t, *sleeps, []time.Duration{10 * time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second})
}

// An author deletion repeated in a replay is dropped; a new one after the
// author wrote again is not.
func TestAuthorDeletionDedupe(t *testing.T) {
	text := func(id string) string {
		return `{"addChatItemAction":{"item":{"liveChatTextMessageRenderer":{"message":{"runs":[{"text":"x"}]},"authorName":{"simpleText":"@spammer"},"authorExternalChannelId":"UCfakeViewer000000000066","id":"` + id + `","timestampUsec":"1790188500000000"}}}}`
	}
	ban := `{"markChatItemsByAuthorAsDeletedAction":{"deletedStateMessage":{"runs":[{"text":"[message deleted]"}]},"externalChannelId":"UCfakeViewer000000000066"}}`
	answer := func(next string, actions ...string) reply {
		conts := `[]`
		if next != "" {
			conts = `[{"timedContinuationData":{"timeoutMs":1000,"continuation":"` + next + `"}}]`
		}
		return reply{status: 200, body: `{"continuationContents":{"liveChatContinuation":{"continuations":` + conts + `,"actions":[` + strings.Join(actions, ",") + `]}}}`}
	}

	f := newFake(t)
	f.pages["/live_chat?v=LIVEVIDEO01"] = []reply{page("live_chat.html")}
	f.polls["live-chat-reload"] = []reply{answer("a2", text("s-1"), ban)}
	f.polls["a2"] = []reply{answer("a3", text("s-1"), ban)}      // a replay: both repeats
	f.polls["a3"] = []reply{answer("a4", text("s-2"), ban, ban)} // wrote again, banned again once
	f.polls["a4"] = []reply{answer("")}
	c, _ := newTestClient(t, f)

	ch, err := c.Open(t.Context(), "LIVEVIDEO01")
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := readAll(t, t.Context(), ch)
	if !errors.Is(err, ErrEnded) {
		t.Fatalf("final error = %v", err)
	}
	var got []string
	for _, m := range msgs {
		got = append(got, string(m.Kind())+":"+m.Meta().ID)
	}
	wantStrings(t, "messages", got, []string{"text:s-1", "author_deletion:", "text:s-2", "author_deletion:"})
}

// noRequestTransport answers like a RoundTripper that does not fill in
// Response.Request, which http.Transport does but a caller's own need not.
type noRequestTransport struct{ h http.Handler }

func (tr noRequestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	tr.h.ServeHTTP(rec, r)
	resp := rec.Result()
	resp.Request = nil
	return resp, nil
}

func TestTransportWithoutResponseRequest(t *testing.T) {
	f := liveFake(t)
	c := New(
		WithBaseURL("https://www.youtube.com"),
		WithHTTPClient(&http.Client{Transport: noRequestTransport{f}}),
		func(o *options) { o.sleep = func(context.Context, time.Duration) error { return nil } },
	)
	ch, err := c.Open(t.Context(), "LIVEVIDEO01")
	if err != nil {
		t.Fatal(err)
	}
	if msgs, err := readAll(t, t.Context(), ch); !errors.Is(err, ErrEnded) || len(msgs) != 11 {
		t.Fatalf("got %d messages and %v, want 11 and ErrEnded", len(msgs), err)
	}
}
