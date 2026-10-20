# 2026-10-18: The hermitage, first build (setting and models)

The first Blender build of Pertapan Jatisrana: the setting (terrain, lake, layout) and the hermitage's models, made by script in `project/bargawa/`. The pendapa is modeled in detail; the other buildings are simple but complete; the plants are placeholders.

## Done

1. **Set up `project/bargawa/`** (README, `.gitignore`, `scripts/run.ps1`):
   - `run.ps1 place jatisrana` builds the place headless.
   - `run.ps1 views jatisrana` renders review stills.
   - The `.blend` files, renders and logs are git-ignored; the scripts rebuild them.
2. **The place's measures** in `scripts/common/jatisrana.py`, the one source of where everything stands:
   - Axes: x east, y north, metres. The pendapa is at the origin, the terrace at z 0, the lake surface at z -4.
   - The lake is 40 × 25 m, south of and below the terrace.
   - The terrain rises north towards the mountain and has wooded valley walls.
3. **Setting**: terrain (400 × 400 m), lake water, the paths between the buildings, three far mountains to the north.
4. **The pendapa** (`common/joglo.py`), modeled as a joglo:
   - stone plinth (bebatur) with steps on the south
   - umpak (pillar bases) under 4 soko guru and 12 soko penanggap
   - blandar and sunduk beams, 4 tumpang sari frames stepping outward, the dhadha peksi beam
   - rafters (usuk) and hip rafters (dudur)
   - a thatched roof with the steep brunjung overhanging the gentler penanggap, a slightly uneven eave, the ridge with end ornaments
5. **Other buildings** (`common/buildings.py`):
   - two huts of woven bamboo with thatched gable roofs, doors facing south
   - the pawon (kitchen) with its clay stove
   - the sanggar pamujan: a stepped stone shrine, its niche, altar stone and stair facing east, and a flat sitting stone
   - the stone gate
   - the kandhang (livestock pen)
   - 12 stone steps down to the bathing stone
6. **Placeholder plants** (`common/plants.py`): banyan (fluted trunk, limbs, 64 hanging and pillar roots, a broad crown), bamboo grove, bananas, kamboja with white flowers, jasmine, glagah plumes, reeds, lotus, tree ferns, and 857 forest trees.
7. **Dressing for state S1** (`common/dressing.py`): Jamadagni's bale, the five mats, the oil lamp, the woodcutter's stump and axe, the kitchen's pots and woodpile, Renuka's kendi, the incense burner. Orange **scale figures** stand where people will be; they only show in review stills.
8. **Light for review**: a plain sky and one sun (morning, south-east). The seven light states come later (milestone 5).

## Checks

Review stills (`renders/checks/jatisrana/sheet.png`): map, overview, pendapa, inside the pendapa, from the lake, the huts, the shrine.

| Problem found | Fix |
|---|---|
| The ground was blue-grey and everything below z 0 was in shadow: the mountains had a closed base 1.7 km wide lying over the whole valley at z 0 | Mountains open at the bottom, moved to 2.4 km north, their feet below the ground |
| The banyan's crown hid the pendapa from above and from the lake | Banyan moved from (5, -9.5) to (8.5, -13.5), at the terrace's edge |
| The ground looked bare brown | The ground material is now mostly grass with patches of earth |

Result: 916 objects. From the lake, the steps now lead up to the pendapa, with the banyan on the right and the mountain behind it.

## Decisions (the user's words)

| Decision | The user said |
|---|---|
| Work on the hermitage first | "lets focus on hermitage" |
| No story cameras yet; models and setting first | "we talk direct camera later, we ensure model and setting first" |

The seven key cameras in environment.md stay a plan for later. The place file has no cameras; review views live in `tools/views.py`.

## Open questions

- **Roof material**: built as light alang-alang thatch. Ijuk (black palm fibre) is the other choice, darker and more like a shrine.
- **Layout**: is the arrangement right (huts behind the pendapa, shrine to the west, kitchen and pen to the east, the lake below to the south)?
- The questions from the first note are still open: render engine, Renuka's version, the funeral, the symbols, the character source.

## Next

- The user reviews the build: open `project/bargawa/blender/places/jatisrana.blend` in Blender, or see `renders/checks/jatisrana/sheet.png`.
- Then, as the user prefers:
  - refine the buildings (huts, pawon, sanggar to the pendapa's level of detail)
  - build the hero plants properly (milestone 3), starting with the banyan
  - add Poly Haven textures (milestone 4)
