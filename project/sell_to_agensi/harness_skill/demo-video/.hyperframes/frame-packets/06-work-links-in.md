# Frame packet: 06-work-links-in

## Project inputs

- Project: D:\research\marketing\project\sell_to_agensi\harness_skill\demo-video
- Design tokens: D:\research\marketing\project\sell_to_agensi\harness_skill\demo-video\frame.md
- RULES_DIR: C:\Users\heris\.claude\skills\hyperframes-animation\rules

## Assigned storyboard block

## Frame 6 — Finished work links itself in

- scene: The real panel of a finished task shows its Done section, then the camera travels along the reference edge to the spec it implemented
- voiceover: "Finish a task — it links itself into the graph, — with what changed and how it was checked."
- duration: 8s
- transition_in: push-slide LEFT
- status: outline
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

## Selected blueprint: camera-journey

# camera-journey — Camera Journey

**intent**: The real viewport camera is the STORYTELLER — a multi-leg journey (dive in → a mid-journey beat fires → travel to the consequence / reposition → landing push, at rest) across ONE continuous world, where the travel itself carries the narrative. Two folded sub-shapes: **(A) action roundtrip** — the camera dives into a UI panel, a cursor/typed action fires, and the camera swoops/pans to another region where the consequence renders as element motion; **(B) cursorless flight** — pure cinematic 3D flight (motion blur, depth-of-field, tilt-to-flatten rotations) over static or self-animating content, no cursor anywhere.

**boundary**: This is NOT `cursor-ui-demo` — there the camera _chases_ the cursor (a servo following the actor); here the camera IS the actor, moving on its own narrative motivation, and in sub-shape A the cursor acts only at the leg hinge (in B it never appears). This is NOT `device-surface-showcase` — there one DEVICE/surface is hero and the camera merely presents it; here no single surface is hero — the journey traverses multiple regions/panels/depth planes and the traversal is the story. This is NOT `spatial-pan-stations` — there pre-placed stations on a flat canvas are visited by repeated pans of the same type; here the legs are heterogeneous (push-in, swoop, pull-back-rotate, whip, dive) and each leg is _motivated_ (by a fired action, or by the reveal it lands on).

**roles served**

- Benefits (from `camera-swoop-panel-action-roundtrip`): when the benefit IS a cause→effect round trip — "do this small thing here, get this big thing there" (comment → chart morphs; agent finding → verified commit; chat message → receipt + ledger). The camera physically connects the action to its payoff, so the viewer _travels_ the value chain instead of being told it.
- Key_Feature (from `cursorless-camera-flight`): when the feature should feel cinematic and inevitable — a payout form or a generated content-plan calendar explored by a flying camera (dives, whip sweeps, tilt-to-flatten, violent final push onto the CTA/hero card), the content acting by itself (a dropdown self-selects; keyword cards simply exist in depth) with no hand on the wheel.

**duration**: 5.6–11.1s (sub-shape A 5.6–9.0s: 001 5.6s · 066 8.6s · 004 9.0s; sub-shape B 6.3–11.1s: Outrank 6.3s · 094 11.1s)

**shot structure** (one oversized `[world]` — a `[UI canvas: design tool / GitHub + agent panels / phone + desktop ledger]` (A) or a `[3D-laid-out space: floating form card / calendar grid with standing keyword cards]` (B) — wrapped by a single virtual camera; content animates as elements _inside_ the world while the camera travels; every leg is a sequential tween on the same camera state)

- **Scene 0 (optional, 0.0–~1.8s) — static prologue.** Camera locked on a `[prologue beat: static promo card with a floating 3D product card / typed headline with an accent word / wide establishing shot of the app]`. A typewriter line may finish (`[headline]` types on, accent word in `[accent color]`). The prologue BREAKS by a hard cut or by the headline shrinking and slipping away as the first dive begins — the stillness exists to make the journey's launch land.

- **Scene 1 (~0.5–2.0s) — LEG 1: dive in.** The camera pushes in FAST and TIGHT onto `[the focal element]`:
  - _Sub-shape A_: a flat whole-viewport push onto `[an actionable element: comment box / agent panel / chat bubble]` where `[typed text]` finishes typing or `[response text]` streams in. The header/context leaves the frame — commitment, not a polite zoom.
  - _Sub-shape B_: the push lands at an ANGLE — a tilted 3D close-up of `[the form region / the calendar grid]`, foreground elements motion-blurred during the travel, neighbors soft under depth-of-field. A huge `[foreground prop: date number / field label]` may dominate the frame, blurred by speed.

- **Scene 2 (~1.5–6.0s) — LEG 2: the mid-journey beat (the hinge).** The camera holds, drifts, or pulls slowly while the content ACTS:
  - _Sub-shape A — the action fires_: a `[cursor]` clicks `[Send / Create PR]` (or a `[message]` sends implicitly) and the acted element CLEARS/vanishes. Optional theater before the click: a `[status spinner]` cycles `[status words]`, `[to-do items]` strike through, `[response text]` streams. The click is the hinge that _motivates_ the next leg.
  - _Sub-shape B — the content self-acts_: a `[dropdown]` expands by itself (pushing `[the field below]` down), shows a `[row hover highlight]` with no cursor, and collapses with the new value selected; OR the flight decelerates INTO FOCUS on `[one card]` — its `[metrics]` sharp, neighboring cards blurred.

- **Scene 3 (~4.0–8.0s) — LEG 3: travel to the consequence / reposition.**
  - _Sub-shape A_: the camera pulls back / swoops / pans to `[region B]` while the CONSEQUENCE builds as element motion — `[bars shrink into the baseline while a node-dotted line draws left→right / a verified commit row slides into the timeline + a reaction pill pops / a receipt card expands row-by-row from a skeleton]`. An optional SECOND leg extends the trip: `[pan up-right to a toolbar → a dropdown cascades open / match cut into an extreme close-up → a fast decelerating zoom-out reveals a ledger table]`.
  - _Sub-shape B_: a repositioning move — a slow pull-back that ROTATES the world flat and centered (3D → straight-on 2D), or a heavily motion-blurred WHIP SWEEP that resolves into a flat lateral pan across `[a month calendar / the full card]`. On the flat hold, quiet element beats may play: a thin `[focus outline]` fades in around one `[field]` and sweeps down to the next; the card keeps a near-imperceptible tilt/scale drift so the hold never dies.

- **Scene 4 (final ~1–2s) — LEG 4: landing.** The journey resolves on the payoff:
  - _Sub-shape A_: the camera comes to REST; the `[cursor]` hovers or drifts toward `[the payoff: an open Export menu item / the commit link / the View-transaction button]`; ends still, on the changed state — the world is visibly different from where the trip began.
  - _Sub-shape B_: a sudden VIOLENT push-in/dive (motion-blurred) onto `[the CTA button scaled huge in frame / the hero keyword card]`, ending holding tight — or holding MID-DIVE (the last frames are still traveling; the flat overview is explicitly not the final image).

**motion vocabulary**: whole-viewport camera push-in (fast/tight and slow/subtle); camera pull-back reframe; camera pan up/right/down; dive/swoop between stacked panels; fast decelerating zoom-out to rest; sudden violent push-in onto a button scaled huge; continuous 3D flight through a card grid; dive into an angled 3D close-up; slow pull-back that rotates/flattens the world to straight-on; heavily motion-blurred whip sweep; motion blur on camera travel; depth-of-field with blurred neighbors; decelerate-into-focus; hard cut / match cut into extreme close-up; near-imperceptible tilt/scale drift on holds; typed text finishing in an input; typewriter headline; headline shrinks and slips away as the camera dives; streaming AI response text; status-word spinner cycling labels; to-do strikethrough draw; cursor click; clicked element clears/vanishes; dropdown cascades open / self-expands and collapses with a row hover highlight (displacing the field below); bar-to-line chart morph (bars shrink into the baseline while a node-dotted line draws left→right, labels persist); commit row slide-in on a timeline; reaction pill appears; skeleton→content card build; receipt/label rows expand row-by-row; thin focus outline fades in and sweeps between fields; camera drift toward a button; 3D card subtle float; cursor hover at rest.

**rule mapping**

- the multi-leg camera itself — sequential push / pull-back / pan / dive phases on one wrapper, plus the micro-drift that keeps holds alive → `multi-phase-camera` (phase sequencing + drift) over `viewport-change` (the base virtual-camera primitive: single `.world` wrapper, one `cam {scale,x,y}` state — one source of truth for every leg)
- diving TIGHT onto an off-center element (comment box, chat bubble, Send button, one keyword card) → `coordinate-target-zoom` (scale + counter-translate; measure the target, don't hand-derive — a journey amplifies centering error on every leg)
- fast decelerating zoom-out from an extreme close-up to rest (066's ledger reveal) → `coordinate-target-zoom` zoom-out variation / `multi-phase-camera` (pull phase, hard `power4.out`)
- motion blur on camera travel (dive, whip sweep, violent final push) → `motion-blur-streak` (Camera-travel carve-out — the blur envelope rides the `.world` wrapper during a leg: the world never leaves frame, the blur peaks at peak velocity and resolves sharp at each landing)
- depth-of-field on neighbors while one card is in focus; decelerate-into-focus → `depth-of-field-blur` (focal pull + blur-the-cluster-while-pushing-in are explicitly in scope; run the DoF tween at the same position as the camera leg)
- the 3D flight itself (sub-shape B's core) — a perspective camera traveling with `rotateX/rotateY/translateZ` through a 3D-laid-out world: the dive into an angled calendar grid, the tilt-to-flatten pull-back (angled 3D → straight-on 2D), the continuous flight between standing cards → `3d-camera-flight` (perspective wrapper + preserve-3d; the 2D camera rules keep owning any flat legs)
- whip sweep → composition: `nudge-curve` (burst-dominant tuning of the slow-fast-slow slide, applied to the world) + `motion-blur-streak` (camera-travel carve-out) on the same window
- typed text finishing in an input; typewriter headline; streaming AI response text; status spinner cycling `[status words]`; skeleton→content state swap → `discrete-text-sequence` (+ `gsap-effects` typewriter; `context-sensitive-cursor` for the input caret)
- which content appears per leg / receipt rows and findings arriving on script windows → `dynamic-content-sequencing`
- cursor click on `[Send / Create PR]` (sub-shape A's hinge) → `cursor-click-ripple` + `press-release-spring` (or `physics-press-reaction` for a weightier press)
- clicked element clears/vanishes; panel state A → B on the return leg → `scale-swap-transition` / `card-morph-anchor`
- to-do strikethrough draw; row hover highlight → `css-marker-patterns` (strike-through) · `asr-keyword-glow` (accent glow on the hovered/selected row)
- bar-to-line chart morph → composite, decomposes cleanly: `stat-bars-and-fills` (bars `scaleY` → baseline) + `svg-path-draw` (node-dotted line draws left→right) at the same timeline position — no single rule names the coordinated chart-type morph, but no new rule needed
- commit row slide-in; reaction pill appears; receipt rows expand row-by-row → `spring-pop-entrance` (single arrivals) / `waterfall-entry` (the row-by-row cascade)
- dropdown self-expands, displacing the field below (094) → `anchored-layout-expand` (the masked edge-anchored expansion of the dropdown body — never tween `height`) + `reactive-displacement` (the expansion tween drives the sibling's displacement)
- focus-ring travel between fields (094: a thin outline fades in on `From`, then sweeps down onto `Amount`) → `ai-tracking-box` restyled as a plain outline (offsets baked at setup; size morphed via scale, never width/height)
- 3D card subtle float; near-imperceptible tilt/scale drift on holds → `sine-wave-loop` (+ `multi-phase-camera`'s drift for the camera-side micro-motion; the _tilt_ component of the drift belongs to `3d-camera-flight`)
- camera drift toward a button; slow subtle zoom-ins riding a hold → `multi-phase-camera` (steady-push mode, tiny spread)

**camera grammar** (the defining layer — this blueprint IS its camera): every leg is a tween on ONE camera state (`viewport-change`'s single `.world` wrapper / `cam` object), sequenced by `multi-phase-camera`, aimed by `coordinate-target-zoom`. Legs must be _motivated_: sub-shape A moves because an action fired (click → swoop to the consequence); sub-shape B moves because the next reveal demands it (dive → focus → reposition → final dive). Vary the leg verbs — a journey of four identical pushes reads as a slideshow. Ease law: hard `out`-family on dives and landings (`power4.out` — violent arrival, sharp settle), `power2.inOut` on repositioning legs; spring/back easing on a camera feels wrong (per `multi-phase-camera`). Sub-shape B layers `3d-camera-flight`'s perspective wrapper under the same single-state discipline.

**Seek-safety (non-negotiable for this much camera):** the entire journey — every leg, every blur envelope, every DoF pull — lives on the ONE paused GSAP timeline, so any frame seek reproduces the exact mid-leg camera pose. One camera state object, transform composed in a single writer (`applyCamera()`), no CSS `transition` anywhere near the wrapper, blur via proxy-tweened attributes / `--dof` vars (both seek-safe), and ending mid-dive is fine — a seek to the last frame just lands mid-tween. Per-leg targets are measured ONCE at setup (after `fonts.ready`) and baked; never `getBoundingClientRect` in `onUpdate`.

**Overflow (required for a clean `check`):** a traveling camera deliberately moves world content past the frame edges on every leg. Keep `overflow: hidden` on the scene root AND mark the moving `.world` wrapper with `data-layout-allow-overflow` — otherwise `check` reports `text_box_overflow` / `container_overflow` for every panel the journey leaves behind (see the same note on `device-surface-showcase`).

## Selected motion rule: svg-path-draw

---
name: svg-path-draw
description: Animate SVG paths drawing progressively using stroke-dasharray and stroke-dashoffset.
metadata:
  tags: svg, stroke, draw, path, reveal, icon, vector
---

# SVG Path Draw

## Contents

- [Recipe](#recipe)
- [Variations](#variations)
- [Values](#values)
- [Critical Constraints](#critical-constraints)
- [See also](#see-also)

Reveals an SVG shape by animating its stroke as if a pen were tracing it. Two stroke properties together: **`stroke-dasharray = <pathLength>`** makes the entire path one dash; **`stroke-dashoffset`** starts at the path length (dash shifted fully out of view → invisible) and tweens to `0` (fully drawn). The length comes from the DOM API `path.getTotalLength()` — measured, never guessed.

Works on anything with a stroke: `<path>`, `<circle>`, `<rect>`, `<line>`, `<polyline>`, `<polygon>`, `<ellipse>`.

## Recipe

```html
<!-- inside a standard scene clip -->
<svg class="logo-mark" viewBox="0 0 200 200" xmlns="http://www.w3.org/2000/svg">
  <path id="bar-left" d="M 60 40 L 60 160" />
  <path id="bar-right" d="M 140 40 L 140 160" />
  <path id="bar-mid" d="M 60 100 L 140 100" />
</svg>
```

```css
.logo-mark path {
  fill: none; /* outline-only draw — a fill would appear immediately and ruin the reveal */
  stroke: {accentColor};
  stroke-width: 12;
  stroke-linecap: round; /* softer endpoints */
  stroke-linejoin: round;
}
```

```js
// Setup: measure each path and set its dash pattern. Real measured geometry, not a magic number.
document.querySelectorAll(".logo-mark path").forEach((p) => {
  const len = p.getTotalLength();
  p.style.strokeDasharray = `${len}`;
  p.style.strokeDashoffset = `${len}`;
});

// Stagger draws so the eye reads continuous motion — each segment starts at
// ~70-80% of the previous segment's duration, before it finishes.
tl.to(
  "#bar-left",
  { strokeDashoffset: 0, duration: SEGMENT_DRAW_DUR, ease: "power2.out" },
  SEG_1_START,
);
tl.to(
  "#bar-right",
  { strokeDashoffset: 0, duration: SEGMENT_DRAW_DUR, ease: "power2.out" },
  SEG_2_START,
);
tl.to(
  "#bar-mid",
  { strokeDashoffset: 0, duration: FINAL_SEGMENT_DUR, ease: "power2.out" },
  SEG_3_START,
);

// Companion wordmark fades in only after the last stroke settles.
tl.to(
  ".brand-line",
  { opacity: 1, duration: BRAND_FADE_DUR, ease: "power1.out" },
  BRAND_FADE_START,
);
```

## Variations

- **Ring starting at 12 o'clock** — `<circle>` / `<rect>` strokes start at 3 o'clock by default; rotate the element `-90deg` so a progress ring draws from the top:

```html
<circle
  cx="100"
  cy="100"
  r="60"
  id="ring"
  style="transform-origin: 100px 100px; transform: rotate(-90deg)"
/>
```

- **Linear (constant-speed) draw** — `ease: "none"` for a steady-rate "real pen" trace.
- **Draw then fill** — for filled shapes, tween `fillOpacity: 0 → 1` AFTER the stroke completes (requires `fill-opacity: 0` initially and a real `fill` in CSS):

```js
tl.to(
  "#path",
  { strokeDashoffset: 0, duration: SEGMENT_DRAW_DUR, ease: "power2.out" },
  SEG_1_START,
);
tl.to(
  "#path",
  { fillOpacity: 1, duration: FILL_FADE_DUR, ease: "power1.out" },
  SEG_1_START + SEGMENT_DRAW_DUR,
);
```

## Values

| token             | range                                   | notes                                                                                              |
| ----------------- | --------------------------------------- | -------------------------------------------------------------------------------------------------- |
| SEGMENT_DRAW_DUR  | 0.3–0.8s                                | fast snap vs deliberate pen trace; >~1s feels sluggish for a logo reveal                           |
| FINAL_SEGMENT_DUR | 60–80% of SEGMENT_DRAW_DUR              | proportional to segment length — a short connector at full duration reads slower than its siblings |
| SEG_N_START       | previous start + 70–80% of its duration | reads as continuous motion, not N isolated animations                                              |
| SEG_1_START       | 0–0.4s                                  | a small ~0.2s lead-in lets the viewer settle before motion                                         |
| BRAND_FADE_START  | ≥ last stroke end (+ ~0.2s beat)        | earlier and the wordmark competes with the draw                                                    |
| BRAND_FADE_DUR    | 0.3–0.8s                                | snap (urgent) vs glide (premium)                                                                   |

Ease families are discrete choices: **stroke draws** use `power2.out` (a hand lifting at end of stroke) or `none` for constant speed — never `back.out` / `elastic.out` (pens don't bounce). **Fades** use `power1.out`.

## Critical Constraints

- **`fill: none`** for outline-only draws — otherwise the fill appears immediately.
- **Dasharray/dashoffset = the measured `getTotalLength()`**, set at setup; requires the SVG in the DOM (inline SVG is fine; a loaded `<image>` SVG is not).
- **Complex paths**: if `getTotalLength()` looks wrong, overestimate slightly (`len * 1.05`) — too large is invisible at animation start; too small clips the end.
- **Stagger multi-path draws at ~70–80%** of the previous segment's duration.
- **A drawn line must land on something.** When the path is a connector (rail, beam, underline, callout) rather than a shape, both endpoints must sit on real elements and the draw must do a job — reveal, route, validate, or emphasize. A stroke that only decorates empty space reads as filler; attach it or cut it.

## See also

`svg-icon-enrichment` (internal parts animate after the outline draws) · `counting-dynamic-scale` (stroke draws an icon while a number counts up) · `hacker-flip-3d` (logo draws, wordmark decodes beneath).

## Selected motion rule: viewport-change

---
name: viewport-change
description: Virtual camera — simulate zoom / pan / focus-lock by transforming a wrapper around all scene content. Camera moves right → world translates left.
metadata:
  tags: viewport, camera, zoom, pan, focus-lock, virtual-camera
---

# Viewport Change (Virtual Camera)

## Contents

- [How It Works](#how-it-works)
- [Recipe](#recipe)
- [Scale Value Guide](#scale-value-guide)
- [Variations](#variations)
- [Values](#values)
- [Critical Constraints](#critical-constraints)
- [See also](#see-also)

Simulates camera effects (zoom / pan / focus-lock on a moving element) by transforming a wrapper around ALL scene content. The "world" moves opposite to the perceived camera. Distinct from [multi-phase-camera](multi-phase-camera.md) (2-3 discrete phases + drift) — viewport-change is a single continuous zoom/pan, often used for focus-lock following a moving element.

## How It Works

Camera intent → world transform. Camera **pans right** → world `translateX(-distance)`; camera **zooms in** → world `scale(>1)`; camera **follows element X** → world `translateX(viewportCenter - elementWorldX)` per-frame. Get the sign right or everything moves the wrong way. The single `.world` wrapper holds the camera transform; elements inside are positioned in world space, unchanged.

**Single-element composite transform (this rule's form).** Both scale and translate live on ONE wrapper as `translate(x, y) scale(S)`. CSS applies scale FIRST, then translate (right-to-left matrix composition), so a point at world offset `(ox, oy)` lands on screen at `(S × ox + x, S × oy + y)`. To map the target to viewport center, solve `S × offset + T = 0`:

```
T = -offset × S
```

This is **different from [coordinate-target-zoom](coordinate-target-zoom.md)**, which uses two nested wrappers (outer scales, inner translates) and derives `T = -offset` (independent of S). Mixing up the two forms drifts the target off-center as scale changes. Use this single-wrapper form when you want one source of truth for camera state (`cam.scale`, `cam.x`, `cam.y`) written via `onUpdate`; use nested wrappers when scale and translate can tween independently with shared ease.

## Recipe

```html
<div class="world" id="world">
  <div class="content">
    <div class="hero">{Brand}</div>
    <div class="tagline">{tagline}</div>
    <div class="cta" id="cta">{ctaUrl}</div>
  </div>
</div>
```

```css
.scene {
  overflow: hidden; /* REQUIRED — any non-1.0 scale reveals edges or pushes content off-frame */
  background: {bgGradient}; /* on .scene, NOT .world — a world-borne background warps with the camera */
}
.world {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  transform-origin: 50% 50%; /* centered scaling is what the math assumes */
  will-change: transform;
}
```

```js
const world = document.getElementById("world");

// Camera state — single source of truth. The world transform is composed from
// this object in ONE place so the transform string order is stable.
const cam = { scale: 1, x: 0, y: 0 };
function applyCamera() {
  world.style.transform = `translate(${cam.x}px, ${cam.y}px) scale(${cam.scale})`;
}
applyCamera(); // seed frame 0

// Zoom in on the CTA: single-element composite transform → T = -offset × S.
// TARGET_OFFSET_Y is the target's measured offset from viewport center at
// neutral camera (sign matters — positive = below center).
const counterY = -TARGET_OFFSET_Y * TARGET_SCALE;

tl.to(
  cam,
  {
    scale: TARGET_SCALE,
    y: counterY,
    duration: ZOOM_DUR,
    ease: "power3.inOut",
    onUpdate: applyCamera,
  },
  ZOOM_START,
);
```

## Scale Value Guide

| Effect      | Scale       | Feel                                |
| ----------- | ----------- | ----------------------------------- |
| Subtle      | 1.02 - 1.05 | Barely perceptible — "professional" |
| Medium      | 1.05 - 1.15 | "Ta-da" emphasis                    |
| Noticeable  | 1.15 - 1.30 | Focus on region                     |
| Dramatic    | 1.5 - 2.5   | Element fills screen                |
| Full-screen | 3.0+        | Element covers viewport             |

Perception: < 5% scale change is imperceptible; 10-15% is comfortable emphasis; > 30% is cinematic/dramatic. For a natural product feel, prefer 1.05-1.15× over 2-3s; save big > 1.3× zooms for dramatic narrative moments.

### Extreme range — 4–12× outward (workspace reveal)

The same single-cam math runs far past the table: a zoom-out workspace reveal opens punched-in at **4–12×** on one detail (a single cell, message, or button) and pulls out to the full workspace in one continuous move. The mechanics don't change — one `cam` object, `T = -offset × S`, one `applyCamera()` writer — only the authoring direction does:

- **Build the workspace at its final (1×) layout and OPEN scaled-in** (`cam.scale = 8`, counter-translate aiming the opening detail; state it in a `fromTo` / seed via `applyCamera()` so a seek to t=0 lands punched-in). The wide landing frame is then everything at native design size — text crisp, raster assets at source resolution.
- **Never the inverse** — authoring the close-up at 1× and scaling the world down to 0.08–0.25 for the wide frame drops every label below legible pixel size and softens raster media; the reveal lands on mush.
- **Measure the opening target** — at S = 8, a 1 px error in the baked offset is 8 px on screen at the opening pose. Take the offset from the target's real laid-out center (`getBoundingClientRect` after `fonts.ready`, once at setup — the measuring doctrine in [coordinate-target-zoom.md](coordinate-target-zoom.md)), never from a layout formula.
- **The opening detail must survive ×S** — it renders at `S ×` its design size on the first frames (vector/DOM text is safe; raster needs `sourceResolution ≥ rendered × S`).

## Variations

- **Focus-lock (camera follows a moving cursor/character)** — keep the element at a fixed screen X by computing the world offset per-frame inside the driver's `onUpdate`:

```js
const focusEl = document.querySelector(".moving-cursor");
const targetScreenX = VIEWPORT_WIDTH * FOCUS_SCREEN_X_FRAC; // 0.4–0.7; 0.5 = dead center
const focusUpdate = { p: 0 };
tl.to(
  focusUpdate,
  {
    p: 1,
    duration: FOLLOW_DUR, // matches how long the focused element is in motion
    ease: "power2.inOut",
    onUpdate: () => {
      const rect = focusEl.getBoundingClientRect();
      cam.x = targetScreenX - (rect.left + rect.width / 2);
      applyCamera();
    },
  },
  FOLLOW_START,
);
```

- **Composite scale (multi-phase)** — two proxy tweens multiplied through one writer: `cam.scale = scaleUp.v * scaleDown.v; applyCamera()`. Combine a slow push-in (~1.15) with a brief release (~0.9) for a breath/punch shape.
- **Camera mode transition (centered → follow)** — crossfade two camera modes via a 0→1 weight tween; intermediate frames interpolate between the modes' offsets.

## Values

| token           | range                                | notes                                                                                       |
| --------------- | ------------------------------------ | ------------------------------------------------------------------------------------------- |
| TARGET_OFFSET_Y | measured, not a free parameter       | target's offset from viewport center at neutral camera; measure via `getBoundingClientRect` |
| TARGET_SCALE    | 1.3× modest → 1.6–2.0× typical → 3×+ | raster media needs `sourceResolution ≥ rendered × TARGET_SCALE`                             |
| ZOOM_START      | content landed + ~0.5s scan time     | let the viewer read before the camera moves                                                 |
| ZOOM_DUR        | 1.0–2.0s                             | under 0.8s teleports, over 2.5s drags                                                       |
| DWELL           | ≥ 1.0s after the zoom settles        | the viewer must be able to read the focal point (climax dwell)                              |
| VIEWPORT_WIDTH  | = the root's `data-width`            | real value, not abstract                                                                    |

## Critical Constraints

- **One `.world` wrapper carries the whole camera** — every scene element lives inside it; a second transformed wrapper is a second camera.
- **Single source of truth via the `cam` object + `applyCamera()`** — when scale and translate both change, write them in ONE place; never split them across tweens that touch `world.style.transform` directly (the transform string composition order becomes unpredictable).
- **Single-wrapper counter-translate is `T = -offset × S`** — don't import the nested-wrapper `T = -offset` formula.
- **`overflow: hidden` on `.scene`**; **`transform-origin: 50% 50%` on `.world`**; **background on `.scene`, never on `.world`**.

## See also

[coordinate-target-zoom.md](coordinate-target-zoom.md) (nested-wrapper alternative, `T = -offset`) · [multi-phase-camera.md](multi-phase-camera.md) (viewport-change inside one phase) · [sine-wave-loop.md](sine-wave-loop.md) (idle micro-drift after the viewport settles).
