# Fire: the sample collections

A file of fire samples, one collection for each kind of fire the story needs, so places and light states link them rather than building fire each time (the user, 2026-10-18: "can we create objects/fire.blend to create fire collection sample ?"). The Blender file is named after this doc: `blender/objects/fire.blend`.

The method follows the fire reference the user gave, Max Hay's video ([references/video-fire-max-hay.md](../../../../project/bargawa/references/video-fire-max-hay.md)). This doc adds what real fire does, so each sample gets its size, height, colour and rhythm.

## What the story needs

From [environment.md](../environment.md): "Warmth is the family". Each warm light belongs to someone, and the last one in beat 3 is the funeral fire.

| Fire | Scene | Sample |
|---|---|---|
| The damar's flames, Jamadagni's lamp | S1 all four, S4 one | `Fire_Lamp` (the same flame as `damar.blend`) |
| Renuka's cooking fire in the pawon's clay stove (luweng) | S1 | `Fire_Hearth` |
| A wood fire in the open | the general size between the two; the video's own campfire | `Fire_Wood` |
| The burned pendopo, smouldering | S5: "smoke columns, falling ash, smouldering embers" | `Fire_Embers` |
| The funeral pyre in night rain, sparks rising | S6 (still an open question, [environment.md](../environment.md), "Open questions" 3) | `Fire_Large` (the fire; the pyre's logs are a story model for later) |

## How real fire behaves

### Height and size

**Flame height (Heskestad).** The mean flame height of a fire of diameter D (m) burning Q (kW) is L = 0.235 Q^(2/5) − 1.02 D. "Mean" means the flame reaches that height half the time. It is the standard correlation in fire safety, valid for Q^(2/5)/D from about 7 to 700 ([UFRJ course notes on flame height](https://eq.ufrj.br/antigo/docentes/cavazjunior/alturai.pdf); also [IAFSS, Fire Safety Science 5](https://publications.iafss.org/publications/fss/5/427)).

| Sample | Base D | Heat Q (assumed) | L from the formula | L used |
|---|---|---|---|---|
| Fire_Hearth | 0.20 m | inside a stove | (confined) | 0.24 m, under the fire mouth's 0.28 m |
| Fire_Wood | 0.45 m | 40 kW, a small campfire | 0.46 m | 0.50 m |
| Fire_Large | 1.40 m | 700 kW, a big log fire | 1.80 m | 1.80 m |

The heat outputs are my estimates, not measurements.
- A small campfire burns about 2–3 g of wood a second, at about 15 MJ/kg: roughly 30–45 kW.
- A pyre is larger still.

**Three zones, from the bottom up.** This is standard fire-plume knowledge; the numbers are approximate:
1. **Continuous**, up to about 0.4 L: solid flame, always there.
2. **Intermittent**, from 0.4 L to about 1.6 L: separate tongues that break off and burn out as they rise.
3. **Plume**: hot gas and smoke only, no flame.

That's what the shader draws. A rising turbulence pattern is added to a brightness that fades with height. Low down the brightness wins, so the flame is solid. Higher up only the turbulence's peaks are bright enough, so tongues appear, break off and vanish.

### Rhythm

**Puffing.** A buoyant fire breathes. Big bulges of flame form at the base and rise, at a frequency set by the diameter. A Loughborough University review found f = 1.68 D^(−1/2) Hz fits a wide range of data (Malalasekera et al. 1996, cited in [the review](https://repository.lboro.ac.uk/articles/journal_contribution/A_review_of_research_and_experimental_study_on_the_pulsation_of_buoyant_diffusion_flames_and_pool_fires/9547682)).

| Sample | D | Puffing |
|---|---|---|
| Fire_Hearth | 0.20 m | 3.8 Hz |
| Fire_Wood | 0.45 m | 2.5 Hz |
| Fire_Large | 1.40 m | 1.4 Hz |

So a big fire moves slowly and a small one quickly. The fire's height and its light's flicker follow this frequency, so each fire moves at its own pace.

### Colour

- **Wood flames are yellow to orange.** The light comes from hot soot. The flame is palest yellow low in the core and turns orange, then deep red at the tips as it cools. Unlike the oil lamp's small flame, there is no visible blue at the root.
- **Embers glow by temperature.** They are dull red from about 525 °C (the Draper point, where hot objects start to glow visibly), cherry red, then orange around 900 °C. The glow is brightest in the cracks of the charcoal. It brightens and dims as air moves over it, and grey-white ash covers the cooler surfaces.
- **Sparks** are glowing bits of wood carried up by the hot gas. They cool from yellow to orange to red and go dark within a few seconds, swirling as they rise.

### The Javanese hearth

- **The pawon is the place of ash.** The word *pawon* is thought to come from *awu*, ash: the place where wood is burned to cook ([Liputan6, on the Javanese pawon](https://www.liputan6.com/regional/read/6049059/menguak-kisah-dan-filosofi-pawon-dapur-tradisional-jawa)).
- **What it holds:** a clay stove (*tungku*, *luweng*) or an iron brazier (*anglo*), the firewood arranged in it, and a wok or pot on top (same source).
- **How the fire is fed:** long sticks are pushed in through the stove's fire mouth and moved forward as their ends burn away. This is general knowledge, not from the source.
- **The hermitage's luweng** is already built in the pawon, 1.7 m wide, with:
  - two fire mouths, 0.30 m wide and 0.28 m tall, in its front
  - two pot holes, 0.17 m across, on top

  `Fire_Hearth` is sized for one fire mouth: sticks pushed in from the front, burning at their tips.

## The samples

All five are collections marked as assets, each with its root at the instance origin, the way the damar's flame is.

| Collection | What's in it | Light |
|---|---|---|
| `Fire_Lamp` | The damar's flame: 3.4 cm, drawn on three cards (`fire.flame_piece`) | 0.5 W, 2 cm |
| `Fire_Hearth` | Three sticks pushed in from the front (−y), their tips charred and glowing; a small bed of coals; flames 0.20 × 0.24 m; a few sparks | 30 W, flickering at 3.8 Hz |
| `Fire_Wood` | Six logs leaning together over two laid flat, charred towards the middle; a bed of coals; flames 0.45 × 0.50 m; sparks | 150 W, 2.5 Hz, the light moving slightly so shadows sway |
| `Fire_Embers` | A bed of coals and ash with three charred stubs, glowing in their cracks; no flame | 12 W, slow and dim |
| `Fire_Large` | Four big logs crossed on a wide bed of coals; flames 1.4 × 1.8 m on four cards; many sparks | 900 W, 1.4 Hz |

### How each part is made

| Part | After the video | Here |
|---|---|---|
| Flames | Fire footage on a plane, keyed to alpha, an orange multiplied in, looped, copies offset in time | **Drawn by the shader** on upright cards through the fire's axis (Blender only, no stock footage), moving with the frame; each instance at its own moment. The wood fires use `fire.fire_material`, described above, with no blue root. The flames add their light to what's behind them (like real flames, which hardly block light), so overlapping cards brighten the core |
| Light | A point light the fire's size does most of the lighting, emission kept low | The same, inside each fire, flickering at the fire's puffing rate |
| Sparks | Particles: no gravity, a Turbulence field, Frame Start −200, lifetime 400, baked | **Geometry Nodes**, calculated from the time alone: each spark has a fixed life, a start and a swirl, so they're already in the air on frame 1 and need no bake. Particles and force fields don't work well inside a linked collection instance, and these samples are linked |
| Embers, charred wood | (not in the video) | A shader: charcoal with grey ash on top and glowing cracks (Voronoi edges) that slowly brighten and dim. The logs fade from bark (Poly Haven `bark_brown_01`, as [environment.md](../environment.md) lists for the pyre) to charcoal towards the fire, glowing where they are most burned |
| Haze, motion blur | Volume Scatter 0.02; motion blur on the sparks | Not in the samples: haze belongs to each scene's light state, and motion blur to the story cameras |

Smoke (the pawon, the incense, S5) is fire's companion but not part of this file. It can come next.

## Built

`scripts\run.ps1 object fire` builds `blender/objects/fire.blend`. The code is `scripts/objects/fire.py`, with `common/fire.py` and `common/fuel.py`. To build it by hand, follow guide [08-fire-samples.md](../../../../project/bargawa/references/08-fire-samples.md).

| Collection | Faces | Light | Puffing |
|---|---|---|---|
| Fire_Lamp | 3 cards | 0.5 W | (the lamp's gentle waver) |
| Fire_Hearth | 850 | 30 W | 3.8 Hz |
| Fire_Wood | 1,354 | 150 W | 2.5 Hz |
| Fire_Embers | 1,292 | 12 W | 0.4 Hz (no flame) |
| Fire_Large | 3,375 | 900 W | 1.4 Hz |

**What it looks like** (stills in `renders/checks/fire/`):
- By day the flames are pale orange and see-through, as fire is in daylight.
- At night each fire lights the ground round it, and its logs throw long shadows that sway.
- The flames are solid yellow low down. Orange tongues curl up and break away above the mean height, and a few detached bits burn out higher still.
- Sparks drift up and sideways, yellow near the fire and red and smaller higher up.
- The embers are a dark bed mottled with grey ash, coals with thin orange cracks, some brighter than others.

**Linked**: Fire_Wood, linked twice into an empty file, gives each copy its logs, coals, flames, flickering light and all 60 sparks.
- The flames differ per copy (Object Info › Random).
- **The sparks are identical in every copy**: Geometry Nodes run once per object, not per instance. Each story fire appears once, so it doesn't show. Two fires side by side would need their own spark objects with different seeds.

**Fixed along the way** (details in guide 08, "Pitfalls"):
1. The flames read pink over daylight. Their ramp now starts at orange-red.
2. They were a soft blob. Stronger turbulence breaks them into tongues.
3. The big fire glowed white: its four crossed cards add up. Its emission is lower.
4. A glowing ball formed where the tepee's tips meet. Those tips are black now.
5. The glowing cracks looked like a giraffe's pattern, and then the ash like camouflage. Both are finer and softer now.
6. The ash bed showed as a clean disc. Its edge is ragged now.

## Open choices

- The damar could link `Fire_Lamp` from this file rather than keep its own flame. It's the same flame; linking would keep one copy.
- S6: whether the pyre is shown at all is still open ([environment.md](../environment.md)). If it is, `Fire_Large` sits on a pyre model.
