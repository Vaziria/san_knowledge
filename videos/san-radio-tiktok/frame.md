---
version: alpha
name: Broadside — Frame (video / frame layer)
description: >
  Video-first companion to Broadside's design.md. The unit is the frame (1920×1080). Atoms are
  identical and sacred — the two-register surface system (dark ink-black / fire-orange), massive
  DejaVu Sans Mono in lowercase weight 900 treated as graphic primitive, IBM Plex Mono chrome (uppercase,
  0.14em), the single fire-orange accent, the flat plane, and 1px hairline dividers. Composition +
  frame scale rewritten for the frame. Motion out of scope.
unit: the frame — 1920×1080 primary; 9:16 and 1:1 documented
principle: atoms are sacred · composition is free · numbers come from the script

colors:
  ink-black: "#0d0b1e"
  ink-black-alt: "#100e24"
  fire-orange: "#ff7ac6"
  cream: "#e6e6f0"
  cream-muted: "#8b8fb5"
  cream-hint: "#5a5b80"
  border-dark: "rgba(160,120,255,0.55)"
  # — the stream's terminal syntax colours (content, not accents; from san_radio's terminal.html) —
  term-host: "#ff7ac6"
  term-path: "#5ccfe6"
  term-arrow: "#9ece6a"
  term-output: "#e0af68"
  term-command: "#e6e6f0"
  term-muted: "#8b8fb5"
  window-fill: "rgba(16,14,36,0.80)"
  window-glow: "0 0 24px rgba(140,90,255,0.25), 0 20px 60px rgba(0,0,0,0.6)"
  rule: "rgba(160,120,255,0.28)"
  ink-on-orange-muted: "rgba(17,17,17,0.75)"
  ink-on-orange-hint: "rgba(17,17,17,0.55)"
  ink-on-orange-faint: "rgba(17,17,17,0.40)"
  ink-on-orange-border: "rgba(17,17,17,0.20)"

typography:
  # — reading ramp —
  body:    { fontFamily: "DejaVu Sans Mono", cqw: 1.2, weight: 400, lineHeight: 1.6 }
  lead:    { fontFamily: "DejaVu Sans Mono", cqw: 1.6, weight: 400, lineHeight: 1.5 }
  caption: { fontFamily: "DejaVu Sans Mono", cqw: 0.9, weight: 400, lineHeight: 1.5 }
  label:   { fontFamily: "DejaVu Sans Mono", cqw: 0.72, weight: 500, tracking: "0.14em", upper: true }
  # — display / hero ramp (DejaVu Sans Mono, lowercase, negative tracking) —
  h3:      { fontFamily: "DejaVu Sans Mono", cqw: 2.8, weight: 600, lineHeight: 1.2, lower: true }
  quote-text:{ fontFamily: "DejaVu Sans Mono", cqw: 3.8, weight: 700, lineHeight: 1.15, tracking: "-0.02em", lower: true }
  h2:      { fontFamily: "DejaVu Sans Mono", cqw: 4.5, weight: 700, lineHeight: 1.1, tracking: "-0.02em", lower: true }
  stat-value:{ fontFamily: "DejaVu Sans Mono", cqw: 5.5, weight: 900, lineHeight: 1.0, tracking: "-0.04em" }
  h1:      { fontFamily: "DejaVu Sans Mono", cqw: 7.5, weight: 800, lineHeight: 0.9, tracking: "-0.03em", lower: true }
  fadelist-item:{ fontFamily: "DejaVu Sans Mono", cqw: 7.5, weight: 900, lineHeight: 1.0, tracking: "-0.03em", lower: true }
  quote-mark:{ fontFamily: "DejaVu Sans Mono", cqw: 10.0, weight: 900, lineHeight: 0.6 }
  fadelist-title:{ fontFamily: "DejaVu Sans Mono", cqw: 10.5, weight: 900, lineHeight: 0.9, tracking: "-0.04em", lower: true }
  display: { fontFamily: "DejaVu Sans Mono", cqw: 13.0, weight: 900, lineHeight: 0.88, tracking: "-0.04em", lower: true }

spacing:
  pad-x: "5.5cqw"
  pad-y: "5.5cqw"
  gap-lg: "3.5cqw"
  gap-md: "2cqw"
  gap-sm: "1cqw"

components:
  registers:
    dark: "ground {colors.ink-black}, text {colors.cream}, accent {colors.fire-orange}"
    orange: "ground {colors.fire-orange}, text {colors.ink-black}"
    description: "Two surfaces only — no cream/paper register. One register per frame."
  slide-chrome:
    rule: "1px solid {colors.border-dark} (dark) / 20% ink (orange)"
    placement: "top + bottom bars (label left, number right)"
    description: "SUPPRESSED on cover/chapter/statement/quote/end — declarative frames let type fill the field."
  kicker:
    typography: "{typography.label}"
    color: "{colors.fire-orange} (dark) / 55% ink (orange)"
    description: "Uppercase mono eyebrow."
  rule:
    backgroundColor: "{colors.fire-orange} (dark) / {colors.ink-black} (orange)"
    size: "36×2px"
    description: "Stub accent bar — the system's only ornament."
  broadside-num:
    typography: "{typography.label}"
    placement: "top-left of orange cover/chapter, low opacity"
    description: "Mono catalogue numeral."
  stat-card:
    borderTop: "1px solid {colors.border-dark}"
    typography: "{typography.stat-value} (orange on dark / ink on orange) + {typography.body} + {typography.label}"
    description: "Top-border-only block, no other borders."
  bullet:
    marker: "orange `/` mono via ::before"
    typography: "{typography.lead}"
    description: "Capped at THREE items."
  bar-track:
    borderLeft: "1px solid {colors.border-dark}"
    bars: "{colors.cream-hint}, one .accent {colors.fire-orange}"
    typography: "{typography.label} axis"
    description: "Vertical bar chart, left axis only."
  compare-panel:
    layout: "two equal panels split by a 1px vertical rule"
    payoff: "right panel may fill {colors.fire-orange}"
    description: "Before/after."
  fadelist:
    typography: "{typography.fadelist-item} ×3 at opacity 1.0/0.5/0.22 + {typography.fadelist-title}"
    description: "Three stacked words + one oversized title opposite."
---

> **Stream adaptation (san-radio-tiktok).** This video reproduces the 24/7 stream's own screen
> (golang/packages/san_radio/background/terminal.html), re-laid for 9:16. Everything is DejaVu Sans
> Mono (capture/assets/DejaVuSansMono.ttf). The window is translucent `window-fill` with a 1.5px
> `border-dark` purple border, 14px radius and `window-glow` — the stream's look, overriding
> Broadside's 0-radius/no-shadow rule for this video only. Terminal text keeps the stream's syntax
> colours (`term-*`); the one emphasis accent is pink `fire-orange` (#ff7ac6). The ground is the
> night scene (capture/assets/night-scene.svg) over `ink-black`.

# Broadside — Frame (video / frame layer)

## Brand adaptation (READ FIRST — the frontmatter is the source of truth)

This is the **broadside** preset remixed onto the captured brand. The YAML frontmatter above (colors · typography · components) is **normative and already correct — use it verbatim.** The prose below is the ORIGINAL preset's intent; read it THROUGH the frontmatter:

- **Fonts** — already set to **DejaVu Sans Mono** (display) / **DejaVu Sans Mono** (body); ignore any preset font name lingering in prose.
- **Colors** — use the frontmatter hex; preset color NAMES in prose (e.g. "cobalt", "cream") mean the remapped brand values.


## Overview

Broadside at frame scale is a **protest-poster system where type is so large it stops reading as
text and becomes graphic primitive.** DejaVu Sans Mono `display` at 13cqw puts a single lowercase word
nearly across the frame. The system runs in **two registers**: a dark ink-black ground with cream
text for documentation, and a fire-orange ground with dark ink for declaration. Fire-orange is the
_only_ color — accent on dark, environment on orange. The plane is flat; hierarchy is weight, size,
and 1px hairlines.

**DejaVu Sans Mono** carries every text role from display to body — expressive range from weight (400–900)
and size, not face contrast. **IBM Plex Mono** is chrome only (numbers, kickers, tags, axis labels,
the `/` bullet marker), always uppercase and tracked. Display is **lowercase** — the system's most
distinctive single decision, a deliberate inversion of the brutalist norm.

**Key characteristics at frame scale:**

- **Two registers** — dark (cream text) / orange (ink text). No cream/paper register.
- **Massive lowercase DejaVu Sans Mono 900**, negative-tracked, as graphic primitive (display 13cqw).
- **Fire-orange is the only color** — accent on dark, full environment on orange.
- **IBM Plex Mono chrome** — uppercase, 0.14em; the `/` bullet marker; mono catalogue numbers.
- **Flat plane** — no shadow, no radius (save nav dots), no gradient; 1px hairlines carry structure.
- **Low density** — one statement per frame, bullets capped at three, chrome suppressed on declarative frames.

## The Frame

### Frame Craft Bar

Three eyeball tests gate every frame before any structural check:

- **Squint** — exactly **one display moment dominates** at 3–6× everything else; nothing competes.
- **Silence** — declarative frames read **45–55% empty**; the **stat grid is the one dense exception**.
- **Restraint** — **one register per frame**; **fire-orange is the only color** (accent on dark, environment on orange); one display moment; bullets capped at three.
- **Reference** — aim at **broadside printing / a SPACE10 report / a Wim Crouwel grid with one loud color**; failure looks like a **multi-accent corporate slide deck**.

- **Primary:** 1920×1080 (16:9). Type authored in **`cqw`** (`px ÷ 1920 × 100 = cqw`; or carry the source's `vw` 1:1).
- **Vertical:** 1080×1920 (9:16). **Square:** 1080×1080 (1:1).
- **Safe area:** `pad-x`/`pad-y` 5.5cqw — deliberately tight so the massive type crowds the frame edge.

**The container law (load-bearing).** Every frame ground sets `container-type: size`; ALL
frame-relative units are `cqw`/`cqh` against it — never `vw` (a `vw`-sized frame inflates when not
full-screen). 1px hairlines stay 1px.

## Colors

Tokens identical to the source, in two registers. **Dark:** `{colors.ink-black}` ground,
`{colors.cream}` text, `{colors.fire-orange}` accent (kickers, accent stat, bullet `/`, lead bar,
quote mark, rule stub). **Orange:** `{colors.fire-orange}` ground, `{colors.ink-black}` headlines +
body, with the dark-ink overlays (75/55/40/20%) as the muted tones. Choose one register per frame
and commit. **No second accent color** — on orange, emphasis is weight/opacity on the ink, never a
new hue. Cream text on orange does not exist (ink-on-fire is absolute).

## Typography

Two ramps. The **reading ramp** (DejaVu Sans Mono body 1.2cqw, mono label 0.72cqw) carries copy + chrome; the
**display ramp** (DejaVu Sans Mono `h2` 4.5cqw → `display` 13cqw, weight 700–900) carries every statement.

- **Legibility floor:** any load-bearing line ≥ **1.4cqw**; mono labels are chrome only.
- **Fit-to-measure:** size the headline to its length. Cap the block at **≤ 78cqw**; ≤2 words → `display`; 3–4 → `h1`; 5+ → `h2`. Broadside packs only ONE display moment per frame.
- **DejaVu Sans Mono display is lowercase, weight 700–900, negative-tracked** (−0.04em largest, −0.02em h2). **Mono chrome is uppercase, 0.1em+.** No italic, no underline, no uppercase display.

## Depth & Surface

Flat plane, the only technique. Hierarchy from:

- **Weight + size contrast** — the dominant signal (900 lowercase display).
- **1px hairlines** — chrome bars, stat-card top, compare divider, bar-track left, chart baseline.
- **Color shift** — orange on ink, ink on cream, cream-muted on cream.
- **Negative space** — generous, intentional empty regions.

**Ceiling:** no box-shadow, no elevation, no rounded surface (save nav dots), no gradient ground.

## Shapes

- **0 radius everywhere** except nav dots (50%). Cards, panels, tags, stat blocks, bars — sharp rectangles.

## Components

- **registers** — the two-surface system. **slide-chrome** — optional hairline bars, suppressed on declarative frames.
- **kicker** (mono eyebrow) / **rule** (36×2 stub) / **broadside-num** (catalogue mark) — the chrome ornament set.
- **stat-card** (top-border only) / **bullet** (orange `/`, max 3) / **bar-track** (one accent bar) / **compare-panel** (orange payoff) / **fadelist** (1.0/0.5/0.22 stack).

## Frame Treatments

> Recipe: ground · register · composes · focal · chrome · accent · silence · Fixed/Free · density.
> One statement per frame; chrome suppressed on declarative frames.

### 1 · Cover (identity · move: massive type · ORANGE register · left)

**Ground** fire-orange. **Composes** broadside-num, rule, kicker, display, lead. **Focal** a 1–2 word
DejaVu Sans Mono `display` (13cqw) lowercase in ink, left-anchored, over a small ink rule stub + mono kicker; a
DejaVu Sans Mono lead line beneath in 75% ink. **Chrome** mono catalogue number top-left, mono meta top-right
(no chrome bars). **Accent** the ink itself is the pop on orange. **Silence** ~45%. **Fixed** ink-on-fire,
lowercase 900, flat. **Free** the word, kicker, lead. **Density** low.

### 2 · Statement (declarative · move: type IS composition · DARK register · left)

**Ground** ink-black. **Composes** kicker, display. **Focal** a 2–4 word DejaVu Sans Mono `display`/`h1`
lowercase in cream, with ONE clause inked `{colors.fire-orange}`. **Chrome** mono kicker; no bars.
**Accent** the orange clause. **Silence** ~55%. **Fixed** lowercase 900, one orange clause, flat.
**Free** the statement, which clause is orange. **Density** low.

### 3 · Stat Grid (data · move: top-border cards · DARK · the dense frame)

**Ground** ink-black, chrome bars present. **Composes** slide-chrome, kicker, 3× stat-card. **Focal** a
row of three top-border-only stat-cards — big DejaVu Sans Mono-900 numeral in `{colors.fire-orange}`, DejaVu Sans Mono
label, mono note. **Chrome** top + bottom hairline bars (label + number). **Accent** the orange
numerals. **Silence** moderate — the density exception. **Fixed** top-border-only cards, orange
numerals, 1px hairlines. **Free** figures (from script), labels. **Density** dense-exception.

### 4 · Fadelist (narrative · move: opacity stack · DARK)

**Ground** ink-black. **Composes** fadelist (3 stacked DejaVu Sans Mono-900 words at 1.0/0.5/0.22), fadelist-title.
**Focal** the three-stage word stack opposite an oversized display title in `{colors.fire-orange}`
(before/during/after). **Accent** the orange title. **Silence** moderate. **Fixed** the opacity
ladder, lowercase 900. **Free** the three words, the title. **Density** low-moderate.

### 5 · Pull Quote (quote · move: oversized mark · DARK · left)

**Ground** ink-black, chrome suppressed. **Composes** quote-mark, quote-text, attribution. **Focal** a
DejaVu Sans Mono `quote-text` (700, lowercase) at ≤78cqw under an oversized fire-orange `quote-mark` (10cqw,
line-height 0.6). **Chrome** mono attribution (name + role). **Accent** the orange quote mark. **Silence**
~50%. **Fixed** orange mark, lowercase quote. **Free** quote, attribution. **Density** low.

### 6 · Compare (argument · move: split + orange payoff · DARK→ORANGE)

**Ground** ink-black left panel + fire-orange right (payoff) panel, 1px divider. **Composes**
compare-panel pair, kicker, h3. **Focal** two panels — left documents (cream on dark), right declares
(ink on orange). **Chrome** mono panel labels. **Accent** the orange payoff panel. **Silence** moderate.
**Fixed** ink-on-fire right panel, 1px divider, flat. **Free** the before/after content. **Density** standard.

## Composition Rules

### Do

- Set every DejaVu Sans Mono display in **lowercase weight 900**, negative-tracked — the system's signature.
- Use **fire-orange as full environment** on declarative frames, the **lone accent** on dark.
- Keep chrome in **IBM Plex Mono uppercase, 0.14em**; use the `/` mono bullet marker.
- **Cap bullets at three; one statement per frame**; build hierarchy from weight, size, 1px hairlines.
- Suppress chrome bars on cover/chapter/statement/quote/end; let type fill the field.
- Lean left on most frames; the type IS the composition.

### Don't

- Never uppercase DejaVu Sans Mono display; never add a second accent color.
- Never put cream text on orange (ink-on-fire is absolute); never a cream/paper register.
- No drop shadow, no rounded surface (save nav dots), no gradient ground.
- No serif companion; chrome is never DejaVu Sans Mono.
- Don't pack two display moments into one frame; don't blow a long line edge-to-edge — step down.

## Aspect-Ratio Behavior

| Treatment  | 16:9                       | 9:16                       | 1:1              |
| ---------- | -------------------------- | -------------------------- | ---------------- |
| Cover      | word left, lead below      | word top, lead below       | centered word    |
| Statement  | display left               | display stacked taller     | display centered |
| Stat Grid  | 3 across                   | 3 stacked                  | 2+1              |
| Fadelist   | stack + title side-by-side | stack over title           | stack over title |
| Pull Quote | mark + quote left          | mark top, quote below      | centered         |
| Compare    | side-by-side panels        | stacked (dark over orange) | stacked          |

`pad-x` holds tight on the short edge; re-step display so the one big line stays ≤78cqw and above the
1.4cqw floor. Mono chrome stays Latin/digit-only.

## Approved Entities

No real customers, logos, or vendors are defined in the source — render any such mark as a
placeholder (the dashed `img-placeholder` at 55cqh). The system supplies type and one color, not brands.

## Numerals & Claims (hard rule)

Never invent figures, percentages, dates, or counts at frame scale. Render slots as `— figure —`,
`{metric}`, `NN%`. Stat-card numerals and bar heights carry placeholders until the script supplies
them. Catalogue numbers (No. 01) are decorative chrome and may be sequential.

## Pre-Render Self-Audit

- **Squint** — exactly one display moment dominates; nothing competes.
- **Silence** — declarative frames ~45–55% empty; only the stat grid runs dense.
- **Register** — one register per frame; ink-on-fire on orange, cream on dark; no second hue.
- **Type** — DejaVu Sans Mono lowercase 900 negative-tracked, fit-to-measure; mono chrome uppercase 0.14em; ≥1.4cqw floor.
- **Depth** — 0 shadow, 0 radius (save nav dots); 1px hairlines only.
- **Bullets** — capped at three, orange `/` marker.
- **Fabrication** — every numeral traces to the script, else placeholder.

## Known Gaps

- **Motion intentionally out of scope.** frame.md specifies composition only; the source's 0.8s deck slide + per-element entry animations are deck mechanics.
- **DejaVu Sans Mono + IBM Plex Mono via Google Fonts**; Noto Sans SC is the CJK fallback (the lowercase-display signal has no CJK equivalent — the two-register color system carries the identity, per the source).
- **9:16 / 1:1 are guidance**; verify the one big line stays ≤78cqw and above the floor per ratio.
- Bars, compare panels, and the dashed image placeholder are CSS-only; no external imagery is required.
