package kb

import (
	"fmt"
	"testing"
)

// cliques builds groups of fully connected nodes "g<group>n<node>" and joins
// group i to group i+1 with a single edge.
func cliques(groups, size int, ring bool) ([]string, []Link) {
	var keys []string
	var links []Link
	key := func(g, n int) string { return fmt.Sprintf("g%dn%d", g, n) }
	for g := 0; g < groups; g++ {
		for a := 0; a < size; a++ {
			keys = append(keys, key(g, a))
			for b := a + 1; b < size; b++ {
				links = append(links, Link{From: key(g, a), Rel: RelReference, To: key(g, b)})
			}
		}
		if g+1 < groups || ring {
			links = append(links, Link{From: key(g, 0), Rel: RelReference, To: key((g+1)%groups, 1)})
		}
	}
	return keys, links
}

func TestLeidenCliques(t *testing.T) {
	keys, links := cliques(4, 6, true)
	keys = append(keys, "lonely")
	got := Leiden(keys, links, 0)
	for g := 0; g < 4; g++ {
		for n := 1; n < 6; n++ {
			if got[fmt.Sprintf("g%dn%d", g, n)] != got[fmt.Sprintf("g%dn0", g)] {
				t.Fatalf("clique %d split: %v", g, got)
			}
		}
		if g > 0 && got[fmt.Sprintf("g%dn0", g)] == got["g0n0"] {
			t.Fatalf("cliques 0 and %d merged: %v", g, got)
		}
	}
	if got["lonely"] != 4 { // the smallest community is numbered last
		t.Fatalf("isolated node: %v", got)
	}
	// Deterministic, independent of input order.
	rev := append([]string(nil), keys...)
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	again := Leiden(rev, links, 0)
	for k, c := range got {
		if again[k] != c {
			t.Fatalf("not deterministic: %v vs %v", got, again)
		}
	}
}

func TestLeidenResolutionAndEdgeCases(t *testing.T) {
	if got := Leiden(nil, nil, 1); len(got) != 0 {
		t.Fatalf("empty: %v", got)
	}
	if got := Leiden([]string{"a", "b"}, []Link{{From: "a", To: "x"}, {From: "a", To: "a"}}, 1); got["a"] == got["b"] {
		t.Fatalf("no edges: %v", got)
	}
	// A low resolution merges the cliques of a chain into fewer communities.
	keys, links := cliques(6, 4, false)
	count := func(m map[string]int) int {
		set := map[int]bool{}
		for _, c := range m {
			set[c] = true
		}
		return len(set)
	}
	if hi, lo := count(Leiden(keys, links, 1)), count(Leiden(keys, links, 0.05)); hi != 6 || lo >= hi {
		t.Fatalf("resolution: γ=1 → %d, γ=0.05 → %d", hi, lo)
	}
}

func TestStoreCommunities(t *testing.T) {
	s, _ := project(t, map[string]string{
		"a.md": "# A\n\n## One\ntext\n\n## Two\ntext\n",
		"b.md": "# B\n\n## Three\ntext\n\n## Four\ntext\n",
	})
	if _, err := s.Sync(); err != nil {
		t.Fatal(err)
	}
	got, err := s.Communities(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 6 || got["docs/a.md#one"] != got["docs/a.md"] || got["docs/b.md#four"] != got["docs/b.md"] || got["docs/a.md"] == got["docs/b.md"] {
		t.Fatalf("communities: %v", got)
	}
}
