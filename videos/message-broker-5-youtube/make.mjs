// Builds the YouTube (16:9, ~3 min, captions only) video for "Membuat Message Broker Sendiri" Bagian 5: a worker pool
// so one slow webhook no longer holds up the other subscribers, order kept by one worker per subscriber, fair turns when
// every worker is busy, pull streams never waiting, the 1/2/4/8-worker benchmark and the try-out. The story, numbers and
// diagrams follow the article (docs/articles/membuat-message-broker-sendiri/seri-...-bagian-5-...md).
// Run: node make.mjs. Scene kit: ../message-broker-shared/wide.mjs and diagrams.mjs.
import { buildWide } from "../message-broker-shared/wide.mjs";
import { DIAGRAM_CSS, box, text, nm, sm, arrow, svg, xmark, join, inJs, drawJs, hotJs, popJs } from "../message-broker-shared/diagrams.mjs";

const m = (s) => `<span class="m">${s}</span>`;
const body = (s, c = "#c8c4bc") => `<span class="dn" style="font-weight: 600; font-size: 30px; color: ${c}">${s}</span>`;
const dot = (id, x, y, s = 20, c = "#f0ece5") =>
  `          <div id="${id}" style="position: absolute; left: ${x - s / 2}px; top: ${y - s / 2}px; width: ${s}px; height: ${s}px; background: ${c}"></div>`;
const fill = (id, x, y, w, h) => `          <div id="${id}" class="bar" style="left: ${x}px; top: ${y}px; width: ${w}px; height: ${h}px; opacity: 0.22"></div>`;
const grow = (id, at, dur) => `      tl.fromTo($("${id}"), { scaleX: 0 }, { scaleX: 1, duration: ${dur}, ease: "none" }, ${at});`;
const clear = { background: "transparent" };
const label = (p, i, y, name, sub) => join(text(`${p}n${i}`, 0, y, nm(name), "cream"), sub ? text(`${p}k${i}`, 0, y + 44, sub, "mute", { "font-size": 21 }) : "");

// the Bagian 4 problem: one webhook sender, stok takes 4 s per call, faktur and laporan wait 8 s, pull email at once
const holds = (p) => {
  const x0 = 240, x1 = 850, px = (x1 - x0) / 8;
  const rows = [["stok", "webhook, lambat", 40], ["faktur", "webhook", 180], ["laporan", "webhook", 320], ["email", "pull", 460]];
  return { css: DIAGRAM_CSS, html: join(
    rows.map(([n, s, y], i) => label(p, i, y + 2, n, s)),
    fill(`${p}f2`, x0, 40, 4 * px, 80), fill(`${p}f3`, x0 + 4 * px, 40, 4 * px, 80),
    [1, 2].map((i) => fill(`${p}g${i}`, x0, rows[i][2], x1 - x0, 80)),
    box(`${p}s2`, x0, 40, 4 * px, 80, nm("pesan 1"), "", clear),
    box(`${p}s3`, x0 + 4 * px - 3, 40, 4 * px + 3, 80, nm("pesan 2"), "", clear),
    [1, 2].map((i) => box(`${p}b${i}`, x0, rows[i][2], x1 - x0, 80, "nunggu pengirim webhook, 8 detik", "hot", { ...clear, color: "#e85d26", "font-size": 24 })),
    [1, 2].map((i) => join(dot(`${p}e${i}`, x1 + 22, rows[i][2] + 40), text(`${p}el${i}`, x1 + 40, rows[i][2] + 26, "1, 2", "cream"))),
    dot(`${p}m`, x0 + 10, 500), text(`${p}ml`, x0 + 40, 486, "langsung dapat pesan 1 dan 2", "cream"),
    svg(960, 820, [{ id: `${p}ax`, d: `M${x0} 640 L${x1} 640 M${x0} 626 L${x0} 654 M${x0 + 4 * px} 626 L${x0 + 4 * px} 654 M${x1} 626 L${x1} 654`, cls: "cream thin" }]),
    text(`${p}x0`, x0 - 10, 670, "0", "mute"), text(`${p}x1`, x0 + 4 * px - 10, 670, "4", "mute"), text(`${p}x2`, x1 - 50, 670, "8 detik", "mute"),
  ), js: join(
    inJs([...rows.flatMap((_, i) => [`${p}n${i}`, `${p}k${i}`]), `${p}ax`, `${p}x0`, `${p}x1`, `${p}x2`], 0.4, { y: 20 }, { stagger: 0.06 }),
    inJs([`${p}s2`, `${p}b1`, `${p}b2`], 1.2, { y: 20 }, { stagger: 0.1 }), inJs([`${p}m`, `${p}ml`], 1.4, { x: -20 }, { stagger: 0.2 }),
    grow(`${p}f2`, 1.4, 4), inJs(`${p}s3`, 5.4, { y: 20 }), grow(`${p}f3`, 5.4, 4), grow(`${p}g1`, 1.4, 8), grow(`${p}g2`, 1.4, 8),
    popJs(`${p}e1`, 9.4), popJs(`${p}e2`, 9.5), inJs([`${p}el1`, `${p}el2`], 9.6, { x: -10 }),
  ) };
};

// the pool: the loop hands messages only to idle workers; results come back to the loop; laporan waits; email is pull
const pool = (p) => {
  const a1 = arrow(300, 290, 376, 140), a2 = arrow(300, 360, 376, 385);
  const b1 = arrow(580, 125, 656, 125), b2 = arrow(580, 385, 656, 385);
  const pull = arrow(110, 412, 110, 566);
  const tgt = (id, y, name, tag, line, cls = "l", tagc = "#888880") => box(id, 660, y, 300, 150,
    `<div style="display: flex; width: 100%; justify-content: space-between; align-items: baseline">${nm(name)}<span class="ds" style="color: ${tagc}">${tag}</span></div>${body(line)}`, cls, { gap: 10, padding: "0 24px" });
  return { css: DIAGRAM_CSS, html: join(
    box(`${p}lp`, 0, 240, 300, 170, `${nm("loop pengantar")}<span class="ds">worker nganggur: <span id="${p}i2">2</span><span id="${p}i0" style="display: none">0</span></span>`, "hot"),
    box(`${p}w1`, 380, 75, 200, 100, nm("worker 1")),
    box(`${p}w2`, 380, 335, 200, 100, nm("worker 2")),
    tgt(`${p}t0`, 50, "stok", "lambat, 4 d", "worker 1: pesan 4"),
    tgt(`${p}t1`, 310, "faktur", "cepat", "worker 2: pesan 9"),
    tgt(`${p}t2`, 570, "laporan", "nunggu", "punya pesan,<br>gak ada worker", "l dash", "#e85d26"),
    box(`${p}em`, 0, 570, 300, 150, `${nm("email")}${sm("stream pull,")}${sm("tanpa worker")}`),
    svg(960, 820, [
      { id: `${p}a1`, d: a1.d }, { id: `${p}a2`, d: a2.d }, { id: `${p}b1`, d: b1.d, cls: "cream thin" }, { id: `${p}b2`, d: b2.d, cls: "cream thin" },
      { id: `${p}r`, d: "M480 439 L480 500 L200 500 L200 418 M200 418 L190 434 M200 418 L210 434", cls: "dash thin" },
      { id: `${p}pl`, d: pull.d, cls: "dash thin" },
    ]),
    text(`${p}rl`, 230, 512, "hasil balik ke loop", "mute"),
  ), js: join(
    inJs(`${p}lp`, 0.4, { scale: 0.9 }), inJs([`${p}w1`, `${p}w2`], 0.7, { x: -20 }, { stagger: 0.12 }),
    drawJs(`${p}a1`, 1.4, a1.len, 0.4), drawJs(`${p}a2`, 1.6, a2.len, 0.4),
    `      tl.set($("${p}i2"), { display: "none" }, 2.0);
      tl.set($("${p}i0"), { display: "inline" }, 2.0);`,
    drawJs(`${p}b1`, 2.1, b1.len, 0.3), drawJs(`${p}b2`, 2.3, b2.len, 0.3),
    inJs([`${p}t0`, `${p}t1`], 2.3, { x: 30 }, { stagger: 0.15 }),
    drawJs(`${p}r`, 5.8, 600, 0.6), inJs(`${p}rl`, 6.2, { y: 10 }),
    inJs(`${p}t2`, 10.2, { x: 30 }),
    drawJs(`${p}pl`, 14.6, pull.len, 0.4), inJs(`${p}em`, 14.9, { y: 30 }),
  ) };
};

// order: with two workers on one subscriber, message 2 can overtake message 1; so one worker per subscriber at most
const order = (p) => ({ css: DIAGRAM_CSS, html: join(
  text(`${p}h`, 0, 0, "kalau dua worker pegang satu subscriber", "up mute"),
  box(`${p}w1`, 0, 44, 960, 110, `${nm("worker 1 · stok")}${body("pesan 1: pesanan 1041 dibuat · tertunda di jaringan")}`, "l", { gap: 8 }),
  box(`${p}w2`, 0, 174, 960, 110, `${nm("worker 2 · stok")}${body("pesan 2: pesanan 1041 dibatalkan")}`, "l", { gap: 8 }),
  text(`${p}h2`, 0, 320, "yang sampai duluan di stok", "up mute"),
  box(`${p}a1`, 0, 364, 470, 100, nm("2 · dibatalkan"), "hot"),
  box(`${p}a2`, 490, 364, 470, 100, nm("1 · dibuat")),
  xmark(`${p}x`, 880, 380),
  text(`${p}s`, 0, 482, "stok ngembaliin stok yang belum pernah diambil", "say mute", { width: 960 }),
  box(`${p}ok`, 0, 590, 960, 190, `${nm("paling banyak satu worker per subscriber")}${body("pesan berikutnya baru diserahkan setelah hasil pesan sebelumnya balik ke loop", "rgba(17, 17, 17, 0.8)")}`, "l fill", { gap: 10 }),
), js: join(
  inJs(`${p}h`, 0.4, { y: 10 }), inJs([`${p}w1`, `${p}w2`], 0.7, { x: 50 }, { stagger: 0.3 }),
  inJs(`${p}h2`, 5.4, { y: 10 }), inJs(`${p}a1`, 5.7, { x: -30 }), inJs(`${p}a2`, 6.3, { x: -30 }), popJs(`${p}x`, 6.8), inJs(`${p}s`, 7.2, { y: 10 }),
  inJs(`${p}ok`, 14.8, { y: 40 }),
) });

// fair turns: one worker, three webhook subscribers with three messages each; calls in order, row by row
const turns = (p) => {
  const grid = (g, x0, list, hot) => list.map((s, i) => {
    const r = Math.floor(i / 3), c = i % 3, cls = hot && s.startsWith("laporan") ? "cell hot" : "cell";
    return `          <div id="${p}${g}${i}" class="${cls}" style="left: ${x0 + c * 151}px; top: ${80 + r * 77}px; width: 154px; height: 80px; font-size: 24px">${s}</div>`;
  });
  const names = ["stok", "faktur", "laporan"];
  const slot = names.flatMap((n) => [1, 2, 3].map((k) => `${n} ${k}`));
  const fair = [1, 2, 3].flatMap((k) => names.map((n) => `${n} ${k}`));
  return { css: DIAGRAM_CSS, html: join(
    text(`${p}ha`, 0, 0, "bagian 4: urutan slot", "cream"), text(`${p}hb`, 504, 0, "bagian 5: yang nunggu dulu", "cream"),
    text(`${p}sa`, 0, 36, "tiap baris: satu subscriber", "mute", { "font-size": 21 }), text(`${p}sb`, 504, 36, "tiap baris: satu giliran", "mute", { "font-size": 21 }),
    grid("a", 0, slot, false), grid("b", 504, fair, true),
    text(`${p}ra`, 0, 340, "laporan nunggu 6 panggilan", "say mute", { width: 456 }),
    text(`${p}rb`, 504, 340, "laporan nunggu paling lama 2 panggilan", "say", { width: 456 }),
    box(`${p}c`, 0, 520, 960, 170, `${nm("cukup satu angka: tempat mulai putaran")}${body("putaran berikutnya mulai dari subscriber pertama yang gak kebagian worker")}`, "l flat", { gap: 10 }),
  ), js: join(
    inJs([`${p}ha`, `${p}sa`], 0.6, { y: 10 }, { stagger: 0.1 }),
    inJs(slot.map((_, i) => `${p}a${i}`), 1.4, { scale: 0.6 }, { stagger: 0.35, dur: 0.3 }),
    inJs(`${p}ra`, 5.6, { y: 10 }),
    inJs([`${p}hb`, `${p}sb`], 10.2, { y: 10 }, { stagger: 0.1 }),
    inJs(fair.map((_, i) => `${p}b${i}`), 10.6, { scale: 0.6 }, { stagger: 0.35, dur: 0.3 }),
    inJs(`${p}rb`, 14.4, { y: 10 }), inJs(`${p}c`, 15.6, { y: 30 }),
  ) };
};

// the benchmark: 8 webhooks at 10 ms each; ms per message for 1, 2, 4 and 8 workers
const bench = (p) => {
  const rows = [["1 worker", 85.8], ["2 worker", 43.3], ["4 worker", 22.0], ["8 worker", 12.2]];
  const k = 700 / 85.8;
  return { css: DIAGRAM_CSS, html: join(
    text(`${p}h`, 0, 0, "8 webhook × 10 ms · waktu per pesan", "up mute"),
    rows.map(([n, v], i) => join(
      text(`${p}n${i}`, 0, 60 + i * 140, n, "cream", { "font-size": 28 }),
      `          <div id="${p}b${i}" class="bar${i < 3 ? " mute" : ""}" style="left: 0; top: ${104 + i * 140}px; width: ${Math.round(v * k)}px; height: 54px"></div>`,
      text(`${p}v${i}`, Math.round(v * k) + 24, 108 + i * 140, `~${String(v.toFixed(1)).replace(".", ",")} ms`, "cream", { "font-size": 34 }),
    )),
    text(`${p}s`, 0, 640, "worker yang berguna paling banyak sejumlah subscriber webhook", "say", { width: 960 }),
  ), js: join(
    inJs(`${p}h`, 0.5, { y: 10 }),
    rows.map((_, i) => join(inJs(`${p}n${i}`, 1.0 + i * 0.9, { y: 10 }),
      `      tl.fromTo($("${p}b${i}"), { scaleX: 0 }, { scaleX: 1, duration: 0.6, ease: "power2.out" }, ${1.2 + i * 0.9});`,
      inJs(`${p}v${i}`, 1.7 + i * 0.9, { x: -20 }))),
    inJs(`${p}s`, 11.2, { y: 20 }),
  ) };
};

// the try-out: when faktur gets its two messages while stok takes 4 s per call
const compare = (p) => {
  const x0 = 320, x1 = 880, px = (x1 - x0) / 8;
  const rows = [["bagian 4", "satu pengirim webhook", 60], ["bagian 5 · 1 worker", "giliran gantian", 230], ["bagian 5 · 2 worker", "", 400]];
  const pin = (id, x, y, s, end = false) => join(dot(`${id}`, x, y + 40, 24),
    text(`${id}l`, end ? x - 188 : x - 70, y - 14, s, "cream", { width: end ? 200 : 140, "text-align": end ? "right" : "center" }));
  return { css: DIAGRAM_CSS, html: join(
    rows.map(([n, s, y], i) => label(p, i, y + 14, n, s)),
    fill(`${p}g0`, x0, rows[0][2] + 14, x1 - x0, 52), fill(`${p}g1`, x0, rows[1][2] + 14, x1 - x0, 52),
    box(`${p}f0`, x0, rows[0][2] + 14, x1 - x0, 52, "", "hot", clear), box(`${p}f1`, x0, rows[1][2] + 14, x1 - x0, 52, "", "hot", clear),
    pin(`${p}p0`, x1, rows[0][2], "pesan 1, 2", true),
    pin(`${p}p1`, x0 + 4 * px, rows[1][2], "pesan 1"), pin(`${p}p2`, x1, rows[1][2], "pesan 2", true),
    pin(`${p}p3`, x0, rows[2][2], "pesan 1, 2"),
    svg(960, 820, [{ id: `${p}ax`, d: `M${x0} 560 L${x1} 560 M${x0} 546 L${x0} 574 M${x0 + 4 * px} 546 L${x0 + 4 * px} 574 M${x1} 546 L${x1} 574`, cls: "cream thin" }]),
    text(`${p}x0`, x0 - 50, 590, "0 detik", "mute"), text(`${p}x1`, x0 + 4 * px - 50, 590, "4 detik", "mute"), text(`${p}x2`, x1 - 60, 590, "8 detik", "mute"),
    text(`${p}s`, 0, 680, "di ketiga percobaan, stream pull email nerima kedua pesan di detik 0", "say mute", { width: 960 }),
  ), js: join(
    inJs([`${p}ax`, `${p}x0`, `${p}x1`, `${p}x2`], 0.5, { y: 10 }, { stagger: 0.08 }),
    inJs([`${p}n0`, `${p}k0`, `${p}f0`], 5.0, { y: 20 }, { stagger: 0.1 }), grow(`${p}g0`, 5.2, 1.6), inJs([`${p}p0`, `${p}p0l`], 6.8, { y: -10 }),
    inJs([`${p}n1`, `${p}k1`, `${p}f1`], 9.6, { y: 20 }, { stagger: 0.1 }), grow(`${p}g1`, 9.8, 1.6),
    inJs([`${p}p1`, `${p}p1l`], 10.6, { y: -10 }), inJs([`${p}p2`, `${p}p2l`], 11.4, { y: -10 }),
    inJs([`${p}n2`, `${p}p3`, `${p}p3l`], 14.2, { y: 20 }, { stagger: 0.15 }),
    inJs(`${p}s`, 15.4, { y: 10 }),
  ) };
};

const { total } = buildWide({
  series: "bikin message broker sendiri · bagian 5",
  part: "05 / 06",
  scenes: [
    { type: "title", reg: "orange", dur: 7, still: true, chapter: "Pembuka: head-of-line blocking",
      kicker: "bagian 5 dari 6",
      lines: [{ text: "webhook lambat,", size: 170 }, { text: "yang lain", size: 170, light: true }, { text: "jangan nunggu", size: 170 }],
      lead: "bagian 5: worker pool agar webhook yang lambat tidak menahan subscriber lain." },
    { type: "split", dur: 17, kicker: "masalah dari bagian 4", title: "satu lambat, semua nunggu", visual: holds("s02_"),
      notes: [
        { at: 1.0, html: "bagian 4: satu goroutine manggil semua webhook" },
        { at: 5.0, html: "stok butuh 4 detik: faktur dan laporan nunggu 8 detik" },
        { at: 9.8, html: `namanya ${m("head-of-line blocking")}: yang di depan nahan semua di belakangnya` },
        { at: 13.8, html: "RFC 9113: karena ini, HTTP/1.1 buka beberapa koneksi" }] },
    { type: "compare", dur: 17, kicker: "beberapa panggilan bersamaan", title: "worker pool atau goroutine per subscriber?", chapter: "Worker pool atau goroutine per subscriber",
      panels: [
        { num: "01", name: "goroutine per subscriber", at: 2.0, lines: ["paling sederhana: urutannya terjaga sendiri", "tapi 500 subscriber bisa jadi 500 panggilan HTTP bersamaan"] },
        { num: "02 · dipilih", name: "worker pool", at: 8.5, pay: true, lines: ["sekumpulan goroutine dengan jumlah tetap: worker", "8 worker = paling banyak 8 panggilan bersamaan"] }] },
    { type: "split", dur: 20, kicker: "anatomi worker pool", title: "dua worker, tiga webhook", visual: pool("s04_"), chapter: "Anatomi worker pool",
      notes: [
        { at: 1.0, html: "loop nyerahin pesan cuma ke worker yang nganggur" },
        { at: 5.5, html: "worker manggil webhook, hasilnya selalu balik ke loop" },
        { at: 10.0, html: "semua worker sibuk? laporan nunggu giliran" },
        { at: 14.5, html: `${m("Options.Workers")}, bawaannya 1. naikin pakai ${m("-workers")}` }] },
    { type: "statement", reg: "orange", dur: 7,
      lines: [{ text: "stream pull", size: 170, light: true }, { text: "gak pernah", size: 170 }, { text: "nunggu worker.", size: 170 }],
      lead: "loop melayani stream pull sendiri, dan nyerahin pesan webhook tanpa nunggu hasilnya.", leadAt: 1.2 },
    { type: "split", dur: 19, kicker: "urutan pesan tetap terjaga", title: "satu subscriber, satu worker", visual: order("s06_"), chapter: "Urutan pesan per subscriber",
      notes: [
        { at: 1.0, html: "dua worker, satu subscriber: pesan 2 bisa nyalip pesan 1" },
        { at: 5.5, html: "stok nerima \"dibatalkan\" sebelum \"dibuat\"" },
        { at: 10.0, html: "Stripe gak jamin urutan. Kafka jamin, per partisi" },
        { at: 14.5, html: "brokerku ikut Kafka: satu subscriber, paling banyak satu worker" }] },
    { type: "split", dur: 21, kicker: "satu worker, tiga subscriber", title: "siapa dapat giliran?", visual: turns("s07_"), chapter: "Giliran saat semua worker sibuk",
      notes: [
        { at: 1.0, html: "bagian 4: loop selalu mulai dari slot pertama" },
        { at: 5.5, html: "stok terus menang, laporan nunggu enam panggilan" },
        { at: 10.2, html: "bagian 5: mulai dari subscriber pertama yang gak kebagian worker" },
        { at: 15.4, html: "tiap subscriber dapat satu panggilan per giliran" }] },
    { type: "compare", dur: 14, kicker: "bukankah ada cara yang lebih adil?", title: "tiga cara bagi giliran",
      panels: [
        { num: "01", name: "mulai acak", at: 1.6, lines: ["adil kalau dirata-rata", "tapi yang sial bisa kalah berkali-kali"] },
        { num: "02", name: "antrean yang nunggu", at: 4.6, lines: ["paling tepat", "tapi antreannya harus dijaga terus"] },
        { num: "03 · dipilih", name: "mulai dari yang nunggu", at: 7.6, pay: true, lines: ["hampir sama adilnya, cukup satu angka", "adil per panggilan, bukan per waktu"] }] },
    { type: "split", dur: 21, kicker: "berapa worker yang dibutuhkan?", title: "benchmark 1, 2, 4, 8 worker", visual: bench("s09_"), chapter: "Berapa worker yang dibutuhkan",
      notes: [
        { at: 1.0, html: "8 webhook, masing-masing butuh 10 ms per pesan" },
        { at: 6.0, html: "worker dilipatduakan, waktunya hampir kebagi dua" },
        { at: 11.0, html: "lebih dari 8 gak nolong: satu subscriber cuma pegang satu worker" },
        { at: 16.0, html: "transport bawaan Go cuma nyimpan 2 koneksi nganggur per host" }] },
    { type: "split", dur: 19, kicker: "dicoba: stok butuh 4 detik", title: "kapan faktur dapat pesannya?", visual: compare("s10_"), chapter: "Dicoba dengan webhook yang lambat",
      notes: [
        { at: 1.0, html: "dua pesan dikirim di detik 0. kapan sampai di faktur?" },
        { at: 5.2, html: "bagian 4: dua-duanya di detik 8" },
        { at: 9.8, html: "1 worker bergiliran: detik 4 dan 8" },
        { at: 14.2, html: "2 worker: langsung. cukup buat satu webhook lambat" }] },
    { type: "bullets", dur: 13, kicker: "kelemahan versi kelima", title: "yang belum beres", chapter: "Kelemahan dan bagian terakhir",
      items: [
        { at: 1.5, text: "jumlah worker tetap, dipilih saat broker jalan" },
        { at: 5.0, text: "giliran dihitung per panggilan, bukan per waktu" },
        { at: 8.5, text: "loop pengantar masih satu goroutine" }] },
    { type: "title", reg: "orange", dur: 10,
      kicker: "lanjut bagian 6 · bagian terakhir",
      lines: [{ text: "menjalankan broker", size: 150 }, { text: "dan menutup seri", size: 150, light: true }],
      lead: "artikel lengkapnya, dengan kode Go, test, benchmark, dan percobaannya: link di deskripsi.", leadAt: 1.4 },
  ],
});
console.log("bagian 5 (YouTube):", total, "s");
