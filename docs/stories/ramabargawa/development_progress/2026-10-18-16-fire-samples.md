# 2026-10-18: The fire samples (fire.blend)

## Request (the user's words)

| Request | The user said |
|---|---|
| Talk about fire | "lets talk about fire" |
| The fire reference again | "i have reference for creating fire https://www.youtube.com/watch?v=azM_ZlToG-M&t=29s": the same video as note 15 |
| A file of fire samples | "can we create objects/fire.blend to create fire collection sample ?" |

The reference was already summarised and applied to the damar (note 15). This step applies the parts still unused, for bigger fires: several flame planes, sparks, a light the fire's size. It answers the open question from the talk: big fires use the video's cards, not Blender's fire simulation. The simulation would mean multi-GB bakes, and the disk is tight.

## Done

- **Research**: [objects/fire.md](../objects/fire.md), named after the file.
  - What real fire does:
    - Heskestad's mean flame height
    - the continuous, intermittent and plume zones
    - the puffing rate 1.68 / √D
    - flame, ember and spark colours
    - the Javanese hearth (pawon from *awu*, ash; sticks pushed into the luweng's fire mouth)
  - The sizes of each sample come from it.
  - Two of my own web searches, no research agents.
- **`blender/objects/fire.blend`** (`run.ps1 object fire`): five collections, all assets.
  - `Fire_Lamp`: the damar's flame.
  - `Fire_Hearth`: Renuka's cooking fire, for one fire mouth of the luweng.
  - `Fire_Wood`: a campfire.
  - `Fire_Embers`: smouldering, for S5.
  - `Fire_Large`: a big log fire, for the pyre in S6.
- **Code**:
  - `common/fire.py` gains:
    - `fire_material`: a wood fire on cards, from rising turbulence over an envelope fading with height, added light
    - `fire_piece`
    - `flicker` with a rate and a sway
    - `sparks`: Geometry Nodes from the time alone, no bake, works when linked
    - `ember_material`
    - `charred`
  - `common/fuel.py` (new): logs, coals, the sizes (`SAMPLES`).
  - Palette `firewood` with Poly Haven `bark_brown_01` (fetched; 1.1 MB).
  - `views.py`: the `fire` view set.
  - The lamp flame's code was only tidied (a shared time helper); its nodes are the same.
- **Docs**:
  - guide [08-fire-samples.md](../../../../project/bargawa/references/08-fire-samples.md), by hand
  - references/README.md
  - video-fire-max-hay.md ("What we can use")
  - environment.md (effects)
  - project README

## Checks

Stills in `renders/checks/fire/`: `all`, `all_night`, `wood`, `wood_night`, `wood_f12`, `hearth_night`, `embers_night`, `large_night`, `large_f12`. Three rounds; what was fixed is in objects/fire.md, "Built". Also:
- **Drivers**: every light's flicker and sway is a simple expression, so it runs without Python scripts allowed.
- **Linked**: `Fire_Wood` linked twice into an empty file gives each copy its logs, coals, flames, light and 60 sparks. The sparks are the same in both copies (Geometry Nodes run per object); the flames differ.
- **Blender 5.2**: a Geometry Nodes modifier's inputs are set through `modifier.properties.inputs.<Socket_N>.value` now. `fire.sparks` tries the old way first.

## Next

- Put `Fire_Hearth` in the pawon's luweng (S1) with the light states (milestone 5). Add smoke from it, and from the incense.
- The damar could link `Fire_Lamp` from fire.blend instead of keeping its own flame.
- S6: whether the pyre is shown is still open. If it is, `Fire_Large` goes on a pyre model.
