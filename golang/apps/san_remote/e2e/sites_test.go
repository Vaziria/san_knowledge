package e2e

import (
	"context"
	"errors"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wargasipil/san_remote/internal/chatgpt"
	"github.com/wargasipil/san_remote/internal/claude"
	"github.com/wargasipil/san_remote/internal/gemini"
	"github.com/wargasipil/san_remote/internal/hub"
	"github.com/wargasipil/san_remote/internal/linkedin"
	"github.com/wargasipil/san_remote/internal/medium"
	"github.com/wargasipil/san_remote/internal/page"
	"github.com/wargasipil/san_remote/internal/tiktok"
)

// The tests of the sites added on 2026-10-25. Each touches a real account,
// so each needs a variable of its own besides SAN_REMOTE_E2E=1, and each
// cleans up what it made, even when a step fails.

// remoteBrowser is serve's RemoteService as the site services use it, and
// its URL.
func remoteBrowser(t *testing.T, gate, what string) (page.Remote, string, string) {
	t.Helper()
	_, browser := client(t)
	if os.Getenv(gate) != "1" {
		t.Skip(what + ": set " + gate + "=1 too")
	}
	server := os.Getenv("SAN_REMOTE_SERVER")
	if server == "" {
		server = hub.DefaultServer
	}
	return page.Remote{C: hub.NewClient(server)}, browser, server
}

// testArticle writes a test article with a subtitle and two captioned
// images.
func testArticle(t *testing.T, title string) string {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"satu.png", "dua.png"} {
		if err := os.WriteFile(filepath.Join(dir, "assets", name), testPNG(t), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	md := "# " + title + "\n\n*Subjudul uji.*\n\nParagraf pembuka dengan **tebal** dan [tautan](https://example.com).\n\n" +
		"## Bagian\n\n- satu\n- dua\n\n![Gambar satu.](assets/satu.png)\n\n*Keterangan satu.*\n\n" +
		"> Kutipan.\n\n![Gambar dua.](assets/dua.png)\n\n*Keterangan dua.*\n\nParagraf penutup.\n"
	path := filepath.Join(dir, "uji.md")
	if err := os.WriteFile(path, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestMediumDraft writes a test article into a Medium draft and schedules it
// 30 days ahead with a topic, finds it in the Scheduled tab, unschedules it,
// and deletes it. Nothing is ever published. It needs SAN_REMOTE_E2E_MEDIUM=1.
func TestMediumDraft(t *testing.T) {
	b, browser, server := remoteBrowser(t, "SAN_REMOTE_E2E_MEDIUM", "makes and deletes a test draft on the Medium account")
	s := &medium.Service{B: b, Progress: func(step string) { t.Log("== " + step) }}
	ctx := context.Background()
	title := fmt.Sprintf("Uji san_remote Medium %s (hapus saja)", time.Now().Format("150405"))
	art, err := medium.ParseFile(testArticle(t, title))
	if err != nil {
		t.Fatal(err)
	}
	day := time.Now().AddDate(0, 0, 30)
	at := time.Date(day.Year(), day.Month(), day.Day(), 9, 0, 0, 0, time.Local)
	story, err := s.Draft(ctx, medium.Draft{Browser: browser, Article: art, At: at, Topics: []string{"Programming"}})
	if story != nil {
		t.Cleanup(func() {
			ctx := context.Background()
			if _, err := s.Unschedule(ctx, browser, story.ID); err != nil && !errors.Is(err, medium.ErrNotFound) {
				t.Errorf("cleanup: %s may still be scheduled: %v", story.URL, err)
			}
			if _, err := s.Delete(ctx, browser, story.ID); err != nil {
				t.Errorf("cleanup: the test draft %s may still be there: %v", story.URL, err)
			}
		})
	}
	if err != nil {
		t.Fatal(hub.ClientError(err, server))
	}
	list, err := s.Scheduled(ctx, browser)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, st := range list {
		found = found || (st.ID == story.ID && st.At.Equal(at))
	}
	if !found {
		t.Fatalf("the Scheduled tab does not list %s at %s: %+v", story.ID, at, list)
	}
}

// TestLinkedInDraft writes a test article with a cover into a LinkedIn draft
// and schedules it 30 days ahead, finds it in the Scheduled tab, moves it
// back to the drafts, and deletes it. Nothing is ever published. It needs
// SAN_REMOTE_E2E_LINKEDIN=1.
func TestLinkedInDraft(t *testing.T) {
	b, browser, server := remoteBrowser(t, "SAN_REMOTE_E2E_LINKEDIN", "makes and deletes a test draft on the LinkedIn account")
	s := &linkedin.Service{B: b, Progress: func(step string) { t.Log("== " + step) }}
	ctx := context.Background()
	title := fmt.Sprintf("Uji san_remote LinkedIn %s (hapus saja)", time.Now().Format("150405"))
	path := testArticle(t, title)
	art, err := linkedin.ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := art.SetCover(filepath.Join(filepath.Dir(path), "assets", "satu.png")); err != nil {
		t.Fatal(err)
	}
	day := time.Now().AddDate(0, 0, 30)
	at := time.Date(day.Year(), day.Month(), day.Day(), 9, 0, 0, 0, time.Local)
	story, err := s.Draft(ctx, linkedin.Draft{Browser: browser, Article: art, At: at, Text: "Uji san_remote.\nBaris kedua."})
	t.Cleanup(func() {
		ctx := context.Background()
		id := ""
		if story != nil {
			id = story.ID
		}
		if back, err := s.Unschedule(ctx, browser, title, ""); err == nil {
			id = back.ID
		} else if !errors.Is(err, linkedin.ErrNotFound) {
			t.Errorf("cleanup: %q may still be scheduled: %v", title, err)
		}
		if id == "" {
			return
		}
		if _, err := s.Delete(ctx, browser, id); err != nil {
			t.Errorf("cleanup: the test draft %s may still be there: %v", id, err)
		}
	})
	if err != nil {
		t.Fatal(hub.ClientError(err, server))
	}
	list, err := s.Scheduled(ctx, browser)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, st := range list {
		found = found || (st.Title == title && st.At.Equal(at))
	}
	if !found {
		t.Fatalf("the Scheduled tab does not list %q at %s: %+v", title, at, list)
	}
}

// TestTikTokSchedule schedules a 6 s test video 9 days ahead for Everyone,
// finds it in Studio's Posts, and deletes it before it goes out. It needs
// SAN_REMOTE_E2E_TIKTOK=1, and ffmpeg.
func TestTikTokSchedule(t *testing.T) {
	b, browser, server := remoteBrowser(t, "SAN_REMOTE_E2E_TIKTOK", "schedules and deletes a test video on the TikTok account")
	ffmpeg, err := osexec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("needs ffmpeg on PATH to make the test video")
	}
	video := filepath.Join(t.TempDir(), "uji.mp4")
	if out, err := osexec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "color=c=0x2b6cb0:size=720x1280:rate=30:duration=6", "-f", "lavfi", "-i", "sine=frequency=440:duration=6",
		"-c:v", "libx264", "-crf", "32", "-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest", video).CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg: %v: %s", err, out)
	}
	s := &tiktok.Service{B: b, Progress: func(step string) { t.Log("== " + step) }}
	ctx := context.Background()
	caption := fmt.Sprintf("Uji san_remote %s, hapus saja.", time.Now().Format("150405"))
	day := time.Now().AddDate(0, 0, 9)
	at := time.Date(day.Year(), day.Month(), day.Day(), 19, 0, 0, 0, time.Local)
	when := at.Format("Jan 2, 3:04 PM")
	t.Cleanup(func() {
		if _, err := s.Delete(context.Background(), browser, caption, when); err != nil && !errors.Is(err, tiktok.ErrNoMatch) {
			t.Errorf("cleanup: the test video %q may still be scheduled: %v", caption, err)
		}
	})
	r, err := s.Post(ctx, tiktok.Post{Browser: browser, Video: video, Caption: caption + "\nBaris kedua #gamedev", At: at})
	if err != nil {
		t.Fatal(hub.ClientError(err, server))
	}
	if !strings.Contains(r.Listed, when) {
		t.Errorf("Studio lists it as %q, not at %s", r.Listed, when)
	}
}

// TestChatGPTAsk asks a short question in a Temporary chat, which ChatGPT
// does not keep. It needs SAN_REMOTE_E2E_CHATGPT=1.
func TestChatGPTAsk(t *testing.T) {
	b, browser, server := remoteBrowser(t, "SAN_REMOTE_E2E_CHATGPT", "asks ChatGPT a question in a Temporary chat")
	s := &chatgpt.Service{B: b, Progress: func(step string) { t.Log("== " + step) }}
	ans, err := s.Ask(context.Background(), chatgpt.Ask{Browser: browser, Prompt: "Jawab dalam markdown: daftar dua kota besar di Indonesia, tanpa penjelasan."})
	if err != nil {
		t.Fatal(hub.ClientError(err, server))
	}
	if !ans.Temporary || ans.Chat != "" || !strings.Contains(ans.Text, "- ") {
		t.Errorf("answer %+v", ans)
	}
	t.Log(ans.Text)
}

func TestGeminiAsk(t *testing.T) {
	b, browser, server := remoteBrowser(t, "SAN_REMOTE_E2E_GEMINI", "asks Gemini a question in a Temporary chat")
	s := &gemini.Service{B: b, Progress: func(step string) { t.Log("== " + step) }}
	ans, err := s.Ask(context.Background(), gemini.Ask{Browser: browser, Prompt: "Jawab dengan satu kata saja: warna langit siang hari yang cerah?"})
	if err != nil {
		t.Fatal(hub.ClientError(err, server))
	}
	if !ans.Temporary || ans.Chat != "" || strings.TrimSpace(ans.Text) == "" {
		t.Errorf("answer %+v", ans)
	}
	t.Log(ans.Text)
}

func TestClaudeAsk(t *testing.T) {
	b, browser, server := remoteBrowser(t, "SAN_REMOTE_E2E_CLAUDE", "asks Claude on claude.ai a question in an incognito chat")
	s := &claude.Service{B: b, Progress: func(step string) { t.Log("== " + step) }}
	ans, err := s.Ask(context.Background(), claude.Ask{Browser: browser, Prompt: "Jawab dalam markdown: daftar dua kota besar di Indonesia, tanpa penjelasan."})
	if err != nil {
		t.Fatal(hub.ClientError(err, server))
	}
	if !ans.Incognito || ans.Chat != "" || !strings.Contains(ans.Text, "- ") {
		t.Errorf("answer %+v", ans)
	}
	t.Log(ans.Text)
}
