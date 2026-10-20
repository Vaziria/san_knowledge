# 2026-10-18: The soko separated, their umpak in chipped stone

## Decisions (the user's words)

| Decision | The user said |
|---|---|
| The two kinds of soko as separate parts | "so we have 2 soko ? separate it" |
| Their stone in the reference's style | "and refine stone in soko like our reference" |

Before this, the user asked which kinds of soko exist. Answered from the sources:
- **soko guru**: the 4 in the centre
- **soko penanggap**: the middle ring
- **soko emper**: the outer ring of a three-tier joglo
- **soko bentung**: hanging posts in a Joglo Lambang Gantung, from Wismantoro et al. 2023

Our two-tier Joglo Lawakan has the first two: 4 + 12 = 16. The soko emper and bentung, with sources, were added to [buildings/pendopo.md](../buildings/pendopo.md).

## Done

- **Separate objects**, each in its own collection inside `Pendopo`:
  - `Pendopo_SokoGuru`: 4 posts and their padma umpak
  - `Pendopo_SokoPenanggap`: 12 posts and their umpak

  The pendopo is now six objects: `Pendopo`, `Pendopo_SokoGuru`, `Pendopo_SokoPenanggap`, `Pendopo_Stair`, `Pendopo_Roof`, `Pendopo_Ridge`. The names were unified from "Pendapa_" to "Pendopo_".
- **Chipped stone umpak** (`common/stone.py`):
  - `stone.umpak()`: a truncated pyramid of chipped stone.
  - `stone.padma()`: a chipped square foot, the two rows of petals with a little irregularity, and **wear on the petal tips**, so the lotus reads.
  - Both use the `stone_kit` material (worn edges lighter), smooth with sharp edges from 30°.
- **`stone.block()`** gained `top_scale` (a truncated pyramid) and `chip_span`. Chips are now cut **before** the taper, so they land on the real edges of a tapered stone. Cutting after the taper would have missed the top edges.
- The plain old padma loft in `joglo.py` was removed.
- Guide 01, Part 2: the separate objects and the stone look by hand.

## Checks

Review stills `renders/checks/pendopo/`: `umpak` (a soko guru's padma), the new `umpak_pen` (a soko penanggap's umpak), `stair_close` and `front`.
- The petals now stand out clearly.
- The plain umpak match the stair stones.

13,032 faces, against 9,200 before. `pendopo.blend` was rebuilt with `-Force`.

## Open

The plinth's facing and coping are still crisp boxes next to the chipped stair and umpak.
