# Forest lake animals state

Written 2026-09-27, when the forest lake's animals were made (asked for directly: "create animal figure from this reference, create as is, dont let old style influence this design, use buffer geometry for better result and if needed its placed in lakeforest/animal", with 14 reference sheets; no task file). All thirteen are built: the twelve animals of the sheets' overview, and the explorer, the boy of the first sheet. Updated 2026-09-28: they are the forest lake meeting's cast ("make story ForestLake Metting like lake meeting"; [forest_lake_meeting.md](forest_lake_meeting.md)), with nothing in their own files changed.

## What works

- **Where they are:** `src/figures/ForestLake/animals/`, under ForestLake → Animals in the Figures tree, in the order of the sheets' overview (wolf, deer, fox, boar, rabbit, squirrel, duck, otter, fish, frog, butterfly, firefly), the explorer last. Each has its spec, a draft written from its sheet for the user to correct, its preview in `forestAnimalPreviews` ([demos.ts](../../src/figures/ForestLake/animals/demos.ts), spread into the table in [previews.ts](../../src/previews.ts)), and its parts in `figureParts` ([parts.ts](../../src/parts.ts)), so each part can be shown alone (`?part=`). Each preview plays its sheet's poses in a loop; its Behaviours buttons are the poses, named as the sheet names them, and `SetColor` with the sheet's colour variations.
- **Their own look**, not the theme's and not the older animals': flat-shaded facets in each sheet's own colours, picked from its pictures. Everything is BufferGeometry built in code, with a toolkit of their own:
  - [parts.ts](../../src/figures/ForestLake/animals/parts.ts): `Sculpt` (flat triangles, each with a colour role and bone weights), lofts through rings, tubes along a path, eggs, ears, fur shingles, eyes with glints, flat sheets for fins and wings, materials (coat, gloss, two-sided, see-through, glowing), bones, skinned and rigid meshes, weights along a chain of joints, and easing helpers.
  - [ForestAnimal.ts](../../src/figures/ForestLake/animals/ForestAnimal.ts): the base class. It holds the colour variations and `speed`; walkers, swimmers and fliers move themselves forward along +z, and a preview steers them by turning them.
  - [FourLegged.ts](../../src/figures/ForestLake/animals/FourLegged.ts): the four-legged rig. A skinned body on a spine (pelvis, chest, neck, head), a tail, and four legs of four bones each, placed by two-bone IK onto planted feet. A walk and a run per animal (stride, cadence, duty, phase offsets, bob), postures eased toward (sitting, rearing, neck and head, jaw, tail), a leap, and hooks for an animal's own poses, feet and moves.
  - Each animal is traced from its sheet's side and front views in the sheet's own pixels (`U` m per pixel), at its real size.
- **The four-legged seven**, each on `FourLegged`:
  - [Wolf](../../src/figures/ForestLake/animals/ForestWolf.md) (`forest-wolf`): grey, 78 cm at the shoulder, 1.5 m long; `gray`, `white`, `brown`, `black`, `snow`. `Howl()` sits on its haunches and howls. Its demo circle is 1.6 m.
  - [Fox](../../src/figures/ForestLake/animals/ForestFox.md) (`forest-fox`): orange, 39 cm at the shoulder, 82 cm to its tail tip; `orange`, `white`, `brown`, `black`. `Sit()`, `LookUp()`.
  - [Deer](../../src/figures/ForestLake/animals/ForestDeer.md) (`forest-deer`): a stag, 1 m at the shoulder, 1.87 m to its antler tips. Its antlers make a lyre from the front, and its ears are broad forward-facing leaves; `brown`, `winter`, `autumn`, `dark`, `snow`. `LookBack()`.
  - [Wild boar](../../src/figures/ForestLake/animals/WildBoar.md) (`wild-boar`): 85 cm at the shoulder, 1.38 m long, with bristles and tusks; `brown`, `dark`, `gray`, `snow`. `Charge()` gallops for three seconds, head down; `Eat()` roots in the ground.
  - [Rabbit](../../src/figures/ForestLake/animals/Rabbit.md) (`rabbit`): sits as the sheet draws it, 39 cm to its ear tips; `brown`, `white`, `gray`, `black`, `spotted`. Its walk and run are hops; it also has `Jump()` and `Eat()`.
  - [Squirrel](../../src/figures/ForestLake/animals/ForestSquirrel.md) (`forest-squirrel`): red, sits up holding an acorn and runs on all fours with it in its mouth; `brown`, `gray`, `red`, `black`, `white`. It has `Jump()`. `Climb()` goes up the trunk 22 cm ahead, turns, comes down head first and lands where it began, facing away. Its demo gives it two bark trunks and climbs each in turn.
  - [Otter](../../src/figures/ForestLake/animals/Otter.md) (`otter`): 97 cm from nose to tail tip, a cream bib from chin to chest, pale whiskers (lines); `brown`, `dark`, `light`, `gray`, `albino`. On land its Idle sits up; `StandUp()` rears on its hind legs, fore paws before its chest, looking about. `Swim()` puts it in the water, back and head out, paddling; `Dive()` arches over, tail up, goes 0.5 m under and comes back up, splashing going in and out; in water Walk and Run swim. With water its demo treads water, rears up, swims a lap and dives on the way; on land it plays the four-legged demo.
- **The firefly** ([ForestFirefly.md](../../src/figures/ForestLake/animals/ForestFirefly.md), `forest-firefly`): 2 cm long, with six skinned legs, four see-through wings and a lantern that pulses with a halo; `yellow`, `green`, `blue`, `purple`, `red`, `white`. `Idle()`, `Hover()`, `Fly()`, `Turn()`, `Land()`. `Land()` comes down on whatever `surfaceAt` gives, each foot on the surface under it. Its demo gives it a leaf to land on (in the environment's scenery, so it goes when the preview does).
- **The fish** ([ForestFish.md](../../src/figures/ForestLake/animals/ForestFish.md), `forest-fish`): a trout, 40 cm long; `trout`, `salmon`, `carp`, `koi`, `blue`. `Idle()`, `Swim()`, `Turn(degrees)`, `OpenMouth()`, `Jump(height)`. In water it swims 15 cm under the surface; its leap splashes the water where it leaves it and where it comes back in. With water its demo runs in it, `place: 'water'`, the camera looking back down its run; without water it lies on the floor and plays the poses that stay in place.
- **The duck** ([Duck.md](../../src/figures/ForestLake/animals/Duck.md), `duck`): a mallard drake, 52 cm from bill to tail, 89 cm across its wings; `mallard`, `white`, `brown`, `mandarin`, `black`. `Idle()`, `Walk()` (a waddle, on planted feet), `Run()`, `Swim()`, `FlapWings()`, `TakeOff()`: a run-up, pattering over water, a climb to 0.9 m, a flight of about 8 m and a landing. On water it floats a third of the way up its body, rides the waves, and splashes as it flaps and lands; there Walk and Run swim. Its demo swims a lap, flaps, takes off and flies round once; on land it waddles and runs first.
- **The frog** ([ForestFrog.md](../../src/figures/ForestLake/animals/ForestFrog.md), `forest-frog`): 8 cm from snout to rump sitting; `green`, `brown`, and the poison dart frogs `blue`, `yellow`, `red`. `Idle()` (its throat pulsing, blinking), `Walk()` (low crawling hops), `Jump()`, `Swim()`. Its hind legs land flat and never go into the ground. In water it floats at the surface, its back out, kicking and splashing; its demo floats, swims half a lap and leaps out and back in.
- **The butterfly** ([Butterfly.md](../../src/figures/ForestLake/animals/Butterfly.md), `butterfly`): a monarch, 10 cm across its wings, hovering 30 cm up; `monarch`, `blue`, `purple`, `yellow`, `white`, `pink`. `Idle()` hovers, its wings beating between the sheet's Flap Up and Flap Down; `Fly()`; `Land(height)` settles on its feet, rests opening and closing its wings, then rises again. Its demo flies a circle and lands on a white daisy (a prop built from the forest lake's own flowers).
- **The explorer** ([Explorer.md](../../src/figures/ForestLake/animals/Explorer.md), `explorer`): the sheet's chibi boy, 1.2 m tall, his head a third of it, on a rig of his own (hips, spine, neck, head; shoulder, elbow and wrist; hip, knee and ankle), rigid faceted parts on it. Messy brown hair, big brown eyes, a green scarf and a cape with the leaf emblem, the cream shirt, belt and pouches, shorts, cuffed boots, the backpack with its bedroll, and a lantern that swings at his side. One colour variation, `default`, the sheet's palette. `Idle()`, `Walk()` and `Run()` (planted feet), `Jump()`, `Interact()` (kneels to touch something low before him), `Wave()`, `Sit()` (on a seat 17 cm high, until told otherwise), `SetExpression(name)` (`happy`, `laugh`, `surprised`, `angry`, `wink`, `smirk`) and `Hold(tool)` (`sword`, `fishing-rod`, `pickaxe`, `axe`, `lantern`, `nothing`). His demo waves, walks and runs a circle, jumps, kneels to a sapling and sits on a log, which spring up and roll in for it; the tools and faces are buttons with menus.
- **For checking:** `forest-animal-shots.html` at the project's root renders a class from its sheet's views (3/4, front, side, back, top), one view or all of them. It can frame one part (`&focus=`, several parts too), run a behaviour first (`&do=`, `&at=`), paint a colour variation (`&color=`) and use the forest lake's light (`&light=forest`).
- The shared glow of the forest lake's lamps (`halo()` in [ForestLake/parts.ts](../../src/figures/ForestLake/parts.ts)) is smoothly filtered now; it was blocky close up.
- **At the forest lake meeting** (since 2026-09-28; [forest_lake_meeting.md](forest_lake_meeting.md)), in its [guests.ts](../../src/story/forest_lake_meeting/guests.ts):
  - **Who plays whom:** the wolf, deer, fox, boar, rabbit, squirrel, duck, otter and frog are the viewers' animals, the explorer is the host, and the fish is the trout that leaps for supporters.
  - **How they are used:** each is a small subclass there, with a speech bubble, a hop where it stands (the four-legged rig's leap, straight up), and its sheet's poses as the viewers' tricks (Howl, FlapWings, Sit, LookUp, LookBack, Eat, StandUp, Wave).
  - **Their colours:** picked from each viewer's id, in the season's coat where the sheet has one.
  - **Not at the meeting:** the butterfly and the firefly.

## How it was last checked

- 2026-09-27, on each four-legged animal, after the last change to the rig (a scratchpad script through Vite's module loader, 120 frames a second, each demo's circle):
  - Planted feet move 0.0000 mm a frame, walking, running and in every pose.
  - Nothing goes below the ground. Lowest points: wolf 1.42 mm, fox 0.59 mm, deer 3.33 mm, boar 0.58 mm, rabbit 0.01 mm, squirrel 0.02 mm, otter 1.46 mm.
  - Feet are down 0.62–0.67 of the time walking and 0.26–0.37 running.
- The others were checked by the sessions that built them:
  - Firefly: no NaN; its wings stay clear of the ground; its feet land on the leaf, 0 corners inside it over 75 s of its demo.
  - Fish: 26 checks. Its depth held to 0.00 mm, the same distance at 30, 60 and 144 frames a second, its leaps' heights exact, two splashes a leap.
  - Duck: nothing under the ground in any behaviour (lowest +0.4 mm, +0.2 mm taking off on a 1.2 m circle), planted feet 0.0000 mm a frame; its demo keeps within 1.9 cm of its circle.
  - Frog: lowest point 0.30 mm (body) and 0.45 mm (feet) walking at 30, 60 and 144 frames a second and jumping; planted feet 0.00 mm and 0.00°; afloat its back 27 mm out of the water.
  - Butterfly: never below what it stands on (+0.28 mm landing on the ground); its feet still to 0.009 mm while it rests; it lands within 3.2 mm of the flower's middle at 60 frames a second.
  - Otter: in water it swims and dives, tipping at most 53° and back to its depth; its demo loops end on their circles, two splashes each.
  - Explorer: walking and running, straight and on a 1.1 m circle, at 60, 120 and 240 frames a second, planted soles 0.0000 mm a frame, 0.00 mm off the ground; its jump 350 mm; sitting, 170.0 mm on the 170 mm seat; nothing below the ground.
- Each was seen beside its sheet in its views, and in the app on the floor and at the forest lake, in headless Edge on the GPU; all thirteen previews load with no errors of their own, and some parts were shown alone (`?part=`).
- `npm run typecheck`: clean.

## Known issues and left for the user

- The specs are drafts, for the user to correct.
- The rabbit still reads a little longer in the leg than the sheet's compact sitting rabbit.
- The fish's belly renders darker than the sheet's: the stage's light hardly reaches faces turned down. Under the forest lake's water it is seen through the water's colour (72% opaque over deep water), pinkish.
- The firefly's lower legs are a little shorter than the sheet draws them.
- The duck's body reads rounder from behind than the sheet's egg shape, and its spread wings read thin from the side.
- The frog's thigh is a faceted tube rather than the sheet's round bulge, and its poison dart patches are smaller than the sheet's.
- The butterfly's hind wings are about 6% narrower than the sheet's, its colours a little less saturated.
- The otter's sit is less compact than the sheet's front view (its rump stays long behind); its `dark` reads as a richer brown rather than a darker one, as the sheet's does.
- The explorer's hair is tidier than the sheet's and his face flat-shaded where the sheet's is smooth; his cape hangs over the pack, as the back and side views draw it (the 3/4 back view shows the pack bare). About 4,100 triangles show, more than the animals' (1,100–2,300).
- The deer stands 3.3 mm above the ground, the wolf 1.4 mm.

## Open questions

- The explorer (sheet 1, a boy) came with the animal sheets and lives in the same folder, a figure on its own rig: should he live elsewhere?
- ~~Should the animals come to the forest lake itself, roaming it as the lake meeting's animals do?~~ Answered 2026-09-28: they roam it at the forest lake meeting ([forest_lake_meeting.md](forest_lake_meeting.md)). Its own open questions: should the duck, otter and frog swim there, and the butterflies and fireflies fly about?

## History

- 2026-09-28, asked for directly (no task file, not committed): the forest lake meeting's cast, from these animals and the explorer; none of their files changed.
- 2026-09-27, no task file, not committed. The forest lake's animals, from the user's 14 sheets:
  - the toolkit, `ForestAnimal` and `FourLegged`;
  - the wolf, fox, deer, boar, rabbit and squirrel, then the firefly, the fish, the duck, the frog, the butterfly, the otter and the explorer, built by background sessions;
  - their previews, parts and specs.
  - Then a pass against the sheets:
    - the wolf now sits to howl;
    - the squirrel's tail streams behind it running;
    - the deer's antlers are a lyre, its ears leaves, its muzzle cream;
    - the rabbit's head is bigger, with broader ears, a white chest and thicker front legs;
    - `FourLegged` keeps a foot flat on a turn (its swing out undone under the leg's bends; a long paw's edge had dipped up to 3 mm into the ground);
    - `halo()` is smooth;
    - the shots page frames several parts at once;
    - the fish's demo camera is closer;
    - the duck's folded wing is a few smooth facets with one blue speculum, its chest a fuller chestnut;
    - `FourLegged` lifts a tail off the ground in steps of 0.02 rad, not 0.1 (it flicked up);
    - the previews in the overview's order.
