package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/wargasipil/san_remote/internal/hubtest"
)

func runCLI(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := root(strings.NewReader(stdin), &out).Run(context.Background(), append([]string{"san_remote"}, args...))
	return out.String(), err
}

func TestCommands(t *testing.T) {
	_, url := hubtest.Start(t)
	hubtest.Attach(t, url, "b1", "Chrome", nil)

	for _, c := range []struct {
		stdin string
		args  []string
		want  string
	}{
		{"", []string{"exec", "return", "1"}, `"return 1"`},
		{"return 2", []string{"exec", "-"}, `"return 2"`},
		{"", []string{"tabs"}, "Example"},
		{"", []string{"browsers"}, "Chrome"},
		{"", []string{"open", "https://example.org/"}, "#8"},
		{"", []string{"navigate", "--tab", "3", "https://example.org/"}, "#3"},
	} {
		out, err := runCLI(t, c.stdin, append([]string{"--server", url}, c.args...)...)
		if err != nil || !strings.Contains(out, c.want) {
			t.Errorf("%v: %q %v, want %q", c.args, out, err, c.want)
		}
	}
	if _, err := runCLI(t, "", "--server", url, "exec", "throw"); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("throw: %v", err)
	}
	if _, err := runCLI(t, "", "--server", url, "exec"); err == nil || !strings.Contains(err.Error(), "no code") {
		t.Errorf("no code: %v", err)
	}
}

func TestServeLoopbackOnly(t *testing.T) {
	_, err := runCLI(t, "", "serve", "--listen", "0.0.0.0:7476")
	if err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Errorf("err = %v", err)
	}
}

func TestNotRunning(t *testing.T) {
	_, err := runCLI(t, "", "--server", "http://127.0.0.1:1", "tabs")
	if err == nil || !strings.Contains(err.Error(), "san_remote serve") {
		t.Errorf("err = %v", err)
	}
}
