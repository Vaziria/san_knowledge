// Package pagetest has a fake browser for the site services' tests: it takes
// the files a service stages into a page (stage(id, piece) calls, in base64)
// and keeps the pieces, so a test can decode them the way the site's page
// script does.
package pagetest

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	v1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
	"github.com/wargasipil/san_remote/internal/page"
)

// ErrNotStage is what Pieces answers to any call but stage.
var ErrNotStage = errors.New("pagetest: only stage is answered")

// Pieces is a browser with one tab, whose page takes stage calls and keeps
// their pieces, by file id.
type Pieces struct {
	page.Browser
	Pieces map[string][]string
}

func (b *Pieces) OpenTab(context.Context, *v1.OpenTabRequest) (*v1.OpenTabResponse, error) {
	return &v1.OpenTabResponse{Tab: &v1.Tab{Id: 1}}, nil
}

func (b *Pieces) CloseTab(context.Context, *v1.CloseTabRequest) (*v1.CloseTabResponse, error) {
	return &v1.CloseTabResponse{}, nil
}

// stageCall is the end of the code page.Session.Call runs for stage.
var stageCall = regexp.MustCompile(`\.stage\(([^\n]*)\);$`)

func (b *Pieces) Exec(_ context.Context, r *v1.ExecRequest) (*v1.ExecResponse, error) {
	m := stageCall.FindStringSubmatch(r.GetCode())
	if m == nil {
		return nil, ErrNotStage
	}
	var args []string
	if err := json.Unmarshal([]byte("["+m[1]+"]"), &args); err != nil || len(args) != 2 {
		return nil, fmt.Errorf("pagetest: stage(%.80s): %v", m[1], err)
	}
	if b.Pieces == nil {
		b.Pieces = map[string][]string{}
	}
	b.Pieces[args[0]] = append(b.Pieces[args[0]], args[1])
	return &v1.ExecResponse{ResultJson: fmt.Sprint(len(b.Pieces[args[0]]))}, nil
}

// File writes a file of n bytes that are not all alike, and returns its
// path and its bytes.
func File(t *testing.T, name string, n int) (string, []byte) {
	t.Helper()
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i*7 + i/251)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return path, b
}

// Joined checks that the pieces of id, joined and then decoded, as x.js,
// facebook.js, medium.js and threads.js decode them, are want.
func (b *Pieces) Joined(t *testing.T, id string, want []byte) {
	t.Helper()
	ps := b.Pieces[id]
	got, err := base64.StdEncoding.DecodeString(strings.Join(ps, ""))
	if err != nil {
		t.Fatalf("%d pieces of %s, joined, are not base64 the page can decode: %v", len(ps), id, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s: the page would get %d bytes, not the file's %d", id, len(got), len(want))
	}
}

// Each checks that the pieces of id, each decoded and then put together, as
// linkedin.js, tiktok.js and vidara.js decode them, are want.
func (b *Pieces) Each(t *testing.T, id string, want []byte) {
	t.Helper()
	var got []byte
	for i, p := range b.Pieces[id] {
		d, err := base64.StdEncoding.DecodeString(p)
		if err != nil {
			t.Fatalf("piece %d of %s is not base64 the page can decode: %v", i+1, id, err)
		}
		got = append(got, d...)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s: the page would get %d bytes, not the file's %d", id, len(got), len(want))
	}
}

// Size is a file size that takes several pieces of every site, with a last
// one of odd length: 8 MiB and 5 bytes.
const Size = 8<<20 + 5
