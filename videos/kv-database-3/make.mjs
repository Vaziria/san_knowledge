// Builds the TikTok cut (9:16, ~37 s, captions only) of "Membuat Database Key-Value Sendiri" Bagian 3: fixed 16 KB
// pages, the meta page pointing at the root, ReadAt vs mmap, the risk of overwriting pages in place, and a Get that
// reads three pages. Facts from the article (docs/articles/membuat-database-key-value-sendiri/seri-...-bagian-3-...md).
// Run: node make.mjs. Scene kit: ../kv-database-shared/tall.mjs and diagrams.mjs. YouTube version: ../kv-database-3-youtube.
import { buildTall } from "../kv-database-shared/tall.mjs";
import { box, text, nm, cells, svg, bar, join, inJs, drawJs, hotJs, growJs } from "../kv-database-shared/diagrams.mjs";

const m = (s) => `<span class="m">${s}</span>`;
const lab = (s) => `<span class="ds" style="font-size: 30px">${s}</span>`;
const head = (x, y, dir) => (dir === "down" ? ` M${x} ${y} L${x - 12} ${y - 18} M${x} ${y} L${x + 12} ${y - 18}` : ` M${x} ${y} L${x - 12} ${y + 18} M${x} ${y} L${x + 12} ${y + 18}`);

// a big number with a unit and two lines under it
const stat = (p, n, u, ux, a, b) => ({ h: 400, html: join(
  text(`${p}n`, 0, 0, n, "stat", { "font-size": 280 }),
  text(`${p}u`, ux, 120, u, "say", { "font-size": 76 }),
  text(`${p}a`, 0, 290, a, "cream", { "font-size": 34 }),
  text(`${p}b`, 0, 345, b, "cream", { "font-size": 34 }),
), js: join(
  `      tl.fromTo($("${p}n"), { scale: 0.6, opacity: 0, transformOrigin: "0% 50%" }, { scale: 1, opacity: 1, duration: 0.5, ease: "back.out(1.6)" }, 0.35);`,
  inJs(`${p}u`, 0.6, { x: -30 }), inJs([`${p}a`, `${p}b`], 1.2, { y: 20 }, { stagger: 0.3 }),
) });

// a file of fixed pages
const pages = (p) => {
  const c = cells(`${p}c`, 0, 0, ["0", "1", "2", "3"].map((t) => ({ t: `hal. ${t}` })), { w: 213, h: 120 });
  return { h: 330, html: join(
    c.html,
    ["0", "16.384", "32.768", "49.152"].map((o, i) => text(`${p}o${i}`, c.xs[i] + 6, 136, o, "mute")),
    box(`${p}f`, 0, 220, 842, 100, `<span class="dn" style="font-size: 40px">halaman n mulai di byte n × 16.384</span>`, "fill"),
  ), js: join(
    inJs(c.ids, 0.3, { y: 30 }, { stagger: 0.08 }), inJs([0, 1, 2, 3].map((i) => `${p}o${i}`), 0.9, { y: 10 }, { stagger: 0.08 }),
    hotJs(c.ids[3], 2.8, { text: true }), inJs(`${p}f`, 2.9, { y: 30 }),
  ) };
};

// the meta page points at root page 3, which points at leaves 1 and 2
const file = (p) => {
  const X = [0, 215, 430, 645], W = 196, Y = 70, H = 200, cxs = X.map((x) => x + W / 2);
  const toRoot = `M${cxs[0]} ${Y - 4} L${cxs[0]} 20 L${cxs[3]} 20 L${cxs[3]} ${Y - 4}` + head(cxs[3], Y - 4, "down");
  const to2 = `M${cxs[3] - 36} ${Y + H + 4} L${cxs[3] - 36} 320 L${cxs[2]} 320 L${cxs[2]} ${Y + H + 4}` + head(cxs[2], Y + H + 4, "up");
  const to1 = `M${cxs[3] + 36} ${Y + H + 4} L${cxs[3] + 36} 370 L${cxs[1]} 370 L${cxs[1]} ${Y + H + 4}` + head(cxs[1], Y + H + 4, "up");
  const pg = [["meta", "root: 3"], ["leaf", "a - l"], ["leaf", "m - z"], ["root", "1, m, 2"]];
  return { h: 390, html: join(
    svg(842, 390, [{ id: `${p}a0`, d: toRoot }, { id: `${p}a1`, d: to1 }, { id: `${p}a2`, d: to2 }]),
    pg.map(([t, l], i) => box(`${p}p${i}`, X[i], Y, W, H, `${lab(`hal. ${i}`)}${nm(t)}${lab(l)}`, i === 0 ? "hot" : "")),
  ), js: join(
    inJs(`${p}p0`, 0.3, { y: 30 }), inJs([1, 2, 3].map((i) => `${p}p${i}`), 0.6, { x: -30 }, { stagger: 0.12 }),
    drawJs(`${p}a0`, 1.4, 900, 0.6), drawJs(`${p}a1`, 2.2, 700, 0.5), drawJs(`${p}a2`, 2.3, 700, 0.5),
  ) };
};

// ReadAt vs mmap copy of one 16 KB page
const bars = (p) => ({ h: 420, html: join(
  text(`${p}w1`, 0, 0, "ReadAt satu halaman", "say", { "font-size": 46 }),
  bar(`${p}b1`, 0, 70, 842, 70, "mute"),
  text(`${p}v1`, 0, 155, "~7,5 µs", "cream", { "font-size": 52 }),
  text(`${p}w2`, 0, 245, "mmap, salin satu halaman", "say", { "font-size": 46 }),
  bar(`${p}b2`, 0, 315, 112, 70),
  text(`${p}v2`, 136, 321, "~1,0 µs", "", { "font-size": 52 }),
), js: join(
  inJs(`${p}w1`, 0.3, { y: 20 }), growJs(`${p}b1`, 0.5, 1.0), inJs(`${p}v1`, 1.3, { y: 10 }),
  inJs(`${p}w2`, 1.6, { y: 20 }), growJs(`${p}b2`, 1.8, 0.3), inJs(`${p}v2`, 2.0, { x: -20 }),
) });

// what Sync should write, and what is in the file after the power fails
const risk = (p) => {
  const X = [0, 286, 572], W = 270;
  const top = [["hal. 2", "m sampai r"], ["hal. 3 root", "+ pemisah s"], ["hal. 4 baru", "s sampai z"]];
  const bot = [["hal. 2", "sudah baru", ""], ["hal. 3 root", "masih lama", "hot"], ["hal. 4", "belum ada", "dash"]];
  return { h: 400, html: join(
    text(`${p}tl`, 0, 0, "seharusnya", "up mute"),
    top.map(([a, b], i) => box(`${p}t${i}`, X[i], 44, W, 130, `${nm(a)}${lab(b)}`)),
    text(`${p}bl`, 0, 214, "setelah listrik padam", "up mute"),
    bot.map(([a, b, c], i) => box(`${p}b${i}`, X[i], 258, W, 130, `${nm(a)}${lab(b)}`, c)),
  ), js: join(
    inJs(`${p}tl`, 0.3, { y: 10 }), inJs(top.map((_, i) => `${p}t${i}`), 0.4, { y: 30 }, { stagger: 0.1 }),
    inJs(`${p}bl`, 2.6, { y: 10 }), inJs(bot.map((_, i) => `${p}b${i}`), 2.7, { y: 30 }, { stagger: 0.2 }),
  ) };
};

const total = buildTall({
  series: "bikin database key-value sendiri",
  part: "03 / 06",
  hook: { dur: 3.2, still: true, kicker: "bagian 3 / 6",
    lines: [{ text: "B+tree di", size: 130, light: true }, { text: "halaman", size: 160 }, { text: "16 KB", size: 160 }],
    lead: "database key-value sendiri, pakai Go" },
  steps: [
    { kicker: "di bagian 2", title: "Get: 0,36 detik", dur: 5.0,
      visual: stat("step1_", "0,36", "detik", 560, "satu Get, file 1 juta key", "tree dibangun ulang dari seluruh log"),
      marks: [
        { at: 0.8, cap: "B+tree di memori dibangun ulang tiap mulai" },
        { at: 2.8, cap: "bagian 3: tree-nya disimpan di file" }] },
    { kicker: "halaman", title: "ukuran tetap", dur: 5.0, visual: pages("step2_"),
      marks: [
        { at: 0.8, cap: "tiap node disimpan di satu halaman 16 KB" },
        { at: 2.8, cap: "posisinya dihitung dari nomor halaman" }] },
    { kicker: "anatomi file", title: "meta ke root", dur: 5.2, visual: file("step3_"),
      marks: [
        { at: 0.8, cap: "halaman 0: meta, isinya nomor halaman root" },
        { at: 2.9, cap: "Open cukup baca halaman 0" }] },
    { kicker: "membaca halaman", title: "ReadAt lawan mmap", dur: 5.2, visual: bars("step4_"),
      marks: [
        { at: 0.8, cap: `${m("mmap")}: file dipetakan ke memori, tanpa salinan` },
        { at: 2.9, cap: "tapi beda kode tiap OS, salah baca jadi crash" }] },
    { kicker: "risikonya", title: "menimpa halaman", dur: 5.2, visual: risk("step5_"),
      marks: [
        { at: 0.8, cap: "Sync menimpa halaman di tempatnya" },
        { at: 2.9, cap: "listrik padam di tengah: key s sampai z hilang" }] },
    { kicker: "satu juta key", title: "3 halaman per Get", dur: 5.0,
      visual: stat("step6_", "0,03", "detik", 560, "satu Get, file 1 juta key", "root, branch, leaf: 3 halaman"),
      marks: [
        { at: 0.8, cap: "buka file gak perlu baca semuanya" },
        { at: 2.8, cap: "dulu ~0,36 detik, sekarang ~0,03" }] },
  ],
  end: { dur: 3.6, kicker: "lanjut bagian 4",
    lines: [{ text: "update dan", size: 140, light: true }, { text: "delete aman", size: 140 }],
    lead: "copy-on-write dan dua halaman meta" },
  endSfx: { file: "ting.wav", at: 0.3, dur: 0.45, vol: 0.4 },
});
console.log("bagian 3 (TikTok):", total, "s");
