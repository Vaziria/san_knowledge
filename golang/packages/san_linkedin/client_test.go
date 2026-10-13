package san_linkedin

import (
	"testing"

	san_profile_manager "github.com/wargasipil/san_profile_manager"
)

func TestHasSession(t *testing.T) {
	visitor := []san_profile_manager.Cookie{
		{Name: "bcookie", Domain: ".linkedin.com", Value: "v=2&x"},
		{Name: "lidc", Domain: ".linkedin.com", Value: "b=x"},
		{Name: "li_at", Domain: ".notlinkedin.com", Value: "x"},
		{Name: "sid", Domain: ".medium.com", Value: "1:abc"}, // another site's session in the shared profile
	}
	if hasSession(visitor) {
		t.Error("a visitor counts as signed in")
	}
	for _, d := range []string{".www.linkedin.com", "www.linkedin.com", ".linkedin.com"} {
		in := append(visitor, san_profile_manager.Cookie{Name: "li_at", Domain: d, Value: "AQED"})
		if !hasSession(in) {
			t.Errorf("an li_at on %s does not count as signed in", d)
		}
	}
}
