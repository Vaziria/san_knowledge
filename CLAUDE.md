# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

Marketing research scoped to the **Indonesian market**, plus `knowledge`, a Go tool that turns the project's documents into a knowledge graph so the user and AI can collaborate. Research write-ups are markdown; files under `docs/`, and the folders listed in `knowledge_data/config.json` (the figure specs in `typescripts/animation`, and its state files in `typescripts/animation/docs/states/`), are tracked in the graph (`knowledge_data/`).

`docs/knowledge.md` is the user's spec for the tool. **Do not edit it.** It says "AI Supposed Not Edit This File". Implement what it says instead; when it changes, re-read it and bring the code in line.

## Commands

All Go work happens in `san_knowledge/` (module name `san_knowledge`, Go 1.25). Both build scripts produce `bin/knowledge.exe` (Windows) and `bin/knowledge` (Linux); use the one for the current OS.

```powershell
pwsh san_knowledge/build.ps1                  # go vet + go test + build both binaries (stamps main.version)
bash san_knowledge/build.sh                   # same, from Linux
cd san_knowledge; go test ./...               # all tests
go test ./internal/kb -run TestSyncLifecycle -v   # single test
go test ./cmd/knowledge -run TestMCP -v
```

Rebuild after any change. The MCP server and the UI run from the binary (`.mcp.json` points at `bin/knowledge`, the Linux one), and Windows can't overwrite it while a `view` or `mcp` process is still running. Windows does allow renaming a running exe, so move it to `bin/knowledge.exe~` (git-ignored) and build. Running servers keep the old code until they restart (`/mcp` reconnect in each open session). Until then, an old server's sync uses the old rules.

Using the tool (run from the repo root; data dir is auto-resolved; on Linux use `bin/knowledge`):

```powershell
bin\knowledge.exe --help                      # command tree (urfave/cli v3)
bin\knowledge.exe sync                        # track ./docs + config folders, then AI writes summaries/keywords/domains (claude -p)
bin\knowledge.exe sync --no-ai | --dry-run    # structure only | preview AI output without saving
bin\knowledge.exe sync --redo-ai              # rewrite all AI-written summaries (e.g. after a model change)
bin\knowledge.exe fetch https://example.com/page   # save a web page into docs/external_sources/web, then sync
bin\knowledge.exe fetch --refresh --no-ai    # fetch every saved web source again
bin\knowledge.exe explain "which commands does the tool have"
bin\knowledge.exe query "MATCH (s)-[r:section_of]->(d {key: 'docs/knowledge.md'}) RETURN s, r, d"
bin\knowledge.exe view                        # web UI on 127.0.0.1:7474
bin\knowledge.exe mcp                         # MCP stdio server (registered in .mcp.json as "knowledge")
```

## Architecture

```
cmd/knowledge      urfave/cli v3 command tree (main.go) + sync/view/mcp entry points (tools.go)
internal/kb        domain layer on goraphdb: schema.go, store.go, markdown.go (parser), sync.go, ai.go, explain.go, web.go
internal/ownview   Knowledge UI: JSON API + embedded static/ (vanilla JS + vendored cytoscape)
internal/mcpserver hand-written MCP (JSON-RPC 2.0 over newline-delimited stdio), no SDK
```

**Everything goes through `internal/kb`.** The CLI, the UI API and the MCP tools are thin adapters over `kb.Store`. Schema rules live there, not in the adapters. The goraphdb `*DB` is only used directly in `kb` and for raw `query`.

**Schema (spec "# Knowledge").**
- `node_type` property: `domain`, `doc` or `doc_section`.
- The graph **label is the title**, made Cypher-safe by `kb.Label` ("Internet Marketing" → `InternetMarketing`), because goraphdb's parser has no backtick quoting. The original title is in the `title` property.
- Edges, with endpoint types enforced by `validRelation`:
  - `domain_of`: doc|doc_section → domain
  - `section_of`: doc_section → parent doc or parent section
  - `reference`: doc|doc_section → doc|doc_section, from markdown links `[text](other.md#heading)`
- Keys, found via a property index on `key` (not a unique constraint):
  - domain: slug of its title
  - doc: its project-relative path (`docs/x.md`)
  - section: path + `#` + slug heading path (`docs/x.md#parent/child`; duplicate sibling headings get `-2`)
- Other properties: `summary`, `keyword` (list, normalized: lowercase, `-`/`_` → space), `loc`, `line_loc`, `level`, `hash`, `needs_summary`, `author`.
- Web sources (spec "Website Source Knowledge") are ordinary `doc` nodes with `uri` and `last_fetched` on the doc node only (not its sections).

**Sync (`kb.Sync`, no AI).**
- Scans `docs/**/*.md`, plus `**/*.md` in each folder listed under `track` in `knowledge_data/config.json` (`{"track": ["typescripts/animation"]}`; project-relative, checked in). `node_modules`, `vendor` and hidden folders are skipped. `docs/` is always tracked, since web sources are saved there. A broken config or a folder outside the project fails the sync rather than deleting docs; dropping a folder from the list removes its docs on the next sync. This goes beyond the spec's "track every doc in `./docs`" at the user's request (2026-09-24).
- The first `# ` heading is the doc title (not a section); every other heading becomes a section.
- Change detection uses per-section text fingerprints. Changed text sets `needs_summary` and keeps the old summary.
- Removed files and headings are deleted. A renamed heading with unchanged text carries over its summary, keywords and domains.
- Empty docs, and headings with no text of their own, never need a summary.
- References are built in a final pass, after all docs exist:
  - The source is the section containing the link, or the doc for links above the first heading.
  - `ResolveLink` tries the path relative to the linking file, then relative to the project root; a `#fragment` is matched to a heading slug, falling back to the doc.
  - Web links, images, inline code, fenced code and non-tracked files are ignored.
  - Stale `reference` edges are removed only if their author is `sync`.
- The doc `hash` covers the text after frontmatter, so a refetch that only bumps `last_fetched` does not ask for a summary. Frontmatter-only edits are still applied.
- Frontmatter `domain:`, `keyword:` and `summary:` are applied with author `frontmatter`; frontmatter domain edges are removed when no longer declared.
- `view` and `mcp` run a structural sync on start (skip with `--no-sync`).

**AI step (`kb.RunAI`, `ai.go`).**
- One `claude -p` call per doc that has pending nodes or no domain. It uses the user's Claude Code login, no API key.
- Flags: `--json-schema` for structured output, `--tools ""`, `--strict-mcp-config` (so it can't call knowledge mcp back), `--no-session-persistence`. The working dir is the temp dir, so project CLAUDE.md and hooks stay out. Default model `sonnet` (`--model` to change). `sync --redo-ai` (`kb.RedoAISummaries`) flags every AI-written summary as pending so it is rewritten, e.g. after changing the model; human and frontmatter summaries are untouched.
- The DB is opened only to build requests and to apply results, never during the call, so view/mcp/CLI keep working.
- `IsHumanAuthor`: summaries written by anyone other than `sync`/`ai` (e.g. `user`, `frontmatter`) are never overwritten.
- A failed doc stays pending and is retried on the next sync.
- `kb.Annotator` is the seam for tests: `fakeAI` / `fakeAnnotator`, and `newAnnotator` in `cmd/knowledge`.

**Web sources (`kb/web.go`).**
- A page is stored as markdown only, in `docs/external_sources/web/<host>/<path-slug>.md`, with `uri:` and `last_fetched:` frontmatter; sync puts them on the doc node. No raw HTML is kept.
- Identity is the `uri` (normalized, `#fragment` dropped): saving a known uri rewrites its file and keeps other frontmatter (e.g. a hand-added `domain:`). A different uri mapping to an existing file name gets `-2`.
- Two ways in, both ending in `SaveWeb` + `Sync`:
  - the tool fetches: CLI `fetch`, MCP `knowledge_fetch`, UI "Fetch URL" (`POST /api/fetch`). `WebFetcher` does the GET, go-readability keeps the main content, html-to-markdown (with table plugin) converts it with absolute links; markdown/plain text responses are kept as-is.
  - the agent supplies content: MCP `knowledge_save_web(uri, title, markdown)` for pages that need JavaScript or a login.
- `unwrapHeadings` runs before readability: heading wrappers with short extra text (Wikipedia's `<div class="mw-heading">…[edit]`) would otherwise be dropped, losing the sections.
- References come only from normal file links; absolute web links between saved pages are not matched to docs.
- The download happens before the database is opened. `kb.Fetcher` is the test seam (`webFetcher` in `cmd/knowledge`, `mcpserver.Server.Fetcher`, `ownview.Options.Fetcher`).

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
- Communities: `/api/graph` also returns `communities` (key → number, 0 = largest) from `kb.Leiden` (`kb/community.go`, deterministic, modularity, `?resolution=`). By default nodes are coloured by community (slots 1–8, rest `--other`; shape still shows type) and the cose layout keeps intra-community edges short. "Community | Type" toggles colouring (`?color=`, localStorage); "Re-layout" reruns the layout. Saved positions use the `knowledge-graph-positions-v2` localStorage key.
- URL params: `?q=` runs explain, `?theme=light|dark`, `#k=<key>` selects a node.
- A web source's panel shows its `uri` as a link (http/https only) and `last_fetched`.
- To check visuals, run `view --no-open --addr 127.0.0.1:<port>` and screenshot with headless Edge (`--screenshot`; minimum window width ~500px). Stop that test view by its PID, never `taskkill /IM knowledge.exe`, which also kills the user's own view and Claude Code's knowledge MCP server.

**MCP (`internal/mcpserver`).**
- Stdout must carry only JSON-RPC; logs go to stderr. Writes default to author `ai`.
- Tools: explain, search, get, list, stats, sync, pending, annotate, add/assign/unassign domain, fetch, save_web. There is deliberately no delete tool.
- `.mcp.json` points at `bin/knowledge.exe mcp` (relative path, which works when Claude Code starts in the repo root).
- End-to-end check: `claude -p "..." --mcp-config .mcp.json --strict-mcp-config --allowedTools mcp__knowledge__knowledge_explain`.

## Research workflow

- Before answering questions about the project's research or docs, pull context (`knowledge_explain` or `bin\knowledge.exe explain`) and cite `loc` (file:line).
- To keep a web source, use `knowledge_fetch` (or `knowledge_save_web` with the full page as markdown when fetching fails) rather than pasting it into a doc by hand.
- Put research write-ups in `docs/` so they are tracked.
- **Ask the user before syncing the knowledge graph** (the user, 2026-09-25: "dont agresively sync knowledge, ask me first, make it as rule"). This covers:
  - `knowledge_sync`, and `bin\knowledge.exe sync` with any flags. The AI step runs `claude -p` once per pending doc on the user's login.
  - Writing summaries for pending nodes (`knowledge_pending`, then `knowledge_annotate`).
  - `knowledge_fetch`, `knowledge_save_web` and `fetch`, which sync as they save, unless the user asked for that page to be kept.

  Finish the edits first, then ask once: which files changed, how many nodes would need summaries, and whether to run the AI step or write the summaries in the chat. Don't sync after every edit. A task file's "call `knowledge_sync`" means ask then. The structural sync `view` and `mcp` run on start is not covered.
- Figures quoted from BPS/APJII via news coverage should be flagged as such.
- The marketplace-crawl MCP server (Tokopedia etc.) is available for product/price research.

## Tasks

`tasks/` is a queue of work shared by every session, one spec per markdown file. The user either gives a task directly or writes it there, and whichever session is free takes the next one, so no session sits waiting for another.

- When you finish your work, or when asked to do the tasks, take tasks one after another until no `.md` file is left directly in `tasks/`.
- Take them in name order (the user can number them). Skip a file changed in the last minute: the user may still be writing it.
- Look in `tasks/doing/` first. If a task touches the same code as one another session is doing, take a different one first. A line `After: <file name>` means: only once that task is in `tasks/done/`.
- **Take** a task by moving it into `tasks/doing/` with `mv`: if the move fails, another session took it. Add `Taken: <date time>` at its end.
- **Done:** move it to `tasks/done/` with a `## Done` section at its end: what changed, how it was checked, anything left for the user. Work in `typescripts/animation` also brings its areas' state files in `typescripts/animation/docs/states/`, and their `index.md`, up to date first (rules.md, Project rule 8).
- **Needs the user:** move it to `tasks/blocked/` with a `## Question` section at its end, and go on with the next. The user answers in the file and moves it back to `tasks/`.
- If every task left waits on one that another session is doing, check again every few minutes rather than stopping.
- The task file is the spec: follow it as you would a figure spec. Commit only if it says so.

## Git

`bin/*.exe` and `knowledge_data/*.db` are git-ignored: the database is a binary bbolt file rewritten on every change. Structure can be rebuilt from `docs/` with sync. Summaries and domains are only in the database, so version them via `bin\knowledge.exe export <file>.json` and restore with `import`.
