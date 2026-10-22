# Pertapan Jatisrana: Model Summary

The summary of `jatisrana.blend`, beside it as the workflow asks ([bargawa_game.md](../../bargawa_game.md), Model Design → "Saved File" → "Model Summary"). The place is built by `scripts/places/jatisrana.py`; its design is [environment.md](../../../../docs/stories/ramabargawa/environment.md).

States of the model design: **proportion and size** (below, 2026-10-20), **texturing** (begun: the sasak thatch and the spiky ground, 2026-10-20; the soil in place of the spiky ground, 2026-10-21) and lighting (to come).

## Proportion and size (2026-10-20)

**How it was measured**: `run.ps1 proportions jatisrana` ([tools/proportions.py](../../scripts/tools/proportions.py)) reads the built place, saves nothing, and writes `renders/checks/jatisrana/proportions.json`.
- **Headroom**: rays through the real geometry find the floor and the free height above it along the ways people walk. The floor is followed as a walker finds it: at most 0.9 m above the last one, so a step counts and a roof doesn't.
- **Stairs**: rises and treads from the floor profile along each stair.
- **Sizes**: the bounding box of every object.

Exact numbers that the rays can't separate from furniture (a post's base, the stove) come from the builders' code (`common/buildings.py`, `common/dressing.py`).

**Human scale**:
- **Indonesian adults now**: about **166 cm (men)** and **154 cm (women)** on average ([NCD-RisC figures, via GNFI 2025](../../../../docs/external_sources/web/goodnewsfromindonesia.id/2025-10-09-rata-rata-tinggi-badan-orang-indonesia-2025-data-terbaru-pr-f1eb3c.md); a news article citing them, not the study).
- **The characters planned in models.md at the time of the check**: men 1.70–1.85 m, **Ramabargawa about 1.90 m**, women 1.55–1.65 m. They have since been changed (see "Applied").

### Results

| What | Measured | Reference | Verdict |
|---|---|---|---|
| **Hut porch** (both huts) | floor 0.30 m; beam underside 2.00 m above the ground = **1.70 m above the porch floor**; the middle of the three porch posts stands 1.45 m in front of the door | Indonesian men 1.66 m | ✗ **a 1.90 m Ramabargawa and the 1.70–1.85 m men must stoop**; a post in the door's way |
| **Hut door** | 0.90 × **1.89 m** clear | as above | fits 1.66 m people; **not 1.90 m** |
| Hut step | 0.16 + 0.14 m up to the porch | garden steps: riser ≤ 15 cm ([Gardens Illustrated](../../../../docs/external_sources/web/gardensillustrated.com/garden-design-design-solutions-coping-with-slopes.md)) | ✓ |
| Hut size | 5.0 × 4.0 m (sons' 6.0 × 4.5 m), walls 2.2 m, ridge 4.5 m | | ✓ plausible |
| Pendopo | floor 0.6 m; eaves 2.1–2.4 m above where one stands; posts and umpak by the pakem | [buildings/pendopo.md](../../../../docs/stories/ramabargawa/buildings/pendopo.md) | ✓ researched |
| Pendopo stair | 4 rises of **0.15 m**, treads **0.36 m** (2R + T = 0.66 m) | outdoor: riser ≤ 15 cm, tread ≥ 30 cm; 2R + T ≈ 63–65 cm (Blondel) | ✓ |
| Pawon | earth floor; **2.05 m** under the beams; drying rack 1.8 m over the stove only | | ✓ |
| **Lake steps** | 12 treads of **0.80 m**; the step tops are on a straight line, but the slope is curved, so the ground buries some steps and leaves others standing. Walked: rises from **0.06 to 0.29 m**, total drop 3.44 m | all risers equal; riser 14–15 cm, tread 30–35 cm outdoors | ✗ uneven and awkward: 0.8 m is neither one stride nor two |
| **Sanggar stair** | rises 0.15, 0.15, 0.15, **0.25, 0.25** m to the second tier (0.95 m), treads 0.31–0.32, then 0.22 m | all risers equal; shrine stairs may be steep | ✗ uneven (0.15 then 0.25) |
| Gate | two pillars 2.8 m (3.4 m with ratna), **2.24 m clear** between them at 1 m | Majapahit: **Bajang Ratu**, a roofed paduraksa, passage **1.4 m** ([Wilwatikta Museum](../../../../docs/external_sources/web/wilwatiktamuseum.wordpress.com/2012-02-07-candi-gapura-bajang-ratu-peninggalan-kerajaan-wilwatikta.md)); **Wringin Lawang**, a split candi bentar, about 3.5 m (one source only) | ✓ within the range of the real gates; a split gate like ours is the wider kind |
| Kandhang | posts 1.4 m, rails at 0.5 and 1.05 m, gate gap 1.2 m, lean-to roof 2.1 → 1.7 m | | ✓ for goats |
| Mats, bale, stump | mats 1.0 × 1.7 m; Jamadagni's bale 2.0 × 1.3 m, 0.36 m high; stump 0.62 m | | ✓ plausible |
| **Kendi** (Renuka's, on the elders' porch) | about **0.50 m tall**, body 0.34 m across | East Javanese earthenware kendi, 13th–15th c.: **27.9 × 12.7 × 15.9 cm** ([LACMA M.90.180.2](../../../../docs/external_sources/web/collections.lacma.org/node-178156.md)); 18.4 × 20 × 16.5 cm (LACMA M.88.72) and 16.8 × 11.1 cm (Brooklyn 75.62.6), from collection search results | ✗ **about twice too big** |
| Damar | lamp 0.21 m on a stand 0.70 m | [objects/damar.md](../../../../docs/stories/ramabargawa/objects/damar.md) | ✓ researched |
| **Kamboja** | **3.0 m** tall, 4 m wide | *Plumeria rubra* grows to **2–8 m** ([Wikipedia](../../../../docs/external_sources/web/en.wikipedia.org/wiki-plumeria-rubra.md)) | ✗ young for the old shrine tree of the design; ~5 m would suit |
| Banyan | crown 21 m across, about 13 m tall | environment.md asks for a canopy about 20 m across | ✓ by the design |
| Bamboo | clumps 12–16 m tall, 7–11 m across | | ✓ plausible (no source checked) |
| Banana, tree ferns, jasmine, glagah, reeds | 3.1–3.5, 2.9–3.3, 1.2, about 4, about 3 m | | ✓ plausible (no source checked) |

### What is not correct, and the proposed fixes

1. **The planned characters are too tall for the place.** The buildings keep real Javanese proportions: doors 1.89 m, the porch beam 1.70 m above the porch floor. The characters planned in models.md (Ramabargawa 1.90 m, men 1.70–1.85 m) stand above Indonesian adults (166 / 154 cm) and would stoop at every hut. This is the user's decision:
   - **(a) People nearer Javanese stature**:
     - Ramabargawa 1.80 m, still the tallest
     - his brothers 1.68–1.74 m
     - Jamadagni 1.66 m
     - Citrarata 1.74 m
     - Renuka 1.55 m

     They pass the doors upright. The tallest still bow under the porch beam, a gesture of respect that the story can use (Bargawa bowing as he comes to his parents' hut).
   - **(b) Keep the heights and raise the huts**: the porch beam to 2.05 m above the porch floor, the doors to 2.0 m.
2. **Lake steps**: equal steps, for example 25 rises of 0.138 m on treads of 0.38 m (2R + T = 0.65 m), with the ground cut to the stair's line so no step is buried. This changes `buildings.lake_steps` and `jatisrana.height` near the stair.
3. **Kendi**: halve it to about 0.27 m tall with a 0.16 m body, as the LACMA kendi.
4. **Sanggar stair**: five equal rises of 0.19 m to the second tier, treads 0.27 m (2R + T = 0.65 m): steep, as a shrine stair, but even.
5. **Kamboja**: grow it to about 5 m, an old tree by the shrine.
6. **The hut porch posts**: move the middle post out of the door's way, so two posts flank the door.

### Applied (2026-10-20)

The user chose "people nearer Javanese size" and "apply all five". The place was rebuilt (`run.ps1 place jatisrana`) and checked again:

| Fix | Where | After |
|---|---|---|
| 1. Character heights: Ramabargawa 1.80 m, brothers 1.68–1.74, Jamadagni 1.66, Citrarata 1.74, Renuka 1.55 | [models.md](../../../../docs/stories/ramabargawa/models.md), [docs/summaries/models/](../../docs/summaries/models/index.md), the game's stand-in (`player.gd`, 1.80 m) | all pass the doors (1.89 m) upright; those over 1.70 m bow under a porch beam |
| 2. Lake steps | `jatisrana.STEPS_*` and `height()` (the ground cut to the stair's line), `buildings.lake_steps` | **25 equal rises of 0.137 m, treads 0.37 m** (2R + T = 0.65 m); no step buried. The top step is flush with the terrace |
| 3. Kendi | `dressing.yard_dressing` | about 0.27 m tall, body 0.18 m |
| 4. Sanggar stair | `buildings.sanggar` | **five equal rises of 0.19 m**, treads 0.27 m, to the 0.95 m tier |
| 5. Kamboja | `plants.kamboja` (taller trunk, longer limbs, same leaves and flowers) | **4.6 m tall**, 5 m across |
| 6. Hut porch posts | `buildings.hut` | four posts; the middle two flank the door |

**Found by the re-check and fixed**: the sons' hut is deeper (4.5 m against 4.0 m). At the same 38° pitch its roof came lower at the front, leaving only **1.50 m** under the porch beam. Now the ridge follows the depth (`ridge = top + 0.35 + pitch × (d/2 + 0.1)`): both huts have 1.70 m under the porch beam, and the sons' ridge is 4.69 m.

Unchanged and correct: the pendopo and its stair, the pawon, the gate, the kandhang, the mats, the bale, the stump and the damar.

## Texturing

### The sasak thatch (2026-10-20)

The user made a better sasak roof material by hand (`blender/textures/material_manual.blend`, "implement it"). It is now the thatch of every roof. The user chose "All thatch", "Real displacement" and "Realistic" shading.

| Roof | Base faces | Film triangles |
|---|---|---|
| `Pendopo_Roof` (in `buildings/pendopo.blend`) | 4 416 | 623 514 |
| `Hut_Sons_Roof` | 720 | 107 424 |
| `Hut_Elders_Roof` | 560 | 85 088 |
| `Pawon_Roof` | 378 | 66 804 |
| `Kandhang_Roof` | 190 | 32 124 |
| **all** | | **914 954** |

- **Material and relief** from the texture library: `M_thatch` and `GN_thatch` ([plants.md](../textures/plants.md)).
- **The straw hangs down every slope**: the roofs have UVs along the eave and up the slope.
- **Real relief on the outside only**:
  - up to 8 cm, about 2.6 cm on average
  - the underside, seen from inside the pendopo, stays flat
  - the cut edge at the eaves shows straw ends
- **The roof shapes are unchanged**, only finer: faces of about 0.3 m. The pendopo has 40 points along each eave instead of 16, and a ring every 0.3 m up the slope. The eaves' small unevenness now changes every 0.3 m.
- **Checked** in small stills (`roof_eave`, a new view at the pendopo's south-west eave; `pendapa`, `huts`, `pawon`, `pendapa_inside`):
  - golden courses on every slope, straight on the east and west sides too
  - relief at the eave corner, straw ends at the cut edge
  - the courses seen between the rafters from inside
  - no broken points (non-finite: 0)

### The bare ground (the soil since 2026-10-21; the spiky ground 2026-10-20)

Bare ground where the camera comes close. The user chose the places, their own photo over a Poly Haven height map, and a lighter version for the game. First it wore their spiky look (their `Ground_2.blend`, "no i like this spiky"). Since 2026-10-21 it wears their soil (the `Soil` material in `material_manual.blend`, "this is better for jatisrana"): soil with leaf litter, its relief from a bump. `terrain.BARE` picks the look; both are in the texture library.

| Patch | Where | Base grid | Film triangles |
|---|---|---|---|
| `BareGround_Yard` | round the pendopo (with its stair) and the two huts: rounded rectangles, `J.YARD` | 14 755 faces at 14 cm | 118 040 |
| `BareGround_FarBank` | about 5 m round Citrarata's spot, down to the water, `J.FAR_BANK_R` | 2 920 faces | 23 360 |

- **Material and relief** from the texture library: `M_ground_soil` and the Geometry Nodes `GN_ground_soil` ([ground.md](../textures/ground.md), "The soil").
  - The seamless photo repeats every 5.6 m, with a strong bump from the photo itself.
  - Its ramp (black at 0.655) raises no point, in the user's plane too, so the patches lie flat for now. The look is the photo and its bump. Lowering the ramp's black stop would make the light leaves and pebbles rise.
  - With the spiky ground, the ramp (white at 0.427) raised each point up to 0.1 m, on a 7 cm grid.
- **Lying on the terrain**: each point is found by a ray on the terrain's own triangles, 2 cm above them, so nothing of the terrain shows through.
- **Edges**: the last metre (`J.BARE_FADE`) fades out. The spikes drop to flat and the material turns see-through, so the patch melts into the ground. The edges wave a little (`J._wobble`), so they don't run straight.
- **By the paths**: no spikes within 0.7 m of a path's centre line (full height from 1.2 m), so none poke through the 1.2 m paths. Checked: within 0.65 m of a path the patch stays at its 2 cm lift, under the paths' 4 cm.
- **By the water**: no faces where the ground is under the lake. The spikes fade out over the last 0.3 m above the water.
- **Under the buildings**: no faces under the pendopo's plinth or the huts' stone bases, where it is never seen.
- **The grass** no longer grows in the yard (`grass.density` × (1 − `J.yard`)), with the same one-metre fade. Before, it grew up to 0.3 m from the pendopo's plinth.

Checked in small stills (`run.ps1 views jatisrana -Only "yard,far_bank,huts"`, new views `yard` and `far_bank`):
- The spiky ground (2026-10-20): the yard read as dark, rough soil round the pendopo, and the far bank as a dark patch round Citrarata's spot, fading into the ground.
- The soil (2026-10-21, also `pendapa`): fine soil in the yard and on the far bank. Under the sky it reads greyer and a little lighter than in the user's plane: the user's Metallic 0.177 darkened it and kept it brown.
- The photo is much darker than the terrain's ground texture, so the patches read darker than the ground round them. This is open for the user's critique.

## Lighting

To come.
