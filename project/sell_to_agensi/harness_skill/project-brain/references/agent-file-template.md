# Agent file template

Skeleton for a project's `CLAUDE.md` or `AGENTS.md`. Fill in only what you verified; delete sections that don't apply. Keep it short enough to read in full at the start of every session (aim for under 200 lines): long explanations go into docs, which the knowledge graph makes findable, and the agent file links to them.

Replace `<skill>` with the real path of this skill's folder (for Claude Code: `.claude/skills/project-brain` in the project or `~/.claude/skills/project-brain`).

```markdown
# <Project name>

<What this project is, who it is for, and what matters most, in two to four sentences.>

## Commands

    <build command>        # <what it does>
    <test command>         # all tests
    <single test command>  # one test
    <run command>

## Layout

<Only what isn't obvious from folder names: where specs live, generated folders not to edit, the user's spec files.>

## Knowledge

- Every `.md` in the project is in the knowledge graph (`knowledge_data/`, tool `<skill>/bin/knowledge`, MCP server "knowledge"; view with `<skill>/bin/knowledge view`).
- Before answering a question about the project's docs or decisions, call `knowledge_explain` and cite `path:line`.
- After editing markdown, sync once (`knowledge sync --no-ai`). Ask before writing summaries: finish the edits, then ask once. CLI writes use `--author ai`.
- `<spec file>` is the user's spec: do not edit it. Implement it; when it changes, re-read it and bring the work in line.

## Tasks

`tasks/` is a queue shared by every session, one task per `.md` file.
- Take tasks in name order; skip a file changed in the last minute. Read `tasks/doing/` first. `After: <file>` waits for that file in `tasks/done/`.
- Take a task by moving it into `tasks/doing/`; if the move fails, another session took it. Append `Taken: <date time>`.
- Done: append `## Done` (what changed, how it was checked, what is left for the user) and move it to `tasks/done/`.
- Needs the user: append `## Question`, move it to `tasks/blocked/`, go on with the next.
- Continue until no `.md` is left directly in `tasks/`. Commit only if the task says so.

## Ask first

- <Costly or outward actions and who approved what, e.g. "**Ask before deploying** (the user, 2026-10-01).">

## Rules

<Rules from feedback that don't belong to a section above, newest last.>
```

## Notes

- **Several agents:** keep this content in `AGENTS.md` and make `CLAUDE.md` contain only `@AGENTS.md`. Claude Code reads `CLAUDE.md` and follows the import; Codex, Cursor and others read `AGENTS.md`.
- **Big projects:** an agent file in a subfolder can hold that area's rules. Link to it from the root file.
- **What not to put here:** the code's structure in detail (it changes and goes stale), secrets, one-off instructions.
