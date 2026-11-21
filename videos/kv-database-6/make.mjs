// Builds the TikTok cut (9:16, ~37 s, captions only) of "Membuat Database Key-Value Sendiri" Bagian 6, the series
// finale: kvku next to bbolt and Badger on the same laptop (designs, Get, Open, Badger's SyncWrites on Windows), the
// Range fix, and which database to use. Facts from the article
// (docs/articles/membuat-database-key-value-sendiri/seri-...-bagian-6-...md).
// Run: node make.mjs. Scene kit: ../kv-database-shared/tall.mjs and diagrams.mjs. YouTube version: ../kv-database-6-youtube.
import { buildTall } from "../kv-database-shared/tall.mjs";
import { box, text, nm, bar, join, inJs, growJs } from "../kv-database-shared/diagrams.mjs";

const m = (s) => `<span class="m">${s}</span>`;
const lab = (s) => `<span class="ds" style="font-size: 30px">${s}</span>`;

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

// three designs, one row each
const designs = (p) => {
  const rows = [["kvku", "B+tree + file data, ReadAt", "fill"], ["bbolt", "B+tree, value di leaf, mmap", ""], ["Badger", "LSM tree: memtable, SSTable", ""]];
  return { h: 460, html: join(rows.map(([a, b, c], i) => box(`${p}r${i}`, 0, i * 155, 842, 135, `${nm(a)}${lab(b)}`, `l ${c}`))),
    js: inJs(rows.map((_, i) => `${p}r${i}`), 0.3, { x: 50 }, { stagger: 0.3 }) };
};

// three bars with values: rows [[name, value, label]]
const bars = (p, rows) => {
  const max = Math.max(...rows.map((r) => r[1]));
  return { h: 420, html: join(rows.map(([n, v, l], i) => {
    const w = Math.max(8, Math.round(560 * v / max));
    return join(
      text(`${p}n${i}`, 0, i * 140, n, i === 0 ? "" : "cream", { "font-size": 36 }),
      bar(`${p}b${i}`, 0, 50 + i * 140, w, 60, i === 0 ? "" : "mute"),
      text(`${p}v${i}`, w + 20, 54 + i * 140, l, i === 0 ? "" : "cream", { "font-size": 44 }),
    );
  })), js: join(rows.map((_, i) => join(inJs(`${p}n${i}`, 0.3 + i * 0.2, { x: -20 }), growJs(`${p}b${i}`, 0.4 + i * 0.2, 0.5), inJs(`${p}v${i}`, 0.8 + i * 0.2, { x: -10 })))) };
};

// which database to use
const choose = (p) => {
  const rows = [["SQLite", "data punya bentuk"], ["bbolt", "bacaan jauh lebih banyak"], ["Badger", "tulisan banyak, data besar"], ["Pebble", "LSM di produksi besar"]];
  return { h: 520, html: join(rows.map(([a, b], i) => box(`${p}r${i}`, 0, i * 130, 842, 112, `${nm(a)}${lab(b)}`, "l"))),
    js: inJs(rows.map((_, i) => `${p}r${i}`), 0.3, { x: 50 }, { stagger: 0.25 }) };
};

const total = buildTall({
  series: "bikin database key-value sendiri",
  part: "06 / 06",
  hook: { dur: 3.2, still: true, kicker: "bagian 6 / 6 · terakhir",
    lines: [{ text: "kvku lawan", size: 130, light: true }, { text: "bbolt,", size: 160 }, { text: "Badger", size: 160 }],
    lead: "diukur di laptop yang sama" },
  steps: [
    { kicker: "tiga desain", title: "B+tree atau LSM", dur: 5.2, visual: designs("step1_"),
      marks: [
        { at: 0.8, cap: "kvku: B+tree seperti bbolt" },
        { at: 2.9, cap: "value terpisah, seperti Badger" }] },
    { kicker: "Get acak · µs", title: "bbolt tercepat", dur: 5.0, visual: bars("step2_", [["kvku", 7.4, "7,4"], ["bbolt", 2.0, "2,0"], ["badger", 3.6, "3,6"]]),
      marks: [
        { at: 0.8, cap: "kvku: satu ReadAt lagi ke file data" },
        { at: 2.8, cap: `bbolt baca lewat ${m("mmap")}` }] },
    { kicker: "Open · ms", title: "kvku menang", dur: 5.0, visual: bars("step3_", [["kvku", 0.43, "0,43"], ["bbolt", 9.0, "9,0"], ["badger", 59.4, "59,4"]]),
      marks: [
        { at: 0.8, cap: "kvku cuma baca halaman 0 dan free list" },
        { at: 2.8, cap: "satu-satunya angka yang kvku menang" }] },
    { kicker: "Badger di Windows", title: "16 µs per commit?", dur: 5.4,
      visual: stat("step4_", "16", "µs", 330, "Put lalu commit, SyncWrites", "bbolt ~1.300 µs, kvku ~1.710 µs"),
      marks: [
        { at: 0.8, cap: "fsync di laptop ini butuh ratusan µs" },
        { at: 3.0, cap: `${m("msync")} Windows di ristretto: TODO, return nil` }] },
    { kicker: "Range 100 key · µs", title: "10× lebih cepat", dur: 5.0, visual: bars("step5_", [["kvku bagian 5", 270, "~270"], ["kvku bagian 6", 27, "~27"], ["bbolt", 5.8, "5,8"]]),
      marks: [
        { at: 0.8, cap: "dulu satu ReadAt untuk setiap key" },
        { at: 2.8, cap: "sekarang record bersebelahan dibaca sekaligus" }] },
    { kicker: "untuk data sungguhan", title: "pakai yang mana?", dur: 5.2, visual: choose("step6_"),
      marks: [
        { at: 0.8, cap: "kvku dibuat untuk belajar" },
        { at: 2.9, cap: "sudah punya PostgreSQL? tabel dua kolom cukup" }] },
  ],
  end: { dur: 3.6, kicker: "akhir seri · 6 / 6",
    lines: [{ text: "enam bagian,", size: 130 }, { text: "satu kvku.", size: 130, light: true }],
    lead: "yang paling berharga: alasan di balik tiap pilihan" },
  endSfx: { file: "ting.wav", at: 0.3, dur: 0.45, vol: 0.4 },
});
console.log("bagian 6 (TikTok):", total, "s");
