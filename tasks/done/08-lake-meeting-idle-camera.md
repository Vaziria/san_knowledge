After: 07-lake-meeting-roaming.md

# Lake meeting: when the chat is quiet, follow an animal from a few angles

The user asked (2026-09-25): "in lake meeting, when no comment new in 30 second, camera randomly follow animal and take few different angle". That line is the user's; the rest of this file was written by Claude from it. Choices marked **(default)** are decided, not open questions: build them as written unless the user has changed this file.

The story is in `src/story/lake_meeting/`. Read `lake_meeting.md` (a draft spec written by Claude, not yet confirmed by the user) and `Meeting.ts`, where the camera's turns are (`shot()`, `WIDE_AFTER`, `LEAD`, `READABLE`, `Shot.cut`). Read task 07 (`tasks/done/07-lake-meeting-roaming.md` once it is done), which makes the animals roam the lake's land. Then read rules.md (Stories, Control panel rules 9 and 11 on the camera).

## What changes

- **The 30 s count:** every comment starts the 30 s again. **So do commands and supporters' leaps (default),** since each shows something too.
- **Before the 30 s, as now:** the camera stays with the last one a few seconds, then goes back to the host's view (spec item 10).
- **After 30 s with nothing new, 30 s on each animal.** The user asked, before this task was taken: "when iddle camera follow random animal for 30 second, and after that follow random animal 30 second, and random again, and so on".
  - The camera picks an animal at random and follows it for 30 s, then another at random for 30 s, then another, and so on.
  - Any animal at the meeting counts, bots and the host included.
  - **Never the same one twice running (default).**
- **Following:** it keeps the animal in frame as it roams, aiming a little ahead of it when it moves, as the turns do (`LEAD`). The animal fills about a third to half of the picture, so a bird's shot is much closer than the bear's.
- **A few different angles (default):** its 30 s on an animal is about four shots of 6–9 s each, each from a different angle, then the next animal. The angles are:
  - three-quarter front at its eye level;
  - side-on;
  - low from the ground, looking up;
  - high, looking down;
  - behind it as it walks.
- **Cuts, not glides (default):** each new angle, and each new animal, is a cut, as a film cuts between shots. Between cuts the camera moves smoothly with the animal.
- **Never through things:** an angle must not put the camera in the ground (the lake's land has humps and hills), under the water, or inside a trunk, boulder or big stone, nor with one of them between the camera and the animal. If it would, pick another angle.
- **It stops at once** for the next comment or command, which takes the camera as a turn does now. It never takes the camera from a viewer who has moved it away: it waits until they let go, as every shot does (spec item 10).
- **Day, night and rain alike.** It runs in the stream's hidden page like the rest of the story.
- **The spec:** add this to item 10 of the draft `lake_meeting.md`, keeping its note at the top that it is a draft for the user to correct.

Other sessions are changing `src/Stage.ts`, `src/previews.ts` and the lake's files. Change only the lines you need, and never restore any of them from a copy.

## Checks

- `npm run typecheck` in `typescripts/animation`.
- **Numeric checks,** over a long run in Node with no comments (10 min of the meeting, 7 bots):
  - it starts following 30 s after the last turn, and not before;
  - the animal is in the picture and nothing stands between it and the camera (project it through the camera);
  - the camera is never in the ground, the water or a trunk, boulder or stone;
  - each animal is followed for 30 s, then the next;
  - the angles change within those 30 s;
  - no animal is followed twice running;
  - nothing is NaN.
- **Screenshots** with headless Edge on a spare port, never 8087 (rules.md, Checking the result), driven by the Story tab's test messages (spec item 9):
  - several angles on a big animal (the bear) and a small one (the bird or the cat);
  - a test comment during the follow, which takes the camera at once;
  - with the Kuwahara filter on, by day and at night (`?time=night`).
- **In a browser session:** dragging the view during the follow keeps it with the viewer, and the next comment takes it back.
- Add what you learned to rules.md (Stories), then call `knowledge_sync`.
- Don't commit.

Taken: 2026-09-25 19:12

Taken over: 2026-09-25 22:25 (the first session stopped; nothing changed in the meeting since 20:19)

## Done

The first session (transcript `45d8f4ec…`) built this and stopped at 20:31, while it was writing its screenshot script. It no longer ran. A second session (this one) checked what it had built, found it complete, and did the rest: the browser checks, the spec and the rules.

**What changed**

- `src/story/lake_meeting/Follow.ts` (new): the camera while the chat is quiet.
  - A random animal, the host and the bots included, for 30 s each, never the same one twice in a row.
  - About four shots of 6–9 s on each, from different angles: front at eye level, side-on, high, low (only while it stands), and behind (only while it walks). A cut at each new shot and each new animal.
  - The animal fills 36–46% of the 16:9 picture. The camera tracks it smoothly between cuts and aims a little ahead of one on the move.
  - A shot that becomes blocked cuts to another angle after 1 s. An animal with no clear angle is passed over.
- `src/story/lake_meeting/Sight.ts` (new): what a camera may not be in or see through.
  - the ground mesh's own heights, and the water;
  - the trees and boulders, as 25 cm cubes marked once (raycasting their 667 k triangles was too slow for every frame);
  - the stones, as cylinders.
- `Meeting.ts`:
  - `FOLLOW_AFTER = 30`: the follow starts 30 s after the last turn and stops at the next one.
  - Each follow frame is a cut, so the stage puts the camera exactly where the follow says.
  - The follow never claims the camera, so a viewer who moved it keeps it until the next turn.
- The draft spec `lake_meeting.md`, item 10: this behaviour, with its note that it is a draft kept.
- `docs/3d_modelling/rules.md`: Stories rule 19. Knowledge synced.

**How it was checked**

- `npm run typecheck` passes.
- **Numeric checks in Node**, 10 min of the meeting with 7 bots and 4 viewers (frog, snake, bird and cat), seed 7:
  - it first followed at 30.0 s;
  - 20 animals of exactly 30 s each, none twice in a row;
  - four shots each, 6.03–9.00 s, never the same angle twice in a row;
  - in 2850 raycast checks the camera was never under the ground mesh, in the water, inside a tree, boulder, stone or animal, and nothing came between it and the animal;
  - the animal's middle was always in the picture, and it filled 40% at the median (5–95%: 32–50%);
  - no NaN;
  - `follow.update` took 0.08 ms a frame;
  - a comment during the follow started its turn at once, stopped the follow, and claimed and cut the camera.
- **Headless Edge on the GPU** (port 8141, frames driven one by one; pictures in this session's scratchpad, `follow/`):
  - it followed after 30 s of quiet;
  - four angles on the bear and three on a bird by day, and two of each at night, with the Kuwahara filter on;
  - a test comment from the Story tab's route (`/__chat/say`, as the owner) cut the camera to the host;
  - after 30 s more of quiet it followed a deer;
  - a drag during the follow kept the camera with the viewer: it moved 0.27 m in 8 s while the follow's shot moved 13.6 m;
  - the next comment took the camera back exactly;
  - no errors on the page.

**Left for you**

- These are choices to adjust: the angles, the shot lengths (6–9 s), the framing (36–46% of the picture), and cuts rather than glides.
- The frog seen from high and the wolf from behind sometimes fill under 30% of the picture, a small share of the frames.
- The fish is not one of the animals it follows.
- 16 knowledge nodes need summaries (`bin\knowledge.exe sync`).
