// Builds the TikTok cut (9:16, ~37 s, captions only) of "Membuat Database Key-Value Sendiri" Bagian 1: a map is lost on
// restart, a log on disk with the map as its index, write vs fsync, the 12-byte record header, torn writes, and the
// slow open of a million keys. Facts from the article (docs/articles/membuat-database-key-value-sendiri/seri-...-bagian-1-...md).
// Run: node make.mjs (writes index.html and compositions/). Scene kit: ../kv-database-shared/tall.mjs and diagrams.mjs.
// YouTube version: ../kv-database-1-youtube.
import { buildTall } from "../kv-database-shared/tall.mjs";
import { box, text, nm, cells, vline, bar, join, inJs, outJs, hotJs, growJs } from "../kv-database-shared/diagrams.mjs";

const m = (s) => `<span class="m">${s}</span>`;
// TikTok labels stay >= 30 px (sm() is 26 px in the tall CSS)
const lab = (s) => `<span class="ds" style="font-size: 30px">${s}</span>`;

// the map before and after a restart
const restart = (p) => ({ h: 360, html: join(
  box(`${p}b`, 0, 0, 400, 300, `${lab("sebelum")}${nm("user:1 = Andi")}${nm("user:2 = Budi")}`),
  box(`${p}e`, 442, 0, 400, 300, `${lab("sesudah restart")}${nm("kosong")}`, "dash"),
), js: join(
  inJs(`${p}b`, 0.35, { y: 30 }),
  inJs(`${p}e`, 2.7, { x: -40 }),
) });

// the log grows at its end; the map points into it
const logIndex = (p) => {
  const c = cells(`${p}c`, 0, 60, [{ t: "user:1" }, { t: "user:2" }, { t: "user:1" }], { w: 283, h: 110 });
  return { h: 400, html: join(
    text(`${p}l`, 0, 0, "file log di disk", "up mute"),
    c.html,
    [0, 22, 44].map((o, i) => text(`${p}o${i}`, c.xs[i] + 8, 186, `posisi ${o}`, "mute")),
    box(`${p}i`, 0, 270, 842, 110, `${lab("index di memori")}${nm("user:1 · 44, user:2 · 22")}`, "hot"),
  ), js: join(
    inJs(`${p}l`, 0.3, { y: 10 }),
    c.ids.map((id, i) => inJs([id, `${p}o${i}`], 0.5 + i * 0.4, { x: -30 })),
    outJs(c.ids[0], 2.9, 0.35, 0.3),
    inJs(`${p}i`, 2.7, { y: 30 }),
  ) };
};

// Put vs Put + Sync (4318 vs 487682 ns/op in the article's benchmark)
const bars = (p) => ({ h: 420, html: join(
  text(`${p}w1`, 0, 0, "Put", "say", { "font-size": 46 }),
  bar(`${p}b1`, 0, 70, 12, 70),
  text(`${p}v1`, 36, 76, "~4,3 µs", "cream", { "font-size": 52 }),
  text(`${p}w2`, 0, 190, "Put + Sync", "say", { "font-size": 46 }),
  bar(`${p}b2`, 0, 260, 842, 70),
  text(`${p}v2`, 0, 345, "~488 µs", "", { "font-size": 52 }),
), js: join(
  inJs(`${p}w1`, 0.3, { y: 20 }), growJs(`${p}b1`, 0.5, 0.3), inJs(`${p}v1`, 0.6, { x: -20 }),
  inJs(`${p}w2`, 1.0, { y: 20 }), growJs(`${p}b2`, 1.2, 1.2), inJs(`${p}v2`, 2.2, { y: 10 }),
) });

// the record, stacked like memory, the CRC highlighted
const record = (p) => {
  const f = [["0", "CRC-32C", "4 byte"], ["4", "panjang key", "4 byte"], ["8", "panjang value", "4 byte"], ["12", "key, lalu value", ""]];
  const row = (a, b) => `<div style="display: flex; width: 100%; justify-content: space-between; align-items: baseline">${nm(a)}${lab(b)}</div>`;
  return { h: 360, html: join(
    f.map(([o, a, b], i) => join(text(`${p}o${i}`, 0, i * 88 + 26, o, "mute"), box(`${p}f${i}`, 90, i * 88, 752, 91, row(a, b), "l"))),
  ), js: join(
    inJs(f.map((_, i) => `${p}f${i}`), 0.35, { x: 60 }, { stagger: 0.1 }), inJs(f.map((_, i) => `${p}o${i}`), 0.5, { y: 0 }, { stagger: 0.1 }),
    hotJs(`${p}f0`, 2.7),
  ) };
};

// a torn write: the third record is half written, the file is cut after record 2
const torn = (p) => ({ h: 300, html: join(
  box(`${p}r1`, 0, 30, 283, 130, nm("user:1")),
  box(`${p}r2`, 280, 30, 283, 130, nm("user:2")),
  box(`${p}r3`, 560, 30, 282, 130, `${nm("user:3")}${lab("setengah")}`, "dash"),
  vline(`${p}cut`, 558, 0, 200),
  text(`${p}cl`, 250, 225, "ujung file setelah Open"),
), js: join(
  inJs([`${p}r1`, `${p}r2`], 0.3, { y: 30 }, { stagger: 0.12 }),
  `      tl.fromTo($("${p}r3"), { scaleX: 0, transformOrigin: "0% 50%" }, { scaleX: 1, duration: 0.9, ease: "power1.in" }, 0.7);`,
  outJs(`${p}r3`, 2.9, 0.15, 0.4), inJs([`${p}cut`, `${p}cl`], 3.0, { y: -20 }),
) });

// opening a log of a million keys (558 ms in the article)
const slow = (p) => ({ h: 400, html: join(
  text(`${p}n`, 0, 0, "558", "stat", { "font-size": 280 }),
  text(`${p}u`, 500, 120, "ms", "say", { "font-size": 76 }),
  text(`${p}a`, 0, 290, "buka file 1 juta key, 40 MB", "cream", { "font-size": 34 }),
  text(`${p}b`, 0, 345, "satu Get pun nunggu selama itu", "cream", { "font-size": 34 }),
), js: join(
  `      tl.fromTo($("${p}n"), { scale: 0.6, opacity: 0, transformOrigin: "0% 50%" }, { scale: 1, opacity: 1, duration: 0.5, ease: "back.out(1.6)" }, 0.35);`,
  inJs(`${p}u`, 0.6, { x: -30 }), inJs([`${p}a`, `${p}b`], 1.2, { y: 20 }, { stagger: 0.3 }),
) });

const total = buildTall({
  series: "bikin database key-value sendiri",
  part: "01 / 06",
  hook: { dur: 3.2, still: true, kicker: "seri baru · bagian 1 / 6",
    lines: [{ text: "bikin database", size: 112, light: true }, { text: "key-value", size: 160 }, { text: "sendiri", size: 160, light: true }],
    lead: "pakai Go, cuma library standar" },
  steps: [
    { kicker: "masalahnya", title: "map hilang", dur: 5.0, visual: restart("step1_"),
      marks: [
        { at: 0.8, cap: "map di memori: penyimpanan key-value paling cepat" },
        { at: 2.7, cap: "tapi program berhenti atau di-restart: isinya hilang" }] },
    { kicker: "solusinya", title: "log + index", dur: 5.2, visual: logIndex("step2_"),
      marks: [
        { at: 0.8, cap: `data ditulis di ${m("log")}: file yang cuma bertambah di ujung` },
        { at: 2.8, cap: "map cuma jadi index: key ke posisinya di file" }] },
    { kicker: "kapan data aman?", title: "write lawan fsync", dur: 5.0, visual: bars("step3_"),
      marks: [
        { at: 0.8, cap: "write belum tentu sampai ke disk" },
        { at: 2.8, cap: `${m("Sync")} maksa ke disk, tapi 100 kali lebih lama` }] },
    { kicker: "anatomi record", title: "header 12 byte", dur: 5.0, visual: record("step4_"),
      marks: [
        { at: 0.8, cap: "tiap Put jadi satu record di ujung log" },
        { at: 2.7, cap: `${m("CRC-32C")}: angka pemeriksa dari isi record` }] },
    { kicker: "listrik padam", title: "record terpotong", dur: 5.0, visual: torn("step5_"),
      marks: [
        { at: 0.8, cap: "Open baca semua record dari awal" },
        { at: 2.8, cap: "record rusak pertama: file dipotong di situ" }] },
    { kicker: "kelemahannya", title: "buka makin lama", dur: 5.0, visual: slow("step6_"),
      marks: [
        { at: 0.8, cap: "tiap buka, seluruh log dibaca ulang" },
        { at: 2.8, cap: "dan Range harus memeriksa semua key" }] },
  ],
  end: { dur: 3.6, kicker: "lanjut bagian 2",
    lines: [{ text: "B+tree", size: 170 }, { text: "di memori", size: 140, light: true }],
    lead: "key urut, Range gak periksa semua key" },
  endSfx: { file: "ting.wav", at: 0.3, dur: 0.45, vol: 0.4 },
});
console.log("bagian 1 (TikTok):", total, "s");
