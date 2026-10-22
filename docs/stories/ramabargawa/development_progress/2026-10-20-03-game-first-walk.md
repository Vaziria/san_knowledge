# 2026-10-20: The game's first walk (milestone 1)

## Request (the user's words)

| Request | The user said |
|---|---|
| Accept the design | "i have reviewed, i accept it" |

The workflow moved to "Game Implementation", and milestone 1 of the [design preview](../../../../project/bargawa/preview/design.html) started: Godot in `tools/`, the `game/` project, `run.ps1 game jatisrana`, a stand-in walking the hermitage in S1, the frame rate measured.

## Done

- **Godot 4.7.2** (the latest stable, 18 Aug 2026), downloaded from its GitHub release, checked against `SHA512-SUMS.txt`, and unzipped into `project/bargawa/tools/godot/`. The folder is git-ignored; `._sc_` keeps its settings there.
  - Not downloaded yet: the 1.28 GB export templates. They are needed only for the Windows preview build (milestone 6), and the disk is tight.
- **The export**, `run.ps1 game jatisrana` (new job): [common/game.py](../../../../project/bargawa/scripts/common/game.py) and [scripts/game/jatisrana.py](../../../../project/bargawa/scripts/game/jatisrana.py). It reads the built place without saving it, and writes `game/assets/export/jatisrana/` (git-ignored) in 12 seconds:
  - **The GLB** (10.7 MB) has each distinct mesh once (60 of them).
  - **The JSON file** has:
    - where each mesh stands, in Godot axes, and its kind, collider and draw range
    - the sun and the sky
    - the walkable area: the clearing and the slope to the lake, not the water; the steps' foot and the bathing stone are in
    - where the player starts: in front of the sons' hut
  - **Thinning**: leaves, blades and plumes (small loose parts of leafy materials) are thinned, and the kept ones grow a little. The bamboo also loses its node rings and 1 cm twigs, which can't be seen at game distances. A clump goes from 54k to 7.8k triangles.
  - **Other cuts**:
    - the banyan's crown: 114k → 42k
    - the grass: 40% of the clumps, drawn within 26 m
    - the terrain: cropped to the forest's ground, 80k → 38k
    - the damar's flame cards: left out until the game's own fire (milestone 5)
  - **The whole valley**: 1.62 million triangles in the film, 0.83 million in the game.
  - **Textures**: each Poly Haven texture is mixed with its palette colour as the Blender shader mixes them, baked into one image. The UVs come from the timber's grain UVs, or a box projection in object space. Bump, wear and weathering layers are left out for now.
- **The Godot project** `project/bargawa/game/`:
  - `project.godot`: Mobile renderer, 1280×720 landscape, 3D at 0.75 scale, Jolt physics, ETC2/ASTC textures for Android.
  - `scripts/place.gd`:
    - draws repeated meshes as MultiMeshes in 16 m chunks; single meshes keep Godot's LODs
    - adds colliders: trimesh for the ground and the buildings, cylinders for trunks and bamboo
    - the ground has its own collision layer
    - a stand-in water material
  - `scripts/player.gd`:
    - the stand-in Ramabargawa, 1.90 m
    - third-person camera on a spring arm
    - walks at 1.6 m/s, never runs
    - climbs single steps, and keeps to the walkable area
  - `scripts/main.gd`:
    - the S1 light from the place's sun and sky, with AgX and a little mist
    - starts him in front of the sons' hut, facing south
    - F3 shows the frame rate, draw calls and triangles
  - `scripts/measure.gd` (`-- --measure`): an autopilot walks a set route and records the numbers into `_logs/game_measure.json`. The route goes from the sons' hut past the elders' hut, the sanggar and the gate, round the pendopo, down the lake steps to the bathing stone, and back up past the stump to the pawon. It takes stills into `renders/checks/game/`.
- **Docs**:
  - the project README, "The game" section
  - the preview's state and milestone 1
  - `common/__init__.py`

## Checks

The final run, on this PC (AMD Radeon iGPU, 512 MB, 7.4 GB RAM, the user's Blender open), at 1280×720 with 3D at 0.75 and vsync off:

| | Average | Worst / 1% low |
|---|---|---|
| Frame rate | 128 fps | 1% low 108 fps |
| Draw calls | 337 | 433 |
| Triangles in view | 437k | 544k |
| Memory | 169 MB video, 102 MB static | |

- The target was 30 fps, so there is plenty of room for characters, the light states and effects. The triangles in view are above the design's 350k guess, but the frame rate says it doesn't matter on this GPU. A phone may differ: check on the first Android build.
- **One stall of about 150 ms** came once on the route, as in the warm-up. It is most likely a shader compiled the first time a material comes into view. To do: warm the materials up behind the opening fade (milestone 3).
- **Fixed along the way**:
  - The spawn ray hit the porch roof and he started on the roof. Now the ground has its own collision layer and the ray only looks for it.
  - The route ran into the gate's fence wings and the woodcutter's stump. Both are real obstacles, so the route goes round them.
  - The chipped stone's wear mix was taken for its texture mix. Now only the mix fed by the image counts.
- The five stills (`start`, `sanggar`, `steps`, `lake`, `pawon`) show the place reading as in the Blender stills:
  - the thatch, the shrine, the banyan's roots, the lake with lotus and glagah, the pawon's luweng and pots
  - the pendopo's plinth is plain grey, as `rock_surface` is at its tile size in Blender too

## Decisions

| Decision | Who |
|---|---|
| Godot 4.7.2 in `tools/godot/`, git-ignored, as in the accepted design | the design |
| The export writes generated files only; nothing in `game/assets/export/` is edited or committed | Claude, after the rule for the .blend files |
| Grass at 40% within 26 m, bamboo leaves at 15%, banyan leaves at 35% | Claude; first numbers, kept since the frame rate allows |

## Open questions

These open questions stay on their proposals until the user answers them:
- **Before milestone 2**: the **animation source** (Mixamo plus Blender, or Blender only), and **Ramabargawa's skin**.
- The other open points: the curse, Renuka's memory, beat 3, why Jamadagni is killed, the Javanese lines, the title.

## Next

- **Milestone 2, the first character**:
  - test MPFB 2 on Blender 5.2
  - Ramabargawa's body, costume and game rig, exported to Godot and walking in place of the stand-in
  - the face test: expressions as shape keys
- **Later**: warm the shaders up behind the opening fade; the touch controls (milestone 3).
