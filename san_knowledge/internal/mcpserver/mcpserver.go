// Package mcpserver exposes the knowledge graph to AI chat sessions over the
// Model Context Protocol (JSON-RPC 2.0, newline-delimited on stdio).
//
// Docs and sections are managed by sync from ./docs, so there are no tools to
// create or delete them: a session reads the graph, writes summaries and
// keywords, and manages domains.
package mcpserver

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"san_knowledge/internal/kb"
)

// Opener runs fn with an open store; the server opens per tool call so the
// database is not locked while the chat session is idle.
type Opener func(fn func(*kb.Store) error) error

type Server struct {
	open    Opener
	author  string
	version string
	out     io.Writer
	mu      sync.Mutex
}

func New(open Opener, author, version string) *Server {
	if author == "" {
		author = kb.AuthorAI
	}
	return &Server{open: open, author: author, version: version}
}

var supportedVersions = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

const instructions = `Knowledge graph of this project, shared between the user and AI.
Nodes: domain (broad area such as "Internet Marketing"), doc (a markdown file in ./docs, key = its path) and doc_section (a heading, key = path#heading-path). Edges: domain_of (doc/section -> domain), section_of (section -> parent doc/section).
Before answering questions about the project's research or docs, call knowledge_explain to load context and cite the loc (file:line).
Docs and sections come from the files: after editing files in ./docs call knowledge_sync. Nodes flagged needs_summary have no or an outdated summary: use knowledge_pending, read the text, then knowledge_annotate (1-3 sentence summary, 3-8 lowercase keywords). Give docs a domain with knowledge_assign_domain, reusing existing domains when they fit.`

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Serve reads requests until in is closed.
func (s *Server) Serve(in io.Reader, out io.Writer) error {
	s.out = out
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 1024*1024), 32*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var req request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.send(nil, nil, &rpcError{Code: -32700, Message: "parse error"})
			continue
		}
		result, rerr := s.dispatch(req)
		if len(req.ID) == 0 || string(req.ID) == "null" {
			continue // notification
		}
		s.send(req.ID, result, rerr)
	}
	return sc.Err()
}

func (s *Server) send(id json.RawMessage, result any, rerr *rpcError) {
	msg := map[string]any{"jsonrpc": "2.0", "id": id}
	if id == nil {
		msg["id"] = nil
	}
	if rerr != nil {
		msg["error"] = rerr
	} else {
		msg["result"] = result
	}
	data, _ := json.Marshal(msg)
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Fprintf(s.out, "%s\n", data)
}

func (s *Server) dispatch(req request) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		json.Unmarshal(req.Params, &p)
		version := supportedVersions[0]
		for _, v := range supportedVersions {
			if v == p.ProtocolVersion {
				version = v
			}
		}
		return map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "knowledge", "title": "Knowledge Graph", "version": s.version},
			"instructions":    instructions,
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": toolDefs()}, nil
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, &rpcError{Code: -32602, Message: "invalid params"}
		}
		text, err := s.call(p.Name, p.Arguments)
		if err != nil {
			return map[string]any{"content": []any{textContent("Error: " + err.Error())}, "isError": true}, nil
		}
		return map[string]any{"content": []any{textContent(text)}}, nil
	default:
		if strings.HasPrefix(req.Method, "notifications/") {
			return nil, nil
		}
		return nil, &rpcError{Code: -32601, Message: "method not found: " + req.Method}
	}
}

func textContent(t string) map[string]any { return map[string]any{"type": "text", "text": t} }

// ---- tool definitions ----------------------------------------------------------

func schema(props map[string]any, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func str(desc string) map[string]any  { return map[string]any{"type": "string", "description": desc} }
func num(desc string) map[string]any  { return map[string]any{"type": "integer", "description": desc} }
func flag(desc string) map[string]any { return map[string]any{"type": "boolean", "description": desc} }
func strs(desc string) map[string]any {
	return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": desc}
}

func tool(name, title, desc string, input map[string]any, readOnly, destructive bool) map[string]any {
	return map[string]any{
		"name": name, "title": title, "description": desc, "inputSchema": input,
		"annotations": map[string]any{"readOnlyHint": readOnly, "destructiveHint": destructive, "openWorldHint": false},
	}
}

var nodeTypeProp = map[string]any{"type": "string", "enum": kb.NodeTypes, "description": "Node type"}

func toolDefs() []any {
	return []any{
		tool("knowledge_explain", "Explain (search knowledge context)",
			"Search the graph with a free-text question. Returns the best matching domains/docs/sections with loc (file:line), domains, summary (or an excerpt when there is none yet) and connected nodes, as markdown. Use this first.",
			schema(map[string]any{
				"query": str("Question or keywords"),
				"limit": num("Max matching nodes (default 5)"),
				"depth": num("Edge hops to include (default 1)"),
			}, "query"), true, false),
		tool("knowledge_search", "Search nodes",
			"Find nodes whose title, keywords, summary, key or file text contain all the given words. Compact list.",
			schema(map[string]any{"query": str("Words that must all match"), "node_type": nodeTypeProp, "limit": num("Max results (default 20)")}, "query"),
			true, false),
		tool("knowledge_get", "Get node",
			"Get one node with all fields and its incoming and outgoing edges.",
			schema(map[string]any{"key": str("Node key, e.g. docs/knowledge.md or docs/knowledge.md#knowledge/domain-node")}, "key"),
			true, false),
		tool("knowledge_list", "List nodes",
			"List nodes, optionally filtered.",
			schema(map[string]any{
				"node_type":     nodeTypeProp,
				"keyword":       str("Only nodes with this keyword"),
				"domain":        str("Only nodes linked to this domain key"),
				"needs_summary": flag("Only nodes waiting for a summary"),
				"limit":         num("Max results (default 100)"),
			}), true, false),
		tool("knowledge_stats", "Graph statistics",
			"Counts of nodes and edges by type, keywords, nodes needing summaries and docs without a domain.",
			schema(map[string]any{}), true, false),
		tool("knowledge_sync", "Sync docs",
			"Update doc and doc_section nodes from the markdown files in ./docs (no AI). Call after creating, editing or deleting docs.",
			schema(map[string]any{}), false, false),
		tool("knowledge_pending", "Nodes needing summaries",
			"Nodes flagged needs_summary (new or changed content), with their current text, so you can write summaries with knowledge_annotate. Human-written summaries are listed but should be left to the user.",
			schema(map[string]any{"limit": num("Max nodes (default 20)")}), true, false),
		tool("knowledge_annotate", "Write summary and keywords",
			"Set a node's summary (1-3 sentences; clears needs_summary) and/or keywords (replaces the list).",
			schema(map[string]any{
				"key":     str("Node key"),
				"summary": str("Short summary"),
				"keyword": strs("3-8 lowercase keywords, most important first"),
			}, "key"), false, false),
		tool("knowledge_add_domain", "Add domain",
			"Create a domain (broad knowledge area), or update an existing domain's summary/keywords. Check existing domains with knowledge_list first.",
			schema(map[string]any{"title": str("Domain title, e.g. Internet Marketing"), "summary": str("Short summary"), "keyword": strs("Keywords")}, "title"),
			false, false),
		tool("knowledge_assign_domain", "Assign domain",
			"Link a doc or section to a domain (domain_of). The domain may be a key or title; it is created if it does not exist.",
			schema(map[string]any{"key": str("Doc or section key"), "domain": str("Domain key or title")}, "key", "domain"),
			false, false),
		tool("knowledge_unassign_domain", "Unassign domain",
			"Remove the domain_of edge between a doc/section and a domain.",
			schema(map[string]any{"key": str("Doc or section key"), "domain": str("Domain key")}, "key", "domain"),
			false, true),
	}
}

// ---- tool calls ------------------------------------------------------------------

func (s *Server) call(name string, raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	var text string
	err := s.open(func(st *kb.Store) error {
		var err error
		text, err = s.run(st, name, raw)
		return err
	})
	return text, err
}

func (s *Server) run(st *kb.Store, name string, raw json.RawMessage) (string, error) {
	var a struct {
		Query        string   `json:"query"`
		Limit        int      `json:"limit"`
		Depth        int      `json:"depth"`
		Key          string   `json:"key"`
		NodeType     string   `json:"node_type"`
		Keyword      any      `json:"keyword"`
		Domain       string   `json:"domain"`
		NeedsSummary bool     `json:"needs_summary"`
		Summary      *string  `json:"summary"`
		Title        string   `json:"title"`
	}
	if err := json.Unmarshal(raw, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	keywords := func() []string {
		switch k := a.Keyword.(type) {
		case string:
			return []string{k}
		case []any:
			out := make([]string, 0, len(k))
			for _, x := range k {
				out = append(out, fmt.Sprint(x))
			}
			return out
		}
		return nil
	}

	switch name {
	case "knowledge_explain":
		if strings.TrimSpace(a.Query) == "" {
			return "", fmt.Errorf("query is required")
		}
		ex, err := st.Explain(a.Query, a.Limit, a.Depth)
		if err != nil {
			return "", err
		}
		return ex.Markdown(), nil

	case "knowledge_search":
		hits, err := st.Search(a.Query, kb.Filter{NodeType: a.NodeType})
		if err != nil {
			return "", err
		}
		nodes := make([]*kb.Node, 0, len(hits))
		for _, h := range hits {
			nodes = append(nodes, h.Node)
		}
		return compactList(nodes, a.Limit, 20), nil

	case "knowledge_get":
		n, err := st.Get(a.Key)
		if err != nil {
			return "", err
		}
		out, in, err := st.Links(n.Key)
		if err != nil {
			return "", err
		}
		return toJSON(map[string]any{"node": n, "out": out, "in": in}), nil

	case "knowledge_list":
		kw := ""
		if k := keywords(); len(k) > 0 {
			kw = k[0]
		}
		nodes, err := st.List(kb.Filter{NodeType: a.NodeType, Keyword: kw, Domain: a.Domain, NeedsSummary: a.NeedsSummary})
		if err != nil {
			return "", err
		}
		return compactList(nodes, a.Limit, 100), nil

	case "knowledge_stats":
		stats, err := st.Stats()
		if err != nil {
			return "", err
		}
		return toJSON(stats), nil

	case "knowledge_sync":
		rep, err := st.Sync()
		if err != nil {
			return "", err
		}
		return toJSON(rep), nil

	case "knowledge_pending":
		nodes, err := st.List(kb.Filter{NeedsSummary: true})
		if err != nil {
			return "", err
		}
		limit := a.Limit
		if limit <= 0 {
			limit = 20
		}
		docs := map[string]*kb.ParsedDoc{}
		var items []map[string]any
		for _, n := range nodes {
			if len(items) == limit {
				break
			}
			item := map[string]any{"key": n.Key, "node_type": n.NodeType, "title": n.Title,
				"loc": n.Loc, "line_loc": n.LineLoc, "current_summary": n.Summary,
				"human_written": kb.IsHumanAuthor(n.Author)}
			if n.Loc != "" {
				d, ok := docs[n.Loc]
				if !ok {
					d, _ = st.ParseDoc(n.Loc)
					docs[n.Loc] = d
				}
				if d != nil {
					text := d.SectionText(n.Key)
					if len(text) > 3000 {
						text = text[:3000] + "…"
					}
					item["text"] = text
				}
			}
			items = append(items, item)
		}
		return toJSON(map[string]any{"total": len(nodes), "nodes": nonNil(items)}), nil

	case "knowledge_annotate":
		if a.Summary == nil && a.Keyword == nil {
			return "", fmt.Errorf("give a summary and/or keyword")
		}
		n, err := st.Annotate(a.Key, kb.Annotation{Summary: a.Summary, Keyword: keywords(), Author: s.author})
		if err != nil {
			return "", err
		}
		return toJSON(n), nil

	case "knowledge_add_domain":
		summary := ""
		if a.Summary != nil {
			summary = *a.Summary
		}
		d, created, err := st.AddDomain(a.Title, summary, keywords(), s.author)
		if err != nil {
			return "", err
		}
		return toJSON(map[string]any{"created": created, "domain": d}), nil

	case "knowledge_assign_domain":
		d, created, err := st.AssignDomain(a.Key, a.Domain, s.author)
		if err != nil {
			return "", err
		}
		return toJSON(map[string]any{"key": a.Key, "domain": d.Key, "domain_created": created}), nil

	case "knowledge_unassign_domain":
		n, err := st.Unlink(a.Key, kb.RelDomainOf, a.Domain)
		if err != nil {
			return "", err
		}
		return toJSON(map[string]int{"removed": n}), nil
	}
	return "", fmt.Errorf("unknown tool %q", name)
}

func compactList(nodes []*kb.Node, limit, def int) string {
	if limit <= 0 {
		limit = def
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d node(s)\n", len(nodes))
	for i, n := range nodes {
		if i == limit {
			fmt.Fprintf(&b, "… %d more\n", len(nodes)-limit)
			break
		}
		line := fmt.Sprintf("- %s [%s] %s", n.Key, n.NodeType, n.Title)
		if n.NeedsSummary {
			line += " (needs summary)"
		}
		if n.Summary != "" {
			line += " — " + n.Summary
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func toJSON(v any) string {
	data, _ := json.MarshalIndent(v, "", "  ")
	return string(data)
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
