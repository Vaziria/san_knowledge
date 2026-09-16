# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

Marketing research scoped to the **Indonesian market**, plus `knowledge`, a Go tool that turns the project's documents into a knowledge graph so the user and AI can collaborate. Research write-ups are markdown; files under `docs/` are tracked in the graph (`knowledge_data/`).

`docs/knowledge.md` is the user's spec for the tool. **Do not edit it.** It says "AI Supposed Not Edit This File". Implement what it says instead; when it changes, re-read it and bring the code in line.

## Commands

All Go work happens in `san_knowledge/` (module name `san_knowledge`, Go 1.25, Windows/PowerShell environment).

```powershell
pwsh san_knowledge/build.ps1                  # go vet + go test + build bin/knowledge.exe (stamps main.version)
cd san_knowledge; go test ./...               # all tests
go test ./internal/kb -run TestSyncLifecycle -v   # single test
go test ./cmd/knowledge -run TestMCP -v
```

Rebuild `bin/knowledge.exe` after any change. The MCP server and the UI run from that binary, and Windows can't overwrite it while a `view` or `mcp` process is still running.

Using the tool (run from the repo root; data dir is auto-resolved):

```powershell
bin\knowledge.exe --help                      # command tree (urfave/cli v3)
bin\knowledge.exe sync                        # track ./docs, then AI writes summaries/keywords/domains (claude -p)
bin\knowledge.exe sync --no-ai | --dry-run    # structure only | preview AI output without saving
bin\knowledge.exe explain "which commands does the tool have"
bin\knowledge.exe query "MATCH (s)-[r:section_of]->(d {key: 'docs/knowledge.md'}) RETURN s, r, d"
bin\knowledge.exe view                        # web UI on 127.0.0.1:7474
bin\knowledge.exe mcp                         # MCP stdio server (registered in .mcp.json as "knowledge")
```

## Architecture

```
cmd/knowledge      urfave/cli v3 command tree (main.go) + sync/view/mcp entry points (tools.go)
internal/kb        domain layer on goraphdb: schema.go, store.go, markdown.go (parser), sync.go, ai.go, explain.go
internal/ownview   Knowledge UI: JSON API + embedded static/ (vanilla JS + vendored cytoscape)
internal/mcpserver hand-written MCP (JSON-RPC 2.0 over newline-delimited stdio), no SDK
```

**Everything goes through `internal/kb`.** The CLI, the UI API and the MCP tools are thin adapters over `kb.Store`. Schema rules live there, not in the adapters. The goraphdb `*DB` is only used directly in `kb` and for raw `query`.

**Schema (spec "# Knowledge").**
- `node_type` property: `domain`, `doc` or `doc_section`.
- The graph **label is the title**, made Cypher-safe by `kb.Label` ("Internet Marketing" → `InternetMarketing`), because goraphdb's parser has no backtick quoting. The original title is in the `title` property.
- Edges: `domain_of` (doc|doc_section → domain) and `section_of` (doc_section → parent doc or parent section). `validRelation` enforces the endpoint types.
- Keys, found via a property index on `key` (not a unique constraint):
  - domain: slug of its title
  - doc: its project-relative path (`docs/x.md`)
  - section: path + `#` + slug heading path (`docs/x.md#parent/child`; duplicate sibling headings get `-2`)
- Other properties: `summary`, `keyword` (list, normalized: lowercase, `-`/`_` → space), `loc`, `line_loc`, `level`, `hash`, `needs_summary`, `author`.

**Sync (`kb.Sync`, no AI).**
- Scans `docs/**/*.md`. The first `# ` heading is the doc title (not a section); every other heading becomes a section.
- Change detection uses per-section text fingerprints. Changed text sets `needs_summary` and keeps the old summary.
- Removed files and headings are deleted. A renamed heading with unchanged text carries over its summary, keywords and domains.
- Empty docs, and headings with no text of their own, never need a summary.
- Frontmatter `domain:`, `keyword:` and `summary:` are applied with author `frontmatter`; frontmatter domain edges are removed when no longer declared.
- `view` and `mcp` run a structural sync on start (skip with `--no-sync`).

**AI step (`kb.RunAI`, `ai.go`).**
- One `claude -p` call per doc that has pending nodes or no domain. It uses the user's Claude Code login, no API key.
- Flags: `--json-schema` for structured output, `--tools ""`, `--strict-mcp-config` (so it can't call knowledge mcp back), `--no-session-persistence`. The working dir is the temp dir, so project CLAUDE.md and hooks stay out. Default model `haiku`.
- The DB is opened only to build requests and to apply results, never during the call, so view/mcp/CLI keep working.
- `IsHumanAuthor`: summaries written by anyone other than `sync`/`ai` (e.g. `user`, `frontmatter`) are never overwritten.
- A failed doc stays pending and is retried on the next sync.
- `kb.Annotator` is the seam for tests: `fakeAI` / `fakeAnnotator`, and `newAnnotator` in `cmd/knowledge`.

**Database locking.** bbolt allows one process at a time. `kb.Open` probes the file lock with a 5s timeout (goraphdb would otherwise block forever) and returns `kb.ErrLocked`. Short CLI commands open/close per invocation. The long-running `view` and `mcp`, and the AI step, must use `kb.With` per operation, never hold a store open; `kb.With` also serialises opens within one process. Tests assert the CLI works while the view runs.

**goraphdb quirks.**
- `UpdateNode` leaves stale entries in *unique-constraint* indexes, which is why keys use a property index.
- A title change must also swap the label; `updateNode` does both.
- `ShortestPath` follows outgoing edges only, so `Path` also tries the reverse direction.
- On Windows bbolt grows the file to `MmapSize`, so `Open` sets 16 MB. `NoSync` is forced off.

**Search vs explain.**
- `rank` scores title, keywords, summary, key, and the **live file text** of docs/sections (read through `ParseDoc`, not stored), so unsummarized sections are still findable.
- `Search` requires every term; `Explain` strips English/Indonesian stopwords and accepts any term.
- `Explain` adds domains, neighbours, and an excerpt when there is no summary.
- `Explanation.Markdown()` is the shared output for the CLI, the UI "Copy as markdown" and `knowledge_explain`.

**CLI specifics (`cmd/knowledge/main.go`).**
- `run(argv, stdin, out)` is the test entry point.
- Global flags `--json`, `--author` (`$KNOWLEDGE_AUTHOR`) and `--data` (`$KNOWLEDGE_DATA`) are persistent.
- Two urfave/cli behaviours are patched in `run`: `disableSliceSeparator` (set on every subcommand; keywords split commas via `values()`) and `stdinArgLast` (urfave stops parsing flags at a positional `-`). Add new bool flags to `boolFlagNames`.
- Docs and sections are managed only by sync: `delete` and the UI refuse them, and only a domain's title is editable.
- Data dir: `--data` → nearest `knowledge_data/` walking up from cwd → `<exe dir>/../knowledge_data`. Docs root = parent of the data dir.

**UI (`internal/ownview/static`).**
- No npm/build step: files are embedded with `go:embed`, so rebuild the exe to see changes.
- Node keys contain `/` and `#`, so the API takes them as `?key=` query params.
- All content is rendered via `textContent`/`h()`, never `innerHTML`. `panel()` filters null children.
- Node types use fixed palette slots 1–3 plus a shape; a dashed border means needs summary.
- URL params: `?q=` runs explain, `?theme=light|dark`, `#k=<key>` selects a node.
- To check visuals, run `view --no-open --addr 127.0.0.1:<port>` and screenshot with headless Edge (`--screenshot`; minimum window width ~500px).

**MCP (`internal/mcpserver`).**
- Stdout must carry only JSON-RPC; logs go to stderr. Writes default to author `ai`.
- Tools: explain, search, get, list, stats, sync, pending, annotate, add/assign/unassign domain. There is deliberately no delete tool.
- `.mcp.json` points at `bin/knowledge.exe mcp` (relative path, which works when Claude Code starts in the repo root).
- End-to-end check: `claude -p "..." --mcp-config .mcp.json --strict-mcp-config --allowedTools mcp__knowledge__knowledge_explain`.

## Research workflow

- Before answering questions about the project's research or docs, pull context (`knowledge_explain` or `bin\knowledge.exe explain`) and cite `loc` (file:line).
- Put research write-ups in `docs/` so they are tracked, then sync (`knowledge_sync`, or `bin\knowledge.exe sync` for AI summaries). In a chat session you can also summarize pending nodes yourself: `knowledge_pending`, then `knowledge_annotate`.
- Figures quoted from BPS/APJII via news coverage should be flagged as such.
- The marketplace-crawl MCP server (Tokopedia etc.) is available for product/price research.

## Git

`bin/*.exe` and `knowledge_data/*.db` are git-ignored: the database is a binary bbolt file rewritten on every change. Structure can be rebuilt from `docs/` with sync. Summaries and domains are only in the database, so version them via `bin\knowledge.exe export <file>.json` and restore with `import`.
