package san_medium

import "testing"

func TestDraftID(t *testing.T) {
	for in, want := range map[string]string{
		"a379bedbcab6":                           "a379bedbcab6",
		" a379bedbcab6 ":                         "a379bedbcab6",
		"https://medium.com/p/a379bedbcab6/edit": "a379bedbcab6",
		"https://medium.com/p/a379bedbcab6/edit?source=your_stories_outbox---writer_outbox_drafts": "a379bedbcab6",
		"https://medium.com/p/a379bedbcab6":                               "a379bedbcab6",
		"https://medium.com/@someone/apa-itu-vpn-a379bedbcab6":            "a379bedbcab6",
		"https://medium.com/@someone/apa-itu-vpn-a379bedbcab6?source=rss": "a379bedbcab6",
	} {
		got, err := DraftID(in)
		if err != nil || got != want {
			t.Errorf("DraftID(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "hello", "https://medium.com/me/stories", "A379BEDBCAB6"} {
		if got, err := DraftID(in); err == nil {
			t.Errorf("DraftID(%q) = %q, want an error", in, got)
		}
	}
}
