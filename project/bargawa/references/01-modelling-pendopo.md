# 01 Modelling the pendopo by hand

Build the hermitage's pendopo (an open joglo pavilion) by hand, with the same sizes as the script model `blender/buildings/pendopo.blend` (`scripts/common/joglo.py`). What each part is and why it is there: [docs/stories/ramabargawa/buildings/pendopo.md](../../../docs/stories/ramabargawa/buildings/pendopo.md).

**You will learn**:
- exact sizes, and applying scale
- the origin and the 3D cursor
- Edit Mode: select, Extrude, Inset, Scale on one axis, Merge, Delete
- the modifiers Mirror, Array and Solidify
- a hip roof and a ridge
- the stepped tumpang sari ceiling

**Time**: one to three sessions. Do the parts in order; each one builds on the last.

## The plan

Seen from above, with the centre of the pendopo at the world origin (0, 0):

```
   y
   ^      o-----o-----o-----o        o  soko penanggap: 12 posts on a ring at ±4.7 m
   |      |                 |           (x or y = -4.7, -1.567, 1.567, 4.7)
   |      o     X     X     o        X  soko guru: 4 posts at (±1.6, ±1.6)
   |      |                 |
   |      o     X     X     o        plinth 10.4 × 10.4 m, floor at 0.6 m
   |      |                 |        steps on the south side (−y)
   |      o-----o-----o-----o
   +----------------------------> x
                 steps
```

| Part | Size (m) | Height (z, m) |
|---|---|---|
| Plinth (bebatur) | 10.4 × 10.4 | 0 → 0.6 (the floor) |
| Umpak under the soko guru (padma pedestal) | 0.56 wide; tall = the post's perimeter (pakem) | 0.6 → 1.32 |
| Umpak under the soko penanggap | 0.30 at the foot → 0.20 at the top; tall = half the post's perimeter | 0.6 → 0.86 |
| Soko guru | 0.18 → 0.17 square | 1.32 → 4.6 |
| Soko penanggap | 0.13 → 0.12 square | 0.86 → 3.05 |
| Blandar on the soko guru | 0.22 × 0.24, 3.8 long | centre 4.72 |
| Blandar on the penanggap ring | 0.15 × 0.20, 10.0 long | centre 3.15 |
| Sunduk (through the soko guru) | 0.165 × 0.22, 3.84 long | centre 4.05 |
| Tumpang sari | 4 tiers of 0.15 | 4.84 → 5.44 |
| Penanggap roof | eave half-size 6.0 → top half-size 2.4 | 2.62 → 5.41 |

These proportions follow the **pakem** (fixed 2026-10-18; research in [pendopo.md, "Pakem soko"](../../../docs/stories/ramabargawa/buildings/pendopo.md)):
- the umpak rule of the Kawruh Kalang (soko guru umpak height = the post's perimeter, soko penanggap umpak = half of it)
- the real Joglo Lawakan of Tamansiswa scaled to our 3.2 m frame: slender posts, soko guru about 1.6 times the soko penanggap's height
| Brunjung roof | eave half-size 2.85 → ridge 2.0 long | 5.30 → 8.6 |
| Thatch thickness | 0.3 | |

## Part 0: set up

1. **A new file.**
   - *What*: an empty scene to practise in.
   - *How*: **File › New › General**, then select the default cube and delete it: **Object › Delete** (X).
   - *Then* **File › Save As**, saving it as `project/bargawa/blender/practice/pendopo_manual.blend`.
   - *Why*: saving first means relative paths work when you bring in textures and references later.
2. **Metres.**
   - *What*: the scene's unit system. *Why*: every number in this guide is in metres.
   - *Where*: **Properties editor › Scene tab (cone and spheres icon) › Units**, Unit System **Metric**, Length **Meters**.
3. **The Sidebar.**
   - *What*: the panel on the right of the 3D Viewport where you type exact numbers.
   - *Where*: **View › Sidebar** (N), then its **Item** tab, showing Location, Rotation, Scale and Dimensions.
   - *You should see*: the Item tab with empty fields, since nothing is selected.
4. **The 3D cursor at the centre.**
   - *What*: the red-white circle where new objects appear.
   - *Why*: the Mirror modifier mirrors around the object's origin, so our origins sit at the world centre.
   - *How*: **Object › Snap › Cursor to World Origin** (Shift+S, then Cursor to World Origin).

## Part 1: the plinth (bebatur)

1. **Add a cube.**
   - *How*: **Add › Mesh › Cube** (Shift+A).
   - *You should see*: a 2 m cube at the centre.
2. **Give it the plinth's size.**
   - *What*: Dimensions sets the size directly in metres.
   - *How*: Sidebar › Item › Dimensions: X **10.4**, Y **10.4**, Z **0.6**. Then Location Z **0.3**: the cube's centre is half its height up, so its top lands at 0.6.
   - *You should see*: a wide, flat slab with its top at 0.6 m.
3. **Apply the scale.**
   - *What*: Dimensions changed the object's Scale, not its mesh. *Apply* bakes the scale into the mesh and resets Scale to 1.
   - *Why*: modifiers (Solidify thickness, Array distances, Bevel) measure in the object's own units. With an unapplied scale of 5.2, a 0.3 m Solidify becomes 1.56 m.
   - *How*: **Object › Apply › Scale** (Ctrl+A, Scale).
   - *You should see*: Scale back to 1, 1, 1 while Dimensions stay 10.4 × 10.4 × 0.6.
4. **Name it.** Double-click it in the Outliner and type `Plinth`. Good names make the later steps easy to follow.
5. **The steps on the south, as plain boxes first.** Three boxes, the outermost lowest. For each: Add › Mesh › Cube, set Dimensions and Location, then apply scale.

   | Step | Dimensions X × Y × Z | Location X, Y, Z |
   |---|---|---|
   | Lowest (outermost) | 3.4 × 0.36 × 0.45 | 0, −6.10, −0.075 |
   | Middle | 3.2 × 0.36 × 0.6 | 0, −5.74, 0.0 |
   | Top | 3.0 × 0.36 × 0.75 | 0, −5.38, 0.075 |

   Their tops are at 0.15, 0.30 and 0.45 m; the floor is at 0.6. They reach below the ground so nothing floats. The script now builds them from stone blocks, in their own object; see Part 1b.
6. *Optional, as in the script*: a darker foot course 10.8 × 10.8 × 0.6 at Location Z −0.15, and a thin coping 10.56 × 10.56 × 0.08 at Location Z 0.56.

## Part 1b: the stone stair (as the script builds it now)

Since 2026-10-18 the stair is its **own object**, `Pendopo_Stair`, in its own collection, and every step is laid from **separate stone blocks**: softened edges, chunks broken off, small joints between them. The look comes from the modular stone video ([video reference](video-modular-stone-wall-brainchild.md)). The script is `scripts/common/stone.py`.

The stair, like the umpak, now lives in the **stone file**, `blender/objects/stone.blend`, as the piece `Stair_Pendopo` (see "The stone file" in Part 2). Its origin is the middle of its back edge, where it meets the plinth; the pendopo places it at (0, −5.2, 0).

**The layout**:
- Each step is a row of 3 to 4 blocks of different lengths (0.55–1.15 m), with joints of about 1.4 cm.
- The blocks are 0.4 m deep, so they reach a little under the next step, and sit 0.3 m into the ground.
- The step tops are the same as before: 0.15, 0.30 and 0.45 m.

**One block by hand, the quick way (modifiers):**
1. A cube with the block's size, for example Dimensions 0.85 × 0.39 × 0.45; apply the scale.
2. **Bevel**: Add Modifier › Generate › **Bevel**, Amount **0.03**, Segments **3**. *What*: rounds every edge; this is the softened stone edge.
3. Turn it into editable mesh: **Object › Apply › Visual Geometry to Mesh**, or the modifier's dropdown › **Apply** (Ctrl+A over it).
4. **Chips**: in Edit Mode, select a few vertices on an edge or corner and push them inward.
   - Use **Mesh › Transform › Move** (G), or for a flat chip, **Scale** them toward the stone (S, a value below 1).
   - Keep it modest: 2 to 6 chips per block, 3 to 8 cm deep, so it still reads as one stone.
5. **Shade**: **Object › Shade Auto Smooth**, angle **30°**. *Why*: the rounded edges stay soft and the chip facets stay crisp. At 38° the chips looked melted.

**One block by hand, the video's way (sculpting):**
1. A cube, then **Object › Apply › Scale**.
2. Give it enough points: **Add Modifier › Generate › Remesh**, mode Voxel, Voxel Size **0.01**, then apply.
3. Switch to **Sculpt Mode** (the mode menu, top left).
4. Use these brushes, with the brush size kept small:
   - **Scrape** knocks the edges off and flattens faces unevenly.
   - **Trim** (Box Trim tool) cuts clean chips.
   - **Pinch** sharpens edges.
   - **Crease** draws cracks.
5. This gives a high-poly stone. In the video it is then rebuilt as a light low-poly mesh and the detail is baked onto it. That is a later guide.

**The worn edges** (lighter edges, as in the video's texturing):
- The script stores a value per point called `wear`: 1 on the rounded edges and chip faces, 0 on flat faces.
- The material reads it with **Add › Input › Attribute** (Name `wear`).
- It feeds a **Mix Color** that lightens the base colour toward **#CECAC0**, by `wear × 0.45`.
- By hand you can paint the same kind of value with Vertex Paint (a colour attribute), or skip it and keep the texture alone.

**A pitfall**: an even rounding everywhere made the blocks look like bars of soap. The script makes the rounding wander along each edge, and chips deeper than the rounding (3–8 cm); 1.5–4.5 cm chips disappeared into it.

## Part 2: the umpak and the posts (one quadrant, then Mirror)

The pendopo is symmetric left-right and front-back. Model one quarter, and let the **Mirror** modifier make the other three. Each object's origin must stay at the world centre, so you move the *geometry* in Edit Mode, not the object.

**In the script model** (since 2026-10-18) the two kinds of soko are separate, each in its own collection:
- `Pendopo_SokoGuru`: the 4 posts (`Pendopo_SokoGuru_1` … `_4`), and 4 padma umpak under them.
- `Pendopo_SokoPenanggap`: the 12 posts (`Pendopo_SokoPenanggap_1` … `_12`), and 12 plain umpak under them.

Both the posts and the umpak are linked from their own files, the soko file and the stone file below.

**The stone file.** The umpak and the stair are not modelled in the pendopo file. They are stone pieces in their own file, `blender/objects/stone.blend` (built by `scripts/objects/stone.py`; the user, 2026-10-18: "separate stone in objects/stone"), and the pendopo **links** them:

| Piece in stone.blend | Used in the pendopo as |
|---|---|
| `Umpak_Guru_1` … `_4` | one under each soko guru |
| `Umpak_Penanggap_1` … `_4` | each under 3 soko penanggap, turned a quarter further each time, so neighbours look different |
| `Stair_Pendopo` | the south steps |

*Why*: like the modular kit in the stone video, a stone is made once and used many times, and any building can use it. Change a stone in the stone file, rebuild it, and every building that links it follows. In stone.blend each piece is its own collection, marked as an **asset**, standing in a row for review; each collection's **instance offset** is the piece's position, so it appears exactly at the empty that instances it.

To do the same by hand:
1. Model the umpak in their own practice file (for example `blender/practice/stone_manual.blend`), each in its own collection, standing on z 0 at the world centre. Save it.
2. In your pendopo practice file: **File › Link** (not Append), open the stone file, then `Collection`, and pick the umpak's collection.
   - *What*: Link brings it in read-only and keeps it connected to its file. Append would copy it, and the copy would no longer follow the stone file.
   - *You should see*: an empty showing the umpak at the 3D cursor.
3. Move that empty to the post's place: (1.6, 1.6, 0.6) for a soko guru. To place more, **Add › Collection Instance** › the umpak's collection, and move each one.
4. *You should see*: the umpak under every post. To change all of them, open the stone file, edit and save it. Then in the pendopo file, reload it: in the **Outliner**, set **Display Mode** (the dropdown at its top) to **Blender File**, right-click the stone file under it, and choose **Reload**. Reopening the pendopo file does the same.

**The soko file.** The teak posts are pieces too, in `blender/objects/soko.blend` (built by `scripts/objects/soko.py`; the user, 2026-10-21: "separate soko in other file"). The pendopo links them the same way, each post standing on the top of its umpak:

| Piece in soko.blend | Size (m) | Used in the pendopo as |
|---|---|---|
| `Soko_Guru_1` … `_4` | 0.18 → 0.17 square, 3.28 tall | one on each padma umpak, at z 1.32 |
| `Soko_Penanggap_1` … `_4` | 0.13 → 0.12 square, 2.19 tall | each 3 soko penanggap at z 0.86, turned like the umpak under them |

- Each piece's origin is the middle of the post's foot, so its empty goes at the umpak's top: (1.6, 1.6, 1.32) for a soko guru.
- Each of the 8 pieces has its own grain. A soko penanggap used three times is turned a quarter further each time, so neighbours show a different face of the wood.
- To carve or change a post, edit it in the soko file (or change `joglo.soko_pieces` and run `run.ps1 object soko`), save, then reload the soko file in the pendopo as above. Every post that uses that piece follows.
- By hand: model one post standing on z 0 at the world centre in its own collection, then link it as in steps 2–4 above, at the umpak's top instead of the floor.

In the steps below, the umpak are modelled in the same object as the post, which is simpler while learning. Once they work, move them to their own file as above. The umpak are **chipped stone**, like the stair: make them plain first (below), then give them the stone look as in Part 1b (Bevel, a few chips, Shade Auto Smooth at 30°).
- On the padma umpak, the lighter **petal tips** come from the same `wear` value as the stone's worn edges.
- By hand, paint them lighter, or select the outermost vertices of each petal ring and give them a lighter colour attribute.

**The soko guru object** (umpak and post together):
1. **The umpak, a tall pedestal.**
   - Add › Mesh › Cube, Dimensions **0.56 × 0.56 × 0.72**, apply scale.
   - *Why 0.72 m*: the Kawruh Kalang makes a soko guru's umpak as tall as the post's perimeter: 4 × 0.18 = 0.72.
   - Press **Tab** to enter **Edit Mode**, the mode where you change the mesh itself.
   - Select all (**Select › All**, A) and move it under the soko guru with its bottom on the floor (0.6). The cube's centre is at z 0, so: **Mesh › Transform › Move** (G), X 1.6, then G Y 1.6, then G Z **0.96** (0.6 + 0.36).
   - *Why in Edit Mode*: the object's origin stays at (0, 0, 0), which the Mirror needs.
   - The lotus on top comes later (see the padma note below). For now it is a plain pedestal.
2. **The post.**
   - Still in Edit Mode, **Add › Mesh › Cube**: in Edit Mode the new cube joins the same object.
   - The new cube is 2 m on each side, so scale it to **0.18 × 0.18 × 3.28**: S X 0.09, then S Y 0.09, then S Z 1.64.
   - Scale its top face to **0.944** (0.17 at the top).
   - Move it to (1.6, 1.6) with its centre at z **2.96**: it stands on the pedestal (1.32) and its top is at 4.6.
3. **Mirror** it: Tab back to Object Mode, then **Properties editor › Modifiers tab (wrench) › Add Modifier › Generate › Mirror**, axes **X** and **Y**.
   - *You should see*: four slender posts on tall pedestals.

**The soko penanggap object**, a second object:
4. **The umpak.**
   - Cube **0.30 × 0.30 × 0.26**, apply scale.
   - Edit Mode: top face, **Scale 0.667** (0.20 at the top).
   - Centre at z **0.73**. *Why 0.26 m*: half the post's perimeter, ½ × 4 × 0.13.
5. **The post**: **0.13 × 0.13 × 2.19**, top face scaled **0.923**, centre at z **1.955**, so it stands on the umpak (0.86) with its top at 3.05.
6. **Three of each in the quadrant**: select the umpak and post, then **Mesh › Duplicate** (Shift+D, right-click) and move them, so they stand at **(4.7, 4.7)**, **(4.7, 1.567)** and **(1.567, 4.7)**.
7. **Mirror** on X and Y.
   - *You should see*: all 12, the ring about 1.5 m lower than the soko guru. Posts grow taller towards the centre, the joglo rule.
   - Note: the posts at (4.7, 1.567) and (1.567, 4.7) mirror to (4.7, −1.567) and so on. With the corner, each side gets 4 posts, as in the plan.

*Script difference*: the script makes the four soko guru umpak a **padma pedestal** (`stone.padma`), 0.72 tall, in four stacked parts:

| Part | Size (m) | z (m) |
|---|---|---|
| Foot, a chipped slab | 0.56 × 0.56 × 0.07 | 0.60 → 0.67 |
| Shaft (dado), a chipped block, top scaled 0.94 | 0.48 × 0.48 × 0.41 | 0.67 → 1.08 |
| Lotus band: two staggered rows of eight petals, then a neck | about 0.5 across, 0.21 tall | 1.08 → 1.29 |
| Seat for the post | 0.21 square, 0.03 tall | 1.29 → 1.32 |

To make the lotus band by hand:
- Use a Cylinder with 32 sides.
- Add loop cuts (Loop Cut tool in the toolbar, Ctrl+R) and scale each ring to the profile: wide at the bottom, narrowing to the seat.
- Then **Select › Checker Deselect** on a ring and scale the remaining vertices out, which makes petals.
- Try it once the basics feel easy; the plain pedestal from step 1 is correct in size without it.

## Part 3: the beams

Make each beam as a long thin cube along X, then mirror it.

1. **Blandar on the soko guru.**
   - Cube, Dimensions 3.8 × 0.22 × 0.24, apply scale.
   - Edit Mode, select all: G Y −1.6, G Z 4.72.
   - Duplicate it (Shift+D, right-click) and rotate the copy 90° about Z: **Mesh › Transform › Rotate** (R Z 90). Then move it so it runs along Y at x = −1.6: it now sits at (0, −1.6), so G X −1.6, G Y 1.6.
   - Mirror on X and Y.
   - *You should see*: a square frame resting on the four soko guru.
2. **Blandar on the outer ring.** The same at Dimensions 10.0 × 0.15 × 0.2, at ±4.7, centre z 3.15.
3. **The sunduk.**
   - *What*: a tie beam that passes *through* the soko guru below their tops, stiffening them.
   - The same as the blandar, at Dimensions 3.84 × 0.165 × 0.22, at ±1.6, centre z 4.05. The Kawruh Kalang makes it as thick as the blandar, its other side ¾ of that.
   - It sticks out past the posts. Add a small **kili** peg near each end: a cube 0.05 × 0.09 × 0.32 at x ±1.84, centre z 4.05.
4. **Tie beams** from the outer ring to the soko guru: cubes 3.1 × 0.12 × 0.17, running from y = −4.7 to −1.6 (centre y −3.15) at x = ±1.6, centre z 3.15, and the same on the other sides. Use Mirror.

## Part 4: the tumpang sari (the stepped ceiling)

- *What*: four square frames stacked on the soko guru. Each frame's **outer edge steps outward** going up, to carry the rafters, and its **inner edge steps inward**, which makes the stepped ceiling you see when you look up.
- *Why it matters*: it is the heart of a joglo, and the space under it is the most sacred spot.

For each tier:
1. **Add › Mesh › Plane**, Dimensions = 2 × outer half-size, apply scale. Set Location Z to the tier's bottom.
2. Edit Mode, face selected: **Face › Inset Faces** (I).
   - Open the small **Adjust Last Operation** panel at the bottom left of the viewport.
   - Type **Thickness** = outer − inner half-size.
   - *You should see*: an inner square inside the outer one.
3. Delete the inner face: select it, then **Mesh › Delete › Faces** (X, Faces). A square ring is left.
4. Give it height with **Add Modifier › Generate › Solidify**: Thickness 0.15, Offset **+1**, so it grows upward from the bottom. If it grows down, set Offset −1 or flip the normals with **Mesh › Normals › Flip**.

| Tier | Inner half | Outer half | Plane size (2 × outer) | Inset thickness | Bottom z |
|---|---|---|---|---|---|
| 1 | 1.48 | 1.89 | 3.78 | 0.41 | 4.84 |
| 2 | 1.29 | 2.06 | 4.12 | 0.77 | 4.99 |
| 3 | 1.10 | 2.23 | 4.46 | 1.13 | 5.14 |
| 4 | 0.91 | 2.40 | 4.80 | 1.49 | 5.29 |

5. **The board on top**: a cube 1.88 × 1.88 × 0.05 at Location Z 5.435. It closes the centre.
6. **The nanasan**, the carved pendant hanging from the board's centre:
   - **Add › Mesh › UV Sphere**: Dimensions 0.24 × 0.24 × 0.38, Location Z 5.11.
   - A **Cylinder** above it, Radius 0.05, Depth 0.12, Location Z 5.35. Set these in the Adjust Last Operation panel right after adding.
   - A **Cone** under it, Radius 1 = 0.004 and Radius 2 = 0.07, Depth 0.16, Location Z 4.86. Radius 1 is the bottom, so the narrow end already points down.
7. **Look up.**
   - Put the view inside: **View › Navigation › Walk Navigation** (Shift+`), or orbit with the middle mouse button.
   - Look up from near the floor at the centre.
   - *You should see*: frames stepping in towards the board, an inverted stepped pyramid.

## Part 5: the roof

### The penanggap (the lower, gentler roof)

A hip roof made from one plane.
1. Add › Mesh › Plane, Dimensions 12.0 × 12.0 (half-size 6.0), Location Z **2.62**, apply scale. The eave is 2.0 m above the floor.
2. Edit Mode, face selected: **Inset Faces** (I) with Thickness **3.6**. The inner square now has half-size 2.4.
3. Lift only the inner face: **G Z 2.79**, which takes it to 5.41.
   - *You should see*: a four-sided sloping skirt, about 38°. It passes just over the blandar of the penanggap ring.
4. Delete the inner face (X, Faces). The top is open; the brunjung covers it.
5. **Thatch thickness**: Add Modifier › Generate › **Solidify**, Thickness **0.3**, Offset **−1**, so the thatch grows inward and downward from the outer surface. If it grows upward, flip the normals (Mesh › Normals › Flip) or use Offset +1.
6. Name it `Roof_Penanggap`.
7. **For the sasak thatch's real relief** ([guide 02](02-texturing-poly-haven.md), "The sasak thatch"):
   - *Finer faces*: before the Solidify, Edit Mode, select all, **Edge › Subdivide** until the faces are about 0.3 m. The script uses 40 points along each eave and a ring every 0.3 m up the slope.
   - *Mark the inner shell*: add an empty vertex group `thatch_inner` (Object Data tab › Vertex Groups › +), then in the Solidify's **Shell** field under Output Vertex Groups, pick `thatch_inner`.
   - *Add the relief*: Add Modifier › Generate › **Geometry Nodes** › `GN_thatch`, below the Solidify.

### The brunjung (the steep top roof with the ridge)

1. Add › Mesh › Plane, Dimensions 5.7 × 5.7 (half-size 2.85), Location Z **5.30**, apply scale.
2. Edit Mode, face selected: **Mesh › Extrude › Extrude Region** (E), then Z, type **3.3**. The new top face is at 8.6.
3. With the top face still selected, turn it into the ridge:
   - **S X 0.351**: 5.7 × 0.351 = 2.0 m, the ridge's length.
   - Then **S Y 0**: the face collapses to a line.
   - *You should see*: a steep hip roof whose top is a short ridge running east-west.
4. Weld the collapsed points: select all, then **Mesh › Merge › By Distance** (M, By Distance).
   - *Why*: the collapse left pairs of points on top of each other.
   - The status bar says how many were removed (2: the four corners collapsed into the ridge's two ends).
5. Delete the bottom face: select it, X, Faces. The brunjung is an open shell like the penanggap.
6. Solidify, Thickness 0.3, Offset −1. Name it `Roof_Brunjung`.
7. **The ridge cap**: a cube 2.8 × 0.44 × 0.3 at Location Z 8.66, plus two small upward cones (Radius 0.12, Depth 0.55) at x ±1.32, z 9.05.

*You should see*: the brunjung stands over the penanggap and overhangs its top edge, with a small step between them. That is the joglo silhouette.

## Part 6: rafters and battens (optional, seen from inside)

- **Rafters (usuk)**:
  1. Make one rafter: a cube 0.07 × 4.55 × 0.09, about the length of the penanggap's slope (3.6 m across, 2.79 m up).
  2. Rotate it about X to the slope, about 37.8° (R X, then type the angle; try −37.8 if it leans the wrong way).
  3. Move it just under the south side of the penanggap.
  4. Add an **Array** modifier: Relative Offset **off**, Constant Offset **on**, Distance X **0.45**, Count about 23.

  *Script difference*: the script cuts each rafter where it meets the hip rafter at the corners. By hand, shorten the corner ones by moving their top vertices, or leave them out; the thatch hides most of them from outside.
- **Hip rafters (dudur)**:
  1. Turn on snapping: the **magnet** icon in the viewport header, with **Snap To › Vertex**.
  2. Make a thin cube.
  3. In Edit Mode, snap one end to the roof's corner and the other to the top corner.
- **Battens (reng)**: thin slats 0.04 × 0.03 across the rafters, one every 0.3 m up the slope. Make one and Array it up the slope with a constant offset along the slope's direction.

## Check against the script model

1. Bring the script's pendopo in: File › Append › `blender/buildings/pendopo.blend` › `Collection` › `Pendopo`. Its umpak and stair come with it, still linked from `blender/objects/stone.blend`, and its posts, linked from `blender/objects/soko.blend`.
2. Toggle its eye icon in the Outliner to compare.

Things that should line up:
- the floor at 0.6
- the umpak tops at 1.32 (soko guru) and 0.86 (soko penanggap)
- the soko penanggap tops at 3.05, the soko guru tops at 4.6
- the tumpang sari's top at 5.44
- the ridge at 8.6

## Pitfalls (learned while building it)

- **Apply the scale** before you add modifiers or Extrude. Otherwise every thickness and distance is multiplied by the hidden scale.
- **The origin decides the Mirror.** Keep the object's origin at the world centre, and move the *geometry* in Edit Mode, not the object.
- **Solidify follows the normals.** If the thickness goes the wrong way, flip the normals or change Offset between −1 and +1.
- **After collapsing a face to a line or point, use Merge By Distance**, or overlapping points remain and shade badly.
- **An umpak is one stone.** A block-masonry texture on it read as a heap of rubble (fixed in the script on 2026-10-18; see guide 02).
- **Don't model inside the generated files** (`blender/buildings/`, `blender/places/`): the scripts overwrite them. Work in your own practice file.
- **No king posts in a joglo.** The steep top roof rests on the tumpang sari. The huts (kampung roofs) do have king posts.
