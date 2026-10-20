# Ramabargawa: Development Progress

A note for every piece of development work on the Ramabargawa story, so the knowledge is recorded and the next session can pick up from here (the user, 2026-10-18). One file per session, named `YYYY-MM-DD-NN-topic.md`, newest at the top of the list below.

## Current state

- **Phase**: building the hermitage, Pertapan Jatisrana: setting and models first, cameras later.
- **Built** (scripts in `project/bargawa/scripts/`, see its README):
  - `blender/objects/stone.blend`: the chipped stone pieces (umpak, stair) in their own file (`run.ps1 object stone`); buildings link them.
  - `blender/objects/damar.blend`: Jamadagni's oil lamp, a bronze pole lamp with four flames (`run.ps1 object damar`); the place links it. Its flames move with time, after the fire video (`common/fire.py`).
  - `blender/objects/fire.blend`: fire samples to link, one collection each (`run.ps1 object fire`): the lamp flame, Renuka's cooking fire for the luweng, a campfire, smouldering embers (S5), a big fire for the pyre (S6); with sparks in Geometry Nodes ([objects/fire.md](../objects/fire.md)).
  - `blender/buildings/pendopo.blend`: the pendopo, a joglo, in its own file (`run.ps1 building pendopo`), linking the stone.
  - `blender/places/jatisrana.blend`: the hermitage, linking the pendopo (`run.ps1 place jatisrana`):
    - terrain, lake and layout
    - detailed huts, pawon, sanggar pamujan, gate, kandhang and lake steps
    - real plants (banyan, bamboo, banana, kamboja, jasmine, lotus, glagah, reeds, tree ferns) and grass
    - Poly Haven textures
    - the S1 dressing
  - Builds overwrite the real files: no test copies, no `.blend1` (the user, about disk space).
- **Designed**: the model list ([models.md](../models.md)) and the environment ([environment.md](../environment.md)).
- **Researched**: the pendopo ([buildings/pendopo.md](../buildings/pendopo.md)); the model was corrected against it (inner tumpang sari and nanasan, dudur, reng, sunduk and kili, padma umpak), its stair and soko built in chipped stone, its teak weathered after the wood reference, and its proportions fixed to the pakem soko (Kawruh Kalang umpak rule, Tamansiswa sizes).
- **Researched and built**: Jamadagni's lamp ([objects/damar.md](../objects/damar.md)), standing in the pendopo's sacred centre.
- **Next**: the pendopo's open choices (sacred centre, ornament, tiers, a stone-faced plinth); then the light states (milestone 5) or the story variants (milestone 6).

## Notes

- [2026-10-18-16: The fire samples (fire.blend)](2026-10-18-16-fire-samples.md)
- [2026-10-18-15: Fire after Max Hay's video, on the damar's flames](2026-10-18-15-fire-reference.md)
- [2026-10-18-14: The damar, Jamadagni's oil lamp, built](2026-10-18-14-damar-built.md)
- [2026-10-18-13: The stone pieces in their own file](2026-10-18-13-stone-objects.md)
- [2026-10-18-12: Research for the Javanese lamp (damar)](2026-10-18-12-damar-research.md)
- [2026-10-18-11: The pendopo's proportions, fixed to the pakem](2026-10-18-11-pakem-proportions.md)
- [2026-10-18-10: The soko's wood, textured correctly](2026-10-18-10-teak-grain.md)
- [2026-10-18-09: Pakem soko, the post sizes](2026-10-18-09-pakem-soko.md)
- [2026-10-18-08: The soko separated, their umpak in chipped stone](2026-10-18-08-soko-separate-stone.md)
- [2026-10-18-07: The pendopo's stair, in chipped stone](2026-10-18-07-stone-stair.md)
- [2026-10-18-06: References for doing it by hand](2026-10-18-06-references.md)
- [2026-10-18-05: Pendopo corrected against the research](2026-10-18-05-pendopo-corrections.md)
- [2026-10-18-04: Pendopo research](2026-10-18-04-pendopo-research.md)
- [2026-10-18-03: The hermitage in detail (buildings, plants, grass, textures)](2026-10-18-03-hermitage-detail.md)
- [2026-10-18-02: The hermitage, first build (setting and models)](2026-10-18-02-hermitage-first-build.md)
- [2026-10-18-01: Research, model breakdown, environment design](2026-10-18-01-research-and-design.md)
