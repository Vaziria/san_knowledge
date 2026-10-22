# 2026-10-20: The Bargawa game, first requirements

## Request (the user's words)

| Request | The user said |
|---|---|
| Make the story a game | "lets create game bargawa, i have workflow @project/bargawa/bargawa_game.md" |

The user's workflow ([bargawa_game.md](../../../../project/bargawa/bargawa_game.md)) is an Android game adapting the grief story, in these states:
1. The user writes the requirements (`project/bargawa/docs/general.md`) and the story critique (`docs/story.md`).
2. Claude designs: the story (`docs/summaries/story.md`) and the models (`docs/summaries/models/*.md`), then the design summary, decisions and state as `preview/design.html`.
3. The user accepts the design, or critiques and it goes round again.
4. The game is built in the engine, then a preview build. The user accepts it, or critiques.
5. The production build for Android.

The workflow's paths `./bargawa/...` are read as `project/bargawa/...`, where the user made the `docs/` folder.

## Done

- **Requirements draft**: [project/bargawa/docs/general.md](../../../../project/bargawa/docs/general.md), written from the user's four answers below, with the status "draft" until the user accepts it. It ends with eight open points for the user, each with a proposal where there is one.

## Decisions (the user's answers)

| Question | The user chose |
|---|---|
| Kind of game | Narrative adventure: explore in third person, interact, story scenes; few or no fights |
| Look and engine | 3D, Godot 4, from our Blender files, with lighter versions for phones |
| First release | Chapter 1, Jatisrana: beats 1–3, about 20–40 minutes |
| Language | Javanese, with Indonesian subtitles |
| general.md and the critique are the user's | "next dont touch general.md and user critique related": from now on Claude only reads them; questions go in the chat |
| general.md accepted | "accepted" (in the chat, 2026-10-20; the file's Status line is the user's to change) |
| Preview builds on Windows first; price later | "we preview in windows first, and we talk price later": the preview build is a Windows build, Android comes after |

## Open questions

The open points at the end of general.md:
- Renuka's version, the funeral, and Ramabargawa's skin (already open in models.md and environment.md)
- voice or text
- who the player is
- where the characters come from
- the test phone
- landscape or portrait
- price

## Next

- The user edits and accepts general.md, and writes the story critique in `project/bargawa/docs/story.md`.
- Then the design state: `docs/summaries/story.md`, `docs/summaries/models/*.md`, `preview/design.html`.
