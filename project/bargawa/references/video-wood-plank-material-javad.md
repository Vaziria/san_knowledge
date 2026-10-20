# Video: a weathered wood plank material (Javad Rajabzade)

An external reference the user added for texturing wood (2026-10-18).

| | |
|---|---|
| **Video** | [Substance Painter Tutorial, Wood Plank Material Breakdown](https://www.youtube.com/watch?v=jQIoM0FRgFM) |
| **Channel** | Javad Rajabzade (sells wood smart material packs on ArtStation; this one is free there) |
| **Published** | 2023-07-31, 34:42 long |
| **Tool** | Adobe Substance 3D Painter, on a flat plane |

**How this summary was made**:
- The video has **no narration** (98 caption words, all "[Music]" or "thank you").
- So it was summarised from **one frame every 30 seconds** at 720p (70 frames). Layer names, masks and the result are readable at that size; slider values are not.

## What it builds

A **smart material of weathered vertical wood planks**, made only from procedural layers, with no photo of a plank:
- warm brown grain with fine fibres
- planks of slightly different tones, with dark gaps between them
- lighter worn edges along the gaps
- dirt specks and water stains, darker streaks
- a final colour balance

Because it is a stack of masks, it can be re-coloured and re-used on any wooden object.

## The layer stack, in the order it is built

| Time | Layer (as named in the video) | What it adds |
|---|---|---|
| 0:00–0:30 | **Wood American Cherry**, then **Wood Rough** (Substance materials, fill layers) | The base wood colour and grain, the colour pulled warmer |
| 1:00–2:30 | **Wood fibers**: a fill whose mask is a procedural *Directional Scratches* / directional noise | Fine fibres along the grain (colour and a little height) |
| 3:00–4:00 | Colour fills; a *Grunge Plaster Damaged* mask | Darker, dirtier patches |
| 4:30–5:30 | A **Gradient** filter (a gradient map) | Re-colours the grain through a set of browns |
| 6:00–8:30 | **Brick / Tile generator** for vertical planks, **Levels**, a **plank anchor** (anchor point) | A mask of separate planks, saved as an anchor for later layers |
| 8:30–13:00 | **Plank color variation** (random per plank); masks from *Grunge Map*, **Warp**, **Blur**, Levels, and a hand-painted stroke layer | Each plank a little different; uneven dark **gaps** between planks |
| 13:00–17:30 | Fills with colour, roughness and height, masked by *Directional Noise* and *Grunge Map 001* | Grain variation per plank, the gaps sunk in height |
| 18:00–20:30 | **Dirt**, **Moisture Noise**, *Grunge Spots Dirty* | Dirt specks, damp patches, roughness variation |
| 21:00–22:00 | **Wood fibers copy** with *Directional Scratches*, Warp, Blur | A second, finer layer of fibres |
| 22:30–26:30 | A **Mask Builder** generator (from the anchor's height), a cream colour fill | **Worn, lighter edges** along every plank, a roughness pass |
| 27:00–30:00 | **Levels – Roughness**, *Grunge Map 002* and *Crystal 2* masks, painting, a **Compare Mask** | Larger stains and dark streaks running down the planks; areas picked by height |
| 30:30–32:30 | A reddish tint fill, **Edge damage**, *Directional Noise 2* | Colour depth, chipped edges |
| 33:00–34:00 | **Color Balance** and **Contrast / Luminosity** filters | The final grade |

## The method in short

1. **Base wood**, then **fibres** from directional noise.
2. **Re-colour** through a gradient.
3. **Planks** from a tile generator, kept as an anchor so later layers can use the plank mask.
4. **Variation per plank**: colour, grain.
5. **Gaps**: dark and lower in height.
6. **Worn edges**: lighter, driven by the plank mask's height.
7. **Dirt, damp, stains, streaks**: grunge masks.
8. **Roughness**: its own pass.
9. **A final colour balance.**

Every effect is a **fill layer with a mask**: colour, roughness and height change together, so a worn edge is lighter, rougher and slightly raised at once.

## What we can use for Ramabargawa

Our wood (posts, beams, tumpang sari, rafters, the huts' doors and frames) is one box-projected Poly Haven texture, `wood_table_worn`, mixed with a palette colour (`scripts/common/textures.py`). With no Substance Painter, the same layers can be built **in Blender's shader nodes**:

| Video layer | In Blender |
|---|---|
| Fibres along the grain | a **Noise Texture** stretched along the piece's length (a Mapping scale like 1 : 1 : 25 on the long axis), mixed in lighter or darker |
| Re-colour through a gradient | a **Color Ramp** on the texture's brightness, with our palette's teak browns |
| Plank colour variation | a **random value per piece**. Our builder already writes a value per point (`wear`); one more attribute, the same for every point of a post or beam, gives each its own tone through Object Info › Random or an Attribute node |
| Gaps between planks | **geometry**: our planks (door leaves, floors) are separate pieces with real gaps, so the shadow is real |
| Worn lighter edges | the **`wear` attribute**, as on the stone: round and chip the timber edges too, and tint and roughen them |
| Dirt, damp, stains | **Musgrave / Noise / Voronoi** masks through a Color Ramp; for streaks, the noise stretched downward; darker near the floor (a height gradient), such as dirt at the foot of each soko |
| Roughness pass | the roughness map through its own Color Ramp, rougher in dirt and on worn edges |
| Final balance | a **Hue/Saturation/Value** or **RGB Curves** node at the end, per light state |

**For the story**, layers like these also give us the **ages of the wood**:
- **S1, the family whole**: clean, oiled teak.
- **S6**: rain-darkened timber.
- **S5**: charred black.
- **The waiting years**: grey and weathered.

These could be the same material with a few masks turned up or down.

**Used** (2026-10-18): our teak now carries this layer stack, rebuilt in nodes (`scripts/common/textures.py`, `weathered_wood`):
- gradient re-colour
- fibres
- worn edges from a second UV (`EdgeUV`)
- dirt from the foot
- damp streaks
- final balance
- a roughness pass

Step by step by hand: [guide 02, "Weathered teak"](02-texturing-poly-haven.md).
