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

	"github.com/wargasipil/san_remote/internal/hub"
	"github.com/wargasipil/san_remote/internal/page"
	"github.com/wargasipil/san_remote/internal/x"
)

// TestXSchedule goes through a scheduled thread's whole life on the real X
// account: it schedules a two-post test thread 30 days ahead, with an image
// on the second post, finds it in Drafts' Scheduled tab, moves it a day
// later, and deletes it. It touches the X account, so it also needs
// SAN_REMOTE_E2E_X=1. Nothing is ever posted now.
func TestXSchedule(t *testing.T) {
	_, browser := client(t)
	if os.Getenv("SAN_REMOTE_E2E_X") != "1" {
		t.Skip("schedules and deletes a test thread on the X account: set SAN_REMOTE_E2E_X=1 too")
	}
	server := os.Getenv("SAN_REMOTE_SERVER")
	if server == "" {
		server = hub.DefaultServer
	}
	s := &x.Service{B: page.Remote{C: hub.NewClient(server)}, Progress: func(step string) { t.Log("== " + step) }}
	ctx := context.Background()

	st, err := s.Status(ctx, browser)
	if err != nil {
		t.Fatal(hub.ClientError(err, server))
	}
	if !st.SignedIn {
		t.Fatal("the browser is not signed in to X")
	}
	t.Logf("signed in as %s (@%s)", st.Name, st.Handle)

	img := filepath.Join(t.TempDir(), "uji.png")
	if err := os.WriteFile(img, testPNG(t), 0o644); err != nil {
		t.Fatal(err)
	}
	day := time.Now().AddDate(0, 0, 30)
	at := time.Date(day.Year(), day.Month(), day.Day(), 9, 0, 0, 0, time.Local)
	text := fmt.Sprintf("Uji jadwal san_remote %s (hapus saja)", time.Now().Format("150405"))
	// Whatever fails, from here on, the test thread must not stay scheduled:
	// Post can fail after X already took it.
	t.Cleanup(func() {
		_, err := s.Unschedule(context.Background(), browser, text, time.Time{})
		if err != nil && !errors.Is(err, x.ErrNoMatch) {
			t.Errorf("cleanup: the test thread %q may still be scheduled: %v", text, err)
		}
	})
	posted, err := s.Post(ctx, x.Thread{
		Browser: browser,
		Posts:   []x.Post{{Text: text + "\nbaris kedua"}, {Text: "Post kedua dari uji yang sama.", Media: []string{img}}},
		At:      at,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("scheduled: %s", posted.Shown)

	find := func(want time.Time) *x.Scheduled {
		t.Helper()
		list, err := s.ListScheduled(ctx, browser)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range list {
			if strings.HasPrefix(p.Text, text) {
				if !p.At.Equal(want) {
					t.Fatalf("scheduled at %s, want %s", p.Shown, want)
				}
				return &p
			}
		}
		return nil
	}
	if p := find(at); p == nil {
		t.Fatal("the Scheduled tab does not have the test thread")
	} else if p.More != "1 more post" {
		t.Errorf("the Scheduled tab says %q of the second post", p.More)
	}

	later := at.AddDate(0, 0, 1)
	moved, err := s.Reschedule(ctx, browser, text, time.Time{}, later)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("moved: %s", moved.Shown)

	gone, err := s.Unschedule(ctx, browser, text, later)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("deleted: %s", gone.Shown)
	if p := find(later); p != nil {
		t.Fatalf("still scheduled after unschedule: %s", p.Shown)
	}
}
