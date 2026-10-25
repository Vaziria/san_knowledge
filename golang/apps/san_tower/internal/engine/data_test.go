package engine

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// logged runs a graph from its Topic Event step "in" and returns what its
// Log steps wrote.
func logged(t *testing.T, g *v1.Graph, env Env) ([]string, error) {
	t.Helper()
	steps, err := Execute(context.Background(), g, "in", []byte(`{}`), env)
	var lines []string
	for _, s := range steps {
		lines = append(lines, s.Logs...)
	}
	return lines, err
}

// storageGraph logs a storage's Path and Absolute Path.
func storageGraph(id string) *v1.Graph {
	return &v1.Graph{
		Steps: []*v1.Step{
			step("in", TopicEvent),
			step("st", "storage", "Storage", id),
			step("p", "log"),
			step("a", "log"),
		},
		Links: []*v1.Link{
			link("in", "Event", "p", "Event"),
			link("in", "Event", "a", "Event"),
			link("st", "Path", "p", "Message"),
			link("st", "Absolute Path", "a", "Message"),
		},
	}
}

func TestStorageStep(t *testing.T) {
	env := Env{Storage: func(id uint32) (store.Storage, error) {
		if id != 7 {
			return store.Storage{}, store.ErrNotFound
		}
		return store.Storage{ID: 7, Name: "Videos", Path: "videos", Abs: "D:/tower/data/videos"}, nil
	}}
	lines, err := logged(t, storageGraph("7"), env)
	if err != nil || strings.Join(lines, "|") != "videos|D:/tower/data/videos" {
		t.Errorf("logged %q, %v", lines, err)
	}
	for id, says := range map[string]string{
		"":    "pick a storage",
		"x":   "not a storage's id",
		"404": "storage 404 is gone",
	} {
		if _, err := logged(t, storageGraph(id), env); err == nil || !strings.Contains(err.Error(), says) {
			t.Errorf("Storage %q: %v; want %q", id, err, says)
		}
	}
	if _, err := logged(t, storageGraph("7"), Env{}); err == nil || !strings.Contains(err.Error(), "no storages") {
		t.Errorf("no Env.Storage: %v", err)
	}
	failing := Env{Storage: func(uint32) (store.Storage, error) { return store.Storage{}, errors.New("disk on fire") }}
	if _, err := logged(t, storageGraph("7"), failing); err == nil || !strings.Contains(err.Error(), "disk on fire") {
		t.Errorf("store error: %v", err)
	}
}

// A UUID step runs once in a run: two steps that take its UUID get the same
// one. The next run makes a new one.
func TestUUIDStep(t *testing.T) {
	g := &v1.Graph{
		Steps: []*v1.Step{step("in", TopicEvent), step("id", "uuid"), step("one", "log"), step("two", "log")},
		Links: []*v1.Link{
			link("in", "Event", "one", "Event"),
			link("in", "Event", "two", "Event"),
			link("id", "UUID", "one", "Message"),
			link("id", "UUID", "two", "Message"),
		},
	}
	v4 := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	first, err := logged(t, g, Env{})
	if err != nil || len(first) < 2 || !v4.MatchString(first[0]) || first[1] != first[0] {
		t.Fatalf("logged %q, %v", first, err)
	}
	second, _ := logged(t, g, Env{})
	if len(second) == 0 || second[0] == first[0] {
		t.Errorf("a second run made %q again", second)
	}
}

func TestMD5Step(t *testing.T) {
	graph := func(typed string, linked bool) *v1.Graph {
		g := &v1.Graph{
			Steps: []*v1.Step{
				step("in", TopicEvent),
				step("tpl", "template", "Text", "hello"),
				step("h", "md5", "Text", typed),
				step("log", "log"),
			},
			Links: []*v1.Link{link("in", "Event", "log", "Event"), link("h", "Hash", "log", "Message")},
		}
		if linked {
			g.Links = append(g.Links, link("in", "Payload", "tpl", "Payload"), link("tpl", "Text", "h", "Text"))
		}
		return g
	}
	for name, c := range map[string]struct {
		g    *v1.Graph
		want string
	}{
		"typed":  {graph("hello", false), "5d41402abc4b2a76b9719d911017c592"},
		"empty":  {graph("", false), "d41d8cd98f00b204e9800998ecf8427e"},
		"linked": {graph("ignored", true), "5d41402abc4b2a76b9719d911017c592"}, // the Template's text wins
	} {
		lines, err := logged(t, c.g, Env{})
		if err != nil || len(lines) != 1 || lines[0] != c.want {
			t.Errorf("%s: %q, %v; want %s", name, lines, err, c.want)
		}
	}
}
