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

