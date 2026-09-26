After: 02-lake-day-night.md

# A mud pit, scattered over the lake's land

The user asked (2026-09-25): "create mud pit, and add randomly to lake environtment". That line is the user's; the rest of this file was written by Claude from it. Choices marked **(default)** are decided, not open questions: build them as written unless the user has changed this file.

Read `docs/3d_modelling/rules.md` first, especially Objects, Environments and Colour and theme.

## The mud pit

An object: `src/figures/objects/MudPit.ts` with its spec `MudPit.md` beside it. The spec is a draft written from the code that says so at its top, as the other objects' are, and names no behaviour. The figure tree then shows it under Objects, as `mud-pit`.

- **What it is (default):** a shallow hollow of wet mud, 1.5–4 m across, with an uneven outline, not a circle.
  - The middle is dark, wet mud, smoother, with one or two shallow puddles of standing water.
  - Toward its edge the mud is drier and paler, cracked like the lake's dried soil.
  - A low, trodden rim is pushed up around it.
  - Nothing else: no footprints, bubbles or sticks (Building shapes rule 7).
- **It never goes below the ground (default).** It reads as a hollow by its raised rim, with its mud at about the ground's level. A real hole would need the ground cut open, and at the lake the water is a sheet under the land that shows in any hollow dug below it (Environments rule 15).
- **Sized** like the other objects, with a seed: `new MudPit({ theme, seed, width, depth })` in meters, with a default size (Objects rule 1). It has no height of its own beyond its rim.
- **Its surface follows the ground:** it can take a height function (the lake's `groundHeight`), so its rim and mud lie on uneven land instead of floating over or sinking into it.
- **In the lake's own style, not faceted (default).** It is part of the lake's ground, like the bank. Reuse the soil texture for the drier, cracked mud (`src/textures/soil.ts`; Building shapes rule 12).
- **Colours come from the theme:**
  - the mud is the bare land's colour (`scene.floor`), darker and wetter toward the middle;
  - the puddles are in the water's colour (`scene.water`), with a glint like the lake's.

  Add a role only if that can't be made to read (Colour and theme rule 2).
- **Preview:** its entry in `src/previews.ts`, on the floor (the default for figures not listed in `figureEnvironments`). The lawn's 2–4 cm blades would grow through it.

## At the lake

- **About 5 pits (default),** placed by their own seeded generator, so the lake is the same on every load.
  - Place them after everything else, so the trees, stones and boulders keep their spots (Environments rules 13 and 18).
  - Check the spots of those three before and after, in numbers.
- **Where:**
  - on the land past the top of the bank, mostly 3–15 m from the water's edge;
  - only where the land is nearly flat;
  - at least one where the lake's preview cameras see it.
- **Keep them clear of:**
  - `Lake.LANDING`'s clearing (`lake.clearing`, which the lake meeting widens with `LakeOptions.clear`);
  - trunks and boulders, by 2 m (`boulders.near()`);
  - the lakeside stones;
  - each other.
- **The grass keeps off them,** as it does off the boulders: no sward tuft or meadow plant grows in a pit.
- **Keep the lake's build time:** it builds in about 0.62 s (Environments rule 13). Measure before and after, and keep the pits to a few milliseconds.
- **Night:** after task 02 the lake has day and night. Check the pits and their puddles in both.

`Lake.md` is the user's spec: don't edit it. Other sessions are changing the lake's files, `src/previews.ts` and `src/theme.ts`. Change only the lines you need, and never restore any of them from a copy.

## Checks

- `npm run typecheck` in `typescripts/animation`.
- Screenshots with headless Edge on a spare port, never 8087 (rules.md, Checking the result):
  - the mud pit's own preview, close enough to see its parts;
  - the lake from its preview camera, by day and at night;
  - close to a pit with the walking camera (`?camera=walk`);
  - all of them with the Kuwahara filter on and off, in `felt` and in `studio`.
- Numeric checks:
  - the trees, stones and boulders are where they were;
  - no grass grows inside a pit;
  - every pit is on nearly flat land, outside the clearing;
  - the lake's build time is measured.
- Add what you learned to rules.md (Objects and Environments), then call `knowledge_sync`.
- Don't commit.

Taken: 2026-09-25 17:46

## Done

**What changed**

- **`src/figures/objects/MudPit.ts` and `MudPit.md`** (new), shown as `mud-pit` under Objects. The spec is a draft written from the code and names no behaviour.
  - **Shape:** a shallow hollow with an uneven outline. The wet middle is dark and smooth, with a sheen and one or two puddles. The drier edge is paler, with the soil texture's grit and cracks. A 7 cm trodden rim runs round it.
  - **Never below the ground:** the mud lies 1.2 cm above the ground, and the outer edge tucks 1.5 cm under it.
  - **Size:** `new MudPit({ theme, seed, width, depth, heightAt })`, built exactly to its footprint (3 × 2.2 m by default). `heightAt` lays it on uneven land.
  - **Look:** in the lake's style, not faceted. Vertex colours come from `scene.floor`, darker where wet; the soil texture shows as on the lake's ground, plus a wet-sheen shader tweak. Puddles are `scene.water` at roughness 0.1. No new colour role.
- **`src/previews.ts`:** a `mud-pit` preview on the floor.
- **`src/environtments/Lake/MudPits.ts`** (new): five pits, 1.5–4 m across, placed by a generator of their own after the trees, stones and boulders.
  - **Where:** past the top of the bank, 3–15 m from the water. Only on nearly flat land (≤ 6 cm of rise per meter, 3.4°), outside the clearing (also the meeting's wider one), 2 m from trunks and boulders, clear of stones and of each other.
  - **Grass:** `near()` keeps it off them. `Sward` and `Meadow` now take anything with `near()` (`Pick<Boulders, 'near'>`), and `Lake.ts` passes one that checks both the boulders and the pits. The pits are built after the trees and before the grass.
- **Docs:** `rules.md` Objects rule 8 (the pit) and Environments rule 24 (the lake's pits). Knowledge synced.

**How it was checked**

- `npm run typecheck` passes.
- **Numeric** (Node):
  - Every tree, stone and boulder is exactly where it was, in the default lake and in the meeting's lake (11, 75 and 4, compared number for number).
  - No grass inside a pit's rim: 0 of about 17 k tufts and plants. The sward lost about 300 tufts to the pits, the meadow 4.
  - Every pit rises 4.0–6.1 cm/m and is 4.6–14.6 m from the water. The nearest pit's edge is 10 m from the landing, well outside its 2.2 m clearing.
  - The five pits take 8–12 ms to place and build (about 1.3 ms each). The lake built in 0.63–0.69 s before and 0.65–0.72 s after, measured while the machine was quiet. Later runs, while other sessions loaded it, took 0.85–0.96 s.
- **Screenshots** (headless Edge on the GPU, port 8131), felt and studio, filter off and on:
  - The pit's own preview.
  - The lake from the penguin's camera, by day and at night.
  - The fox's lake view.
  - The walking camera beside the west pit, by day and at night. On the lawn it reads as bare mud with a dark wet middle and a puddle. In studio it shows mostly by its dark middle, since that theme's floor and lawn are nearly the same grey.

**Left for you**

- **"At least one where the lake's preview cameras see it" is not really met.**
  - The first two pits are placed inside the frames of the penguin's lake camera (west shore) and the animals' (east shore). Both are about 20 m off across the water, and at that distance a flat pit seen from under 2 m up is a sliver that the grass and the land's rise hide; the screenshots show neither.
  - Nearer, those cameras see only the clearing, the bank and the water. A scan of every spot near the landing where a small pit would be even partly in the penguin's view found none the other rules allow: the bank ruled out 8.5 k spots and the clearing 1.2 k.
  - To see one now, walk the camera (`?camera=walk`) toward the west shore.
  - If you want one in the default view, tell me which rule to relax: a pit on the bank beside the clearing, one just outside the clearing behind the figure, or pits with a taller or darker rim so they read from 20 m.
- **Flatness:** I allowed 6 cm of rise per meter (3.4°) as "nearly flat". At 4 cm almost no land near the water qualified.
- **Night:** the puddles dull only through the moonlight; they have no night roughness of their own, as the lake's water now does.
