package e2e

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wargasipil/san_remote/internal/facebook"
	"github.com/wargasipil/san_remote/internal/hub"
	"github.com/wargasipil/san_remote/internal/page"
)

// TestFacebookSchedule goes through a scheduled profile post's life on the
// real account: it schedules an Only-me test post with an image 25 days
// ahead, finds it in the Content Library, and deletes it. It touches the
// Facebook account, so it also needs SAN_REMOTE_E2E_FACEBOOK=1. Nothing is
// posted now, and only the user can see the post while it exists.
func TestFacebookSchedule(t *testing.T) {
	_, browser := client(t)
	if os.Getenv("SAN_REMOTE_E2E_FACEBOOK") != "1" {
		t.Skip("schedules and deletes an Only-me test post on the Facebook profile: set SAN_REMOTE_E2E_FACEBOOK=1 too")
	}
	server := os.Getenv("SAN_REMOTE_SERVER")
	if server == "" {
		server = hub.DefaultServer
	}
	s := &facebook.Service{B: page.Remote{C: hub.NewClient(server)}, Progress: func(step string) { t.Log("== " + step) }}
	ctx := context.Background()

	st, err := s.Status(ctx, browser)
	if err != nil {
		t.Fatal(hub.ClientError(err, server))
	}
	if !st.SignedIn {
		t.Fatal("the browser is not signed in to Facebook")
	}
	t.Logf("signed in as %s, %d Pages, %d groups", st.Name, len(st.Pages), len(st.Groups))

	img := filepath.Join(t.TempDir(), "uji.png")
	if err := os.WriteFile(img, testPNG(t), 0o644); err != nil {
		t.Fatal(err)
	}
	day := time.Now().AddDate(0, 0, 25)
	at := time.Date(day.Year(), day.Month(), day.Day(), 9, 0, 0, 0, time.Local)
	text := fmt.Sprintf("Uji jadwal san_remote %s (hanya saya, hapus saja)", time.Now().Format("150405"))
	// Whatever fails, from here on, the test post must not stay scheduled:
	// Post can fail after Facebook already took it.
	t.Cleanup(func() {
		_, err := s.Delete(context.Background(), browser, "scheduled", text)
		if err != nil && !errors.Is(err, facebook.ErrNoMatch) {
			t.Errorf("cleanup: the test post %q may still be scheduled: %v", text, err)
		}
	})
	posted, err := s.Post(ctx, facebook.Post{Browser: browser, Target: facebook.Profile, Audience: facebook.OnlyMe, Text: text, Media: []string{img}, At: at})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("scheduled: %s", posted.Shown)

	find := func() *facebook.Listed {
		t.Helper()
		rows, err := s.Posts(ctx, browser, "scheduled")
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			if strings.HasPrefix(r.Text, text) {
				return &r
			}
		}
		return nil
	}
	if find() == nil {
		t.Fatal("the Content Library's Terjadwal does not have the test post")
	}

	gone, err := s.Delete(ctx, browser, "scheduled", text)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("deleted: %s %s", gone.Status, gone.When)
	if r := find(); r != nil {
		t.Fatalf("still scheduled after delete: %s", r.When)
	}
}
