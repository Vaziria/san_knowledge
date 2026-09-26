# Animation: record every development state summary in docs/states

The user asked (2026-09-25): "add task, every development state summary in typescripts/animation is recorded in typescripts/animation/docs/states/*.md". That line is the user's; the rest of this file was written by Claude from it. Choices marked **(default)** are decided, not open questions: build them as written unless the user has changed this file.

A "development state summary" is the step "Writing Summary Development State Report" (`state_report`) in `docs/development_lifecycle.md`: after the implementation is tested, the agent writes down where things stand. That file puts state reports in `docs/development_state/[big_context]/[small_context].md`. For the animation project they go in `typescripts/animation/docs/states/` instead, as the user asked here. Read `docs/development_lifecycle.md`, rules.md (Project, Figure specs, Checking the result), and the Done sections in `tasks/done/`.

Today a finished piece of work is summed up only in its task's `## Done` section, in the chat, or not at all, and nothing says what the project as a whole can do now.

## What changes

- **One file per area (default),** `typescripts/animation/docs/states/<area>.md`, in lowercase with underscores like `lake_meeting.md`. The area is the lifecycle's `[small_context]`; the folder stays flat, as the user wrote `docs/states/*.md`. Start with these areas, and split or merge them where the code says otherwise:
  - `lake_meeting.md` and `piano_play.md`, the stories in `src/story/`;
  - `environments.md`: the lake (water, weather, day and night, rain), the floor, the grass field, the sky, the terrains;
  - `trees.md`, `grasses.md`, `animals.md` (the penguin too), `objects.md` (with the boat, the fish and the fishing rod), `pianos.md`;
  - `control_panel.md`: the panel's tabs, the tree, the URL settings, the Kuwahara filter;
  - `streaming.md`: the stream, the chat reader, the audio and OSC bridges.
- **Each file says where the area stands now (default),** short, in the plain style of rules.md:
  - `# <Area> state`, then a line with the date and the task (or piece of work) that last changed it;
  - **What works:** what the area can do today, linking the specs, rules.md sections and the source files it is about;
  - **How it was last checked:** typecheck, numeric checks, screenshots, a browser session, with their dates;
  - **Known issues and left for the user:** from the "Left for the user" parts of the Done sections, and anything seen but not fixed;
  - **Open questions:** drafts the user hasn't confirmed (such as `lake_meeting.md`), and tasks in `tasks/blocked/`;
  - **History:** one dated line per change, newest first, pointing to the task file in `tasks/done/`.

  It is rewritten, not appended to, apart from History: an old state that is no longer true is removed. A task's Done section stays as it is: it tells what the task changed, the state file tells where the area stands.
- **An index,** `docs/states/index.md`: every area with its one-line state and the date it was last updated, and a line pointing to `docs/development_lifecycle.md`.
- **Written from now on (default):** whoever finishes a piece of work in `typescripts/animation`, a task or work the user gives directly, updates the state files of the areas it changed, and the index, before moving the task to `tasks/done/`. Work that changes no behaviour (a comment, a rename) needs none.
- **The state as it is today:** fill every file now from the code, the specs, rules.md, the Done sections of `tasks/done/` (01–13 and 05-stream-through-code-changes), and `git log -- typescripts/animation`. Write what the code does, not what a task hoped for: where they differ, the code wins and the file says so.
- **Tasks already taken:** tasks 09 (dock), 15 (graphify) and 16 (figure part preview) were taken before this rule, and their sessions won't know it. When one of them reaches `tasks/done/` before you finish, record its state from its Done section. Name the ones still in `tasks/doing/` in your Done section, so the next session records them.
- **The rule goes where sessions read it:**
  - rules.md: a new Project rule (the folder, the layout above, and when to write it), linking `docs/development_lifecycle.md`;
  - CLAUDE.md, Tasks, the **Done** bullet: a task in `typescripts/animation` also updates its areas' files in `typescripts/animation/docs/states/`.
- **In the knowledge graph:** `typescripts/animation` is already tracked (`knowledge_data/config.json`), so the new files come in with the next sync; no config change. Links from them to specs and rules.md become `reference` edges, which is why they should link rather than only name things.

Other sessions are changing rules.md, CLAUDE.md and the lake meeting's files. Change only the lines you need, and never restore any of them from a copy.

## Checks

- Every area's file exists and the index lists each one; `docs/states/` holds nothing else.
- Spot-check each file against the code: pick three claims per file under "What works" and find them in the source. Every link resolves: `knowledge_sync`, then look for `reference` edges from each state file to the specs and rules.md it links (`bin\knowledge.exe query`).
- The Known issues of each file cover the "Left for the user" parts of the Done sections for its area, unless the code has fixed them since (then say so).
- rules.md and CLAUDE.md carry the rule; a new session reading CLAUDE.md alone would know to write a state file.
- `knowledge_sync`, then summarize the new nodes (`knowledge_pending`, `knowledge_annotate`) and give the docs a domain, reusing the one the figure specs have.
- Don't commit.

Taken: 2026-09-25 22:58

## Done

**What changed**

- `typescripts/animation/docs/states/` holds 11 files:
  - `index.md`: every area with its one-line state and date, and what cuts across them.
  - One file per area: `lake_meeting.md`, `piano_play.md`, `environments.md`, `trees.md`, `grasses.md`, `animals.md`, `objects.md`, `pianos.md`, `control_panel.md` and `streaming.md`.

  Each is in the layout above. They were written from the code in the working tree, the specs, rules.md, the Done sections of tasks 01–13 and `git log`, where the code wins over a task or a spec.
- Half-built work is named on an "In progress" line in the file of each area it touches, not described as working: the dock (task 09), the part-by-part preview (task 16), and the terrains with their Terrains tab. You asked for the terrains directly; they have no task file.
- Tasks not yet done are named, not linked, since their files move. Tasks in `tasks/done/` are linked.
- rules.md, a new Project rule 8: the folder, the files' layout, rewriting rather than appending, and whoever finishes work in `typescripts/animation` updating it.
- CLAUDE.md: the Done bullet says the same, and the tracked folders now include the state files.

**How it was checked**

- **Claims against the code:** two or three from each file's "What works", found in the source. Among them:
  - the meeting's 12-viewer limit and `!flap`;
  - the fish's own leaps: 20–60 s apart, 0.15–0.28 m;
  - day and night: 6 min of day, 1 of dusk, 4 of night, 1 of dawn;
  - the first shower at 40 s;
  - the willow's 9-clump cascades;
  - the tabs in `TABS`, and the 0.5° drag that frees the view;
  - the OSC port 57121;
  - the frog's gaits, the bear standing upright, the bubble's 9 words;
  - the firefly's flash, `JumpOutFromWater` clamped to 2 m, and the stream URL and key checks.
- **Links:** every relative link and every rules.md anchor in the 11 files resolves.
- **Knowledge graph:** `knowledge_sync` added the 11 docs and 50 sections, with 126 `reference` edges from them. Every file links rules.md and the specs it is about. All 11 docs have the `coding` domain, as the figure specs do.
- `npm run typecheck` is clean at 23:10, with other sessions' unfinished work in the tree.

**Left for the user**

- **Summaries are not finished.** During this task you asked for a new rule: "dont agresively sync knowledge, ask me first, make it as rule". So I stopped the AI step (`bin\knowledge.exe sync`) partway through.
  - It had summarized `animals`, `environments`, `control_panel`, `objects`, `index` and `grasses`. It also summarized `objects/Dock.md`, task 09's spec, which that sync picked up.
  - Still pending: `lake_meeting`, `piano_play`, `pianos`, `streaming` and `trees`, their sections, and rules.md's Project section.
  - Say whether to run the AI step or have a session write them in the chat.
- **The rule, as it now stands:**
  - CLAUDE.md, Research workflow: ask before `knowledge_sync`, `sync`, writing summaries, or a fetch you didn't ask for. Ask once, when the edits are done.
  - The MCP server's instructions and its `knowledge_sync`, `knowledge_fetch` and `knowledge_save_web` descriptions say the same. `bin/knowledge.exe` is rebuilt, and vet and tests pass.
  - Open sessions get the new MCP text only after `/mcp` reconnects.
  - Tasks 09 and 16 are in other sessions' hands, and their task files still end with "call `knowledge_sync`". Those sessions read CLAUDE.md before the rule, so they may still sync.
- **Found while writing, and recorded in the state files:**
  - Most work since commit 278ad67 is not committed, the whole lake meeting included.
  - There are no kept tests.
  - All eight tree specs are empty files.
  - The grand piano is in no preview.
  - rules.md is behind the code in a few places: the tab list, the URL parameters, Checking rule 1's file list, and Objects rule 1's `fit()`.
  - `StreamTab.tsx` still says web + stream streams "with silent sound".
- **States still to record:**
  - Tasks 09 and 16 were still in `tasks/doing/` when this task was done: whoever finishes them should record their states.
  - Task 15 (graphify) finished meanwhile. It isn't animation work, so it has no state file.
- Nothing is committed.
