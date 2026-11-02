package engine

import (
	"context"
	"testing"

	threadsv1 "github.com/wargasipil/san_remote/gen/san/remote/threads/v1"
	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestSetBrowser(t *testing.T) {
	r := &threadsv1.SchedulePostRequest{}
	setBrowser(r, "personal")
	if r.Browser != "personal" {
		t.Errorf("empty browser: got %q, want personal", r.Browser)
	}
	r = &threadsv1.SchedulePostRequest{Browser: "work"}
	setBrowser(r, "personal")
	if r.Browser != "work" {
		t.Errorf("a request's own browser was replaced: %q", r.Browser)
	}
	r = &threadsv1.SchedulePostRequest{}
	setBrowser(r, "")
	if r.Browser != "" {
		t.Errorf("no browser set one: %q", r.Browser)
	}
	// A message without the field, and not a message at all, are left be.
	s := structpb.NewStringValue("x")
	setBrowser(s, "personal")
	setBrowser("not a message", "personal")
	if s.GetStringValue() != "x" {
		t.Errorf("a message without browser changed: %v", s)
	}
}

// A step on a connection with a browser sends it to san_remote; one
// without leaves it to san_remote, which takes the only browser.
func TestConnectionBrowser(t *testing.T) {
	f, env := remote(t)
	one := env.Connection
	env.Connection = func(id uint32) (store.Connection, error) {
		c, err := one(1)
		if id != 3 {
			return one(id)
		}
		c.ID, c.Name = 3, "Personal Chrome"
		c.Settings = &v1.ConnectionSettings{Type: &v1.ConnectionSettings_SanRemote{SanRemote: &v1.SanRemoteSettings{
			Url: c.Settings.GetSanRemote().GetUrl(), Browser: "personal",
		}}}
		return c, err
	}
	for _, c := range []struct{ conn, want string }{{"3", "personal"}, {"1", ""}} {
		g := &v1.Graph{
			Steps: []*v1.Step{
				step("ev", TopicEvent),
				step("post", "threads-post", "Connection", c.conn, "Text", "hello", "At", "+1d"),
			},
			Links: []*v1.Link{link("ev", "Event", "post", "Event")},
		}
		f.threads = nil
		if _, err := Execute(context.Background(), g, "ev", nil, env); err != nil {
			t.Fatalf("connection %s: %v", c.conn, err)
		}
		if got := f.threads.GetBrowser(); got != c.want {
			t.Errorf("connection %s sent browser %q, want %q", c.conn, got, c.want)
		}
	}
}
