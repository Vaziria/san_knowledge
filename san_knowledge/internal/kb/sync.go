package kb

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	graphdb "github.com/mstrYoda/goraphdb"
)

// DocsDir is where tracked documents live, relative to the project root.
const DocsDir = "docs"

// AuthorFrontmatter marks summary/keyword/domain taken from a doc's frontmatter.
const AuthorFrontmatter = "frontmatter"

// SyncReport describes what a structural sync changed.
type SyncReport struct {
	DocsAdded         []string `json:"docs_added"`
	DocsUpdated       []string `json:"docs_updated"`
	DocsRemoved       []string `json:"docs_removed"`
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

// ScanDocs returns the project-relative locs of all markdown files in docs/.
func (s *Store) ScanDocs() ([]string, error) {
	root := filepath.Join(s.Root, DocsDir)
	var locs []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return filepath.SkipDir
			}
			return err
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(p), ".md") {
			return nil
		}
		rel, err := filepath.Rel(s.Root, p)
		if err != nil {
			return err
		}
		locs = append(locs, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(locs)
	return locs, err
}

// ParseDoc reads and parses a tracked doc by its loc.
func (s *Store) ParseDoc(loc string) (*ParsedDoc, error) {
	data, err := os.ReadFile(filepath.Join(s.Root, filepath.FromSlash(loc)))
	if err != nil {
		return nil, err
	}
	return ParseMarkdown(loc, data), nil
}

// Sync makes the graph match the markdown files in docs/: doc and
// doc_section nodes, section_of edges and frontmatter domains. It never calls
// an AI; changed content is flagged needs_summary instead.
func (s *Store) Sync() (*SyncReport, error) {
	r := &SyncReport{DocsAdded: []string{}, DocsUpdated: []string{}, DocsRemoved: []string{}}
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
	for _, loc := range locs {
		if doc := parsed[loc]; doc != nil {
			if err := s.syncDoc(doc, r); err != nil {
				r.Errors = append(r.Errors, loc+": "+err.Error())
				delete(parsed, loc)
			}
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
		r.DocsRemoved = append(r.DocsRemoved, d.Key)
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
		n := Node{Key: doc.Loc, NodeType: TypeDoc, Title: doc.Title, Loc: doc.Loc, Hash: doc.Hash, NeedsSummary: pending, Author: AuthorSync}
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
		if gn.GetString("hash") != doc.Hash || gn.GetString("title") != doc.Title {
			patch := graphdb.Props{"title": doc.Title, "hash": doc.Hash, "needs_summary": pending}
			if fm.Summary != "" {
				patch["summary"], patch["needs_summary"], patch["author"] = fm.Summary, false, AuthorFrontmatter
			}
			if len(fm.Keyword) > 0 {
				patch["keyword"] = NormalizeKeyword(fm.Keyword)
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
