---
name: knowledge-sync
description: Sync the knowledge graph with docs/ and the folders in knowledge_data/config.json, report what changed, then ask how to write the pending summaries. Trigger with /knowledge-sync (docs/knowledge.md "Claude Trigger").
---

# Knowledge sync

Calling `/knowledge-sync` is the user's go-ahead for the structural sync. The AI step still needs their answer (CLAUDE.md, "Ask the user before syncing").

Run everything from the repo root. Use `bin\knowledge.exe` on Windows and `bin/knowledge` on Linux.

1. **Binary up to date.** Compare the binary's time with the newest file under `golang/packages/san_knowledge/`. If the source is newer, rebuild first (`pwsh golang/packages/san_knowledge/build.ps1` or `bash golang/packages/san_knowledge/build.sh`). An old binary syncs by old rules: one from before 2026-09-24 ignores `knowledge_data/config.json`. If a `view` or `mcp` is running, move the exe to `bin/knowledge.exe~` before building. Running servers keep the old code until they restart.
2. **Structural sync.** `bin\knowledge.exe sync --no-ai`. On `ErrLocked`, another process holds the database: say which one and stop.
3. **Report**, briefly:
   - the sync counts (docs, sections and references added, changed and removed);
   - the files behind them (`git status --short -- docs` plus the folders in `config.json`);
   - the nodes needing a summary (`bin\knowledge.exe list --needs-summary`), grouped by doc.
4. **Ask once** with AskUserQuestion, unless the user already said which:
   - run the AI step: `bin\knowledge.exe sync` makes one `claude -p` call per pending doc on the user's login;
   - write the summaries in the chat: `knowledge_pending`, then `knowledge_annotate` (or CLI `annotate`);
   - leave them pending.
   Do nothing more until they answer.
