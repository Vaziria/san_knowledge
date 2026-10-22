// 16:9 long-form scenes (1920x1080, ~3 min) for the YouTube versions of "Bikin VPN sendiri" (YouTube gets long-form
// only; the 9:16 cuts are TikTok's). Broadside (frame.md, its primary 16:9 frame). Scene types:
//   title / statement  { type, reg, dur, kicker, lines: [{ text, size, acc?, light? }], lead?, slam? }
//   bullets            { type, dur, kicker, title, items: [{ text, at }] }                       (max three, frame.md)
//   compare            { type, dur, kicker, title, panels: [{ num, name, lines: [], at, pay? }] }   (the last may be the orange payoff)
//   split              { type, dur, kicker, title, notes: [{ at, html, card?, line? }], cards?: [{ head, lines }], visual? }
//     notes build up on the left (the older ones dim); a note with `line` moves the orange band onto that card line.
//     lines as in cards.mjs: { cmd, at } typed after "$ ", { cont, at }, { conf, at }, { out, at, check? }, { blank: true }
//     visual: { html, css, js } a custom diagram in the right area (960x820 at x 860, y 150), its js in scene time.
import { esc, writeScene, writeHost, round, GSAP, TEXT_PLUGIN } from "./frame.mjs";

const W = 1920, H = 1080, LINE_H = 54, PAD_T = 24, PAD_B = 28, HEAD_H = 60;

const FONTS = [600, 700, 800, 900]
  .map((w) => `@font-face { font-family: "Barlow"; font-weight: ${w}; font-display: block; src: url("assets/fonts/barlow-latin-${w}-normal.woff2") format("woff2"); }`)
  .join("\n    ");

const WIDE_CSS = `
    ${FONTS}
    #root { position: absolute; inset: 0; width: 100%; height: 100%; overflow: hidden; }
    .beat { position: absolute; inset: 0; overflow: hidden; container-type: size; }
    .dark { background: #111111; color: #f0ece5; }
    .orange { background: #e85d26; color: #111111; }
    .chrome { position: absolute; top: 48px; font-family: "IBM Plex Mono"; font-weight: 500; font-size: 22px; letter-spacing: 0.14em;
      text-transform: uppercase; white-space: nowrap; }
    .chrome.l { left: 100px; }
    .chrome.r { right: 100px; }
    .dark .chrome { color: #888880; }
    .orange .chrome { color: rgba(17, 17, 17, 0.92); }
    .rule { position: absolute; left: 100px; right: 100px; top: 96px; height: 1px; background: #282826; }
    .orange .rule { background: rgba(17, 17, 17, 0.2); }
    .kicker { display: block; font-family: "IBM Plex Mono"; font-weight: 500; font-size: 26px; letter-spacing: 0.14em;
      text-transform: uppercase; color: #e85d26; margin-bottom: 28px; white-space: nowrap; }
    .orange .kicker { color: rgba(17, 17, 17, 0.92); }
    .stage { position: absolute; left: 100px; top: 130px; width: 1500px; height: 820px; display: flex; flex-direction: column;
      justify-content: center; align-items: flex-start; }
    .w { display: block; font-family: "Barlow"; font-weight: 900; line-height: 0.92; letter-spacing: -0.035em; white-space: nowrap; }
    .h1 { font-weight: 800; letter-spacing: -0.03em; }
    .acc { color: #e85d26; }
    .mask { display: block; overflow: hidden; padding-bottom: 0.08em; }
    .lead { display: block; margin-top: 40px; width: 1150px; font-family: "Barlow"; font-weight: 700; font-size: 44px; line-height: 1.25; }
    .dark .lead { color: #c8c4bc; }
    .orange .lead { color: rgba(17, 17, 17, 0.8); }
    .lead b { font-weight: 900; }
    .dark .lead b { color: #f0ece5; }
    .title { display: block; font-family: "Barlow"; font-weight: 800; font-size: 78px; line-height: 0.98; letter-spacing: -0.03em; }
    /* split: notes on the left, the visual on the right */
    .col { position: absolute; left: 100px; top: 150px; width: 700px; height: 820px; display: flex; flex-direction: column; justify-content: center; }
    .notes { display: flex; flex-direction: column; gap: 22px; margin-top: 44px; }
    .nt { display: flex; gap: 20px; font-family: "Barlow"; font-weight: 700; font-size: 40px; line-height: 1.25; color: #f0ece5; }
    .nt b { flex: none; font-family: "IBM Plex Mono"; font-weight: 600; color: #e85d26; }
    .nt .m { font-family: "IBM Plex Mono"; font-weight: 500; font-size: 0.85em; color: #e85d26; white-space: nowrap; }
    .vis { position: absolute; left: 860px; top: 150px; width: 960px; height: 820px; display: flex; flex-direction: column;
      justify-content: center; gap: 32px; }
    .card { width: 960px; background: #1a1a18; border: 2px solid #282826; }
    .head { height: ${HEAD_H}px; border-bottom: 2px solid #282826; padding: 0 36px; font-family: "IBM Plex Mono"; font-weight: 500;
      font-size: 22px; line-height: ${HEAD_H - 2}px; letter-spacing: 0.14em; text-transform: uppercase; color: #888880; white-space: nowrap; }
    .head.path { text-transform: none; letter-spacing: 0.04em; font-size: 24px; color: #a8a49c; }
    .code { position: relative; padding: ${PAD_T}px 36px ${PAD_B}px; }
    .band { position: absolute; left: 0; right: 0; top: 0; height: ${LINE_H}px; background: #e85d26; }
    .ln { position: relative; display: flex; align-items: center; height: ${LINE_H}px; font-family: "IBM Plex Mono"; font-weight: 400;
      font-size: 34px; line-height: ${LINE_H}px; white-space: pre; color: #f0ece5; font-variant-ligatures: none; }
    .ln .p { color: #e85d26; }
    .ln.out { color: #a8a49c; }
    .caret { display: block; flex: none; width: 20px; height: 38px; background: #f0ece5; }
    .chk { display: block; flex: none; width: 32px; height: 28px; margin-left: 20px; overflow: visible; }
    .chk path { fill: none; stroke: #e85d26; stroke-width: 4; }
    /* bullets */
    .bul { display: flex; align-items: baseline; gap: 28px; font-family: "Barlow"; font-weight: 700; font-size: 56px; line-height: 1.15;
      margin-top: 30px; white-space: nowrap; }
    .bul b { font-family: "IBM Plex Mono"; font-weight: 600; color: #e85d26; }
    /* compare */
    .cmp { position: absolute; top: 380px; width: 540px; height: 520px; padding: 44px 44px; border-top: 1px solid #282826; background: #1a1a18; }
    .cmp.pay { background: #e85d26; color: #111111; border-top-color: #e85d26; }
    .cmp .num { font-family: "IBM Plex Mono"; font-weight: 500; font-size: 26px; letter-spacing: 0.14em; color: #e85d26; }
    .cmp.pay .num { color: rgba(17, 17, 17, 0.6); }
    .cmp .nm { margin-top: 26px; font-family: "Barlow"; font-weight: 800; font-size: 60px; line-height: 1.0; letter-spacing: -0.02em; }
    .cmp .tx { margin-top: 26px; font-family: "Barlow"; font-weight: 600; font-size: 36px; line-height: 1.3; color: #c8c4bc; }
    .cmp.pay .tx { color: rgba(17, 17, 17, 0.82); }
    .ctitle { position: absolute; left: 100px; top: 170px; }`;

const typeDur = (s) => Math.min(1.5, Math.max(0.5, s.length * 0.034));

function chromeHtml(series, part) {
  return `<div class="chrome l">${esc(series)}</div><div class="chrome r">${esc(part)}</div><div class="rule"></div>`;
}

// ── shared pieces ──────────────────────────────────────────────────────────────────────────────────────────────────
function cardsHtml(id, cards) {
  return cards.map((card, ci) => {
    const rows = card.lines.map((L, li) => {
      const rid = `${id}c${ci}l${li}`;
      if (L.blank) return `            <div id="${rid}" class="ln"> </div>`;
      if (L.cmd !== undefined || L.cont !== undefined)
        return `            <div id="${rid}" class="ln"><span class="p">${L.cmd !== undefined ? "$ " : "  "}</span><span id="${rid}t"></span><span id="${rid}c" class="caret"></span></div>`;
      const check = L.check ? `<svg class="chk" viewBox="0 0 32 28" aria-hidden="true"><path id="${rid}k" d="M3 15 L12 24 L29 6" /></svg>` : "";
      return `            <div id="${rid}" class="ln${L.out !== undefined ? " out" : ""}">${esc(L.conf ?? L.out)}${check}</div>`;
    });
    return `        <div id="${id}c${ci}" class="card">
          <div class="head${card.head.startsWith("/") ? " path" : ""}">${esc(card.head)}</div>
          <div class="code">
            <div id="${id}c${ci}b" class="band" style="opacity: 0"></div>
${rows.join("\n")}
          </div>
        </div>`;
  }).join("\n");
}

// typed commands (caret blinks idle, solid while typing), revealed config/output, check marks; returns { js, sfx }
function linesJs(id, cards, dur) {
  const js = [], sfx = [], typed = [];
  cards.forEach((card, ci) => card.lines.forEach((L, li) => {
    const rid = `${id}c${ci}l${li}`;
    if (L.cmd !== undefined || L.cont !== undefined) typed.push({ rid, text: L.cmd ?? L.cont, at: L.at, cont: L.cont !== undefined });
    else if (!L.blank) {
      js.push(`      tl.fromTo($("${rid}"), { x: -18, opacity: 0 }, { x: 0, opacity: 1, duration: 0.3, ease: "power2.out" }, ${round(L.at)});`);
      if (L.check) {
        if (!sfx.some((a) => a.file === "done.wav")) sfx.push({ file: "done.wav", at: L.at + 0.1, dur: 0.6, vol: 0.3 });
        js.push(`      $("${rid}k").style.strokeDasharray = "36 36";`);
        js.push(`      tl.fromTo($("${rid}k"), { strokeDashoffset: 36 }, { strokeDashoffset: 0, duration: 0.2, ease: "power3.out" }, ${round(L.at + 0.1)});`);
      }
    }
  }));
  const showAt = (T) => (T.cont ? T.at - 0.05 : Math.max(0.5, T.at - 0.45));
  typed.forEach((T, i) => {
    const show = showAt(T), end = T.at + typeDur(T.text), next = typed[i + 1];
    const leave = next ? Math.max(end + 0.15, showAt(next)) : dur;
    js.push(`      tl.fromTo($("${T.rid}"), { opacity: 0 }, { opacity: 1, duration: 0.04, ease: "none" }, ${round(show)});`);
    js.push(`      tl.to($("${T.rid}t"), { text: { value: ${JSON.stringify(T.text)} }, duration: ${round(end - T.at)}, ease: "none" }, ${round(T.at)});`);
    const ev = [];
    const blink = (a, b) => { for (let k = 0; a + k * 0.4 < b - 1e-6; k++) ev.push([a + k * 0.4, k % 2 === 0]); };
    blink(show, T.at); ev.push([T.at, true]); blink(end, leave); if (leave < dur) ev.push([leave, false]);
    let on = true;
    for (const [t, v] of ev) { if (v === on) continue; on = v; js.push(`      tl.set($("${T.rid}c"), { opacity: ${v ? 1 : 0} }, ${round(t)});`); }
    if (!T.cont) sfx.push({ file: "keys.wav", at: T.at, dur: 0.9, vol: 0.45 });
  });
  return { js, sfx };
}

// the band moves onto a card line: instant colour sets, so no frame is cream-on-orange or ink-on-dark
function bandJs(id, marks) {
  const js = [], last = {};
  for (const m of marks) {
    if (m.line === undefined) continue;
    const ci = m.card ?? 0, band = `${id}c${ci}b`, y = PAD_T + m.line * LINE_H, rid = `${id}c${ci}l${m.line}`;
    let arrive = m.at;
    if (last[ci] === undefined) js.push(`      tl.set($("${band}"), { y: ${y}, opacity: 1 }, ${round(m.at)});`);
    else {
      arrive = m.at + 0.24;
      js.push(`      tl.set($("${last[ci]}"), { color: "#f0ece5" }, ${round(m.at)});`);
      js.push(`      tl.set($("${band}"), { opacity: 0.25 }, ${round(m.at)});`);
      js.push(`      tl.to($("${band}"), { y: ${y}, duration: 0.22, ease: "power3.inOut" }, ${round(m.at)});`);
      js.push(`      tl.set($("${band}"), { opacity: 1 }, ${round(arrive)});`);
    }
    js.push(`      tl.set($("${rid}"), { color: "#111111" }, ${round(arrive)});`);
    js.push(`      tl.fromTo($("${band}"), { scaleY: 1.16 }, { scaleY: 1, duration: 0.22, ease: "power3.out" }, ${round(arrive)});`);
    last[ci] = rid;
  }
  return js;
}

// ── scene builders ─────────────────────────────────────────────────────────────────────────────────────────────────
function titleScene(id, s, C) {
  const lines = s.lines.map((l, i) =>
    `        <span class="mask"><span id="${id}w${i}" class="w${l.light ? " h1" : ""}${l.acc ? " acc" : ""}" style="font-size: ${l.size}px">${esc(l.text)}</span></span>`);
  const ids = s.lines.map((_, i) => `$("${id}w${i}")`).join(", ");
  const body = `    <div id="${id}" class="clip beat ${s.reg ?? "orange"}" data-start="0" data-duration="${s.dur}" data-track-index="0">
      ${C}
      <div id="${id}st" class="stage">
        ${s.kicker ? `<span id="${id}k" class="kicker">${esc(s.kicker)}</span>` : ""}
${lines.join("\n")}
${s.lead ? `        <span id="${id}l" class="lead">${s.lead}</span>` : ""}
      </div>
    </div>`;
  const js = [
    s.kicker ? `      tl.fromTo($("${id}k"), { x: -50, opacity: 0 }, { x: 0, opacity: 1, duration: 0.35, ease: "expo.out" }, 0.05);` : "",
    s.slam
      ? `      tl.fromTo([${ids}], { scale: 1.35, filter: "blur(14px)", opacity: 0, transformOrigin: "0% 50%" }, { scale: 1, filter: "blur(0px)", opacity: 1, duration: 0.55, ease: "power4.out", stagger: 0.14 }, 0.12);`
      : `      tl.fromTo([${ids}], { yPercent: 110 }, { yPercent: 0, duration: 0.55, ease: "power3.out", stagger: 0.16 }, 0.12);`,
    s.lead ? `      tl.fromTo($("${id}l"), { y: 24, opacity: 0 }, { y: 0, opacity: 1, duration: 0.5, ease: "power3.out" }, ${round(s.leadAt ?? 0.9)});` : "",
    `      tl.fromTo($("${id}st"), { scale: 1, transformOrigin: "0% 50%" }, { scale: 1.015, duration: ${round(Math.max(1, s.dur - 1.2))}, ease: "sine.inOut" }, 1.0);`,
  ].filter(Boolean).join("\n");
  return { id, duration: s.dur, body, js, sfx: [] };
}

function bulletsScene(id, s, C) {
  const items = s.items.map((it, i) => `        <div id="${id}b${i}" class="bul"><b>/</b><span>${it.text}</span></div>`);
  const body = `    <div id="${id}" class="clip beat dark" data-start="0" data-duration="${s.dur}" data-track-index="0">
      ${C}
      <div class="stage">
        <span id="${id}k" class="kicker">${esc(s.kicker)}</span>
        <span id="${id}t" class="title" style="font-size: 96px">${esc(s.title)}</span>
${items.join("\n")}
      </div>
    </div>`;
  const js = [
    `      tl.fromTo($("${id}k"), { x: -50, opacity: 0 }, { x: 0, opacity: 1, duration: 0.35, ease: "expo.out" }, 0.05);`,
    `      tl.fromTo($("${id}t"), { y: 40, opacity: 0 }, { y: 0, opacity: 1, duration: 0.5, ease: "power3.out" }, 0.15);`,
    ...s.items.map((it, i) => `      tl.fromTo($("${id}b${i}"), { x: 90, opacity: 0 }, { x: 0, opacity: 1, duration: 0.5, ease: "expo.out" }, ${round(it.at)});`),
  ].join("\n");
  return { id, duration: s.dur, body, js, sfx: [] };
}

function compareScene(id, s, C) {
  const panels = s.panels.map((p, i) => `      <div id="${id}p${i}" class="cmp${p.pay ? " pay" : ""}" style="left: ${100 + i * 590}px">
        <div class="num">${esc(p.num)}</div>
        <div class="nm">${esc(p.name)}</div>
${p.lines.map((t) => `        <div class="tx">${t}</div>`).join("\n")}
      </div>`);
  const body = `    <div id="${id}" class="clip beat dark" data-start="0" data-duration="${s.dur}" data-track-index="0">
      ${C}
      <div class="ctitle">
        <span id="${id}k" class="kicker">${esc(s.kicker)}</span>
        <span id="${id}t" class="title">${esc(s.title)}</span>
      </div>
${panels.join("\n")}
    </div>`;
  const js = [
    `      tl.fromTo($("${id}k"), { x: -50, opacity: 0 }, { x: 0, opacity: 1, duration: 0.35, ease: "expo.out" }, 0.05);`,
    `      tl.fromTo($("${id}t"), { y: 40, opacity: 0 }, { y: 0, opacity: 1, duration: 0.5, ease: "power3.out" }, 0.15);`,
    ...s.panels.map((p, i) => p.pay
      ? `      tl.fromTo($("${id}p${i}"), { scale: 0.85, opacity: 0, transformOrigin: "50% 100%" }, { scale: 1, opacity: 1, duration: 0.55, ease: "back.out(1.4)" }, ${round(p.at)});`
      : `      tl.fromTo($("${id}p${i}"), { y: 80, opacity: 0 }, { y: 0, opacity: 1, duration: 0.5, ease: "power3.out" }, ${round(p.at)});`),
  ].join("\n");
  return { id, duration: s.dur, body, js, sfx: [] };
}

function splitScene(id, s, C) {
  const cards = s.cards ?? [];
  const notes = s.notes.map((n, i) => `          <div id="${id}n${i}" class="nt"><b>/</b><span>${n.html}</span></div>`);
  const visual = cards.length ? cardsHtml(id, cards) : `        <div id="${id}v" style="position: relative; width: 960px; height: 820px">
${s.visual.html}
        </div>`;
  const body = `    <div id="${id}" class="clip beat dark" data-start="0" data-duration="${s.dur}" data-track-index="0">
      ${C}
      <div class="col">
        <span id="${id}k" class="kicker">${esc(s.kicker)}</span>
        <span id="${id}t" class="title">${esc(s.title)}</span>
        <div class="notes">
${notes.join("\n")}
        </div>
      </div>
      <div class="vis">
${visual}
      </div>
    </div>`;
  const js = [
    `      tl.fromTo($("${id}k"), { x: -50, opacity: 0 }, { x: 0, opacity: 1, duration: 0.35, ease: "expo.out" }, 0.05);`,
    `      tl.fromTo($("${id}t"), { y: 40, opacity: 0 }, { y: 0, opacity: 1, duration: 0.5, ease: "power3.out" }, 0.15);`,
  ];
  if (cards.length) cards.forEach((_, ci) =>
    js.push(`      tl.fromTo($("${id}c${ci}"), { x: 80, opacity: 0 }, { x: 0, opacity: 1, duration: 0.5, ease: "power4.out" }, ${round(0.3 + ci * 0.15)});`));
  else js.push(`      tl.fromTo($("${id}v"), { opacity: 0 }, { opacity: 1, duration: 0.4, ease: "power2.out" }, 0.25);`);
  // notes build up; the newest is bright, the older ones dim
  s.notes.forEach((n, i) => {
    js.push(`      tl.fromTo($("${id}n${i}"), { y: 20, opacity: 0 }, { y: 0, opacity: 1, duration: 0.4, ease: "power3.out" }, ${round(n.at)});`);
    if (i > 0) js.push(`      tl.to($("${id}n${i - 1}"), { opacity: 0.42, duration: 0.3, ease: "power2.out" }, ${round(n.at)});`);
  });
  let sfx = [];
  if (cards.length) {
    const L = linesJs(id, cards, s.dur);
    js.push(...L.js, ...bandJs(id, s.notes));
    sfx = L.sfx;
  }
  if (s.visual?.js) js.push(s.visual.js);
  return { id, duration: s.dur, css: s.visual?.css ?? "", body, js: js.join("\n"), sfx,
    scripts: cards.length ? [GSAP, TEXT_PLUGIN] : [GSAP], prelude: cards.length ? "gsap.registerPlugin(TextPlugin);" : "" };
}

const BUILD = { title: titleScene, statement: titleScene, bullets: bulletsScene, compare: compareScene, split: splitScene };

// Builds compositions/<id>.html per scene and index.html (1920x1080). Returns the total length in seconds.
export function buildWide({ series, part, scenes: specs }) {
  const C = chromeHtml(series, part);
  const scenes = specs.map((s, i) => BUILD[s.type](s.id ?? `s${String(i + 1).padStart(2, "0")}`, s, C));
  let t = 0;
  const sfx = [];
  for (const s of scenes) {
    const js = s.prelude ? `      ${s.prelude}\n${s.js}` : s.js;
    writeScene(`compositions/${s.id}.html`, { ...s, js, width: W, height: H, baseCss: WIDE_CSS });
    sfx.push({ file: "pop.wav", at: t + (t === 0 ? 0.05 : 0), dur: 0.18, vol: 0.25 });
    for (const a of s.sfx) sfx.push({ ...a, at: t + a.at });
    t += s.duration;
  }
  sfx.sort((a, b) => a.at - b.at);
  return writeHost("index.html", { duration: round(t), scenes, sfx, width: W, height: H });
}
