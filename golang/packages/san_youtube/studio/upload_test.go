package studio

import "testing"

func TestPlainSpaces(t *testing.T) {
	tests := map[string]string{
		"7:00 PM":      "7:00 PM", // Studio's time, 2026-10-14
		"Oct 15, 2026": "Oct 15, 2026",
		"7:00 PM":           "7:00 PM",
		"":                  "",
	}
	for in, want := range tests {
		if got := plainSpaces(in); got != want {
			t.Errorf("plainSpaces(%q) = %q, want %q", in, got, want)
		}
	}
}
