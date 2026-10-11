package san_tiktok

import (
	"testing"

	san_profile_manager "github.com/wargasipil/san_profile_manager"
)

func TestHasSession(t *testing.T) {
	visitor := []san_profile_manager.Cookie{
		{Name: "ttwid", Domain: ".tiktok.com", Value: "1|abc"},
		{Name: "sid", Domain: ".medium.com", Value: "1:abc"}, // another site's session in the shared profile
	}
	if hasSession(visitor) {
		t.Error("a visitor counts as signed in")
	}
	in := append(visitor, san_profile_manager.Cookie{Name: "sessionid", Domain: ".tiktok.com", Value: "0f9c"})
	if !hasSession(in) {
		t.Error("a TikTok sessionid does not count as signed in")
	}
}
