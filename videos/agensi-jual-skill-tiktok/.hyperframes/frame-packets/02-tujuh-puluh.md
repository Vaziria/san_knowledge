# Frame packet: 02-tujuh-puluh

## Project inputs

- Project: D:\research\marketing\videos\agensi-jual-skill-tiktok
- Design tokens: D:\research\marketing\videos\agensi-jual-skill-tiktok\frame.md
- RULES_DIR: C:\Users\heris\.claude\skills\hyperframes-animation\rules

## Assigned storyboard block

## Frame 2 — Kamu simpan 70%

- scene: Agensi's real "Keep 70%" line in a window, then a huge orange 70%
- voiceover: "Di Agensi.io, — kamu simpan 70% — dari setiap penjualan skill."
- duration: 9s
- transition_in: zoom-through
- status: outline
- src: compositions/frames/02-tujuh-puluh.html
- type: feature_showcase
- persuasion: Value stacking (the money)
- beat: excitement
- blueprint: video-text-pivot (Adapt)
- asset_candidates: assets/agensi-home-build-business.png — Agensi's "Build a business on Agensi": "Keep 70%, paid to your bank - payouts via Stripe, or USDC on Solana", creator card "You keep 70% of $9"; assets/agensi-home-hero.png — Agensi home page top, "The Skill Marketplace for AI Agents"
- focal: assets/agensi-home-build-business.png
- roles: agensi-home-hero = supporting (the window's first screen) · agensi-home-build-business = cutout (the window's second screen)
- host_sfx: ting.wav at 4.2s

narrativeRole: name the marketplace and the money in one beat.
keyMessage: on Agensi, creators keep 70% of every sale.

Adapt: keep the slide-aside signature (the page holds, then gives its weight to the stat).
Scene 1 (0.0–3.0s): kicker "✱ AGENSI.IO" and "Di Agensi.io," (display) at the top (y 180-420); a window (x 60-1020, ~16:9) below it shows agensi-home-hero.png, then at 1.6s its screen advances (scale-swap) to agensi-home-build-business.png zoomed to the "Keep 70%, paid to your bank" line, with a blue marker box drawn around it at 2.4s.
Scene 2 (3.0–9.0s): the window shrinks up out of the way (scale ~0.7, top); a number-lockup "70%" lands at 4.2s in `number-hero` sized huge (figure in orange #eb6834) with "kamu simpan" above it at 3.6s and "dari setiap penjualan skill." under it at 5.2s; hold still.

## Selected blueprint: video-text-pivot

# video-text-pivot — Video → Text Pivot

**intent**: A product video holds center and claims attention, then slides aside to hand its weight to a hero stat in the space it vacates, then both clear and kinetic text types into the center — accent words carrying the meaning the video used to carry — sealed by a gradient pill. The arc is "show → yield → pivot → stamp," and each handoff pairs an exit with a same-anchor entrance so two beats read, not four.

**roles served**

- Product_Intro (from `metric-video-text-pivot`): when the open is "see the feature" then "see the impact" and the `[product video]` must stay visible through the stat reveal — it slides, it doesn't cut.
- Key_Feature: a feature clip that yields to a frame-filling metric and a typographic impact line.

**duration**: 6–8s

**shot structure** (a `[bg]` canvas; one `[product video]` as a real muted `.mp4` clip, a hero stat, then kinetic text — each pair shares a screen anchor so the handoff reads as a weight-transfer)

- **Scene 1 (0.0–~1.6s) — the video shows.** The `[product video]` lands centered on a smooth scale-up and breathes (a small y-bob), claiming full attention. Camera static.
- **Scene 2 (~1.6–3.2s) — yield + stat (signature move).** The video SLIDES aside (x + scale down) **into the very space** the `[hero stat]` now fills as the stat pops in with 3D-depth type — one weight-transfer reading as a single event, not two. The stat breathes within this window.
- **Scene 3 (~3.2–5.0s) — pivot to text.** Both video and stat clear out and kinetic `[impact text]` TYPES into the vacated center, character by character; its `[accent words]` carry the meaning the video used to carry.
- **Scene 4 (~5.0–end) — stamp.** A gradient `[pill]` snaps shut around the closing line (`scaleX` 0→1), its glow halo resolving a beat behind so the silhouette reads before the bloom — sealing the statement as one graphic. Holds.

**motion vocabulary**: video scale-in + small breath; weight-transfer slide (video x + scale-down handing off to the stat at the same anchor); 3D-depth stat type; character-stream typing; gradient pill scaleX-snap; glow-halo bloom trailing the silhouette.

**rule mapping**

- video entrance (smooth) and the weight-transfer slide → `gsap-effects` (scale/opacity then x + scale on a long-tail `power3`); the video itself is a muted `<video class="clip">` direct child of the root
- hero stat's frame-filling 3D type → `3d-text-depth-layers` (static-depth variation — layers built at setup, no cascade fighting the entry)
- the same-anchor video-exit ↔ stat-entry handoff (if treated as a morph) → `scale-swap-transition` (shared center)
- character-by-character impact typing through segmented spans → `dynamic-content-sequencing` (clean character stream) or `discrete-text-sequence`
- pill `scaleX` snap + trailing glow halo → `gsap-effects` (scaleX) + `ambient-glow-bloom` (the halo, resolving a beat behind)
- video / stat breath within their windows → `sine-wave-loop` (low-amplitude register — subtle jitter, gated to each element's window, never a forever loop)

**camera modifier**: camera-static — all motion is element-space (the video translates), so the "pivot" is the elements moving, not a camera.
