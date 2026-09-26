# Brand Colour Rules: getresolved.id

The colour rules of the **getresolved.id** brand, and how they apply to 3D figures. They were extracted on 2026-09-23 from [wargasipil/getresolved](https://github.com/wargasipil/getresolved/tree/dev) (branch `dev`):

- `branding/guidelines/brand.html`, the brand and logo guideline (written in Indonesian). It says the brand repo `wargasipil/getresolvedid` is the original source.
- `branding/README.md`, a cheat sheet of the guideline.
- `frontend/src/shared/index.css`, the design-system tokens the apps use, including dark mode.
- `branding/assets/logos/getresolved-mark.svg`, the mark (app icon), for its gradients.

In this project, the brand was the `getresolved` and `getresolved-dark` themes in `typescripts/animation/src/theme.ts`. The user dropped every theme but `felt` on 2026-09-26 ("just keep felt, we dont need other"), these two with the rest, so nothing uses the brand now. This doc keeps its colours and how they were mapped onto the theme roles, in case the brand comes back. The themes' last committed code is in commit 278ad67. Later changes to them were never committed and went with them, though the table below keeps the brand's sky, night and glow.

## Brand colours

| Token | Hex | Brand use |
|---|---|---|
| Indigo (primary) | `#4F46E5` | wordmark "resolved", the mark, main accent |
| Indigo Deep | `#4338CA` | gradient depth, hover |
| Indigo Light | `#818CF8` | gradient start in the mark; primary in dark mode |
| Green (accent) | `#10B981` | the "resolve point" (the bullseye `o`), ".id", success states |
| Green Light | `#34D399` | gradient start of the green dot; accent in dark mode |
| Ink | `#0F172A` | wordmark "get", body text, dark backgrounds |
| Slate | `#64748B` | muted text |
| Red | `#EF4444` | brand red, only as a swatch. Error text uses `#C81E1E`, because `#EF4444` fails WCAG AA contrast on light surfaces |

Light surfaces are neutral cool greys: background `#F6F8FB`, surface `#FFFFFF`, surface-2 `#F1F5F9`, band `#EEF2F7` (a step deeper than the background), line `#E8EDF3`, muted text `#475569`.

Dark surfaces: background `#0B1020`, surface `#111A30`, surface-2 `#18223C`, line `#22304D`, text `#E2E8F0`, muted `#94A3B8`.

## Meaning

- **Indigo = technology and trust.** It is the primary colour and carries the brand.
- **Green = the universal "done / OK" signal.** It marks the one point where a problem is "resolved": the bullseye in the logo. Green is the accent, used sparingly on the thing that matters.
- **Ink** is the dark neutral, used for text and dark backgrounds. The brand has no pure black.

## Rules from the brand

1. Keep the official colours and proportions. Don't change colours freely.
2. Don't put the coloured wordmark on a busy or low-contrast background.
3. Don't add shadows, outlines or heavy effects to the logo.
4. On dark backgrounds, use the inverse variant. For one colour, use a mono variant (black or white).
5. Dark mode switches primary to Indigo Light `#818CF8` (hover `#A5B4FC`) and accent to Green Light `#34D399`, because the standard shades are too dark there.
6. Text colours must pass WCAG AA contrast; the app checks this with axe on every story. That's why red text is `#C81E1E`, not the brand red.
7. The sticky header bar is the surface colour with 5% indigo mixed in. A whisper of brand colour is enough to make a surface read as deliberate.
8. The mark's gradients run top-left to bottom-right: Indigo Light → Indigo Deep for the tile, Green Light → Green for the dot, with a white ring between them.

## Applied to 3D figures

How each brand colour maps onto the theme roles (see [rules.md](rules.md#colour-and-theme)):

| Theme role | Light (`getresolved`) | Dark (`getresolved-dark`) | Why |
|---|---|---|---|
| `body` | Indigo `#4F46E5` | Indigo Light `#818CF8` | primary, as `--primary` in each mode |
| `trim` | Indigo Deep `#4338CA` | Indigo `#4F46E5` | "depth" |
| `accent` | Green `#10B981` | Green Light `#34D399` | controls such as knobs are the "resolve point" |
| `light` | `#FFFFFF` | `#FFFFFF` | white keys = surface |
| `dark` | Ink `#0F172A` | Ink `#0F172A` | black keys; the brand's dark is ink, not black |
| `ink` | `#FFFFFF` | Ink `#0F172A` | details on the body: white on indigo like the mark's ring; white is too faint on Indigo Light |
| `marker` | Ink `#0F172A` | Ink `#0F172A` | dot on a green knob; white on green is too faint |
| `metal` | Slate `#64748B` | `#94A3B8` | the brand has no metal colour, so the muted neutral |
| `wood` | Slate 300 `#CBD5E1` | Slate `#64748B` | the brand has no brown, and a brown would be a second hue beside green, so weathered grey wood from its neutrals |
| background / floor | `#F6F8FB` / `#EEF2F7` | `#0B1020` / `#111A30` | page background / band (light), surface (dark) |
| zenith (the sky overhead) | `#DBE0EF` | `#111A30` | light: slate 200 `#E2E8F0` with 5% indigo, the most tint rule 2 allows; dark: surface, a step up from the background. The sky deepens to it from the background at the horizon |
| hemisphere ground | `#E8EDF3` | `#18223C` | line / surface-2 |
| water (lake) | `#C5CEE1` | `#22304D` | light: slate 300 `#CBD5E1` with 5% indigo, the most tint rule 2 allows; dark: line. The brand has no water or blue colour |
| stone (lakeside rocks) | `#94A3B8` | `#475569` | slate 400 (dark --muted) and slate 600 (--muted text): neutral greys that stand out from the floor |
| grass (a lawn) | `#E2E8F0` | `#18223C` | slate 200 and dark --surface-2: green is kept for controls, so a neutral lawn |
| night (the lake's) | horizon Ink `#0F172A`, zenith `#0B1020`, sky light `#94A3B8`, ground `#18223C`, moon `#E2E8F0` | the same | the brand's dark mode: ink and dark --bg for the sky, dark --muted from above, dark --surface-2 bounced, and dark --text as the moon. The dark theme's day is already dark, so there its night is mostly the dimmer light |
| `glow` (a firefly's lantern) | Indigo Light `#818CF8` | `#A5B4FC` | the brand has no yellow and green is only for controls, so its light indigo: the mark's gradient start, and in dark mode the primary's hover, a step lighter than the indigo-light body |

Rules specific to 3D:

1. **Green goes only on controls.** Green is the "resolved" signal, so it goes on what the user acts on (knobs, pads, buttons) and nothing else. Never use it for a body or large surfaces.
2. **Keep the scene neutral.** The brand's surfaces are neutral cool greys. Don't tint the background or floor with indigo the way the `purple` theme tints everything violet. A tint of at most 5% indigo is allowed, as in the app header.
3. **Satin finish, not glossy.** The brand forbids heavy effects, so the finish is `roughness 0.5`, `clearcoat 0.4`, `clearcoatRoughness 0.3`, calmer than the toy gloss.
4. **The brand chose its accent.** The generic rule "accent = the colour opposite the brand colour" does not apply here. The opposite of indigo would be yellow, but the brand chose green.
5. **Measure the rendered colour.** The main lit face of the body must read as the brand colour. With tone mapping off, the light theme's indigo `#4F46E5` renders as `#4941D3` on the body top and `#4B42D5` on its side, and green `#10B981` renders as `#2FBD87` on the knobs. With Neutral tone mapping, the indigo rendered `#3224CE`, clearly off-brand, which is why tone mapping is off.
6. **Use unlit materials for exact colours.** A logo, the mark, or any flat brand swatch placed in 3D uses `MeshBasicMaterial` (unlit), so it shows the exact hex value. Use the SVG assets from `branding/assets/logos/` rather than redrawing them.
