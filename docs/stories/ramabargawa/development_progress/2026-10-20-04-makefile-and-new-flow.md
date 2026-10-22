# 2026-10-20: `make play`, and the updated workflow

## Request (the user's words)

| Request | The user said |
|---|---|
| The knowledge sync | "done it" (the new docs are in the graph; their summaries are still pending) |
| A Makefile | "create makefile so i can "make play"" |
| The workflow changed | "i change few flow in @project/bargawa/bargawa_game.md" |

## Done

- **`project/bargawa/Makefile`**:
  - The targets are `play`, `export`, `import`, `check`, `measure`, `editor` and `clean-cache`.
  - Run it from `project/bargawa`, or with `make -C project/bargawa <target>` from the repo root.
  - `play`, `check` and `measure` export the place first when its export is missing or older than the Blender place, `scripts/game/<place>.py` or `common/game.py`.
  - Every target runs one job of `scripts/run.ps1`. The first Makefile used `grep` and `test`. They worked from Git Bash, but from PowerShell make fell back to `cmd`, where they don't exist; `SHELL := pwsh` was ignored by this make (GNU Make 4.4.1 from Chocolatey). One plain command per target works from every terminal.
- **`run.ps1`**:
  - new game jobs: `play`, `import`, `check`, `measure`, `editor`, `clean-cache`
  - `game <place>` now runs Godot's import right after the export
- **README** ("The game") lists the make targets.
- **The preview's state** follows the new flow: model design in Blender is the next step, before the user's answer.
- **Checked**:
  - `make help`, `make -n play` and `make check` ("OK: no errors"), from PowerShell
  - `make help` and `make check` from Git Bash, in the folder and with `-C` from the root
  - `make play` itself opens the game window, so it was left for the user

## The updated workflow ([bargawa_game.md](../../../../project/bargawa/bargawa_game.md), the user's file)

- **References**: "General Reference" files in `project/bargawa/docs/reference/general/*.md` go into San Knowledge, and reach the design through its MCP. The folder doesn't exist yet.
- **Model Design** is now its own branch, from the same join as the story design:
  1. Design in Blender.
  2. Then, side by side: proportion and size; texturing; lighting.
  3. A model summary.

  It goes to the same accept-or-critique choice as the story design.
- **The Design Process** keeps the story design, the design summary, decisions and state, and the HTML preview.

So the characters (Ramabargawa, Jamadagni, Renuka, the brothers, Citrarata) are designed and built in Blender, and accepted, before they go into the game. This covers most of what milestone 2 was. The summaries in `docs/summaries/models/` become the model summaries.

### The second edit ("im edited @project/bargawa/bargawa_game.md")

- **AI Research → San Knowledge ("cached in knowledge")**: what Claude researches goes into the knowledge graph, which feeds the design through MCP.
  - The pages behind today's decisions were cached with `knowledge_fetch`, into `docs/external_sources/web/`:
    - the Godot 4.7.2-stable release on GitHub
    - the MakeHuman license page (core assets CC0, "or derivates")
    - About MPFB
    - the MPFB 2.0 beta 1 notes (the game-engine rig, the Game Engine material)
    - MPFB on Blender Extensions (now at 2.0.18, 10 Oct 2026)
  - They are synced into the graph; their 31 nodes wait for summaries.
- **Model Design → "Saved File"**, then the model summary "same path as blender file": a model's summary sits next to its .blend.
  - [docs/summaries/models/index.md](../../../../project/bargawa/docs/summaries/models/index.md) now says so: its files are the accepted plan each model starts from.
- The Design Process is now called "Story Design", and its empty `model` start is gone.

## Open questions

- References for the model design (`docs/reference/general/`): will the user add some first?
- Ramabargawa's skin, for the texturing. Otherwise both tones are made.
- The animation source matters only once the characters are rigged for the game.

## Next

- The model design in Blender, starting with Ramabargawa:
  - an MPFB body
  - his proportions and size checked against the pendopo and the others
  - the costume textured
  - a light set-up for the review stills
  - then the model summary
- MPFB is installed into a project folder, not into the user's Blender: Blender's `BLENDER_USER_EXTENSIONS` points there for the headless builds.
