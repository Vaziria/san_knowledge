---
workflow: general-video
flow: automation
storyboard: no
message: "Blender untuk aset game, dalam tiga tutorial layar sekitar 30 detik"
destination: youtube + tiktok
aspect: 1920x1080 + 1080x1920
language: id
audience: "Pemula Blender berbahasa Indonesia yang ingin membuat aset game"
length: 30-36s per part
angle: screen-tutorial
---

## Intent

What the user learned on 2026-10-16/17 (summary: `docs/chat_summary/blender_game_asset_basics.md`), shared
as screen tutorials. The user asked (2026-10-17): "create video tutorial explanation ... schedule it on
youtube ... schedule it on TikTok ... if in video to dense with explaining in 30s, make it max 3 part."
The user chose the split "Physics, stone, mirror" and approved the titles and captions before sleeping.

Same pipeline and look as `videos/blender-material-tutorial` (see its BRIEF.md): Claude drives a
separate factory Blender (fullscreen, UI scale 1.25) with real input while ffmpeg records the screen,
then each part is edited with captions, key chips, cursor ring, click ripples and zooms.

## Layout

```
recording/          shared by the three parts
  driver.py         launch <part> | exec | shot | move/click/key | take <part> <name> | quit
  blender_session.py  start scene of each part (fisika, batu, mirror), command channel
  steps_<part>.py   the calibrated steps and captions of each part
  make_data.py      python make_data.py <part> <take>: footage, tutorial-data.js, caption/chip clips
  out/              <part>-<take>.mp4 + .events.json
01-fisika/          HyperFrames project: index.html (16:9), compositions/vertical.html (9:16)
02-batu/
03-mirror/
render_all.sh       renders both formats of every part, one after another
```

Each part holds `youtube.md` and `tiktok.md` (the posts) and `renders/`.

## Parts

1. **Fisika** (`01-fisika`, take `fisika-take1`, 34.5 s): a cube in the air over the ground; Physics tab,
   Rigid Body (Active) on the cube, Rigid Body with Type Passive on the ground, Space plays the fall,
   then Shift+Left and Space replay it.
2. **Batu** (`02-batu`, `batu-take1`, 35.5 s): Shift+A, Mesh, Ico Sphere; Tab; Mesh, Transform,
   Randomize, Amount 0.25; S, Z, 0.6; Tab. The viewport statistics overlay is on, so "80 segitiga" shows.
   The 16:9 caption and chips sit bottom right, because the Randomize panel and Mesh menu are on the left.
3. **Mirror** (`03-mirror`, `mirror-take1`, 31.9 s): one pillar (12 sides, origin at the world centre);
   Modifier tab, Add Modifier, Generate, Mirror; nothing changes (mirror at the origin); Tab, A, G, X,
   1.5, Enter; Tab: two pillars 3 m apart. Front view (Euler 75, 0, 18 degrees).

## Notes

- No music, no voice, no title card, no emoji.
- Points in the scene (cube, ground, pillar, origin) are projected from Blender at run time; UI positions
  were calibrated with screenshots (`recording/.state/`). They hold for 1920x1080, factory settings, UI
  scale 1.25, Blender 5.2.2.
- The user stepped away during the takes; their own Blender stayed open (2.1 GB RAM free was enough).

## Posted

- YouTube (Heri Santoso Dev), scheduled 2026-10-17: part 1 https://youtu.be/Ki6CScK9qMw (Oct 18 19:00),
  part 2 https://youtu.be/9kxugPpL16o (Oct 19 19:00), part 3 https://youtu.be/6L-GiCTgGcE (Oct 20 19:00).
- TikTok (@archetipe_), scheduled 2026-10-17 for Everyone: part 1 Oct 19, part 2 Oct 20, part 3 Oct 21,
  each 19:00. The first run was stopped by Claude Code (low memory) while it typed part 1's caption and left
  nothing in Studio; after the user's own Blender was closed (empty default scene) the three posts went
  through one at a time, watched live. Studio's Posts list then showed each once, next to Oct 17 and 18.
