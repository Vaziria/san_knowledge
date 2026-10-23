// Package mcp serves san_remote to Claude over the Model Context Protocol
// (JSON-RPC 2.0, newline-delimited on stdio). It is a client of a running
// `san_remote serve`, like the CLI.
//
// Stdout carries only JSON-RPC; nothing else may print to it.
package mcp

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	threadsv1 "github.com/wargasipil/san_remote/gen/san/remote/threads/v1"
	v1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/v1/remotev1connect"
	"github.com/wargasipil/san_remote/internal/hub"
	"github.com/wargasipil/san_remote/internal/threads"
)

type Server struct {
	Client remotev1connect.RemoteServiceClient
	// Threads runs the threads_* tools in this process, through Client.
	Threads *threads.Service
	// Server is the URL Client talks to, for error messages.
	Server  string
	Version string
}

var supportedVersions = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

const instructions = `san_remote runs JavaScript in the user's own browser (Chrome or Edge with the San Remote extension), where they are signed in to their accounts.
remote_tabs lists the tabs; remote_exec runs code in one (tab_id 0 is the active tab of the focused window). The code is the body of an async function: use return and await; what it returns comes back as JSON. world isolated (the default) sees the page's DOM but not its JavaScript globals, and the page cannot see it; world main runs in the page's own JavaScript.
remote_open and remote_navigate load a page and wait until it has loaded. A script that navigates away loses its result, so navigate with remote_navigate, then exec. remote_screenshot shows what a tab shows. Chrome throttles a page that is not shown, so for anything more than reading, keep its window in front (remote_open with focus, remote_focus).
This is the user's real browser: read what the task needs, and ask the user before submitting forms, posting, buying or deleting anything. Clicks and key events made from JavaScript have isTrusted false, and some sites ignore them.
threads_* post to the user's Threads account through that browser, as a person does in Threads' composer: threads_post schedules a thread (a markdown file, or one post's text), threads_scheduled lists what is scheduled, threads_reschedule and threads_unschedule change one, threads_status and threads_posts read. Schedule or post only what the user asked for; never use now unless the user asked to publish at once. A threads call that clicks through Threads brings Chrome to the front with a tab of its own for a minute or two.`

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Serve answers requests from in until it closes.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 1<<20), 32<<20)
	enc := json.NewEncoder(out)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var req request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			enc.Encode(map[string]any{"jsonrpc": "2.0", "id": nil, "error": rpcError{-32700, "parse error"}})
			continue
		}
		result, rerr := s.dispatch(ctx, req)
		if len(req.ID) == 0 || string(req.ID) == "null" {
			continue // a notification
		}
		msg := map[string]any{"jsonrpc": "2.0", "id": req.ID}
		if rerr != nil {
			msg["error"] = rerr
		} else {
			msg["result"] = result
		}
		if err := enc.Encode(msg); err != nil {
			return err
		}
	}
	return sc.Err()
}

func (s *Server) dispatch(ctx context.Context, req request) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		json.Unmarshal(req.Params, &p)
		version := supportedVersions[0]
		for _, v := range supportedVersions {
			if v == p.ProtocolVersion {
				version = v
			}
		}
		return map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "san_remote", "title": "San Remote", "version": s.Version},
			"instructions":    instructions,
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": toolDefs()}, nil
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, &rpcError{-32602, "invalid params"}
		}
		content, err := s.Call(ctx, p.Name, p.Arguments)
		if err != nil {
			return map[string]any{"content": []any{textContent("Error: " + err.Error())}, "isError": true}, nil
		}
		return map[string]any{"content": content}, nil
	}
	if strings.HasPrefix(req.Method, "notifications/") {
		return nil, nil
	}
	return nil, &rpcError{-32601, "method not found: " + req.Method}
}

func textContent(t string) map[string]any { return map[string]any{"type": "text", "text": t} }

func imageContent(png []byte) map[string]any {
	return map[string]any{"type": "image", "data": base64.StdEncoding.EncodeToString(png), "mimeType": "image/png"}
}

// ---- tools ---------------------------------------------------------------------------

func schema(props map[string]any, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func str(desc string) map[string]any  { return map[string]any{"type": "string", "description": desc} }
func intg(desc string) map[string]any { return map[string]any{"type": "integer", "description": desc} }
func flag(desc string) map[string]any { return map[string]any{"type": "boolean", "description": desc} }
func strs(desc string) map[string]any {
	return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": desc}
}
func enum(desc string, vals ...string) map[string]any {
	return map[string]any{"type": "string", "enum": vals, "description": desc}
}

func tool(name, title, desc string, input map[string]any, readOnly, destructive bool) map[string]any {
	return map[string]any{
		"name": name, "title": title, "description": desc, "inputSchema": input,
		"annotations": map[string]any{"readOnlyHint": readOnly, "destructiveHint": destructive, "openWorldHint": true},
	}
}

var (
	browserProp = str("The browser's id or name (remote_browsers); may be left out when only one is attached")
	tabProp     = intg("Tab id from remote_tabs; 0 or left out is the active tab of the focused window")
	timeoutProp = intg("Milliseconds to wait (default 30000)")
	timeProp    = func(what string) map[string]any {
		return str(what + `: local time "2006-01-02 15:04", or RFC 3339`)
	}
	matchProp = str("Part of the scheduled thread's text; with at, it must name exactly one")
)

func toolDefs() []any {
	return []any{
		tool("remote_browsers", "Attached browsers",
			"The browsers attached to san_remote through the San Remote extension.",
			schema(map[string]any{}), true, false),
		tool("remote_tabs", "List tabs",
			"Every tab of the browser: id, window, active, title and url.",
			schema(map[string]any{"browser": browserProp}), true, false),
		tool("remote_open", "Open a tab",
			"Open a url in a new tab and wait until it has loaded.",
			schema(map[string]any{
				"url":        str("The page to open"),
				"background": flag("Open it without switching to it"),
				"focus":      flag("Also bring the browser window to the front, when the user should watch"),
				"browser":    browserProp,
				"timeout_ms": timeoutProp,
			}, "url"), false, false),
		tool("remote_navigate", "Navigate a tab",
			"Load a url in a tab and wait until it has loaded.",
			schema(map[string]any{
				"tab_id":     tabProp,
				"url":        str("The page to load"),
				"browser":    browserProp,
				"timeout_ms": timeoutProp,
			}, "url"), false, false),
		tool("remote_close", "Close a tab",
			"Close a tab.",
			schema(map[string]any{"tab_id": intg("Tab id from remote_tabs"), "browser": browserProp}, "tab_id"), false, true),
		tool("remote_focus", "Bring a tab to the front",
			"Make a tab the active one of its window and bring the window to the front, so its page is shown and not throttled.",
			schema(map[string]any{"tab_id": tabProp, "browser": browserProp}), false, false),
		tool("remote_screenshot", "Screenshot a tab",
			"What a tab shows, as an image. The tab becomes the active one of its window.",
			schema(map[string]any{"tab_id": tabProp, "browser": browserProp}), true, false),
		tool("remote_exec", "Run JavaScript",
			"Run JavaScript in a tab and return its result as JSON. The code is the body of an async function: `return document.title`, `await new Promise(r => setTimeout(r, 500))`. DOM nodes do not survive JSON; return their text or attributes.",
			schema(map[string]any{
				"code":       str("The body of an async function; its return value comes back as JSON"),
				"tab_id":     tabProp,
				"world":      enum("isolated (default): sees the DOM, not the page's JavaScript; main: the page's own JavaScript world", "isolated", "main"),
				"browser":    browserProp,
				"timeout_ms": timeoutProp,
			}, "code"), false, true),

		tool("threads_status", "Threads: signed in?",
			"Whether the browser is signed in to Threads, and as whom. Reads the home page in a background tab.",
			schema(map[string]any{"browser": browserProp}), true, false),
		tool("threads_post", "Threads: schedule a thread",
			"Schedule a thread on the user's Threads account, through the composer: from a markdown file (frontmatter at: and topic:, posts separated by lines holding only ---, media as ![](file)), or one post's text with media. With now it is published at once, publicly. Brings Chrome to the front with a tab of its own for a minute or two; returns the time Threads shows.",
			schema(map[string]any{
				"file":    str("Path of the thread's markdown file"),
				"text":    str("One post's text, instead of a file (at most 500 characters)"),
				"media":   strs("Image or video files for that post (jpg, png, webp, avif, mp4, mov, webm)"),
				"topic":   str("The first post's topic, picked from Threads' suggestions"),
				"at":      timeProp("When Threads publishes it, 15 minutes to 90 days ahead; overrides the file's at"),
				"now":     flag("Publish at once, publicly, instead of scheduling. Only when the user asked for that"),
				"browser": browserProp,
			}), false, false),
		tool("threads_scheduled", "Threads: scheduled threads",
			"The threads waiting to be published, soonest first, with their times, from the composer's Drafts. Brings Chrome to the front briefly.",
			schema(map[string]any{"browser": browserProp}), true, false),
		tool("threads_reschedule", "Threads: move a scheduled thread",
			"Move one scheduled thread to a new time, named by part of its text and/or its current time.",
			schema(map[string]any{
				"match":   matchProp,
				"at":      timeProp("Its current time"),
				"to":      timeProp("Its new time"),
				"browser": browserProp,
			}, "to"), false, false),
		tool("threads_unschedule", "Threads: delete a scheduled thread",
			"Delete one scheduled thread, named by part of its text and/or its time. Threads asks nothing first, and it cannot be undone.",
			schema(map[string]any{
				"match":   matchProp,
				"at":      timeProp("Its time"),
				"browser": browserProp,
			}), false, true),
		tool("threads_posts", "Threads: my posts",
			"The account's own published posts, newest first: link, time and text. Reads the profile in a background tab.",
			schema(map[string]any{"limit": intg("How many (default 10)"), "browser": browserProp}), true, false),
	}
}

// args are the arguments of every tool.
type args struct {
	Browser    string   `json:"browser"`
	TabID      int32    `json:"tab_id"`
	URL        string   `json:"url"`
	Background bool     `json:"background"`
	Focus      bool     `json:"focus"`
	TimeoutMs  int32    `json:"timeout_ms"`
	Code       string   `json:"code"`
	World      string   `json:"world"`
	File       string   `json:"file"`
	Text       string   `json:"text"`
	Media      []string `json:"media"`
	Topic      string   `json:"topic"`
	At         string   `json:"at"`
	Now        bool     `json:"now"`
	Match      string   `json:"match"`
	To         string   `json:"to"`
	Limit      int32    `json:"limit"`
}

// Call runs one tool and returns its content: text, or for
// remote_screenshot an image.
func (s *Server) Call(ctx context.Context, name string, raw json.RawMessage) ([]any, error) {
	var a args
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, fmt.Errorf("arguments: %w", err)
		}
	}
	if name == "remote_screenshot" {
		r, err := s.Client.Screenshot(ctx, connect.NewRequest(&v1.ScreenshotRequest{Browser: a.Browser, TabId: a.TabID}))
		if err != nil {
			return nil, hub.ClientError(err, s.Server)
		}
		return []any{imageContent(r.Msg.Png)}, nil
	}
	var text string
	var err error
	if strings.HasPrefix(name, "threads_") {
		text, err = s.threads(ctx, name, a)
	} else {
		text, err = s.call(ctx, name, a)
	}
	if err != nil {
		return nil, hub.ClientError(err, s.Server)
	}
	return []any{textContent(text)}, nil
}

func (s *Server) call(ctx context.Context, name string, a args) (string, error) {
	c := s.Client
	browser, tabID, url, timeoutMs := a.Browser, a.TabID, a.URL, a.TimeoutMs
	switch name {
	case "remote_browsers":
		r, err := c.ListBrowsers(ctx, connect.NewRequest(&v1.ListBrowsersRequest{}))
		if err != nil {
			return "", err
		}
		if len(r.Msg.Browsers) == 0 {
			return "No browser is attached.", nil
		}
		var b strings.Builder
		for _, br := range r.Msg.Browsers {
			fmt.Fprintf(&b, "%s (id %s), attached %s, extension %s\n%s\n",
				br.Name, br.Id, br.AttachedAt.AsTime().Local().Format("2006-01-02 15:04:05"), br.ExtensionVersion, br.UserAgent)
		}
		return b.String(), nil
	case "remote_tabs":
		r, err := c.ListTabs(ctx, connect.NewRequest(&v1.ListTabsRequest{Browser: browser}))
		if err != nil {
			return "", err
		}
		var b strings.Builder
		for _, t := range r.Msg.Tabs {
			fmt.Fprintln(&b, FormatTab(t))
		}
		return b.String(), nil
	case "remote_open":
		if url == "" {
			return "", fmt.Errorf("url is required")
		}
		r, err := c.OpenTab(ctx, connect.NewRequest(&v1.OpenTabRequest{Browser: browser, Url: url, Background: a.Background, Focus: a.Focus, TimeoutMs: timeoutMs}))
		if err != nil {
			return "", err
		}
		return FormatTab(r.Msg.Tab), nil
	case "remote_navigate":
		if url == "" {
			return "", fmt.Errorf("url is required")
		}
		r, err := c.Navigate(ctx, connect.NewRequest(&v1.NavigateRequest{Browser: browser, TabId: tabID, Url: url, TimeoutMs: timeoutMs}))
		if err != nil {
			return "", err
		}
		return FormatTab(r.Msg.Tab), nil
	case "remote_close":
		if _, err := c.CloseTab(ctx, connect.NewRequest(&v1.CloseTabRequest{Browser: browser, TabId: tabID})); err != nil {
			return "", err
		}
		return fmt.Sprintf("Closed tab %d.", tabID), nil
	case "remote_focus":
		r, err := c.FocusTab(ctx, connect.NewRequest(&v1.FocusTabRequest{Browser: browser, TabId: tabID}))
		if err != nil {
			return "", err
		}
		return FormatTab(r.Msg.Tab), nil
	case "remote_exec":
		if a.Code == "" {
			return "", fmt.Errorf("code is required")
		}
		w, err := ParseWorld(a.World)
		if err != nil {
			return "", err
		}
		r, err := c.Exec(ctx, connect.NewRequest(&v1.ExecRequest{Browser: browser, TabId: tabID, Code: a.Code, World: w, TimeoutMs: timeoutMs}))
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Ran in %s\n\n%s", FormatTab(r.Msg.Tab), r.Msg.ResultJson), nil
	}
	return "", fmt.Errorf("unknown tool %q", name)
}

func (s *Server) threads(ctx context.Context, name string, a args) (string, error) {
	t := s.Threads
	switch name {
	case "threads_status":
		st, err := t.Status(ctx, a.Browser)
		if err != nil {
			return "", err
		}
		if !st.SignedIn {
			return "The browser is not signed in to Threads.", nil
		}
		return "Signed in to Threads as @" + st.Username + ".", nil
	case "threads_post":
		req, err := postRequest(a)
		if err != nil {
			return "", err
		}
		r, err := t.Post(ctx, req)
		if err != nil {
			return "", err
		}
		if a.Now {
			return "Posted.", nil
		}
		return "Scheduled: " + r.Shown, nil
	case "threads_scheduled":
		posts, err := t.ListScheduled(ctx, a.Browser)
		if err != nil {
			return "", err
		}
		if len(posts) == 0 {
			return "Nothing is scheduled.", nil
		}
		var b strings.Builder
		for _, p := range posts {
			fmt.Fprintf(&b, "%s (%s)\n%s\n\n", localTime(p.At), p.Shown, p.Text)
		}
		return b.String(), nil
	case "threads_reschedule":
		to, err := threads.ParseTime(a.To)
		if err != nil {
			return "", err
		}
		at, err := optionalTime(a.At)
		if err != nil {
			return "", err
		}
		p, err := t.Reschedule(ctx, a.Browser, a.Match, at, to)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Moved to %s (%s):\n%s", localTime(p.At), p.Shown, p.Text), nil
	case "threads_unschedule":
		at, err := optionalTime(a.At)
		if err != nil {
			return "", err
		}
		p, err := t.Unschedule(ctx, a.Browser, a.Match, at)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Deleted the thread scheduled %s:\n%s", p.Shown, p.Text), nil
	case "threads_posts":
		posts, err := t.ListPosts(ctx, a.Browser, int(a.Limit))
		if err != nil {
			return "", err
		}
		var b strings.Builder
		for _, p := range posts {
			fmt.Fprintf(&b, "%s %s\n%s\n\n", localTime(p.At), p.Url, p.Text)
		}
		return b.String(), nil
	}
	return "", fmt.Errorf("unknown tool %q", name)
}

// postRequest makes threads_post's request: from the file, or from text and
// media, with at overriding the file's time.
func postRequest(a args) (*threadsv1.PostRequest, error) {
	var th *threads.Thread
	switch {
	case a.File != "" && a.Text != "":
		return nil, errors.New("give file or text, not both")
	case a.File != "":
		var err error
		if th, err = threads.ParseFile(a.File); err != nil {
			return nil, err
		}
	case a.Text != "" || len(a.Media) > 0:
		th = &threads.Thread{Posts: []threads.Post{{Text: a.Text, Media: a.Media}}}
	default:
		return nil, errors.New("file or text is required")
	}
	if a.At != "" {
		at, err := threads.ParseTime(a.At)
		if err != nil {
			return nil, err
		}
		th.At = at
	}
	if a.Topic != "" && len(th.Posts) > 0 {
		th.Posts[0].Topic = a.Topic
	}
	switch {
	case a.Now && !th.At.IsZero():
		return nil, errors.New("give at or now, not both")
	case !a.Now && th.At.IsZero():
		return nil, errors.New("give at (or at: in the file); now only when the user asked to publish at once")
	}
	if err := th.Check(); err != nil {
		return nil, err
	}
	return th.Request(a.Browser, a.Now)
}

// optionalTime reads a time argument, zero when it is not given.
func optionalTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return threads.ParseTime(s)
}

func localTime(ts *timestamppb.Timestamp) string {
	if ts == nil {
		return "?"
	}
	return ts.AsTime().In(time.Local).Format("2006-01-02 15:04")
}

// FormatTab is one tab on one line: `#12 [active] Title (complete) https://...`.
func FormatTab(t *v1.Tab) string {
	if t == nil {
		return "(no tab)"
	}
	active := ""
	if t.Active {
		active = " [active]"
	}
	status := ""
	if t.Status != "" && t.Status != "complete" {
		status = " (" + t.Status + ")"
	}
	return fmt.Sprintf("#%d window %d%s %s%s %s", t.Id, t.WindowId, active, t.Title, status, t.Url)
}

// ParseWorld turns "isolated", "main" or "" into a World.
func ParseWorld(s string) (v1.World, error) {
	switch strings.ToLower(s) {
	case "", "isolated":
		return v1.World_WORLD_ISOLATED, nil
	case "main":
		return v1.World_WORLD_MAIN, nil
	}
	return 0, fmt.Errorf("world %q: want isolated or main", s)
}
