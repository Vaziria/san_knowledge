package server

import (
	"slices"
	"strings"
	"testing"

	remotev1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
)

// A San Remote connection's Browser is checked against the attached
// browsers as san_remote picks one: by id or name, any case; empty for
// the only one.
func TestTestBrowser(t *testing.T) {
	personal := &remotev1.Browser{Id: "b1", Name: "personal", ExtensionVersion: "0.1.0"}
	work := &remotev1.Browser{Id: "b2", Name: "work", ExtensionVersion: "0.1.0"}
	chrome1 := &remotev1.Browser{Id: "c1", Name: "Chrome", ExtensionVersion: "0.1.0"}
	chrome2 := &remotev1.Browser{Id: "c2", Name: "Chrome", ExtensionVersion: "0.1.0"}
	two := []*remotev1.Browser{personal, work}
	for _, c := range []struct {
		name     string
		attached []*remotev1.Browser
		sel      string
		ok       bool
		says     string
		browsers []string
	}{
		{"none attached", nil, "personal", false, "no browser is attached", nil},
		{"the only one", []*remotev1.Browser{personal}, "", true, "attached: personal", []string{"personal"}},
		{"several, none picked", two, "", false, "pick one under Browser", []string{"personal", "work"}},
		{"by name", two, "work", true, "steps drive work (b2)", []string{"personal", "work"}},
		{"by name, any case", two, " Personal ", true, "steps drive personal (b1)", []string{"personal", "work"}},
		{"by id", two, "b2", true, "steps drive work (b2)", []string{"personal", "work"}},
		{"not attached", two, "getresolved", false, `no attached browser is called "getresolved"`, []string{"personal", "work"}},
		{"same names, by name", []*remotev1.Browser{chrome1, chrome2}, "chrome", false, "give the id instead", []string{"c1", "c2"}},
		{"same names, by id", []*remotev1.Browser{chrome1, chrome2}, "c2", true, "steps drive Chrome (c2)", []string{"c1", "c2"}},
	} {
		r := testBrowser(c.attached, c.sel)
		if r.Ok != c.ok || !strings.Contains(r.Message, c.says) || !slices.Equal(r.Browsers, c.browsers) {
			t.Errorf("%s: got ok=%v %q %v, want ok=%v, saying %q, browsers %v", c.name, r.Ok, r.Message, r.Browsers, c.ok, c.says, c.browsers)
		}
	}
}
