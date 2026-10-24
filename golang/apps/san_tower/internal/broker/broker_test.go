package broker

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"testing"

	"connectrpc.com/connect"
	"github.com/wargasipil/san_message"
	messagev1 "github.com/wargasipil/san_message/gen/san/message/v1"
	"github.com/wargasipil/san_message/gen/san/message/v1/messagev1connect"
)

func open(t *testing.T, towerDir string) *Broker {
	t.Helper()
	m, err := Open(towerDir, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close() })
	return m
}

// topics lists the topics through Handler, as the UI does.
func topics(t *testing.T, m *Broker) []string {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(m.Handler())
	ts := httptest.NewServer(mux)
	defer ts.Close()
	r, err := messagev1connect.NewMessageServiceClient(ts.Client(), ts.URL).ListTopics(context.Background(), connect.NewRequest(&messagev1.ListTopicsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, tp := range r.Msg.Topics {
		keys = append(keys, tp.Key)
	}
	return keys
}

func TestSetDataDir(t *testing.T) {
	tower := t.TempDir()
	m := open(t, tower)
	if s := m.State(); s.DataDir != "" || s.DataPath != filepath.Join(tower, DefaultDir) || s.DefaultPath != s.DataPath {
		t.Fatalf("state = %+v", s)
	}
	if err := m.Broker().CreateTopic("orders"); err != nil {
		t.Fatal(err)
	}

	// A new folder has its own, empty, broker.
	other := filepath.Join(t.TempDir(), "broker")
	if err := m.SetDataDir(other); err != nil {
		t.Fatal(err)
	}
	if s := m.State(); s.DataDir != other || s.DataPath != other {
		t.Errorf("state = %+v", s)
	}
	if got := topics(t, m); len(got) != 0 {
		t.Errorf("topics in the new folder = %v", got)
	}

	// The setting is kept: the tower opens there next time.
	m.Close()
	m = open(t, tower)
	if s := m.State(); s.DataPath != other {
		t.Errorf("reopened in %s, want %s", s.DataPath, other)
	}

	// Back to the default: the old topics are still there.
	if err := m.SetDataDir(""); err != nil {
		t.Fatal(err)
	}
	if got := topics(t, m); !slices.Equal(got, []string{"orders"}) {
		t.Errorf("topics back in the default folder = %v", got)
	}
}

func TestSetDataDirRelative(t *testing.T) {
	tower := t.TempDir()
	m := open(t, tower)
	if err := m.SetDataDir("broker2"); err != nil {
		t.Fatal(err)
	}
	if s := m.State(); s.DataDir != "broker2" || s.DataPath != filepath.Join(tower, "broker2") {
		t.Errorf("state = %+v", s)
	}
	// The default folder named by its path is the same folder: no move.
	b := m.Broker()
	if err := m.SetDataDir(filepath.Join(tower, "broker2")); err != nil {
		t.Fatal(err)
	}
	if m.Broker() != b {
		t.Error("the broker was reopened for the folder it is in")
	}
}

func TestSetDataDirRefused(t *testing.T) {
	tower := t.TempDir()
	m := open(t, tower)
	before := m.State()
	err := m.SetDataDir(filepath.Join(tower, "no", "such", "parent"))
	if !errors.Is(err, san_message.ErrInvalid) {
		t.Errorf("err = %v, want ErrInvalid", err)
	}
	if m.State() != before {
		t.Errorf("state changed to %+v", m.State())
	}
	if got := topics(t, m); got != nil {
		t.Errorf("the broker in use does not answer as before: %v", got)
	}
}
