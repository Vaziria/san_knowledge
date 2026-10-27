package kb

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	graphdb "github.com/mstrYoda/goraphdb"
)

// DocsDir is where documents live, relative to the project root. It is always
// tracked, since saved web pages go there too.
const DocsDir = "docs"

// ConfigFile, in the data directory, lists the folders tracked besides docs/,
// relative to the project root ("." tracks the whole project), and the folders
// or files left out of them:
//
//	{"track": ["."], "exclude": ["project/sell_to_agensi"]}
const ConfigFile = "config.json"

// Config is the data directory's ConfigFile. A missing file is an empty config.
type Config struct {
	Track   []string `json:"track"`   // folders whose markdown files are tracked besides docs/
	Exclude []string `json:"exclude"` // folders or files never tracked, even inside a tracked folder
}

// LoadConfig reads the ConfigFile in the data directory dir.
func LoadConfig(dir string) (Config, error) {
	var c Config
	data, err := os.ReadFile(filepath.Join(dir, ConfigFile))
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return c, fmt.Errorf("%s: %w", ConfigFile, err)
	}
	return c, nil
}

// TrackedDirs returns the project-relative folders whose markdown files are
// tracked: docs/ first, then the config's, without repeats. An unreadable
// config is an error, so a typo never makes sync delete docs.
func TrackedDirs(dir string) ([]string, error) {
	c, err := LoadConfig(dir)
	if err != nil {
		return nil, err
	}
	track, err := projectPaths("track", c.Track)
	if err != nil {
		return nil, err
	}
	dirs := []string{DocsDir}
	seen := map[string]bool{DocsDir: true}
	for _, d := range track {
		if !seen[d] {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}
	return dirs, nil
}

// ExcludedPaths returns the config's project-relative folders and files that
// are never tracked, even inside a tracked folder.
func ExcludedPaths(dir string) ([]string, error) {
	c, err := LoadConfig(dir)
	if err != nil {
		return nil, err
	}
	return projectPaths("exclude", c.Exclude)
}

// projectPaths cleans a config list into slash-separated paths inside the project.
func projectPaths(field string, list []string) ([]string, error) {
	var paths []string
	for _, p := range list {
		clean := filepath.Clean(filepath.FromSlash(strings.TrimSpace(p)))
		if strings.TrimSpace(p) == "" || !filepath.IsLocal(clean) {
			return nil, fmt.Errorf("%s: %s %q must be a path inside the project", ConfigFile, field, p)
		}
		paths = append(paths, filepath.ToSlash(clean))
	}
	return paths, nil
}

// isExcluded reports whether the project-relative loc is, or is inside, one of excluded.
func isExcluded(loc string, excluded []string) bool {
	for _, e := range excluded {
		if loc == e || strings.HasPrefix(loc, e+"/") {
			return true
		}
	}
	return false
}

// skipDir reports folders never scanned: hidden ones (.git, .vite) and
// installed packages, whose READMEs are not the project's knowledge.
func skipDir(name string) bool {
	return strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor"
}

// AuthorFrontmatter marks summary/keyword/domain taken from a doc's frontmatter.
const AuthorFrontmatter = "frontmatter"

// SyncReport describes what a structural sync changed.
type SyncReport struct {
	DocsAdded         []string `json:"docs_added"`
	DocsUpdated       []string `json:"docs_updated"`
	DocsRemoved       []string `json:"docs_removed"`
	DocsMoved         []string `json:"docs_moved"` // "old → new"; not counted as added or removed
	SectionsAdded     int      `json:"sections_added"`
	SectionsUpdated   int      `json:"sections_updated"`
	SectionsRemoved   int      `json:"sections_removed"`
	SummariesKept     int      `json:"summaries_carried_over"`
	ReferencesAdded   int      `json:"references_added"`
	ReferencesRemoved int      `json:"references_removed"`
	NeedsSummary      int      `json:"needs_summary"`
	Errors            []string `json:"errors,omitempty"`
}

// IsHumanAuthor reports whether summary/keyword were written by a person (or
// frontmatter) and must not be overwritten by the AI step.
func IsHumanAuthor(author string) bool {
	return author != "" && author != AuthorSync && author != AuthorAI
}

// ScanDocs returns the project-relative locs of all markdown files in the
// tracked folders (TrackedDirs) but not in the excluded ones (ExcludedPaths),
// sorted and without repeats.
func (s *Store) ScanDocs() ([]string, error) {
	dirs, err := TrackedDirs(s.Dir)
	if err != nil {
		return nil, err
	}
	excluded, err := ExcludedPaths(s.Dir)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var locs []string
	for _, dir := range dirs {
		root := filepath.Join(s.Root, filepath.FromSlash(dir))
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return filepath.SkipDir
				}
				return err
			}
			if d.IsDir() {
				if p != root && skipDir(d.Name()) {
					return filepath.SkipDir
				}
			} else if !strings.EqualFold(filepath.Ext(p), ".md") {
				return nil
			}
			rel, err := filepath.Rel(s.Root, p)
			if err != nil {
				return err
			}
			loc := filepath.ToSlash(rel)
			if isExcluded(loc, excluded) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if !d.IsDir() && !seen[loc] {
				seen[loc] = true
				locs = append(locs, loc)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(locs)
	return locs, nil
}

// ParseDoc reads and parses a tracked doc by its loc.
func (s *Store) ParseDoc(loc string) (*ParsedDoc, error) {
	data, err := os.ReadFile(filepath.Join(s.Root, filepath.FromSlash(loc)))
	if err != nil {
		return nil, err
	}
	return ParseMarkdown(loc, data), nil
}

// Sync makes the graph match the markdown files in the tracked folders: doc and
// doc_section nodes, section_of edges and frontmatter domains. It never calls
// an AI; changed content is flagged needs_summary instead.
func (s *Store) Sync() (*SyncReport, error) {
	r := &SyncReport{DocsAdded: []string{}, DocsUpdated: []string{}, DocsRemoved: []string{}, DocsMoved: []string{}}
	locs, err := s.ScanDocs()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	parsed := map[string]*ParsedDoc{}
	for _, loc := range locs {
		seen[loc] = true
		doc, err := s.ParseDoc(loc)
		if err != nil {
			r.Errors = append(r.Errors, loc+": "+err.Error())
			continue
		}
		parsed[loc] = doc
	}
	moves, err := s.findMoves(seen, parsed)
	if err != nil {
		return nil, err
	}
	for _, loc := range locs {
		if doc := parsed[loc]; doc != nil {
			if err := s.syncDoc(doc, r); err != nil {
				r.Errors = append(r.Errors, loc+": "+err.Error())
				delete(parsed, loc)
			}
		}
	}
	movedFrom := map[string]bool{}
	for _, loc := range locs {
		if old := moves[loc]; old != nil && parsed[loc] != nil {
			if err := s.carryMove(old, parsed[loc], r); err != nil {
				r.Errors = append(r.Errors, loc+": "+err.Error())
				continue
			}
			movedFrom[old.Key] = true
			r.DocsAdded = without(r.DocsAdded, loc)
		}
	}
	// References need every doc and section to exist, so they come last.
	for _, loc := range locs {
		if doc := parsed[loc]; doc != nil {
			if err := s.syncReferences(doc, parsed, r); err != nil {
				r.Errors = append(r.Errors, loc+": "+err.Error())
			}
		}
	}

	docs, err := s.List(Filter{NodeType: TypeDoc})
	if err != nil {
		return nil, err
	}
	for _, d := range docs {
		if seen[d.Key] {
			continue
		}
		sections, err := s.List(Filter{Loc: d.Key, NodeType: TypeSection})
		if err != nil {
			return nil, err
		}
		for _, sec := range sections {
			if err := s.Delete(sec.Key); err != nil {
				return nil, err
			}
			r.SectionsRemoved++
		}
		if err := s.Delete(d.Key); err != nil {
			return nil, err
		}
		if !movedFrom[d.Key] {
			r.DocsRemoved = append(r.DocsRemoved, d.Key)
		}
	}

	pending, err := s.List(Filter{NeedsSummary: true})
	if err != nil {
		return nil, err
	}
	r.NeedsSummary = len(pending)
	return r, nil
}

func (s *Store) syncDoc(doc *ParsedDoc, r *SyncReport) error {
	fm := doc.Front
	// An empty file has nothing to summarize.
	pending := strings.TrimSpace(doc.Text) != ""
	// --- doc node
	gn, err := s.nodeByKey(doc.Loc)
	switch {
	case errors.Is(err, ErrNotFound):
		n := Node{Key: doc.Loc, NodeType: TypeDoc, Title: doc.Title, Loc: doc.Loc, Hash: doc.Hash, NeedsSummary: pending, Author: AuthorSync,
			URI: fm.URI, LastFetched: fm.LastFetched}
		if fm.Summary != "" {
			n.Summary, n.NeedsSummary, n.Author = fm.Summary, false, AuthorFrontmatter
		}
		if len(fm.Keyword) > 0 {
			n.Keyword = fm.Keyword
		}
		if _, err := s.createNode(n); err != nil {
			return err
		}
		r.DocsAdded = append(r.DocsAdded, doc.Loc)
	case err != nil:
		return err
	default:
		if gn.GetString("node_type") != TypeDoc {
			return errors.New("key is already used by a " + gn.GetString("node_type"))
		}
		patch := graphdb.Props{}
		if gn.GetString("hash") != doc.Hash || gn.GetString("title") != doc.Title {
			patch["title"], patch["hash"], patch["needs_summary"] = doc.Title, doc.Hash, pending
		}
		if gn.GetString("uri") != fm.URI || gn.GetString("last_fetched") != fm.LastFetched {
			patch["uri"], patch["last_fetched"] = fm.URI, fm.LastFetched
		}
		if fm.Summary != "" && (gn.GetString("summary") != fm.Summary || gn.GetString("author") != AuthorFrontmatter) {
			patch["summary"], patch["needs_summary"], patch["author"] = fm.Summary, false, AuthorFrontmatter
		}
		if len(fm.Keyword) > 0 && strings.Join(stringList(gn.Props["keyword"]), ",") != strings.Join(NormalizeKeyword(fm.Keyword), ",") {
			patch["keyword"] = NormalizeKeyword(fm.Keyword)
		}
		if len(patch) > 0 {
			if fm.Summary != "" { // a frontmatter summary never goes pending
				patch["summary"], patch["needs_summary"], patch["author"] = fm.Summary, false, AuthorFrontmatter
			}
			if err := s.updateNode(gn, patch); err != nil {
				return err
			}
			r.DocsUpdated = append(r.DocsUpdated, doc.Loc)
		}
	}
	if err := s.syncFrontmatterDomains(doc); err != nil {
		return err
	}

	// --- sections
	existing, err := s.List(Filter{Loc: doc.Loc, NodeType: TypeSection})
	if err != nil {
		return err
	}
	old := map[string]*Node{}
	for _, n := range existing {
		old[n.Key] = n
	}
	parsed := map[string]bool{}
	for _, sec := range doc.Sections {
		parsed[sec.Key] = true
	}
	// Sections about to disappear, by content: a renamed heading with the same
	// text keeps its summary, keyword and domains.
	orphanByHash := map[string]*Node{}
	for _, n := range existing {
		if !parsed[n.Key] && n.Summary != "" && n.Hash != "" {
			orphanByHash[n.Hash] = n
		}
	}

	for _, sec := range doc.Sections {
		if cur, ok := old[sec.Key]; ok {
			gsec, err := s.nodeByKey(sec.Key)
			if err != nil {
				return err
			}
			patch := graphdb.Props{}
			if cur.LineLoc != sec.Line {
				patch["line_loc"] = sec.Line
			}
			if cur.Level != sec.Level {
				patch["level"] = sec.Level
			}
			changed := cur.Hash != sec.Hash || cur.Title != sec.Title
			if changed {
				patch["hash"], patch["title"], patch["needs_summary"] = sec.Hash, sec.Title, sec.Text != ""
				r.SectionsUpdated++
			} else if sec.Text == "" && cur.NeedsSummary {
				patch["needs_summary"] = false // heading with only subsections
			}
			if len(patch) > 0 {
				if err := s.updateNode(gsec, patch); err != nil {
					return err
				}
			}
		} else {
			// A heading with no text of its own (only subsections) has nothing to summarize.
			n := Node{Key: sec.Key, NodeType: TypeSection, Title: sec.Title, Loc: doc.Loc,
				LineLoc: sec.Line, Level: sec.Level, Hash: sec.Hash, NeedsSummary: sec.Text != "", Author: AuthorSync}
			var carried *Node
			if o, ok := orphanByHash[sec.Hash]; ok {
				carried = o
				n.Summary, n.Keyword, n.Author, n.NeedsSummary = o.Summary, o.Keyword, o.Author, o.NeedsSummary
				delete(orphanByHash, sec.Hash)
				r.SummariesKept++
			}
			if _, err := s.createNode(n); err != nil {
				return err
			}
			if carried != nil {
				ds, err := s.DomainsOf(carried.Key)
				if err != nil {
					return err
				}
				for _, d := range ds {
					if _, _, err := s.Link(Link{From: n.Key, Rel: RelDomainOf, To: d.Key, Author: carried.Author}); err != nil {
						return err
					}
				}
			}
			r.SectionsAdded++
		}
		if err := s.setParent(sec.Key, sec.ParentKey); err != nil {
			return err
		}
	}

	for _, n := range existing {
		if !parsed[n.Key] {
			if err := s.Delete(n.Key); err != nil {
				return err
			}
			r.SectionsRemoved++
		}
	}
	return nil
}

// syncReferences makes the reference edges leaving this doc and its sections
// match the links in the file. Only edges written by sync are removed.
func (s *Store) syncReferences(doc *ParsedDoc, docs map[string]*ParsedDoc, r *SyncReport) error {
	type pair struct{ from, to string }
	want := map[pair]bool{}
	for _, l := range doc.Links {
		to, ok := ResolveLink(doc.Loc, l.Target, docs)
		if ok && to != l.FromKey {
			want[pair{l.FromKey, to}] = true
		}
	}

	sources := []string{doc.Loc}
	for _, sec := range doc.Sections {
		sources = append(sources, sec.Key)
	}
	for _, from := range sources {
		out, _, err := s.Links(from)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return err
		}
		for _, l := range out {
			if l.Rel != RelReference {
				continue
			}
			p := pair{from, l.To}
			if want[p] {
				delete(want, p) // already present
				continue
			}
			if l.Author == AuthorSync {
				if _, err := s.Unlink(from, RelReference, l.To); err != nil {
					return err
				}
				r.ReferencesRemoved++
			}
		}
	}

	pairs := make([]pair, 0, len(want))
	for p := range want {
		pairs = append(pairs, p)
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].from != pairs[j].from {
			return pairs[i].from < pairs[j].from
		}
		return pairs[i].to < pairs[j].to
	})
	for _, p := range pairs {
		if _, created, err := s.Link(Link{From: p.from, Rel: RelReference, To: p.to, Author: AuthorSync}); err != nil {
			return err
		} else if created {
			r.ReferencesAdded++
		}
	}
	return nil
}

// findMoves pairs docs that are new at a loc with docs gone from theirs: first
// by identical text, then by the same file name and title (a task file moved
// between tasks/ folders and edited on the way). It returns new loc → old doc.
func (s *Store) findMoves(seen map[string]bool, parsed map[string]*ParsedDoc) (map[string]*Node, error) {
	docs, err := s.List(Filter{NodeType: TypeDoc})
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	var gone []*Node
	for _, d := range docs {
		known[d.Key] = true
		if !seen[d.Key] {
			gone = append(gone, d)
		}
	}
	var fresh []string
	for loc := range parsed {
		if !known[loc] {
			fresh = append(fresh, loc)
		}
	}
	sort.Strings(fresh)
	moves := map[string]*Node{}
	taken := map[string]bool{}
	pair := func(same func(d *Node, doc *ParsedDoc) bool) {
		for _, loc := range fresh {
			for _, d := range gone {
				if moves[loc] == nil && !taken[d.Key] && same(d, parsed[loc]) {
					moves[loc], taken[d.Key] = d, true
				}
			}
		}
	}
	pair(func(d *Node, doc *ParsedDoc) bool { return d.Hash != "" && d.Hash == doc.Hash })
	pair(func(d *Node, doc *ParsedDoc) bool { return path.Base(d.Key) == path.Base(doc.Loc) && d.Title == doc.Title })
	return moves, nil
}

// carryMove gives a moved doc and its sections what was written about them at
// the old loc: summary, keyword, author and domains. Text that changed on the
// way is flagged needs_summary and keeps the old summary until a new one is written.
func (s *Store) carryMove(old *Node, doc *ParsedDoc, r *SyncReport) error {
	pairs := [][2]string{{old.Key, doc.Loc}}
	hash := map[string]string{doc.Loc: doc.Hash}
	for _, sec := range doc.Sections {
		pairs = append(pairs, [2]string{old.Key + strings.TrimPrefix(sec.Key, doc.Loc), sec.Key})
		hash[sec.Key] = sec.Hash
	}
	for _, p := range pairs {
		from, err := s.Get(p[0])
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		to, err := s.nodeByKey(p[1])
		if err != nil {
			return err
		}
		if from.Summary != "" && to.GetString("author") != AuthorFrontmatter {
			patch := graphdb.Props{"summary": from.Summary, "author": from.Author, "needs_summary": from.NeedsSummary || from.Hash != hash[p[1]]}
			if len(from.Keyword) > 0 {
				patch["keyword"] = from.Keyword
			}
			if err := s.updateNode(to, patch); err != nil {
				return err
			}
		}
		out, _, err := s.Links(p[0])
		if err != nil {
			return err
		}
		for _, l := range out {
			if l.Rel == RelDomainOf && l.Author != AuthorFrontmatter { // frontmatter domains come from the file
				if _, _, err := s.Link(Link{From: p[1], Rel: RelDomainOf, To: l.To, Author: l.Author}); err != nil {
					return err
				}
			}
		}
	}
	r.DocsMoved = append(r.DocsMoved, old.Key+" → "+doc.Loc)
	return nil
}

func without(list []string, v string) []string {
	out := list[:0]
	for _, x := range list {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}

// setParent makes section_of point to exactly one parent.
func (s *Store) setParent(key, parentKey string) error {
	gn, err := s.nodeByKey(key)
	if err != nil {
		return err
	}
	parent, err := s.nodeByKey(parentKey)
	if err != nil {
		return err
	}
	edges, err := s.db.OutEdgesLabeled(gn.ID, RelSectionOf)
	if err != nil {
		return err
	}
	ok := false
	for _, e := range edges {
		if e.To == parent.ID && !ok {
			ok = true
			continue
		}
		if err := s.db.DeleteEdge(e.ID); err != nil {
			return err
		}
	}
	if ok {
		return nil
	}
	_, _, err = s.Link(Link{From: key, Rel: RelSectionOf, To: parentKey, Author: AuthorSync})
	return err
}

// syncFrontmatterDomains adds domain_of edges for "domain:" frontmatter and
// removes frontmatter edges that are no longer declared.
func (s *Store) syncFrontmatterDomains(doc *ParsedDoc) error {
	want := map[string]bool{}
	for _, title := range doc.Front.Domain {
		d, _, err := s.AssignDomain(doc.Loc, title, AuthorFrontmatter)
		if err != nil {
			return err
		}
		want[d.Key] = true
	}
	out, _, err := s.Links(doc.Loc)
	if err != nil {
		return err
	}
	for _, l := range out {
		if l.Rel == RelDomainOf && l.Author == AuthorFrontmatter && !want[l.To] {
			if _, err := s.Unlink(l.From, RelDomainOf, l.To); err != nil {
				return err
			}
		}
	}
	return nil
}
