# 08: Fire samples (wood fire, embers, sparks)

Build by hand the fire samples that `scripts/objects/fire.py` builds into `blender/objects/fire.blend`. The code is in `scripts/common/fire.py` (flames, light, sparks, embers) and `scripts/common/fuel.py` (logs, coals, sizes). The numbers are the script's, in metres; this guide follows **Fire_Wood**, the campfire, and gives the other samples' numbers in tables. Research and reasons: [docs/stories/ramabargawa/objects/fire.md](../../../docs/stories/ramabargawa/objects/fire.md). The method is Max Hay's fire video ([video-fire-max-hay.md](video-fire-max-hay.md)).

New in this guide:
- a **wood fire drawn by its shader** on crossed cards: rising turbulence over a brightness that fades with height, so the flame is solid low down and breaks into tongues higher up
- the **Add Shader** with a Transparent BSDF: fire that adds its light to what's behind it
- **Geometry Nodes from an empty object**: sparks worked out from the time alone (Points, Random Value, Scene Time, Instance on Points, Store Named Attribute)
- **Voronoi › Distance to Edge**: glowing cracks in charcoal
- a **colour attribute painted by hand** that a material reads: how burned each part of a log is
- the flicker **driver** of guide 07, now at the fire's own rate, and a second driver that moves the light so its shadows sway

Before this guide: [07-modelling-damar.md](07-modelling-damar.md), Parts 7 and 8 (the lamp flame on cards, the time driver, the flicker driver).

## The plan

```
   side view of Fire_Wood                        what's in the collection
        .   *        sparks rise to ~1.6 m        Fire_Wood_Logs     6 leaning + 2 flat, charred towards the middle
      )  (    *      tongues break off            Fire_Wood_Coals    35 coals on a ragged bed of ash
     ( /\ )          up to ~1.0 m (2 L)           Fire_Wood_Flames   3 crossed cards, 0.56 wide, -0.015 -> 1.0 m
    ( /  \ )  0.50   mean flame height L          Fire_Wood_Flames_Light   150 W at z 0.18, radius 0.135, flickering
   (_/____\_) 0.03   the flames' root             Fire_Wood_Sparks   an empty mesh with the sparks' Geometry Nodes
    /_coals_\ 0      the ground
   |<-0.45->|        base D
```

| Sample | Where in the file | Base D | Flames L | Root z0 | Light | Flame strength | Cards | Sparks (count, radius, height, life, size, swirl) |
|---|---|---|---|---|---|---|---|---|
| Fire_Lamp | (−2.4, 0, 0.02) | the damar's flame: guide 07, Part 7 | | | 0.5 W | | 3 | none |
| Fire_Hearth | (−1.4, 0, 0) | 0.20 | 0.24 | 0.02 | 30 W | 1.8 | 3 | 14, 0.06, 0.5, 1.5, 0.0025, 0.08 |
| Fire_Wood | (0, 0, 0) | 0.45 | 0.50 | 0.03 | 150 W | 1.6 | 3 | 60, 0.15, 1.6, 2.5, 0.003, 0.25 |
| Fire_Embers | (1.4, 0, 0) | 0.50 | none | 0.02 | 12 W, low in the coals | | | none |
| Fire_Large | (5, 0, 0) | 1.40 | 1.80 | 0.10 | 900 W | 1.1 | 4 | 220, 0.5, 5.0, 4.0, 0.005, 0.8 |

Each sample's puffing rate (how often it breathes) is 1.68 / √D: 3.8 Hz for the hearth, 2.5 Hz for the wood fire, 1.4 Hz for the large one.

## Part 0: set up

1. **File › New › General**, delete the cube (X). **File › Save As** `blender/practice/fire_manual.blend`.
2. A ground to see the light on: **Add › Mesh › Cylinder** (Shift+A), Vertices 64, Radius 9, Depth 0.05; move it down so its top is at z 0 (G Z −0.025).
3. **Render Properties › Render Engine › EEVEE**; **Color Management › View Transform › AgX**.
4. To see the fire at night: **World Properties › Surface › Strength** 0.004 (as the review stills do), and hide the sun.

## Part 1: a log

What: a piece of firewood about 0.38 m long, a little uneven, like bark. Why a mesh rather than a curve: the material needs to know, at every point, how burned it is (Part 3), and you paint it (Part 2) that on the mesh's points.

1. **Add › Mesh › Cylinder**: Vertices 9, Radius 0.04, Depth 0.38 (one of the six leaning logs: 0.075–0.105 × D in radius). You see a short nine-sided post.
2. **Edit Mode** (Tab). **Edge › Loop Cuts** (Ctrl+R) along it, 6 cuts. Why: points along the length, so it can bend and taper.
3. Make the bark uneven: select all (A), **Mesh › Transform › Randomize** (in the operator panel, Amount 0.004, Normal 1). The section is no longer a perfect polygon.
4. Taper it: select the top ring (Alt+click an edge of it), scale it to 0.88 (S 0.88). The script makes each log 12% thinner at its far end.
5. **Object Mode** (Tab). Lean it: rotate it so its foot is 0.28 m out from the centre (R = 0.62 × D) and its top at about 0.07 × D from the centre, 0.28 m up (0.62 × D). Duplicate it (Shift+D) five times round the centre, 60° apart (R Z 60), varying each a little.
6. Two logs lie flat under them, radius 0.038 (0.085 × D), crossing: from (−0.25, −0.09, 0.035) to (0.23, 0.07, 0.04), and from (−0.11, 0.23, 0.08) to (0.09, −0.25, 0.07).
7. Join the eight logs into one object: select them all, **Object › Join** (Ctrl+J). Name it `Fire_Wood_Logs`.

## Part 2: how burned each point is (a painted attribute)

What: a value per point, 0 for bark, 1 for glowing charcoal, that the material reads. The script writes it as the float attribute `wear`. By hand, a **colour attribute** painted in Vertex Paint does the same: the Attribute node reads its greyness as `Fac`.

1. With the logs selected: **Object Data Properties › Color Attributes › +**, Name `wear`, Domain **Vertex**, Data Type **Color**, colour **black**. Black means bark.
2. Switch to **Vertex Paint** mode (the mode menu at the top left, or Ctrl+Tab).
3. Paint **white** where the fire is hottest: the logs' parts low in the middle. The script's rule:
   - fully burned within 0.05 × D of the centre (2 cm), fading to bark at 0.55 × D (25 cm)
   - the tips above the flames only half as burned, so they're black but don't glow
   - Brush strength 0.5, soft falloff. You see the logs grey to white towards the middle.

## Part 3: the firewood material, charred

What: Poly Haven bark that turns to charcoal where the logs are burned, and glows in its cracks where they're burned most.

1. The bark: as in guide 02. Poly Haven **bark_brown_01**, 1k, Box projection; **Mapping** Scale 1/0.4 = 2.5 (a 0.4 m tile: at 0.8 m its plates looked like wrinkled tubes on thin sticks); mixed 0.3 with the palette colour #604C3A; Bump strength 0.4.
2. **Add › Input › Attribute** (Shift+A, type "Attribute"), Name `wear`. Its **Fac** is how burned.
3. **Map Range** (Fac from 0.15 → 0.6, to 0 → 1, Smooth Step): how much charcoal.
4. **Mix Color**: A = the bark colour you had, B = charcoal **#1A1715**, Factor = step 3. Into **Base Color**.
5. The glow, only where Fac is above 0.6: a second **Map Range**, 0.6 → 1.0. Call it *heat*, and build the glow from Part 5 with Voronoi Scale 55. Into **Emission Color** and **Emission Strength** (glow × 4).

## Part 4: the coals and the ash bed

1. A coal: **Add › Mesh › UV Sphere**, Segments 7, Rings 4, Radius 0.022. Scale it irregular (S X 1.1, S Y 0.8, S Z 0.45), then in Edit Mode **Randomize** 0.003 as in Part 1. Low and lumpy, like a chunk of charcoal.
2. Scatter about 35 of them within 0.22 m (0.5 × D) of the centre:
   - Shift+D, then **Object › Transform › Randomize Transform**: Location ±0.15, Rotation Z 180°, Scale 0.5–1.4.
   - Heap them a little in the middle (up to 0.03 m).
   - Join them (Ctrl+J).
3. The ash bed under them: **Add › Mesh › Circle**, 28 vertices, Radius 0.25, **Fill Type Triangle Fan**. Lift the centre point to 0.04 m. To give the edge a ragged outline (a clean disc showed in the stills), **Randomize** the outer points by 0.03 in X and Y only.
4. Paint `wear` on both as in Part 2:
   - coals: white in the middle, grey at the edge
   - the bed: at most mid-grey (0.45) in the middle, black at the edge
5. Join coals and bed into `Fire_Wood_Coals`.

## Part 5: the ember material (charcoal, ash and glowing cracks)

Build it in the Shader Editor (Shading workspace); Shift+A then type the node's name finds any node.

| # | Node | Settings | Why |
|---|---|---|---|
| 1 | Attribute | Name `wear` | *heat*, how hot each point is |
| 2 | Value, with a driver | `frame / 24` (guide 07, Part 8, step 1); add Object Info › Random × 100 | the time in seconds, each copy at its own moment |
| 3 | Texture Coordinate › Object | | the pattern sticks to the coals |
| 4 | Noise Texture | Scale 42 (0.6 × 70), from step 3 | to bend the cracks |
| 5 | Vector Math › Multiply Add | the noise's Color × (0.017, 0.017, 0.017), + the coordinates | warped coordinates: the cracks aren't tidy polygons |
| 6 | Voronoi Texture | Feature **Distance to Edge**, Scale 70, from step 5 | cells about 1.4 cm across; Distance is 0 on their edges |
| 7 | Map Range | Distance 0 → 0.05 to 1 → 0 | *crack*: 1 in a thin line along each edge |
| 8 | Noise Texture | **4D**, Scale 5, Detail 2, W = time × 0.4; Map Range 0.35 → 0.65 to 0.35 → 1 | *breathing*: the glow slowly brightens and dims |
| 9 | Noise Texture | Scale 9, Detail 2; Map Range 0.42 → 0.62 to 0.25 → 1 | *patches*: some parts glow, some hardly |
| 10 | Math | glow = heat × breathing × patches × (0.15 + 0.85 × crack) | brightest in the cracks, faintly between |
| 11 | Color Ramp | glow: 0 #3C0600, 0.35 #B41E04, 0.65 #FF6010, 1 #FFB450 | dull red to orange, as charcoal glows from 525 °C upwards |
| 12 | Noise Texture | Scale 60, Detail 4; Map Range 0.38 → 0.66; × Map Range of heat 0.1 → 0.6 to 1 → 0 | *ash*: mottled grey on the cooler parts, none on hot coals (a sharp 0.48 → 0.6 step made camouflage blobs) |
| 13 | Mix Color | charcoal #1A1715 → ash #787470 by *ash* | into **Base Color**; Roughness 0.9 |
| 14 | | step 11 into **Emission Color**; glow × 6 × (1 − 0.8 × ash) into **Emission Strength** | ash covers the glow |

What you see at night: a dark bed, grey with ash at its edge, the coals black with thin orange cracks, some brighter than others, slowly changing as you scrub the timeline.

## Part 6: the flames

What: three cards through the fire's axis, and a material that draws the fire on them, moving with time. Why cards: the camera can go round the fire, and from any side one card faces it (guide 07, Part 7).

1. A card: **Add › Mesh › Plane**, rotate it upright (R X 90). Size it 0.56 wide (2 × 0.62 × D) and from z −0.015 to 1.0 (−0.03 × L to 2 × L), its origin at the bottom centre. Shift+D twice, rotating each copy 60° round Z (R Z 60). Join (Ctrl+J), name `Fire_Wood_Flames`, and place it so its origin is at z 0.03, the flames' root.
2. **Object Properties › Visibility**: untick **Shadow**. The light sits inside the fire.
3. The material, `M_Fire_Wood_Flames`. **Material Properties › Settings › Render Method › Blended**. Why: the flames add light, so the order of the crossing cards doesn't matter.

| # | Node | Settings (D 0.45, L 0.5) | Why |
|---|---|---|---|
| 1 | Value with the driver `frame / 24`, + Object Info Random × 100 | | *t*, seconds |
| 2 | Texture Coordinate › Object › Separate XYZ | | x, y, z; the root at 0 |
| 3 | Noise Texture **1D** | W = t × 1.5 (0.6 × 2.5 Hz), Scale 1, Detail 1; Separate Color, each − 0.5 | *sx, sy, sh*: slow lean (two ways) and height |
| 4 | Math › Sine | t × 15.7 (2π × 2.5) | *puff*: the fire breathes at its puffing rate |
| 5 | Math | lh = (1 + 0.1 × puff + 0.6 × sh) × 0.5 | the flame height of the moment |
| 6 | Math | hgt = z / lh | 1 at the mean flame height |
| 7 | Math | lean = hgt² × 0.1575 (0.35 × D); ax = x − sx × lean; ay = y − sy × lean | the top leans more than the root |
| 8 | Combine XYZ | (ax, ay, z − t × 1.13), where 1.13 = 1.6 × √L | the pattern rises through the fire |
| 9 | Noise Texture **4D** | from step 8, W = t × 0.5, Scale 3.33 (1.5 / D), Detail 1; Vector Math: (Color − 0.5) × 0.2025 (0.45 × D), added to step 8 | a big slow warp, so the tongues curl |
| 10 | Mapping | Scale (1, 1, 0.5) | features twice as tall as wide |
| 11 | Noise Texture **4D** | from step 10, W = t × 0.7, Scale 7.78 (3.5 / D), Detail 4, Roughness 0.6 | *turb*: the tongues |
| 12 | Math | rr = √(ax² + ay²) / 0.225; edge = rr / MapRange(hgt 0 → 1.6 to 1 → 0.25) | distance from the axis, the fire narrowing as it rises |
| 13 | Math, Map Range | base = MapRange(hgt −0.02 → 0.08) × MapRange(hgt 0.15 → 1.6 to 1 → 0) × MapRange(edge 0.35 → 1 to 1 → 0) | the fire's envelope: solid low, nothing above 1.6 L |
| 14 | Math | field = base + (turb − 0.5) × 2.4 | at 1.6 the fire was a soft blob; at 2.4 it breaks into tongues |
| 15 | Map Range | heat = field 0.45 → 1.15 | |
| 16 | Color Ramp | heat × MapRange(hgt 0.2 → 1.5 to 1 → 0.55): 0 #962804, 0.25 #EB6010, 0.5 #FF8C24, 0.75 #FFC460, 0.92 #FFE4A0, 1 #FFF2CD | cooler towards the tips. A deeper red, added over daylight, read pink |
| 17 | Mix Color › Multiply | white, named `Fire_Tint` | the video's orange multiply; white here, for light states to change |
| 18 | Map Range | alpha = heat 0 → 0.15; × MapRange(Layer Weight › Facing, Blend 0.5: 0.55 → 0.85 to 1 → 0) | crisp-edged tongues; a card seen edge-on fades out |
| 19 | Emission | Color step 17, Strength alpha × 1.6 | |
| 20 | Add Shader | Transparent BSDF + Emission | into the Material Output's Surface |

What you see when you scrub the timeline (Alt+scroll): a solid yellow core, orange tongues rising, curling and breaking off above about half a metre, the whole fire breathing two or three times a second.

## Part 7: the light

1. **Add › Light › Point**, at (0, 0, 0.18): z0 + 0.3 × L. Power 150 W, Radius 0.135 (0.3 × D), colour #FF9640.
2. The flicker: right-click **Power › Add Driver**, type Scripted Expression:
   `150.0*(1 + 0.125*sin(frame*0.656) + 0.075*sin(frame*1.554 + 1.3) + 0.050*sin(frame*2.6 + 0.4))`
   - 0.656 radians a frame is 2.5 Hz at 24 fps, the fire's puffing rate. The other two terms are faster.
   - The three amplitudes add up to ±25%, more than the oil lamp's ±12%.
   - Nothing faster than 2.6 radians a frame: 24 fps can't show it.
3. The sway: right-click **Location X › Add Driver**:
   `0.0000 + 0.0270*(0.6*sin(frame*0.525 + 0.7) + 0.4*sin(frame*1.246 + 1.4))`
   For **Y**, the same with phases 2.1 and 4.2. The light moves up to 2.7 cm (0.06 × D), so the logs' shadows sway as round a real fire. The first number is the light's own X (or Y); change it if your fire stands elsewhere.

For the embers there's no flame: a 12 W light 5 cm above the root, radius 0.2, colour #FF6E28, flickering only ±15% at 0.4 Hz.

## Part 8: the sparks (Geometry Nodes)

What: 60 small glowing balls rising from the fire, swirling, cooling and going out, each starting again when it's gone. Why not the video's particle system: particles need a bake, and a particle system and its force field don't work inside a linked collection instance. These sparks are worked out from the time alone, so they need no bake and work when linked.

1. An empty object to hold them: add a Plane, Edit Mode, delete all its points (X › Vertices). Name it `Fire_Wood_Sparks`; place it at the flames' root (0, 0, 0.03). Untick **Shadow** (Object › Visibility).
2. **Modifier Properties › Add Modifier › Geometry Nodes › New**. In the Geometry Nodes workspace, add these inputs to the group (N panel › Group › +): Count (Integer) 60, Radius 0.15, Height 1.6, Life 2.5, Swirl 0.25, Size 0.003, Seed (Integer) 4, Material (Material).
3. The nodes. Every **Random Value** is Float, its ID from **Index**, its Seed = Seed + 7 × its number below, so each draws different numbers per spark:

| # | Node | Settings | Why |
|---|---|---|---|
| 1 | Scene Time | Seconds | the time |
| 2 | Random Value (1), 0.6 → 1.3 | life = Life × it | each spark lives 1.5–3.3 s |
| 3 | Random Value (2), 0 → 1; Math | age = Fraction(Seconds / life + it) | 0 born … 1 gone; every spark at its own moment, so they're in the air on frame 1 (the video's Frame Start −200) |
| 4 | Random Value (3), 0 → 2π; (4), 0 → 1 | r = Radius × √(4) × (1 + 1.5 × age) | where it starts in the disc, spreading as it rises |
| 5 | Combine XYZ | (r cos(3), r sin(3), Height × Random(5) 0.5 → 1.3 × age) | rising |
| 6 | Noise Texture 4D | Vector step 5, W = Seconds × 0.35 + Random(6) 0 → 10, Scale = 2 / Height, Detail 1; Vector Math (Color − 0.5) × Swirl × 2 × age | the swirl: the video's Turbulence field, growing as it rises |
| 7 | Vector Math › Add | step 5 + step 6 | the spark's position |
| 8 | Points | Count = Count, Position = step 7 | one point per spark |
| 9 | Store Named Attribute | Float, Point, Name `age`, Value = age | for the material: sparks cool with age |
| 10 | Ico Sphere | Radius 1, Subdivisions 1 (the video's spark), then Set Material = Material | |
| 11 | Instance on Points | Points step 9, Instance step 10, Scale = Size × Random(7) 0.5 → 1.5 × MapRange(age 0 → 0.05) × MapRange(age 0.55 → 1 to 1 → 0) | each grows in, then shrinks to nothing before it restarts, so the jump back down is never seen |

4. The spark material, `M_fire_spark`:
   - **Attribute** node, Type **Instancer**, Name `age`
   - **Color Ramp**: 0 #FFECBE, 0.3 #FFA03C, 0.7 #DC460C, 1 #781804
   - **Emission**, Strength from a Map Range of age, 0.2 → 1 to 8 → 1.6
   - Set the modifier's Material input to it.

What you see when you scrub: sparks drifting up and sideways, yellow near the fire, red and smaller higher up.

## Part 9: the collection, an asset

1. Put the five objects in a collection `Fire_Wood` (M › New Collection).
2. **Object Properties** of the collection (Properties › Collection tab) › **Instancing › Offset**: (0, 0, 0), the fire's root on the ground. A collection instance then puts the fire's root where the instance stands.
3. Right-click the collection in the Outliner › **Mark as Asset**; add a description.
4. To use it in a place: **File › Link** › `fire.blend` › Collection › `Fire_Wood`. The flames and the light differ for every copy (Object Info › Random), but **the sparks are the same in every copy**: Geometry Nodes run once per object, not per instance. In the story each fire appears once, so this doesn't show.

## Check against the script model

**File › Append** `blender/objects/fire.blend` › Collection › `Fire_Wood` and compare at frames 1 and 12. The review stills are in `renders/checks/fire/` (`run.ps1 views fire -Object`): `all`, `all_night`, `wood`, `wood_night`, `wood_f12`, `hearth_night`, `embers_night`, `large_night`, `large_f12`.

## Pitfalls (learned while building it)

- **The flame read pink over daylight**: a deep red at the ramp's bottom, added at low alpha over a pale ground. The ramp starts at orange-red #962804 now.
- **A soft blob, no tongues**: the turbulence added at × 1.6 never broke the flame up. At × 2.4, with Detail 4, tongues form and break off.
- **The big fire glowed white**: four crossing cards add their light. Its emission strength is 1.1 (the campfire's 1.6, the hearth's 1.8).
- **A glowing ball at the tepee's top**: the tips met in the middle, where the rule "burned near the centre" made them glow. Points high above the flames are only half as burned now: black, not glowing.
- **The charcoal looked like a giraffe**: Voronoi cells 3 cm across with wide cracks. Cells about 1.4 cm, cracks a twentieth of a cell, warped by a noise, glowing in patches.
- **Camouflage ash**: a sharp ash threshold (0.48 → 0.6 of a Scale 14 noise) made blobs. A fine noise (Scale 60) and a soft ramp (0.38 → 0.66) make mottled grey.
- **A clean disc of ash** showed on the ground: the bed's outer points are pushed in and out at random.
- **Script only (Blender 5.2)**: a Geometry Nodes modifier's inputs are no longer set with `modifier["Socket_1"] = value`; it's `modifier.properties.inputs.Socket_1.value`. By hand you just type into the modifier panel.
