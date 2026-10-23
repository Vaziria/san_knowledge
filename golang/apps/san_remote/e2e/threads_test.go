package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	threadsv1 "github.com/wargasipil/san_remote/gen/san/remote/threads/v1"
	"github.com/wargasipil/san_remote/internal/hub"
	"github.com/wargasipil/san_remote/internal/threads"
)

// TestThreadsSchedule goes through a scheduled thread's whole life on the
// real account: it schedules a two-post test thread 30 days ahead, with a
// topic and an image, finds it among the scheduled ones, moves it a day
// later, and deletes it. It touches the Threads account, so it also needs
// SAN_REMOTE_E2E_THREADS=1. Nothing is ever posted now.
func TestThreadsSchedule(t *testing.T) {
	_, browser := client(t)
	if os.Getenv("SAN_REMOTE_E2E_THREADS") != "1" {
		t.Skip("schedules and deletes a test thread on the Threads account: set SAN_REMOTE_E2E_THREADS=1 too")
	}
	server := os.Getenv("SAN_REMOTE_SERVER")
	if server == "" {
		server = hub.DefaultServer
	}
	s := &threads.Service{B: threads.Remote{C: hub.NewClient(server)}, Progress: func(step string) { t.Log("== " + step) }}
	ctx := context.Background()

	st, err := s.Status(ctx, browser)
	if err != nil {
		t.Fatal(hub.ClientError(err, server))
	}
	if !st.SignedIn {
		t.Fatal("the browser is not signed in to Threads")
	}
	t.Logf("signed in as @%s", st.Username)

	day := time.Now().AddDate(0, 0, 30)
	at := time.Date(day.Year(), day.Month(), day.Day(), 9, 0, 0, 0, time.Local)
	text := fmt.Sprintf("Uji jadwal san_remote %s (hapus saja)", time.Now().Format("150405"))
	// Whatever fails, from here on, the test thread must not stay scheduled:
	// Post can fail after Threads already took it.
	t.Cleanup(func() {
		_, err := s.Unschedule(context.Background(), browser, text, time.Time{})
		if err != nil && !errors.Is(err, threads.ErrNoMatch) {
			t.Errorf("cleanup: the test thread %q may still be scheduled: %v", text, err)
		}
	})
	posted, err := s.Post(ctx, &threadsv1.PostRequest{
		Browser: browser,
		Posts: []*threadsv1.Post{
			{Text: text + "\nbaris kedua", Topic: "Programming", Media: []*threadsv1.Media{{Name: "uji.png", Data: testPNG(t)}}},
			{Text: "Post kedua dari uji yang sama."},
		},
		At: timestamppb.New(at),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("scheduled: %s", posted.Shown)

	find := func(want time.Time) *threadsv1.ScheduledPost {
		t.Helper()
		posts, err := s.ListScheduled(ctx, browser)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range posts {
			if strings.HasPrefix(p.Text, text) {
				if !p.At.AsTime().Equal(want) {
					t.Fatalf("scheduled at %s (%s), want %s", p.At.AsTime().Local(), p.Shown, want)
				}
				return p
			}
		}
		return nil
	}
	if find(at) == nil {
		t.Fatal("the scheduled list does not have the test thread")
	}

	later := at.AddDate(0, 0, 1)
	moved, err := s.Reschedule(ctx, browser, text, time.Time{}, later)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("moved: %s", moved.Shown)
	if find(later) == nil {
		t.Fatal("the scheduled list lost the test thread after the move")
	}

	gone, err := s.Unschedule(ctx, browser, text, later)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("deleted: %s", gone.Shown)
	if p := find(later); p != nil {
		t.Fatalf("still scheduled after unschedule: %s", p.Shown)
	}
}

// testPNG is a small blue square.
func testPNG(t *testing.T) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 320, 320))
	for x := range 320 {
		for y := range 320 {
			img.Set(x, y, color.RGBA{0x39, 0x87, 0xe5, 0xff})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
