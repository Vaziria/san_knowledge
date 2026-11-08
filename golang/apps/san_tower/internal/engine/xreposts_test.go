package engine

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	xv1 "github.com/wargasipil/san_remote/gen/san/remote/x/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// Two reposts, the latest first, as San Remote lists them; the first with
// its repost time, the second without (X's app data did not have it).
func (f fakeX) ListReposts(_ context.Context, req *connect.Request[xv1.ListRepostsRequest]) (*connect.Response[xv1.ListRepostsResponse], error) {
	f.xreposts = req.Msg
	day := time.Date(2026, 11, 8, 6, 0, 0, 0, time.UTC)
	return connect.NewResponse(&xv1.ListRepostsResponse{Account: "BangsaPersatuan", Reposts: []*xv1.Repost{
		{Link: "https://x.com/a/status/1", Id: "1", Author: "A", Handle: "a", Text: "first", Created: timestamppb.New(day.Add(-time.Hour)),
			Reposted: timestamppb.New(day), HasMedia: true, Media: []string{"video"}},
		{Link: "https://x.com/b/status/2", Id: "2", Author: "B", Handle: "b", Text: "second", Created: timestamppb.New(day.Add(-48 * time.Hour))},
	}}), nil
}

// Post 1 is reposted, so it is undone; post 2 is not; post 404 is gone.
func (f fakeX) Unrepost(_ context.Context, req *connect.Request[xv1.UnrepostRequest]) (*connect.Response[xv1.UnrepostResponse], error) {
	f.xunrepost = append(f.xunrepost, req.Msg.Link)
	id := req.Msg.Link
	if m := regexp.MustCompile(`/status/(\d+)`).FindStringSubmatch(id); m != nil {
		id = m[1]
	}
	switch id {
	case "404":
		return nil, connect.NewError(connect.CodeNotFound, errors.New(`x: the post is not there: X says "Hmm...this page doesn’t exist. Try searching for something else."`))
	case "1":
		return connect.NewResponse(&xv1.UnrepostResponse{Link: "https://x.com/a/status/1", Undone: true}), nil
	}
	return connect.NewResponse(&xv1.UnrepostResponse{Link: req.Msg.Link}), nil
}

// X Reposts fires Each once per repost, the latest first, with Item and
// Link; an X Undo Repost on Each undoes each one, and a Log after the loop
// sees Count.
func TestXReposts(t *testing.T) {
	f, env := remote(t)
	g := &v1.Graph{
		Steps: []*v1.Step{
			step("ev", TopicEvent),
			step("r", "x-reposts", "Connection", "1", "Limit", "5"),
			step("item", "template", "Text", "{{ payload.author }}: {{ payload.text }}"),
			step("log", "log"),
			step("undo", "x-undo-repost", "Connection", "1"),
			step("count", "log"),
		},
		Links: []*v1.Link{
			link("ev", "Event", "r", "Event"),
			link("r", "Each", "log", "Event"),
			link("r", "Item", "item", "Payload"),
			link("item", "Text", "log", "Message"),
			link("r", "Each", "undo", "Event"),
			link("r", "Link", "undo", "Link"),
			link("r", "Done", "count", "Event"),
			link("r", "Count", "count", "Message"),
		},
	}
	steps, err := Execute(context.Background(), g, "ev", nil, env)
	if err != nil {
		t.Fatal(err)
	}
	got := ran(steps)
	for _, want := range []string{
		"item:: log::A: first",
		"undo:Undone,Done:undid the repost of https://x.com/a/status/1",
		"item:: log::B: second",
		"undo:Not Reposted,Done:https://x.com/b/status/2 is not reposted by the account: nothing to undo",
		"r:Each,Done:@BangsaPersatuan has 2 reposts listed (up to 5) count::2",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("ran %s\nwant %q in it", got, want)
		}
	}
	if strings.Index(got, "log::A: first") > strings.Index(got, "log::B: second") {
		t.Errorf("not the latest first: %s", got)
	}
	if f.xreposts.GetLimit() != 5 || strings.Join(f.xunrepost, " ") != "https://x.com/a/status/1 https://x.com/b/status/2" {
		t.Errorf("asked %v, undid %v", f.xreposts, f.xunrepost)
	}
	var r *v1.StepRun
	for _, s := range steps {
		if s.StepId == "r" {
			r = s
		}
	}
	list := objectOut(t, r, "Reposts").GetListValue().GetValues()
	if len(list) != 2 || textOut(t, r, "Count") != "2" {
		t.Fatalf("Reposts = %v", list)
	}
	first := list[0].GetStructValue().GetFields()
	if first["link"].GetStringValue() != "https://x.com/a/status/1" || first["reposted"].GetStringValue() != "2026-11-08T06:00:00Z" ||
		!first["hasMedia"].GetBoolValue() || first["handle"].GetStringValue() != "a" {
		t.Errorf("Item = %v", first)
	}
	if _, ok := list[1].GetStructValue().GetFields()["reposted"]; ok {
		t.Errorf("a repost without its time has one: %v", list[1])
	}
	// After the loop, Item and Link are the last repost's.
	if textOut(t, r, "Link") != "https://x.com/b/status/2" {
		t.Errorf("Link = %s", textOut(t, r, "Link"))
	}

	// Limit: empty is San Remote's 20; out of range fails before it is asked.
	f.xreposts = nil
	if _, err := run(t, env, "x-reposts", "Connection", "1"); err != nil || f.xreposts.GetLimit() != 20 {
		t.Errorf("default limit: %v, %v", f.xreposts, err)
	}
	for _, bad := range []string{"0", "201", "lots"} {
		f.xreposts = nil
		if _, err := run(t, env, "x-reposts", "Connection", "1", "Limit", bad); err == nil || !strings.Contains(err.Error(), "1 to 200") || f.xreposts != nil {
			t.Errorf("Limit %s: %v, asked %v", bad, err, f.xreposts)
		}
	}
}

// X Undo Repost: Undone, Not Reposted, a post that is gone, and links
// refused before San Remote is asked.
func TestXUndoRepost(t *testing.T) {
	f, env := remote(t)
	steps, err := run(t, env, "x-undo-repost", "Connection", "1", "Link", "https://twitter.com/a/status/1?s=20")
	if err != nil || ran(steps) != "ev:Event: s:Undone,Done:undid the repost of https://x.com/a/status/1" {
		t.Errorf("undone: %s, %v", ran(steps), err)
	}
	steps, err = run(t, env, "x-undo-repost", "Connection", "1", "Link", "2")
	if err != nil || ran(steps) != "ev:Event: s:Not Reposted,Done:2 is not reposted by the account: nothing to undo" {
		t.Errorf("not reposted: %s, %v", ran(steps), err)
	}
	if _, err := run(t, env, "x-undo-repost", "Connection", "1", "Link", "https://x.com/a/status/404"); err == nil || !strings.Contains(err.Error(), "doesn’t exist") {
		t.Errorf("gone: %v", err)
	}
	// Linked Failed takes the error instead.
	g := &v1.Graph{
		Steps: []*v1.Step{step("ev", TopicEvent), step("s", "x-undo-repost", "Connection", "1", "Link", "https://x.com/a/status/404"), step("log", "log")},
		Links: []*v1.Link{link("ev", "Event", "s", "Event"), link("s", "Failed", "log", "Event"), link("s", "Error", "log", "Message")},
	}
	if steps, err := Execute(context.Background(), g, "ev", nil, env); err != nil || !strings.Contains(ran(steps), "s:Failed:") || !strings.Contains(ran(steps), "log::x: the post is not there") {
		t.Errorf("Failed: %s, %v", ran(steps), err)
	}
	n := len(f.xunrepost)
	for _, bad := range []string{"", "https://x.com/a", "https://example.com/a/status/1", "12a"} {
		if _, err := run(t, env, "x-undo-repost", "Connection", "1", "Link", bad); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
	if len(f.xunrepost) != n {
		t.Errorf("San Remote was asked for bad links: %v", f.xunrepost[n:])
	}
}

func TestXPostLink(t *testing.T) {
	for link, want := range map[string]bool{
		"https://x.com/dayamatest0607/status/2118787984070005168": true,
		"https://twitter.com/a_b/status/123/photo/1":              true,
		"https://mobile.x.com/a/status/123/video/1?s=46&t=x":      true,
		"x.com/i/web/status/456":                                  true,
		"https://mobile.twitter.com/user/statuses/789":            true,
		"www.x.com/a/status/1#m":                                  true,
		"2118787984070005168":                                     true,
		"https://x.com/a":                                         false,
		"https://example.com/a/status/1":                          false,
		"https://x.com/a/status/12a":                              false,
		"12a":                                                     false,
		"see https://x.com/a/status/1":                            false,
		"https://x.com/dayamatest0607/status/2118787984070005168 https://x.com/b/status/2": false,
	} {
		if got := xPostLink.MatchString(link); got != want {
			t.Errorf("%q: %v", link, got)
		}
	}
}
