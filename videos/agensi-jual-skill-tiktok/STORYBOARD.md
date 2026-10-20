---
format: 1080x1920
duration: 30s
message: "Keahlianmu bisa jadi produk: jual skill AI agent di Agensi.io"
arc: hook -> how creators earn (70%) -> what sells and the price -> call to action
audience: developer dan pembuat konten Indonesia di TikTok yang memakai AI agent
mode: autonomous
music: none
captions: none
---

The vertical cut of ../agensi-jual-skill. No narration and no music: each frame's `voiceover` line is its ON-SCREEN TEXT, in Indonesian, revealed cue by cue at the em-dashes. Audio is only the user's own sound effects (`host_sfx`, placed by the orchestrator at the given frame times; land the matching reveal exactly then). Every Agensi page shown is a real screenshot; never rebuild Agensi's UI.

## Video direction

- **Canvas:** 1080x1920 vertical. TikTok draws its own buttons along the right edge and its caption at the bottom: keep all content inside x 60-900 and y 160-1500; nothing important lower than y 1500.
- **Palette (frame.md roles):** ground navy #1f2a44 on every frame (full-duration background clip); voice `ink` #f3f2ee; one blue #2a78d6 voltage per frame (use the lighter #3987e5 for small blue text on navy, for contrast); orange #eb6834 only for the "70%" payoff; cards `tile` #26324f with the hairline; Agensi pages as floating windows (radius-lg, hairline, one soft shadow), scaled to the column width, zoom-to-target allowed.
- **Type:** big and few words: headlines in `display` sized for a phone (about 9-11% of the frame width per line height), one phrase in `display-italic`; kicker labels as `kicker-spike` (blue ✱ as an inline SVG); numbers as `number-lockup`. Indonesian copy exactly as written; no emoji or icons.
- **Motion grammar:** fast but smooth: `power3`/`expo` settles, no bounce; each cue appears at its time; camera moves only early in a frame, then hold; no slow drift or push in a frame's back half; fromTo entrances; no repeat/yoyo/random/CSS animation. Only Frame 4 has a real exit.
- **Negative list:** no rebuilt UI, no invented numbers, no bokeh/purple gradients, no glow, no bounce, no breathing, no slideshow front-loading.

## Frame 1 — Bisa kamu jual

- scene: Big stacked Indonesian type: agents are only as smart as their instructions, and you can sell those
- voiceover: "AI agent hanya sepintar — instruksi yang ia punya. — Instruksi itu bisa kamu jual."
- duration: 6s
- transition_in: cut
- status: animated
- src: compositions/frames/01-hook.html
- type: hook
- persuasion: Future pacing
- beat: curiosity
- blueprint: kinetic-type-beats (Adapt)
- asset_candidates:
- focal: typography only
- roles: none
- host_sfx: ting.wav at 3.9s

narrativeRole: stop the scroll with the opportunity.
keyMessage: what you know can be sold to AI agents.

Adapt: keep the statement-builds signature with hard cuts; two statements, the second the payoff.
Scene 1 (0.0–3.7s): centered vertically around y 700, left-aligned at x 80: kicker "✱ AI AGENT"; "AI agent hanya sepintar" (display) slams in (`kinetic-beat-slam`) at 0.15s, then "instruksi yang ia punya." (display-italic) at 1.4s, stacked.
Scene 2 (3.7–6.0s): hard cut to "Instruksi itu" (display, ink) and at 3.9s "bisa kamu jual." punches in beneath in display blue (the voltage); hold.

## Frame 2 — Kamu simpan 70%

- scene: Agensi's real "Keep 70%" line in a window, then a huge orange 70%
- voiceover: "Di Agensi.io, — kamu simpan 70% — dari setiap penjualan skill."
- duration: 9s
- transition_in: zoom-through
- status: animated
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

## Frame 3 — Yang laku dan harganya

- scene: Agensi's guide on what sells, three chips, then two price cards
- voiceover: "Yang paling laku: — code review, testing, otomasi git. — Harga per skill: — $5–15 atau $20–50."
- duration: 9s
- transition_in: push-slide UP
- status: animated
- src: compositions/frames/03-laku-harga.html
- type: feature_showcase
- persuasion: Friction reduction (what to build, what to charge)
- beat: aspiration
- blueprint: grid-card-assemble (Adapt)
- asset_candidates: assets/agensi-guide-what-sells.png — guide "What SKILL.md Skills Sell Best?": code review, testing and git automation sell the most
- focal: typography (chips and price cards)
- roles: agensi-guide-what-sells = supporting (a window, top, zoomed onto its quick answer)
- host_sfx: pop.wav at 2.0s, pop.wav at 5.6s

narrativeRole: tell the viewer what to build and what to charge.
keyMessage: code review, testing and git automation sell; $5-15 for one job, $20-50 for a full skill.

Adapt: keep the self-assembling signature; chips and cards pop into place and stay.
Scene 1 (0.0–1.8s): kicker "✱ YANG LAKU" and "Yang paling laku:" (display) at the top; the window (x 60-1020) shows the guide zoomed onto its quick answer, a blue underline under "Code review, testing, and git automation skills sell the most" at 1.0s.
Scene 2 (1.8–4.8s): three chips pop in a column-friendly wrap at 2.0s: "code review", "testing", "otomasi git" (tile cards, mono).
Scene 3 (4.8–9.0s): "Harga per skill:" (headline) at 5.0s; two price cards pop side by side at 5.6s: "$5–15" / "satu tugas" and "$20–50" / "skill lengkap" (the first figure in blue, the voltage); a mono-label "panduan Agensi" under them; hold still.

## Frame 4 — Mulai di agensi.io/sell

- scene: Agensi's sell page and the address, our crane signing off
- voiceover: "Mulai jual skill-mu: — agensi.io/sell"
- duration: 6s
- transition_in: crossfade
- status: animated
- src: compositions/frames/04-mulai.html
- type: cta
- persuasion: Friction reduction (one address)
- beat: motivation
- blueprint: logo-assemble-lockup (Adapt)
- asset_candidates: assets/agensi-sell.png — Agensi's creator page "How to make money with AI: sell agent skills", button "Start Selling"; assets/project-brain-logo.svg — our origami crane logo
- focal: typography (the address pill)
- roles: agensi-sell = supporting (a window, upper middle) · project-brain-logo = supporting (small sign-off mark)
- host_sfx: ting.wav at 2.4s

narrativeRole: one action.
keyMessage: start at agensi.io/sell.

Adapt: keep the lockup-extended-to-CTA signature; the address is the lockup.
Scene 1 (0.0–2.2s): the window with agensi-sell.png (its headline "How to make money with AI: sell agent skills" readable) rises in at the top (y 180-760); "Mulai jual skill-mu:" (display) beneath it at 0.6s.
Scene 2 (2.2–6.0s): a big blue pill (`coral-callout`, the voltage) "agensi.io/sell" wipes in at 2.4s, centered around y 1180; the small crane logo (~90 px) settles under it at 3.0s; hold; the whole frame fades to the navy ground in the last 0.5s (the video's only exit).
