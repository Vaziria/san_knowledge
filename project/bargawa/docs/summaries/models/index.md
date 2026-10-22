# Bargawa, Chapter 1: Model Design

Claude's model design for Chapter 1: the "Model Design" state of the workflow in [bargawa_game.md](../../../bargawa_game.md). Written 2026-10-20.

**Since the workflow's update on 2026-10-20**, models are designed in Blender (proportion and size, texturing, lighting). Each model's summary is written next to its Blender file, for example `blender/characters/ramabargawa.md` beside `ramabargawa.blend`. The files in this folder are the accepted plan each model starts from. When a model's own summary exists, it takes over from the plan.

It covers every model the scenes in [story.md](../story.md) need, in their game versions. The film-level design of each one (look, costume, sources) stays in [docs/stories/ramabargawa/models.md](../../../../../docs/stories/ramabargawa/models.md) and is not repeated here.

The user's choices behind it:
- characters from MakeHuman (MPFB) bases, dressed in Blender
- 3D in Godot 4
- previews on Windows first, Android after

## What Chapter 1 needs

| Model | File | Scenes | Source | State | Game budget |
|---|---|---|---|---|---|
| C01 Ramabargawa, young (the player) | [ramabargawa.md](ramabargawa.md) | 1–7 | MPFB + Blender | to build | 20k tris, 2K textures |
| C04 Resi Jamadagni | [jamadagni.md](jamadagni.md) | 1–5 (body in 6) | MPFB + Blender | to build | 12k, 1K |
| C05 Dewi Renuka | [renuka.md](renuka.md) | 1–4 | MPFB + Blender | to build | 16k, 2K |
| C06 The four brothers | [brothers.md](brothers.md) | 1, 3, 4 | one MPFB base, four variants | to build | 10k each, one shared 1K set |
| C07 Prabu Citrarata | [citrarata.md](citrarata.md) | 2, only across the lake | MPFB + Blender | to build | 6k, 512 |
| Props: kapak, kendi, japamala, mats, pyre and the rest | [props.md](props.md) | all | Blender, Poly Haven | 1 built (damar), the rest to build | see the file |
| E01–E03 Pertapan Jatisrana, its lake and forest | [jatisrana.md](jatisrana.md) | all | the built Blender place | built in Blender; game version to build | see the file |

**Not modelled for Chapter 1**, because the story keeps them off-screen:
- **C08 Maespati soldiers**: only a broken tombak in the gate (S5).
- **A01 the goats**: heard in the kandhang in S1; the pen is empty with a broken fence in S5.
- **A03 lake birds**: sound only.
- Everyone else from models.md (C02, C03, C09–C14) belongs to later chapters.

## Character pipeline (all characters)

Everything is made by script, like the environment: `run.ps1 character <name>` builds `blender/characters/<name>.blend` and its game export. MPFB stands in for the hand-made body.

1. **Body from MPFB**: MakeHuman's Blender extension, MPFB 2. It needs Blender 4.2 or newer; it must be tested on our Blender 5.2 first.
   - The script sets the macros: age, gender, muscle, weight, height, proportions, and an Asian mix for a Javanese look.
   - Then the skin material, eyes, eyebrows and eyelashes.
   - The heights come from models.md, nearer Javanese stature since the proportion check of 2026-10-20 (Indonesian adults about 166 / 154 cm; the huts' doors 1.89 m): Ramabargawa 1.80 m, his brothers 1.68–1.74 m, Jamadagni 1.66 m, Citrarata 1.74 m, Renuka 1.55 m.
   - MakeHuman's core assets are CC0 ("you are free to do as you see fit with the asset or derivates of the assets", [MakeHuman license](https://static.makehumancommunity.org/about/license.html)). Its code is GPL, which does not reach the models.
2. **Rig**: MPFB's game-engine rig (MPFB 2.0 beta 1 release notes), which is lighter than its default or Rigify rigs. In Godot the skeleton is mapped to the humanoid profile (`SkeletonProfileHumanoid`), so every character shares one animation library.
3. **Costume by script in Blender**: kain, sabuk, upawita, jarik, kemben, selendang, jewellery and hair (gelung, sanggul). Each piece is its own mesh, with its weights taken from the body. Hidden body parts under the cloth are deleted, to save triangles and stop them poking through.
4. **Long cloth** (the end of a kain, the selendang, the jarik's hem): short chains of extra bones. In Godot they swing with `SpringBoneSimulator3D` (there since Godot 4.5; to be checked in 4.7). No real cloth simulation on phones.
5. **Faces**:
   - Shape keys for blink, speech (open and round), calm, grief, rage and pain.
   - MPFB poses faces with bones in its default rig. Whether those poses can be baked into shape keys for the game rig is **the first thing to test**, because the faces carry the story.
6. **Export**: one GLB per character, with its textures packed into one set (base colour, normal, and roughness with ambient occlusion).

**Animation**: an open decision for the user.
- Walking, standing, carrying and kneeling could come from Mixamo, a free library that needs an Adobe account. Its terms must be checked before use. The other way is keying them in Blender.
- Either way, the Javanese gestures are keyed in Blender, because no library has them:
  - sembah (palms together to the face)
  - sila (sitting cross-legged)
  - timpuh (a woman's kneel)
  - semedi (meditation)
  - the bow of "sendika dhawuh"

## Budgets (first targets, measured in the first preview)

The Windows preview runs on this PC: an AMD Radeon iGPU with 512 MB of shared VRAM and 7.4 GB of RAM. That is close to a mid-range phone, so the same budget fits both.

- **30 frames a second** at a 3D render scale of 0.75 of 1280×720, with Godot's Mobile renderer.
- **On screen at once**: at most about 350k triangles and 300 draw calls. Characters take up to 100k of that (scene 3 has seven people).
- **Textures**: 1K by default, 2K only for the faces and surfaces seen close (Ramabargawa, Renuka, the pendopo's soko and umpak). Compressed on import (S3TC/BPTC on Windows, ASTC on Android).
- **Memory**: under 1.5 GB for the game while it runs.

## Open decisions (for the user)

1. **Animation source**: Mixamo for the common moves plus Blender for the Javanese ones, or everything keyed in Blender.
2. **Ramabargawa's skin**: natural brown or ash-grey, as in the wayang puppet. The preview shows both tones.
