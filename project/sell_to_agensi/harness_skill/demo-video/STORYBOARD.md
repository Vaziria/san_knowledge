---
format: 1920x1080
duration: 60s
message: "One living knowledge graph for you and your AI agents"
arc: PAS into a demo loop - hook (pain) -> product intro -> demo 1 graph -> demo 2 explain -> demo 3 outdated -> demo 4 task -> benefits -> CTA
audience: developers who run AI coding agents across many sessions on a long-lived project; Agensi.io skill buyers
mode: autonomous
music: none
captions: none
---

No narration. Each frame's `voiceover` line is its ON-SCREEN TEXT (no audio voice): set it as typography, revealed cue by cue at the em-dashes / sentence breaks. The music bed is the user's own track (host audio, see audio_meta.json); hence `music: none`. All UI shown is real: screenshots of san_knowledge's view on the Paperboat sample project. Never invent UI; zoom, crop, highlight and annotate the real screens.

## Video direction

- **Palette (frame.md roles):** ground = `cream` key, navy #1f2a44, on every frame (as a full-duration background clip). Voice = `ink` #f3f2ee for all headlines and body. Voltage = `coral` key, the brand blue #2a78d6: exactly ONE per frame (the key word, the marker ring, or the CTA pill). Second accent = `tile-strong` orange #eb6834: only the crane's own orange and the "outdated" emphasis in Frame 5. Content cards = `tile` #26324f with the hairline border. Terminal / code cards = the `code-surface` component (deep navy #121a2c, cream JetBrains Mono, title bar in navy-elev).
- **Real UI windows:** the screenshots (3840x2160, i.e. 2x of 1920x1080) are the ONLY UI. Show them as floating windows on the navy ground: radius-lg corners, the hairline border, one soft shadow, no browser chrome. Scale them down to fit; push in (zoom-to-target) up to 2x and stay sharp. Never rebuild, repaint or retype UI. Allowed: crop/zoom, a ground-matched patch that hides a real element until its cue (then reveal the real pixels), a typed-text overlay in an input field, and marker emphasis (rounded box / underline) drawn ON TOP of the real pixels.
- **Type (frame.md ramp):** headlines in `headline` or `display` (EB Garamond 400, sentence case); the one emphasized phrase per frame in `display-italic`; frame labels as the `kicker-spike` component (JetBrains Mono uppercase with the blue ✱ prefix, 2-5 words); paths and line numbers in `mono-label`. No emoji, no icons in text.
- **Motion grammar:** long-tail `power3` settles, smooth over bouncy (no back/bounce/elastic). Reveal model: text cues reveal one at a time at reading pace (about 0.3 s per word plus a short gap), each piece appears only when its cue starts; nothing is front-loaded. Camera moves (zoom-to-target / pan) happen in the FIRST part of a scene to point at the proof, then hold; no slow pan or push in a frame's back half. During holds: stillness, at most a subtle jitter. Entrances use fromTo; no repeat/yoyo, no random, no CSS animation. Only Frame 8 has a real exit.
- **Rhythm:** high energy in Frame 1 (hard-cut beats), Frame 4 (type, submit, answer) and Frame 5 (the badge snap). Deliberate held breathers: Frame 2 (calm title cards on the music's first full phrase) and Frame 7 (list builds, then holds). Frame 8 ends on a held lockup and fades with the music.
- **Music:** the bed's big phrase lands at 5.0 s, exactly where Frame 2 starts; Frame 2's crane settle lands on it.
- **Keep-out:** plan all important content into the top ~83% of the canvas.
- **Negative list:** no rebuilt or fake UI, no invented numbers or names, no stock imagery, no bokeh or purple "AI" gradients, no glow halos on content, no bounce, no lazy breathing, no screensaver (elements floating independently), no slideshow (everything on screen in the first quarter, then frozen), no back-half slow pans/pushes.

## Frame 1 — Starts from zero

- scene: Bold type on the navy ground: an agent session restarts and forgets everything
- voiceover: "New session. — Your agent forgot everything. — So you explain the project. Again."
- duration: 5s
- transition_in: cut
- status: animated
- src: compositions/frames/01-starts-from-zero.html
- type: hook
- persuasion: Pain validation
- beat: frustration
- blueprint: kinetic-type-beats (Adapt)
- asset_candidates:
- focal: typography only
- roles: none
- sfx: none

narrativeRole: name the pain every multi-session agent user knows, in their own words, before any product.
keyMessage: re-explaining your project to the agent every session is the problem.

Adapt: keep the statement-builds-across-beats signature with hard-cut swaps; three short statements, the last one escalated by a one-word punch.
Scene 1 (0.0–1.3s): navy ground; centered, ~50% width: kicker "✱ NEW AGENT SESSION" fades in above; the display line "New session." lands via kinetic beat-slam (`kinetic-beat-slam`) on a smooth settle. Centered hero, 2 layers.
Scene 2 (1.3–2.9s): hard-cut word-swap (`discrete-text-sequence`) to "Your agent forgot everything." in `display`; on "forgot everything" (display-italic) the words dim from full ink to ~30% left to right, as if being forgotten (opacity sweep, deterministic per-letter stagger).
Scene 3 (2.9–5.0s): hard cut to "So you explain the project." (headline size, centered upper-middle); at 4.0s "Again." punches in beneath it in `display` blue (the frame's one voltage) via kinetic beat-slam and holds still to the cut.

## Frame 2 — Meet project-brain

- scene: The origami crane mark settles, then the name and the one-line promise land on calm title cards
- voiceover: "project-brain. — One living knowledge graph — for you and your AI agents."
- duration: 6s
- transition_in: zoom-through
- status: animated
- src: compositions/frames/02-meet-project-brain.html
- type: product_intro
- persuasion: Negative contrast (forgetting -> one shared memory)
- beat: curiosity + relief
- blueprint: titlecard-reveal (Adapt)
- asset_candidates: assets/project-brain-logo.svg — origami crane logo on a navy tile (blue and green wings, orange body)
- focal: assets/project-brain-logo.svg
- roles: project-brain-logo = cutout (the mark, centered)
- sfx: none

narrativeRole: name the product and land the promise by beat 2.
keyMessage: one shared, living knowledge graph replaces re-explaining.

Adapt: keep the near-still title chain with ONE restrained move per card; the mark is the first card, the name and the claim follow with blur-snap handoffs; centered stack.
Scene 1 (0.0–1.9s): the crane logo (its navy tile matches the ground; show it at ~24% of frame height, centered upper-middle) arrives with a smooth spring-pop entrance (`spring-pop-entrance`, settle, no overshoot) landing exactly on the music hit at 0.0–0.6s; holds.
Scene 2 (1.9–3.6s): blur-snap handoff: the mark eases up a little; the wordmark "project-brain" (display, ink, sentence case as written) slide-up crossfades in beneath it; holds.
Scene 3 (3.6–6.0s): the claim reveals in two cues beneath the wordmark in `display-italic` at headline size: "One living knowledge graph" at 3.6s, "for you and your AI agents." at 4.6s; the word "living" is the frame's one blue voltage. Held still to the end (a breather).

## Frame 3 — Every file becomes one graph

- scene: The real view opens on five domains, then the whole project unfolds into one connected graph
- voiceover: "Every markdown file in your project — docs, specs, decisions, tasks — becomes one graph you can see."
- duration: 9s
- transition_in: blur-crossfade
- status: animated
- src: compositions/frames/03-one-graph.html
- type: feature_showcase
- persuasion: Show-don't-tell proof
- beat: curiosity + awe
- blueprint: device-surface-showcase (Adapt)
- asset_candidates: assets/01-overview.png — the graph's top level, five domain hexagons and the docs without a domain, header counts "5 domains · 20 docs · 50 sections"; assets/05-expanded.png — every doc and section as one connected graph, coloured by community
- focal: assets/05-expanded.png
- roles: 01-overview = supporting (the window's first screen) · 05-expanded = cutout (the window's payoff screen)
- sfx: none

narrativeRole: the first look at the real surface; the "living knowledge graph" made visible.
keyMessage: your whole project, as one graph, in your browser.

Adapt: keep the cursorless stepwise-flow signature (one floating window is the hero and its screen advances through a real flow); two real states instead of a full loop; asymmetric 35/65 layout.
Scene 1 (0.0–1.6s): asymmetric 35/65, 3 depth layers. Right ~62% of the width: the floating window rises in (fromTo y + opacity) showing 01-overview.png scaled to fit. Left column: kicker "✱ YOUR PROJECT, AS A GRAPH", then headline "Every markdown file in your project" reveals.
Scene 2 (1.6–4.2s): beneath the headline, four mono chips reveal one by one as tile cards (per-word staggered reveal, `dynamic-content-sequencing`): "docs", "specs", "decisions", "tasks". Meanwhile, in the first part of this window only, the window content zooms to target (`coordinate-target-zoom`) onto the left half of the graph canvas so the domain hexagons and their "(+N)" labels read large.
Scene 3 (4.2–6.6s): "becomes one graph" reveals (headline, ink); inside the window the screen advances: a scale-swap (`scale-swap-transition`) from the zoomed 01-overview to 05-expanded shown whole, so the five domains unfold into the full connected graph.
Scene 4 (6.6–9.0s): "you can see." lands in display-italic with "see" in blue (the voltage); the window holds still on the expanded graph (subtle jitter at most). Held read.

## Frame 4 — Answers with file and line

- scene: A question types into the real search box; the panel answers with matches citing path:line, and the graph lights up only the context
- voiceover: "Ask a question. — Get the answer — with the file and line it came from. — Your agents get the same answer, over MCP."
- duration: 10s
- transition_in: crossfade
- status: animated
- src: compositions/frames/04-answers-cite.html
- type: feature_showcase
- persuasion: Show-don't-tell proof + Risk reversal (answers you can check)
- beat: clarity + trust
- blueprint: prompt-type-submit-generate (Adapt)
- asset_candidates: assets/03-explain.png — Explain for "why did we drop the free tier", 8 matches with paths and lines (docs/product/pricing.md:15), the graph highlighting only the context; assets/terminal-explain.txt — the real CLI explain output an agent receives (loc: docs/product/pricing.md:15)
- focal: assets/03-explain.png
- roles: 03-explain = cutout (the result window) · terminal-explain = supporting (a code-surface card quoting its real lines)
- registry_blocks: terminal-simulator (the code-surface card streaming the real terminal-explain lines; restyle to frame.md code-surface) · marker-highlight (the blue box around docs/product/pricing.md:15)
- sfx: none

narrativeRole: the core loop: ask, get a cited answer; the agent side shown with the real terminal output.
keyMessage: every answer is checkable, for you and for every agent session.

Adapt: keep the full ask → answer loop on the REAL input (the query types into the real search box, the real result surface answers); the second half adds the agent's terminal view instead of a second query. Layout: top text strip + one large centered window beneath.
Scene 1 (0.0–3.0s): top strip: kicker "✱ EXPLAIN" and headline "Ask a question." Beneath, the window shows 03-explain.png zoomed to target on its top bar search box (search box + Explain button fill most of the window); a ground-matched patch hides the real query, and a typed overlay types "why did we drop the free tier" character by character with a simple blinking caret (`discrete-text-sequence`), matching the box's font and position; at 2.6s the real Explain button gets a short press (a slight scale-down and back, smooth), then the patch is gone and the real query pixels show.
Scene 2 (3.0–6.4s): on the submit, the window zooms out (one decelerating move, `coordinate-target-zoom` reversed) to show the whole result: the side panel's 8 matches with paths and lines, the graph lighting only the context. Headline cue 2 "Get the answer" reveals, then cue 3 "with the file and line it came from." (headline, ink). At 5.0s a short zoom-to-target onto the panel's second match, and at 5.4s a blue rounded marker box draws around its line `docs/product/pricing.md:15` (`css-marker-patterns`, the frame's one voltage).
Scene 3 (6.4–10.0s): a code-surface card (title bar "knowledge explain") slides in from the right edge, overlapping the window's right third (layered depth, 3 layers); its real lines from terminal-explain.txt stream in one by one (terminal log append): "## 2. Why there is no free tier (doc_section ...)", "- loc: docs/product/pricing.md:15", "- keyword: free tier, trial, conversion", "The free tier was dropped in September after 4,100 sign-ups produced only 38 upgrades; a 14-day Pro trial replaced it." Headline cue 4 "Your agents get the same answer, over MCP." reveals at 7.0s. Hold from 8.6s.

## Frame 5 — Nothing goes stale silently

- scene: The real section panel: the old summary says two months free, the edited text says three; a dashed "needs summary" badge snaps on
- voiceover: "Change the text — and the old summary is flagged as outdated. — Nothing goes stale silently."
- duration: 9s
- transition_in: push-slide LEFT
- status: animated
- src: compositions/frames/05-flags-outdated.html
- type: feature_showcase
- persuasion: Feature-to-benefit translation
- beat: tension -> control
- blueprint: panel-edit-live-sync (Adapt)
- asset_candidates: assets/07-outdated.png — section "Annual discount" with the dashed needs-summary badge, the old summary (two months free, 17%) above the opened section text (three months free, 25%); assets/terminal-sync.txt — the real sync output after the edit (sections ~1, needs summary: 14)
- focal: assets/07-outdated.png
- roles: 07-outdated = cutout (a tall crop of the real side panel, x 0-1200, y 120-1300 of the 3840 image) · terminal-sync = supporting (a small code-surface strip)
- registry_blocks: terminal-simulator (the small strip appending the real terminal-sync lines; restyle to frame.md code-surface) · marker-highlight (the orange underline under the old summary)
- sfx: none

narrativeRole: the trust beat: the graph knows when it is wrong.
keyMessage: outdated knowledge is flagged, not trusted.

Adapt: keep the live-sync couple signature: the edit (section text) and its bound surface (badge + old summary) change in the same beat; the editor and the target are both parts of the real panel. Split layout 55/45: panel crop left, text column right.
Scene 1 (0.0–2.6s): left ~55%: a tall window with the real side-panel crop of 07-outdated.png (title "Annual discount", `docs/product/pricing.md:11`, the old summary line, and the opened Section text box). Ground-matched patches hide the dashed "needs summary" badge and the section text. Right column: kicker "✱ STAYS TRUE" and headline "Change the text". In the Section text box, a typed overlay shows "Annual plans get two months free (17% off)." then backspaces "two months free (17% off)" and retypes "three months free (25% off)" (backspace-and-retype, `discrete-text-sequence`); at the end of the retype the patch drops and the real section-text pixels show.
Scene 2 (2.6–5.4s): the couple fires: the real "needs summary" badge snaps in (spring-pop entrance, smooth settle) at 2.8s, and an orange underline marker (`css-marker-patterns`, tile-strong orange) draws under the old summary line "two months free (17% off)" at 3.4s. Headline cue 2 "and the old summary is flagged as outdated." reveals. At 4.2s a small code-surface strip appears under the text column and its real lines from terminal-sync.txt append one by one: "sections: +0 ~1 -0", "needs summary: 14".
Scene 3 (5.4–9.0s): cue 3 "Nothing goes stale silently." lands in display-italic, "silently" in blue (the voltage). Held still to the end.

## Frame 6 — Finished work links itself in

- scene: The real panel of a finished task shows its Done section, then the camera travels along the reference edge to the spec it implemented
- voiceover: "Finish a task — it links itself into the graph, — with what changed and how it was checked."
- duration: 8s
- transition_in: push-slide LEFT
- status: animated
- src: compositions/frames/06-work-links-in.html
- type: feature_showcase
- persuasion: Show-don't-tell proof
- beat: confidence
- blueprint: camera-journey (Adapt)
- asset_candidates: assets/04-task-done.png — finished task "Invoice PDF with a payment link": Done summary (112 passing tests), sections, reference to "Spec: invoice PDF", the task node linked to the spec and the Engineering hub in the graph
- focal: assets/04-task-done.png
- roles: 04-task-done = cutout (one large window, ~90% width)
- registry_blocks: marker-highlight (the blue underline under the Done line)
- sfx: none

narrativeRole: work becomes knowledge; the workflow trail builds itself.
keyMessage: every finished task leaves a linked, checked record.

Adapt: keep the action-roundtrip signature (dive to the panel → travel to where the consequence lives in the graph → landing push); no cursor; the "action" is the finished task itself. Layered layout: one large window (~90% width, top-aligned), the text on a small tile card overlaid at the window's lower-right (inside the top 83%).
Scene 1 (0.0–2.4s): the window with 04-task-done.png arrives already dived in (zoom-to-target on the side panel's title "Invoice PDF with a payment link" and its Done section line "make test passed (112 tests, 9 new)"); a blue marker underline draws under that Done line at 1.2s (the frame's one voltage). Overlay card: kicker "✱ TASKS" + headline "Finish a task".
Scene 2 (2.4–5.2s): cue 2 "it links itself into the graph," reveals. The camera travels (`viewport-change`, one smooth move, done by 4.4s) from the panel's References entry "Spec: invoice PDF" across to the graph, landing on the task node and its reference edge to "Spec: invoice PDF" plus its domain_of edge to the Engineering hub; an ink-colored dashed stroke self-draws over the real reference edge (`svg-path-draw`) as the camera arrives.
Scene 3 (5.2–8.0s): cue 3 "with what changed and how it was checked." reveals on the card; the camera eases back to a medium framing that shows the panel and the linked nodes together, settling by 6.2s; held still to the end.

## Frame 7 — Yours, local, every agent

- scene: Four short value lines assemble one by one over a dimmed table of every node
- voiceover: "Your files stay the source of truth. — Runs locally. No API key. — One MCP server for every agent session. — Built in Go, on an embedded graph database."
- duration: 7s
- transition_in: crossfade
- status: animated
- src: compositions/frames/07-yours-local.html
- type: benefit_highlight
- persuasion: Value stacking + Friction reduction
- beat: ease + control
- blueprint: grid-card-assemble (Adapt)
- asset_candidates: assets/06-table.png — table of every node with path:line locations and done / needs-summary chips (dimmed backdrop only)
- focal: typography (the accumulating list)
- roles: 06-table = background (full-bleed, dim to ~25% and soft blur, `depth-of-field-blur`)
- sfx: none

narrativeRole: answer the buyer's practical questions (ownership, privacy, compatibility, stack) in one breath.
keyMessage: no lock-in, no cloud, works with every agent.

Adapt: keep the accumulating vertical-list signature (lines populate their slots one by one and stay co-resident); four lines on hairline rows; centered-left column ~60% width over the dimmed table backdrop.
Scene 1 (0.0–1.4s): the 06-table backdrop fades in dimmed and softly blurred; kicker "✱ NO LOCK-IN" above an empty 4-slot list; line 1 "Your files stay the source of truth." pops into slot 1 (spring-pop entrance, smooth), "Your files" in blue (the frame's one voltage).
Scene 2 (1.4–2.8s): line 2 "Runs locally. No API key." pops into slot 2.
Scene 3 (2.8–4.2s): line 3 "One MCP server for every agent session." pops into slot 3.
Scene 4 (4.2–5.6s): line 4 "Built in Go, on an embedded graph database." pops into slot 4 (a mono-label "goraphdb" chip may sit at its end).
Scene 5 (5.6–7.0s): all four lines co-resident; held still (a breather before the close).

## Frame 8 — project-brain on Agensi

- scene: The origami crane folds together piece by piece into the lockup, then the call to action appears
- voiceover: "project-brain — One living knowledge graph for you and your AI agents. — Find it on Agensi.io"
- duration: 6s
- transition_in: zoom-through
- status: animated
- src: compositions/frames/08-on-agensi.html
- type: cta
- persuasion: Future pacing
- beat: motivation
- blueprint: logo-assemble-lockup (Adapt)
- asset_candidates: assets/project-brain-logo.svg — origami crane logo whose wings, neck, tail and body are separate polygons that can assemble
- focal: assets/project-brain-logo.svg
- roles: project-brain-logo = cutout (inline its polygons as separate parts; the navy tile rect may be dropped since it matches the ground)
- registry_blocks: logo-outro (piece-by-piece assembly + tagline + URL pill; customize: the crane polygons as the pieces, NO glow bloom, the pill reads "Find it on Agensi.io")
- sfx: none

narrativeRole: brand lockup and the one action.
keyMessage: get project-brain on Agensi.io.

Adapt: keep the parts-assembly signature (the mark comes to exist from its parts) with the text-clear-bloom lockup (mark + wordmark balanced), extended to the CTA line; static camera.
Scene 1 (0.0–2.2s): cleared navy stage; the crane's pieces fold in one at a time like paper (each polygon flips in from a slight rotateY/rotateX of its own, scale 0.9→1, opacity, deterministic stagger): back wing, tail, neck and head, body, then the front wing last; the mark is centered, ~30% of frame height.
Scene 2 (2.2–3.8s): the mark slides a short distance left while the wordmark "project-brain" (display, ink) reveals to its right (slide-out wipe); the balanced mark + wordmark lockup centers and holds.
Scene 3 (3.8–6.0s): beneath the lockup, the lead line "One living knowledge graph for you and your AI agents." fades up (Inter lead, ink); at 4.6s a blue pill (`coral-callout` component, the one voltage) with "Find it on Agensi.io" wipes in left to right; hold; in the last 0.5s the whole frame fades to the navy ground (the video's only real exit, with the music fade).
