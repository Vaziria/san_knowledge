---
format: 1920x1080
duration: 116s
message: "Keahlianmu bisa jadi produk: jual skill AI agent di Agensi.io"
arc: Future pacing into a demo of the platform - hook -> name the marketplace -> what a skill is -> works everywhere -> how creators earn -> price -> what sells -> trust -> our example -> how to start
audience: developer dan pembuat konten Indonesia yang memakai AI agent dan ingin penghasilan dari keahliannya
mode: autonomous
music: none
captions: none
---

No narration and no music. Each frame's `voiceover` line is its ON-SCREEN TEXT, in Indonesian, revealed cue by cue at the em-dashes. Audio is only the user's own sound effects (`host_sfx`, placed by the orchestrator at the given frame times; keep the matching reveal exactly on that time). Every Agensi page shown is a real screenshot (assets/agensi-*.png); never rebuild Agensi's UI. Every figure comes from Agensi's own pages; "menurut Agensi" marks their own claims.

## Video direction

- **Palette (frame.md roles):** ground = navy #1f2a44 (`cream` key) on every frame as a full-duration background clip; voice = `ink` #f3f2ee; the ONE voltage per frame = `coral` key, blue #2a78d6 (key word, number or marker); `tile-strong` orange #eb6834 only for the crane and the "70%" payoff; cards = `tile` #26324f with the hairline; code/terminal = the `code-surface` component (deep navy #121a2c, cream JetBrains Mono).
- **Agensi screenshots:** light pages, shown as floating windows on the navy ground: radius-lg, hairline border, one soft shadow, no browser chrome; scaled to fit, zoom-to-target up to 2x stays sharp. Allowed on top of them: crop/zoom, a blue rounded marker box or underline drawn over real pixels. Never repaint or retype Agensi's UI, and never invent a skill name, price or count that is not on the page.
- **Type:** headlines `headline`/`display` (EB Garamond 400, sentence case); the one emphasized phrase per frame in `display-italic`; frame labels as `kicker-spike` (JetBrains Mono uppercase, blue ✱ drawn as an inline SVG, 2-4 words); numbers as `number-lockup` (EB Garamond figure + JetBrains Mono unit); small source notes ("menurut Agensi", "panduan Agensi") in `mono-label` at ~60% ink. Indonesian copy exactly as written in each frame's `voiceover`; no emoji or icons.
- **Motion grammar:** `power3` long-tail settles, no bounce/overshoot; each text cue appears only at its time (reading pace, about 0.3 s per word); camera moves (zoom-to-target) only in the first part of a scene to point at the proof, then hold; no back-half slow push or drift; holds are still (subtle jitter at most); fromTo entrances; no repeat/yoyo/random/CSS animation. Only Frame 10 has a real exit.
- **Rhythm:** energetic: 1 (hard cuts), 2 (count-up), 5 (the 70%), 8 (checklist). Breathers that hold still: 3 and 9. Frame 10 ends on a held CTA.
- **Keep-out:** plan all important content into the top ~83% (y <= 900).
- **Negative list:** no rebuilt UI, no invented numbers or skills, no stock imagery, no bokeh/purple "AI" gradients, no glow halos, no bounce, no lazy breathing, no screensaver floating, no slideshow (all content in the first quarter, then frozen), no back-half pans.

## Frame 1 — Instruksi itu bisa dijual

- scene: Bold Indonesian type on navy: agents are everywhere, they are only as smart as their instructions, and those instructions can be sold
- voiceover: "AI agent sekarang ada di mana-mana. — Tapi agent hanya sepintar instruksi yang ia punya. — Instruksi itu bisa kamu jual."
- duration: 9s
- transition_in: cut
- status: animated
- src: compositions/frames/01-instruksi.html
- type: hook
- persuasion: Future pacing
- beat: curiosity
- blueprint: kinetic-type-beats (Adapt)
- asset_candidates:
- focal: typography only
- roles: none
- host_sfx: ting.wav at 6.4s

narrativeRole: open on the shift (agents everywhere) and turn it into the viewer's opportunity before naming anything.
keyMessage: what you know can be sold to AI agents.

Adapt: keep the statement-builds-across-beats signature with hard-cut swaps; three statements, the last one the payoff.
Scene 1 (0.0–2.6s): centered, ~55% width: kicker "✱ AI AGENT" fades in; display line "AI agent sekarang ada di mana-mana." lands via kinetic beat-slam (`kinetic-beat-slam`), smooth settle.
Scene 2 (2.6–6.2s): hard-cut word swap (`discrete-text-sequence`) to "Tapi agent hanya sepintar" (display) over "instruksi yang ia punya." (display-italic); the word "instruksi" lands last.
Scene 3 (6.2–9.0s): hard cut to "Instruksi itu" (headline, ink) and at 6.4s "bisa kamu jual." punches in beneath in `display` blue (the one voltage); holds still to the cut.

## Frame 2 — Agensi.io

- scene: The real Agensi home page arrives; its counts climb: 12.000+ skill, 1.500+ kreator
- voiceover: "Namanya Agensi.io — marketplace skill untuk AI agent. — 12.000+ skill, 1.500+ kreator."
- duration: 11s
- transition_in: zoom-through
- status: animated
- src: compositions/frames/02-agensi.html
- type: product_intro
- persuasion: Authority by association (scale)
- beat: curiosity + trust
- blueprint: dataviz-countup (Adapt)
- asset_candidates: assets/agensi-home-hero.png — Agensi home page top: "The Skill Marketplace for AI Agents", counts 12,000+ Skills · 12,000+ Users · 1,500+ Creators, recently listed skills with prices
- focal: assets/agensi-home-hero.png
- roles: agensi-home-hero = cutout (one window, right ~60%)
- host_sfx: ting.wav at 7.4s

narrativeRole: name the marketplace and give its scale, from its own page.
keyMessage: Agensi is a real, busy marketplace for agent skills.

Adapt: keep the count-up signature (numbers climb to their value as they are named); the screenshot is the context, the counts are the hero. Asymmetric 40/60.
Scene 1 (0.0–3.0s): right ~60%: the window with agensi-home-hero.png rises in; left column: kicker "✱ MARKETPLACE" and the display line "Namanya Agensi.io" (Agensi.io in blue, the voltage).
Scene 2 (3.0–5.6s): "marketplace skill untuk AI agent." reveals (headline, ink) under it; in the window, a zoom-to-target onto the page's counts row ("12,000+ Skills · 12,000+ Users · 1,500+ Creators") with a blue marker underline drawn under it at 5.0s.
Scene 3 (5.6–11.0s): two number-lockups count up in the left column (`counting-dynamic-scale`): "12.000+" with unit "SKILL", then "1.500+" with unit "KREATOR", each landing by 7.4s; a small mono-label beneath: "angka dari halaman Agensi, Oktober 2026". Hold still.

## Frame 3 — Apa itu skill?

- scene: Agensi's own "What is a skill?" section beside a small SKILL.md that types itself
- voiceover: "Skill itu satu file: SKILL.md. — Isinya langkah kerja seorang ahli. — Agent memuatnya sebelum menjawab."
- duration: 12s
- transition_in: crossfade
- status: animated
- src: compositions/frames/03-apa-itu-skill.html
- type: feature_showcase
- persuasion: Feature-to-benefit translation
- beat: clarity
- blueprint: compose
- asset_candidates: assets/agensi-home-what-is-skill.png — Agensi's "What is a skill?" section: a capability your agent loads before it answers, the exact steps an expert would follow
- focal: assets/agensi-home-what-is-skill.png
- roles: agensi-home-what-is-skill = supporting (a window, left ~52%, cropped to the "What is a skill?" block)
- host_sfx: keys.wav at 3.6s, keys.wav at 4.6s

narrativeRole: make "skill" concrete: a file with an expert's steps.
keyMessage: a skill is one SKILL.md with expert steps that the agent loads.

Compose: split 52/48. Left: the real section as a window; right: a code-surface card and the text cues. A held breather once built.
Scene 1 (0.0–2.8s): left window with agensi-home-what-is-skill.png, zoomed so "What is a skill?" and its first paragraph read large; right column: kicker "✱ SKILL" and headline "Skill itu satu file:" with "SKILL.md" in mono blue (the voltage).
Scene 2 (2.8–6.6s): under the headline, a code-surface card (title bar "SKILL.md", label "contoh" in mono-label) types four lines with a caret (`discrete-text-sequence`), from 3.6s: "---", "name: cold-email-writer", "description: Menulis cold email yang dibalas", "---". Cue "Isinya langkah kerja seorang ahli." reveals at 4.8s under the card.
Scene 3 (6.6–12.0s): cue "Agent memuatnya sebelum menjawab." lands in display-italic at 7.2s; a blue marker underline draws on the window's line "A skill is a capability your agent loads before it answers you." at 7.6s. Hold still.

## Frame 4 — Satu skill, semua agent

- scene: One pinned line while the agent names cycle beside it, over Agensi's agent-marketplace grid
- voiceover: "Satu skill — jalan di Claude Code, Cursor, Codex CLI, GitHub Copilot, Gemini CLI."
- duration: 10s
- transition_in: crossfade
- status: animated
- src: compositions/frames/04-semua-agent.html
- type: benefit_highlight
- persuasion: Value stacking (reach)
- beat: confidence
- blueprint: fixed-anchor-cycle (Adapt)
- asset_candidates: assets/agensi-home-agents.png — Agensi's "Browse by Agent marketplace" grid: Claude Code, Cursor, Codex CLI, Gemini CLI, GitHub Copilot, Windsurf, OpenClaw and more
- focal: typography (the anchored line)
- roles: agensi-home-agents = background (a window behind the text, dimmed ~35%)
- host_sfx: pop.wav at 2.4s, pop.wav at 6.8s

narrativeRole: one skill sells to every agent ecosystem: the market is wide.
keyMessage: SKILL.md is an open standard; one skill reaches many agents.

Adapt: keep the pinned-anchor signature (one line enters once and never moves while its slot cycles), landing on the last name.
Scene 1 (0.0–2.2s): the agents window fades in behind (dimmed ~35%, centered, large); kicker "✱ STANDAR TERBUKA"; the anchor "Satu skill, jalan di" (display) enters center-left and stays.
Scene 2 (2.2–7.0s): the slot to its right cycles by hard cut (`discrete-text-sequence`) at a steady step: "Claude Code" (2.4s), "Cursor" (3.4s), "Codex CLI" (4.4s), "GitHub Copilot" (5.4s), "Gemini CLI" (6.4s), in display-italic blue (the voltage).
Scene 3 (7.0–10.0s): the slot settles on "Gemini CLI" and a mono-label line appears beneath: "dan agent lain yang memakai standar SKILL.md". Hold still.

## Frame 5 — Kamu simpan 70%

- scene: Agensi's "Build a business" section slides aside and hands the frame to a big 70%
- voiceover: "Kamu simpan 70% — dari setiap penjualan. — Jual sekali, atau langganan. — Dibayar lewat Stripe, atau USDC."
- duration: 13s
- transition_in: push-slide LEFT
- status: animated
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

## Frame 6 — Berapa harganya?

- scene: Three price cards assemble over the real "What users are buying" prices
- voiceover: "Berapa harganya? — Gratis untuk alat sederhana. — $5–15 untuk satu tugas. — $20–50 untuk skill lengkap."
- duration: 11s
- transition_in: crossfade
- status: animated
- src: compositions/frames/06-harga.html
- type: feature_showcase
- persuasion: Friction reduction (a concrete price to start from)
- beat: clarity
- blueprint: grid-card-assemble (Adapt)
- asset_candidates: assets/agensi-home-buying.png — "What users are buying": real skills by category with prices from $4 to $49
- focal: typography (the three price cards)
- roles: agensi-home-buying = background (full window behind, dimmed ~30%, real prices still legible)
- host_sfx: pop.wav at 3.2s, pop.wav at 5.4s, pop.wav at 7.6s

narrativeRole: give a starting price, from Agensi's own guide, with real listings behind as proof.
keyMessage: free for simple tools, $5-15 for one job, $20-50 for a full skill.

Adapt: keep the self-assembling grid signature; three cards in a row, each popping in on its cue.
Scene 1 (0.0–2.6s): the buying window fades in behind (dimmed ~30%); kicker "✱ HARGA"; headline "Berapa harganya?" (display) centered upper.
Scene 2 (2.6–8.4s): three tile cards assemble left to right (spring-pop entrance, smooth): card 1 at 3.2s "Gratis" (number-lockup figure) / "alat sederhana"; card 2 at 5.4s "$5–15" / "satu tugas"; card 3 at 7.6s "$20–50" / "skill lengkap"; the middle card's figure in blue (the voltage).
Scene 3 (8.4–11.0s): a mono-label under the row: "panduan harga dari Agensi"; hold still.

## Frame 7 — Yang paling laku

- scene: Agensi's own guide on what sells, its answer highlighted line by line
- voiceover: "Yang paling laku: — code review, testing, otomasi git. — Peluang yang masih terbuka: — DevOps, skill Python dan JavaScript."
- duration: 12s
- transition_in: push-slide LEFT
- status: animated
- src: compositions/frames/07-paling-laku.html
- type: feature_showcase
- persuasion: Show-don't-tell proof
- beat: curiosity + aspiration
- blueprint: camera-journey (Adapt)
- asset_candidates: assets/agensi-guide-what-sells.png — guide "What SKILL.md Skills Sell Best?": code review, testing and git automation sell the most, and the untapped ones are OpenClaw-specific skills, DevOps workflows, Python and JavaScript skills
- focal: assets/agensi-guide-what-sells.png
- roles: agensi-guide-what-sells = cutout (one large window, ~62% width, left)
- host_sfx: pop.wav at 3.0s, pop.wav at 8.2s

narrativeRole: tell the viewer what to build first, from Agensi's own data.
keyMessage: code review, testing and git automation sell; DevOps and language skills are open.

Adapt: keep the cursorless camera roundtrip (dive to the answer, travel to the second point, land); text column right ~34%.
Scene 1 (0.0–2.6s): the window arrives already dived onto the guide's title and its quick-answer box; right column: kicker "✱ YANG LAKU" and headline "Yang paling laku:".
Scene 2 (2.6–6.6s): a blue marker underline draws under "Code review, testing, and git automation skills sell the most" at 2.8s; three chips pop into the right column at 3.0s: "code review", "testing", "otomasi git" (tile cards, mono-label).
Scene 3 (6.6–12.0s): the camera travels (one smooth move, done by 7.6s) to the quick answer's last sentence ("The biggest untapped opportunities ... DevOps workflows, and language-specific skills for Python and JavaScript"), an orange-free blue underline draws at 7.8s; headline "Peluang yang masih terbuka:" and two chips "DevOps", "Python dan JavaScript" pop at 8.2s; hold still.

## Frame 8 — Delapan cek keamanan

- scene: The security guide beside a checklist of the eight checks ticking off one by one
- voiceover: "Sebelum tayang, setiap skill lolos — 8 cek keamanan otomatis."
- duration: 11s
- transition_in: crossfade
- status: animated
- src: compositions/frames/08-keamanan.html
- type: feature_showcase
- persuasion: Risk reversal (buyers can trust what they buy)
- beat: trust
- blueprint: agent-progress-theater (Adapt)
- asset_candidates: assets/agensi-guide-security-scan.png — guide "How the Agensi Security Scan Works": every skill passes an 8-point automated scan: prompt injection, data exfiltration, dangerous commands, hardcoded secrets, obfuscated code, network access, zip structure, SKILL.md validity
- focal: typography (the checklist)
- roles: agensi-guide-security-scan = supporting (a window, right ~44%)
- host_sfx: done.wav at 8.6s

narrativeRole: trust: buyers buy because skills are checked; sellers know the bar.
keyMessage: every skill passes 8 automated security checks before it goes live.

Adapt: keep the receipt signature (rows arrive and check off one by one), no typed prompt; the trigger is the headline.
Scene 1 (0.0–2.4s): right window with agensi-guide-security-scan.png (title and quick answer readable); left: kicker "✱ KEAMANAN", headline "Sebelum tayang, setiap skill lolos" then "8 cek keamanan otomatis." in display-italic with "8" in blue (the voltage) at 1.6s.
Scene 2 (2.4–8.6s): a tile card lists eight rows that arrive and tick (a small check drawing on each, every 0.75s from 2.6s): "Prompt injection", "Kebocoran data", "Perintah berbahaya", "Kunci rahasia di file", "Kode tersamar", "Akses jaringan mencurigakan", "Struktur zip", "Validitas SKILL.md"; the last tick lands at 7.85s and the card's footer "8/8 lolos" appears at 8.6s.
Scene 3 (8.6–11.0s): hold still.

## Frame 9 — Contoh kami: project-brain

- scene: Our origami crane and project-brain's graph view, with an honest status: in review
- voiceover: "Kami juga mencoba: project-brain. — Knowledge graph untuk kamu dan AI agent-mu. — Status: sudah tayang di Agensi."
- duration: 14s
- transition_in: zoom-through
- status: animated
- src: compositions/frames/09-project-brain.html
- type: social_proof
- persuasion: Authenticity (we did it ourselves)
- beat: belonging
- blueprint: device-surface-showcase (Adapt)
- asset_candidates: assets/project-brain-logo.svg — our origami crane logo; assets/project-brain-1-graph-view.png — project-brain's graph view, a whole project as one knowledge graph; assets/project-brain-2-explain-with-citations.png — answers with path:line citations
- focal: assets/project-brain-1-graph-view.png
- roles: project-brain-logo = cutout (small, beside the name) · project-brain-1-graph-view = cutout (the window) · project-brain-2-explain-with-citations = supporting (the window's second screen)
- host_sfx: ting.wav at 10.2s

narrativeRole: we are doing this too; a real example, with an honest status.
keyMessage: project-brain is our skill on Agensi, in review now.

Adapt: keep the window-hero signature with its screens advancing (two real screens), cursorless; asymmetric 38/62; a breather once built.
Scene 1 (0.0–3.2s): left: the crane logo settles (~110 px tall) beside kicker "✱ CONTOH KAMI"; headline "Kami juga mencoba:" then "project-brain" (display, ink) at 1.6s. Right ~60%: the window rises in with project-brain-1-graph-view.png.
Scene 2 (3.2–9.6s): cue "Knowledge graph untuk kamu dan AI agent-mu." (headline-italic) reveals at 3.8s; at 6.4s the window's screen advances (scale-swap) to project-brain-2-explain-with-citations.png.
Scene 3 (9.6–14.0s): a status pill appears at 10.2s under the text: "Status: sedang direview Agensi" (tile card, the word "direview" in blue, the voltage); hold still.

## Frame 10 — Cara mulai

- scene: Four steps from Agensi's guide build up, then the sell page and the call to action
- voiceover: "Cara mulai: — 1. Buat akun, sambungkan Stripe. — 2. Kemas skill jadi zip berisi SKILL.md. — 3. Kirim untuk review keamanan. — 4. Pasang harga, lalu tayang. — agensi.io/sell"
- duration: 13s
- transition_in: crossfade
- status: animated
- src: compositions/frames/10-cara-mulai.html
- type: cta
- persuasion: Friction reduction (four steps)
- beat: motivation
- blueprint: grid-card-assemble (Adapt)
- asset_candidates: assets/agensi-sell.png — Agensi's creator page "How to make money with AI: sell agent skills", button "Start Selling", 70% to you; assets/agensi-guide-how-to-sell.png — guide "How to Sell Skills on Agensi" with the quick answer: account, Stripe, zip with SKILL.md, security review, price, live
- focal: typography (the four steps) then the CTA
- roles: agensi-sell = supporting (window, right ~44%, appears for the CTA) · agensi-guide-how-to-sell = supporting (window, right, first)
- host_sfx: pop.wav at 2.0s, pop.wav at 3.6s, pop.wav at 5.2s, pop.wav at 6.8s, ting.wav at 9.0s

narrativeRole: make starting feel small, then the one action.
keyMessage: four steps, start at agensi.io/sell.

Adapt: keep the accumulating-list signature (steps pop into slots and stay), extended to a CTA pill.
Scene 1 (0.0–1.8s): right window with agensi-guide-how-to-sell.png (title and quick answer); left: kicker "✱ CARA MULAI", headline "Cara mulai:".
Scene 2 (1.8–8.2s): four numbered rows pop into the left list at 2.0s, 3.6s, 5.2s and 6.8s: "1. Buat akun, sambungkan Stripe.", "2. Kemas skill jadi zip berisi SKILL.md.", "3. Kirim untuk review keamanan.", "4. Pasang harga, lalu tayang." (lead, ink; numbers in mono).
Scene 3 (8.2–13.0s): the right window's screen advances to agensi-sell.png (scale-swap) at 8.4s; a blue pill (`coral-callout`, the voltage) "agensi.io/sell" wipes in under the list at 9.0s, with the mono-label "panduan lengkap di Agensi" beneath; hold; the whole frame fades to the navy ground in the last 0.5s (the video's only real exit).
