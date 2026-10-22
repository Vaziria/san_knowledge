# Bargawa: Blender production of the Ramabargawa story

Code, Blender files and renders for the Ramabargawa story. The design lives in the knowledge docs:
- [docs/stories/ramabargawa/models.md](../../docs/stories/ramabargawa/models.md): every model the story needs
- [docs/stories/ramabargawa/environment.md](../../docs/stories/ramabargawa/environment.md): the hermitage (Pertapan Jatisrana)
- [docs/stories/ramabargawa/development_progress/](../../docs/stories/ramabargawa/development_progress/index.md): what has been done, session by session

Look: stylized cinematic, focused on grief and sorrow. Sources: Blender scripts and Poly Haven (CC0); no Meshy.

To do it by hand: [references/](references/README.md) has step-by-step guides with the scripts' numbers (modelling the pendopo, texturing with Poly Haven), and summaries of external references such as videos.

## Layout

```
project/bargawa/
  scripts/
    run.ps1                 the jobs (headless Blender)
    common/                 shared code (see common/__init__.py for the list)
    textures/build.py       builds blender/textures/<family>.blend, the texture library (wood, stone, plants, ground, metal,
                            palette; the families in common/textures.py); every other file links its materials from it
    objects/stone.py        builds blender/objects/stone.blend (the chipped stone pieces: umpak, stair; buildings link them)
    objects/soko.py         builds blender/objects/soko.blend (the pendopo's teak posts, soko guru and soko penanggap; linked)
    objects/damar.py        builds blender/objects/damar.blend (Jamadagni's oil lamp; the place links it)
    objects/fire.py         builds blender/objects/fire.blend (fire samples: lamp, hearth, campfire, embers, large; to link)
    buildings/pendopo.py    builds blender/buildings/pendopo.blend (a building in its own file; places link it)
    places/jatisrana.py     builds blender/places/jatisrana.blend (the hermitage, linking the pendopo and the damar)
    tools/views.py          review stills (temporary cameras, nothing saved)
    tools/polyhaven.py      downloads the Poly Haven textures
    game/jatisrana.py       the game's copy of the place: settings per kind (thinning, colliders, draw ranges), the walkable area
  game/                     the Godot 4.7 project of the game (docs/summaries/story.md); see "The game" below
  tools/godot/              the Godot editor (git-ignored; downloaded, see below)
  blender/textures/         the texture library: built .blend files (git-ignored), each with its summary .md beside it, and
                            blender_assets.cats.txt (the Asset Browser's catalogs Bargawa/Wood ...)
  blender/objects/          built object files (git-ignored; rebuild with run.ps1)
  blender/buildings/        built building files (git-ignored; rebuild with run.ps1)
  blender/places/           built place files (git-ignored)
  blender/assets/polyhaven/ downloaded textures (git-ignored; the list is common/polyhaven.py)
  blender/assets/photos/    images the user picked (kept in git): ground_spiky.jpg (the spiky ground), sasak.jpg (the thatch),
                            1cb0165c04eb35eb7bd78df3f83cc534.jpg (the soil)
  renders/checks/<name>/    review stills and contact sheets (git-ignored)
  _logs/                    full logs of every job (git-ignored)
```

## Commands

Blender 5.2 at `D:\blender\blender.exe` (or `$env:BLENDER`). Run from PowerShell, in this order the first time:

```powershell
project\bargawa\scripts\run.ps1 fetch                        # download the Poly Haven textures (once; kept if there)
project\bargawa\scripts\run.ps1 texture all                  # build the texture library, blender\textures\*.blend (every file links it)
project\bargawa\scripts\run.ps1 register                     # once, Blender closed: the library in the Asset Browser ("Bargawa Textures")
project\bargawa\scripts\run.ps1 object stone                 # build blender\objects\stone.blend (the stone pieces)
project\bargawa\scripts\run.ps1 object soko                  # build blender\objects\soko.blend (the pendopo's posts)
project\bargawa\scripts\run.ps1 object damar                 # build blender\objects\damar.blend (the oil lamp)
project\bargawa\scripts\run.ps1 object fire                  # build blender\objects\fire.blend (the fire samples)
project\bargawa\scripts\run.ps1 building pendopo             # build blender\buildings\pendopo.blend (links the stone and the soko)
project\bargawa\scripts\run.ps1 place jatisrana              # build blender\places\jatisrana.blend (links the pendopo)
project\bargawa\scripts\run.ps1 views jatisrana -Pct 40      # review stills + sheet.png -> renders\checks\jatisrana\
project\bargawa\scripts\run.ps1 views pendopo -Building      # review stills round a building file
project\bargawa\scripts\run.ps1 views stone -Object          # review stills of an object file
```

The .blend files are generated: change the scripts, not the files. Builds overwrite them in place, with no test copies and no
`.blend1` backups (disk space). The runner refuses to overwrite a file open in Blender; `-Force` goes on anyway, and the open
window then needs File > Revert (saving it would put the old version back).

## The game

The Android game of Chapter 1 follows the workflow in [bargawa_game.md](bargawa_game.md). Its requirements are in [docs/general.md](docs/general.md) (the user's file). Its design is in [docs/summaries/](docs/summaries/story.md) and [preview/design.html](preview/design.html).

- **Engine**: Godot 4.7.2 with the Mobile renderer, in `tools/godot/` (git-ignored). To get it, download `Godot_v4.7.2-stable_win64.exe.zip` from the [4.7.2-stable release](https://github.com/godotengine/godot/releases/tag/4.7.2-stable), check it against the release's `SHA512-SUMS.txt`, and unzip it there. An empty `._sc_` file next to the exe keeps the editor's settings in that folder.
- **The place in the game**: `run.ps1 game jatisrana` writes `game/assets/export/jatisrana/` (git-ignored) from the built place, with `common/game.py` and `scripts/game/jatisrana.py`:
  - one GLB with each distinct mesh once
  - a JSON file with the placements, kinds, colliders, draw ranges, sun, sky, walkable area and start
  - leaves and grass thinned; textures mixed with their palette colour, on UVs made by the export
- **In Godot**, `game/scripts/place.gd` draws repeated meshes as MultiMeshes in 16 m chunks and adds the colliders. The ground has its own collision layer.

From `project/bargawa` (or `make -C project/bargawa <target>` from the repo root). Every target is one job of `scripts\run.ps1`, so `run.ps1 play` and the rest work without make too:

```powershell
make play          # play: WASD, mouse to look (click; Esc frees it), F3 numbers; exports and imports first if the place changed
make export        # the place's game copy (run.ps1 game jatisrana), then Godot's import
make check         # import, then a short headless run; errors only
make measure       # walk a set route: _logs\game_measure.json and renders\checks\game\*.png
make editor        # the Godot editor
make clean-cache   # delete game\.godot (the next import rebuilds it)
```

## Conventions

- Axes: x east, y north, z up, metres. The pendopo's centre is the origin; the terrace is at z 0, the lake at z -4.
- `common/jatisrana.py` is the one source of where everything stands; builders and later scenes read it.
- A building in its own file sits at the origin facing south in a collection named after it; places link that collection.
- An object file (`blender/objects/`) holds pieces that buildings reuse, each in its own collection, marked as an asset, with
  its instance offset at the piece's base, so a linked instance stands where its empty is. Build order: textures, objects,
  buildings, places.
- Materials come from the texture library (`blender/textures/`, one file per family, linked by `mesh.mats`): change a texture
  in `common/textures.py` or `common/polyhaven.py`, rebuild its family, then what uses it. Only special materials stay in their
  own files (the fire shaders, the review ground).
- Textures are box-projected in object space (`common/textures.py`), except timber, which gets UVs along each piece's grain (`mesh._grain_uvs`).
- Story cameras are not part of the place (the user, 2026-10-18: "we talk direct camera later"). Review cameras live only in `tools/views.py`.
- Each script starts with the `sys.path` line and `common.reload()` (see `common/__init__.py`).
