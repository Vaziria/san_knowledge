// Command knowledge is the unified knowledge-graph tool shared by people and
// AI agents. Run "knowledge --help" for usage.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	graphdb "github.com/mstrYoda/goraphdb"
	"github.com/urfave/cli/v3"

	"san_knowledge/internal/kb"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// run builds and executes the command tree; tests call it directly.
func run(argv []string, stdin io.Reader, out io.Writer) error {
	root := newApp(stdin, out)
	disableSliceSeparator(root)
	return root.Run(context.Background(), append([]string{"knowledge"}, stdinArgLast(argv)...))
}

// disableSliceSeparator applies to every command: urfave/cli reads the
// setting per command. Keywords still accept comma lists via values().
func disableSliceSeparator(cmd *cli.Command) {
	cmd.DisableSliceFlagSeparator = true
	for _, sub := range cmd.Commands {
		disableSliceSeparator(sub)
	}
}

// boolFlagNames never take a value, so a "-" after them is positional.
var boolFlagNames = map[string]bool{"json": true, "force": true, "no-open": true, "help": true, "h": true,
	"version": true, "v": true, "no-ai": true, "dry-run": true, "needs-summary": true, "no-sync": true, "refresh": true}

// stdinArgLast moves a positional "-" (read stdin) to the end. urfave/cli stops
// parsing flags at "-", which would silently ignore e.g. "import - --json".
func stdinArgLast(argv []string) []string {
	out := make([]string, 0, len(argv))
	moved := 0
	for i, a := range argv {
		if a == "--" {
			out = append(out, argv[i:]...)
			break
		}
		if a == "-" && i > 0 {
			prev := argv[i-1]
			isFlagValue := strings.HasPrefix(prev, "-") && prev != "-" && !strings.Contains(prev, "=") &&
				!boolFlagNames[strings.TrimLeft(prev, "-")]
			if !isFlagValue {
				moved++
				continue
			}
		}
		out = append(out, a)
	}
	for ; moved > 0; moved-- {
		out = append(out, "-")
	}
	return out
}

func newApp(stdin io.Reader, out io.Writer) *cli.Command {
	listFlags := func() []cli.Flag {
		return []cli.Flag{
			&cli.StringFlag{Name: "type", Usage: "node_type: domain, doc or doc_section"},
			&cli.StringFlag{Name: "keyword", Usage: "only nodes with this keyword"},
			&cli.StringFlag{Name: "domain", Usage: "only nodes linked to this domain key"},
			&cli.BoolFlag{Name: "needs-summary", Usage: "only nodes waiting for a summary"},
		}
	}
	syncFlags := []cli.Flag{
		&cli.BoolFlag{Name: "no-ai", Usage: "structural sync only; do not call the AI"},
		&cli.BoolFlag{Name: "dry-run", Usage: "show what the AI would write without saving it"},
		&cli.StringFlag{Name: "model", Value: "haiku", Usage: "Claude model for the AI step"},
		&cli.IntFlag{Name: "parallel", Value: 3, Usage: "concurrent AI calls"},
	}

	return &cli.Command{
		Name:    "knowledge",
		Usage:   "shared knowledge graph for people and AI",
		Version: version,
		Description: "Nodes: domain, doc, doc_section (node_type property; the graph label is the title).\n" +
			"Edges: domain_of (doc|doc_section -> domain), section_of (doc_section -> doc|doc_section),\n" +
			"reference (doc|doc_section -> doc|doc_section it links to with [text](other.md#heading)).\n" +
			"Docs in ./docs are tracked by \"knowledge sync\".",
		Reader:                stdin,
		Writer:                out,
		ErrWriter:             os.Stderr,
		EnableShellCompletion: true,
		// Errors are printed once by main, not also by the framework.
		ExitErrHandler: func(context.Context, *cli.Command, error) {},
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "json", Usage: "machine-readable output"},
			&cli.StringFlag{Name: "author", Usage: "who made the change: user, ai or a name", Sources: cli.EnvVars("KNOWLEDGE_AUTHOR")},
			&cli.StringFlag{Name: "data", Usage: "data directory (default: nearest ./knowledge_data, else <exe dir>/../knowledge_data)", Sources: cli.EnvVars("KNOWLEDGE_DATA")},
		},
		Commands: []*cli.Command{
			// ---- tools --------------------------------------------------------
			{
				Name: "view", Category: "tools", ArgsUsage: " ",
				Usage: "open the Knowledge UI: the graph plus an explain query form (Ctrl+C to stop)",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "addr", Usage: "listen address", Value: "127.0.0.1:7474"},
					&cli.BoolFlag{Name: "no-open", Usage: "do not open the browser"},
					&cli.BoolFlag{Name: "no-sync", Usage: "do not sync ./docs on start"},
					&cli.StringFlag{Name: "model", Value: "haiku", Usage: "Claude model for \"Summarize with AI\""},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					dir, err := dataDir(cmd.String("data"))
					if err != nil {
						return err
					}
					return view(cmd, dir, cmd.Root().Writer)
				},
			},
			{
				Name: "explain", Category: "tools", ArgsUsage: "<query...>",
				Usage: "search knowledge and print its context (matches, domains, connected nodes)",
				Flags: []cli.Flag{
					&cli.IntFlag{Name: "limit", Value: 5, Usage: "max matching nodes"},
					&cli.IntFlag{Name: "depth", Value: 1, Usage: "edge hops to include"},
				},
				Action: withStore(1, (*app).explain),
			},
			{
				Name: "mcp", Category: "tools", ArgsUsage: " ",
				Usage: "run as an MCP server on stdio so AI chat sessions can use the knowledge graph",
				Description: "Registered in .mcp.json as \"knowledge\". Syncs ./docs (without AI) on start.\n" +
					"Tools: knowledge_explain, _search, _get, _list, _stats, _sync, _pending, _annotate, _add_domain, _assign_domain, _unassign_domain,\n" +
					"_fetch, _save_web.",
				Flags: []cli.Flag{&cli.BoolFlag{Name: "no-sync", Usage: "do not sync ./docs on start"}},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					dir, err := dataDir(cmd.String("data"))
					if err != nil {
						return err
					}
					return serveMCP(cmd, dir, cmd.Root().Reader, cmd.Root().Writer)
				},
			},
			{
				Name: "query", Category: "tools", ArgsUsage: `"<rawcypher-query>"`,
				Usage: "run a raw Cypher query",
				Description: "Examples:\n" +
					"  knowledge query \"MATCH (n {node_type: 'domain'}) RETURN n.key, n.title\"\n" +
					"  knowledge query \"MATCH (s)-[r:section_of]->(d {key: 'docs/knowledge.md'}) RETURN s, r, d\"\n" +
					"Labels are titles without spaces/punctuation, e.g. MATCH (n:InternetMarketing).\n" +
					"Properties: key, node_type, title, summary, keyword, loc, line_loc, level, needs_summary, author,\n" +
					"uri, last_fetched (web sources).\n" +
					"Raw writes (CREATE/SET/DELETE) bypass the schema; prefer sync/annotate/assign.",
				Action: withStore(1, (*app).query),
			},
			{
				Name: "sync", Category: "tools", ArgsUsage: " ",
				Usage: "track ./docs: update doc/doc_section nodes, then let the AI write summaries, keywords and domains",
				Description: "Step 1 (no AI): markdown files in ./docs become doc and doc_section nodes; changed content is\n" +
					"flagged needs_summary; removed files/headings are deleted; links to other docs become reference\n" +
					"edges; frontmatter \"domain:\", \"keyword:\" and\n" +
					"\"summary:\" are applied. Step 2 (AI, skip with --no-ai): for each doc with pending nodes, \"claude -p\"\n" +
					"writes summaries/keywords and picks or creates domains. Uses your Claude Code login (no API key).\n" +
					"Summaries written by a person or frontmatter are never overwritten.",
				Flags:  syncFlags,
				Action: func(ctx context.Context, cmd *cli.Command) error { return syncCmd(ctx, cmd, nil) },
			},
			{
				Name: "fetch", Category: "tools", ArgsUsage: "<url...>",
				Usage: "save web pages as docs in " + kb.WebDir + ", then sync them",
				Description: "Downloads each page, keeps its main content and converts it to markdown with \"uri:\" and\n" +
					"\"last_fetched:\" frontmatter. Fetching a saved uri again rewrites the same file; unchanged sections\n" +
					"keep their summaries. --refresh fetches every saved web source again. Then runs sync (see sync --help).\n" +
					"Pages that need JavaScript or a login can be saved from an AI chat with the knowledge_save_web MCP tool.",
				Flags:  append([]cli.Flag{&cli.BoolFlag{Name: "refresh", Usage: "fetch every saved web source again"}}, syncFlags...),
				Action: func(ctx context.Context, cmd *cli.Command) error { return fetchCmd(ctx, cmd) },
			},

			// ---- nodes --------------------------------------------------------
			{Name: "get", Category: "nodes", Usage: "show a node with its edges", ArgsUsage: "<key>", Action: withStore(1, (*app).get)},
			{Name: "list", Category: "nodes", Usage: "list nodes", ArgsUsage: " ", Flags: listFlags(), Action: withStore(0, (*app).list)},
			{Name: "search", Category: "nodes", Usage: "find nodes matching all words", ArgsUsage: "<words...>", Flags: listFlags(), Action: withStore(1, (*app).search)},
			{
				Name: "annotate", Category: "nodes", ArgsUsage: "<key>",
				Usage: "write a node's summary and keywords (and a domain's title)",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "summary", Usage: "short summary (clears needs_summary)"},
					&cli.StringSliceFlag{Name: "keyword", Usage: "keyword (repeatable or comma separated); replaces the list"},
					&cli.StringFlag{Name: "title", Usage: "new title (domains only)"},
				},
				Action: withStore(1, (*app).annotate),
			},
			{
				Name: "domain", Category: "nodes", ArgsUsage: "<title>",
				Usage: "create a domain, or update an existing one's summary/keywords",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "summary", Usage: "short summary"},
					&cli.StringSliceFlag{Name: "keyword", Usage: "keyword (repeatable or comma separated)"},
				},
				Action: withStore(1, (*app).domain),
			},
			{
				Name: "delete", Category: "nodes", ArgsUsage: "<domain-key>",
				Usage:  "delete a domain and its edges (docs and sections are managed by sync)",
				Flags:  []cli.Flag{&cli.BoolFlag{Name: "force", Usage: "confirm deletion"}},
				Action: withStore(1, (*app).delete),
			},

			// ---- edges --------------------------------------------------------
			{Name: "assign", Category: "edges", Usage: "link a doc or section to a domain (domain_of); creates the domain if needed", ArgsUsage: "<node-key> <domain>", Action: withStore(2, (*app).assign)},
			{Name: "unassign", Category: "edges", Usage: "remove a domain_of edge", ArgsUsage: "<node-key> <domain-key>", Action: withStore(2, (*app).unassign)},
			{
				Name: "neighbors", Category: "edges", Usage: "nodes reachable within N hops", ArgsUsage: "<key>",
				Flags:  []cli.Flag{&cli.IntFlag{Name: "depth", Value: 1, Usage: "hops"}},
				Action: withStore(1, (*app).neighbors),
			},
			{Name: "path", Category: "edges", Usage: "shortest edge chain between two nodes", ArgsUsage: "<from> <to>", Action: withStore(2, (*app).path)},

			// ---- graph --------------------------------------------------------
			{Name: "stats", Category: "graph", Usage: "counts by node/edge type; nodes needing summaries; docs without domain", ArgsUsage: " ", Action: withStore(0, (*app).stats)},
			{Name: "map", Category: "graph", Usage: "print a Mermaid diagram (domains and docs by default)", ArgsUsage: " ", Flags: listFlags(), Action: withStore(0, (*app).mermaid)},
			{Name: "export", Category: "graph", Usage: "write the whole graph as JSON (stdout by default)", ArgsUsage: "[file]", Action: withStore(0, (*app).export)},
			{Name: "import", Category: "graph", Usage: "restore nodes, summaries and edges from an export", ArgsUsage: "<file|->", Action: withStore(1, (*app).importCmd)},
			{
				Name: "schema", Category: "graph", Usage: "print node types and edge rules", ArgsUsage: " ",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					w := cmd.Root().Writer
					v := map[string]any{"node_types": kb.NodeTypes, "edges": map[string]string{
						kb.RelDomainOf: "doc|doc_section -> domain", kb.RelSectionOf: "doc_section -> doc|doc_section",
						kb.RelReference: "doc|doc_section -> doc|doc_section"}}
					return emit(w, cmd.Bool("json"), v, func() {
						fmt.Fprintf(w, "node types: %s\nedges:\n  domain_of   doc|doc_section -> domain\n  section_of  doc_section -> doc|doc_section\n  reference   doc|doc_section -> doc|doc_section (markdown links)\n", strings.Join(kb.NodeTypes, ", "))
					})
				},
			},
			{
				Name: "where", Category: "graph", Usage: "print the data directory", ArgsUsage: " ",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					dir, err := dataDir(cmd.String("data"))
					if err != nil {
						return err
					}
					w := cmd.Root().Writer
					return emit(w, cmd.Bool("json"), map[string]string{"data": dir, "docs": filepath.Join(filepath.Dir(dir), kb.DocsDir)}, func() { fmt.Fprintln(w, dir) })
				},
			},
		},
	}
}

// app is the per-invocation state handed to commands that need the store.
type app struct {
	store *kb.Store
	cmd   *cli.Command
	args  []string
	out   io.Writer
	in    io.Reader
}

// withStore checks the positional argument count, opens the store and runs fn.
func withStore(minArgs int, fn func(*app) error) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		if cmd.Args().Len() < minArgs {
			return fmt.Errorf("usage: knowledge %s %s", cmd.Name, cmd.ArgsUsage)
		}
		dir, err := dataDir(cmd.String("data"))
		if err != nil {
			return err
		}
		store, err := kb.Open(dir)
		if err != nil {
			return err
		}
		defer store.Close()
		return fn(&app{store: store, cmd: cmd, args: cmd.Args().Slice(), out: cmd.Root().Writer, in: cmd.Root().Reader})
	}
}

// dataDir resolves where the database lives.
func dataDir(flag string) (string, error) {
	if flag != "" {
		return filepath.Abs(flag)
	}
	if wd, err := os.Getwd(); err == nil {
		for d := wd; ; d = filepath.Dir(d) {
			cand := filepath.Join(d, "knowledge_data")
			if st, err := os.Stat(cand); err == nil && st.IsDir() {
				return cand, nil
			}
			if filepath.Dir(d) == d {
				break
			}
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(filepath.Dir(exe)), "knowledge_data"), nil
}

func (c *app) emit(v any, text func()) error { return emit(c.out, c.cmd.Bool("json"), v, text) }

func (c *app) author() string { return authorFrom(c.cmd, "user") }

// values returns repeated flag values, also splitting on commas.
func (c *app) values(name string) []string {
	var out []string
	for _, v := range c.cmd.StringSlice(name) {
		for _, p := range strings.Split(v, ",") {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

func (c *app) filter() kb.Filter {
	return kb.Filter{NodeType: c.cmd.String("type"), Keyword: c.cmd.String("keyword"),
		Domain: c.cmd.String("domain"), NeedsSummary: c.cmd.Bool("needs-summary")}
}

func where(n *kb.Node) string {
	switch {
	case n.LineLoc > 0:
		return fmt.Sprintf("%s:%d", n.Loc, n.LineLoc)
	case n.Loc != "":
		return n.Loc
	}
	return ""
}

func printNode(w io.Writer, n *kb.Node) {
	fmt.Fprintf(w, "%s [%s]\n%s\n", n.Key, n.NodeType, n.Title)
	if n.Summary != "" {
		fmt.Fprintf(w, "\n%s\n", n.Summary)
	}
	fmt.Fprintln(w)
	if loc := where(n); loc != "" {
		fmt.Fprintf(w, "loc:      %s\n", loc)
	}
	if len(n.Keyword) > 0 {
		fmt.Fprintf(w, "keyword:  %s\n", strings.Join(n.Keyword, ", "))
	}
	if n.NeedsSummary {
		fmt.Fprintln(w, "status:   needs summary")
	}
	keys := make([]string, 0, len(n.Props))
	for k := range n.Props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(w, "%s: %v\n", k, n.Props[k])
	}
	fmt.Fprintf(w, "author:   %s   updated: %s\n", n.Author, n.Updated)
}

func (c *app) get() error {
	n, err := c.store.Get(c.args[0])
	if err != nil {
		return err
	}
	out, in, err := c.store.Links(n.Key)
	if err != nil {
		return err
	}
	return c.emit(map[string]any{"node": n, "out": out, "in": in}, func() {
		printNode(c.out, n)
		if len(out) > 0 {
			fmt.Fprintln(c.out, "\nedges out:")
			for _, l := range out {
				fmt.Fprintf(c.out, "  -%s-> %s\n", l.Rel, l.To)
			}
		}
		if len(in) > 0 {
			fmt.Fprintln(c.out, "\nedges in:")
			for _, l := range in {
				fmt.Fprintf(c.out, "  %s -%s->\n", l.From, l.Rel)
			}
		}
	})
}

func (c *app) printList(nodes []*kb.Node) {
	for _, n := range nodes {
		flag := " "
		if n.NeedsSummary {
			flag = "*"
		}
		fmt.Fprintf(c.out, "%s %-11s %-50s %s\n", flag, n.NodeType, n.Key, n.Title)
	}
}

func (c *app) list() error {
	nodes, err := c.store.List(c.filter())
	if err != nil {
		return err
	}
	return c.emit(nodes, func() {
		c.printList(nodes)
		fmt.Fprintf(c.out, "%d nodes (* = needs summary)\n", len(nodes))
	})
}

func (c *app) search() error {
	hits, err := c.store.Search(strings.Join(c.args, " "), c.filter())
	if err != nil {
		return err
	}
	return c.emit(hits, func() {
		for _, h := range hits {
			fmt.Fprintf(c.out, "%3d  %-11s %-50s %s\n", h.Score, h.Node.NodeType, h.Node.Key, h.Node.Title)
		}
		fmt.Fprintf(c.out, "%d matches\n", len(hits))
	})
}

func (c *app) annotate() error {
	a := kb.Annotation{Author: c.author()}
	if c.cmd.IsSet("summary") {
		s := c.cmd.String("summary")
		a.Summary = &s
	}
	if c.cmd.IsSet("keyword") {
		a.Keyword = c.values("keyword")
	}
	if c.cmd.IsSet("title") {
		t := c.cmd.String("title")
		a.Title = &t
	}
	if a.Summary == nil && a.Keyword == nil && a.Title == nil {
		return errors.New("nothing to change: use --summary, --keyword or --title")
	}
	n, err := c.store.Annotate(c.args[0], a)
	if err != nil {
		return err
	}
	return c.emit(n, func() { printNode(c.out, n) })
}

func (c *app) domain() error {
	d, created, err := c.store.AddDomain(strings.Join(c.args, " "), c.cmd.String("summary"), c.values("keyword"), c.author())
	if err != nil {
		return err
	}
	action := "updated"
	if created {
		action = "created"
	}
	return c.emit(map[string]any{"action": action, "node": d}, func() {
		fmt.Fprintf(c.out, "%s domain %s (%s)\n", action, d.Key, d.Title)
	})
}

func (c *app) delete() error {
	n, err := c.store.Get(c.args[0])
	if err != nil {
		return err
	}
	if n.NodeType != kb.TypeDomain {
		return fmt.Errorf("%s nodes are managed by sync: edit or delete the file in ./docs instead", n.NodeType)
	}
	if !c.cmd.Bool("force") {
		return errors.New("delete removes the domain and all its edges; add --force to confirm")
	}
	if err := c.store.Delete(n.Key); err != nil {
		return err
	}
	return c.emit(map[string]string{"deleted": n.Key}, func() { fmt.Fprintf(c.out, "deleted %s\n", n.Key) })
}

func (c *app) assign() error {
	d, created, err := c.store.AssignDomain(c.args[0], strings.Join(c.args[1:], " "), c.author())
	if err != nil {
		return err
	}
	return c.emit(map[string]any{"node": c.args[0], "domain": d, "domain_created": created}, func() {
		note := ""
		if created {
			note = " (new domain)"
		}
		fmt.Fprintf(c.out, "%s -domain_of-> %s%s\n", c.args[0], d.Key, note)
	})
}

func (c *app) unassign() error {
	n, err := c.store.Unlink(c.args[0], kb.RelDomainOf, c.args[1])
	if err != nil {
		return err
	}
	return c.emit(map[string]int{"removed": n}, func() { fmt.Fprintf(c.out, "removed %d edge(s)\n", n) })
}

func (c *app) neighbors() error {
	hits, err := c.store.Neighbors(c.args[0], int(c.cmd.Int("depth")))
	if err != nil {
		return err
	}
	return c.emit(hits, func() {
		for _, h := range hits {
			var via []string
			for _, l := range h.Via {
				via = append(via, fmt.Sprintf("%s -%s-> %s", l.From, l.Rel, l.To))
			}
			fmt.Fprintf(c.out, "%d  %-11s %-50s %s\n     via %s\n", h.Depth, h.Node.NodeType, h.Node.Key, h.Node.Title, strings.Join(via, ", "))
		}
		fmt.Fprintf(c.out, "%d nodes\n", len(hits))
	})
}

func (c *app) path() error {
	nodes, links, err := c.store.Path(c.args[0], c.args[1])
	if err != nil {
		return err
	}
	return c.emit(map[string]any{"nodes": nodes, "links": links}, func() {
		for _, l := range links {
			fmt.Fprintf(c.out, "%s -%s-> %s\n", l.From, l.Rel, l.To)
		}
		if len(links) == 0 && len(nodes) > 0 {
			fmt.Fprintln(c.out, nodes[0].Key)
		}
	})
}

func (c *app) stats() error {
	st, err := c.store.Stats()
	if err != nil {
		return err
	}
	return c.emit(st, func() {
		fmt.Fprintf(c.out, "data:   %s\nnodes:  %d\nedges:  %d\n", st.Dir, st.Nodes, st.Edges)
		printCounts(c.out, "node types", st.NodeTypes)
		printCounts(c.out, "edge types", st.EdgeTypes)
		printCounts(c.out, "keywords", st.Keywords)
		if len(st.NeedsSummary) > 0 {
			fmt.Fprintf(c.out, "\nneeds summary (%d): run \"knowledge sync\"\n", len(st.NeedsSummary))
			for _, k := range st.NeedsSummary {
				fmt.Fprintf(c.out, "  %s\n", k)
			}
		}
		if len(st.NoDomain) > 0 {
			fmt.Fprintf(c.out, "\ndocs without domain: %s\n", strings.Join(st.NoDomain, ", "))
		}
	})
}

func printCounts(w io.Writer, title string, m map[string]int) {
	if len(m) == 0 {
		return
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if m[keys[i]] != m[keys[j]] {
			return m[keys[i]] > m[keys[j]]
		}
		return keys[i] < keys[j]
	})
	fmt.Fprintf(w, "\n%s:\n", title)
	for _, k := range keys {
		fmt.Fprintf(w, "  %-24s %d\n", k, m[k])
	}
}

func (c *app) mermaid() error {
	f := c.filter()
	nodes, err := c.store.List(f)
	if err != nil {
		return err
	}
	links, err := c.store.AllLinks()
	if err != nil {
		return err
	}
	in := map[string]bool{}
	var b strings.Builder
	b.WriteString("graph LR\n")
	ids := map[string]string{}
	for i, n := range nodes {
		if f.NodeType == "" && n.NodeType == kb.TypeSection {
			continue // default map: domains and docs
		}
		in[n.Key] = true
		ids[n.Key] = fmt.Sprintf("n%d", i)
		label := strings.ReplaceAll(n.Title, `"`, "'")
		shape := `["%s<br/><i>%s</i>"]`
		if n.NodeType == kb.TypeDomain {
			shape = `{{"%s<br/><i>%s</i>"}}`
		}
		fmt.Fprintf(&b, "  %s"+shape+"\n", ids[n.Key], label, n.NodeType)
	}
	for _, l := range links {
		if in[l.From] && in[l.To] {
			fmt.Fprintf(&b, "  %s -->|%s| %s\n", ids[l.From], l.Rel, ids[l.To])
		}
	}
	return c.emit(map[string]string{"mermaid": b.String()}, func() { fmt.Fprint(c.out, b.String()) })
}

func (c *app) export() error {
	snap, err := c.store.Export()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	if len(c.args) == 0 || c.args[0] == "-" {
		_, err = fmt.Fprintln(c.out, string(data))
		return err
	}
	if err := os.WriteFile(c.args[0], append(data, '\n'), 0o644); err != nil {
		return err
	}
	return c.emit(map[string]any{"file": c.args[0], "nodes": len(snap.Nodes), "links": len(snap.Links)}, func() {
		fmt.Fprintf(c.out, "exported %d nodes, %d edges to %s\n", len(snap.Nodes), len(snap.Links), c.args[0])
	})
}

func (c *app) importCmd() error {
	var data []byte
	var err error
	if c.args[0] == "-" {
		data, err = io.ReadAll(c.in)
	} else {
		data, err = os.ReadFile(c.args[0])
	}
	if err != nil {
		return err
	}
	var snap kb.Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	r := c.store.Import(&snap)
	if err := c.emit(r, func() {
		fmt.Fprintf(c.out, "nodes: %d created, %d updated\nedges: %d created, %d already present\n", r.Created, r.Updated, r.LinksCreated, r.LinksSkipped)
		for _, e := range r.Errors {
			fmt.Fprintln(c.out, "  error:", e)
		}
	}); err != nil {
		return err
	}
	if len(r.Errors) > 0 {
		return fmt.Errorf("%d item(s) failed to import", len(r.Errors))
	}
	return nil
}

func (c *app) query() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := c.store.Query(ctx, strings.Join(c.args, " "))
	if err != nil {
		return err
	}
	rows := res.Rows
	if rows == nil {
		rows = []map[string]any{}
	}
	keys := map[graphdb.NodeID]string{}
	return c.emit(map[string]any{"columns": res.Columns, "rows": rows}, func() {
		fmt.Fprintln(c.out, strings.Join(res.Columns, "\t"))
		for _, r := range rows {
			cells := make([]string, len(res.Columns))
			for i, col := range res.Columns {
				cells[i] = c.formatCell(r[col], keys)
			}
			fmt.Fprintln(c.out, strings.Join(cells, "\t"))
		}
		fmt.Fprintf(c.out, "%d rows\n", len(rows))
	})
}

// formatCell renders Cypher values for text output: nodes as
// `key [node_type] "title"`, edges as `from -rel-> to`, everything else as-is.
func (c *app) formatCell(v any, keys map[graphdb.NodeID]string) string {
	keyOf := func(id graphdb.NodeID) string {
		if k, ok := keys[id]; ok {
			return k
		}
		k := fmt.Sprintf("#%d", id)
		if n, err := c.store.DB().GetNode(id); err == nil && n != nil && n.GetString("key") != "" {
			k = n.GetString("key")
		}
		keys[id] = k
		return k
	}
	switch x := v.(type) {
	case nil:
		return "null"
	case *graphdb.Node:
		if x == nil {
			return "null"
		}
		return fmt.Sprintf("%s [%s] %q", keyOf(x.ID), x.GetString("node_type"), x.GetString("title"))
	case *graphdb.Edge:
		if x == nil {
			return "null"
		}
		return fmt.Sprintf("%s -%s-> %s", keyOf(x.From), x.Label, keyOf(x.To))
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = c.formatCell(e, keys)
		}
		return strings.Join(parts, ", ")
	}
	return fmt.Sprint(v)
}

// emit prints JSON when asJSON is set, otherwise runs the text printer.
func emit(w io.Writer, asJSON bool, v any, text func()) error {
	if asJSON {
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, string(data))
		return err
	}
	text()
	return nil
}
