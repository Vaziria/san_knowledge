package engine

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

func TestHTTPRequest(t *testing.T) {
	var got struct {
		method, path, contentType string
		body                      map[string]any
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.method, got.path, got.contentType = r.Method, r.URL.Path, r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		got.body = nil
		_ = json.Unmarshal(b, &got.body)
		if strings.HasSuffix(r.URL.Path, "/missing") {
			http.Error(w, "no such order", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok": true, "n": 2}`))
	}))
	defer srv.Close()

	run := func(method, url string) ([]*v1.StepRun, error) {
		g := &v1.Graph{
			Steps: []*v1.Step{step("ev", TopicEvent), step("http", "http-request", "Method", method, "URL", url)},
			Links: []*v1.Link{link("ev", "Event", "http", "Event"), link("ev", "Payload", "http", "Body")},
		}
		return Execute(context.Background(), g, "ev", []byte(`{"id": "A1", "total": 5}`), Env{})
	}

	steps, err := run("POST", srv.URL+"/orders/{{ payload.id }}")
	if err != nil {
		t.Fatal(err)
	}
	if got.method != "POST" || got.path != "/orders/A1" || got.contentType != "application/json" || got.body["total"] != 5.0 {
		t.Errorf("server got %+v", got)
	}
	var resp structpb.Value
	if err := steps[1].Outputs["Response"].UnmarshalTo(&resp); err != nil {
		t.Fatal(err)
	}
	f := resp.GetStructValue().Fields
	if f["status"].GetNumberValue() != 200 || !f["body"].GetStructValue().Fields["ok"].GetBoolValue() || steps[1].Fired[0] != "Done" {
		t.Errorf("Response = %v, fired %v", &resp, steps[1].Fired)
	}

	// GET sends no body.
	if _, err := run("GET", srv.URL+"/orders"); err != nil || got.method != "GET" || got.body != nil {
		t.Errorf("GET: %v, server got %+v", err, got)
	}

	// A 404 fails the step; its answer is still in the run.
	steps, err = run("GET", srv.URL+"/orders/missing")
	if err == nil || !strings.Contains(err.Error(), "404") || !strings.Contains(err.Error(), "no such order") {
		t.Errorf("404: %v", err)
	}
	if steps[1].Outputs["Response"] == nil {
		t.Error("the failed step has no Response")
	}

	for _, bad := range []string{"", "ftp://x", "{{ payload. }}"} {
		if _, err := run("GET", bad); err == nil {
			t.Errorf("URL %q worked", bad)
		}
	}
}

func TestPublish(t *testing.T) {
	type published struct {
		topic, data string
	}
	var got []published
	env := Env{Publish: func(_ context.Context, topic string, data []byte) (uint64, error) {
		if topic == "nope" {
			return 0, errors.New("topic nope does not exist")
		}
		got = append(got, published{topic, string(data)})
		return uint64(len(got)), nil
	}}
	run := func(topic, payload string, linked bool) ([]*v1.StepRun, error) {
		g := &v1.Graph{
			Steps: []*v1.Step{step("ev", TopicEvent), step("pub", "publish", "Topic", topic)},
			Links: []*v1.Link{link("ev", "Event", "pub", "Event")},
		}
		if linked {
			g.Links = append(g.Links, link("ev", "Payload", "pub", "Payload"))
		}
		return Execute(context.Background(), g, "ev", []byte(payload), env)
	}

	steps, err := run("big-orders", `{"id": "A1"}`, true)
	if err != nil {
		t.Fatal(err)
	}
	var result structpb.Value
	if err := steps[1].Outputs["Result"].UnmarshalTo(&result); err != nil || result.GetStructValue().Fields["eventId"].GetStringValue() != "1" {
		t.Errorf("Result = %v, %v", &result, err)
	}
	if _, err := run("notes", "just text", true); err != nil {
		t.Fatal(err)
	}
	if _, err := run("pings", `{"x": 1}`, false); err != nil {
		t.Fatal(err)
	}
	want := []published{{"big-orders", `{"id":"A1"}`}, {"notes", "just text"}, {"pings", ""}}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("published %q, want %q", got, want)
	}

	if _, err := run("nope", "{}", true); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("missing topic: %v", err)
	}
	if _, err := run("", "{}", true); err == nil || !strings.Contains(err.Error(), "Topic is empty") {
		t.Errorf("no topic: %v", err)
	}
	g := &v1.Graph{Steps: []*v1.Step{step("ev", TopicEvent), step("pub", "publish", "Topic", "x")}, Links: []*v1.Link{link("ev", "Event", "pub", "Event")}}
	if _, err := Execute(context.Background(), g, "ev", nil, Env{}); err == nil || !strings.Contains(err.Error(), "cannot publish") {
		t.Errorf("no publisher: %v", err)
	}
}
