package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The post steps' Format (Plain or Markdown) and Source (the text input or
// a File): markdown goes to San Remote as it is, to be read as a post file.
func TestPostFormats(t *testing.T) {
	f, env := remote(t)
	dir := t.TempDir()
	md := "---\nat: 2026-11-20 09:00\n---\n**Halo**\n---\nDua"

	// Markdown text: no At needed, the frontmatter may give it.
	if _, err := run(t, env, "threads-post", "Connection", "1", "Topic", "Golang", "Format", "Markdown", "Text", md); err != nil {
		t.Fatal(err)
	}
	if r := f.threads; r.GetMarkdown() != md || r.GetFile() != "" || len(r.GetPosts()) != 0 || r.GetAt() != nil || r.GetTopic() != "Golang" {
		t.Errorf("markdown text: %v", r)
	}

	// A markdown file, with At.
	file := filepath.Join(dir, "thread.md")
	if _, err := run(t, env, "threads-post", "Connection", "1", "Format", "Markdown", "Source", "File", "File", file, "Text", "ignored", "At", "+1d"); err != nil {
		t.Fatal(err)
	}
	if r := f.threads; r.GetFile() != file || r.GetMarkdown() != "" || len(r.GetPosts()) != 0 || r.GetAt() == nil {
		t.Errorf("markdown file: %v", r)
	}

	// A plain file is read here, and cut into posts.
	plain := filepath.Join(dir, "plain.txt")
	if err := os.WriteFile(plain, []byte("**satu**\r\n---\r\ndua"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, env, "threads-post", "Connection", "1", "Source", "File", "File", plain, "At", "+1d"); err != nil {
		t.Fatal(err)
	}
	if r := f.threads; r.GetFile() != "" || r.GetMarkdown() != "" || len(r.GetPosts()) != 2 || r.Posts[0].Text != "**satu**" || r.Posts[1].Text != "dua" {
		t.Errorf("plain file: %v", r)
	}

	// Plain text still needs At; a relative File never reaches San Remote.
	if _, err := run(t, env, "threads-post", "Connection", "1", "Text", "Halo"); err == nil || !strings.Contains(err.Error(), "only scheduled") {
		t.Errorf("plain without At: %v", err)
	}
	if _, err := run(t, env, "threads-post", "Connection", "1", "Format", "Markdown", "Source", "File", "File", "thread.md"); err == nil || !strings.Contains(err.Error(), "absolute path") {
		t.Errorf("relative file: %v", err)
	}
	if _, err := run(t, env, "threads-post", "Connection", "1", "Format", "Markdown", "Text", "  "); err == nil || !strings.Contains(err.Error(), "Text is empty") {
		t.Errorf("empty markdown: %v", err)
	}

	if _, err := run(t, env, "facebook-post", "Connection", "1", "Format", "Markdown", "Text", md); err != nil {
		t.Fatal(err)
	}
	if r := f.facebook; r.GetMarkdown() != md || r.GetText() != "" || r.GetAt() != nil {
		t.Errorf("facebook markdown: %v", r)
	}

	// TikTok: a markdown file may name the video itself.
	if _, err := run(t, env, "tiktok-post", "Connection", "1", "Format", "Markdown", "Source", "File", "File", file, "At", "+1d"); err != nil {
		t.Fatal(err)
	}
	if r := f.tiktok; r.GetFile() != file || r.GetVideo() != "" || r.GetCaption() != "" {
		t.Errorf("tiktok markdown file: %v", r)
	}
	if _, err := run(t, env, "tiktok-post", "Connection", "1", "Caption", "Halo", "At", "+1d"); err == nil || !strings.Contains(err.Error(), "Video") {
		t.Errorf("tiktok plain without video: %v", err)
	}

	// Articles: the markdown File, or the Article text.
	if _, err := run(t, env, "medium-draft", "Connection", "1", "Source", "Text", "Article", "# Judul\n\nIsi", "File", "ignored.md"); err != nil {
		t.Fatal(err)
	}
	if r := f.medium; r.GetMarkdown() != "# Judul\n\nIsi" || r.GetFile() != "" {
		t.Errorf("medium text: %v", r)
	}
	if _, err := run(t, env, "linkedin-draft", "Connection", "1", "Source", "Text", "Article", "# Judul"); err != nil {
		t.Fatal(err)
	}
	if r := f.linkedin; r.GetMarkdown() != "# Judul" || r.GetFile() != "" {
		t.Errorf("linkedin text: %v", r)
	}
	if _, err := run(t, env, "medium-draft", "Connection", "1", "Source", "Text"); err == nil || !strings.Contains(err.Error(), "Article is empty") {
		t.Errorf("empty article: %v", err)
	}
}
