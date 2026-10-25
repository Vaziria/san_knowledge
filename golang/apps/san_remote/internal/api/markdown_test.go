package api

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	facebookv1 "github.com/wargasipil/san_remote/gen/san/remote/facebook/v1"
	threadsv1 "github.com/wargasipil/san_remote/gen/san/remote/threads/v1"
	tiktokv1 "github.com/wargasipil/san_remote/gen/san/remote/tiktok/v1"
	xv1 "github.com/wargasipil/san_remote/gen/san/remote/x/v1"
	"github.com/wargasipil/san_remote/internal/facebook"
	"github.com/wargasipil/san_remote/internal/medium"
)

func write(t *testing.T, path, text string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestThreadOfMarkdown(t *testing.T) {
	dir := t.TempDir()
	img := filepath.ToSlash(write(t, filepath.Join(dir, "a.png"), "png"))
	md := "---\nat: 2026-12-01 09:00\ntopic: Programming\n---\n\n# Halo\n\n**Tebal** dan [tautan](https://a.id)\n\n---\n\nDua\n\n![](" + img + ")\n"
	th, err := threadOf(&threadsv1.SchedulePostRequest{Markdown: md})
	if err != nil {
		t.Fatal(err)
	}
	if len(th.Posts) != 2 || th.Posts[0].Text != "Halo\n\nTebal dan tautan (https://a.id)" || th.Posts[0].Topic != "Programming" ||
		th.Posts[1].Text != "Dua" || len(th.Posts[1].Media) != 1 || th.Posts[1].Media[0] != img {
		t.Errorf("posts = %+v", th.Posts)
	}
	if want := time.Date(2026, 12, 1, 9, 0, 0, 0, time.Local); !th.At.Equal(want) {
		t.Errorf("at = %v, want the frontmatter's %v", th.At, want)
	}

	// The request's at and topic win over the frontmatter's.
	at := time.Date(2026, 12, 2, 10, 0, 0, 0, time.Local)
	th, err = threadOf(&threadsv1.SchedulePostRequest{Markdown: md, At: timestamppb.New(at), Topic: "Golang"})
	if err != nil || !th.At.Equal(at) || th.Posts[0].Topic != "Golang" {
		t.Errorf("at %v topic %q, %v", th.At, th.Posts[0].Topic, err)
	}

	// A file's images are relative to it.
	file := write(t, filepath.Join(dir, "thread.md"), "Satu\n\n![](a.png)\n")
	th, err = threadOf(&threadsv1.SchedulePostRequest{File: file, At: timestamppb.New(at)})
	if err != nil || th.Posts[0].Text != "Satu" || th.Posts[0].Media[0] != filepath.Join(dir, "a.png") {
		t.Errorf("file: %+v, %v", th, err)
	}
}

func TestMarkdownRefused(t *testing.T) {
	later := timestamppb.New(time.Now().Add(48 * time.Hour))
	dir := t.TempDir()
	file := write(t, filepath.Join(dir, "post.md"), "Halo")
	for name, err := range map[string]error{
		"file and markdown":       second(threadOf(&threadsv1.SchedulePostRequest{File: file, Markdown: "Halo", At: later})),
		"posts and markdown":      second(threadOf(&threadsv1.SchedulePostRequest{Posts: []*threadsv1.Post{{Text: "a"}}, Markdown: "Halo", At: later})),
		"relative file":           second(threadOf(&threadsv1.SchedulePostRequest{File: "post.md", At: later})),
		"no at anywhere":          second(threadOf(&threadsv1.SchedulePostRequest{Markdown: "Halo"})),
		"relative image in text":  second(threadOf(&threadsv1.SchedulePostRequest{Markdown: "Halo ![](a.png)", At: later})),
		"x relative image":        second(xThreadOf(&xv1.SchedulePostRequest{Markdown: "Halo\n\n![](a.png)", At: later})),
		"facebook text and file":  second(facebookPostOf(&facebookv1.SchedulePostRequest{Text: "a", File: file, At: later})),
		"facebook relative image": second(facebookPostOf(&facebookv1.SchedulePostRequest{Markdown: "![](a.png)", At: later})),
		"tiktok relative video":   second(tiktokPostOf(&tiktokv1.SchedulePostRequest{Markdown: "---\nvideo: v.mp4\n---\nHalo", At: later})),
		"tiktok no video":         second(tiktokPostOf(&tiktokv1.SchedulePostRequest{Markdown: "Halo", At: later})),
		"tiktok caption and file": second(tiktokPostOf(&tiktokv1.SchedulePostRequest{Caption: "a", File: file, At: later})),
		"article relative image":  second(articleOf("", "# Judul\n\n![](a.png)", medium.ParseFile, medium.Parse)),
		"article nothing":         second(articleOf("", " ", medium.ParseFile, medium.Parse)),
		"article file and text":   second(articleOf(file, "# Judul", medium.ParseFile, medium.Parse)),
	} {
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: %v, want InvalidArgument", name, err)
		}
	}
}

func TestPostsOfMarkdown(t *testing.T) {
	dir := t.TempDir()
	img := filepath.ToSlash(write(t, filepath.Join(dir, "a.png"), "png"))
	video := write(t, filepath.Join(dir, "v.mp4"), "mp4")
	at := timestamppb.New(time.Date(2026, 12, 2, 10, 0, 0, 0, time.Local))

	th, err := xThreadOf(&xv1.SchedulePostRequest{Markdown: "*Satu*\n\n---\n\n- dua\n- tiga\n\n![](" + img + ")", At: at, Browser: "b"})
	if err != nil || th.Browser != "b" || len(th.Posts) != 2 || th.Posts[0].Text != "Satu" || th.Posts[1].Text != "- dua\n- tiga" || len(th.Posts[1].Media) != 1 {
		t.Errorf("x: %+v, %v", th, err)
	}

	p, err := facebookPostOf(&facebookv1.SchedulePostRequest{Markdown: "---\naudience: friends\nat: 2026-12-01 09:00\n---\n**Halo** semua", Audience: "only_me"})
	if err != nil || p.Text != "Halo semua" || p.Audience != facebook.OnlyMe || p.At.IsZero() {
		t.Errorf("facebook: %+v, %v", p, err)
	}

	file := write(t, filepath.Join(dir, "tiktok.md"), "---\nvideo: v.mp4\n---\n#blender **Batu** low poly")
	tp, err := tiktokPostOf(&tiktokv1.SchedulePostRequest{File: file, At: at})
	if err != nil || tp.Video != video || tp.Caption != "#blender Batu low poly" {
		t.Errorf("tiktok file: %+v, %v", tp, err)
	}
	// A file that names no video, with the request's.
	file = write(t, filepath.Join(dir, "caption.md"), "Hanya caption")
	tp, err = tiktokPostOf(&tiktokv1.SchedulePostRequest{File: file, Video: video, At: at})
	if err != nil || tp.Video != video || tp.Caption != "Hanya caption" {
		t.Errorf("tiktok video given: %+v, %v", tp, err)
	}

	art, err := articleOf("", "# Judul\n\nIsi.\n\n![gambar]("+img+")\n", medium.ParseFile, medium.Parse)
	if err != nil || art.Title != "Judul" || len(art.Images()) != 1 || filepath.Clean(art.Images()[0].Path) != filepath.Clean(img) {
		t.Errorf("article: %+v, %v", art, err)
	}
}
