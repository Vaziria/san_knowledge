// Builds the TikTok cut (9:16, ~37 s, captions only) of "Membuat Message Broker Sendiri" Bagian 5: head-of-line
// blocking, the worker pool, one worker per subscriber for order, fair turns, the 1/2/4/8-worker benchmark and the
// try-out. Facts from the article (docs/articles/membuat-message-broker-sendiri/seri-...-bagian-5-...md).
// Run: node make.mjs. Scene kit: ../message-broker-shared/tall.mjs and diagrams.mjs. YouTube: ../message-broker-5-youtube.
import { buildTall } from "../message-broker-shared/tall.mjs";
import { box, text, nm, arrow, svg, xmark, join, inJs, drawJs, hotJs, popJs } from "../message-broker-shared/diagrams.mjs";

const m = (s) => `<span class="m">${s}</span>`;
// labels at 30 px or more: a phone screen shows the 1080-wide frame at about a third of its size
const say = (s, c = "#c8c4bc") => `<span class="dn" style="font-weight: 600; font-size: 34px; color: ${c}">${s}</span>`;
const dot = (id, x, y, s = 26, c = "#f0ece5") =>
  `          <div id="${id}" style="position: absolute; left: ${x - s / 2}px; top: ${y - s / 2}px; width: ${s}px; height: ${s}px; background: ${c}"></div>`;
const fill = (id, x, y, w, h) => `          <div id="${id}" class="bar" style="left: ${x}px; top: ${y}px; width: ${w}px; height: ${h}px; opacity: 0.22"></div>`;
const grow = (id, at, dur) => `      tl.fromTo($("${id}"), { scaleX: 0 }, { scaleX: 1, duration: ${dur}, ease: "none" }, ${at});`;
const clear = { background: "transparent" };

// Bagian 4: one webhook sender, so stok's 4 s calls make faktur wait 8 s; the pull stream email is not held
const slow = (p) => {
  const x0 = 180, x1 = 842, half = (x1 - x0) / 2;
  return { h: 470, html: join(
    [["stok", 0], ["faktur", 140], ["email", 280]].map(([n, y], i) => text(`${p}n${i}`, 0, y + 26, nm(n), "cream")),
    fill(`${p}f0`, x0, 0, half, 100), fill(`${p}f1`, x0 + half, 0, half, 100), fill(`${p}f2`, x0, 140, x1 - x0, 100),
    box(`${p}s0`, x0, 0, half, 100, nm("pesan 1"), "", clear), box(`${p}s1`, x0 + half - 3, 0, half + 3, 100, nm("pesan 2"), "", clear),
    box(`${p}w`, x0, 140, x1 - x0, 100, nm("nunggu 8 detik"), "hot", { ...clear, color: "#e85d26" }),
    dot(`${p}e`, x0 + 14, 330, 24), text(`${p}el`, x0 + 46, 308, "langsung dapat 1 dan 2", "cream"),
    svg(842, 470, [{ id: `${p}ax`, d: `M${x0} 420 L${x1} 420 M${x0} 406 L${x0} 434 M${x0 + half} 406 L${x0 + half} 434 M${x1 - 2} 406 L${x1 - 2} 434`, cls: "cream thin" }]),
    text(`${p}t0`, x0 - 8, 440, "0", "mute"), text(`${p}t1`, x0 + half - 8, 440, "4", "mute"), text(`${p}t2`, x1 - 200, 440, "8 detik", "mute", { width: 200, "text-align": "right" }),
  ), js: join(
    inJs([`${p}n0`, `${p}n1`, `${p}n2`, `${p}ax`, `${p}t0`, `${p}t1`, `${p}t2`], 0.25, { y: 20 }, { stagger: 0.05 }),
    inJs([`${p}s0`, `${p}w`], 0.5, { y: 20 }, { stagger: 0.1 }), inJs([`${p}e`, `${p}el`], 0.7, { x: -20 }, { stagger: 0.1 }),
    grow(`${p}f0`, 0.6, 1.5), inJs(`${p}s1`, 2.1, { y: 20 }), grow(`${p}f1`, 2.1, 1.5), grow(`${p}f2`, 0.6, 3.0),
  ) };
};

// the pool: the loop hands messages to idle workers only; laporan waits when both are busy
const pool = (p) => {
  const a = [arrow(330, 114, 200, 176), arrow(512, 114, 642, 176), arrow(200, 284, 200, 346), arrow(642, 284, 642, 346)];
  return { h: 610, html: join(
    box(`${p}lp`, 221, 0, 400, 110, nm("loop pengantar"), "hot"),
    box(`${p}w1`, 0, 180, 400, 100, nm("worker 1")), box(`${p}w2`, 442, 180, 400, 100, nm("worker 2")),
    box(`${p}t1`, 0, 350, 400, 120, `${nm("stok")}${say("lambat, 4 detik")}`, "", { gap: 4 }),
    box(`${p}t2`, 442, 350, 400, 120, `${nm("faktur")}${say("cepat")}`, "", { gap: 4 }),
    svg(842, 610, a.map((x, i) => ({ id: `${p}a${i}`, d: x.d, cls: i < 2 ? "" : "cream thin" }))),
    box(`${p}l`, 0, 500, 842, 110, `${nm("laporan")}${say("nunggu worker bebas", "#e85d26")}`, "dash", { gap: 4 }),
  ), js: join(
    inJs(`${p}lp`, 0.3, { scale: 0.9 }), drawJs(`${p}a0`, 0.6, a[0].len, 0.3), drawJs(`${p}a1`, 0.7, a[1].len, 0.3),
    inJs([`${p}w1`, `${p}w2`], 0.8, { y: 20 }, { stagger: 0.1 }),
    drawJs(`${p}a2`, 1.2, a[2].len, 0.3), drawJs(`${p}a3`, 1.3, a[3].len, 0.3), inJs([`${p}t1`, `${p}t2`], 1.4, { y: 20 }, { stagger: 0.1 }),
    inJs(`${p}l`, 2.9, { y: 30 }),
  ) };
};

// order: two workers on one subscriber could let message 2 overtake message 1, so each subscriber holds at most one
const order = (p) => ({ h: 440, html: join(
  box(`${p}r1`, 0, 0, 842, 124, `${nm("worker 1 · stok")}${say("pesan 1: pesanan 1041 dibuat")}`, "l", { gap: 6 }),
  box(`${p}r2`, 0, 144, 842, 124, `${nm("worker 2 · stok")}${say("pesan 2: pesanan 1041 dibatalkan")}`, "l dash", { gap: 6 }),
  xmark(`${p}x`, 748, 170),
  box(`${p}ok`, 0, 300, 842, 130, `${nm("pesan 2 nunggu hasil pesan 1")}`, "fill"),
), js: join(
  inJs(`${p}r1`, 0.3, { x: 50 }), inJs(`${p}r2`, 0.7, { x: 50 }), popJs(`${p}x`, 2.6),
  `      tl.fromTo($("${p}ok"), { scale: 0.85, opacity: 0 }, { scale: 1, opacity: 1, duration: 0.5, ease: "back.out(1.6)" }, 2.9);`,
) });

// fair turns: one worker, three subscribers with three messages each; calls in order, row by row
const turns = (p) => {
  const names = ["stok", "faktur", "laporan"];
  const slot = names.flatMap((n) => [1, 2, 3].map((k) => `${n} ${k}`));
  const fair = [1, 2, 3].flatMap((k) => names.map((n) => `${n} ${k}`));
  const grid = (g, y0, list, hot) => list.map((s, i) => {
    const r = Math.floor(i / 3), c = i % 3, cls = hot && s.startsWith("laporan") ? "cell hot" : "cell";
    return `          <div id="${p}${g}${i}" class="${cls}" style="left: ${c * 283}px; top: ${y0 + r * 69}px; width: 286px; height: 72px; font-size: 30px">${s}</div>`;
  });
  return { h: 540, html: join(
    text(`${p}ha`, 0, 0, "dulu: urutan slot", "mute"), grid("a", 44, slot, false),
    text(`${p}hb`, 0, 290, "sekarang: yang nunggu duluan", "cream"), grid("b", 334, fair, true),
  ), js: join(
    inJs(`${p}ha`, 0.3, { y: 10 }), inJs(slot.map((_, i) => `${p}a${i}`), 0.4, { scale: 0.6 }, { stagger: 0.12, dur: 0.25 }),
    inJs(`${p}hb`, 2.8, { y: 10 }), inJs(fair.map((_, i) => `${p}b${i}`), 2.9, { scale: 0.6 }, { stagger: 0.12, dur: 0.25 }),
  ) };
};

// the benchmark: 8 webhooks at 10 ms each, time per message for 1, 2, 4 and 8 workers
const bench = (p) => {
  const rows = [["1 worker", 85.8], ["2 worker", 43.3], ["4 worker", 22.0], ["8 worker", 12.2]], k = 580 / 85.8;
  return { h: 460, html: join(rows.map(([n, v], i) => join(
    text(`${p}n${i}`, 0, i * 116, n, "cream"),
    `          <div id="${p}b${i}" class="bar${i < 3 ? " mute" : ""}" style="left: 0; top: ${i * 116 + 46}px; width: ${Math.round(v * k)}px; height: 50px"></div>`,
    text(`${p}v${i}`, Math.round(v * k) + 22, i * 116 + 50, `~${v.toFixed(1).replace(".", ",")} ms`, "cream", { "font-size": 36 }),
  ))), js: join(rows.map((_, i) => join(inJs(`${p}n${i}`, 0.3 + i * 0.45, { y: 10 }),
    `      tl.fromTo($("${p}b${i}"), { scaleX: 0 }, { scaleX: 1, duration: 0.4, ease: "power2.out" }, ${0.4 + i * 0.45});`,
    inJs(`${p}v${i}`, 0.7 + i * 0.45, { x: -20 })))) };
};

// the try-out: when faktur gets its two messages, sent at second 0, while stok takes 4 s per call
const tryout = (p) => {
  const x0 = 0, x1 = 800, half = (x1 - x0) / 2;
  const rows = [["bagian 4", 0], ["bagian 5 · 1 worker", 150], ["bagian 5 · 2 worker", 300]];
  return { h: 540, html: join(
    rows.map(([n, y], i) => text(`${p}n${i}`, 0, y, nm(n), "cream")),
    [0, 1].map((i) => join(fill(`${p}g${i}`, x0, rows[i][1] + 54, x1 - x0, 50), box(`${p}f${i}`, x0, rows[i][1] + 54, x1 - x0, 50, "", "hot", clear))),
    dot(`${p}d0`, x1, rows[0][1] + 79), dot(`${p}d1`, half, rows[1][1] + 79), dot(`${p}d2`, x1, rows[1][1] + 79), dot(`${p}d3`, x0 + 13, rows[2][1] + 79),
    text(`${p}l3`, 50, rows[2][1] + 58, "langsung, dua-duanya", "cream"),
    text(`${p}p0`, x1 - 300, rows[0][1] + 6, "pesan 1, 2", "", { width: 312, "text-align": "right" }),
    text(`${p}p1`, half - 14, rows[1][1] + 6, "pesan 1", ""),
    text(`${p}p2`, x1 - 300, rows[1][1] + 6, "pesan 2", "", { width: 312, "text-align": "right" }),
    svg(842, 540, [{ id: `${p}ax`, d: `M${x0} 470 L${x1} 470 M${x0 + 2} 456 L${x0 + 2} 484 M${half} 456 L${half} 484 M${x1} 456 L${x1} 484`, cls: "cream thin" }]),
    text(`${p}t0`, 0, 494, "0", "mute"), text(`${p}t1`, half - 10, 494, "4", "mute"), text(`${p}t2`, x1 - 160, 494, "8 detik", "mute", { width: 200, "text-align": "right" }),
  ), js: join(
    inJs([`${p}ax`, `${p}t0`, `${p}t1`, `${p}t2`], 0.25, { y: 10 }, { stagger: 0.05 }),
    inJs([`${p}n0`, `${p}f0`], 0.4, { y: 20 }, { stagger: 0.1 }), grow(`${p}g0`, 0.5, 1.0), popJs(`${p}d0`, 1.5), inJs(`${p}p0`, 1.6, { y: 10 }),
    inJs([`${p}n1`, `${p}f1`], 1.6, { y: 20 }, { stagger: 0.1 }), grow(`${p}g1`, 1.7, 1.0), popJs(`${p}d1`, 2.2), inJs(`${p}p1`, 2.3, { y: 10 }),
    popJs(`${p}d2`, 2.7), inJs(`${p}p2`, 2.8, { y: 10 }),
    inJs(`${p}n2`, 2.9, { y: 20 }), popJs(`${p}d3`, 3.1), inJs(`${p}l3`, 3.2, { x: -20 }),
  ) };
};

const total = buildTall({
  series: "bikin message broker sendiri",
  part: "05 / 06",
  hook: { dur: 3.2, still: true, kicker: "bagian 5 / 6",
    lines: [{ text: "satu webhook", size: 120 }, { text: "lambat,", size: 130, light: true }, { text: "semua nunggu?", size: 120 }],
    lead: "worker pool: biar yang lain gak ikut ketahan" },
  steps: [
    { kicker: "masalah bagian 4", title: "nunggu 8 detik", dur: 5.0, visual: slow("step1_"),
      marks: [
        { at: 0.8, cap: "satu goroutine manggil semua webhook" },
        { at: 2.9, cap: `namanya ${m("head-of-line blocking")}` }] },
    { kicker: "worker pool", title: "beberapa sekaligus", dur: 4.8, visual: pool("step2_"),
      marks: [
        { at: 0.8, cap: "sekumpulan goroutine, jumlahnya tetap: worker" },
        { at: 2.8, cap: "semua sibuk? yang lain nunggu worker bebas" }] },
    { kicker: "urutan pesan", title: "satu worker aja", dur: 5.0, visual: order("step3_"),
      marks: [
        { at: 0.8, cap: "dua worker bisa bikin pesan 2 nyalip pesan 1" },
        { at: 2.8, cap: "jadi tiap subscriber pegang paling banyak 1 worker" }] },
    { kicker: "giliran yang adil", title: "yang nunggu duluan", dur: 5.4, visual: turns("step4_"),
      marks: [
        { at: 0.8, cap: "dulu: stok selalu menang, laporan nunggu 6 panggilan" },
        { at: 3.0, cap: "sekarang: tiap subscriber 1 panggilan per giliran" }] },
    { kicker: "berapa worker?", title: "8 webhook × 10 ms", dur: 5.0, visual: bench("step5_"),
      marks: [
        { at: 0.8, cap: "worker dua kali lipat, waktunya hampir separuh" },
        { at: 2.9, cap: "lebih dari 8 gak nolong: 1 subscriber, 1 worker" }] },
    { kicker: "dicoba: stok 4 detik", title: "faktur dapat kapan?", dur: 5.0, visual: tryout("step6_"),
      marks: [
        { at: 0.8, cap: "bagian 4: pesan 1 dan 2 sampai di detik 8" },
        { at: 2.9, cap: "2 worker: langsung sampai" }] },
  ],
  end: { dur: 3.6, kicker: "lanjut bagian 6 · terakhir",
    lines: [{ text: "tiga cara", size: 140 }, { text: "menjalankan", size: 140, light: true }, { text: "broker", size: 140 }],
    lead: "lalu kapan RabbitMQ, NATS, atau Kafka lebih tepat" },
  endSfx: { file: "ting.wav", at: 0.3, dur: 0.45, vol: 0.4 },
});
console.log("bagian 5 (TikTok):", total, "s");
