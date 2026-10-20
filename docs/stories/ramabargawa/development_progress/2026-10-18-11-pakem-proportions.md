# 2026-10-18: The pendopo's proportions, fixed to the pakem

The open question from [note 09](2026-10-18-09-pakem-soko.md) (chunky posts, low umpak) was decided and built.

## Decision (the user's words)

| Decision | The user said |
|---|---|
| Bring the soko to the pakem | "yes, fix proportion" |

## Done

`scripts/common/joglo.py`, its constants now derived from the pakem (comments cite [buildings/pendopo.md](../buildings/pendopo.md), "Pakem soko"):

| | Before | Now | Rule |
|---|---|---|---|
| Soko guru section | 0.30 m | **0.18 m** (tapering to 0.17) | slimmer, near the proposal's 20 cm |
| Soko guru umpak | 0.3 m, a squat padma | **0.72 m**, a padma pedestal | = the post's perimeter (Kawruh Kalang) |
| Soko penanggap section | 0.23 m | **0.13 m** (to 0.12) | near the proposal's 14 cm |
| Soko penanggap umpak | 0.3 m | **0.26 m** | = half the perimeter |
| Soko penanggap ring | ±4.0 m | **±4.7 m** | Tamansiswa's spacing (3.1 : 3.2 against 5 : 6) |
| Soko penanggap top | 3.0 m above the floor | **2.45 m** | soko guru : penanggap 1.63, Tamansiswa 1.74 |
| Penanggap roof eave | half-size 5.6 at 2.35 m above the floor | **6.0 at 2.0 m** | the eave height is computed so the roof passes just over the ring's blandar |
| Beams | ring blandar 0.20 × 0.22, guru blandar 0.24 × 0.24, sunduk 0.16 × 0.18, tie beams 0.16 × 0.2 | **0.15 × 0.20, 0.22 × 0.24, 0.165 × 0.22, 0.12 × 0.17** | slimmer with the posts; the sunduk ¾ of the blandar on one side (Kawruh Kalang) |

Unchanged: the floor (0.6 m, as Tamansiswa), the soko guru frame (3.2 m), the soko guru tops (4.0 m above the floor), the tumpang sari, the brunjung and the ridge.

- **`stone.padma`** was rewritten for a tall umpak: a chipped square foot (7 cm), then, when the umpak is taller than about 0.45 m, a plain chipped **shaft (dado)**, then the lotus band (up to 24 cm: two staggered rows of eight petals and a neck) and the seat. It takes `base` and `seat` widths, so the pedestal fits the post (0.56 m foot, 0.21 m seat).
- `stone.umpak` takes the penanggap's sizes: 0.30 → 0.20 m, 0.26 tall.

**Not used**: the Kawruh Kalang's "soko guru length = the frame's diagonal". It would make the soko guru 4.5 m on a 0.72 m pedestal and lift the whole centre and roof by about 1.2 m, a tower over the small frame; Tamansiswa doesn't follow it either (6.25 m posts against a 7.8 m diagonal).

**Kept a little sturdier than Tamansiswa**: the posts are 1 : 18 against its 1 : 28 (strictly scaled they would be about 14 cm). The stylized look keeps some weight, and the posts still read at the story's camera distances.

## Checks

- `run.ps1 building pendopo -Force`: 12,272 faces.
- Review stills `renders/checks/pendopo/`: `front`, `side`, `inside`, `umpak` (the pedestal: foot, shaft, lotus, seat), `umpak_pen`, `soko_wood`. Slender posts, the outer ring lower and further out, the penanggap roof passing just over its blandar.
- `run.ps1 place jatisrana -Force`, stills `renders/checks/jatisrana/`: `pendapa`, `overview`, `from_lake`. The pendopo still fits its place among the huts and trees.
- `pendopo.blend` and `jatisrana.blend` were overwritten: File > Revert in any open window.

## Docs updated

- [buildings/pendopo.md](../buildings/pendopo.md): "Our pendopo against the pakem" (before and now, and why), "What the model is now", the research table.
- Guide [01-modelling-pendopo.md](../../../../project/bargawa/references/01-modelling-pendopo.md): the plan, the sizes table, Part 2 rewritten (the pedestal and the slimmer posts by hand, with the reasons), Part 3 beams, Part 5 roof, Part 6 rafter, the check list.

## Next

- The pendopo's other open choices: the sacred centre in the story, modest ornament (tlacapan, praba), two or three tiers.
- The plinth's facing in stone blocks, like the stair.
- The light states (milestone 5).
