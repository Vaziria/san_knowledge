# Set up graphify, to spend fewer tokens

The user asked (2026-09-25): "initialize graphifyy on this projects, so its help reduce token consumption". That line is the user's; the rest of this file was written by Claude from it. Choices marked **(default)** are decided, not open questions: build them as written unless the user has changed this file.

graphify (PyPI `graphifyy` with two y's; the command is `graphify`) turns a project into a knowledge graph that an assistant queries instead of grepping and reading whole files.
- Code is parsed locally with tree-sitter, with no LLM. Docs, PDFs and images are read by an LLM, which costs tokens.
- The graph goes in `graphify-out/` (`graph.json`, `GRAPH_REPORT.md`, `graph.html`).
- `graphify claude install` adds a section to CLAUDE.md, and a `PreToolUse` hook that points Claude Code at the graph before it searches or reads files.
- It changes fast (0.9.67 came out on 2026-09-23), and its README, PyPI page and website disagree on some flags. Go by the installed version's `graphify --help` and its skill file. Save its README as a web source first (`knowledge_fetch` on https://github.com/Graphify-Labs/graphify).

This project already has a graph of its own, `knowledge` (see CLAUDE.md). It holds the docs, the figure specs and the web sources, with AI summaries, and sessions must use it for the research and the specs. graphify must not compete with it.

## What to set up

- **Installed with uv (default):** `uv tool install graphifyy`. uv 0.11 is installed. There is no system Python, and uv brings its own. `graphify` must be on the PATH that Claude Code's Bash and PowerShell tools see.
- **Code only (default):** graphify maps the code, and `knowledge` keeps the docs.
  - A code-only graph costs no tokens to build or to keep up to date.
  - The docs are already summarized in `knowledge`. Having an LLM read the 130 docs (111 of them saved web pages) and some 40 figure specs again would spend the tokens this task is meant to save. It would also give every session a second map of the docs to choose from.
  - The code is the animation (`typescripts/animation`, about 150 TypeScript files) and the Go tools (`san_knowledge`, `golang/packages/*` with the `san_tunnels` submodule, and `experimental/transcript`).
- **Left out:** what git ignores (graphify reads each folder's `.gitignore`: node_modules, dist, bin, the Vosk models, vendor). A `.graphifyignore` also leaves out:
  - `san_knowledge/internal/ownview/static/vendor/` (cytoscape, minified);
  - `golang/packages/san_tunnels/gen/` (generated protobuf code);
  - `docs/external_sources/`.
- **In this project only (default):** the skill and the Claude Code settings go in this project (`graphify install --project` or the installed version's equivalent: `.claude/skills/`, `.claude/settings.json`), not in `~/.claude`. The user's other projects stay untouched.
- **The Claude Code hook:** run `graphify claude install`, then read what it wrote.
  - It may point to the graph, but must never block a Read, Grep or Glob. Sessions read specs, tasks and rules.md, which are not in the graph.
  - If it also fires before reading markdown files, narrow it to code where graphify allows that.
  - It must be harmless where graphify isn't installed: the project is also used from Linux (`.mcp.json` runs `bin/knowledge`), and it must not print an error on every tool call there.
- **CLAUDE.md:** keep graphify's section short and in the file's own style. Say plainly which graph is for what:
  - graphify for the code: where something is defined, what calls it, how the parts connect;
  - `knowledge` for the research, the docs and the specs, with `loc` to cite.

  Add `graphify` to Commands.
- **Kept fresh without commits (default):** sessions change the code all day and rarely commit (tasks say "Don't commit"), so git hooks alone would leave the graph stale.
  - Install graphify's git hooks (`graphify hook install`) for commits and branch switches.
  - Also update the code graph when each Claude Code session starts (a `SessionStart` hook), the way `view` and `mcp` sync `knowledge` when they start, if the update takes a few seconds at most.
  - Keep that hook quiet: what a `SessionStart` hook prints goes into the session's context.
  - Several sessions run at once, so two updates at the same time must not break `graph.json`.
- **Allowed without asking:** allow graphify's read-only commands (`query`, `path`, `explain`, or whatever the installed version has) in `.claude/settings.json`, so a query doesn't stop a session to ask for permission.
- **No MCP server (default):** the command line and the skill are enough. `.mcp.json` keeps only `knowledge`.
- **Git:** ignore `graphify-out/` (default). It is rebuilt from the code in seconds, like `knowledge_data/*.db`, and it would change with every session. `.graphifyignore`, `.claude/settings.json` and the skill are files to check in.

Other sessions are working in this project while you do this, and they edit CLAUDE.md too. Change only the lines you need, and never restore it from a copy.

## Checks

- `graphify --version`, then a build of the whole code graph: how long it took, how many nodes and edges it has, and that nothing from the left-out folders is in it.
- Ask the graph three questions a session here would ask, and check each answer against the code:
  - where the lake meeting decides the camera's shot after a turn;
  - what calls `kb.Sync`;
  - where san_youtube sets its poll interval.
- **Tokens, since that is the point:**
  - Before installing anything, run three or four such code questions with `claude -p "<question>" --output-format json` from the repo root. Run each twice, since runs vary. Record the tokens (input, cache reads, output) and the cost from its `usage`. Once graphify is set up, run the same questions the same way.
  - CLAUDE.md tells a session that has finished its work to take tasks from `tasks/`. So allow these runs only read-only tools, and tell each one (`--append-system-prompt`) to answer its question and stop.
  - Add one question about the research or a spec: sessions must still go to `knowledge` for those.
  - The answers must still be right, not just cheaper.
  - If it doesn't save tokens, keep the graph but leave out the hook and the CLAUDE.md section (default), and put the numbers in the Done section for the user.
- Change a code file, then start a new session: the graph has the change, and the session started with no wait and no error.
- The hook never blocks a tool call. A session that reads `docs/3d_modelling/rules.md` or a task file isn't steered away from it.
- Put what you learned in graphify's part of CLAUDE.md.
- Don't commit.

Taken: 2026-09-25 22:44

## Done

**The answer: graphify doesn't save tokens here, so sessions are not pointed at it.** As the task's default says for that case, the graph is kept, and the Claude Code hook and the CLAUDE.md section are left out. What was learned is here rather than in CLAUDE.md, where every line costs tokens in every session.

**What changed**

- **graphify was already installed:** `graphifyy[sql]` 0.9.16, a uv tool since 2026-07-16. `uv tool install graphifyy` kept that version but dropped the `sql` extra (`tree-sitter-sql`). It was put back (`uv tool install "graphifyy[sql]"`), so the tool is as it was. It was not upgraded to the latest (0.9.68), since 0.9.16 has everything used here and the tool may be used by other projects.
- **`.graphifyignore`** (new): graphify maps the code only. It leaves out, beyond what `.gitignore` does: the vendored cytoscape, the generated protobuf code in `san_tunnels/gen/`, test fixtures (`**/testdata/`) and `docs/external_sources/`.
- **`.gitignore`:** `graphify-out/`.
- **`graphify-out/`**, built code-only (`graphify extract . --code-only`, then `graphify update .`):
  - about 240 code files, 3,500 nodes and 7,900 edges, with no LLM, in 8–9 s;
  - no file from a left-out folder;
  - `graph.html` to browse, and `GRAPH_REPORT.md` (its communities are unnamed: naming them needs an LLM API key).
- **graphify's README** saved as a web source (`docs/external_sources/web/github.com/graphify-labs-graphify.md`) and summarized by `bin\knowledge.exe sync`.
- **Not set up:** the `PreToolUse` hooks, the CLAUDE.md section, the skill, an update when a session starts, and the allow rules.
- **No git hooks either:** they can't find graphify's Python on this machine, so they would never rebuild. The profile path has a space (`ASUS TUF`), which graphify's path allowlist rejects, and there is no system Python. Installed and run by hand, `post-checkout` printed "could not locate a Python with graphify installed" and exited 0. They were removed again.

**How it was checked**

- **The measurement:** `claude -p` from the repo root, with the user's default model (Opus, xhigh effort) and read-only tools (`--permission-mode dontAsk`), told to answer and stop.
  - The questions were three about the code, each run twice: when the lake meeting's camera goes where after a turn, what calls `kb.Sync`, and san_youtube's poll interval. A fourth was about the lake meeting's spec, run once.
  - Three setups, 7 runs each:
    - before graphify;
    - graphify's stock setup: its CLAUDE.md text and both hooks;
    - a tuned text (graphify for the code, `explain` for callers, `knowledge` for the docs) with no hooks.

    The setups were passed with `--settings` and `--append-system-prompt`, so the shared files didn't change while other sessions worked.

  | setup | cost (API prices) | cache writes | cache reads | output | turns (avg) |
  |---|---|---|---|---|---|
  | before | $1.485 | 143,533 | 818,895 | 6,983 | 3.9 |
  | stock | $1.664 (+12%) | 157,507 | 999,204 | 7,039 | 4.6 |
  | tuned | $1.459 (−2%) | 126,929 | 965,108 | 7,198 | 4.4 |

  - The runs vary a lot: the same question cost twice as much in one run as in the other. A −2% is noise; +12% with more turns is the stock setup's real cost.
  - Only 2 of the 7 stock runs used graphify, and only one of the 7 tuned runs. Sessions mostly grep anyway: in this code, file and symbol names say what they are.
- **Right answers:**
  - Before: all right.
  - Stock: two lake-meeting answers missed the 3.5 s return to the host's view. One of them had followed graphify to `Follow.ts` and `Sight.ts`, the wrong files, and cost the most of any run ($0.456). A poll-interval answer was wrong too, without graphify.
  - Tuned: one lake-meeting answer missed the 3.5 s.
- **Why graphify doesn't help here:**
  - `graphify query` walks the graph from every name that matches a word. "what calls kb Sync" started from `sync()` in `audio.ts` and san_tunnels' tests. The lake-meeting question listed 405 nodes by how connected they are. Each answer is 3.5–6 KB, mostly beside the point.
  - The graph doesn't link Go method calls to their callers (`s.Sync()`), so `explain "Sync"` shows no callers.
  - `explain` on an exact name is precise (`WithPollInterval` → `chat.go` L47 and its callers), but Grep gives the same for less.
  - graphify's own `benchmark` says 20.9x fewer tokens. That compares one query (about 11,000 tokens) with reading all 234,000 tokens of code for every question, which no session does.
- **The stock hooks** (read in graphify's code, and seen firing in the runs):
  - Before every Read or Glob of a `.ts`, `.go`, `.md` or `.txt` file, they add "MANDATORY: … You MUST run graphify before reading source files". That includes rules.md, the specs and the task files, which a code-only graph doesn't have.
  - They add the same before any Bash command containing `grep`, `find `, `rg ` or `ag `.
  - They never block a call.
  - The command is an absolute Windows path (`C:\Users\ASUS TUF\.local\bin\graphify.EXE`). In the checked-in `.claude/settings.json` it would fail on every call on another machine.

**Left for the user**

- **Keep or remove the graph:**
  - To keep it, browse `graphify-out/graph.html`, and rebuild with `graphify update .` (9 s, no LLM) after code changes.
  - To remove it, delete `graphify-out/`, `.graphifyignore` and the `graphify-out/` line in `.gitignore`.
- **To point sessions at it anyway,** run `graphify claude install`. Then move its hooks from `.claude/settings.json` to `.claude/settings.local.json`, since they carry this machine's path.
- **The sample is small:** 7 runs a setup, of questions that find their way round the code. Sessions doing a task read the files they change anyway, so they would gain even less.
- Nothing is committed.
