# Brain

A self-improving memory for AI agents, stored as a graph instead of markdown files. Agents read it and write it while they work, and it improves from the outcomes of that work. Knowledge in it emerges; nobody designs it.

Status: agreed with the user in a discussion on 2026-10-31. All six stages of the [build order](#build-order) were built that day, on the `knowledge` branch, and the parts for daily use on 2026-11-01. It builds on the [knowledge tool](knowledge.md).

## Idea

- **Loop engineering plus a self-improving agent.** An agent runs tasks in a loop, every task is checked, and what helped or misled is written back to its memory.
- **The memory is a graph, not files.** Hermes-style agents keep `MEMORY.md` and skill files. Here the memory is a goraphdb graph. Agents query it with raw Cypher and change it through guarded tools.
- **A brain is one graph file.** Brains can be switched, forked and compared. A fork is a copy that learns on its own, so two ways of learning can be compared on the same exam.
- **No roles or personas,** not even emergent ones. A brain only holds knowledge. The part of it a task activates is the only "persona", and it is never named or stored.

## Layers

### Fact layer

The existing knowledge graph: docs, sections, domains and links, built from the files by sync. A brain reads it and never writes it.

### Mind layer

Everything the AI writes: concepts, claims, runs, outcomes and the edges between them. Each brain has its own mind layer in its own database file.

A database file cannot link to another file's nodes. A brain therefore points at the fact layer by key: a `source` node that holds the fact node's `key` (`docs/x.md#section`) and copies its title and file:line. When sync removes or changes that doc or section, the brain's claims resting on it are flagged for a new check (see [Sleep](#sleep)).

### Keys and labels

- **Keys** start with the node type:
  - `concept:posting-time`: the slug of the name
  - `claim:3f2a9c0e1b7d`: a hash of the text, so the same claim told twice is one node
  - `source:docs/x.md#section`: the fact's key
  - `run:20261031-153000.123` and `outcome:20261031-153000.123`: the time of the run
- **Labels** are the node types (`Source`, `Concept`, `Claim`, `Run`, `Outcome`), so raw queries read `MATCH (c:Claim)-[:about]->(t:Concept {key: 'concept:tiktok'}) RETURN c, t`.

## Grammar

The grammar is the set of node types, edge types, their rules and the properties they must carry. The content (which nodes and edges exist, and their weights) emerges. The grammar changes only through the [grammar gate](#grammar-gate).

### Node types

| Type | What it is |
|---|---|
| `source` | A fact-layer doc or section, by key. Read-only. |
| `concept` | A thing or idea: "TikTok", "posting time", "Go" |
| `claim` | A statement that can be true or false, in free text |
| `run` | One task the brain worked on |
| `outcome` | The checked result of a run |

### Edge types

| Edge | From → to | Meaning | What the algorithms do with it |
|---|---|---|---|
| `about` | claim → concept | What the claim talks about | Activation spreads along it |
| `evidence_for` | source, run → claim | Backs the claim | Raises the claim's confidence |
| `evidence_against` | source, run → claim | Weakens the claim | Lowers it |
| `applies_when` | claim → concept | The claim holds only in this context | The claim is active only when the context is |
| `leads_to` | claim, concept → claim, concept | Cause or consequence, as the evidence states | "Why" chains |
| `part_of` | concept → concept | Hierarchy ("TikTok" part of "social media") | Activation spreads along it |
| `same_as` | concept → concept | Duplicates | Sleep merges them |
| `supersedes` | claim → claim | The newer claim replaces the older one | The older claim is no longer activated |
| `used` | run → claim, concept | What a run relied on | Credit after the outcome |
| `result` | run → outcome | What happened | Credit after the outcome |

### Required properties

Every mind-layer edge carries:

- `reason`: why the edge exists, in a sentence
- `evidence`: file:line, or a run id
- `author`: which step wrote it, and with which model
- `confidence`
- `status`: its trust level (see [Verify](#verify))
- `created`
- `uses` and `wins`

The guarded write tools refuse anything outside the grammar, the way the knowledge tool's `validRelation` does for the fact layer.

### Guarded writes

A brain changes only through these. Raw Cypher on a brain is read-only: the library's read-only mode refuses CREATE, MERGE, SET and DELETE.

- **remember:** adds a concept, a claim or a source. A source must be a doc or section of the knowledge graph. Remembering the same name, text or fact again returns the node already there.
- **link:** adds an edge the grammar allows, with its reason and evidence. It starts as proposed.
- **retire:** takes a node or edge out of use, with why. It stays in the brain with the reason; sleep may remove it later. Runs, outcomes and the edges between them are history and are never retired.
- **record a run:** its task, its check, the claims and concepts it used, and the edges it followed. Its outcome is recorded with it or later, once.
- **Credit only from outcomes.** An outcome gives everything the run used one use, plus a win if the run passed. There is no tool to raise uses or wins by hand.

### Claims carry meaning, edges carry structure

Edges hold only the meaning the algorithms need. Anything else is a claim, whose text can say anything:

```
claim: "posting at 19:00 gets twice the reach of 12:00 for Indonesian audiences"
   ├── about ──────────▶ posting time
   ├── about ──────────▶ reach
   ├── applies_when ───▶ Indonesian audience
   └── evidence_for ◀── run #42 (outcome: reach 2.1×)
```

A small grammar therefore does not limit what a brain can know.

## Learning

A brain improves in three places:

- [verify](#verify): what gets accepted
- credit, in [a run](#run): what gets stronger or weaker
- [sleep](#sleep): what gets merged, pruned or questioned

### Run

1. **Task.** A task arrives with its own [check](#outcome-checks).
2. **Activate.** The task's concepts and the [core](#activation) are loaded as context.
3. **Work.** The agent does the task. It can query the brain further with raw read Cypher.
4. **Check.** The outcome is checked and recorded as a `run` with its `outcome`.
5. **Credit.** The claims and concepts the run `used` gain uses and, if it succeeded, wins.
6. **Learn.** New claims and edges are proposed from the run and from changed docs. They pass [verify](#verify) and are stored as hypotheses.

### Verify

A neutral truth check, the same for every brain. A separate, fresh model call (not the agent that proposed it) checks each new edge against its quoted evidence: does the source really say this? Edges that fail are not stored.

Levels of trust, lowest first:

1. proposed
2. verified
3. confirmed by the user

How learning does it (`knowledge brain learn`, `kb/learn.go`), per doc:

1. **Propose:** one call (sonnet). It reads the doc's own text and its sections, plus the concepts the brain already has that the doc mentions and the claims about them. It answers with:
   - claims, each with its section, an exact quote, the concepts it is about, the contexts it only holds in, and a confidence
   - this doc's evidence for or against known claims
   - links: `leads_to`, `part_of`, `same_as`, `supersedes`
2. **Quote check, no AI:** every item's quote must appear word for word in the section it names. Items that fail are dropped.
3. **Verify:** a second, separate call (haiku) judges each remaining item against its section, strictly.
4. **Write:** only what passes is written, as `verified`, with the quote on its edges. Then the doc is marked learned, with the hash of its text, so it is learned again only when it changes.

`knowledge brain sample` shows random verified claims with their evidence, for the user to confirm (`brain confirm`) or retire.

### Activation

How a task finds what it needs:

- **Seeds.** The task's text is matched to concept and claim nodes by keyword ranking, the way the knowledge tool's search ranks nodes. goraphdb has no vector search.
- **Spread.** Activation spreads one or two hops along the strongest edges, weaker at each hop, and only the top few nodes are kept.
- **Context.** A claim with `applies_when` is active only when its context concept is active.
- **Hubs.** Very common hub concepts count less, like common words in a search engine. Otherwise everything reinforces "marketing" until it means nothing.
- **Core.** The most used and most successful nodes are always loaded, within a small fixed budget, as a stable context start. The core emerges; nobody writes it.

As built (`kb/recall.go`, `knowledge brain recall`, MCP `brain_recall`):

- **Seeds.** A node's seed is the share of the task's words in its name or text. Words match when they are equal, or when one starts with the other and both have 4 letters or more.
  - A claim's seed is weighted by its record in the task's context (below), its confidence and its trust level.
  - A concept's seed is divided by log₂(2 + its degree), so hubs count less.
- **Spread.** Two hops, half as strong each hop. How much passes along an edge is the edge type's `spread` in the grammar, times the edge's own confidence and record in the task's context, divided by log₂(2 + the out-degree).
- **Record in context.** A run's context is the concepts its task names. Each use of a claim or an edge is kept with its run's context and whether the run passed (the latest 50, property `ctx`).
  - For a task, each use counts as much as its context overlaps the task's (Jaccard over the concepts), and the record is `2·(wins+1)/(uses+2)` with those weighted counts.
  - A claim with no record in the task's context counts as neutral (1). So a win in one kind of task no longer pushes it into unrelated ones.
  - `RecallOptions.GlobalCredit` uses the whole record instead, for comparison.
- **Damping.** `applies_when` and `supersedes` multiply a claim's score by 0.3 when it does not hold in an active context, or a newer active claim replaces it.
- **Facts.** The sources reached are the facts, best first. Used together with explain, the two rankings are fused by reciprocal rank (k = 60).

### Sleep

A scheduled pass over one brain, with a [snapshot](#brains) first:

1. **Decay.** Edges that are not used lose weight.
2. **Merge.** `same_as` duplicates become one node.
3. **Prune.** Edges and claims with very low weight are removed.
4. **Contradictions.** Two claims that contradict but each win in their own context are separated with `applies_when` context nodes, not by splitting the brain.
5. **Stale evidence.** Claims whose `source` changed or disappeared in the fact layer are checked again.
6. **Gaps.** Concepts with claims but no evidence, or isolated clusters, become questions.
7. **Grammar patterns.** Many claims of the same shape become a [grammar proposal](#grammar-gate).

Sleep is incremental: it only looks at what changed since the last one.

As built (`kb/sleep.go`, `knowledge brain sleep`, tower step Brain Sleep), with no AI:

- **Snapshot** first.
- **Decay:** what was not used since the last sleep keeps 95% of its confidence.
- **Merge:** concepts joined by a verified `same_as` become one.
- **Retire:** claims and edges whose weight fell under 0.1 are retired. The weight is (0.5 + confidence) × record × trust.
- **Remove:** what was retired more than 30 days ago is deleted.
- **Stale:** a claim whose every source changed or disappeared goes back to `proposed`, marked `stale`. Learning the doc again verifies it again.
- **Listed for a look:** claims with evidence both for and against (contested), and claims with no evidence left (gaps).
- **AI part** (`kb/dream.go`; `knowledge brain sleep --ai`; Brain Sleep with AI Yes). It runs after the rest, and the brain is closed during the calls.
  - **Contexts:** a claim is a candidate when it is contested (evidence for and against) or has a mixed record (it won in some runs and lost in others). Contested claims go first, up to 8 per sleep, in one call (sonnet).
    - The AI gets each claim with its evidence (the sections' text), its runs (the task, the task's concepts, passed or not) and the other claims about its concepts.
    - It names the contexts the claim holds in, citing the evidence or runs that show it. Each becomes a proposed `applies_when` edge to that concept, with a file:line or a run key as its evidence. A context with no evidence is dropped.
    - Each claim is looked at once; the list is kept in `dream.json` in the brain's folder.
  - **Grammar patterns:** once a week, if the brain has 30 claims or more, one call gets:
    - the grammar
    - how each edge type is used (count, uses, wins)
    - the 60 newest claims with their edges

    It may propose a change. A proposal that fits the grammar is written to `knowledge_data/brains/proposals/` for the [gate](#grammar-gate). It is never applied automatically.
  - **First real run** (2026-11-01, on a fork of root that was then deleted): 2 calls, 19 s.
    - Of 8 claims, one got a context: the fee figures of `golang/apps/autotrade/USAGE.md:103` hold only for the Binance testnet account. The other 7 had no context.
    - No grammar proposal.

## Four speeds

| Layer | What changes | How often | Who decides |
|---|---|---|---|
| Weights | Edge and node weights | Every run | Outcomes |
| Content | Nodes, edges, claims | Every run and every sleep | The AI, through verify |
| Grammar | Node and edge types and their rules | Rarely | The [grammar gate](#grammar-gate) |
| Meta-rules | How learning and grammar changes work | Never by itself | Only the user |

### Grammar gate

1. Sleep proposes a change, with a definition and examples. It can add, merge or retire a type.
2. The brain is forked, and the fork is migrated to the new grammar.
3. Both take the held-out [exam](#exam).
4. The change is kept only if the fork scores better; otherwise the fork is deleted.
5. On a pass, the grammar version goes up and every brain is migrated, each after a snapshot.

There is one grammar, shared by all brains and versioned, so every brain stays forkable and comparable.

As built (`kb/gate.go`, `knowledge brain gate <proposal.json>`):

- **The grammar** is the built-in grammar 1 until a gate keeps a change. It is then written to `knowledge_data/brains/grammar.json`.
- **A proposal** is a list of ops: `add` (a new edge type with its rule), `drop`, `rename`, `spread` (how much activation an edge type passes). `used` and `result` are never changed.
- **The score** is the held-out MRR of the brain's recall alone, since only the brain changes. A change is kept only when it is strictly better.
- **The record:** every gate is saved in `knowledge_data/brains/gates/`.
- **Where proposals come from:** a person, or a sleep with its AI part, which writes them to `knowledge_data/brains/proposals/` with example claims of the pattern. A gate is still run by hand (`knowledge brain gate <file>`), because every gate looks at the held-out exam.
- **Caution:** every gate run looks at the held-out questions. Many proposals tried against the same exam fit the exam rather than the knowledge. Keep gates rare, and make a new exam when many have run.

### Meta-rules

These change only when the user changes them:

- Every edge carries evidence and provenance.
- The truth check is neutral and checks against the source.
- Outcomes come from the task's check, never from the brain judging itself.
- Grammar changes must pass the grammar gate.
- A snapshot comes before every sleep and every grammar migration.
- The brain cannot see or edit the held-out exam.

A system that can rewrite the rules that judge it rewrites them to make success easy. Everything else may emerge.

## Brains

- **Files.** A brain is a folder, `knowledge_data/brains/<name>/`, git-ignored like the knowledge database:
  - `brain.json`: its name, grammar version, and the brain it was forked from
  - `graph/`: the goraphdb graph
  - `snapshots/`: JSON snapshots, one file each
- **Versioning.** A brain is versioned with an export to JSON (`knowledge brain export <file>`), as the knowledge tool does.
- **Fork** copies a brain exactly; **compare** runs the exam on two brains.
- **Restore** puts a brain back as a snapshot has it, after first snapshotting how it was, so a restore can be undone. Only the user restores, from the CLI.
- **Switching** happens only between runs, never during one, so what a run learns lands in the brain it started with. A brain loaded at the start of a run also keeps the prompt's beginning stable for caching.
- **The root brain** is the first one. More brains come from forks.
- **Locking.** Each brain is its own file with its own lock. Runs on different brains do not block each other; runs on one brain take turns, through `kb.With`.

## Outcome checks

- **In the experiment:** the [exam](#exam).
  - **Practice** (`knowledge exam practice`) runs the brain on each practice question and records a run. The run's check is that the first fact recalled is the answering section; its outcome credits the claims that led there, and the edges from that source.
  - **Score:** `knowledge exam score --brain <name>` scores the recall alone, and fused with explain.
- **In daily use:** the citation check. It verifies, the same way as [verify](#verify), that the sources an answer cites really support it. It is automatic and objective.
  - **As built** (`kb/citation.go`, 2026-11-01):
    - **Citations:** an answer cites by `path.md:line`, `path.md#heading`, or `path.md` alone. The end of a path is enough when only one doc has it. Each is mapped to the doc or section it names.
    - **The check:** each line that cites a section is checked once, up to 20, in one call (haiku). The call judges each line against the text it cites.
    - **The run:** `Brains.Answered` records the task and the answer as a run. What it used are the claims behind the facts the brain recalls for the task that the answer cites, and the `evidence_for` edges from their sources.
    - **The outcome:** a pass when every citation holds; the score is the share that holds. An answer that cites nothing gets no outcome.
    - **Where:** MCP `brain_check_answer`, `knowledge brain check --task … <answer-file>`, and in the tower, Brain Run and the Brain Tool's `check`.
    - **First real run** (2026-11-01): an answer with a true line and a false line, both citing `golang/apps/autotrade/USAGE.md:103`. The check held the first and failed the second ("It is not filled at market price"), and the run was recorded as failed.
- **Later:** the user's reaction to an answer, and tasks that bring their own check (tests pass, a post's reach). Either goes to Brain Run's Success instead of the citation check, or to Brain Outcome (MCP `brain_record_outcome`) for a run that waits for its outcome.

## Exam

The question set the brain is scored on.

- **Generated once,** by a separate AI pass over the docs. Each question comes with the file:line that answers it.
- **Checked by the user** at a glance, then frozen.
- **Two parts:**
  - **Practice:** the brain is run on these and learns from their outcomes.
  - **Held-out:** used only for scoring and the [grammar gate](#grammar-gate). The brain never learns from them and cannot see them. Otherwise it would learn the exam instead of the knowledge.
- **Score:** did the answer find the right source, and does the citation check pass?
- **Baseline:** today's knowledge graph (`explain`) on the same questions. A brain only counts as better if it beats the baseline on the held-out part.

### How it is kept and scored

- **Where:** `knowledge_data/exam/exam.json`, with scorings in `knowledge_data/exam/results/`. It is JSON, not markdown, so sync never puts it into the graph, where `explain` or a brain would find it. There are no MCP tools for it.
- **Generate** (`knowledge exam generate`):
  - **Which docs:** ones with at least two sections of 200 or more characters of their own text, spread across the project's folders.
  - **No copied text:** a section whose text also appears in another doc is never asked about, because it has more than one right answer. Template files such as the `AGENTS.md` in every video folder are like this. `exam show` marks questions like that in a draft so they can be dropped. The first draft (2026-10-31) lost 16 of its 80 questions this way. The default is 20 docs, one `claude -p` call each, two questions per doc. `--plan` shows the docs and the number of calls without calling anything.
  - **The prompt:** the AI is asked to word each question like someone who hasn't read the doc, not with the section's heading. Each question names the section that answers it; questions naming a section it wasn't given are dropped.
  - **Parts:** questions alternate between practice and held out.
- **Check and freeze:** `knowledge exam show`, `knowledge exam drop <id…>`, `knowledge exam freeze`. A frozen exam is never written again.
- **Score** (`knowledge exam score`, no AI) ranks each question the way `explain` does and records where the answering section lands in the top k (default 5):
  - `hit@1`: the right section comes first
  - `hit@k`: the right section is in the top k
  - `doc hit@k`: a match from the right doc is in the top k
  - `MRR`: the mean of 1/rank
  - Each part is scored on its own. A question whose section has since disappeared counts as stale, not as a miss.

## Where it runs

- **Code:** inside the knowledge package (`golang/packages/san_knowledge`), in `internal/kb` like the rest. Other modules, and the tower, use it through the `graph` package.
- **Agent access:** MCP tools on `knowledge mcp`, on the brain named by `--brain` (default `root`) unless a call names another.
  - Read: `brain_list`, `brain_grammar`, `brain_stats`, `brain_get`, `brain_recall` (activation), `brain_query` (read-only).
  - Guarded writes: `brain_remember`, `brain_link`, `brain_retire`.
  - Loop: `brain_record_run`, `brain_record_outcome`, `brain_check_answer` (the citation check, which records the run).
  - Brains: `brain_snapshot`, `brain_fork`, `brain_create`. There are no delete or restore tools; those are the user's, from the CLI.
- **Schedule:** tower workflows. The steps **Brain Learn** and **Brain Sleep** (`golang/apps/san_tower/internal/engine/brain.go`) run from a Schedule step, the way Knowledge Sync and Knowledge Summarize keep a knowledge up to date. A nightly workflow is: Knowledge Sync → Brain Learn → Brain Sleep. Brain Learn calls `claude -p` itself, with settings Brain, Limit, Model and Verify Model. Brain Sleep has AI (No or Yes) and Model. The tower needs a release before it has these steps.
  - **The nightly workflow** is workflow 29 `nightly brain` on the local tower, in its `brain` folder (`data/add-brain-workflow.ps1`). At 03:00 it runs on Research Marketing's root brain: Knowledge Sync, then Brain Learn (Limit 20 docs, so 40 calls at most), then Brain Sleep with AI. It was created switched off, for the user to switch on.
- **Daily use in the tower:**
  - **Brain Recall** gives an AI step's prompt what the brain recalls for a task.
  - **Brain Run** records the task and the answer as a run. Its outcome comes from the citation check, from the task's own check (Success), or later.
  - **Brain Outcome** gives a waiting run its outcome.
  - **Brain Tool** gives an AI step `recall` and `check` (Remote Shell's or Claude Ask's Tools).
- **UI:** the view's Brain mode (`knowledge view`, the Brain button, or `?view=brain`).
  - Pick a brain.
  - Its claims can be filtered by words and by trust level (or retired), and sorted by weight, uses or newest. Each shows its concepts, contexts, record and weight; opened, it shows its evidence, where a source selects its doc or section.
  - A recall box, and the latest runs with their outcomes.
  - It is read-only (`kb/brainview.go`, `ownview/brain.go`, `ui/src/BrainView.tsx`).
- **CLI:** `knowledge brain …` subcommands for running by hand.

## Cost

- **Caps:** every run and every sleep has a cap on model calls. What is left over waits for the next one, as `AIOptions.MaxDocs` does for summaries.
- **Models:** verify uses a cheap model; proposals use the default model.

## Rules with the existing knowledge

- **The mind layer learns on its own** (the user, 2026-10-31). Its writes do not need the user's OK, because:
  - they stay inside the brain's own file
  - they never touch the fact layer
  - every sleep is snapshotted
- **The fact layer keeps its rule:** sync and summaries are asked for first.

## Build order

Each stage has its own check before the next starts.

1. **Storage.** Brain files, the grammar, guarded writes, raw reads, fork and snapshot (CLI and MCP).
   - Check: tests, and a snapshot restores exactly.
   - Done 2026-10-31: `internal/kb/brain.go`, `knowledge brain …`, the `brain_*` MCP tools. Tests cover the grammar and required properties, read-only raw queries, credit from outcomes only, and exact fork and restore.
2. **Exam.** Generate the question set and score the baseline.
   - Check: a baseline score.
   - Code done 2026-10-31 (`internal/kb/exam.go`, `knowledge exam …`).
   - **The draft:** generated the same day from 40 docs with 40 AI calls. It had 80 questions; 16 that ask about copied text were dropped, leaving 64 (32 practice, 32 held out). It waits for the user to freeze it.
   - **Baseline on the draft**, top 5:

     | | hit@1 | hit@5 | doc hit@5 | MRR |
     |---|---|---|---|---|
     | practice | 0.00 | 0.38 | 0.81 | 0.12 |
     | held out | 0.03 | 0.44 | 0.69 | 0.16 |

     Explain usually finds the right doc but rarely ranks the right section first: a whole doc matches more of a question's words than the one section that answers it.
3. **Learn from docs.** Propose and verify.
   - Check: the user samples edges; the share they agree with.
   - **Built 2026-10-31.** The root brain learned 40 docs with 80 AI calls: the 32 the exam asks about, plus 8 others spread through the project.
     - It wrote 573 claims, 490 concepts, 11 links and 1 piece of evidence on a claim from another doc. 44 items were rejected by the truth check.
     - The user's sample is still to come (`knowledge brain sample`). Claude's own reading of 20: none clearly wrong. But the quote often covers only part of a claim, and some claims are in Indonesian although their source is English.
4. **Runs.** Recall, credit, and scoring on the held-out exam.
   - Check: beats the baseline.
   - **Built and checked 2026-10-31.** Held-out scores, top 5:

     | | hit@1 | hit@5 | MRR |
     |---|---|---|---|
     | explain on every doc (the baseline) | 0.03 | 0.44 | 0.16 |
     | explain on the 40 learned docs (the fair control) | 0.06 | 0.66 | 0.27 |
     | the brain alone | 0.28 | 0.66 | 0.43 |
     | the brain fused with explain | 0.41 | 0.81 | 0.60 |

     - The brain mostly ranks the right section first: the same hit@5 as the control, but far more hit@1.
     - **Practice did not help the held-out questions.** After the 32 practice runs (15 passed), the practice questions rose (hit@1 0.44 → 0.53), while the held-out ones fell slightly (brain alone MRR 0.43 → 0.40, about one question).
     - The cause: credit is global. A claim that won once is boosted for every task, so a section that won one practice question (`project/bargawa/README.md#conventions`) now comes first for unrelated questions. This is the interference that "weights relative to context" was meant to prevent.
     - **Fixed the same night:** credit is now [relative to the task's context](#activation). On the frozen exam, after the same practice:

       | brain alone, MRR | practice | held out |
       |---|---|---|
       | before practice | 0.54 | 0.43 |
       | after practice, global credit | 0.60 | 0.40 |
       | after practice, context credit | 0.62 | 0.44 |

       Held out, the brain alone no longer falls after practice, and practice rises more. Fused with explain, held out is 0.59, against 0.17 for explain alone and 0.27 for the fair control.
       - A first version that counted uses per concept made held out worse (0.34). It was replaced by the per-run record.
       - The root brain's snapshot from before this practice is `20261031-234006.294`.
5. **Sleep,** scheduled in a tower workflow.
   - Check: the score does not drop after a sleep.
   - **Built 2026-10-31,** with the tower steps Brain Learn and Brain Sleep. The first sleep of the root brain had nothing to do (no history yet), and the score did not move.
6. **Grammar gate.**
   - Check: a planted good proposal passes and a planted bad one fails.
   - **Built 2026-10-31.** The planted proposals behave in the tests. On the real brain, a planted bad proposal (drop `evidence_for`) was rejected: held-out MRR 0.40 → 0, grammar stays 1.

7. **Daily use.** The citation check, context credit, sleep's AI part, the tower's brain steps and Brain Tool, the view's Brain mode, and the nightly workflow.
   - Check: tests with fake AI calls, and one real call of each new kind (above).
   - **Built 2026-11-01.**
   - **Still to do:** switch the nightly workflow on, and have daily tasks run through Brain Run, `brain_check_answer` or the Brain Tool, so that real outcomes come in.

## To tune

- Activation: how many hops, the decay per hop, how many nodes are kept, the hub penalty, the core's budget
- The weight formula. A starting point:
  - confidence from evidence counts, (for + 1) / (for + against + 2)
  - usefulness from wins over uses, with the same kind of prior
- Sleep: its schedule, the order of its steps, and the prune threshold
- The exam's size, and the practice/held-out split
- Call caps and the model for each step
- How brains show in the UI (a first version: the view's Brain mode)
