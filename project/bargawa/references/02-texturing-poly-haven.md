# 02 Texturing with Poly Haven

Put real, scanned textures on a model by hand, the same way the scripts do (`scripts/common/textures.py`, list in `scripts/common/polyhaven.py`). Practise on the pendopo you modelled in [01](01-modelling-pendopo.md).

**You will learn**:
- what PBR texture maps are
- the Shading workspace and its nodes
- Box projection (texturing without a UV map), and the Mapping node for texture size
- roughness and bump
- mixing in a palette colour to keep the look stylized
- the usual route with a UV map, for comparison
- using the project's texture library from the Asset Browser

## What the maps are

Poly Haven ([polyhaven.com](https://polyhaven.com), all free, CC0) gives each texture as several images of the same surface:

| Map | What it holds | Colour space |
|---|---|---|
| **Diffuse** (base colour) | the colour of the surface | sRGB |
| **Rough** (roughness) | how matte (white) or shiny (black) each spot is | **Non-Color** |
| **Displacement** (height) | how high each spot is (white high, black low), used here for bump | **Non-Color** |
| Normal (not used here) | the surface's tiny slopes; needs a UV map | Non-Color |

*Non-Color* tells Blender the image holds numbers, not colours. Leave it as sRGB and the roughness and bump come out wrong.

**The files are already here**: `project/bargawa/blender/assets/polyhaven/<id>/<id>_<Map>_1k.jpg`, fetched by `run.ps1 fetch`. To get a new one by hand:
1. Open the texture's page on polyhaven.com.
2. Choose **1K** and **JPG**.
3. Download the Diffuse, Rough and Displacement maps into that folder.

## Box projection or a UV map?

- **A UV map** unfolds the model flat, like a paper pattern, so the image knows where to go. It is the usual way, and it allows a normal map, but every object needs unwrapping.
- **Box projection** throws the image onto the object from six sides (front, back, left, right, top, bottom) and blends the edges. It needs **no UV map**: right for buildings made of boxes, and what our scripts use.

This guide uses box projection; the UV way is at the end.

## One material, step by step: the thatch

Since 2026-10-20 the project's thatch is your own sasak photo, with real relief (see "The sasak thatch" below). The walk-through here, with Poly Haven's `reed_roof_04`, still shows the method for any Poly Haven texture.

1. **Open the Shading workspace.**
   - *Where*: the **Shading** tab at the top of the window.
   - *You should see*: the 3D view in Material Preview mode, the **Shader Editor** below it, and a file browser on the left.
2. **Select the roof** (`Roof_Penanggap`) and give it a material.
   - *Where*: **Properties editor › Material tab (red sphere)**, button **New**. Rename it `M_thatch`.
   - *You should see*: in the Shader Editor, two nodes joined: **Principled BSDF** (the surface) and **Material Output**.
3. **Texture Coordinate.**
   - *What*: tells the textures which positions to read.
   - *Where*: in the Shader Editor, **Add › Input › Texture Coordinate** (Shift+A).
   - We use its **Object** output: positions in the object's own space, in metres.
4. **Mapping.**
   - *What*: scales the texture.
   - *Where*: **Add › Vector › Mapping**. Join **Texture Coordinate: Object** to **Mapping: Vector**.
   - Set **Scale** X, Y, Z = **1 ÷ the tile size in metres**. The thatch tile is 1.0 m, so 1, 1, 1. A stone tile of 2 m would be 0.5.
   - *Why*: the size of the straw, wood grain or stones should look right next to a 1.7 m person. A tile too large looks smooth: at 1.6 m the thatch lost its straw.
5. **The colour image.**
   - *Where*: **Add › Texture › Image Texture**, then **Open** › `reed_roof_04/reed_roof_04_Diffuse_1k.jpg`. Join **Mapping: Vector** to its **Vector**.
   - In the node, change **Flat** to **Box**, and set **Blend** to **0.3** so the six sides fade into each other.
   - *You should see* (Material Preview: the viewport header's sphere buttons, or Z): straw on the roof from every side.
6. **Mix in the palette colour.**
   - *What*: blends the photo with our chosen colour, so the palette, not the photo, sets the mood.
   - *Where*: **Add › Color › Mix Color**, blending mode **Mix**.
   - Join the image's **Color** to **A**. Set **B** to the palette colour (click the swatch, then type the **Hex**, for the thatch **#967A4E**). **Factor 0.25** means 25 % palette.
   - Join **Result** to **Principled BSDF: Base Color**.
7. **Roughness.**
   - Add another **Image Texture** with `reed_roof_04_Rough_1k.jpg`. Set its **Color Space** to **Non-Color**, projection **Box**, Blend **0.3**, and feed it from the same Mapping.
   - Join its **Color** to **Principled BSDF: Roughness**.
8. **Bump**, small relief without changing the shape.
   - Add an **Image Texture** with `reed_roof_04_Displacement_1k.jpg`: **Non-Color**, Box, Blend 0.3, from the same Mapping.
   - **Add › Vector › Bump**. Join the image's **Color** to **Bump: Height**. **Strength 0.9** for the thatch, **Distance 0.02**.
   - Join **Bump: Normal** to **Principled BSDF: Normal**.
   - *You should see*: the straw looks raised and catches the light.

The chain:

```
Texture Coordinate (Object) ─ Mapping (Scale = 1/tile) ─┬─ Image: Diffuse (Box) ─ Mix Color A ─┐  B = palette hex
                                                         ├─ Image: Rough (Non-Color) ───────────┼─ Principled: Roughness
                                                         └─ Image: Displacement (Non-Color) ─ Bump (Height) ─ Principled: Normal
                                                                                     Mix Color Result ─ Principled: Base Color
```

## All the project's materials

The same chain for each, with these values (as in `scripts/common/polyhaven.py`):

| Material | Used on | Poly Haven id | Tile (m) | Mapping scale | Palette mix | Palette hex | Bump strength |
|---|---|---|---|---|---|---|---|
| thatch | roofs | until 2026-10-20 `reed_roof_04`; now the sasak photo (below) | 1.0 | 1.0 | 0.25 | #967A4E | 0.9 |
| woven | bamboo walls (gedhek) | `bamboo_wall_03` | 1.0 | 1.0 | 0.15 | #AA9C82 | 0.3 |
| teak | posts, beams, tumpang sari | `teak_veneer` (**UV**, see below) | 1.2 | 0.833 | 0.25 | #5C3A22 | 0.15 |
| teak_dark | rafters, carved lips, kili | `teak_veneer` (**UV**) | 1.2 | 0.833 | 0.5 | #3A2416 | 0.15 |
| stone | plinth, steps, umpak | `rock_surface` | 2.0 | 0.5 | 0.3 | #807C74 | 0.4 |
| stone_dark | shrine, foot courses | `rock_wall_08` | 1.6 | 0.625 | 0.25 | #565450 | 0.5 |
| bark | banyan, trees | `tree_bark_03` | 1.8 | 0.556 | 0.55 | #5C5246 | 0.5 |
| bark_dark | hanging roots | `bark_platanus` | 1.4 | 0.714 | 0.2 | #3E362E | 0.4 |
| earth | paths, kitchen floor | `forest_ground_04` | 2.5 | 0.4 | 0.25 | #785C42 | 0.3 |
| ground | the terrain | `brown_mud_leaves_01` | 4.0 | 0.25 | 0.35 | #505C34 | 0.3 |

**A faster way to make the others**: in the Material tab, click the number beside the material's name to make a single-user copy, rename it, and in the Shader Editor swap the three images and the values. Or select the nodes, copy them with **Node › Copy** (Ctrl+C), and paste them into another material (Ctrl+V).

**Several materials on one object**:
1. Add slots with the **+** in the Material tab.
2. In Edit Mode select the faces, pick a slot and press **Assign**.

Or keep each material on its own object (posts, roof, plinth), which is simpler while learning.

## Using the texture library

Every material above is ready-made in the texture library, `project/bargawa/blender/textures/`, one file per family, with a summary beside each file:
- wood: teak, teak_dark, teak_turned, firewood
- stone: stone, stone_kit, stone_dark
- plants: bark, bark_dark, thatch, woven
- ground: ground, earth, water, ground_spiky, ground_soil (each with its Geometry Nodes, see "The spiky ground" and "The soil" below)
- metal: bronze
- palette: the plain colours (leaves, flowers, clay…)

The scripts link every model's materials from these files. You can do the same, instead of building the nodes again.

1. **Open the Asset Browser.**
   - *What*: Blender's library window. It shows the assets of the folders registered in the preferences.
   - *Why*: a finished material is dragged onto a model in one move.
   - *Menu*: switch any editor to **Asset Browser** (the editor-type icon at its top left). Then pick the library **Bargawa Textures** in the list at its top left. `run.ps1 register` registered it once.
   - *You see*: the catalogs on the left (Bargawa › Wood, Stone, Plants, Ground, Metal, Palette), and the materials of the one you click. A material with no picture has no preview in its file yet: the scripts don't render previews.
2. **Drag a material onto an object** in the 3D Viewport.
   - *What*: links that material from its library file into your file.
   - *You see*: the object takes the material. Switch the viewport to Material Preview (Z › Material Preview) to see the texture. In the Material tab, a chain icon beside the name says it is linked.
   - *Without the Asset Browser*: **File › Link** (no shortcut), open `blender/textures/wood.blend`, then the `Material` folder, pick `M_teak`, and **Link**. Then pick it in the object's Material tab.
3. **To look at a texture**, open its family file. The collection `Samples` holds a block and a post for each material.
4. **To change a texture for everyone**, change its values in `scripts/common/polyhaven.py` (tile, mix, bump) or `scripts/common/textures.py` (the layers). Then rebuild its family (`run.ps1 texture wood`), and the objects, buildings and places that use it. The library files are rebuilt by script, so changes made by hand inside them are lost.
5. **To try changes by hand** in your own file: in the Material tab, click the chain icon to make the material local, then edit your copy. It no longer follows the library.

## Wood: the grain must follow each piece

**Box projection is wrong for timber.** It throws the texture from the six sides, so it cannot know which way a post or a beam runs, and the grain ends up crosswise or swirled. That is what our soko looked like until 2026-10-18: dark, blotchy, "like leather". Real timber always has its **grain along its length**. So wood gets **UVs**, with each piece's length along the texture's grain:
- **The texture**: `teak_veneer` (real teak, straight grain). In its image the grain runs **horizontally**, so each piece's length must run along U (the UV editor's horizontal axis). The earlier `wood_table_worn` is a polished tabletop with swirly figure, wrong for posts.
- **By hand, per piece** (a post, a beam):
  1. Select it and Tab into Edit Mode.
  2. **UV › Cube Projection**, or Smart UV Project.
  3. In the **UV Editing** workspace, select all and rotate the islands (R 90) until the long sides run left to right, along the grain.
  4. Scale them so the texture's size looks right: 1 m of the piece is about 0.83 of the image.
  5. Move each piece's islands to a different spot (G), so two posts never show the same knots.
- **In the material**: **Texture Coordinate: UV** into the Mapping (Scale 0.833), projection **Flat**.
- **A tone per piece**:
  - The script stores a value per piece, `tone` (Attribute node), and feeds it through **Math › Multiply Add** (× 0.36 + 0.82) into a **Hue/Saturation/Value** node's **Value**. Each post or beam ends up a little lighter or darker (0.82–1.18).
  - By hand: duplicate the material with a slightly different Value, or skip it.
- **What the script does** (`scripts/common/mesh.py`, `_grain_uvs`): for every separate piece in a timber material, it finds the piece's longest direction from its points. It then gives each face U = position along that direction and V = position across the face (both in metres), plus a random offset and tone per piece. You get the same result as unwrapping each piece by hand.

## Weathered teak: the wood video's layers in nodes

Our teak is not just a texture. It is the layer stack of the [wood plank video](video-wood-plank-material-javad.md), rebuilt in Blender's Shader Editor on top of the teak texture (`scripts/common/textures.py`, `weathered_wood`). Each layer below is: what it is, the nodes (Add menu), the values, and what you see. Build them in order, each one taking the colour from the step before.

0. **Start**: the teak material from "Wood" above, ending in the per-piece tone (Hue/Saturation/Value).
1. **Re-colour through a gradient**, the video's Gradient filter.
   - *What*: maps the texture's light and dark onto our own range of teak browns, so the palette decides the colours.
   - *Nodes*: **Converter › RGB to BW**, then **Converter › Color Ramp** with three stops: 0.05 = #342014, 0.40 = #68482C, 0.80 = #967048. Then **Color › Mix Color**, Factor **0.55**: A = the colour so far, B = the ramp.
   - *You see*: richer, more even browns.
2. **Fibres along the grain**, the video's "Wood fibers".
   - *Nodes*: from the Mapping of the UVs, another **Vector › Mapping** with Scale **0.6, 40, 1** (stretched along U, the length); then **Texture › Noise Texture**, Scale 6, Detail 6; then **Converter › Map Range** (Smooth Step), 0.48 → 0.62 to 0 → 1.
   - Then darken by it: a **Mix Color** set to **Multiply** with B = mid grey (0.62), its Factor = fibres × **0.45** (Math › Multiply).
   - *You see*: fine dark streaks running along each post and beam.
3. **Worn, lighter edges**, the video's Mask Builder and Edge damage.
   - *What*: the long edges of every piece are rubbed lighter.
   - *Nodes*: the script gives each face a second UV, **EdgeUV** (x = 0 at one long edge to 1 at the other, y = the face's width): **Input › UV Map** (EdgeUV), then **Converter › Separate XYZ**.
   - Distance to the nearest edge: Math **Minimum**(x, 1 − x) × y. Then **Map Range** (Smooth Step) **0.022 → 0.004** to 0 → 1, so the edge is 1, fading out within about 2 cm.
   - Break it up with a Noise along the length (Mapping 2, 0.5, 1; Noise Scale 4; Map Range 0.35 → 0.6) and multiply.
   - **Mix Color** toward **#AA8860**, Factor = edge × **0.5**.
   - At 0.7 the edges looked like painted lines.
   - *By hand without EdgeUV*: Edit Mode, select the long edges, **Edge › Bevel** them 1 cm, put the bevel faces in a lighter material, or paint them in Vertex Paint.
4. **Dirt rising from the foot**, the video's Dirt and Grunge Spots.
   - *Nodes*: **Input › Texture Coordinate › Object**, **Separate XYZ › Z**, **Map Range** (Smooth Step) **2.0 → 0.9** to 0 → 1, so there's none above 2 m and full dirt at 0.9 m. Multiply by a blotchy Noise on Object (Scale 3.5, Detail 5, Map Range 0.35 → 0.65).
   - **Mix Color** toward **#2C221A**, Factor = dirt × **0.6**.
   - *You see*: the foot of every soko darker and uneven, like years of splashing and hands.
5. **Damp streaks running down**, the video's Moisture Noise and streaks.
   - *Nodes*: Object coordinates through **Mapping** Scale **9, 9, 0.7** (stretched along Z), **Noise** Scale 2.5, **Map Range** 0.56 → 0.70.
   - **Mix Color › Multiply** with (0.72, 0.70, 0.68), Factor = streaks × **0.5**.
6. **Final balance**, the video's Color Balance: a **Hue/Saturation/Value** with Saturation **0.9**, into Principled Base Color.
7. **Roughness pass**: the texture's Rough map through **Map Range** 0 → 1 to **0.5 → 0.78**. Then **add** edge × 0.15 and dirt × 0.12 (worn and dirty spots are rougher), cap at 1 (Math › Minimum), into Principled Roughness.

**For the story later**: the same stack, with the dirt and streaks turned up and the browns pushed grey, gives the weathered teak of the waiting years. With everything pushed to black, it gives the burned pendopo of S5.

## The spiky ground: a photo as height, in Geometry Nodes

Your look from `blender/textures/Ground_2.blend` (2026-10-20): a photo's brightness, through a Color Ramp, pushes a fine grid up into spikes. In Jatisrana it covered the yard round the pendopo and the huts (the grass was taken out there) and the far bank round Citrarata's spot, until your soil took its place (2026-10-21, see "The soil" below). It is still in the library. The photo is `blender/assets/photos/ground_spiky.jpg`.

**Your shader version** (what you built by hand):
- Image Texture → **Converter › Color Ramp** (black at 0.0, white at 0.427) → **Vector › Displacement** (Height; Midlevel 0, Scale 0.1) → Material Output's **Displacement** input.
- Material tab › Settings › Surface › Displacement: **Displacement and Bump**.
- It shows in the viewport and in renders. But it is *shader* displacement: the mesh itself stays flat, so anything that reads the mesh never sees the spikes (the game export, the proportion check).

**The project's version** is the same chain as a **Geometry Nodes modifier**, so the spikes are real geometry. Build it in this order:

1. **A fine, flat grid.**
   - *What*: one spike per point, so the grid's spacing is the spikes' spacing. Your plane had a point every 7 cm.
   - *Menu*: **Add › Mesh › Grid** (Shift+A), with X and Y Subdivisions = size ÷ 0.07. Leave it flat-shaded (**Object › Shade Flat**): the crisp facets are part of the look.
   - *In the project*: the grid is 14 cm and the modifier halves it once (Detail 1).
2. **UVs that count tiles of the photo.**
   - *What*: the script sets U = x ÷ 2.86 m and V = y ÷ 4.5 m from the world's origin. That is the photo's own shape (689 × 1085 px), not squashed, and the photo runs on unbroken from one patch to the next.
   - *By hand*: Edit Mode, select all, **UV › Project from View** while looking straight down (numpad 7), then scale the islands in the UV Editor.
3. **The modifier.**
   - *Menu*: Properties › Modifiers (wrench) › **Add Modifier › Generate › Geometry Nodes**, then pick **GN_ground_spiky** in its node-group field. To get the group in another file: **File › Link**, `blender/textures/ground.blend`, `NodeTree`, `GN_ground_spiky`.
   - *Its inputs* (per object, in the modifier panel): **Height** 0.1 m, **Midlevel** 0, **Detail** 1.
   - *Inside* (the **Geometry Nodes** workspace):
     - **Subdivide Mesh** (Level = Detail).
     - Then **Set Position**, its Offset = **Normal** × (**Color Ramp**(**Image Texture** at the UVMap) − Midlevel) × Height × the `spikes` weight.
     - The Image Texture's extension is **Mirror**: every other tile is flipped, so the photo's edges meet without a hard seam.
   - *You see*: the grid rises into spikes, the same ones your shader made (checked side by side, 2026-10-20).
4. **Where the spikes grow: the vertex group `spikes`.**
   - *What*: weight 1 = full height, 0 = flat. The script sets 0 by the paths (no spike pokes through them) and at the water's edge. A mesh without the group gets the full height everywhere.
   - *By hand*: **Weight Paint** mode (Ctrl+Tab), with `spikes` picked in Object Data › Vertex Groups. Paint to add spikes, and use Subtract to remove them.
5. **The material `M_ground_spiky`.**
   - The photo on the UVMap (extension Mirror) straight into Base Color: the movie aims for realism, so no palette colour is mixed in.
   - **Roughness 0.85, Metallic 0, IOR 1.5**: matte, dry earth.
   - **Bump** (Strength 0.3) from the same Color Ramp, for the detail between the points.
   - **Alpha** = 1 − the mesh's `fade_out` attribute, which is 1 at a patch's outer edge. The edge turns see-through (dithered), so the ground under it shows and the patch melts into it.

**To change it for everyone**:
- The look: `SPIKY` in `scripts/common/terrain.py` (tile, ramp, height, grid, roughness).
- Where it grows: `YARD`, `FAR_BANK_R` and `BARE_FADE` in `scripts/common/jatisrana.py`.
- Which look Jatisrana wears: `BARE` in `scripts/common/terrain.py` (`"ground_soil"`, or `"ground_spiky"` for the spikes again).
- Then rebuild with `run.ps1 texture ground`, then `run.ps1 place jatisrana`.

**To try it by hand**: change a patch's Height, Midlevel or Detail in its modifier panel, or paint its `spikes` group. The group's inside (the Color Ramp) is linked from the library: to edit it in your own file, make it local first (the chain icon).

## The soil: Jatisrana's bare ground

Your `Soil` material in `blender/textures/material_manual.blend` (2026-10-21, "this is better for jatisrana"). It now covers the yard and the far bank, where the spiky ground was (`BareGround_Yard`, `BareGround_FarBank`). The photo is `blender/assets/photos/1cb0165c04eb35eb7bd78df3f83cc534.jpg`, soil with leaf litter, under the file name your material uses.

**Your version** (on a 5.59 m plane, Subdivision Surface › Simple, level 6):
- Image Texture → Principled **Base Color**.
- The same Image Texture → **Vector › Bump** (Height; Strength 1.0, Distance 0.8) → Principled **Normal**.
- Image Texture → **Converter › RGB to BW** → **Color Ramp** (black at 0.655, white at 0.932) → **Displacement** (Midlevel 0, Scale 0.1) → Material Output, set to Displacement and Bump.
- Principled: Metallic 0.177, Roughness 0.445, IOR 1.0.

**The project's version** (`M_ground_soil`, `GN_ground_soil` in `blender/textures/ground.blend`) is built like the spiky ground above, with these differences:
- **The photo is seamless**, so it repeats (extension **Repeat**), once every 5.6 m, as on your plane. The spiky photo needs Mirror.
- **The bump comes from the photo itself**, Strength 1.0, Distance 0.8, as yours. The spiky ground's bump comes from its ramp.
- **Matte shading**: Roughness 0.85, Metallic 0, IOR 1.5, as you chose for the thatch.
  - *What changes*: under the sky the soil reads a little lighter and greyer than on your plane. Your Metallic darkened it and kept it brown.
  - *Same otherwise*: with the same shading values, your plane and the library's material render the same.
- **The relief is set up but flat**: `GN_ground_soil` carries your ramp, which raises no point (see the pitfall on a Color Ramp's stops below). To have the light leaves and pebbles rise, move the ramp's black stop down, for example to 0.3: `ramp` in `SOIL` (`scripts/common/terrain.py`), or in your own file first.

## The sasak thatch: your photo down every slope, with real relief

Your `sasak` material from `blender/textures/material_manual.blend` (2026-10-20) is now the thatch of every roof: the pendopo, both huts, the pawon and the kandhang. The photo is `blender/assets/photos/sasak.jpg`: straw hanging down, in seven courses.

**Your version** (by hand, on the 64 × 64 plane):
- Image Texture into Base Color, and also straight into a **Bump** (Strength 1.0, Distance 0.8).
- Image Texture → **RGB to BW** → **Color Ramp** (0 → 1) → **Displacement** (Scale 0.1, Midlevel 0), set to **Displacement and Bump**.

**The project's version**, on the roofs:

1. **The straw must run down every slope.**
   - *What*: the photo's straw hangs down and its courses run across. On a hip roof like the pendopo's, box projection throws it from above, which is right on the south and north slopes but turns the straw sideways on the east and west ones.
   - *So*: the thatch gets **UVs** (`mesh._slope_uvs`): U along the eave, to the right as you face the slope, and V up the slope, in metres, on every side.
   - *By hand*: select one slope's faces, look straight at it (**View › Align View › Align View to Active**, Shift+numpad 7), **UV › Project from View**, then turn the islands in the UV Editor so the straw hangs down. Repeat for each slope.
2. **The tile.** The photo is 384 × 672 px. At **0.686 × 1.2 m** a course is about 17 cm and a straw about 7 mm, like real alang-alang. (On your plane the photo covered 4.5 m, so its courses were 65 cm.) In the material: **Mapping** Scale 1/0.686 and 1/1.2 on the UVs.
3. **The material `M_thatch`.**
   - The photo into Base Color, with no palette colour: the movie aims for realism.
   - **Roughness 0.85, Metallic 0, IOR 1.5.**
   - **Bump** Strength 1.0, Distance **0.2**. That is your 0.8 scaled to the smaller tile, so it looks as strong as on your plane.
4. **Real relief: `GN_thatch`**, a Geometry Nodes modifier after the roof's **Solidify**.
   - *Finer faces first*: the roofs are built in faces of about 0.3 m (`ROOF_STEP` in `buildings.py` and `joglo.py`). The modifier halves them 3 times (**Detail 3**, so 3.75 cm), then raises each point along its normal by your chain: (Color Ramp(photo) − Midlevel) × **Height 0.1**.
   - *Only the outside*: the Solidify marks its inner shell with the vertex group `thatch_inner` (Output Vertex Groups › **Shell**). The nodes neither subdivide nor raise those faces, so the underside, seen from inside the pendopo, stays light.
   - *The cut edge*: Solidify only copies the UVs onto the eave's cut edge, which stretched the photo into vertical bands like book spines. So the nodes store `thatch_inner_depth` (0 outside, 1 on the underside), and the material moves V down by depth × 0.25 m. The straw then runs on down the edge as straw ends.
   - *Cost*: about 915 000 triangles over all the roofs (the pendopo 624 000), as you chose.
   - *You see*: in the stills, golden courses of straw with real relief at the eaves and ridges, and straw ends at the cut edge.

**To change it for everyone**:
- The look: `THATCH` in `scripts/common/textures.py` (tile, ramp, height, detail, roughness, bump).
- Then rebuild with `run.ps1 texture plants`. The roofs link the material and the nodes, so they change with it. Rebuild the buildings and places only when the meshes change (`ROOF_STEP`, the UVs).

## The usual way, with a UV map

To try it on one object:
1. Edit Mode, select all, then **UV › Smart UV Project**.
2. In the Shader Editor, use **Texture Coordinate: UV** instead of **Object**, and projection **Flat** instead of Box.
3. Now you can also use the **Normal** map: an Image Texture (Non-Color) into **Add › Vector › Normal Map**, then into Principled: Normal, in place of the Bump.
4. Check the layout in the **UV Editing** workspace: no stretched or squashed areas.

## Pitfalls (learned while building it)

- **Non-Color for Rough and Displacement.** In sRGB they come out too light and the surface looks flat and wrong.
- **Apply the scale first** (Object › Apply › Scale). Object coordinates follow the object's scale, so an unapplied scale stretches the texture.
- **Use the Mix Color node** (Add › Color). A plain Mix node set to Float has no colour inputs. In a script, the Mix node has a socket per data type, so pick the colour ones (`textures.py` does).
- **A bright texture overpowers the palette.** The banyan's grey bark almost glowed until 55 % of the palette colour was mixed in.
- **Tile size changes the character.** The thatch at a 1.6 m tile looked smooth; at 1.0 m the straw shows.
- **Wood needs its grain along the length**: box projection (fine for stone, thatch, earth) made the posts look like leather. Use UVs along each piece (see "Wood" above).
- **Check what a texture really is before using it**: open its Diffuse image. "wood_table_worn" was a polished tabletop with swirly figure, not timber.
- **The old Displace modifier can't put a Color Ramp on a photo.** Its texture's Colors › Ramp reads the image's alpha, not its brightness, so every point went to full height. That is why the spiky ground uses Geometry Nodes.
- **A Color Ramp's stops are on linear brightness, not on what the photo looks like.**
  - The soil's ramp starts at 0.655. That is about 0.83 on the photo's own sRGB scale, brighter than almost all of it.
  - Only 0.03 % of its pixels pass it (the brightest is 0.817), and none of your plane's 4,225 points lands on one. So its displacement did nothing: the render is the same with it on or off (checked 2026-10-21).
  - To see what will rise, preview the ramp's output. With the Node Wrangler add-on, Ctrl+Shift+click the Color Ramp: white rises, black stays flat.
  - The points also have to land on the bright spots. A leaf smaller than the grid's spacing (7–9 cm) is mostly missed.
- **A dark photo gives low spikes.** The spiky photo's average brightness is 0.05 (linear). With the ramp's white at 0.427, most spikes are 1–2 cm and only the brightest reach the full 10 cm. Move the white stop left for more tall spikes.
- **Dirt is not metal, and IOR 1.0 reflects nothing.** Keep Metallic at 0. To lose the shine, raise Roughness. IOR 1.0 (air) makes the surface flat and chalky.
- **A directional photo needs UVs on a hip roof.** Box projection throws it from above, so the straw hangs right on two sides and lies sideways on the other two.
- **Solidify copies UVs onto the cut edge**, which stretches the texture into vertical bands. Move the edge's UVs by how deep each point is (see "The sasak thatch"), or give the rim its own material (Solidify › Materials › Rim offset).
- **Keep source images in the project**, not in Downloads. A .blend only stores the image's path unless you pack it (**File › External Data › Pack Resources**). Photos for the project go in `blender/assets/photos/`.
- **Material Preview is not the final light.** Judge colours under the scene's own light (Rendered mode, the viewport header's last sphere button). The light states come later (guide 06).
