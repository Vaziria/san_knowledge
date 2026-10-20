# project-brain

A living knowledge graph for your project, shared by you and your AI agents.

Agents forget everything between sessions, and a project's knowledge is scattered across docs, notes, specs and finished tasks. project-brain turns every markdown file into one graph: your agents query it before they answer and cite the file and line, and you browse it in your browser. It flags what is out of date, grows as the agents finish tasks, and keeps your corrections as rules.

## What you get

- **san_knowledge, a knowledge graph of every markdown file in the project.** Docs and sections become graph nodes with summaries, keywords, domains and the links between them, in an embedded graph database (goraphdb). One Go binary gives you:
  - a CLI (`knowledge explain "how much is the Ramadan discount"` answers with `path:line`);
  - an MCP server, so every agent session queries the graph before answering;
  - a graph view in your browser on `127.0.0.1:7474`: click domains open into docs and sections, read and edit summaries, run explain, filter, colour by community;
  - AI summaries in the background (Claude Code), or written by the agent in the chat;
  - web pages saved as project knowledge, with their source URL.
- **An agent file** (`CLAUDE.md` / `AGENTS.md`) with verified commands, layout and rules, built from what is already in the project.
- **A task queue** in `tasks/` that several agent sessions work through in parallel: write a task file, and any free session takes it, does it, and leaves a "Done" or "Question" section.
- **Rules from feedback.** When you correct the agent, it writes the correction down as a dated rule with your words and the reason, in the right place, once.
- **Ask-first habits** for anything costly, public or hard to undo, batched into one question.

## Install

1. Copy the `project-brain` folder into your agent's skills folder:
   - Claude Code: `.claude/skills/` in the project, or `~/.claude/skills/` for all projects.
   - Other agents with SKILL.md support: their skills folder (see the agent's docs).
2. Ask the agent to "set up the project brain". It checks for Go 1.25.3+ and Node.js 20.19+ or 22.12+ (Node only for the graph view), asks you, then builds the tool once from the source in `san_knowledge/` into the skill's `bin/` folder. The build downloads the Go modules and npm packages pinned in `go.sum` and `package-lock.json`; nothing is installed system-wide.

No binaries ship in this package: you build the tool from source you can read.

## Use

Ask your agent, for example:
- "Set up the project brain for this project."
- "What do our docs say about the refund policy?"
- "Open the knowledge view."
- "Keep this page as knowledge: https://..."
- "Add this as a rule: never deploy on Fridays."
- "Do the tasks."

## What it touches

- In your project: the agent file, `.mcp.json`, `.gitignore`, `tasks/`, and `knowledge_data/` (the graph database and its config).
- In the skill folder: the build output (`bin/`, `san_knowledge/ui/node_modules/`, the built view).
- Network: only the build's package downloads, and pages you ask it to fetch. No API keys; the optional AI summaries use your own Claude Code login.
- It asks before building, before writing summaries in bulk, and before fetching pages you didn't ask for.

## Files

```
project-brain/
  SKILL.md                            the instructions the agent follows
  README.md                           this file
  references/knowledge-tool.md        san_knowledge: commands, MCP tools, schema, sync, view, troubleshooting
  references/task-queue.md            the task queue protocol and templates
  references/rules-from-feedback.md   how corrections become rules
  references/agent-file-template.md   skeleton for CLAUDE.md / AGENTS.md
  examples/                           an explain result; tasks before, done and blocked
  san_knowledge/                      the knowledge tool's source (Go + the view's React UI)
```
