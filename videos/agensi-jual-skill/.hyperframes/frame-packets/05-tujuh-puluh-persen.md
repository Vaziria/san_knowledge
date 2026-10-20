# Frame packet: 05-tujuh-puluh-persen

## Project inputs

- Project: D:\research\marketing\videos\agensi-jual-skill
- Design tokens: D:\research\marketing\videos\agensi-jual-skill\frame.md
- RULES_DIR: C:\Users\heris\.claude\skills\hyperframes-animation\rules

## Assigned storyboard block

## Frame 5 — Kamu simpan 70%

- scene: Agensi's "Build a business" section slides aside and hands the frame to a big 70%
- voiceover: "Kamu simpan 70% — dari setiap penjualan. — Jual sekali, atau langganan. — Dibayar lewat Stripe, atau USDC."
- duration: 13s
- transition_in: push-slide LEFT
- status: outline
- src: compositions/frames/05-tujuh-puluh-persen.html
- type: feature_showcase
- persuasion: Value stacking (the money)
- beat: excitement
- blueprint: video-text-pivot (Adapt)
- asset_candidates: assets/agensi-home-build-business.png — Agensi's "Build a business on Agensi": one-time or subscription, updates reach every buyer, "Keep 70%, paid to your bank - payouts via Stripe, or USDC on Solana", creator card "seo-audit-pro $9.00 · You keep 70% of $9"
- focal: assets/agensi-home-build-business.png
- roles: agensi-home-build-business = cutout (a window that starts centered, then slides to the right ~45%)
- host_sfx: ting.wav at 3.0s

narrativeRole: the money: how creators earn.
keyMessage: creators keep 70% of every sale, one-time or subscription.

Adapt: keep the slide-aside signature (the page holds, then slides aside to hand its weight to the stat, never cut away); the stat is 70%.
Scene 1 (0.0–2.6s): the window with agensi-home-build-business.png arrives centered and large; zoom-to-target on its "Keep 70%, paid to your bank" line, a blue marker box draws around it at 1.8s.
Scene 2 (2.6–6.0s): the window slides right to ~45% width; in the freed left side a number-lockup "70%" lands at 3.0s in `number-hero`, the figure in orange #eb6834 (this frame's payoff), then "dari setiap penjualan." (headline) at 4.0s; kicker "✱ UNTUK KREATOR" above.
Scene 3 (6.0–13.0s): two short lines reveal under the stat: "Jual sekali, atau langganan." at 6.6s and "Dibayar lewat Stripe, atau USDC." at 9.0s (body/lead, ink); hold still.

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
