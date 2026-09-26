# Objects state

Updated 2026-09-26 by the fish made three kinds from the user's low poly models (asked for directly, no task file: the Fish line); earlier that day when felt became the only theme (two notes about the studio theme went); before that by task 11 (the floating logs) and task 09 (the dock); first written by task 17, from the code as it was then.

## What works

**What the objects share** ([rules.md, Objects](../../../../docs/3d_modelling/rules.md#objects)):
- The cliff, cloud, fog, log, mud pit and firefly live in `src/figures/objects/`, each next to a draft spec written from the code. The specs name no behaviour, so the Behaviours tab has none for them.
- Each is built from a seed and sized in meters, `new Cliff({ theme, seed, width, height, depth })`, with a default size. The cliff, cloud and log are built near that size and stretched to it exactly (`fit()` in [parts.ts](../../src/figures/objects/parts.ts)); the fog keeps its puffs inside its box; the mud pit is built to its footprint.
- The cliff, cloud and fog start on the lawn, the dock at the lake, and the log, mud pit and firefly on the floor (`figureEnvironments` in [previews.ts](../../src/previews.ts)).

**Each object:**
- [Cliff](../../src/figures/objects/Cliff.md) ([Cliff.ts](../../src/figures/objects/Cliff.ts)): 10 × 6 × 4 m of layered rock with ledges, overhangs, cracks and fallen boulders, in `scene.stone`. The lake's border is built from cliffs ([environments.md](environments.md)).
- [Cloud](../../src/figures/objects/Cloud.md) ([Cloud.ts](../../src/figures/objects/Cloud.ts)): an 8 × 4 × 5 m heap of lumpy clumps, flat below, `light` shading greyer underneath. It doesn't float by itself: its preview lifts it 3 m. The lake's sky uses it (environments.md).
- [Fog](../../src/figures/objects/Fog.md) ([Fog.ts](../../src/figures/objects/Fog.ts)): 8 × 1.5 × 5 m of soft puffs facing the camera, one instanced mesh. It drifts and swirls on its own (`update(delta)`) and loops forever. The border's mist reuses its puff material (`puffMaterial`).
- [Log](../../src/figures/objects/Log.md) ([Log.ts](../../src/figures/objects/Log.ts)): a 2.4 × 0.48 × 0.52 m trunk along x in bark (`bark(theme)`), with sawn-off stubs, sawn ends showing the end grain, bedded and cut flat underneath, as its preview shows it. A `round` log is neither bedded nor cut flat, so that it can float: the lake meeting's four ([lake_meeting.md](lake_meeting.md)). `axis` and `radius` say where its trunk is.
- [Mud pit](../../src/figures/objects/MudPit.md) ([MudPit.ts](../../src/figures/objects/MudPit.ts)): 3 × 2.2 m by default, a dark wet middle with one or two puddles, a paler cracked edge (the soil texture) and a 7 cm rim. It never goes below the ground, and `heightAt` lays it on uneven land. Five lie about the lake's land (environments.md).
- [Firefly](../../src/figures/objects/Firefly.md) ([Firefly.ts](../../src/figures/objects/Firefly.ts)): 2 cm, faceted from the animals' parts. It hovers 2.2 cm up, bobbing and swaying, its four wings beating together, and its lantern flashes about every 2.5 s in the `glow` role, with a small point light. It has no speech and no behaviours. `speed` flies it forward and `rotation.y` steers it (the lake's night swarm); a `seed`, `light: false` and a `lit` level suit a swarm.
- [Dock](../../src/figures/objects/Dock.md) ([Dock.ts](../../src/figures/objects/Dock.ts)): a village jetty 3 × 6.5 m, its deck 0.36 m over the water. It is 768–840 triangles in two meshes, wood and bark.
  - **Its parts:** planks across three beams with narrow gaps, cross-beams on pairs of log posts in bark about every 2 m, and a 1.2 m ramp down to its `foot` at the landward end.
  - **Where its posts stand:** on `bedAt`, the lakebed or the bank; a cross-beam that comes down to the bank rests on it and has no posts.
  - **Built:** from a seed, and a little uneven.
  - **Its preview:** at the lake, where the lake meeting has it (`Lake.siteDock`, [DockSite.ts](../../src/environtments/Lake/DockSite.ts)), seen from over the water.

**Boat, fish and fishing rod**, each in its own folder with its parts as readonly fields ([rules.md, Building shapes](../../../../docs/3d_modelling/rules.md#building-shapes) rules 8–13):
- [Boat](../../src/figures/Boat/Boat.md) ([Boat.ts](../../src/figures/Boat/Boat.ts)): a 2.4 m wooden dinghy, only a hull and a rudder (the user had the sail and the mast taken off), planks with wood grain, the inside kept free of the lake's water (`keepDry`). On the lake its preview floats it, easing its height, pitch and roll toward the waves under it; elsewhere it rests on its keel.
- [Fish](../../src/figures/Fish/Fish.md) ([Fish.ts](../../src/figures/Fish/Fish.ts), [models.ts](../../src/figures/Fish/models.ts); its shapes in [animals/Fish.md](../../src/figures/animals/Fish.md)): three kinds, each the user's own low poly model, `new Fish({ theme, kind })`: the **salmon** (29.6 cm, the default: the lake meeting's fish and the one the animals hold), the **piranha** (25 cm, a deep body, a red belly and an underbite with two rows of teeth) and the **clownfish** (10 cm, the user's "Nemo": orange with three white bands edged in dark). At rest each is its model to 0.00001 mm; swimming, its body bends along a wave as its model sways it, the rings built again every frame and the fins bending with them. `SwimOnSurface(direction)`, `SwimOnDepth(direction)` and `JumpOutFromWater(height)` as the spec names them, plus `Stop()`. It moves itself, its tail beat following its speed. A leap is worked out when it starts, clears the water by exactly `height` (0 to 2 m), and sends a `splash` event where it leaves the water and where it comes back in, which the lake turns into a splash ([rules.md, Moving parts](../../../../docs/3d_modelling/rules.md#moving-parts) rules 10–12). Each kind's preview (`salmon`, `piranha`, `clownfish`, in the Figures tree's Fish folder) swims a square with a leap in it, the salmon's scaled to its size; distances and speeds follow its size too. `?figure=fish` is gone (it shows the default figure). The meeting's fish is in [lake_meeting.md](lake_meeting.md).
- [Fishing rod](../../src/figures/FishingRod/FishingRod.md) ([FishingRod.ts](../../src/figures/FishingRod/FishingRod.ts)): a 2 m branch in bark with a snapped twig, a leafy twig and a leaf. An 8 mm line is tied below the tip, kept sharp through the filter and turned straight down every frame (`update()`). Its origin is where the hand holds it, 30 cm from the butt. The penguin and the animals can hold it.

## How it was last checked

- 2026-09-26, the fish's three kinds (no task file): typecheck. **In Node:** each kind against its page's own code, every triangle and colour at rest to 0.00001 mm and every closed face facing out; five leaps each (from the surface and from depth, 0.15 m to 1 m) at 240 and 20 fps, the lowest point at the top exactly the height asked, level there, smooth, two splashes each where the middle crosses the surface; each preview's square stays in place over 12 loops; the lake meeting run 7 × 30 min with a supporter's leap every 3 min, its own leaps and its way back to its loop as the old fish's. **Headless Edge on the GPU (port 8175):** each kind side on and three-quarters, filter off and on, beside its page; swimming, mid-leap, turning; the piranha's teeth and the clownfish's bands close up; the bear, the cat and the penguin holding a salmon; the meeting's leaps, a test Super Chat's to 0.8 m.
- 2026-09-26, [task 11](../../../../tasks/done/11-lake-meeting-logs.md), the round log: typecheck.
  - **In Node:** the preview's log, and seven others of other seeds and sizes, hashed the same before and after, every attribute bit for bit. A round log has 1 bark vertex at its very bottom, where the bedded one has 590 on its flat underside.
  - **Headless Edge on the GPU (port 8142):** floating logs close up at the waterline in felt and studio, filter off and on, by day and at night; one end on, its end grain showing and its round underside through the water. The log's own preview wasn't photographed: its geometry is the same to the bit.
- 2026-09-25, [task 09](../../../../tasks/done/09-lake-meeting-dock.md), the dock: typecheck.
  - **Headless Edge on the GPU (port 8141):** its preview in felt and studio, filter off and on, by day and at night. The walking camera on its deck stood 1.6 m over it.
  - **In Node:** its site at the meeting's lake, and animals walking out on it ([lake_meeting.md](lake_meeting.md)).
- 2026-09-25, task 17: `npm run typecheck` passes on today's working tree, the half-built code of tasks 09 and 16 included.
- 2026-09-25, [task 13](../../../../tasks/done/13-lake-meeting-fish-leaps.md): the fish's leap caught in the air in headless Edge on the meeting's page. In Node, four 30 min runs: its own leaps 0.15–0.28 m, nothing NaN.
- 2026-09-25, [task 04](../../../../tasks/done/04-mud-pit.md), the mud pit: in Node the lake's trees, stones and boulders stayed where they were, no grass grew in a pit, the pits rise 4.0–6.1 cm a meter, and five build in 8–12 ms. Screenshots in headless Edge (port 8131), felt and studio, filter off and on: its preview, the lake by day and at night, the walking camera by a pit.
- 2026-09-25, [task 03](../../../../tasks/done/03-firefly-to-objects.md), the firefly as an object: the lake swarm's world matrices matched before and after the move (240 k numbers). Its box is 10 × 9.4 × 22.3 mm and it flashes every 2.512 s. The panel lists it under Objects and says it has no behaviours. Screenshots close up, filter off and on.
- 2026-09-25, [task 01](../../../../tasks/done/01-firefly-figure.md): the stall the first time a firefly is shown, measured in headless Edge on the RTX 3050.
- Before the task queue (commits ca1bf63 and 278ad67, and the log on 2026-09-25), rules.md records the checks, undated: each object's box against its size, to the centimeter (Objects rule 1); the boat's bench corners raycast against the hull (Building shapes rule 10); the fish's leap height and its smoothness at 240 and 20 fps (Moving parts rule 12).

## Known issues and left for the user

- **Firefly** (task 01): its thorax is `trim`, felt's orange. The first time one is shown the page stalls 0.37 s on the floor and 1.8 s on the lake (fresh shader cache), because a new light recompiles every material; after that it costs what any figure does. Its legs and antennae are about three times real thickness, so they survive the filter.
- **Mud pit** (task 04): none shows in the lake's default views. From 20 m off a flat pit is hidden by the grass and the land's rise; walk the camera (`?camera=walk`) toward the west shore to see one. "Nearly flat" allows 6 cm of rise a meter. The puddles have no night look of their own.
- **Fish, as its models have them:** the salmon's pink stripe never shows (with eight corners no face lies in its band; one threshold to show it), the body wave runs toward the head (a real fish's runs toward the tail; one sign), and part of each pectoral stroke takes the fin inside the body. Felt has no red, so the piranha's reds are rust. The piranha's teeth read only close up and the clownfish's thin dark edges only in a large window; the filter takes them at the preview's distance. The lake's splash drops are sized for the salmon, so the clownfish's leap throws drops as big as itself (`Splash.ts`, not changed). Bending the body costs about 40 µs a frame (the smooth fish 1.8): fine for one fish, not for a school.
- **Fish:** a leap's length follows from its height, and the fish knows nothing of the shore. At the meeting a supporter's leap, started anywhere on its loop, can land it on the lake bed by the shore (task 13; [lake_meeting.md](lake_meeting.md)).
- **The boat and the rod do nothing of their own:** the rudder turns on the transom but nothing turns it, and the rod's preview holds it still 1 m up, as if by an unseen hand.
- **Not committed:** the firefly, the mud pit and the log are new files only in the working tree, and so is the fog's `puffMaterial` change.
- The header of `objects/parts.ts` still says it is for the cliff, the cloud and the fog; the log uses `fit()` and the terrains its `wobble()` too.

## Open questions

- Every object's spec (Cliff, Cloud, Fog, Log, MudPit, Firefly) is a draft written from the code, not yet confirmed by the user. Log.md now names the round, floating log too (task 11).
- [Boat.md](../../src/figures/Boat/Boat.md) and [FishingRod.md](../../src/figures/FishingRod/FishingRod.md) are the user's and hold only a title: any behaviour waits on them.
- Firefly (task 01): both sides' wings beat together, as an insect's do, not one side against the other as in the reference. Its lean and wing beat in flight were Claude's choices.
- Mud pit (task 04): which rule to relax so one shows in the default view: a pit on the bank beside the clearing, one just outside it behind the figure, or a taller or darker rim.
- Fish: the values of `Direction` (forward +z, back −z, left −x, right +x) are Claude's reading of the spec, and `Stop()` is beyond it. Should the salmon's pink stripe show, the wave run toward the tail, the pectorals stay out of the body, and the piranha get a red of its own?
- The dock's spec, `Dock.md`, is a draft written from the code. Its 3 m width, wider than a village jetty so that the meeting's biggest animals fit on it, and its ramp are task 09's choices.

## History

- 2026-09-26, no task file, not committed: the fish rebuilt in three kinds from the user's two model pages (the user's choice: "Three kinds of the fish ... The lake and the Hold menu use the salmon."), its motion kept; the `fish` preview became `salmon`, `piranha` and `clownfish`, in a Fish folder of the Figures tree (`SPECS` in `figureTree.ts`).
- 2026-09-26, asked for directly ("just keep felt, we dont need other", no task file): felt is the only theme ([control_panel.md](control_panel.md)). No object changed; only this file's notes about studio went.
- 2026-09-26 [task 11, floating logs](../../../../tasks/done/11-lake-meeting-logs.md): the log's `round` option, and its `axis` and `radius`; `fit()` returns the stretch it made. The bedded log is unchanged.
- 2026-09-25 [task 09, dock](../../../../tasks/done/09-lake-meeting-dock.md): the dock, with its preview at the lake.
- 2026-09-25 [task 13, fish leaps](../../../../tasks/done/13-lake-meeting-fish-leaps.md): the meeting's fish leaps now and then on its own; `Fish.ts` unchanged ([lake_meeting.md](lake_meeting.md)).
- 2026-09-25 [task 04, mud pit](../../../../tasks/done/04-mud-pit.md): the mud pit; five at the lake.
- 2026-09-25 [task 03, firefly to objects](../../../../tasks/done/03-firefly-to-objects.md): the firefly became an object, with no speech or behaviours and a plain `speed`.
- 2026-09-25 [task 02, lake day and night](../../../../tasks/done/02-lake-day-night.md): firefly options for the lake's night swarm (`seed`, `light: false`, `lit`, `flash`, `Firefly.HOVER`).
- 2026-09-25 [task 01, firefly](../../../../tasks/done/01-firefly-figure.md): the firefly, first as an animal, with the new `glow` colour role; the stage's near plane now follows the camera's distance (`fitNear()`), so a 2 cm figure can be seen close.
- 2026-09-25, no task file, not committed: the log, with the end-grain texture; the fog's puff material opened to the border's mist.
- 2026-09-24 commit 278ad67: the cliff, the cloud and the fog; the fish's `JumpOutFromWater(height)` with its splashes.
- 2026-09-24 commit 9572e26: the user renamed the fish's third behaviour to `JumpOutFromWater(height)` (then in the spec only).
- 2026-09-24 commit ca1bf63: the boat, the fish swimming at the surface and at depth, and the fishing rod.
