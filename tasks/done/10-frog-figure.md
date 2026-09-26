# A frog, among the animals and at the lake meeting

The user asked (2026-09-25): "add figure animal frog to, in lake_meeting" (a frog figure among the animals, and in the lake meeting too). That line is the user's; the rest of this file was written by Claude from it. Choices marked **(default)** are decided, not open questions: build them as written unless the user has changed this file.

Read `docs/3d_modelling/rules.md` first, especially Figure specs, Animals and Colour and theme. Look at how the bird and the snake (their own bodies on `Animal`) and the bear (the faceted look, Animals rule 13) are built.

## The frog

`src/figures/animals/Frog.ts` with its spec `Frog.md` beside it, plus part files if it needs them. It then shows under Animals in the figure tree.

- **What it is (default):** a common pond frog at real size, about 9 cm long sitting and 5 cm tall (rules.md, Units).
  - a squat body with no neck and no tail;
  - a broad, flat head with a wide mouth;
  - big round eyes high on the head;
  - short front legs;
  - long hind legs folded in a zigzag at its sides, with webbed feet.
- **Low poly (default),** like the bear and the firefly, the user's latest figures: the animals' faceted look (`createMaterials(theme, 'faceted')`, `blob()` and `solid()` with `m.look`). The eyes are a few faces too, as the bear's are.
- **Its own body on `Animal` (default),** as the bird and the snake have. A frog hops rather than walks, and the four-legged rig's walk and gallop don't fit it.
- **Colours from the theme:**
  - the skin olive green, mixed from `fur` toward `grass` as the snake's is (`mix(p.fur, p.grass, …)`);
  - its belly and throat paler, toward `light`;
  - a few darker blotches on its back, toward `dark`.

  No new role (Colour and theme rule 2).
- **Behaviours:** the animals' own (Animals rule 1): `Jump()`, `Hold(figure)`, `Walk()`, `Run()` and `Speech(text)`, plus `Stop()`. For a frog **(default)**:
  - `Walk()` is a string of small hops, and `Run()` is long, quick hops. It moves itself forward at `speed` either way (Moving parts rule 7). A planted frog doesn't slide: between hops it sits still on the ground.
  - `Jump()` is one high leap, the hind legs flung out straight and folded again on landing.
  - `Hold(figure)` carries it in its mouth, scaled down to about its own length (Animals rule 8).
  - `Speech(text)` shows the bubble. Set `bubbleScale` so its words read in its own preview, as the bird does (Animals rule 9).
- **Spec:** write `Frog.md` in the user's layout (Figure specs rule 6) with these behaviours, saying at its top that it is a draft for the user to correct.
- **Preview:** `frog: animal(Frog, 'frog', <radius>)` in `src/previews.ts`, on the floor, with a circle sized to it.

## At the lake meeting

- **A new kind:** add `frog` to `KINDS` in `src/story/lake_meeting/events.ts` and to the figures in `kinds.ts`. It can then be:
  - a viewer's random animal;
  - a bot;
  - picked with `!frog`, as `!cat` and the others are.
- **Update the spec:** item 3 of the draft `lake_meeting.md` (the kinds and their `!` commands). Keep its note at the top that it is a draft for the user to correct.
- **Whatever the meeting does now, the frog does too:** today the animals stand in rows. If task 07 (the animals roam the land) is done or in progress, the frog roams like the others. It keeps out of the water, as they do.
- The camera's turns frame it by its size, as they do the bird (`READABLE`). Check that its bubble reads in the stream's 720p picture.

Tasks 07–09 change the meeting too (`Meeting.ts`, `Member.ts`, `lake_meeting.md`), and other sessions are changing `src/previews.ts`. Change only the lines you need, and never restore any of them from a copy.

## Checks

- `npm run typecheck` in `typescripts/animation`.
- **Numeric checks,** in Node over its demo:
  - its box is about 9 cm long;
  - between hops it sits still, its feet not sliding;
  - the demo's circle closes loop after loop;
  - nothing is NaN.
- **Screenshots** with headless Edge on a spare port, never 8087 (rules.md, Checking the result):
  - the frog's own preview, close up, mid-hop and sitting, with the Kuwahara filter on and off, in `felt` and `studio`;
  - the lake meeting with a frog: send `!frog` as a test message from the Story tab (spec item 9), and have it say something.
- Add what you learned to rules.md (Animals and Stories), then call `knowledge_sync`.
- Don't commit.

Taken: 2026-09-25 18:37

## Done

**What changed**

- **`src/figures/animals/Frog.ts` and `Frog.md`** (new): shown under Animals as `frog`. The spec is a draft in the user's layout, items 2–7 (Jump, Hold, Walk, Run, Speech, Stop).
  - **Shape:** a low poly pond frog, 8.6 cm long sitting, 7.6 cm across its legs and 6 cm to the top of its eyes. It is its own body on `Animal`: a squat head-up body, a broad flat head with a dark mouth line, big faceted eyes, short front legs with flat hands, and long zigzag hind legs with webbed feet and a ball at each joint.
  - **Colours:** olive skin (`fur` toward `grass`), a paler belly and throat (toward `light`), dark blotches on the back (toward `dark`). No new role.
  - **Behaviours:**
    - `Walk()` is small hops (8 cm/s) and `Run()` long, quick hops (34 cm/s).
    - Between hops it sits exactly still, whatever the preview's steering does.
    - `Jump()` is one high 12 cm leap, the hind legs flung out straight and folded again.
    - `Hold()` carries the figure in its mouth, up to 9 cm.
    - Its bubble is 0.5 of the penguin's.
- **`src/previews.ts`:** `frog: animal(Frog, 'frog', 0.2)`, on the floor.
- **Lake meeting:**
  - `frog` is added to `KINDS` in `events.ts` and to the figures in `kinds.ts`. It can be a viewer's random animal or a bot, and `!frog` works. The dev server's `meeting.ts` reads the same `KINDS`.
  - Item 3 of `lake_meeting.md` names the frog and `!frog`; its draft note is kept.
  - I changed only those lines, since task 07 is changing the meeting in another session.
- **`rules.md`:** Animals rule 14 (the frog), the Animals list, and a note in Stories rule 11. Knowledge synced.

**How it was checked**

- `npm run typecheck` passed for this work; at the end, `src/story/lake_meeting/Roam.ts` (task 07, in progress in another session) showed one error of its own (an unused `velocity`).
- **Numeric** (Node, 120 s of its demo):
  - The box is 75.5 × 60.8 × 85.5 mm. At rest it is bedded 1.8 mm into the ground; at the lowest point in the whole demo, 2.1 mm (landing a running hop).
  - Over 1,484 sitting frames, a planted foot moved 0.0000 mm.
  - The demo's circle stays put (each loop stops at 0.995 and 0.9995 of the radius).
  - Nothing is NaN.
- **Screenshots** (headless Edge, port 8131): sitting and mid-leap close up, and hops from the preview's camera, in felt and studio, with the filter off and on.
- **Lake meeting** (GPU, 1280×720, test messages through `/__chat/say`): a viewer said "Halo!", then `!frog`, then "Kwak kwak! Aku katak dari danau.". The meeting listed Kodok as a frog, and the close-up showed the frog with its named bubble readable, at about 20 px text.
- **First try:**
  - With the bubble at 0.2, the words were about 8 px high, because the meeting's close-up comes no nearer than 0.6 m. Hence 0.5, the bird's.
  - The first legs went 13 mm (the hands) and 39 mm (flung out) into the ground, and the crouch pushed the feet in. The legs were reshaped and the crouch dropped; see rules.md.

**Left for you**

- **No crouch:** the frog has none before a leap. Its legs are rigid segments, so a crouch pushed them into the ground.
- **My choices:** its size, speeds, colours and bubble size, and the mouth line (a thin dark line, for "a wide mouth").
- **Roaming:** whatever the meeting does, the frog does too, since it is an ordinary kind there. I haven't checked it roaming, because task 07 is still in another session's hands.
