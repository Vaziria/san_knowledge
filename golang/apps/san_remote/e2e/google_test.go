// Package e2e drives the user's real browser through a running
// `san_remote serve`. The tests are skipped unless SAN_REMOTE_E2E=1:
//
//	$env:SAN_REMOTE_E2E=1; go test ./e2e -v
package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/v1/remotev1connect"
	"github.com/wargasipil/san_remote/internal/hub"
)

func client(t *testing.T) (remotev1connect.RemoteServiceClient, string) {
	if os.Getenv("SAN_REMOTE_E2E") != "1" {
		t.Skip("drives the real browser: set SAN_REMOTE_E2E=1 to run it")
	}
	server := os.Getenv("SAN_REMOTE_SERVER")
	if server == "" {
		server = hub.DefaultServer
	}
	return hub.NewClient(server), os.Getenv("SAN_REMOTE_BROWSER")
}

// TestGoogleSearch searches Google the way a person would: it opens the home
// page with the window in front, types the query into the search box key by
// key, submits it, and reads the results page. The tab is left open to look at.
func TestGoogleSearch(t *testing.T) {
	c, browser := client(t)
	ctx := context.Background()
	const query = "cute cat"

	opened, err := c.OpenTab(ctx, connect.NewRequest(&v1.OpenTabRequest{Browser: browser, Url: "https://www.google.com/", Focus: true}))
	if err != nil {
		t.Fatal(hub.ClientError(err, hub.DefaultServer))
	}
	tab := opened.Msg.Tab.Id
	t.Logf("opened %s in tab %d", opened.Msg.Tab.Url, tab)

	q, _ := json.Marshal(query)
	typed := exec(t, c, browser, tab, fmt.Sprintf(`
const box = document.querySelector('textarea[name=q], input[name=q]');
if (!box) throw new Error('no search box on ' + location.href);
box.focus();
for (const ch of %s) {
  box.value += ch;
  box.dispatchEvent(new InputEvent('input', { bubbles: true, data: ch, inputType: 'insertText' }));
  await new Promise(r => setTimeout(r, 80 + Math.random() * 120));
}
await new Promise(r => setTimeout(r, 400));
// Submit once this script has returned: the navigation would cut its result off.
setTimeout(() => box.form.requestSubmit(), 50);
return box.value;`, q))
	if typed != string(q) {
		t.Fatalf("typed %s, want %s", typed, q)
	}

	// The results page loads in place of the home page; ask until it is there.
	var page struct {
		URL    string   `json:"url"`
		Ready  string   `json:"ready"`
		Query  string   `json:"query"`
		Titles []string `json:"titles"`
	}
	deadline := time.Now().Add(20 * time.Second)
	for {
		r, err := c.Exec(ctx, connect.NewRequest(&v1.ExecRequest{Browser: browser, TabId: tab, TimeoutMs: 5000, Code: `
return {
  url: location.href,
  ready: document.readyState,
  query: document.querySelector('textarea[name=q], input[name=q]')?.value ?? '',
  titles: [...document.querySelectorAll('#search a h3')].map(h => h.textContent).slice(0, 5),
};`}))
		if err == nil && json.Unmarshal([]byte(r.Msg.ResultJson), &page) == nil &&
			strings.Contains(page.URL, "/search") && page.Ready == "complete" && len(page.Titles) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no results page after 20s: %+v (last error: %v)", page, err)
		}
		time.Sleep(500 * time.Millisecond)
	}

	t.Logf("results page %s", page.URL)
	for _, title := range page.Titles {
		t.Logf("  %s", title)
	}
	if page.Query != query {
		t.Errorf("search box on the results page holds %q, want %q", page.Query, query)
	}
	found := false
	for _, title := range page.Titles {
		l := strings.ToLower(title)
		found = found || strings.Contains(l, "cat") || strings.Contains(l, "kucing")
	}
	if !found {
		t.Errorf("no result title mentions a cat: %q", page.Titles)
	}
}

// exec runs code in the tab and returns its JSON result.
func exec(t *testing.T, c remotev1connect.RemoteServiceClient, browser string, tab int32, code string) string {
	t.Helper()
	r, err := c.Exec(context.Background(), connect.NewRequest(&v1.ExecRequest{Browser: browser, TabId: tab, Code: code}))
	if err != nil {
		t.Fatal(err)
	}
	return r.Msg.ResultJson
}
