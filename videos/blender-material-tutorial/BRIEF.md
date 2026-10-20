---
workflow: general-video
flow: automation
storyboard: no
message: "Material di Blender mengatur tampilan permukaan objek: warna, logam, kekasaran"
destination: youtube + tiktok
aspect: 1920x1080 + 1080x1920
language: id
audience: "Pemula Blender berbahasa Indonesia"
length: 60-75s
angle: screen-tutorial
---

## Intent

A sample screen-recorded Blender tutorial, "Apa itu Material di Blender?", in Bahasa Indonesia.
Claude drives a separate, clean Blender window (factory settings, larger UI text) while ffmpeg
records the screen; the edit then explains each step. The user wants to know whether this way
of making tutorials works ("lets create sample").

The user's words: "we take video screen pc, explain how process we create animation, or explain
blender work"; "because its tutorial, we need cursor focus or keyboard bind show"; "we use bahasa
indonesia in tutorial".

Steps and captions (approved plan):

1. Grey sphere in the viewport: "Apa itu material? Material = tampilan permukaan objek."
2. Z, Material Preview: "Tekan Z, pilih Material Preview agar material terlihat."
3. Click the sphere: "Klik objek untuk memilihnya."
4. Material tab: "Buka tab Material. Bola ini belum punya material."
5. New: "Klik New. Material baru dibuat (Principled BSDF)."
6. Base Color, pick a colour: "Base Color: warna dasar permukaan."
7. Metallic, type 1, Enter: "Metallic 1: permukaan jadi seperti logam."
8. Drag Roughness 0.8 to 0.1: "Roughness kecil = mengkilap, besar = doff."
9. Shading workspace: "Di balik layar, material adalah susunan node."
10. Point at Principled BSDF and Material Output: "Principled BSDF mengatur warna, logam, kekasaran."
11. End: "Material = resep tampilan permukaan objek."

## Customizations

- Mix of Python (scene setup) and real mouse moves, clicks and shortcuts in Blender.
- Key-press labels (e.g. `Z`, `Enter`) added in the edit from the input log.
- Cursor focus: highlight around the cursor, ripple on clicks, zoom that follows the action.
- On-screen captions per step, Bahasa Indonesia.
- Two exports: 16:9 for YouTube, 9:16 for Shorts/TikTok (crop follows the cursor; captions in
  the lower part).

- Built from take 3 (`recording/out/take3.*`); the last explanation is split into two captions:
  "Hasilnya dikirim ke Material Output." then "Material = resep tampilan permukaan objek."
- 9:16 layout: the scene (sphere) on top, caption band in the middle, a close-up of where the
  cursor acts below; key labels sit at the bottom-left of the scene view. 16:9: one view with
  zooms per step, caption bottom-left, key labels just above it.
- Key-label words: Tekan (key), Ketik (typed value), Klik (button), Geser (slider drag).

## Notes

- No music, no voice, no title card, no emoji.
- Captions and key labels reuse the look of the san_* TikTok videos (frame.md "Broadside":
  Barlow, fire-orange accent) — inferred, not asked.
- The recording Blender is separate from the user's projects; nothing in D:\blender_project is
  touched. The user closes their own Blender first (RAM ~900 MB free).
- Nothing is uploaded.
