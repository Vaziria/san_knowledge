After: 02-lake-day-night.md

# Move the firefly to the objects: it can't talk

The user asked (2026-09-25): "move firefly to object, it can't talk". That line is the user's; the rest of this file was written by Claude from it. Choices marked **(default)** are decided, not open questions: build them as written unless the user has changed this file.

The firefly was built as an animal in task 01 (`tasks/done/01-firefly-figure.md`). Task 02 (`tasks/done/02-lake-day-night.md` once it is done) puts a swarm of them over the lake at night. Read both first, and rules.md (Animals, Objects).

## What changes

- **It becomes an object.** Move `Firefly.ts` and `Firefly.md` from `src/figures/animals/` to `src/figures/objects/`, and any part files it has. The figure tree places a figure by its spec's path, so it then shows under Objects. Its key stays `firefly`, so `?figure=firefly` still works.
- **It can't talk.** It no longer extends `Animal`, so it has no `Speech(text)` and no speech bubble.
- **No behaviours at all (default).** The objects' specs name no behaviour (rules.md, Objects), so it also drops `Jump()`, `Hold(figure)`, `Walk()`, `Run()` and `Stop()`. The Behaviours tab then says it has none.
- **What stays:**
  - its idle motion: hovering, bobbing and swaying, the wing beat and the lantern's flash;
  - its real size (2 cm), its faceted look, its parts and the `glow` role;
  - the options task 02 gave it for the swarm (its own flash timing, its speed, no light).

  Objects rule 1 (sized in three directions with `fit()`) was for the cliff, the cloud and the fog, which the user asked to size. The firefly keeps its real size.
- **The lake's fireflies must keep working and look the same.** If task 02 flies them with `Walk()` and steering, give the object a plain way to fly: a speed it flies forward at along its +z, leaning into it, turned by `rotation.y`. Otherwise have the swarm move them itself. It is not a behaviour for the panel.
- **The preview:** replace `firefly: animal(Firefly, 'firefly', 0.03)` in `src/previews.ts` with an object preview like the others'. It shows the firefly hovering in place, close enough to see it (task 01's framing). **Keep it on the floor (default),** not on the lawn like the big objects: a 2 cm firefly hides among the lawn's 2–4 cm blades.
- **The spec:** move `Firefly.md` along with it and take its behaviours out. Keep the note at its top that it is a draft for the user to correct.
- **rules.md:**
  - take the firefly out of the Animals list and add it to the Objects list;
  - move its rule (Animals rule 14) to Objects;
  - add anything the move teaches.
- **Everything else that uses it:** search for `Firefly` (the lake's swarm, previews, stories) and update the imports. The lake meeting's animals are the viewers' kinds: the firefly must not become one.

Other sessions are changing `src/previews.ts` and the lake's files. Change only the lines you need, and never restore any of them from a copy.

## Checks

- `npm run typecheck` in `typescripts/animation`.
- Screenshots with headless Edge on a spare port, never 8087 (rules.md, Checking the result):
  - the firefly's own preview, close up, with the Kuwahara filter on and off;
  - the lake at night with its fireflies, compared with task 02's screenshots.
- The panel: the firefly is under Objects in the figure tree, and the Behaviours tab says it has none.
- Numeric checks: its box is still about 2 cm long, it still flashes about every 2.5 s, and nothing is NaN.
- Add what you learned to rules.md, then call `knowledge_sync`.
- Don't commit.

Taken: 2026-09-25 17:39

## Done

**What changed**

- **Files moved:** `Firefly.ts` and `Firefly.md` are now in `src/figures/objects/` (plain `mv`; both were new and untracked). There are no part files. The figure tree now shows the firefly under Objects, and its key is still `firefly`.
- **`Firefly.ts` is a plain `THREE.Group`**, no longer an `Animal`:
  - No `Speech`, bubble, `Jump`, `Hold`, `Walk`, `Run` or `Stop`, and no holder.
  - It keeps its idle motion (`update(delta)`: the hover, bob, sway, wing beat and flash), its real size, its faceted parts built from `animals/parts.ts`, and the `glow` role.
  - It keeps task 02's options (`seed`, `light: false`, `lit`). The walk/run speeds option became one plain `speed` in m/s, an option and a property: it flies forward along +z, eased in and out, leaning into it, and `rotation.y` steers it.
- **The lake's swarm** (`Lake/Fireflies.ts`) now imports the object and passes `speed` instead of calling `Walk()`.
- **`src/previews.ts`:**
  - The `animal(Firefly, …)` entry is gone. A new object preview after `log` shows the firefly hovering in place, close up (8 cm, three-quarters from the front). It stays on the floor.
  - The import moved, and the comment in `animal()` about lifting the view no longer names an animal.
- **Spec:** `Firefly.md` has its behaviours taken out, in the objects' layout ("No animation behaviour…"), and still says at the top that it is a draft for you.
- **`rules.md`:**
  - The firefly is out of the Animals list and its cost line.
  - Animals rule 14 became Objects rule 7. Hold and the demo framing are gone, and it adds the move's lesson: an animal's body code moves over as it is; only its `Animal` bookkeeping (the gait easing and the forward step) has to come along.
  - Objects rule 6 now says the firefly starts on the floor, and Environments rule 23 points to Objects rule 7 and `speed`.
  - Knowledge synced: `animals/Firefly.md` removed, `objects/Firefly.md` added.

**How it was checked**

- `npm run typecheck` passes.
- **The lake's fireflies are exactly as before.** Every mesh's world matrix was recorded through 20 s of the swarm at night and 10 s of a lone hovering firefly, before and after the move. The largest difference is 0 over 240 k numbers, lantern glows included.
- **Numeric:**
  - The box is 10 × 9.4 × 22.3 mm, lowest point 19.2 mm up.
  - It flashes every 2.512 s, first at 0.65 s.
  - It has none of the old behaviour methods, and the preview lists no behaviours.
  - Over 30 s the preview's firefly doesn't move from its spot, and nothing is NaN.
- **Panel** (headless Edge through the remote-debugging protocol): the figure tree lists Objects → cliff, cloud, fog, log, firefly (picked). The Behaviours tab says "firefly has no behaviours in its spec."
- **Screenshots** (port 8131):
  - The preview close up, filter off and on: it hovers and flashes, lighting its pool on the floor.
  - The lake at night: it looks as in task 02. Two runs of the same code differ in about 0.1% of pixels, because the page's clock starts at a slightly different moment each load, and task 02's shot predates its steering fix, so the bit-exact Node run is the real comparison.
- The lake meeting's kinds are an explicit list, so the firefly was never one.

**Left for you**

- The firefly's spec is still a draft for you to correct.
- The `animal()` preview keeps its lifted view for an animal off the ground. No animal uses it now; it changes nothing for the others.
