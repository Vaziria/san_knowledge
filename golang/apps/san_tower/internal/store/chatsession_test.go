package store

import (
	"log/slog"
	"testing"
)

// A session keeps a chat's link per site, replaces it, and forgets it when
// its workflow is deleted.
func TestChatSessions(t *testing.T) {
	s, err := Open(t.TempDir(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	w, err := s.CreateWorkflow("Chats")
	if err != nil {
		t.Fatal(err)
	}
	session, other := ChatSessionKey(w.ID, "session"), ChatSessionKey(w.ID+1, "session")
	if !IsChatSession(session) || IsChatSession("https://chatgpt.com/c/1") {
		t.Fatal("IsChatSession")
	}
	if link, err := s.ChatLink(session, "ChatGPT"); err != nil || link != "" {
		t.Fatalf("none yet: %q, %v", link, err)
	}
	for _, c := range [][3]string{
		{session, "ChatGPT", "https://chatgpt.com/c/1"}, {session, "Gemini", "https://gemini.google.com/app/a1"},
		{session, "ChatGPT", "https://chatgpt.com/c/2"}, {other, "ChatGPT", "https://chatgpt.com/c/9"},
	} {
		if err := s.SetChatLink(c[0], c[1], c[2]); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range [][3]string{
		{session, "ChatGPT", "https://chatgpt.com/c/2"}, {session, "Gemini", "https://gemini.google.com/app/a1"},
		{session, "Claude", ""}, {other, "ChatGPT", "https://chatgpt.com/c/9"},
	} {
		if link, err := s.ChatLink(c[0], c[1]); err != nil || link != c[2] {
			t.Errorf("%s on %s: %q, %v", c[0], c[1], link, err)
		}
	}
	if err := s.DeleteWorkflow(w.ID); err != nil {
		t.Fatal(err)
	}
	if link, _ := s.ChatLink(session, "ChatGPT"); link != "" {
		t.Errorf("kept after the workflow went: %q", link)
	}
	if link, _ := s.ChatLink(other, "ChatGPT"); link == "" {
		t.Error("another workflow's chat went too")
	}
}
