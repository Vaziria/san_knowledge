// Code-card scenes for "Bikin VPN sendiri" parts 2 and 3 (blueprint typewriter-reveal for the typed commands, rules
// discrete-text-sequence + waterfall-entry + css-marker-patterns for the line highlight). A video is a hook (orange), one
// scene per step (dark: kicker, title, one or two cards, captions), and an end card (orange).
//
// Step spec:
//   { kicker, title, dur, cards: [{ head, lines }], marks: [{ at, card?, line?, cap }] }
//   line: { cmd: "..." , at }   typed after a "$ " prompt (keys SFX at `at`)
//         { cont: "...", at }   typed continuation of the line above, no prompt
//         { conf: "...", at }   config text, revealed      { out: "...", at, check? }   output, revealed (muted)
//         { blank: true }
//   mark: at `at`, move the orange band onto card[card].lines[line] (if given) and show caption `cap` (HTML allowed).
import { chrome, esc, writeScene, writeHost, round, GSAP, TEXT_PLUGIN } from "./frame.mjs";

const LINE_H = 48, PAD_T = 26, PAD_B = 30, HEAD_H = 64, CARD_GAP = 36, CARD_TOP = 590;

const CARD_CSS = `
    .kick { position: absolute; left: 59px; top: 400px; }
    .title { position: absolute; left: 59px; top: 450px; width: 842px; font-family: "Barlow"; font-weight: 800; font-size: 84px;
      line-height: 0.95; letter-spacing: -0.03em; white-space: nowrap; }
    .card { position: absolute; left: 50px; width: 855px; background: #1a1a18; border: 2px solid #282826; }
    .head { height: ${HEAD_H}px; border-bottom: 2px solid #282826; padding: 0 34px; font-family: "IBM Plex Mono"; font-weight: 500;
      font-size: 22px; line-height: ${HEAD_H - 2}px; letter-spacing: 0.14em; text-transform: uppercase; color: #888880; white-space: nowrap; }
    .head.path { text-transform: none; letter-spacing: 0.04em; font-size: 24px; color: #a8a49c; }
    .code { position: relative; padding: ${PAD_T}px 34px ${PAD_B}px; }
    .band { position: absolute; left: 0; right: 0; top: 0; height: ${LINE_H}px; background: #e85d26; }
    .ln { position: relative; display: flex; align-items: center; height: ${LINE_H}px; font-family: "IBM Plex Mono"; font-weight: 400;
      font-size: 30px; line-height: ${LINE_H}px; white-space: pre; color: #f0ece5; font-variant-ligatures: none; }
    .ln .p { color: #e85d26; }
    .ln.out { color: #a8a49c; }
    .caret { display: block; flex: none; width: 18px; height: 34px; background: #f0ece5; }
    .chk { display: block; flex: none; width: 30px; height: 26px; margin-left: 18px; overflow: visible; }
    .chk path { fill: none; stroke: #e85d26; stroke-width: 4; }
    .cap { position: absolute; left: 59px; width: 840px; font-family: "Barlow"; font-weight: 700; font-size: 50px; line-height: 1.2; color: #f0ece5; }
    .cap .m { font-family: "IBM Plex Mono"; font-weight: 500; font-size: 0.82em; color: #e85d26; }`;

const typeDur = (s) => Math.min(1.4, Math.max(0.5, s.length * 0.032));

function cardHeight(card) { return HEAD_H + PAD_T + PAD_B + card.lines.length * LINE_H + 4; }

// One step scene: returns { id, duration, css, body, js, scripts, sfx: [{file, at (local), dur, vol}] }.
function stepScene(id, step, series, part) {
  const sfx = [];
  let top = CARD_TOP, html = [], js = [];
  const tops = [];
  step.cards.forEach((card, ci) => {
    tops.push(top);
    const rows = card.lines.map((L, li) => {
      const rid = `${id}c${ci}l${li}`;
      if (L.blank) return `          <div id="${rid}" class="ln"> </div>`;
      if (L.cmd !== undefined || L.cont !== undefined)
        return `          <div id="${rid}" class="ln"><span class="p">${L.cmd !== undefined ? "$ " : "  "}</span><span id="${rid}t"></span><span id="${rid}c" class="caret"></span></div>`;
      const text = esc(L.conf ?? L.out);
      const check = L.check ? `<svg class="chk" viewBox="0 0 30 26" aria-hidden="true"><path id="${rid}k" d="M3 14 L11 22 L27 6" /></svg>` : "";
      return `          <div id="${rid}" class="ln${L.out !== undefined ? " out" : ""}">${text}${check}</div>`;
    });
    html.push(`      <div id="${id}c${ci}" class="card" style="top: ${top}px">
        <div class="head${card.head.startsWith("/") ? " path" : ""}">${esc(card.head)}</div>
        <div class="code">
          <div id="${id}c${ci}b" class="band" style="opacity: 0"></div>
${rows.join("\n")}
        </div>
      </div>`);
    top += cardHeight(card) + CARD_GAP;
  });
  const capTop = top - CARD_GAP + 44;
  step.marks.forEach((m, mi) => {
    if (m.cap) html.push(`      <div id="${id}m${mi}" class="cap" style="top: ${capTop}px">${m.cap}</div>`);
  });

  // entrances: kicker, title, cards
  js.push(`      tl.fromTo($("${id}k"), { x: -50, opacity: 0 }, { x: 0, opacity: 1, duration: 0.4, ease: "expo.out" }, 0.05);`);
  js.push(`      tl.fromTo($("${id}t"), { y: 40, opacity: 0 }, { y: 0, opacity: 1, duration: 0.45, ease: "power3.out" }, 0.12);`);
  step.cards.forEach((c, ci) =>
    js.push(`      tl.fromTo($("${id}c${ci}"), { y: 60, opacity: 0 }, { y: 0, opacity: 1, duration: 0.45, ease: "power4.out" }, ${round(0.3 + ci * 0.15)});`));

  // lines: typed commands with a caret, revealed config / output
  const typed = [];
  step.cards.forEach((card, ci) => card.lines.forEach((L, li) => {
    const rid = `${id}c${ci}l${li}`;
    if (L.cmd !== undefined || L.cont !== undefined) typed.push({ rid, text: L.cmd ?? L.cont, at: L.at, cont: L.cont !== undefined });
    else if (!L.blank) {
      js.push(`      tl.fromTo($("${rid}"), { x: -18, opacity: 0 }, { x: 0, opacity: 1, duration: 0.3, ease: "power2.out" }, ${round(L.at)});`);
      if (L.check) {
        if (!sfx.some((a) => a.file === "done.wav")) sfx.push({ file: "done.wav", at: L.at + 0.1, dur: 0.6, vol: 0.3 });
        js.push(`      $("${rid}k").style.strokeDasharray = "35 35";`);
        js.push(`      tl.fromTo($("${rid}k"), { strokeDashoffset: 35 }, { strokeDashoffset: 0, duration: 0.2, ease: "power3.out" }, ${round(L.at + 0.1)});`);
      }
    }
  }));
  typed.forEach((T, i) => {
    const show = T.cont ? T.at - 0.05 : Math.max(0.5, T.at - 0.45);
    const end = T.at + typeDur(T.text);
    const next = typed[i + 1];
    const leave = next ? Math.max(end + 0.15, (next.cont ? next.at - 0.05 : Math.max(0.5, next.at - 0.45))) : step.dur;
    js.push(`      tl.fromTo($("${T.rid}"), { opacity: 0 }, { opacity: 1, duration: 0.04, ease: "none" }, ${round(show)});`);
    js.push(`      tl.to($("${T.rid}t"), { text: { value: ${JSON.stringify(T.text)} }, duration: ${round(end - T.at)}, ease: "none" }, ${round(T.at)});`);
    // caret: blinks while idle, solid while typing, leaves at Enter (or blinks to the end on the last command)
    const ev = [];
    const blink = (a, b) => { for (let k = 0; a + k * 0.4 < b - 1e-6; k++) ev.push([a + k * 0.4, k % 2 === 0]); };
    blink(show, T.at); ev.push([T.at, true]); blink(end, leave); if (leave < step.dur) ev.push([leave, false]);
    let on = true;
    for (const [t, v] of ev) { if (v === on) continue; on = v; js.push(`      tl.set($("${T.rid}c"), { opacity: ${v ? 1 : 0} }, ${round(t)});`); }
    if (!T.cont) sfx.push({ file: "keys.wav", at: T.at, dur: 0.9, vol: 0.45 });
  });

  // marks: the band moves onto a line (ink on fire), the caption swaps
  const lastLine = {};
  step.marks.forEach((m, mi) => {
    const ci = m.card ?? 0;
    if (m.line !== undefined) {
      const band = `${id}c${ci}b`, y = PAD_T + m.line * LINE_H, rid = `${id}c${ci}l${m.line}`;
      // Colour changes are instant sets, so no frame shows cream on orange or ink on dark: the band dims while it
      // travels, and only on arrival goes full orange with the line turning ink (ink on fire), with a small pop.
      let arrive = m.at;
      if (lastLine[ci] === undefined) {
        js.push(`      tl.set($("${band}"), { y: ${y}, opacity: 1 }, ${round(m.at)});`);
      } else {
        arrive = m.at + 0.24;
        js.push(`      tl.set($("${lastLine[ci]}"), { color: "#f0ece5" }, ${round(m.at)});`);
        js.push(`      tl.set($("${band}"), { opacity: 0.25 }, ${round(m.at)});`);
        js.push(`      tl.to($("${band}"), { y: ${y}, duration: 0.22, ease: "power3.inOut" }, ${round(m.at)});`);
        js.push(`      tl.set($("${band}"), { opacity: 1 }, ${round(arrive)});`);
      }
      js.push(`      tl.set($("${rid}"), { color: "#111111" }, ${round(arrive)});`);
      js.push(`      tl.fromTo($("${band}"), { scaleY: 1.18 }, { scaleY: 1, duration: 0.22, ease: "power3.out" }, ${round(arrive)});`);
      lastLine[ci] = rid;
    }
    if (m.cap) {
      const prev = step.marks.slice(0, mi).filter((p) => p.cap).length ? step.marks.slice(0, mi).map((p, pi) => (p.cap ? pi : -1)).filter((x) => x >= 0).pop() : -1;
      if (prev >= 0) js.push(`      tl.to($("${id}m${prev}"), { opacity: 0, duration: 0.18, ease: "power2.in" }, ${round(m.at)});`);
      js.push(`      tl.fromTo($("${id}m${mi}"), { y: 18, opacity: 0 }, { y: 0, opacity: 1, duration: 0.35, ease: "power3.out" }, ${round(m.at + 0.12)});`);
    }
  });

  const body = `    <div id="${id}" class="clip beat dark" data-start="0" data-duration="${step.dur}" data-track-index="0">
      ${chrome(series, part)}
      <span id="${id}k" class="kicker kick">${esc(step.kicker)}</span>
      <div id="${id}t" class="title">${esc(step.title)}</div>
${html.join("\n")}
    </div>`;
  return { id, duration: step.dur, css: CARD_CSS, body, js: js.join("\n"), scripts: [GSAP, TEXT_PLUGIN], sfx,
    prelude: "gsap.registerPlugin(TextPlugin);" };
}

// The hook and the end card: orange, a kicker, two or three big lines, an optional lead line.
function titleScene(id, t, series, part) {
  const lines = t.lines.map((l, i) =>
    `        <span class="mask"><span id="${id}w${i}" class="w${l.light ? " h1" : ""}" style="font-size: ${l.size}px">${esc(l.text)}</span></span>`);
  const body = `    <div id="${id}" class="clip beat orange" data-start="0" data-duration="${t.dur}" data-track-index="0">
      ${chrome(series, part)}
      <div id="${id}st" class="stage">
        <span id="${id}k" class="kicker">${esc(t.kicker)}</span>
${lines.join("\n")}
${t.lead ? `        <span id="${id}l" class="lead">${t.lead}</span>` : ""}
      </div>
    </div>`;
  const ids = t.lines.map((_, i) => `$("${id}w${i}")`).join(", ");
  const js = [
    `      tl.fromTo($("${id}k"), { x: -50, opacity: 0 }, { x: 0, opacity: 1, duration: 0.35, ease: "expo.out" }, 0.05);`,
    t.slam
      ? `      tl.fromTo([${ids}], { scale: 1.4, filter: "blur(14px)", opacity: 0, transformOrigin: "0% 50%" }, { scale: 1, filter: "blur(0px)", opacity: 1, duration: 0.5, ease: "power4.out", stagger: 0.12 }, 0.1);`
      : `      tl.fromTo([${ids}], { yPercent: 110 }, { yPercent: 0, duration: 0.5, ease: "power3.out", stagger: 0.14 }, 0.12);`,
    t.lead ? `      tl.fromTo($("${id}l"), { y: 24, opacity: 0 }, { y: 0, opacity: 1, duration: 0.45, ease: "power3.out" }, 0.75);` : "",
    t.dur > 2.6 ? `      tl.fromTo($("${id}st"), { scale: 1, transformOrigin: "0% 50%" }, { scale: 1.02, duration: ${round(t.dur - 1.0)}, ease: "sine.inOut" }, 0.9);` : "",
  ].filter(Boolean).join("\n");
  return { id, duration: t.dur, body, js, sfx: [] };
}

// Builds the whole video: compositions/<scene>.html for each scene, and index.html. Returns the total length.
export function buildVideo({ series, part, hook, steps, end, endSfx }) {
  const scenes = [titleScene("hook", hook, series, part)];
  steps.forEach((s, i) => scenes.push(stepScene(`step${i + 1}`, s, series, part)));
  scenes.push(titleScene("akhir", end, series, part));
  let t = 0;
  const sfx = [];
  for (const s of scenes) {
    const js = s.prelude ? `      ${s.prelude}\n${s.js}` : s.js;
    writeScene(`compositions/${s.id}.html`, { ...s, js });
    sfx.push({ file: "pop.wav", at: t + (t === 0 ? 0.05 : 0), dur: 0.18, vol: 0.3 });
    for (const a of s.sfx) sfx.push({ ...a, at: t + a.at });
    t += s.duration;
  }
  if (endSfx) sfx.push({ ...endSfx, at: t - end.dur + endSfx.at });
  sfx.sort((a, b) => a.at - b.at);
  return writeHost("index.html", { duration: round(t), scenes, sfx });
}
