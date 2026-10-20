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
    objects/stone.py        builds blender/objects/stone.blend (the chipped stone pieces: umpak, stair; buildings link them)
    objects/damar.py        builds blender/objects/damar.blend (Jamadagni's oil lamp; the place links it)
    objects/fire.py         builds blender/objects/fire.blend (fire samples: lamp, hearth, campfire, embers, large; to link)
    buildings/pendopo.py    builds blender/buildings/pendopo.blend (a building in its own file; places link it)
    places/jatisrana.py     builds blender/places/jatisrana.blend (the hermitage, linking the pendopo and the damar)
    tools/views.py          review stills (temporary cameras, nothing saved)
    tools/polyhaven.py      downloads the Poly Haven textures
  blender/objects/          built object files (git-ignored; rebuild with run.ps1)
  blender/buildings/        built building files (git-ignored; rebuild with run.ps1)
  blender/places/           built place files (git-ignored)
  blender/assets/polyhaven/ downloaded textures (git-ignored; the list is common/polyhaven.py)
  renders/checks/<name>/    review stills and contact sheets (git-ignored)
  _logs/                    full logs of every job (git-ignored)
```

## Commands

Blender 5.2 at `D:\blender\blender.exe` (or `$env:BLENDER`). Run from PowerShell, in this order the first time:

```powershell
project\bargawa\scripts\run.ps1 fetch                        # download the Poly Haven textures (once; kept if there)
project\bargawa\scripts\run.ps1 object stone                 # build blender\objects\stone.blend (the stone pieces)
project\bargawa\scripts\run.ps1 object damar                 # build blender\objects\damar.blend (the oil lamp)
project\bargawa\scripts\run.ps1 object fire                  # build blender\objects\fire.blend (the fire samples)
project\bargawa\scripts\run.ps1 building pendopo             # build blender\buildings\pendopo.blend (links the stone)
project\bargawa\scripts\run.ps1 place jatisrana              # build blender\places\jatisrana.blend (links the pendopo)
project\bargawa\scripts\run.ps1 views jatisrana -Pct 40      # review stills + sheet.png -> renders\checks\jatisrana\
project\bargawa\scripts\run.ps1 views pendopo -Building      # review stills round a building file
project\bargawa\scripts\run.ps1 views stone -Object          # review stills of an object file
```

The .blend files are generated: change the scripts, not the files. Builds overwrite them in place, with no test copies and no
`.blend1` backups (disk space). The runner refuses to overwrite a file open in Blender; `-Force` goes on anyway, and the open
window then needs File > Revert (saving it would put the old version back).

## Conventions

- Axes: x east, y north, z up, metres. The pendopo's centre is the origin; the terrace is at z 0, the lake at z -4.
- `common/jatisrana.py` is the one source of where everything stands; builders and later scenes read it.
- A building in its own file sits at the origin facing south in a collection named after it; places link that collection.
- An object file (`blender/objects/`) holds pieces that buildings reuse, each in its own collection, marked as an asset, with
  its instance offset at the piece's base, so a linked instance stands where its empty is. Build order: objects, buildings, places.
- Textures are box-projected in object space (`common/textures.py`), except timber, which gets UVs along each piece's grain (`mesh._grain_uvs`).
- Story cameras are not part of the place (the user, 2026-10-18: "we talk direct camera later"). Review cameras live only in `tools/views.py`.
- Each script starts with the `sys.path` line and `common.reload()` (see `common/__init__.py`).
