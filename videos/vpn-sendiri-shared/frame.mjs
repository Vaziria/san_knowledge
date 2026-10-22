// Shared pieces for the "Bikin VPN sendiri" videos (videos/vpn-sendiri-*): the Broadside CSS (frame.md), and writers for
// a sub-composition file and the host index.html. Each project's make.mjs builds its scenes with these; edit the specs
// there and rerun `node make.mjs`, never the generated HTML.
import { mkdirSync, writeFileSync } from "node:fs";
import { dirname } from "node:path";

export const GSAP = "https://cdn.jsdelivr.net/npm/gsap@3.14.2/dist/gsap.min.js";
export const TEXT_PLUGIN = "https://cdn.jsdelivr.net/npm/gsap@3.14.2/dist/TextPlugin.min.js";

export const esc = (s) => s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");

const FONTS = [600, 700, 800, 900]
  .map((w) => `@font-face { font-family: "Barlow"; font-weight: ${w}; font-display: block; src: url("assets/fonts/barlow-latin-${w}-normal.woff2") format("woff2"); }`)
  .join("\n    ");

// Broadside at 1080x1920 (frame.md): two registers, lowercase Barlow display, IBM Plex Mono chrome (bundled by the
// renderer), flat plane, hairlines. Safe zones: below TikTok's top tabs (~8%), above the caption band (bottom 17%),
// left of x 907 below 40% height (the right rail).
export const BASE_CSS = `
    ${FONTS}
    #root { position: absolute; inset: 0; width: 100%; height: 100%; overflow: hidden; }
    .beat { position: absolute; inset: 0; overflow: hidden; container-type: size; }
    .dark { background: #111111; color: #f0ece5; }
    .orange { background: #e85d26; color: #111111; }
    .chrome { position: absolute; top: 170px; font-family: "IBM Plex Mono"; font-weight: 500; font-size: 24px;
      letter-spacing: 0.14em; text-transform: uppercase; white-space: nowrap; }
    .chrome.l { left: 59px; }
    .chrome.r { right: 59px; }
    .dark .chrome { color: #888880; }
    .orange .chrome { color: rgba(17, 17, 17, 0.92); }
    .stage { position: absolute; left: 59px; top: 300px; width: 842px; height: 1200px; display: flex;
      flex-direction: column; justify-content: center; align-items: flex-start; }
    .kicker { display: block; font-family: "IBM Plex Mono"; font-weight: 500; font-size: 30px; letter-spacing: 0.14em;
      text-transform: uppercase; color: #e85d26; margin-bottom: 34px; white-space: nowrap; }
    .orange .kicker { color: rgba(17, 17, 17, 0.92); }
    .w { display: block; font-family: "Barlow"; font-weight: 900; line-height: 0.92; letter-spacing: -0.035em; white-space: nowrap; }
    .h1 { font-weight: 800; letter-spacing: -0.03em; }
    .acc { color: #e85d26; }
    .mask { display: block; overflow: hidden; padding-bottom: 0.08em; }
    .note { display: block; margin-top: 40px; font-family: "IBM Plex Mono"; font-weight: 500; font-size: 28px;
      letter-spacing: 0.14em; text-transform: uppercase; color: #888880; white-space: nowrap; }
    .orange .note { color: rgba(17, 17, 17, 0.7); }
    .lead { display: block; margin-top: 44px; width: 842px; font-family: "Barlow"; font-weight: 700; font-size: 46px; line-height: 1.2; }
    .orange .lead { color: rgba(17, 17, 17, 0.8); }
    .lead b { font-weight: 900; color: #111111; }`;

export function chrome(series, part) {
  return `<div class="chrome l">${esc(series)}</div><div class="chrome r">${esc(part)}</div>`;
}

// A sub-composition file: <template> with its own style and script (the assembler drops a templated file's <head>).
export function writeScene(path, { id, duration, css = "", body, js, scripts = [GSAP], width = 1080, height = 1920, baseCss = BASE_CSS }) {
  const html = `<template id="${id}-template">
  <style>${baseCss}${css}
  </style>

  <div id="root" data-composition-id="${id}" data-start="0" data-duration="${duration}" data-width="${width}" data-height="${height}">
${body}
  </div>

${scripts.map((s) => `  <script src="${s}"></script>`).join("\n")}
  <script>
    (function () {
      var tl = gsap.timeline({ paused: true });
      var $ = function (id) { return document.getElementById(id); };
${js}
      tl.set({}, {}, ${duration});
      window.__timelines["${id}"] = tl;
    })();
  </script>
</template>
`;
  mkdirSync(dirname(path), { recursive: true });
  writeFileSync(path, html);
}

// The host: one sub-composition per scene on track 0, the SFX on their own tracks.
export function writeHost(path, { duration, scenes, sfx, width = 1080, height = 1920 }) {
  let t = 0;
  const hosts = scenes.map((s) => {
    const line = `      <div id="el-${s.id}" class="scene" data-composition-id="${s.id}" data-composition-src="compositions/${s.id}.html" data-start="${round(t)}" data-duration="${s.duration}" data-track-index="0"></div>`;
    t += s.duration;
    return line;
  });
  const audio = sfx.map((a, i) =>
    `      <audio id="sfx-${i + 1}" src="assets/sfx/${a.file}" data-start="${round(a.at)}" data-duration="${a.dur}" data-track-index="${10 + i}" data-volume="${a.vol}"></audio>`);
  const html = `<!doctype html>
<html lang="id">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=${width}, height=${height}" />
    <script src="${GSAP}"></script>
    <style>
      * { margin: 0; padding: 0; box-sizing: border-box; }
      html, body { width: ${width}px; height: ${height}px; overflow: hidden; background: #111111; }
      #root { position: relative; width: 100%; height: 100%; overflow: hidden; background: #111111; }
      .scene { position: absolute; inset: 0; width: 100%; height: 100%; }
    </style>
  </head>
  <body>
    <!-- generated by make.mjs: edit the spec there and rerun it -->
    <div id="root" data-composition-id="main" data-start="0" data-duration="${duration}" data-width="${width}" data-height="${height}">
${hosts.join("\n")}

${audio.join("\n")}
    </div>
    <script>
      var tl = gsap.timeline({ paused: true });
      tl.to({}, { duration: ${duration} }, 0);
      window.__timelines["main"] = tl;
    </script>
  </body>
</html>
`;
  writeFileSync(path, html);
  return t;
}

export const round = (x) => Math.round(x * 1000) / 1000;
