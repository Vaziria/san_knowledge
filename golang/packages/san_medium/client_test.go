package san_medium

import (
	"testing"

	san_profile_manager "github.com/wargasipil/san_profile_manager"
)

func TestHasSession(t *testing.T) {
	visitor := []san_profile_manager.Cookie{
		{Name: "uid", Domain: ".medium.com", Value: "lo_123"},
		{Name: "sessionid", Domain: ".tiktok.com", Value: "x"}, // another site's session in the shared profile
	}
	if hasSession(visitor) {
		t.Error("a visitor counts as signed in")
	}
	in := append(visitor, san_profile_manager.Cookie{Name: "sid", Domain: ".medium.com", Value: "1:abc"})
	if !hasSession(in) {
		t.Error("a Medium sid does not count as signed in")
	}
}
