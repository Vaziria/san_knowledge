// Diagram primitives for the message broker videos: boxes, free text, log cells, pointers, drawn arrows, bars. A
// diagram is { html, css, js } for wide.mjs's split `visual` (960x820 area) or tall.mjs's `visual` (842 wide). Ids are
// prefixed with the scene id; times are scene times. Broadside colours only: ink, cream, fire-orange, greys.
import { round } from "./frame.mjs";

export const DIAGRAM_CSS = `
    .dbox { position: absolute; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 6px;
      border: 3px solid #f0ece5; background: #1a1a18; font-family: "IBM Plex Mono"; font-weight: 600; font-size: 26px;
      letter-spacing: 0.04em; color: #f0ece5; white-space: nowrap; text-align: center; }
    .dbox.l { align-items: flex-start; padding: 0 28px; text-align: left; }
    .dbox.hot { border-color: #e85d26; }
    .dbox.fill { background: #e85d26; border-color: #e85d26; color: #111111; }
    .dbox.mute { border-color: #505048; color: #888880; }
    .dbox.dash { border-style: dashed; border-color: #888880; color: #888880; background: transparent; }
    .dbox.flat { border-color: #282826; }
    .dn { font-family: "Barlow"; font-weight: 700; font-size: 32px; letter-spacing: 0; line-height: 1.15; white-space: normal; }
    .ds { font-family: "IBM Plex Mono"; font-weight: 500; font-size: 21px; letter-spacing: 0.04em; color: #888880; white-space: nowrap; }
    .fill .ds { color: rgba(17, 17, 17, 0.7); }
    .fill .dn { color: #111111; }
    .dbig { font-family: "Barlow"; font-weight: 900; font-size: 84px; line-height: 0.9; letter-spacing: -0.03em; color: #e85d26; }
    .fill .dbig { color: #111111; }
    .dt { position: absolute; font-family: "IBM Plex Mono"; font-weight: 600; font-size: 24px; letter-spacing: 0.06em;
      white-space: nowrap; color: #e85d26; }
    .dt.cream { color: #f0ece5; }
    .dt.mute { color: #888880; font-weight: 500; }
    .dt.up { text-transform: uppercase; letter-spacing: 0.14em; font-weight: 500; }
    .dt.say { font-family: "Barlow"; font-weight: 700; font-size: 30px; letter-spacing: 0; line-height: 1.25; white-space: normal; color: #f0ece5; }
    .dt.say.mute { color: #a8a49c; font-weight: 600; }
    .dt.stat { font-family: "Barlow"; font-weight: 900; font-size: 150px; line-height: 0.9; letter-spacing: -0.04em; }
    .cell { position: absolute; display: flex; align-items: center; justify-content: center; border: 3px solid #f0ece5;
      background: #1a1a18; font-family: "IBM Plex Mono"; font-weight: 600; font-size: 28px; color: #f0ece5; white-space: nowrap; }
    .cell.hot { border-color: #e85d26; color: #e85d26; }
    .cell.dash { border-style: dashed; border-color: #888880; color: #888880; background: transparent; }
    .ptr { position: absolute; }
    .ptr svg { position: absolute; left: 0; top: 0; overflow: visible; }
    .ptr svg path { fill: none; stroke: #e85d26; stroke-width: 5; stroke-linecap: square; }
    .ptr .lab { position: absolute; left: 0; display: flex; align-items: center; justify-content: center; border: 3px solid #e85d26;
      background: #111111; font-family: "IBM Plex Mono"; font-weight: 600; font-size: 24px; color: #f0ece5; white-space: nowrap; }
    svg.dg { position: absolute; left: 0; top: 0; overflow: visible; }
    svg.dg path { fill: none; stroke: #e85d26; stroke-width: 5; stroke-linecap: square; stroke-linejoin: miter; }
    svg.dg path.cream { stroke: #f0ece5; }
    svg.dg path.mute { stroke: #505048; }
    svg.dg path.dash { stroke: #888880; stroke-dasharray: 12 10; }
    svg.dg path.thin { stroke-width: 3; }
    .vline { position: absolute; width: 0; border-left: 4px dashed #e85d26; }
    .vline.mute { border-left-color: #505048; }
    .bar { position: absolute; background: #e85d26; transform-origin: 0% 50%; }
    .bar.mute { background: #505048; }
    .xmark { position: absolute; width: 70px; height: 70px; }
    .xmark i { position: absolute; left: 50%; top: 50%; width: 70px; height: 10px; margin: -5px 0 0 -35px; background: #e85d26; }
    .xmark i:first-child { transform: rotate(45deg); }
    .xmark i:last-child { transform: rotate(-45deg); }
    .pkt { position: absolute; left: 0; top: 0; width: 22px; height: 22px; margin: -11px 0 0 -11px; background: #e85d26; }`;

// Bigger text for the 9:16 cuts (read on a phone): applied inside tall.mjs's visual area (.tv).
export const TALL_DIAGRAM_CSS = `
    .tv .dbox { font-size: 32px; }
    .tv .dn { font-size: 38px; }
    .tv .ds { font-size: 26px; }
    .tv .dt { font-size: 30px; }
    .tv .dt.say { font-size: 36px; }
    .tv .cell { font-size: 34px; }
    .tv .ptr .lab { font-size: 30px; }`;

const px = (v) => (typeof v === "number" ? `${v}px` : v);
const st = (o) => Object.entries(o).filter(([, v]) => v !== undefined).map(([k, v]) => `${k}: ${px(v)}`).join("; ");

// ── html ─────────────────────────────────────────────────────────────────────────────────────────────────────────────
export const box = (id, x, y, w, h, inner, cls = "", extra = {}) =>
  `          <div id="${id}" class="dbox ${cls}" style="${st({ left: x, top: y, width: w, height: h, ...extra })}">${inner}</div>`;
export const text = (id, x, y, html, cls = "", extra = {}) =>
  `          <div id="${id}" class="dt ${cls}" style="${st({ left: x, top: y, ...extra })}">${html}</div>`;
export const nm = (s) => `<span class="dn">${s}</span>`;
export const sm = (s) => `<span class="ds">${s}</span>`;
export const big = (s) => `<span class="dbig">${s}</span>`;

// A row of log cells sharing borders: cells [{ t, cls?, w? }] from (x, y); returns { html, ids, xs (left edges), end }.
export function cells(p, x, y, list, { w = 90, h = 80 } = {}) {
  let cx = x;
  const html = [], ids = [], xs = [];
  list.forEach((c, i) => {
    const cw = c.w ?? w;
    html.push(`          <div id="${p}${i}" class="cell ${c.cls ?? ""}" style="left: ${cx}px; top: ${y}px; width: ${cw}px; height: ${h}px">${c.t}</div>`);
    ids.push(`${p}${i}`); xs.push(cx);
    cx += cw - 3;
  });
  return { html: html.join("\n"), ids, xs, end: cx + 3 };
}

// A pointer: an arrow pointing up at (x, y) from a label box below it. len = arrow length, w/h = label box size.
// The group's left edge is x - w / 2, so moveJs(id, at, dx) slides the whole pointer sideways.
export function pointer(id, x, y, label, { len = 70, w = 150, h = 56 } = {}) {
  const c = w / 2;
  return `          <div id="${id}" class="ptr" style="left: ${x - c}px; top: ${y}px; width: ${w}px; height: ${len + h}px">
            <svg width="${w}" height="${len}" viewBox="0 0 ${w} ${len}" aria-hidden="true"><path d="M${c} ${len} L${c} 4 M${c} 4 L${c - 12} 20 M${c} 4 L${c + 12} 20" /></svg>
            <div class="lab" style="top: ${len}px; width: ${w}px; height: ${h}px">${label}</div>
          </div>`;
}

// An arrow path from (x1, y1) to (x2, y2) with a head at the end: { d, len } (len for drawJs).
export function arrow(x1, y1, x2, y2, head = 18) {
  const a = Math.atan2(y2 - y1, x2 - x1), s = 0.5;
  const h1 = [x2 - head * Math.cos(a - s), y2 - head * Math.sin(a - s)];
  const h2 = [x2 - head * Math.cos(a + s), y2 - head * Math.sin(a + s)];
  const f = (n) => Math.round(n * 10) / 10;
  return { d: `M${f(x1)} ${f(y1)} L${f(x2)} ${f(y2)} M${f(x2)} ${f(y2)} L${f(h1[0])} ${f(h1[1])} M${f(x2)} ${f(y2)} L${f(h2[0])} ${f(h2[1])}`,
    len: Math.ceil(Math.hypot(x2 - x1, y2 - y1) + 2 * head + 4) };
}

// An svg layer of paths [{ id, d, cls? }] covering W x H.
export const svg = (W, H, paths) => `          <svg class="dg" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}" aria-hidden="true">
${paths.map((p) => `            <path id="${p.id}" ${p.cls ? `class="${p.cls}" ` : ""}d="${p.d}" />`).join("\n")}
          </svg>`;

export const vline = (id, x, y, h, cls = "") => `          <div id="${id}" class="vline ${cls}" style="left: ${x}px; top: ${y}px; height: ${h}px"></div>`;
export const bar = (id, x, y, w, h, cls = "") => `          <div id="${id}" class="bar ${cls}" style="left: ${x}px; top: ${y}px; width: ${w}px; height: ${h}px"></div>`;
export const xmark = (id, x, y) => `          <div id="${id}" class="xmark" style="left: ${x}px; top: ${y}px"><i></i><i></i></div>`;

// ── js (scene time) ──────────────────────────────────────────────────────────────────────────────────────────────────
const sel = (ids) => (Array.isArray(ids) ? `[${ids.map((i) => `$("${i}")`).join(", ")}]` : `$("${ids}")`);

// fade/rise in; `from` keys animate back to 0 (scale to 1); stagger for arrays
export function inJs(ids, at, from = { y: 30 }, { dur = 0.45, stagger = 0.08 } = {}) {
  const a = Object.entries(from).map(([k, v]) => `${k}: ${v}`).join(", ");
  const b = Object.keys(from).map((k) => `${k}: ${k.startsWith("scale") ? 1 : 0}`).join(", ");
  return `      tl.fromTo(${sel(ids)}, { ${a}${a ? ", " : ""}opacity: 0 }, { ${b}${b ? ", " : ""}opacity: 1, duration: ${dur}, ease: "power3.out"${Array.isArray(ids) ? `, stagger: ${stagger}` : ""} }, ${round(at)});`;
}
export const outJs = (ids, at, to = 0, dur = 0.3) =>
  `      tl.to(${sel(ids)}, { opacity: ${to}, duration: ${dur}, ease: "power2.out" }, ${round(at)});`;
export const drawJs = (id, at, len = 600, dur = 0.6) =>
  `      $("${id}").style.strokeDasharray = "${len} ${len}";
      tl.fromTo($("${id}"), { strokeDashoffset: ${len} }, { strokeDashoffset: 0, duration: ${dur}, ease: "power2.inOut" }, ${round(at)});`;
export const popJs = (id, at) =>
  `      tl.fromTo($("${id}"), { scale: 0, rotation: -90 }, { scale: 1, rotation: 0, duration: 0.4, ease: "back.out(2)" }, ${round(at)});`;
// highlight: the border (and text, for cells) turns orange at `at` (an instant set), with a small pulse
export const hotJs = (id, at, { text = false, fill = false } = {}) =>
  `      tl.set($("${id}"), { borderColor: "#e85d26", zIndex: 2${text ? `, color: "#e85d26"` : ""}${fill ? `, backgroundColor: "#e85d26", color: "#111111"` : ""} }, ${round(at)});
      tl.fromTo($("${id}"), { scale: 1.06 }, { scale: 1, duration: 0.35, ease: "power3.out" }, ${round(at)});`;
export const moveJs = (id, at, dx, dur = 0.5) =>
  `      tl.to($("${id}"), { x: ${dx}, duration: ${dur}, ease: "power3.inOut" }, ${round(at)});`;
export const growJs = (id, at, dur = 0.9) =>
  `      tl.fromTo($("${id}"), { scaleX: 0 }, { scaleX: 1, duration: ${dur}, ease: "power2.out" }, ${round(at)});`;
// a packet moving along points [[x, y], ...] (seconds per leg `leg`), hidden before and after
export function packetJs(id, at, pts, leg = 0.7) {
  const out = [`      tl.set($("${id}"), { x: ${pts[0][0]}, y: ${pts[0][1]}, opacity: 1 }, ${round(at)});`];
  pts.slice(1).forEach((p, i) => out.push(`      tl.to($("${id}"), { x: ${p[0]}, y: ${p[1]}, duration: ${leg}, ease: "power1.inOut" }, ${round(at + i * leg)});`));
  out.push(`      tl.set($("${id}"), { opacity: 0 }, ${round(at + (pts.length - 1) * leg + 0.1)});`);
  return out.join("\n");
}

export const join = (...parts) => parts.flat().filter(Boolean).join("\n");
