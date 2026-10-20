# 07: Modelling the damar (Jamadagni's oil lamp)

Build by hand the lamp that `scripts/common/damar.py` builds into `blender/objects/damar.blend`: an Old Javanese bronze pole lamp with four oil dishes and a seated ascetic, on a turned teak stand. The numbers are the script's, in metres. Research and reasons: [docs/stories/ramabargawa/objects/damar.md](../../../docs/stories/ramabargawa/objects/damar.md).

New in this guide:
- the **Screw** modifier, for turned (lathe) shapes
- **Bezier curves with a bevel**, for thin round tubes
- **Proportional editing**, for soft shapes
- the **Solidify** modifier, for thin walls
- a **moving flame drawn by its shader** on crossed cards, after Max Hay's fire video ([video-fire-max-hay.md](video-fire-max-hay.md)), and a point light that its own flame must not shadow
- **drivers**: a value that follows the frame (`#frame / 24`), and a light that flickers
- a **collection instance** for pieces used several times

## The plan

```
        side view                                  top view (the ascetic faces south, -Y)
           ^ 0.89  aureole tip                          N
           | 0.84  head                        dish  \       /  dish        four dishes on the diagonals,
   flame   | 0.78  plinth top, ascetic sits      \   [plinth]   /           so none stands in front of him
     |     | 0.768 dish rims                      /     ^       \
   ~~\_/~~ | 0.75 → 0.78  three square tiers    dish  ascetic   dish
     [ ]   | 0.675 → 0.75  bronze socket                S
      |    | 0.70  top of the teak shaft
      |    |
     _|_   | 0.085 top of the foot
    /___\  | 0     the foot's underside (the object's origin)
```

| Part | Size | Height (m) |
|---|---|---|
| Foot (teak, turned) | 0.25 across | 0 → 0.085 |
| Shaft (teak, turned) | 0.036–0.06 thick | 0.085 → 0.70 |
| Socket (bronze) | 0.058 across, a lip of 0.067 | 0.675 → 0.75 |
| Plinth: three square tiers | 0.072, 0.062, 0.052 wide, 0.01 each | 0.75 → 0.78 |
| Dishes | 0.08 long, up to 0.048 wide, 0.015 deep, from radius 0.04 to 0.12 | rim 0.768 |
| Ascetic | 0.08 tall | 0.78 → 0.86 |
| Aureole | 0.046 wide, 2 mm thick | 0.80 → 0.89 |
| Flames | 0.011 wide, 0.034 tall, roots at (±0.089, ±0.089); drawn on three cards 0.024 wide | 0.782 → 0.816 |

**Work at this scale.** The lamp is small: zoom in with **View › Frame Selected** (numpad .), and set **View › Sidebar** (N) › View › Clip Start to **0.001 m**. Otherwise the view cuts into the model when you come close.

## Part 0: set up

1. **File › New › General**, delete the cube (X).
2. **Scene Properties** (the cone icon) › Units: Metric, Length Meters.
3. Keep the 3D cursor at the world origin (**Object › Snap › Cursor to World Origin**, Shift+S). The lamp's foot stands on it.

## Part 1: the stand (Screw modifier)

*What*: a turned shape is one profile spun round an axis, as on a lathe. The **Screw** modifier spins a line of points round the object's Z axis.

1. **The profile's first point.**
   - Add › Mesh › Plane.
   - Tab into Edit Mode, then **Mesh › Merge › At Center** (M): the plane becomes one point.
2. **Type its position.** Open the Sidebar (N), **Item** tab, Vertex: X **0.125**, Y 0, Z 0.
3. **Draw the foot and shaft** as a line of points:
   - Extrude a new point (**Mesh › Extrude › Extrude Vertices**, E, then Enter).
   - Type its X and Z in the Sidebar. X is the radius, Z the height.
   - Do it for every pair below, in order (radius, height):
     - Foot: (0.125, 0.018), (0.118, 0.024), (0.105, 0.028), (0.105, 0.04), (0.09, 0.046), (0.065, 0.054), (0.045, 0.068), (0.032, 0.085)
     - Shaft: (0.026, 0.1), (0.022, 0.13), (0.03, 0.15), (0.022, 0.17), (0.019, 0.32), (0.024, 0.34), (0.024, 0.355), (0.019, 0.375), (0.018, 0.56), (0.023, 0.58), (0.023, 0.595), (0.018, 0.615), (0.018, 0.68), (0.026, 0.69), (0.026, 0.70)
   - Last, a point at (0, 0.70) closes the top, and you go back to the first point to add (0, 0) for the bottom.
   - *You should see*: a zigzag line, the outline of half the stand.
4. **Spin it.**
   - Tab out, then **Properties › Modifiers › Add Modifier › Generate › Screw**: Axis **Z**, Angle **360°**, Steps Viewport and Render **24**.
   - Tick **Merge**, so the points on the axis weld.
   - *You should see*: the turned foot and shaft.
5. **Smooth it**: **Object › Shade Auto Smooth**, 40°. The rings stay crisp and the round faces smooth.

## Part 2: the socket and the plinth

1. **Socket.**
   - Add › Mesh › Cylinder: Vertices 24, Radius **0.029**, Depth **0.075**, Location Z **0.7125**, so it runs 0.675 → 0.75.
   - *Why it starts below 0.70*: it is a sleeve over the shaft's top, hiding the joint.
   - **The lip**: in Edit Mode, **Loop Cut** (Ctrl+R) twice near the top, at z 0.733 and 0.738. Select the ring between them and Scale it to 1.155, so the lip is 0.0335 in radius.
2. **Plinth.** Three cubes, each 0.01 tall, centred on the axis:
   - 0.072 × 0.072 at Z 0.755
   - 0.062 × 0.062 at Z 0.765
   - 0.052 × 0.052 at Z 0.775
   - Use Dimensions in the Sidebar and apply scale (Ctrl+A › Scale).
   - Join them and the socket into one object, the head: select all four, **Object › Join** (Ctrl+J).

## Part 3: one dish, then the other three

1. **A bowl.**
   - Add › Mesh › UV Sphere: Segments 16, Rings 8, Radius 1.
   - In Edit Mode, view from the front (numpad 1) and box-select the upper half's points (B). Delete them: **Mesh › Delete › Vertices** (X).
   - Select all and Scale: S X 0.04, S Y 0.024, S Z 0.015 (a half-egg 0.08 long, 0.048 wide, 0.015 deep).
2. **The leaf point and the spout.**
   - Select the point at the +X tip.
   - Turn on **Proportional Editing** (O; the header's circle icon), falloff **Smooth**.
   - Move it G X **0.012**. While moving, turn the mouse wheel until the influence circle covers about half the bowl. The tip draws out into a leaf point.
   - Then G Z **0.006**: the spout rises.
   - Turn Proportional Editing off again (O).
3. **The wall.**
   - **Add Modifier › Generate › Solidify**, Thickness **0.0025**, Offset −1 (inward).
   - *Why*: a dish has a thin wall you see at the rim. Without it, the bowl is a paper-thin skin.
4. **Put it in place.**
   - In Edit Mode, select all and rotate R Z **45**.
   - Move it so its centre is at radius 0.08 on the diagonal: G X **0.0566**, G Y **0.0566**, then G Z so the rim is at **0.768**.
   - *You should see*: one leaf-shaped dish reaching out from the plinth's corner.
5. **Four of them.** In Object Mode, **Add Modifier › Generate › Mirror**, axes **X** and **Y**. The object's origin must be at the world centre (the same rule as the pendopo's posts).
   - *You should see*: four dishes on the diagonals.

## Part 4: flanges and struts

1. **A flange.**
   - Add › Mesh › Plane, Dimensions **0.017 × 0.03**, standing up: R X 90.
   - Shape it into a leaf: in Edit Mode, **Subdivide** (right-click › Subdivide, Number of Cuts 3), then pull the top-middle point up and pinch the corners in with Proportional Editing.
   - **Curl it outward**: Add Modifier › Deform › **Simple Deform**, mode **Bend**, Angle **45°**. Pick the axis that curls the top away from the dish.
   - Place it on one side of the dish, at radius about 0.074 on the diagonal, its foot on the rim. Mirror it like the dish (X and Y), and add a second one on the dish's other side.
2. **A strut** under each dish, a curved rod from the socket to the dish's underside.
   - Add › Curve › **Bezier**. In Edit Mode, put its first point at radius **0.022**, z **0.734**, on the diagonal, and its last point at radius **0.078**, z **0.756**. Bend it with the handles so it hangs a little lower in the middle.
   - **Object Data** (the curve icon) › Geometry › **Bevel › Depth 0.003**, Resolution 3. *What*: the curve becomes a round tube.
   - Mirror on X and Y.

## Part 5: the ascetic and the aureole

The plinth's top is at **0.78**. All parts are bronze; he faces −Y.

| Part | How | Size / place |
|---|---|---|
| Lotus cushion | Cylinder, 16 sides | radius 0.026, 0.0085 tall, on 0.78 |
| Crossed legs | UV Sphere scaled | 0.044 × 0.03 × 0.0136, centre (0, −0.003, 0.7945) |
| Torso | UV Sphere scaled 0.02 × 0.014 × 0.04, then shaped with Proportional Editing: a thin waist, a bony chest 0.024 wide at z 0.821, sloping shoulders 0.025 wide at 0.8265, a narrow neck | from 0.792 to 0.832 |
| Head | UV Sphere | 0.014 × 0.015 × 0.017, centre (0, 0.0005, 0.8395) |
| Topknot | Cone | radius 0.0046 → 0.0024, 0.012 tall, from 0.8455 |
| Arms | two Bezier curves, bevel depth 0.003 | from the shoulders (±0.0118, 0.002, 0.8245) out round the elbows to the hands (±0.0055, −0.0108, 0.8105) |
| Lontar (palm-leaf book) | Cube | 0.026 × 0.0048 × 0.006, bottom at 0.8075, at y −0.0125, between his hands |

**The aureole** (the flames behind him):
1. Add › Curve › **Circle**. In Edit Mode, view from the front (numpad 1) and shape it into a flame outline:
   - 0.046 wide at the bottom (z 0.80), narrowing to a tip at z **0.89**.
   - Four tongues on each side that bulge out and **curl up** to their tips.
   - Add points with **Subdivide**, and move them with G.
2. Object Data › Shape › Fill Mode **Both**; Geometry › **Extrude 0.001** (a 2 mm plate).
3. Rotate it R X 90 so it stands up, and move it behind him to y **0.0118**.
4. *Pitfall*: tongues that point straight out make it look like a stepped tower. Keep each one curling upward.

## Part 6: oil and wicks

1. **Oil**: in each dish, a Plane shaped to the bowl's inside, about a third of the way down from the rim (z about 0.763), with the oil material. One dish's oil with a Mirror X and Y does all four.
2. **Wick**: a Bezier curve (bevel depth **0.002**) lying in the spout. It starts in the oil, runs along the spout and sticks out about 5 mm past the tip, ending at (0.089, 0.089, **0.781**). Mirror X and Y.
3. **The sooted tip**: select the last few millimetres of the wick and give them a dark soot material. Convert the curve to a mesh first: **Object › Convert › Mesh**.

## Part 7: a flame and its light, then four of them

1. **The flame's cards.** The flame is not modelled. Its shader (Part 8) draws it on flat cards, as the fire video puts fire footage on a plane.
   - Add › Mesh › **Plane** at the world origin. Rotate it 90° on X (R X 90) so it stands upright, and scale it to **0.024 wide × 0.051 tall**.
   - Move it so its bottom edge is at z **−0.003** and its top at **0.048**. The flame's root (the wick tip) is at 0. There is room around the 0.034 flame for it to sway and lick.
   - Duplicate it twice (Shift+D) and turn the copies **60°** and **120°** about Z (R Z 60). Join the three (Ctrl+J). Seen from above they make a six-pointed star through one axis.
   - *Why three cards*: one card only works when the camera looks straight at it. With three, one is always within 30° of facing the camera. From straight above the flame still disappears, as the video warns.
2. **The light.** Add › Light › **Point** at (0, 0, 0.012), inside the flame: Power **0.5 W**, Radius **0.02**, a warm colour (#FFB266).
   - **Make it flicker**: right-click Power › **Add Driver**. In the driver popup, set the expression to
     `0.5*(1 + 0.06*sin(frame*0.37) + 0.04*sin(frame*0.91 + 1.3) + 0.025*sin(frame*2.3 + 0.4))`.
   - The field turns purple. Play the timeline (Space): the power wavers about ±12%, one to nine times a second.
   - *Why sines*: `sin` and `frame` are allowed in Blender's simple expressions, so the driver runs without allowing Python scripts.
   - *Why the light, not the flame, lights the lamp*: the video's advice. Emission is noisy and hard to render as a light source; a point light the flame's size does it cleanly.
3. **The flame must not shadow its own light.**
   - Select the cards, then **Object Properties › Visibility › Ray Visibility** and untick **Shadow**.
   - *Why*: the light sits inside the flame. If the flame casts shadows, it blocks the light and the lamp stays dark at night.
4. **A collection.** Put the flame and the light in a new collection `Flame`: with both selected, **Object › Collection › Move to Collection** (M) › New Collection.
5. **Four flames.**
   - **Add › Collection Instance › Flame**, four times, one at each wick tip: (0.089, 0.089, 0.782), (−0.089, 0.089, 0.782), (−0.089, −0.089, 0.782), (0.089, −0.089, 0.782).
   - Exclude the original `Flame` collection from the view layer (untick its checkbox in the Outliner), so it doesn't show at the origin.
   - *Why a collection*: for the moonlit night (S4) you can delete three instances and keep one. The one flame is still exactly the same piece.
   - The four instances share one material, yet each flame moves differently: the shader reads each instance's own random number (Part 8, step 3). The video instead copies the material and offsets the footage by 50 frames.

## Part 8: materials

**Bronze** (the head, dishes, flanges, struts, ascetic, aureole). In the Shading workspace:
1. **Principled BSDF**, Metallic 1, Roughness 0.3, Base Color bronze **#946636**.
2. **Patina in the hollows**:
   - Add › Input › **Ambient Occlusion**, Distance **0.012**, Only Local on.
   - Its AO goes into a **Map Range**, From Min 0.95, From Max 0.45, so the hollows give 1.
   - That drives a **Mix Color** from the bronze to olive patina **#3E422E**.
3. **Faint patches**: add a **Noise Texture** (Object coordinates, Scale 70) through a Map Range (0.42 → 0.68), times **0.25**, to the patina amount.
4. **Polish where hands rub**:
   - Paint a colour attribute in **Vertex Paint** mode, white on the dish rims, the socket lip, the flange tips, his knees and the crown of his head.
   - Read it with Add › Input › **Attribute** (its name).
   - It lightens the colour toward polished **#DAA862** by × 0.75, and keeps the patina away there.
   - The script stores this as the `wear` attribute.
5. **Patina is not metal**: Metallic = 1 − patina × 0.65, and Roughness = 0.3 + patina × 0.42.

**Turned teak** (the stand): the teak texture of guide 02, darker (palette **#402818**, mix 0.55), tile 0.4 m.
- Unwrap the stand with **UV › Cylinder Projection** (from the front view) so the grain runs up the shaft.
- Not the weathered layers of the pendopo's posts: on a 2 cm shaft every face reads as a worn edge.

**Oil**: Principled, Base Color **#2E220E**, Roughness 0.05. **Wick**: **#A8926A**, Roughness 0.8. **Soot**: **#2C2622**.

**Flame** (on the cards; after [Max Hay's fire video](video-fire-max-hay.md)). The video puts a fire video on a plane. We have no fire video, since our sources are Blender and Poly Haven, so the shader draws the "footage": a teardrop that sways, breathes and licks with time. Every Map Range below is set to **Smooth Step**; *x* means a Math node.

1. **The output.**
   - Delete the Principled BSDF. Add a **Mix Shader**: its top input a **Transparent BSDF**, its bottom an **Emission** (Strength **1.6**). The Mix Shader's Factor will be the alpha (step 9).
   - **Material Properties › Settings › Render Method: Dithered**, the default. Cards crossing each other then sort correctly.
2. **Time.** Add › Input › **Value**, named `Flame_Time`. Click its field and type `#frame / 24`, then Enter. The field turns purple: it is a driver, and gives the time in seconds. This replaces the video's looping footage.
3. **Each flame its own moment.** Add › Input › **Object Info**. Its **Random** × 100, added to the time, gives **t**. Each instance has its own random number, so the four flames never move together.
4. **Where we are.** Texture Coordinate › **Object** › **Separate XYZ** gives x, y and z in metres, with the root at 0.
5. **The whole flame moves slowly: sway and breathing.**
   - A **Noise Texture** set to **1D**, W = t × 1.3, Scale **1**, Detail **2**.
   - Its **Color** › **Separate Color** gives three slow wobbles. Take 0.5 from each: **sx**, **sy**, **sh**.
   - The flame's height **hz** = 0.034 × (1 + sh), so it breathes about ±20%.
   - The height up the flame **u** = z / hz: 0 at the root, 1 at the tip.
   - It leans more towards the top: **bend** = u × u × 0.02.
   - The distance from its swaying axis: **r** = √((x − sx × bend)² + (y − sy × bend)²).
6. **Small fast licks, rising.**
   - A **Combine XYZ** (x, y, z − t × 0.03), so the pattern rises 3 cm a second, into a **Noise Texture** set to **4D**. Its W = t × 2, Scale **190**, Detail **2**.
   - Its Color › Separate Color, take 0.5: **lr**, **lz**.
   - Then r = r + lr × u × 0.0041, and u = (z + lz × u × 0.0061) / hz. The licks grow towards the tip.
7. **The teardrop.**
   - Add › Converter › **Float Curve**, its Value from u. Points: (0, 0.22), (0.09, 0.8), (0.24, 1.0), (0.41, 0.91), (0.62, 0.69), (0.82, 0.36), (1, 0). That is the flame's half-width up its height: a narrow root, widest a quarter of the way up, a point at the tip.
   - In the curve's **wrench menu**, set **Extend Horizontal**.
   - The edge's distance **dist** = r / maximum(curve × 0.0058, 0.00001). It is 1 at the flame's edge.
8. **Brightness and colour.**
   - **body** = Map Range dist 0.5 → 1.05 to 1 → 0 (a soft edge), × Map Range u −0.06 → 0.04 (nothing under the root).
   - **heat** = body × Map Range u 0.15 → 1 to 1 → 0.55 (cooler towards the tip), × (0.75 + 0.25 × Map Range dist 0.2 → 0.7 to 1 → 0) (a hotter core).
   - A **Color Ramp** on heat: **#962804** at 0, **#EB6010** at 0.3, **#FF8C28** at 0.6, **#FFD682** at 0.85, **#FFF0C8** at 1.
   - **A touch of blue at the root**: a Mix Color towards **#466EFF**, Factor = Map Range u 0.02 → 0.16 to 1 → 0, × Map Range dist 0.35 → 0.9, × body, × 0.7.
   - **The video's orange, multiplied in**: a Mix Color set to **Multiply**, Factor 1, B = RGB **1.0, 0.82, 0.6** (#FFEACB). Name it `Flame_Tint`: a light state can turn the flame redder here. Its result is the Emission's Color.
9. **Alpha, keyed from the brightness**, as the video keys its footage.
   - A second **Color Ramp** on heat: black at **0.04**, white at **0.7**. Raising the black handle off 0 cuts the faint edge, as in the video.
   - × Map Range of **Layer Weight › Facing** (Blend 0.5) 0.55 → 0.85 to 1 → 0. A card turned edge-on to the camera fades out.
   - The result goes into the Mix Shader's Factor.
10. **Check.** In Rendered view, play the timeline (Space). The flame should sway, stretch and lick, each one differently. By day it is pale gold inside with an orange rim; at night it lights its dish.

## Part 9: into the hermitage

The script places the lamp in the pendopo of the hermitage, in the sacred centre, between Jamadagni's bale and the sons' mats:
1. **File › Link** › `blender/objects/damar.blend` › `Collection` › `Damar`.
2. Move the instance to **(0, 1.3, 0.6)**: the pendopo's floor is at 0.6.

## Check against the script model

- **File › Append** › `blender/objects/damar.blend` › `Collection` › `Damar`, and toggle it on and off against yours.
- The foot on 0, the dish rims at 0.768, the wick tips at (±0.089, ±0.089, 0.782), the aureole's tip at 0.89.
- Review stills: `run.ps1 views damar -Object` (`lamp`, `head`, `ascetic`, `dish`, `top`, and `night`, `night_head` with the sun off). Also `flame`, `flame_f7` and `flame_f15`, one flame at frames 1, 7 and 15 to see it move, and `flame_night`.

## Pitfalls (learned while building it)

- **The flame shadows its own light**: untick Shadow in its Ray Visibility (Part 7).
- **Lights a centimetre from a surface are very strong.** At 1.5 W the bronze turned copper-pink in daylight; 0.5 W with a 2 cm radius is enough.
- **Thin round things need enough sides.** Tubes of 6–8 sides read as flat bands under Auto Smooth 40°. Use 10–12, or a curve's bevel resolution 3.
- **Flat 5-point leaves read as diamonds.** Subdivide and bend them.
- **Clip Start**: at the default 0.1 m (and in cameras), close views cut into the lamp. Use 0.001 m.
- **The flame on cards**:
  - **A Float Curve runs on past its ends** unless Extend is Horizontal. Above the tip the width went negative, and a solid orange band filled the top of the cards.
  - **A card seen edge-on shows as a thin line** through the flame. Fade it out with Layer Weight › Facing.
  - **Alpha from the shape alone drew an outline.** The dim red rim was opaque. Keyed from the brightness, as in the video, the rim fades out.
  - **A deep red rim reads pink** over a pale sky; it is orange now.
  - **From straight above**, the cards are edge-on and the flame is nearly gone. Keep story cameras off the top.
