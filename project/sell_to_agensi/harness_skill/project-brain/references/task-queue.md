# Task queue reference

A folder of markdown task files shared by every agent session on the project. The user writes tasks whenever they think of them; any session that is free takes the next one. No session sits idle waiting for another, and every result and open question stays in the task's own file.

## Layout

```
tasks/
  03-pricing-page.md      waiting, taken in name order
  04-copy-review.md
  doing/                  taken by a session right now
  done/                   finished, with a "## Done" section
  blocked/                waiting for the user, with a "## Question" section
```

Number the files (`01-`, `02-`) to set the order. Keep a `.gitkeep` in each subfolder so the folders exist in a fresh clone.

Optional: `tasks/backlog/` parks tasks that aren't ready yet (sessions never take from it; the user moves one to `tasks/` when it is), and a `Labels: a, b` line labels a task. The knowledge view's Board shows the queue as columns (Backlog, Ready = `tasks/`, In progress = `doing/`, Review = `blocked/`, Done) and moves a task's file when you drag its card.

## Protocol

1. **Pick the next task.** List the `.md` files directly in `tasks/` in name order.
   - Skip a file modified in the last minute: the user may still be writing it.
   - Read the tasks in `tasks/doing/`. If the next task touches the same files or code as one in progress, take a later one first.
   - A line `After: 03-pricing-page.md` means the task waits until that file is in `tasks/done/`.
2. **Take it** by moving it into `tasks/doing/`:
   - POSIX: `mv tasks/04-copy-review.md tasks/doing/`
   - PowerShell: `Move-Item tasks/04-copy-review.md tasks/doing/`

   The move is the lock. If it fails because the file is gone, another session took it: go back to step 1. Then append `Taken: 2026-10-19 14:05` to the end of the file.
3. **Do it.** The task file is the spec, and the agent file's rules still apply. Commit only if the task says so.
4. **Finish** with one of:
   - **Done:** append a `## Done` section, then move the file to `tasks/done/`.
   - **Blocked:** append a `## Question` section, move the file to `tasks/blocked/`, and go on with the next task. Do not wait for the answer.
5. **Repeat** until no `.md` is left directly in `tasks/`. If every task left waits (`After:`) on one another session is doing, check again every few minutes instead of stopping.

The user answers a blocked task in its file (under the question, or by editing the spec) and moves it back to `tasks/`. It is then picked up like any new task: read the whole file, including your earlier notes.

## What goes in each section

**`## Done`**
- What changed: files, behaviour, anything the user will notice.
- How you checked it: the commands you ran and what they printed (tests passed, page rendered, numbers matched). Unchecked is said plainly: "not run: needs the production key".
- Left for the user: decisions, manual steps, follow-up tasks you suggest (don't create them unless the task says so).

**`## Question`**
- The decision needed, with the options and your recommendation.
- What you already did, so nobody repeats it.
- What you need from the user exactly (a file, a value, a yes/no).

## Edge cases

- **Never take a file out of `tasks/doing/`.** Another session owns it, even if it looks stale. If a task has sat in `doing/` with an old `Taken:` and no session is on it, ask the user.
- **A task that turns out to be two tasks:** do the part the spec asks for, and suggest the rest under Done.
- **A task that contradicts the agent file's rules:** the newer instruction wins, but say so under Done, and update the rule if the user meant it to change.
- **Several sessions:** they only coordinate through the folders. Keep task files self-contained: a session that never saw the conversation must be able to do the task from the file alone.
- **Tasks that change markdown:** sync the knowledge graph (`knowledge sync --no-ai`) when done; asking about summaries follows the agent file's rule (usually: ask once, at the end of the queue).

## Template for a new task

```markdown
# <What should exist when this is done>

<Why: the goal behind it, in one or two sentences.>

## Do
1. <Concrete step or requirement>
2. <...>

## Check
- <How to know it worked: a command, a page to open, a number to match>

After: <file name, if it must wait for another task>
Commit: <yes, with message "...", or leave it out for no commit>
```

See `examples/task-before.md`, `examples/task-done.md` and `examples/task-blocked.md`.
