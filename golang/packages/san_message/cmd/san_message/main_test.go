package main

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wargasipil/san_message"
)

func TestManageTopicsAndSubscribers(t *testing.T) {
	b, err := san_message.Open(san_message.Options{
		Dir: t.TempDir(),
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	mux := http.NewServeMux()
	mux.Handle(b.Handler())
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	run := func(args ...string) (string, error) {
		var out bytes.Buffer
		err := root(&out).Run(t.Context(), append([]string{"san_message", "--server", srv.URL}, args...))
		return out.String(), err
	}
	ok := func(args ...string) string {
		t.Helper()
		out, err := run(args...)
		if err != nil {
			t.Fatalf("%s: %v", strings.Join(args, " "), err)
		}
		return out
	}

	ok("topic", "create", "orders")
	ok("subscriber", "create", "billing", "--topic", "orders", "--type", "webhook", "--webhook-uri", "http://127.0.0.1:1/hook")
	ok("subscriber", "create", "app", "--topic", "orders", "--type", "pull", "--from", "earliest")

	if out := ok("topic", "list"); !strings.Contains(out, "orders") || !strings.Contains(out, "SUBSCRIBERS") {
		t.Fatalf("topic list:\n%s", out)
	}
	out := ok("subscriber", "list", "--topic", "orders")
	for _, want := range []string{"billing", "webhook", "http://127.0.0.1:1/hook", "app", "pull"} {
		if !strings.Contains(out, want) {
			t.Fatalf("subscriber list misses %q:\n%s", want, out)
		}
	}

	if _, err := run("topic", "delete", "orders"); err == nil || !strings.Contains(err.Error(), "still has subscribers") {
		t.Fatalf("deleting a topic with subscribers: %v", err)
	}
	if _, err := run("subscriber", "create", "x", "--topic", "orders", "--type", "queue"); err == nil {
		t.Fatal("--type queue worked, want an error")
	}
	if _, err := run("subscriber", "create", "x", "--topic", "orders", "--type", "pull", "--from", "middle"); err == nil {
		t.Fatal("--from middle worked, want an error")
	}

	ok("subscriber", "delete", "billing")
	ok("subscriber", "delete", "app")
	ok("topic", "delete", "orders")
	if out := ok("topic", "list"); !strings.Contains(out, "No topics.") {
		t.Fatalf("topic list after delete:\n%s", out)
	}
}
