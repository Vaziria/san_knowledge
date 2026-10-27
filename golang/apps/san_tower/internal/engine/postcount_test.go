package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	xv1 "github.com/wargasipil/san_remote/gen/san/remote/x/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/x/v1/xv1connect"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// countX is an X with two posts scheduled on 2026-11-20, one the day after,
// and one published on the 20th and one the day before.
type countX struct {
	xv1connect.UnimplementedXServiceHandler
	limit *uint32
}

func at(day, hour int) *timestamppb.Timestamp {
	return timestamppb.New(time.Date(2026, 11, day, hour, 0, 0, 0, time.Local))
}

func (countX) ListScheduled(context.Context, *connect.Request[xv1.ListScheduledRequest]) (*connect.Response[xv1.ListScheduledResponse], error) {
	return connect.NewResponse(&xv1.ListScheduledResponse{Threads: []*xv1.ScheduledThread{
		{At: at(20, 9), Text: "morning"}, {At: at(20, 23), Text: "night"}, {At: at(21, 0), Text: "next day"},
	}}), nil
}

func (f countX) ListPosts(_ context.Context, req *connect.Request[xv1.ListPostsRequest]) (*connect.Response[xv1.ListPostsResponse], error) {
	*f.limit = req.Msg.Limit
	return connect.NewResponse(&xv1.ListPostsResponse{Posts: []*xv1.PublishedPost{
		{At: at(20, 8), Text: "posted", Link: "https://x.com/a/status/2"}, {At: at(19, 10), Text: "older", Link: "https://x.com/a/status/1"},
	}}), nil
}

// Post Count counts the posts whose time falls on Date's day.
func TestPostCount(t *testing.T) {
	var limit uint32
	mux := http.NewServeMux()
	mux.Handle(xv1connect.NewXServiceHandler(countX{limit: &limit}))
	ts := httptest.NewServer(mux)
	defer ts.Close()
	env := Env{Connection: func(uint32) (store.Connection, error) {
		return store.Connection{ID: 1, Name: "Chrome", Type: store.SanRemote, Settings: &v1.ConnectionSettings{
			Type: &v1.ConnectionSettings_SanRemote{SanRemote: &v1.SanRemoteSettings{Url: ts.URL}},
		}}, nil
	}}
	defer func(f func() time.Time) { now = f }(now)
	now = func() time.Time { return time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local) }

	count := func(site, list, date string) (*v1.StepRun, error) {
		t.Helper()
		g := &v1.Graph{
			Steps: []*v1.Step{step("go", Manual), step("count", "post-count", "Connection", "1", "Site", site, "List", list, "Date", date)},
			Links: []*v1.Link{link("go", "Event", "count", "Event")},
		}
		steps, err := Execute(context.Background(), g, "go", nil, env)
		if len(steps) < 2 {
			return nil, err
		}
		return steps[1], err
	}
	text := func(s *v1.StepRun, output string) string {
		var v wrapperspb.StringValue
		if err := s.Outputs[output].UnmarshalTo(&v); err != nil {
			t.Fatalf("%s: %v", output, err)
		}
		return v.Value
	}

	s, err := count("X", "Both", "2026-11-20")
	if err != nil {
		t.Fatal(err)
	}
	if text(s, "Count") != "3" || text(s, "Scheduled") != "2" || text(s, "Published") != "1" || text(s, "Date") != "2026-11-20" {
		t.Errorf("both: count %s, scheduled %s, published %s, date %s", text(s, "Count"), text(s, "Scheduled"), text(s, "Published"), text(s, "Date"))
	}
	if limit != countPublished {
		t.Errorf("asked for %d published posts", limit)
	}
	var posts structpb.Value
	if err := s.Outputs["Posts"].UnmarshalTo(&posts); err != nil {
		t.Fatal(err)
	}
	var lists []string
	for _, p := range posts.GetListValue().GetValues() {
		lists = append(lists, p.GetStructValue().GetFields()["list"].GetStringValue()+" "+p.GetStructValue().GetFields()["text"].GetStringValue())
	}
	if strings.Join(lists, ", ") != "scheduled morning, scheduled night, published posted" {
		t.Errorf("posts: %v", lists)
	}

	// A datetime counts its day; one list leaves the other's count empty.
	if s, err = count("X", "Scheduled", "2026-11-21T15:00"); err != nil || text(s, "Count") != "1" || text(s, "Published") != "" {
		t.Errorf("scheduled on the 21st: %v, %v", s, err)
	}
	// No date: today (the 19th here).
	if s, err = count("X", "", ""); err != nil || text(s, "Count") != "1" || text(s, "Published") != "1" || text(s, "Date") != "2026-11-19" {
		t.Errorf("today: %v, %v", s, err)
	}
	if _, err := count("X", "Both", "soon"); err == nil || !strings.Contains(err.Error(), "Date") {
		t.Errorf("a bad date: %v", err)
	}
	if _, err := count("Facebook", "Both", ""); err == nil || !strings.Contains(err.Error(), "X, Threads") {
		t.Errorf("facebook: %v", err)
	}
}
