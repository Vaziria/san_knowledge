# Ramabargawa: Environment Design, Pertapan Jatisrana

The design of the first environment, before anything is built: the hermitage, the lake and the forest (sets E01–E03 in [models.md](models.md)), where beats 1–3 take place.

**Decisions (the user, 2026-10-18):**
- **Stylized look**, focused on emotion: grief and sorrow ("we bring and focus on emotional, grief, and sorrow. make sure its developed well").
- **Sources**: Poly Haven and Blender, no Meshy ("forget meshy, polyhaven and blender", read as: forget Meshy, use Poly Haven and Blender).
- **Production** lives in `project/bargawa/`.

Everything below is a design proposal to review before building.

## Emotional intent

Jatisrana is the family. The audience first learns the place when it is warm and whole, so that later its emptiness hurts. The environment carries what the story doesn't show: the violence happens off-screen, and the place shows what it left behind.

### Principles
1. **One place, changing states.** Beats 1–3 return to the same spots and the same camera setups. Only the light, the weather and the objects change. The five mats in the pendapa are full, then tense, then empty and burned.
2. **Warmth is the family.** Each warm light source belongs to someone: Renuka's jasmine and kitchen fire, Jamadagni's incense and oil lamp, the hearth smoke of the sons. Each loss takes one away, and the palette gets colder. The last warm light in beat 3 is the funeral fire.
3. **Small people, big world.** A great mountain and a great banyan tree stand over the hermitage, so the people are always small. In grief shots the camera goes wider, not closer: Ramabargawa alone in a lot of empty space.
4. **Weather follows the heart, held back.** Mist, gold, overcast, moonlight. Rain falls only once, at the peak of grief, so it means something.
5. **Stillness speaks.** At the worst moments the wind stops, the bamboo goes quiet and the birds stop. This is a note for the sound too.
6. **Objects remember.** Violence is shown through what remains: a dropped kendi and its water soaking into the earth, lotus petals on dark water, scattered prayer beads, smoke.

## Colour script: seven light states

Each state is one switchable set-up in the place file: sky, sun, atmosphere and colour grade.

| State | Name | Beat | Light and weather | Palette | Feeling |
|---|---|---|---|---|---|
| S1 | Embun pagi (morning dew) | 1, family | low warm sun through mist, long soft shadows, dew | sage green, cream, honey gold | home, safety, love |
| S2 | Sore keemasan (golden afternoon) | 1, Renuka at the lake | low sun across the water, glitter on ripples, warm haze | saturated gold, amber, deep green | beauty with unease |
| S3 | Mendung (overcast) | 2, the order | heavy grey sky, flat light, wind rising in the bamboo | grey-blue, drained green, no gold | dread |
| S4 | Malam bulan (moonlit night) | 2, after the boons | cold moonlight, one small oil lamp in the pendapa | blue-black, one warm point | the family together but apart |
| S5 | Abu (ash) | 3, the return | white sun through smoke, ash falling, the pendapa burned | grey, charcoal, sickly pale yellow | shock, silence |
| S6 | Hujan malam (night rain) | 3, the grief | rain on the lake, darkness, the only light is the funeral fire | deep blue-black, firelight orange | grief at its peak |
| S7 | Fajar merah (red dawn) | 3, the vow | the rain stops, clouds burn red over the mountain, the hermitage in silhouette | red, black, ember orange | the vow; grief turned to rage |

The states go from warm to cold and back to a hot red. That red is not warmth returning; it is rage. It leads into beat 4, the purge.

## Symbols in the place

Proposals, each tied to a person or a loss:

| Symbol | Belongs to | How it changes |
|---|---|---|
| **Beringin** (banyan) between the pendapa and the lake | the family; in Java the banyan stands for shelter and protection (pengayoman) | S1 full and sheltering; S5 scorched on one side; S6 Ramabargawa kneels between its hanging roots, which close around him like bars |
| **Lotus** on the lake | Renuka | S1–S2 in bloom; S3 closed; S6 loose petals drifting on dark, rain-pocked water |
| **Kendi** (clay water jar) | Renuka | full in S1–S2; dropped and broken in beat 2, its water soaking into the ground |
| **Jasmine bush** by the huts | Renuka (jasmine in her hair) | flowering in S1; flowers fallen in S4 |
| **Oil lamp** (damar) in the pendapa | Jamadagni, the household | its four flames all lit in S1; only one left in S4, the only light; knocked over and out in S5 ([objects/damar.md](objects/damar.md)) |
| **Incense** at the sanggar pamujan | Jamadagni | smoke rising straight in S1; burned out and cold in S5 |
| **Japamala** (prayer beads) | Jamadagni | in his hand; scattered on the shrine steps in S5 |
| **Kamboja** (frangipani) tree by the shrine | death; in Java the frangipani grows in graveyards | a few white flowers in S1, easy to miss; they fall around the body in S5–S6 |
| **Five mats** in the pendapa | the five sons | full in S1; Ramabargawa's mat apart from the others in S4; empty and burned in S5 |
| **Woodcutter's axe** on a stump | foreshadowing | an ordinary axe in S1, in the same spot where Ramabargawa later sets down the kapak |
| **Glagah** (wild white plumes) by the lake | mourning | moving in the wind; white plumes against the dark in S6 |

## Layout

The hermitage sits on a gentle rise at the foot of a mountain, in a clearing in the forest, above a small lake. The mountain is to the north and the lake to the south; the sun rises in the east and sets in the south-west, across the lake.

```
                         N   (mountain, far)
      ~~~~~~~~~~~~~~ bamboo grove ~~~~~~~~~~~~~~ tree ferns
                                                          
   [sanggar pamujan]     [hut: Jamadagni   [hut: sons]   [kandhang:
    + kamboja tree        & Renuka]                       livestock]
     (faces east)    jasmine                [pawon: kitchen, smoke]
          \________ path ________[  PENDAPA  ]________/
                                 5 mats, lamp
                gate ── path ──┘     |
                                 (BERINGIN)   woodcutter's stump
                                     |   stone steps down
     glagah   ~~~~~~~~~~~~~~~ TELAGA (lake) ~~~~~~~~~~~~~~~   reeds
              lotus     bathing stone      lotus
     far bank: forest, the place where Citrarata appears (south-west)
                         S   (sunset over the lake)
```

### Sizes (first guess, checked in the blockout)
- Clearing about 60 × 45 m on a slope of a few metres.
- **Pendapa**: about 9 × 9 m on a stone plinth, four soko guru (main pillars) on stone umpak (pillar bases), a joglo roof with a steep top.
- Huts: about 4 × 5 m, woven bamboo walls (gedhek), thatched roofs.
- Sanggar pamujan: a small andesite stone shrine on a stepped base, about 3 × 3 m.
- Beringin: canopy about 20 m across, trunk with hanging and pillar roots.
- **Lake**: about 40 × 25 m, 15–20 m below the pendapa, stone steps (undhak-undhakan) down to a flat bathing stone.
- The mountain is a distant backdrop, never reached.

## Key camera setups

Planned now so that the layout gives every shot a clear view. These are the setups the beats return to.

| Cam | View | Used for |
|---|---|---|
| K1 | **Establishing**: high on the slope to the north-east, looking south-west over the pendapa and banyan to the lake and the sunset | every state, to show how the place changes |
| K2 | **Pendapa interior**: from the back, looking south through the pillars to the banyan and the lake | the family on the five mats |
| K3 | **Lake steps**: low by the water, looking up to the pendapa, framed by banyan roots | Renuka with the kendi; the return |
| K4 | **Across the lake**: from the far bank (Citrarata's view), Renuka on the bathing stone, the pendapa above | S2 |
| K5 | **Under the banyan**: low among the hanging roots | the grief (S6) |
| K6 | **Sanggar pamujan**: facing east at sunrise | Jamadagni's meditation; the scattered beads |
| K7 | **Alone**: very wide or high, Ramabargawa tiny at the edge of the dark lake | the end of the grief; the vow at red dawn |

In S2 the sun is low in the south-west. From Renuka's shore looking south-west, Citrarata stands against the sun: a dark shape with a bright edge, beautiful and unreadable.

## Elements and sources

**PH** = Poly Haven (CC0, free). **BL** = built by script in Blender. The Poly Haven IDs are candidates from its catalogue, to preview before use.

### Terrain, water, sky
| Element | Source | Notes |
|---|---|---|
| Terrain | BL | sculpted by script: the rise, the slope down to the lake, the path ruts; ground textures from PH: `red_laterite_soil_stones` (Java's red-brown soil), `forest_ground_04/05/06`, `forest_leaves_02/03/04`, `brown_mud_leaves_01`, `mossy_rock`; `burned_ground_01` for S5 |
| Lake | BL | water shader (stylized: calm, dark, reflective), ripples, rain rings for S6, shore mud |
| Mountain backdrop | BL | a large far shape with haze; never close |
| Sky and light | PH | HDRI candidates per state: S1 `kloofendal_misty_morning_puresky`, `misty_pines`; S2 `kiara_7_late-afternoon`, `kiara_8_sunset`, `belfast_sunset_puresky`; S3 `kloofendal_overcast_puresky`, `approaching_storm`; S4 `qwantani_moonrise_puresky`, `rogland_moonlit_night`; S6 `qwantani_night_puresky`; S7 `kiara_1_dawn`, `klippad_dawn_1` |

### Buildings (all BL, no Javanese buildings on Poly Haven)
| Element | Notes | Textures (PH) |
|---|---|---|
| Pendapa | joglo frame (soko guru, tumpang sari ceiling, beams), stone plinth and umpak, thatched roof; **burned variant** for S5 | `thatch_roof_angled`, `reed_roof_03/04`, `riet_01`; wood `dark_wooden_planks`, `wood_table_worn`; stone `stone_pathway`, `mossy_stone_wall` |
| Huts (2) | woven bamboo walls, thatch, a small porch | `bamboo_wall`, `bamboo_wall_02/03`, `thatch_roof_angled` |
| Pawon (kitchen) | open shed, clay stove, smoke | `raw_plank_wall`, `bamboo_wall` |
| Sanggar pamujan | stepped andesite shrine with a niche | `rock_wall_*`, `dark_rock`, `mossy_stone_wall` (preview for a grey volcanic look) |
| Gate | a simple stone or bamboo gate on the path | as above |
| Kandhang (livestock pen) | bamboo fence, a roof over half | `bamboo_wall` |
| Funeral pyre | stacked logs by the lake for S6 | `bark_brown_01`, `wood_chips` |

### Vegetation (BL, except where PH fits)
| Element | Source | Notes |
|---|---|---|
| **Beringin** (banyan) | BL | the hero tree: a broad trunk, hanging roots down to the ground, dense small leaves; **scorched variant** for S5; bark candidates `bark_brown_02`, `tree_bark_03`, `bark_platanus` |
| Bamboo grove | BL | tall culms in clumps, moving and creaking in the wind |
| Banana plants | BL | big torn leaves by the huts |
| Kamboja (frangipani) | BL | a crooked tree with white flowers that can fall |
| Jasmine bush | BL | small white flowers that can fall |
| Lotus | BL | pads, buds, flowers, loose petals |
| Glagah and reeds | BL | white plumes at the lake edge |
| Tree ferns and ferns | BL + PH | PH `fern_02`, `moss_01` for the ground layer |
| Grass and undergrowth | BL + PH | PH `grass_medium_01/02`, `shrub_01–04` |
| Far forest | BL | a simple layer of tree shapes in the haze |
| Forest floor detail | PH | `tree_stump_01/02`, `root_cluster_01/02`, `dead_tree_trunk`, `dry_branches_medium_01`, `bark_debris_01` |
| Rocks | PH | `rock_moss_set_01/02`, `boulder_01`, `rock_07`, `rock_09`, `stone_01` |

### Set dressing (props, P11 in models.md)
| Element | Source | Notes |
|---|---|---|
| Oil lamp | BL | built: `blender/objects/damar.blend`, an Old Javanese bronze pole lamp (dipa) on a teak stand ([objects/damar.md](objects/damar.md)) |
| Kendi | BL | Renuka's jar; **broken variant** |
| Mats (5) | BL | woven pandan mats; texture candidate PH `tatami_mat`; **burned variant** |
| Pots, bowls, baskets | PH | `brass_pot_01/02`, `ceramic_pot`, `wooden_bowl_01/02`, `wicker_basket_01/02`, `wooden_bucket_01` |
| Broom | PH | `wooden_broom` |
| Dakon board | PH | `sungka_board` (the same mancala game as Javanese dakon): a sign of family life in S1 |
| Woodcutter's axe on a stump | PH | `wooden_axe` on `tree_stump_01` |
| Incense burner, offering trays | BL | at the sanggar pamujan |
| Japamala | BL | also a scattered version |

### Atmosphere and effects
Unlike the origami project, particles and volumes are allowed here: rain, ash and smoke may use them.

| Effect | States |
|---|---|
| Ground mist and haze | S1, S2 (light), S3 |
| Wind in leaves, bamboo and grass | all; strongest in S3; stops at key moments |
| Ripples on the lake | S1, S2; rain rings in S6 |
| Smoke from the pawon | S1 |
| Incense smoke | S1 |
| Smoke columns, falling ash, smouldering embers | S5 |
| Rain | S6 only |
| Fire (the pyre), sparks rising | S6 |

Fire follows Max Hay's video ([references/video-fire-max-hay.md](../../../project/bargawa/references/video-fire-max-hay.md)): flames drawn on cards, keyed into alpha and tinted, moving with time; a point light the fire's size; Volume Scatter haze; sparks from particles in a turbulence field. The damar's flames are done this way (`scripts/common/fire.py`), and so are the fire samples in `blender/objects/fire.blend` ([objects/fire.md](objects/fire.md)): Renuka's cooking fire for the luweng, a campfire, smouldering embers (S5) and a big fire for the pyre (S6), with sparks. Places link them; haze and motion blur come with the light states and the cameras.
| Falling flowers (kamboja, jasmine) and petals on water | S4–S6 |

## Stylized treatment

- Real light and real materials, but simplified forms: clean silhouettes, fewer and bigger leaf clusters, slightly exaggerated shapes (a heavier banyan, a taller pendapa roof).
- Poly Haven textures give surface detail, but each state's **colour grade** controls the palette, so the textures never decide the mood.
- Light shapes the feeling: one clear key light, soft fill, haze to separate the layers (near, middle, far).
- Grief scenes are low-key: deep shadows, few lights. The user prefers balanced light that is never washed out, and that fits here.

## Technical plan

### Folder (`project/bargawa/`)
```
project/bargawa/
  README.md
  blender/
    places/jatisrana.blend        built by script, not edited by hand
    assets/polyhaven/             downloaded Poly Haven files (CC0)
  scripts/
    common/                       paths, Poly Haven download, palette and states, vegetation and building builders
    places/jatisrana.py           builds the place
```
- The `.blend` files and Poly Haven downloads stay out of git; the scripts and a manifest of Poly Haven IDs and resolutions are tracked, so the place can be rebuilt.
- Poly Haven files are downloaded by script into `blender/assets/polyhaven/`, so a rebuild doesn't depend on clicking in the MCP.

### Budget for this PC (7.5 GB RAM)
- Textures at 1K, 2K only for hero surfaces seen close.
- All repeated vegetation as instances (Geometry Nodes), simpler versions far from the camera.
- Mist as light local volumes near the camera rather than one big world volume.
- Heavy jobs run headless with the user's Blender closed.

### Milestones
Each milestone ends with small check renders from the key cameras, never a full render unless the user asks.

1. **Blockout**: terrain, lake shape, grey boxes for buildings and trees, the seven key cameras. Review the layout and the shots.
2. **Buildings**: pendapa, huts, pawon, sanggar pamujan, gate, fences.
3. **Hero vegetation**: beringin, bamboo, banana, kamboja, jasmine, lotus, glagah.
4. **Ground, rocks and props**: Poly Haven textures and models, set dressing.
5. **Light states S1–S7** with atmosphere.
6. **Story variants**: the burned pendapa, the scorched banyan, the broken kendi, the scattered beads, the pyre, the lamp lit and out.

## Open questions

1. **Render engine**: EEVEE (fast, good for stylized looks and volumes) or Cycles (slower, but the office render farm can only run Cycles).
2. **Renuka's story version** changes beat 2: the classic version needs the dropped kendi at the lake; Rickyansyah's version (Citrarata kills her) needs a struggle at the lake instead.
3. **Funeral**: a cremation pyre by the lake (as proposed in S6), or does Ramabargawa grieve over the body under the banyan without a funeral on screen?
4. **The symbols** above are proposals: keep, drop or change any of them.
