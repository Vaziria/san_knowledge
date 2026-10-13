package san_linkedin

import "testing"

func TestDraftID(t *testing.T) {
	for in, want := range map[string]string{
		"7515797311315378176":   "7515797311315378176",
		" 7515797311315378176 ": "7515797311315378176",
		"https://www.linkedin.com/article/edit/7515797311315378176/":                        "7515797311315378176",
		"https://www.linkedin.com/article/edit/7515797311315378176/?author=urn":             "7515797311315378176",
		"/article/edit/7515797311315378176/":                                                "7515797311315378176",
		"urn:li:fsd_firstPartyArticle:7515797311315378176":                                  "7515797311315378176",
		"https://www.linkedin.com/voyager/api/x/urn:li:linkedInArticle:7515797311315378176": "7515797311315378176",
	} {
		got, err := DraftID(in)
		if err != nil || got != want {
			t.Errorf("DraftID(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "hello", "12345", "https://www.linkedin.com/article/new/", "https://www.linkedin.com/pulse/apa-itu-vpn-heri-santoso-abcde"} {
		if got, err := DraftID(in); err == nil {
			t.Errorf("DraftID(%q) = %q, want an error", in, got)
		}
	}
}
