# 2026-10-18: The stone pieces in their own file

## Decision (the user's words)

| Request | The user said |
|---|---|
| Move the stone into its own file | "separate stone in objects/stone" |

## Done

- **A new kind of file, objects**: `scripts/objects/stone.py` builds `blender/objects/stone.blend` (`run.ps1 object stone`). Its collection `Stone` holds one collection per piece, each marked as an **asset**:

  | Piece | What |
  |---|---|
  | `Umpak_Guru_1` … `_4` | the padma pedestals of the soko guru (0.72 m) |
  | `Umpak_Penanggap_1` … `_4` | the plain umpak of the soko penanggap (0.26 m) |
  | `Stair_Pendopo` | the south steps, its origin at the middle of its back edge |

  - The pieces stand in rows for review, small to tall from the front.
  - Each collection's **instance offset** is its piece's position, so an instance shows the piece at its empty.
  - The file has its own `_Review` ground, sun and sky.
- **The pendopo links them** (`joglo._stones`, `mesh.link_collections`): an instance under every post (`Pendopo_UmpakGuru_1..4`, `Pendopo_UmpakPenanggap_1..12`) and `Pendopo_Stair`.
  - The four plain umpak are used three times each, turned a quarter further each time, so neighbours differ: a modular kit, as in the stone video.
  - `Pendopo_SokoGuru` and `Pendopo_SokoPenanggap` are now the wooden posts only.
  - The stone's sizes still come from the pendopo's pakem constants (`joglo.stone_pieces`).
  - `pendopo.blend` went from 577 KB to 225 KB.
- **Code**:
  - `stone.stair` returns Parts (built with `STAIR_BUILD`).
  - `paths.object_file`.
  - `run.ps1 object <name>` and `views <name> -Object`.
  - `views.py` `STONE_VIEWS`: all, padma, umpak, stair, top.
- Fixed the `umpak_pen` review camera, which still aimed at the ring's old ±4.0 m position.

## Checks

- `run.ps1 object stone`: 9 pieces, 8,936 faces. Stills `renders/checks/stone/`: every piece reads; the first layout hid the small umpak behind the stair, so they moved to the front row.
- `run.ps1 building pendopo -Force` and stills `front`, `umpak`, `umpak_pen`, `soko_wood`, `stair`, `stair_close`: every umpak sits under its post, the stair against the plinth, the same as before the split.
- `run.ps1 place jatisrana -Force`, stills `pendapa`, `pendapa_inside`: the two-level link (place → pendopo → stone) works.
- **The user's open window**: before this rebuild, `pendopo.blend` had been saved at 16:40 from the user's open Blender window. That save held the **old proportions** (the window was opened before note 11's rebuild and never reverted); no other change was found when comparing it with the build. The rebuild replaced it. Its `.blend1` backup was deleted (disk space).
- Use File > Revert in any open window, and don't save there.

## Docs updated

- Guide [01-modelling-pendopo.md](../../../../project/bargawa/references/01-modelling-pendopo.md):
  - Part 1b: the stair is a stone piece.
  - Part 2: "The stone file", with how to link and reload by hand.
  - The check list.
- [buildings/pendopo.md](../buildings/pendopo.md): "What the model is now".
- [project/bargawa/README.md](../../../../project/bargawa/README.md): the layout, the commands, an object-file convention, the build order (objects, buildings, places).

## Next

- The lamp, once the user picks the option and its file (note 12).
