package studio

import (
	"testing"

	san_profile_manager "github.com/wargasipil/san_profile_manager"
)

func TestHasSession(t *testing.T) {
	type c = san_profile_manager.Cookie
	tests := []struct {
		name    string
		cookies []c
		want    bool
	}{
		{"none", nil, false},
		{"visitor", []c{{Name: "VISITOR_INFO1_LIVE", Domain: ".youtube.com", Value: "x"}, {Name: "YSC", Domain: ".youtube.com", Value: "y"}}, false},
		{"google only", []c{{Name: "SID", Domain: ".google.com", Value: "x"}}, false},
		{"signed in", []c{{Name: "SID", Domain: ".youtube.com", Value: "x"}}, true},
		{"empty SID", []c{{Name: "SID", Domain: ".youtube.com"}}, false},
		{"another site", []c{{Name: "SID", Domain: ".notyoutube.com", Value: "x"}}, false},
	}
	for _, tt := range tests {
		if got := hasSession(tt.cookies); got != tt.want {
			t.Errorf("%s: hasSession = %v, want %v", tt.name, got, tt.want)
		}
	}
}
