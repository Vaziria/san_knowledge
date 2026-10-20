# Video: modular stone floor and wall (brainchildpl)

An external reference the user added (2026-10-18).

| | |
|---|---|
| **Video** | [3D Modular STONE FLOOR in Blender, Substance and UE5](https://www.youtube.com/watch?v=sk3ryzQ0Y44) |
| **Channel** | brainchildpl ([brainchild.pl](http://www.brainchild.pl/)) |
| **Published** | 2022-12-19, 51:34 long |
| **Tools** | Blender (sculpting, low-poly modelling, UVs), Substance Painter (texturing), Unreal Engine 5 (material, kit assembly, lighting). Baking was most likely done in Marmoset Toolbag, judging by its interface in the frames; the video doesn't name it |

**How this summary was made**:
- The video has **no narration**, only music; its captions are "[Music]" and its chapter names are YouTube's generic automatic ones.
- So it was summarised from **one frame every 30 seconds** (103 frames) and the keystrokes shown on screen.
- The steps and their order are reliable. Exact settings are not visible at that resolution.

## What it builds

A stylized **modular dungeon kit**: a stone floor tile, a stone wall piece and steps. Each is a **low-poly mesh carrying the detail of a high-poly sculpt**. The pieces are textured once, then snapped together in Unreal into a corner of a room with walls, floor and steps. The end card calls it "Modular Stone Wall: full process".

The look: chunky stones with **chipped, softened edges and small cracks**, light grey with pale edge highlights, under cool blue-violet light. Stylized, not photoreal.

## The process, with timestamps

| Time | Tool | What happens |
|---|---|---|
| 0:00–4:00 | Blender, Sculpt Mode | **Sculpting single stones from cubes**: the edges are knocked off and chipped, the faces flattened unevenly, small cracks pressed in. Several stone variants. |
| 4:00–6:30 | Blender | **Arranging the stones** into floor and wall layouts by duplicating and moving them (Shift+D, G on screen). This is the high-poly source of each modular piece. |
| 7:00–8:30 | Blender | **The low-poly piece**: a simple block the size of the tile, then its **UV layout**, packed with the UVPackmaster add-on (visible in the side panel). |
| 9:00–22:30 | Blender, Edit Mode | **Low-poly modelling over the sculpt**: cuts between the stones with Vertex Connect Path (J), vertices moved to follow the stone edges (G, and G G for Vertex Slide), sharp edges marked (Ctrl+E › Mark Sharp), some Bevels, UV seams (red edges). The result: a light mesh that follows the big shapes of the stones. |
| 23:00–23:30 | Baking tool | **Baking**: the high-poly sculpt's detail is baked onto the low-poly mesh as texture maps (normal map, and probably ambient occlusion and curvature). |
| 24:00–27:00 | Blender, Sculpt Mode | **The wall piece's sculpt**: stones laid in a wall pattern, then detailed. |
| 27:30–33:30 | Baking tool, Blender | The wall's **normal map** (the blue-purple image), then its low-poly mesh, seams and UVs, and test bakes. |
| 34:30–38:00 | Substance Painter | **Texturing**: a grey stone material, lighter worn edges driven by the baked curvature, darker gaps, some colour variation. Then the textures are exported. |
| 38:30–39:30 | Unreal Engine 5 | **Import and material**: mesh plus base colour, normal, and a packed map (the yellow texture: ambient occlusion, roughness and metallic in one image's channels). |
| 39:30–42:00 | Unreal Engine 5 | **Assembling the kit**: walls, floor tiles and steps snapped together into a corner. |
| 42:00–50:00 | Unreal Engine 5 | **Lighting**: point lights in cyan and blue, a key light, colour and atmosphere until the scene glows. |
| 50:30 | | End card. |

## Shortcuts seen on screen

| Keys | What it does | Blender menu |
|---|---|---|
| G, R, S | move, rotate, scale | Object / Mesh › Transform |
| G G | Vertex Slide: slide a vertex along its edges, keeping the shape | Mesh › Transform › Vertex Slide (or Vertex › Slide Vertices) |
| J | Vertex Connect Path: cut a new edge between selected vertices | Vertex › Connect Vertex Path |
| Ctrl+E › Mark Sharp | keeps an edge hard under smooth shading | Edge › Mark Sharp |
| Ctrl+B | Bevel: rounds or chamfers an edge | Edge › Bevel Edges |
| Shift+D | duplicate | Object › Duplicate Objects |
| Tab | Object / Edit Mode | the mode menu in the header |
| A | select all (A A deselects) | Select › All |
| Ctrl+Z | undo | Edit › Undo |

## What we can use for Ramabargawa

Our pipeline is Blender and Poly Haven only, with no Substance, Marmoset or Unreal. What carries over:

1. **Chipped, softened stone edges** are what makes stone read as *stylized cinematic*, not as boxes.
   - Our sanggar pamujan, gate, umpak, plinths and lake steps are still crisp boxes with a texture.
   - The same idea in our build: **bevel and break the edges** of every stone block, and use **one block per stone** instead of one box per wall.
2. **The modular kit**. A few stone pieces can build every stone structure in the story:
   - a block, a step, a coping stone, a floor tile, a corner
   - the hermitage's shrine and gate now, and later the gate of Maespati and the ruins
3. **High to low**: sculpt the detail, bake it onto a light mesh. In Blender:
   - **Sculpt** a stone: Remesh, then the Scrape, Clay Strips, Pinch and Crease brushes.
   - **Bake** onto the low-poly copy with Cycles: Render Properties › Bake, *Selected to Active*, the high-poly selected and the low-poly active, with a small extrusion.
   - Bake a normal map and an ambient occlusion map.
   - We used the same Cycles *Selected to Active* bake in the origami project for its base tile, so it works on this PC.
4. **Edge wear from curvature**: the light edges and dark gaps came from baked curvature. In Blender we can bake ambient occlusion, and use the **Pointiness** attribute (Cycles only) or a baked map, to lighten edges and darken gaps over the Poly Haven rock texture.
5. **Light sets the mood**: half the video is lighting. A plain stone kit becomes atmospheric with coloured points of light and a dark background. This is the same lesson as our light states S1–S7.

**A practice exercise** for later, by hand, following this video's first five minutes:
1. Sculpt one stone block from a cube.
2. Make a low-poly copy.
3. Bake its normal map in Cycles.

A step-by-step guide for it could become [references](README.md) guide 07.
