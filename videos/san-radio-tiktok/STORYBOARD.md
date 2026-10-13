---
format: 1080x1920
duration: 24s
message: "Radio musik santai 24 jam — tulis komentar, namamu diketik di layar"
arc: Hook (your name on screen) → what it is (a Pi, 24 jam) → the music → CTA
audience: Indonesian TikTok viewers who play calm music while they work, study or relax at night
mode: autonomous
music: none
language: id
---

## Video direction

The whole video is the 24/7 stream's own terminal (golang/packages/san_radio/background/terminal.html),
re-laid for a phone. One window, one place, four frames joined by **cuts**: the chrome (ground,
window, tab bar, bottom prompt bar) is pixel-identical in every frame and never animates, so a cut
reads as the terminal clearing and typing the next thing. Only the text area changes.

**Shared chrome — identical in all four frames, present from t=0, no entrance, no motion.** Canvas
1080×1920. All text is DejaVu Sans Mono from `assets/DejaVuSansMono.ttf` (load with @font-face).
- Ground: full-bleed clip layer, `#0d0b1e`, with `assets/night-scene.svg` as a full-bleed `<img>`
  (object-fit: cover; object-position: right bottom) so the snowy mountain and torii sit lower right.
  Over it, a full-bleed `rgba(13,11,30,0.35)` veil. Static.
- Window: absolute, left 56px, top 150px, width 968px, height 1380px; background
  `rgba(16,14,36,0.80)`; border 1.5px solid `rgba(160,120,255,0.55)`; border-radius 22px;
  box-shadow `0 0 24px rgba(140,90,255,0.25), 0 20px 60px rgba(0,0,0,0.6)`.
- Tab bar (inside the window's top 84px): three dots 20px diameter at window-relative x 36, 68,
  100 (centres 46, 78, 110), y-centre 42, colours #ff5f57, #febc2e, #28c840; a tab pill at x 136,
  y 18, 190×48, radius 10, fill `rgba(70,70,120,0.45)`, holding "~" (white #e6e6f0, 30px) at its
  left + 22px and "×" (#8b8fb5, 24px) at its right − 34px; the clock "21:04" (#e6e6f0, 30px)
  right-aligned at window-relative x 928, y-centre 42. A 1px rule `rgba(160,120,255,0.28)` across
  the window at window-relative y 84.
- Bottom prompt bar: top at absolute y 1446, height 56, starting at absolute x 104: segment 1 is a
  right-pointing chevron outline (2px #ff7ac6 stroke, no fill), x 104–268, holding "radio" in
  #ff7ac6 34px; segment 2 a filled #5ccfe6 chevron x 262–392 holding "~" in #0d0b1e 34px; then "›"
  in #ff7ac6 38px at x 412; then a static cursor block 20×40 #8b8fb5 at x 446. Static, no blink.
- Text area: left edge absolute x 104, first line top absolute y 270; font-size 40px, line-height
  52px, `white-space: pre`. At most 33 characters per line (keeps everything left of the 84% line,
  x 907, clear of TikTok's right rail). Lines below are numbered L0, L1 … from that first line.
- Terminal colours (content): prompt host `radio@getresolved` #ff7ac6, path #5ccfe6, arrow "❯"
  #9ece6a, typed commands #e6e6f0, program output #e0af68, comments (`# …`) and tree #8b8fb5. The
  one emphasis accent is pink #ff7ac6.

**Typing.** Commands type character by character behind a block caret (#e6e6f0, 24×44, sitting
after the last typed character on the line being typed; it hides when the line is finished).
Typing speed about 22 characters per second, steady, no randomness. A typed line's caret moves to
the next line only on a new command. → `discrete-text-sequence` + `context-sensitive-cursor`.
Output lines appear whole (no typing) on their cue. Comment lines type like commands.

**Safe zones.** Everything load-bearing between y 150 and y 1530 (TikTok's caption band is the
bottom 17%); text never right of x 907.

**Motion grammar.** No camera moves except the one payoff push-in in frame 4. No drifting, no
breathing loops, no bounce or elastic eases, no Math.random / Date.now, no CSS transitions or
keyframes. Holds are still.

**Negative list.** No new colours, no logos (no YouTube logo), no emoji except the stream's own
glyphs (❯ ♪ ◀ › ├── └──), no invented numbers (no viewer counts, no uptime), no real viewer names
(`@kamu` stands for the viewer), never a stream key.

## Frame 1 — Your name on the screen

- scene: the radio's terminal greets @kamu — a comment becomes `Hello, @kamu!` on screen
- voiceover: ""
- duration: 5s
- transition_in: cut
- status: animated
- src: compositions/frames/01-hook.html
- type: hook
- persuasion: Show the reward first (the viewer imagines their own name typed)
- beat: curiosity → delight
- blueprint: prompt-type-submit-generate (Adapt)
- focal: the line `Hello, @kamu!`
- roles: none
- asset_candidates: assets/night-scene.svg, assets/DejaVuSansMono.ttf
- sfx: host — keys @0.3, keys @2.4

narrativeRole: open on the payoff — a live chat comment is typed on the radio's screen and the
radio answers by name — so the viewer wants that for themselves in the first second.
keyMessage: tulis komentar, namamu diketik di layar.

Adapt: keep the typed-command-is-the-show signature; the "response" is the output line.
Scene 1 (0.0s): chrome in place. L0 shows `# radio musik santai 24 jam` (#8b8fb5), L2 shows the
  prompt `radio@getresolved ~` (host pink, "~" blue). L3 holds "❯ " (green) with the caret after it.
Scene 2 (0.3–1.5s): L3 types `greet @kamu "enak banget!"` (#e6e6f0) behind the caret.
  → `discrete-text-sequence` + `context-sensitive-cursor`
Scene 3 (1.8s): caret hides; L4 `Hello, @kamu!` appears whole in #e0af68 with a soft yellow glow
  (text-shadow 0 0 18px rgba(224,175,104,0.55)) that rises from 0 and settles in ~0.5s; the line
  enters with a 0.96→1 scale settle (power3, transform-origin left centre). → `ambient-glow-bloom`
Scene 4 (2.4–3.8s): L6 types `# tulis komentar di live chat,` then L7 `# namamu diketik di sini`
  (#8b8fb5), caret moving to L7 after L6.
Scene 5 (3.8–5.0s): hold still; caret hidden.

## Frame 2 — A Raspberry Pi, all day and night

- scene: `neofetch` shows the pixel-art card and the Pi the radio runs on
- voiceover: ""
- duration: 6s
- transition_in: cut
- status: animated
- src: compositions/frames/02-neofetch.html
- type: product_intro
- persuasion: Concrete, believable detail (a small Pi, real specs)
- beat: delight → trust
- blueprint: agent-progress-theater (Adapt)
- focal: the pixel-art card
- roles: hero image = pixel-art
- asset_candidates: assets/pixel-art.png, assets/night-scene.svg, assets/DejaVuSansMono.ttf
- sfx: host — keys @0.2, keys @3.8

narrativeRole: say what this is — a music radio running on a tiny Raspberry Pi, 24 hours a day.
keyMessage: radio ini jalan 24 jam di sebuah Raspberry Pi.

Adapt: the "work" is neofetch printing; the payoff is the info block filling in.
Scene 1 (0.0s): chrome in place; L0 prompt `radio@getresolved ~`; L1 "❯ " with caret.
Scene 2 (0.2–0.7s): L1 types `neofetch` (#9ece6a, as on the stream).
Scene 3 (0.9–1.5s): `assets/pixel-art.png` appears at absolute x 104, y 382, width 520, height
  431, image-rendering: pixelated, revealed top-to-bottom with a clip-path wipe (like a terminal
  drawing the image row by row). → `waterfall-entry`
Scene 4 (1.6–3.2s): the info block appears line by line below the art, starting at absolute y 850
  (line-height 52px), each line whole, ~0.2s apart: `radio@getresolved` (#ff7ac6),
  `-----------------` (#8b8fb5), then label/value pairs with labels in #5ccfe6 padded to 8
  characters and values in #e6e6f0: `OS      Raspberry Pi OS`, `Host    Raspberry Pi 3B+`,
  `Shell   san_radio`, `Tracks  8`, `Now     auntum.wav`. → `waterfall-entry`
Scene 5 (3.8–5.1s): the next line after the block types `# jalan 24 jam di Pi kecil ini`
  (#8b8fb5).
Scene 6 (5.1–6.0s): hold still.

## Frame 3 — The music

- scene: `eza --tree` lists the radio's own songs; `play auntum.wav` starts the next one
- voiceover: ""
- duration: 6s
- transition_in: cut
- status: animated
- src: compositions/frames/03-music.html
- type: key_feature
- persuasion: Show the real catalogue (real file names, not stock)
- beat: trust → want
- blueprint: prompt-type-submit-generate (Adapt)
- focal: the playing line `♪ auntum.wav  ◀`
- roles: none
- asset_candidates: assets/night-scene.svg, assets/DejaVuSansMono.ttf
- sfx: host — keys @0.2, keys @3.0, keys @4.2

narrativeRole: the radio plays the owner's own music, and the terminal shows it changing tracks.
keyMessage: lagu ciptaan sendiri, diputar bergantian 24 jam.

Scene 1 (0.0s): chrome in place; L0 prompt `radio@getresolved ~/music` (host pink, path blue);
  L1 "❯ " with caret.
Scene 2 (0.2–0.7s): L1 types `eza --tree` ("eza" #e0af68, "--tree" #e6e6f0, as on the stream).
Scene 3 (0.9–2.1s): the tree appears line by line in #8b8fb5, ~0.13s apart, from L2:
  `.`
  `├── ♪ Adventure in Forest Dungeo…`
  `├── ♪ Broken Legendary Shrine.wav`
  `├── ♪ Lake of silence and death.…`
  `├── ♪ Temple of Monk Warrior.wav`
  `├── ♪ arabic.wav`
  `├── ♪ auntum.wav`
  `├── ♪ mistjapanese.wav`
  `└── ♪ ruin.wav`
  (the two long names are cut with "…" at 33 characters, exactly as the stream fits names.)
  → `waterfall-entry`
Scene 4 (2.6s): the `♪ auntum.wav` line turns #ff7ac6 and gains `  ◀` after it (the stream's
  playing marker), with a 0.2s colour crossfade. → `discrete-text-sequence`
Scene 5 (2.9–3.7s): L12 shows the prompt `radio@getresolved ~`; L13 "❯ " then types
  `play auntum.wav`.
Scene 6 (4.2–5.3s): L15 types `# lagu ciptaan sendiri, 24 jam` (#8b8fb5).
Scene 7 (5.3–6.0s): hold still.

## Frame 4 — Come say hi

- scene: the terminal opens the channel; the big CTA "@GetResolved — cari di YouTube" lands
- voiceover: ""
- duration: 7s
- transition_in: cut
- status: animated
- src: compositions/frames/04-cta.html
- type: cta
- persuasion: Clear next step with a reward (your name on screen)
- beat: want → action
- blueprint: prompt-type-submit-generate (Adapt)
- focal: the CTA block "@GetResolved"
- roles: none
- asset_candidates: assets/night-scene.svg, assets/DejaVuSansMono.ttf
- sfx: host — keys @0.2

narrativeRole: tell the viewer exactly where to go and what they get there.
keyMessage: cari @GetResolved di YouTube — tulis komentar, namamu muncul di layar.

Adapt: the install-command end card — the command types, the output answers, the CTA lands and
holds.
Scene 1 (0.0s): chrome in place; L0 prompt `radio@getresolved ~`; L1 "❯ " with caret.
Scene 2 (0.2–1.6s): L1 types `open youtube.com/@GetResolved` (#e6e6f0).
Scene 3 (1.9s): output lines appear whole in #e0af68: L2 `live 24 jam · musik santai`,
  L3 `komentar → namamu di layar`.
Scene 4 (2.6–3.4s): the CTA block lands, centred horizontally between x 104 and x 904, top at
  absolute y 900: "@GetResolved" in #ff7ac6 at 88px (bold not needed; DejaVu Sans Mono regular),
  with a soft pink glow (text-shadow 0 0 28px rgba(255,122,198,0.45)); 26px below it
  "cari di YouTube" in #e6e6f0 at 52px. The block enters with the video's one camera move, a
  push-in: scale 0.88→1 and opacity 0→1 on a long power3 settle (~0.8s), transform-origin at the
  block's centre. → `spring-pop-entrance` (no overshoot)
Scene 5 (3.4–7.0s): hold completely still to the end.
