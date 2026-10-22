# 2026-10-20: Proportion and size check of Jatisrana, and the Wilwatikta reference

## Request (the user's words)

| Request | The user said |
|---|---|
| The workflow's "Design Model Proportionality & Size" | "check proportionality our model, if not correct, doing research" |
| A reference | "this web can be object reference to our game https://wilwatiktamuseum.wordpress.com/" |

## Done

- **A reusable check**, `run.ps1 proportions <place>` ([scripts/tools/proportions.py](../../../../project/bargawa/scripts/tools/proportions.py)). It reads the built place without saving it, and writes `renders/checks/<place>/proportions.json`:
  - headroom along the ways people walk, with the floor followed as a walker finds it
  - stair rises and treads
  - passage widths
  - every object's size
  - A first version took roofs and the banyan's limbs for floors (rays from 30 m up), so every line read "open to the sky". Now each floor is looked for at most 0.9 m above the last.
- **The model summary beside the Blender file**, as the workflow now asks: [project/bargawa/blender/places/jatisrana.md](../../../../project/bargawa/blender/places/jatisrana.md), "Proportion and size". It has every measure against its reference, the verdicts, and six proposed fixes.
- **Research** (my own web searches, no agents). As the workflow asks ("AI Research → cached in knowledge"), the sources were cached with `knowledge_fetch`, and with `knowledge_save_web` for the two LACMA records, which need JavaScript:
  - Indonesian adult height (GNFI 2025, citing NCD-RisC): men 166 cm, women 154 cm.
  - Garden steps (Gardens Illustrated): riser at most 15 cm, tread at least 30 cm, 2R + T ≈ 63 cm. Tobermore's guide refused the download (403).
  - Kendi: LACMA M.90.180.2, East Java, circa 14th c., 27.94 × 12.7 × 15.88 cm. Two more seen in search results only: LACMA M.88.72, 18.4 × 20 × 16.5 cm; Brooklyn 75.62.6, 16.8 × 11.1 cm. Brooklyn's page refused the download (429).
  - LACMA AC1993.239.4, an incense burner cover from the Trowulan region (8.3 × 8.9 × 9.5 cm), for the sanggar's pedupaan.
  - *Plumeria rubra* (Wikipedia): 2–8 m.
  - Candi bentar and Wringin Lawang (Wikipedia).
  - A search result says low roofs (±1.8–2 m) on the Joglo Rembang make guests bow out of courtesy. Its source is not pinned down, so it is not cited in the summary.
- **The user's reference**: [project/bargawa/docs/reference/general/wilwatikta-museum.md](../../../../project/bargawa/docs/reference/general/wilwatikta-museum.md). It says what the Majapahit online museum gives each of our models:
  - Bajang Ratu, with its 1.4 m passage
  - bronze oil lamps of 15–20 cm
  - ritual bells and a vajra
  - a livestock bell
  - terracotta post figures
  - jewellery, crowns, keris, dance

  Its home page, the Bajang Ratu post and the Karya Seni category are cached.

## Findings

- **Correct**:
  - the pendopo and its stair (4 × 0.15 m, treads 0.36 m)
  - the hut steps, the pawon's headroom (2.05 m) and the kandhang
  - the mats, the bale, the stump and the damar
  - the gate: 2.24 m clear, between Bajang Ratu's 1.4 m and Wringin Lawang's ~3.5 m
  - the banyan, as designed
- **Not correct**:
  1. **The planned characters are too tall for the huts.** The doors are 1.89 m, and the porch beam is 1.70 m above the porch floor. Ramabargawa is planned at 1.90 m and the men at 1.70–1.85 m, against Indonesian averages of 166 and 154 cm.
  2. **The lake steps**: 0.8 m treads with rises from 0.06 to 0.29 m; the curved slope buries some steps.
  3. **The kendi** is about 0.5 m tall, twice a real one.
  4. **The sanggar stair** rises unevenly (0.15, then 0.25 m).
  5. **The kamboja** is 3 m, young for an old shrine tree.
  6. **A porch post** stands in front of each hut door.

## Decisions (the user's answers)

| Question | The user chose |
|---|---|
| Character heights | "People nearer Javanese size": Ramabargawa 1.80 m, brothers 1.68–1.74, Jamadagni 1.66, Citrarata 1.74, Renuka 1.55. The huts stay true to Javanese size |
| The other fixes | "Yes, apply all five" |

## Applied

- **Heights**:
  - [models.md](../models.md) "Heights" (the old values kept for the record)
  - the five character plans in `project/bargawa/docs/summaries/models/`
  - the game's stand-in (`player.gd`, 1.80 m)
- **Builders**:
  - **The ground**: `jatisrana.height()` now cuts it to the lake steps' line. The old height is `_natural()`; the stair's constants are `STEPS_W`, `STEPS_N`, `STEPS_Z0` and `STEPS_Z1`.
  - **The lake steps**: `buildings.lake_steps`, 26 equal steps.
  - **The sanggar stair**: `buildings.sanggar`, four steps of 0.19 m to the 0.95 m tier.
  - **The huts**: `buildings.hut` has four porch posts, and its ridge follows the depth.
  - **The kendi**: `dressing.yard_dressing`, 0.54×.
  - **The kamboja**: `plants.kamboja` has a 2.3 m trunk and limbs ×1.45.
- **Rebuilt** with `run.ps1 place jatisrana` (7 s), and **checked again** with `run.ps1 proportions jatisrana`:
  - the lake steps: 25 rises of 0.137 m, treads 0.36–0.38 m
  - the sanggar: 5 rises of 0.19 m, treads 0.26–0.28 m
  - the kamboja: 4.58 m
  - both huts: 1.70 m under the porch beam
- **Found by the re-check**: the sons' deeper hut had only 1.50 m under its porch beam. Fixed: its ridge now rises with the depth, to 4.69 m.
- **The analytic check** in Python (no Blender) confirmed every step's top stands above the cut ground and its foot below it, at both edges and the middle of each tread.
- **The game**: `make measure` exported, imported and walked the route with the 1.80 m stand-in, down the new lake steps and back up:
  - nothing stuck
  - 116 fps on average, 1% low 92
  - 432k triangles and 332 draw calls on average

  The `lake` still shows the even stair down to the bathing stone.
- **The summary beside the place**, [jatisrana.md](../../../../project/bargawa/blender/places/jatisrana.md), has an "Applied" section with the new measures.
- **No manual guide** in `project/bargawa/references/` covers these builders, so no guide changed.

## Next

- The place's texturing and lighting states, in the same summary.
- The characters' model design in Blender (MPFB), at the new heights.
