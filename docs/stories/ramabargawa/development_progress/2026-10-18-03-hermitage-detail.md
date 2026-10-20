# 2026-10-18: The hermitage in detail (buildings, plants, grass, textures)

Second pass on Pertapan Jatisrana: every building at the pendopo's level of detail, the pendopo in its own file, real plants instead of placeholders, grass across the clearing, and Poly Haven textures.

## Decisions (the user's words)

| Decision | The user said |
|---|---|
| Keep the alang-alang thatch as built | "for 1, alang alang as build" |
| The layout is right | "2, right" |
| Go on with the next steps (refine buildings, hero plants, textures) | "and do next" |
| The pendopo in its own Blender file | "separate pendopo to other blender file buildings/pendopo" |

## Done

1. **The pendopo is its own file**: `project/bargawa/blender/buildings/pendopo.blend`.
   - It is built by `scripts/buildings/pendopo.py` (`run.ps1 building pendopo`). Its collection `Pendopo` sits at the origin, facing south, and is marked as an asset. A `_Review` collection holds a ground disc, a sun and the sky.
   - The place **links** that collection and shows it as a collection instance at the pendopo's spot. A rebuilt pendopo shows up in the place when the place is reloaded or rebuilt.
   - The S1 dressing (mats, bale, lamp) stays in the place, because it changes with the light states.
   - `run.ps1 views pendopo -Building` renders review stills around a building file.
2. **Buildings refined** (`common/buildings.py`):
   - **Huts**: a timber frame on umpak stones (saka posts, blandar wall plates, pengeret tie beams, ander king posts, molo ridge beam, gording purlins, usuk rafters), woven bamboo walls with rails, a plank door in its kusen frame, barred windows east and west, the gable ends, and a ridge roll. The south roof drops into a **porch roof (emper) on three posts**, with a lincak (bench) under it. The sons' hut is larger (6 × 4.5 m).
   - **Pawon**: the clay stove (luweng) with fire holes and fire mouths, a pot (kendil) and a wok (wajan), the water jar (gentong) with lid and ladle (siwur), a shelf of pots, a drying rack (para-para) over the fire, firewood, and a raised cap over the ridge for the smoke.
   - **Sanggar pamujan**: moulded base tiers, a body with corner pilasters, a framed niche under a kala boss, three roof tiers with small ratna at the corners and a crowning ratna, and a stair whose side walls (pipi tangga) end in scrolls.
   - **Gate**: moulded pillars with stepped crowns and ratna, walls with a coping.
   - **Kandhang**: a gate gap in the fence, a trough, straw on the ground.
3. **Real plants** (`common/plants.py`). Each leaf clump is a dark inner blob with leaves on its skin, the stylized look.
   - **Banyan**: a fluted trunk flaring into 7 buttress roots over the ground, 7 limbs with side branches, curtains of thin hanging roots (a few grown down into pillar roots), and a crown of 29 leaf clumps (about 32,000 leaves).
   - **Bamboo**: arching culms with nodes, twigs with fans of narrow drooping leaves (2 clump shapes, 14 copies).
   - **Banana**: broad leaves on arching midribs, some torn, dead leaves hanging down the stem.
   - **Kamboja**: a crooked trunk forking into blunt branches, leaf rosettes, white five-petalled flowers, and **fallen flowers on the ground**.
   - **Jasmine**: a round bush of small dark leaves with white star flowers.
   - **Lotus**: pads with their notch (some raised on stalks), open pink flowers, closed buds.
   - **Glagah**: tufts with tall stalks and white feathery plumes; reeds with brownish plumes.
   - **Tree ferns**: fronds with leaflets.
   - The far forest stays as simple tree shapes.
4. **Grass** (`common/grass.py`): 46,000 clumps of three kinds over the clearing, from a hair particle system on a hidden emitter. A density map keeps grass off the paths, the buildings, the steps, the water and the banyan's foot, and thins it out towards the forest.
5. **Poly Haven textures** (CC0). `run.ps1 fetch` downloads them; the list is in `common/polyhaven.py`.

   | Material | Texture |
   |---|---|
   | Thatch | `reed_roof_04` |
   | Woven walls | `bamboo_wall_03` |
   | Teak | `wood_table_worn` |
   | Andesite blocks | `rock_wall_08` |
   | Stone | `rock_surface` |
   | Banyan bark | `tree_bark_03` |
   | Dark bark | `bark_platanus` |
   | Paths | `forest_ground_04` |
   | Ground | `brown_mud_leaves_01` |

   - The meshes have no UV maps, so the textures are box-projected in object space, with bump from the height map.
   - Each texture is mixed with its palette colour, so the palette still sets the mood.

## Checks

Review stills: `renders/checks/jatisrana/sheet.png` (16 views) and `renders/checks/pendopo/sheet.png`.

| Problem found | Fix |
|---|---|
| Grass about 1.5 m tall | Hair particles also scale their instances by hair length (default 4); set to 1 |
| Grass orientation (tested in Blender 5.2) | Orientation Global Z with a random phase turns each instance's **+Y** up and spins it about the vertical, so the grass clumps are modelled with +Y up |
| The banyan's pillar roots looked like a forest of poles | Far fewer, only from the limbs, curved and flaring at the foot |
| Bamboo tops bare, jasmine like spiky grass, kamboja rosettes tiny | More twigs and leaves; jasmine rebuilt as leaf clumps with flowers; bigger rosettes |
| Banyan bark too pale, almost glowing | More of the palette's bark colour mixed in |
| Thatch looked smooth | Finer tile (1 m) and stronger bump |

The place builds in about 5 s: 922 objects, about 230,000 faces of unique meshes, an 8 MB file.

## Files and disk space

- While the user had `blender/places/jatisrana.blend` open, the work above was first built into a test copy in `_work/`. The user then said: "its okay you rewrite it, dont make temp file that runout my space".
- So the real files were rebuilt directly (`run.ps1 place jatisrana -Force`; the user's open session needs File > Revert).
- The test copy and every `.blend1` file were deleted.
- The builders now turn off `.blend1` backups (`common/mesh.fresh_file`).
- Sizes: `jatisrana.blend` 9.9 MB, `pendopo.blend` 0.1 MB (the textures are linked from `blender/assets/polyhaven/`, about 15 MB).

## Notes

- The lake water is still a plain dark surface, and the light is one plain morning sun. Both are milestone 5, the light states.
- The far forest is still simple shapes.

## Next

- As the user prefers:
  - the light states S1–S7 with the lake's water and atmosphere (milestone 5)
  - the story variants (milestone 6): burned pendopo, scorched banyan, broken kendi, scattered beads, pyre
  - the other buildings in their own files like the pendopo
  - a better far forest
