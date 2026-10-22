// Diagram visuals for the 16:9 split scenes (right area, 960x820): boxes, drawn arrows, a travelling packet. Each returns
// { html, css, js } for wide.mjs's `visual`; ids are prefixed with the scene id `p`, times are scene times.
import { round } from "./frame.mjs";

export const DIAGRAM_CSS = `
    .dbox { position: absolute; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 6px;
      border: 4px solid #f0ece5; background: #1a1a18; font-family: "IBM Plex Mono"; font-weight: 600; font-size: 28px;
      letter-spacing: 0.1em; text-transform: uppercase; color: #f0ece5; white-space: nowrap; }
    .dbox.hot { border-color: #e85d26; }
    .dbox .big { font-family: "Barlow"; font-weight: 900; font-size: 92px; line-height: 0.9; letter-spacing: -0.03em; color: #e85d26; text-transform: none; }
    .dbox .ip { font-family: "IBM Plex Mono"; font-weight: 500; font-size: 26px; letter-spacing: 0.04em; text-transform: none; color: #e85d26; }
    .dbox .tag { font-size: 20px; letter-spacing: 0.14em; color: #888880; }
    svg.dg { position: absolute; left: 0; top: 0; width: 960px; height: 820px; overflow: visible; }
    svg.dg path { fill: none; stroke: #e85d26; stroke-width: 8; stroke-linecap: square; stroke-linejoin: miter; }
    svg.dg path.dim { stroke: #505048; stroke-dasharray: 14 12; }
    .dlabel { position: absolute; font-family: "IBM Plex Mono"; font-weight: 600; font-size: 26px; letter-spacing: 0.1em;
      text-transform: uppercase; white-space: nowrap; color: #e85d26; }
    .dlabel.cream { color: #f0ece5; }
    .xmark { position: absolute; width: 90px; height: 90px; }
    .xmark i { position: absolute; left: 50%; top: 50%; width: 90px; height: 12px; margin: -6px 0 0 -45px; background: #e85d26; }
    .xmark i:first-child { transform: rotate(45deg); }
    .xmark i:last-child { transform: rotate(-45deg); }
    .pkt { position: absolute; left: 0; top: 0; width: 24px; height: 24px; margin: -12px 0 0 -12px; background: #e85d26; }
    .strike { position: absolute; height: 6px; background: #e85d26; transform-origin: 0 50%; }`;

export const box = (id, x, y, w, h, inner, hot = false) =>
  `          <div id="${id}" class="dbox${hot ? " hot" : ""}" style="left: ${x}px; top: ${y}px; width: ${w}px; height: ${h}px">${inner}</div>`;

// js: fade/rise a box in; draw a path (len >= its length); pop an X mark
export const inJs = (id, at, from = { y: 40 }) =>
  `      tl.fromTo($("${id}"), { ${Object.entries(from).map(([k, v]) => `${k}: ${v}`).join(", ")}, opacity: 0 }, { ${Object.keys(from).map((k) => `${k}: ${k === "scale" ? 1 : 0}`).join(", ")}, opacity: 1, duration: 0.45, ease: "power3.out" }, ${round(at)});`;
export const drawJs = (id, at, len = 600, dur = 0.6) =>
  `      $("${id}").style.strokeDasharray = "${len} ${len}";
      tl.fromTo($("${id}"), { strokeDashoffset: ${len} }, { strokeDashoffset: 0, duration: ${dur}, ease: "power2.inOut" }, ${round(at)});`;
export const popJs = (id, at) =>
  `      tl.fromTo($("${id}"), { scale: 0, rotation: -90 }, { scale: 1, rotation: 0, duration: 0.4, ease: "back.out(2)" }, ${round(at)});`;

// The meeting point: VPS on top (10.8.0.1), PC rumah (10.8.0.2) and PC kantor (10.8.0.3) below, arrows up to the VPS,
// optionally a packet travelling rumah -> VPS -> kantor. hot: which boxes are highlighted ("vps" | "pcs").
export function vpsMap(p, { at = 0.4, arrowsAt = 1.0, packetAt = null, hot = null } = {}) {
  const html = [
    box(`${p}vps`, 330, 40, 300, 200, `<span class="big">VPS</span><span class="tag">IP publik</span><span class="ip">10.8.0.1</span>`, hot === "vps"),
    `          <svg class="dg" viewBox="0 0 960 820" aria-hidden="true">
            <path id="${p}a1" d="M205 560 L420 244 M420 244 L417 280 M420 244 L389 262" />
            <path id="${p}a2" d="M755 560 L540 244 M540 244 L571 262 M540 244 L543 280" />
          </svg>`,
    box(`${p}rumah`, 40, 560, 330, 170, `PC rumah<span class="ip">10.8.0.2</span>`, hot === "pcs"),
    box(`${p}kantor`, 590, 560, 330, 170, `PC kantor<span class="ip">10.8.0.3</span>`, hot === "pcs"),
    packetAt !== null ? `          <div id="${p}pkt" class="pkt" style="opacity: 0"></div>` : "",
  ].filter(Boolean).join("\n");
  const js = [
    inJs(`${p}vps`, at, { scale: 0.8 }),
    inJs(`${p}rumah`, at + 0.2),
    inJs(`${p}kantor`, at + 0.3),
    drawJs(`${p}a1`, arrowsAt, 520),
    drawJs(`${p}a2`, arrowsAt + 0.2, 520),
  ];
  if (packetAt !== null) {
    js.push(`      tl.set($("${p}pkt"), { x: 205, y: 548, opacity: 1 }, ${round(packetAt)});
      tl.to($("${p}pkt"), { x: 480, y: 252, duration: 0.9, ease: "power1.inOut" }, ${round(packetAt)});
      tl.to($("${p}pkt"), { x: 755, y: 548, duration: 0.9, ease: "power1.inOut" }, ${round(packetAt + 1.1)});
      tl.set($("${p}pkt"), { opacity: 0 }, ${round(packetAt + 2.2)});
      tl.set($("${p}pkt"), { x: 205, y: 548, opacity: 1 }, ${round(packetAt + 2.6)});
      tl.to($("${p}pkt"), { x: 480, y: 252, duration: 0.9, ease: "power1.inOut" }, ${round(packetAt + 2.6)});
      tl.to($("${p}pkt"), { x: 755, y: 548, duration: 0.9, ease: "power1.inOut" }, ${round(packetAt + 3.7)});
      tl.set($("${p}pkt"), { opacity: 0 }, ${round(packetAt + 4.8)});`);
  }
  return { html, css: DIAGRAM_CSS, js: js.join("\n") };
}
