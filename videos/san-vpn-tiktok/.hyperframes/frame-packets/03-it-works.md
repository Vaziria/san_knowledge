# Frame packet: 03-it-works

## Project inputs

- Project: D:\research\marketing\videos\san-vpn-tiktok
- Design tokens: D:\research\marketing\videos\san-vpn-tiktok\frame.md
- RULES_DIR: C:\Users\heris\.claude\skills\hyperframes-animation\rules

## Assigned storyboard block

## Frame 3 — It works

- scene: status shows the other machines online by name; ping office.vpn answers
- voiceover: ""
- duration: 6s
- transition_in: crossfade
- status: outline
- src: compositions/frames/03-it-works.html
- type: benefit_highlight
- persuasion: Show-don't-tell proof
- beat: confidence
- blueprint: agent-progress-theater (Adapt)
- focal: the ping card
- roles: none (no assets)
- asset_candidates:
- sfx: host — keys @0.2, keys @2.4, done @3.3

narrativeRole: the proof — machines in different places on one network, reachable by name.
keyMessage: from home, office.vpn answers.

Adapt: keep the receipt-cascade signature (rows arrive one by one and flip to a done state);
two stacked cards instead of a checklist.
Layout: two stacked terminal cards (same styling as frame 2), status card in the upper half
(~12–45% height), ping card below (~50–75%). Card header bars in mono: "PC RUMAH".
Scene 1 (0.0–0.2s): both cards' frames present, empty; status card caret blinking.
Scene 2 (0.2–0.8s): types `san_vpn status` in the status card.
Scene 3 (1.0–1.8s): output rows cascade in one by one (slide-up + fade, staggered), mono:
  `home  10.77.0.2/24  connected`
  `NAME             IP         RELAY` (cream-muted)
  `office.vpn       10.77.0.1  online`
  `jakarta-vps.vpn  10.77.0.3  online`
  ("online" and "connected" in fire-orange; columns aligned).
Scene 4 (2.4–3.0s): types `ping office.vpn` in the ping card.
Scene 5 (3.3s, 3.7s, 4.1s): three lines `Reply from 10.77.0.1` arrive one by one, each with a
  fire-orange ✓ at its end.
Scene 6 (4.3–4.9s): one small punch-in (coordinate-target-zoom, ~1.12×) centring the ping card.
Scene 7 (4.9–6.0s): hold still.

## Selected blueprint: agent-progress-theater

# agent-progress-theater — Agent Progress Theater

**intent**: Agent work performed as WORKING-STATE theater — a short trigger beat hands the frame to the machine, which then visibly _works_: loaders spin, status phrases swap, dots pulse, counters tick — before the receipt arrives as a card whose rows cascade in and CHANGE STATE (badges flip to checks, labels strike through, severity pills read out), or as a conversation thread building message-by-message onto a camera push-in payoff. The subject is the machine performing labor over time. It is NOT a typed prompt awaiting output (no prompt/input is ever typed — the trigger is a click, a menu choice, or an already-running scan); NOT `cursor-ui-demo` (at most ONE igniting click here, then the cursor exits and the UI performs itself); NOT `grid-card-assemble` (rows there assemble into a static enumeration and hold — rows here are alive: they arrive as agent output and then MUTATE, checking off one by one while the viewer watches).

**roles served**

- Key_Feature (from `agent-progress-theater`): when the feature is the agent doing multi-step work (build a plan / scan a repo / fix a vulnerability / handle infra for you) and the proof is status theater — a loader lockup with a typed label, status couplets swapping under an `[accent]` spinner, then a checklist/findings card that populates and checks off in front of the viewer.
- Key_Feature (from `message-thread-payoff`): when the agent's work lives inside a conversation or automation thread — user/agent bubbles and tool-call/reply cards popping in sequence, the working state carried by pulsing loading dots or rapidly ticking diff counters, resolved by ONE camera push-in tight on the confirmation line (`[reaction pill]`, "Sent using `[@Bot]`", a thank-you bubble).

**duration**: 4.2–11.6s (short members are a single card-and-check-off or thread beat at ~4–5s; long members chain trigger → interstitial → status swaps → receipt card at ~9–12s; thread payoff spans 4.2–9.1s)

**shot structure** (a warm flat canvas — `[off-white / warm beige / near-white bg]`, optional `[faint grid / dot-grid / wavy-line]` texture; white rounded cards with soft drop shadows; ONE `[working accent]` color reserved for the machine (spinner, status words, active step) and one `[done color]` for completion (checks, "Completed"); camera static or ONE slow move — motion is overwhelmingly element-level springs, staggers, and state flips. Two folded sub-shapes — **(A) checklist/findings theater** and **(B) message-thread payoff**.)

- **Scene 1 (0.0–~1.5s) — the trigger.** Something asks the machine to work, in ONE beat:
  - _Variant — option menu (A)_: a centered white pill card poses `[the question]`; it SPRINGS open downward into a rounded menu — `[3–4 option rows]` fade/slide in staggered, each with a number badge. A cursor enters, hover-dances between rows (a pale `[hover fill]` highlight follows it), and CLICKS the chosen row (~press-down spring); the whole menu scales down toward its center and fades out. This is the only cursor appearance in the shot.
  - _Variant — modal click (A)_: close-up of a white modal with `[Dismiss]` / `[action button]`; a hand cursor clicks the action (quick press-down spring); the modal fades away. Optionally followed by a serif `[interstitial line]` on the bare canvas — words land staggered, hold, fade out word-staggered as the bg swaps.
  - _Variant — already working (A)_: a `[Scan in progress]`-style state — a thin `[accent]` arc spinner rotating over a heading + body copy + a `[Starting…]` pill (cursor resting on it, motionless); only the spinner moves. The whole scene then rapidly scales up and fades — a push-through exit.
  - _Variant — workspace push-through (A)_: a rapid camera push-in THROUGH a multi-panel `[workspace: builder / editor / terminal]` — panels scale past the viewport edges and clear away to the bare canvas.
  - _Variant — thread opener (B)_: a `[user bubble]` spring-pops in ("`[the ask]`"), OR a stats card pops in whose green/red `[diff counters]` rapidly tick and settle — the automation's opening receipt.

- **Scene 2 (~1–4s) — the working state (the machine performs).** The frame belongs to the machine; nothing is clickable. Pick 1–3 working motifs and CHAIN them:
  - A loader lockup: a spinning `[accent asterisk / arc]` beside a `[working label]` typed on rapidly ("`Buildi` → `Building plan…`"), a left→right shimmer sweep passing through the letters; the spinner may momentarily morph asterisk↔dot and back.
  - Status couplets: 2–3 centered pairs — a dark `[action line]` over an `[accent status word]` ("Thinking…", "Noodling…") with its spinner — swapping via quick fades/slides at a steady cadence.
  - A `[scan/tool label]` types/expands rightward to its full string, then SHRINKS and DOCKS to the top-left as a fixed corner header (the canvas now belongs to what it produces).
  - A status heading flips tense as rows land beneath it ("Using `[Tool]`" → "Used `[Tool]`"), with a gently pulsing "Thinking" and gray meta-lines ("Exploring `[N]` files…") fading in below.
  - _Variant — thread machinery (B)_: the `[agent reply]` fades/slides up, then a monospace `[tool_call]` line appears beneath it — small icon + `[tool name]` + three pulsing loading dots; OR an instruction bubble scrolls into view (internal window scroll, frame static) followed by a `[brand logo]` pop-in beside a "Sending message…" row. **The pulse dies the instant the result lands** — dots vanish as the payload arrives.

- **Scene 3 (~2–4s) — the receipt cascades in (the payoff engine).** The work materializes as a card that BUILDS:
  - _Variant — checklist (A)_: a white `[Progress / summary]` pill or card SPRING-pops in with a bounce, then springs open downward (or the summary card glides UP as a taller `[findings]` panel expands beneath it). Rows cascade in one by one — slide-up + fade, staggered — each with `[number badge / severity pill]` + `[label]` + optional gray `[meta line]`. Then the STATE MUTATION runs: badges flip one by one from numbered outline to a solid `[done color]` circle + white checkmark (slight scale bounce), the checked label simultaneously strikes through and dims; pending items keep partially-drawn arc outlines animating. End the run mid-list — some items checked, some still numbered — the work is visibly _ongoing_.
  - _Variant — thread payload (B)_: the camera pushes in / pans down centering the `[tool_call]` line as a white payload card expands downward from it — 2–4 light monospace `[key: value]` lines fading in. Then the `[resolution message]` expands into place below (inline `[code chips]` and `[link]` coloring), OR a dark `[thread card]` scales up from a status row to DOMINATE the frame while the background darkens, its `[reply]` expanding into place under a "1 reply" divider.

- **Scene 4 (final ~1–2.5s) — resolve.** Two endings:
  - _Variant — hold / scroll (A)_: the finished (or mid-mutation) card stack holds static to the end, OR the viewport scrolls down the final card (fast in the last beat) revealing `[a second heading + numbered list]`, ending mid-list. A slow continuous zoom into the card may run underneath (the header drifts off the top of frame).
  - _Variant — payoff push-in (B)_: ONE camera push-in + pan-down lands tight on the payoff line — "`Sent using [@Bot]`" / the confirmation + `[thank-you bubble]` spring-in — then a `[reaction button]` springs into an active pill with bouncy overshoot and a count. The push eases into a gentle near-imperceptible drift and the clip ends on the close-up. No end card.

**motion vocabulary**: pill springs open downward into a menu/checklist · option rows fade/slide in staggered · cursor hover-dance (pale highlight fill follows the cursor between rows) · single igniting click with press-down spring · menu scale-down fade exit · modal fade-away · thin `[accent]` arc spinner rotation · spinning asterisk loader · asterisk↔dot morph · typed-on loader label with caret · left→right text shimmer sweep · serif interstitial with word-staggered fade in/out · status couplets swapping via quick fades/slides under an `[accent]` spinner · pulsing "Thinking" label · status heading tense flip (Using→Used) · label types/expands rightward then shrinks and docks as a corner header · scene scale-up/fade push-through exit · rapid camera push-in through a multi-panel workspace · slow continuous zoom into a card (header drifts off frame) · summary card spring pop with bounce · card glides up as a panel expands beneath it · anchored downward panel/payload expansion · rows stagger in (slide-up + fade) · badge flip from numbered outline to solid circle + white checkmark with scale bounce · strikethrough + dim on completion · partially-drawn arc outlines animating on pending items · severity-pill readouts (Critical / High) · viewport scroll down the final card · chat bubble spring scale-up pop-in · reply fade/slide-up · monospace tool-call line with three pulsing loading dots (dots die the instant the result lands) · payload card expands downward from the line · green/red diff counters rapid tick-and-settle · internal window scroll (frame static) · brand logo pop-in beside a status row · card scales up from a row to dominate the frame while the background darkens · reply message expands into place · inline code chips / link coloring · reaction button springs into an active pill with bouncy overshoot + count · camera push-in + pan-down centering the payoff · slight pull-back · gentle end drift · static hold.

**rule mapping**

- pill springs open downward into a menu / panel expands beneath a gliding card / payload card expands downward from a tool-call line → `anchored-layout-expand` (edge-anchored container growth: height-masked wrapper + inner counter-translate, container drawn at final size); spring flavor from `spring-pop-entrance`
- option rows / findings rows / task rows stagger in (slide-up + fade) → `spring-pop-entrance` (staggered-group form, ≤500ms cap) or `gsap-effects` (plain fade+translate stagger) — NOT `waterfall-entry` (its binary no-fade arrival law contradicts this dialect's soft fade/slide cascade)
- cursor glides to a row and clicks; hand cursor clicks the modal button → `cursor-click-ripple` (move + press) + `press-release-spring` (the button's press-down spring)
- pale hover-highlight fill following the cursor between rows → `gsap-effects` (a background fill translated row-to-row; no dedicated rule needed)
- menu scale-down fade exit / scene scale-up push-through exit / palette-for-window swap → `scale-swap-transition`
- thin arc spinner rotation / spinning asterisk loader → `svg-icon-enrichment` (rotating internal SVG parts via `setAttribute('transform','rotate(deg cx cy)')`; timeline-driven, finite)
- asterisk↔dot morph and back → `scale-swap-transition` (two elements morphing at the same center)
- typed-on loader label ("Building plan…") / scan label typing to its full string → `discrete-text-sequence` (+ `context-sensitive-cursor` for the caret)
- left→right shimmer sweep through the loader letters → `ambient-glow-bloom` (single-pass traveling sheen) or `css-marker-patterns` (highlight sweep) — pick sheen for light-on-text, marker for a drawn band
- serif interstitial word-staggered fade in/out; status couplets swapping on a cadence → `dynamic-content-sequencing` (phrase windows) + `discrete-text-sequence` (the whole-state swaps); per-word stagger via `gsap-effects`
- pulsing "Thinking" label / three pulsing loading dots (phase-offset) → `sine-wave-loop` (finite repeats; kill the tween at the resolve beat — see doctrine note)
- status heading tense flip (Using→Used) / gray meta-lines fading in / final-token snaps → `discrete-text-sequence`
- label shrinks and docks to the top-left as a fixed corner header → `gsap-effects` (plain scale + translate tween; no dedicated rule needed)
- rapid camera push-in through the multi-panel workspace → `viewport-change` (the push) + `multi-phase-camera` (phasing) + optional `motion-blur-streak` (velocity blur as panels clear the frame)
- slow continuous zoom into the receipt card (header drifts off top) → `multi-phase-camera` (steady-push phase) or `viewport-change`
- summary card / progress pill / chat bubble / brand logo / file chip spring pop-in → `spring-pop-entrance`
- summary card glides up as the findings panel expands beneath → `gsap-effects` (the glide) + `anchored-layout-expand` (the panel)
- badge flip: numbered outline → solid circle + white checkmark with scale bounce → `scale-swap-transition` (outline↔solid swap at same center) + `svg-path-draw` (checkmark draw-in) + `spring-pop-entrance` (the bounce); the pending→active→complete progression itself → `dynamic-content-sequencing` (a snap state machine, per cursor-ui-demo's workflow-approve-press precedent)
- strikethrough + dim on the checked label → `css-marker-patterns` (strike-through draw) + `gsap-effects` (opacity dim)
- partially-drawn arc outlines animating on pending items → `svg-path-draw` (partial dashoffset, held mid-draw)
- viewport scroll down the final card / internal window scroll under a static frame → `gsap-effects` (transform-only content translate inside a masked window) — use `viewport-change` only if the FRAME moves
- green/red diff counters rapid tick-and-settle → `counting-dynamic-scale` (numeric proxy count-up; suppress the scale-growth component — these tick at fixed size)
- dark thread card scales up from a row to dominate the frame → `card-morph-anchor` (row → full-frame morph + handoff) with the background darkening as a `gsap-effects` overlay fade
- reply message / resolution line expands into place → `spring-pop-entrance` (soft overshoot) or `anchored-layout-expand` for a true downward growth
- reaction button springs into an active pill with overshoot + count → `spring-pop-entrance` (the pop) + `press-release-spring` (activation flavor) + `counting-dynamic-scale` (the count, if it ticks)
- camera push-in + pan-down centering the tool call / the payoff line → `coordinate-target-zoom` (non-centered target: scale + counter-translate) or `viewport-change`
- slight pull-back then gentle end drift → `multi-phase-camera` (pull-back phase + continuous micro-drift; keep the drift near-imperceptible)
- static hold on the final stack → no rule needed

**camera modifier** (default is a STATIC frame — the theater is element-level; at most ONE real move per shot, chosen from):

- Trigger push-through: a rapid push-in through the opening workspace that clears to the bare canvas → `viewport-change` + `multi-phase-camera`, optional `motion-blur-streak`.
- Receipt zoom: one slow continuous zoom into the checklist card across the whole mutation run, letting the header drift off the top → `multi-phase-camera` (steady push).
- Payoff push-in (sub-shape B's defining move): static through the build, then ONE push-in + pan-down tightening onto the confirmation line, easing to a micro-drift end → `coordinate-target-zoom` / `viewport-change` + `multi-phase-camera` (drift).
- Everything else — swaps, cascades, check-offs, scrolls — happens on a locked frame (any "scroll" is the content translating inside its window, not the camera).

**doctrine note (idle-motion ban)**: the working-state motifs (spinner rotation, pulsing dots, pulsing "Thinking") brush against motion-doctrine's idle-motion ban — here they are DIEGETIC: the pulse _performs_ "the machine is working" and is the narrative content of Scene 2, not decorative breathing. Keep every loop finite, timeline-driven, and seek-safe (`sine-wave-loop` finite repeats, `svg-icon-enrichment` rotation), and kill it at the exact frame the state resolves — the corpus does this explicitly (the loading dots vanish the instant the payload card expands; the spinner swaps out with the loader lockup).

## Selected motion rule: coordinate-target-zoom

---
name: coordinate-target-zoom
description: Zoom into a specific non-centered element by combining scale with counter-translation — target ends at viewport center after the zoom completes.
metadata:
  tags: camera, zoom, scale, translate, target, off-center, focus
---

# Coordinate Target Zoom

A simple `scale > 1` on a wrapper pushes off-center content OFF the visible canvas. To zoom _into_ a specific non-centered element, apply scale AND an inverse translation in lockstep so the target lands at viewport center.

## How It Works

Two nested wrappers, separated concerns — never scale and translate on the SAME element (`translate * scale` ≠ `scale * translate` in CSS transform composition):

1. **Outer wrapper** applies `scale` (the zoom) around `transform-origin: 50% 50%`
2. **Inner wrapper** applies `translate(x, y)` (the counter-shift)

The counter-translate is the **negation** of the target's offset from viewport center:

```
T = -offset
```

Derivation: the inner translate moves the target to `offset + T` in pre-scale units; the outer scale S (around center) maps that to `S × (offset + T)`; landing at center means `S × (offset + T) = 0` → **`T = -offset`**. The formula does NOT depend on S — the translate is identical at 1.5×, 2×, or 3×. A common wrong intuition is `T = -offset × (S - 1)`: it coincidentally matches at S = 2 and is wrong at every other scale.

⚠️ **This is the NESTED-wrapper formula.** The single-wrapper camera in [viewport-change.md](viewport-change.md) puts `translate(x,y) scale(S)` on ONE element, where CSS applies scale first — there the counter-translate is **`T = -offset × S`**. The two formulas are not interchangeable; match the formula to the wrapper structure.

## Getting the offset

`T = -offset` is only as good as `offset`. The #1 way this pattern ships broken is hand-computing `offset` from a layout formula, getting the **sign** or magnitude wrong, and letting the zoom amplify a small error off-screen. **Default to measuring the target's real laid-out center; reserve the formula for symmetric rows.**

**Default — measure the actual center (works for ANY layout).** Immune to sign errors because it reads the rendered DOM, not a mental model:

```js
await document.fonts.ready; // metrics final; fallback fonts are 10–30px off → tens of px after a 3×+ zoom
const W = 1920,
  H = 1080;
const r = document.getElementById("target-card").getBoundingClientRect();
const TARGET_OFFSET_X = r.left + r.width / 2 - W / 2;
const TARGET_OFFSET_Y = r.top + r.height / 2 - H / 2;
```

Measure **once at setup** and bake — never per-frame in `onUpdate`. Because the measurement is async (`fonts.ready`), build and register the timeline inside the same `async` setup so the baked offset is ready before `window.__timelines[id]` is published.

**Shortcut — symmetric equal-width row ONLY:**

```js
const index_offset = targetIndex - (N - 1) / 2;
const TARGET_OFFSET_X = index_offset * (CARD_WIDTH + CARD_GAP);
```

⚠️ This assumes every sibling is the **same width**. The moment the row is asymmetric, it gives the wrong answer — often the wrong **sign**: the heavier side shifts the centered target the _opposite_ way you'd guess (e.g. `companion(220) + gap + wordmark + gap + chip(110)` puts the wordmark ~55px **right** of center, but "chip − companion" intuition says left). For anything but equal cards, **measure**.

**Headroom budget — cap the scale from the measured size.** A zoom multiplies any centering error; keep the target ≤ ~88% of the canvas at peak:

```js
const maxScale = Math.min((0.88 * W) / r.width, (0.88 * H) / r.height);
const ZOOM_SCALE = Math.min(DESIRED_SCALE, maxScale);
```

A target filling 97%+ of the frame reads as cut-off the instant its center is slightly off — and a hand-baked offset always is. (The perception gate flags this as `primary-offscreen`; `data-layout-allow-overflow` does **not** exempt it.)

## Recipe

```html
<div class="zoom-outer" id="zoom-outer">
  <div class="zoom-inner" id="zoom-inner">
    <div class="content">
      <div class="card">{other}</div>
      <div class="card target" id="target-card">{target}</div>
      <div class="card">{other}</div>
    </div>
  </div>
</div>
```

```css
.scene {
  overflow: hidden; /* REQUIRED — at zoom > 1 the scaled content leaks past the frame */
}
.zoom-outer {
  width: 100%;
  height: 100%;
  display: grid;
  place-items: center;
  transform-origin: 50% 50%; /* center scaling is what the counter-translate math assumes */
  will-change: transform;
}
.zoom-inner {
  display: grid;
  place-items: center;
  will-change: transform;
}
```

```js
// TARGET_OFFSET_X/Y and ZOOM_SCALE come from "Getting the offset" — measured
// at setup (after fonts.ready), baked. Counter-translation = -offset.
const counterX = -TARGET_OFFSET_X;
const counterY = -TARGET_OFFSET_Y;

// Scale and counter-translate MUST share position, duration, AND ease —
// otherwise the target visibly wanders mid-zoom.
tl.to("#zoom-outer", { scale: ZOOM_SCALE, duration: ZOOM_DUR, ease: "power3.inOut" }, ZOOM_AT);
tl.to(
  "#zoom-inner",
  { x: counterX, y: counterY, duration: ZOOM_DUR, ease: "power3.inOut" },
  ZOOM_AT,
);
```

## Variations

- **Zoom out (target → wide view)**: reverse the phases — start zoomed-in, then tween to `scale: 1` + `x: 0, y: 0`; the "reveal" beat is the panorama.
- **Multi-target zoom sequence**: chain zooms (target A → pause → target B → pull back); each segment needs its own counter-translation pair.

## Values

| token      | range                                   | notes                                                                                      |
| ---------- | --------------------------------------- | ------------------------------------------------------------------------------------------ |
| ZOOM_SCALE | 1.5× modest → 3× dominant → 5×+ extreme | cap via the headroom budget; raster media needs `sourceResolution ≥ rendered × ZOOM_SCALE` |
| ZOOM_DUR   | 1.0–2.0s                                | under 0.8s feels like a teleport, over 2.5s drags; both tweens share it                    |
| ZOOM_AT    | after the layout lands + 0.5–1.5s       | give the viewer time to scan the layout before the camera commits                          |
| DWELL      | ≥ 1.0s after the zoom settles           | 1.5–2s ideal — the viewer must be able to read the target (climax dwell)                   |

## Critical Constraints

- **Outer scales, inner translates** — never both transforms on one element; nested wrappers keep the math clean.
- **`transform-origin: 50% 50%` on the outer wrapper** — non-center origin breaks the counter-translate derivation.
- **`overflow: hidden` on the scene root** — zoomed content leaks past the frame otherwise.
- **Scale and counter-translate share duration + ease** at the same timeline position, or the target drifts mid-zoom.
- **Offset measured once at setup** (after `fonts.ready`), baked — never recomputed per-frame, never hand-derived for a non-symmetric layout (wrong sign → target shoved off-frame).
- **Scale within the headroom budget** — target ≤ ~88% of the canvas at peak, derived from the measured size.

## See also

[viewport-change.md](viewport-change.md) (single-wrapper form, `T = -offset × S`) · [multi-phase-camera.md](multi-phase-camera.md) (a zoom phase inside a phased camera) · [sine-wave-loop.md](sine-wave-loop.md) (idle breathing after the zoom settles) · [discrete-text-sequence.md](discrete-text-sequence.md) (text assembly in the target before the zoom).
