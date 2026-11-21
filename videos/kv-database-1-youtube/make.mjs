// Builds the YouTube (16:9, ~3 min, captions only) video for "Membuat Database Key-Value Sendiri" Bagian 1: why a map in
// memory is not enough, then the first kvku: a log on disk with a map as its index. The story, numbers and diagrams
// follow the article (docs/articles/membuat-database-key-value-sendiri/seri-...-bagian-1-...md). Run: node make.mjs.
// Scene kit: ../kv-database-shared/wide.mjs and diagrams.mjs.
import { buildWide } from "../kv-database-shared/wide.mjs";
import {
  DIAGRAM_CSS, box, text, nm, sm, cells, pointer, arrow, svg, vline, bar, join,
  inJs, outJs, drawJs, hotJs, growJs,
} from "../kv-database-shared/diagrams.mjs";

const m = (s) => `<span class="m">${s}</span>`;

// a key and its value, then the three operations
const ops = (p) => {
  const a = arrow(300, 105, 415, 105);
  const rows = [["Put", "simpan value untuk key, yang lama diganti"], ["Get", "ambil value untuk key: user:1, hasilnya Andi"],
    ["Range", "semua key dari satu key sampai key lain, urut"]];
  return { css: DIAGRAM_CSS, html: join(
    box(`${p}k`, 0, 40, 300, 130, `${nm("user:1")}${sm("key")}`, "hot"),
    svg(960, 820, [{ id: `${p}a`, d: a.d, cls: "cream thin" }]),
    box(`${p}v`, 420, 40, 540, 130, `${nm("Andi")}${sm("value: teks, JSON, byte apa saja")}`),
    rows.map(([a1, b], i) => box(`${p}o${i}`, 0, 270 + i * 170, 960, 140, `${nm(a1)}${sm(b)}`, "l")),
  ), js: join(
    inJs(`${p}k`, 0.6, { y: 30 }), drawJs(`${p}a`, 1.2, a.len, 0.4), inJs(`${p}v`, 1.5, { y: 30 }),
    inJs(rows.map((_, i) => `${p}o${i}`), 8.6, { x: 60 }, { stagger: 0.25 }),
    hotJs(`${p}o2`, 11.0),
  ) };
};

// the series map: six parts, this one highlighted
const parts = (p) => {
  const rows = ["kenapa map gak cukup", "B+tree: struktur dan pencarian", "halaman 16 KB dan mmap", "update dan delete yang aman",
    "compaction, data lebih besar dari RAM", "mengukur dan menutup seri"];
  return { css: DIAGRAM_CSS + `
    .prow { display: flex; width: 100%; align-items: baseline; gap: 30px; }
    .prow .ds { font-size: 26px; }`, html: join(rows.map((r, i) =>
    box(`${p}r${i}`, 0, i * 134 + 12, 960, 112,
      `<div class="prow">${sm(`0${i + 1}`)}${nm(r)}${i === 0 ? `<span class="ds" style="margin-left: auto">VIDEO INI</span>` : ""}</div>`,
      i === 0 ? "l fill" : "l flat"))),
  js: inJs(rows.map((_, i) => `${p}r${i}`), 0.4, { x: 60 }, { stagger: 0.12 }) };
};

// a map only in memory: full before the restart, empty after it
const restart = (p) => {
  const a = arrow(430, 300, 525, 300);
  return { css: DIAGRAM_CSS, html: join(
    text(`${p}l1`, 0, 100, "sebelum restart", "up mute"),
    box(`${p}b`, 0, 150, 420, 300, `${sm("map di memori")}${nm("user:1 = Andi")}${nm("user:2 = Budi")}`),
    svg(960, 820, [{ id: `${p}a`, d: a.d, cls: "cream thin" }]),
    text(`${p}al`, 428, 318, "restart", "cream"),
    text(`${p}l2`, 540, 100, "sesudah restart", "up mute"),
    box(`${p}e`, 540, 150, 420, 300, `${sm("map di memori")}${nm("kosong")}`, "dash"),
    text(`${p}say`, 0, 540, "program berhenti, crash, atau di-restart: memorinya kembali ke OS, isi map ikut hilang", "say mute", { width: 960 }),
  ), js: join(
    inJs([`${p}l1`, `${p}b`], 0.4, { y: 30 }, { stagger: 0.1 }),
    drawJs(`${p}a`, 5.8, a.len, 0.4), inJs(`${p}al`, 6.0, { y: 10 }),
    inJs([`${p}l2`, `${p}e`], 6.4, { y: 30 }, { stagger: 0.1 }),
    inJs(`${p}say`, 10.6, { y: 20 }),
  ) };
};

// the log on disk grows at its end; the map becomes the index pointing into it
const logIndex = (p) => {
  const c = cells(`${p}c`, 0, 480, [{ t: "user:1 Andi" }, { t: "user:2 Budi" }, { t: "user:1 Andi W." }], { w: 320, h: 100 });
  const a1 = arrow(440, 155, c.xs[2] + 160, 474), a2 = arrow(440, 265, c.xs[1] + 160, 474);
  return { css: DIAGRAM_CSS, html: join(
    text(`${p}il`, 0, 60, "index di memori (map)", "up mute"),
    box(`${p}i0`, 0, 110, 440, 90, nm("user:1 · posisi 44"), "l hot"),
    box(`${p}i1`, 0, 220, 440, 90, nm("user:2 · posisi 22"), "l hot"),
    svg(960, 820, [{ id: `${p}a1`, d: a1.d, cls: "thin" }, { id: `${p}a2`, d: a2.d, cls: "thin" }]),
    text(`${p}gl`, 520, 330, "Get", ""),
    text(`${p}ll`, 0, 430, "file log di disk: cuma bertambah di ujung", "up mute"),
    c.html,
    [0, 22, 44].map((o, i) => text(`${p}o${i}`, c.xs[i] + 8, 600, `posisi ${o}`, "mute")),
    vline(`${p}end`, c.end + 4, 460, 140),
    text(`${p}say`, 0, 690, "Open: baca seluruh log dari awal, lalu isi map lagi", "say mute", { width: 960 }),
  ), js: join(
    inJs(`${p}ll`, 0.4, { y: 10 }),
    inJs(c.ids[0], 1.0, { x: -30 }), inJs(`${p}o0`, 1.1, { y: 0 }),
    inJs(c.ids[1], 2.2, { x: -30 }), inJs(`${p}o1`, 2.3, { y: 0 }),
    inJs(c.ids[2], 3.4, { x: -30 }), inJs(`${p}o2`, 3.5, { y: 0 }), inJs(`${p}end`, 3.6, { y: 0 }),
    inJs([`${p}il`, `${p}i0`, `${p}i1`], 10.2, { y: 30 }, { stagger: 0.12 }),
    drawJs(`${p}a1`, 11.0, a1.len, 0.5), drawJs(`${p}a2`, 11.2, a2.len, 0.5), inJs(`${p}gl`, 11.5, { y: 10 }),
    outJs(c.ids[0], 12.6, 0.35, 0.4),
    inJs(`${p}say`, 15.2, { y: 20 }),
  ) };
};

// Put vs Put + Sync, from the article's benchmark (4318 and 487682 ns/op)
const fsyncBars = (p) => ({ css: DIAGRAM_CSS, html: join(
  text(`${p}w1`, 0, 150, "Put", "say", { "font-size": 44 }),
  bar(`${p}b1`, 0, 215, 12, 70),
  text(`${p}v1`, 36, 218, "~4,3 µs", "cream", { "font-size": 52 }),
  text(`${p}w2`, 0, 350, "Put + Sync", "say", { "font-size": 44 }),
  bar(`${p}b2`, 0, 415, 960, 70),
  text(`${p}v2`, 0, 505, "~488 µs", "", { "font-size": 52 }),
  text(`${p}x`, 300, 510, "lebih dari 100 kali lebih lama", "say mute", { "font-size": 36 }),
  box(`${p}f`, 0, 640, 960, 120, `<span class="dn" style="font-size: 46px">programnya yang memanggil Sync</span>`, "fill"),
), js: join(
  inJs(`${p}w1`, 1.2, { y: 20 }), growJs(`${p}b1`, 1.5, 0.3), inJs(`${p}v1`, 1.7, { x: -20 }),
  inJs(`${p}w2`, 5.2, { y: 20 }), growJs(`${p}b2`, 5.5, 1.6), inJs(`${p}v2`, 7.0, { y: 10 }),
  inJs(`${p}x`, 10.2, { y: 10 }),
  inJs(`${p}f`, 15.2, { y: 30 }),
) });

// a map has no order (every key checked, then sorted); a sorted index finds the first key and reads right
const sorted = (p) => {
  const mp = cells(`${p}m`, 0, 120, ["user:4", "", "user:1", "user:5", "user:2", "user:3"].map((t) => ({ t })), { w: 163, h: 96 });
  const so = cells(`${p}s`, 0, 470, ["user:1", "user:2", "user:3", "user:4", "user:5"].map((t) => ({ t })), { w: 195, h: 96 });
  const r = arrow(so.xs[1] + 40, 610, so.xs[3] + 170, 610);
  return { css: DIAGRAM_CSS, html: join(
    text(`${p}ml`, 0, 70, "map Go: urutannya gak ditentukan", "up mute"),
    mp.html,
    text(`${p}mx`, 0, 250, "semua diperiksa, lalu diurutkan: kerja sebanding jumlah SEMUA key", "say mute", { "font-size": 28, width: 960 }),
    text(`${p}sl`, 0, 420, "index yang urut", "up mute"),
    so.html,
    svg(960, 820, [{ id: `${p}r`, d: r.d }]),
    text(`${p}rl`, so.xs[1] + 40, 630, "cari user:2, lalu baca ke kanan", "cream"),
    text(`${p}sx`, 0, 720, "kerja sebanding jumlah key yang diminta", "say mute", { "font-size": 28, width: 960 }),
  ), js: join(
    inJs(`${p}ml`, 0.4, { y: 10 }), inJs(mp.ids, 0.6, { scale: 0.6 }, { stagger: 0.07 }),
    mp.ids.filter((_, i) => i !== 1).map((id, i) => hotJs(id, 5.2 + i * 0.15, { text: true })),
    inJs(`${p}mx`, 6.2, { y: 20 }),
    inJs(`${p}sl`, 10.0, { y: 10 }), inJs(so.ids, 10.2, { y: 30 }, { stagger: 0.08 }),
    drawJs(`${p}r`, 11.4, r.len, 0.7), inJs(`${p}rl`, 11.6, { y: 10 }),
    [1, 2, 3].map((i) => hotJs(so.ids[i], 11.5 + (i - 1) * 0.2, { text: true })),
    inJs(`${p}sx`, 12.6, { y: 20 }),
  ) };
};

// one record: a 12-byte header, key and value; then the log after three Puts
const record = (p) => {
  const f = [["CRC-32C", "4 byte", 0, 190], ["panjang key", "4 byte", 187, 210], ["panjang value", "4 byte", 394, 240],
    ["key", "", 631, 160], ["value", "", 788, 172]];
  const lg = cells(`${p}c`, 0, 520, [{ t: "user:1 Andi" }, { t: "user:2 Budi" }, { t: "user:1 Andi Wijaya", w: 360 }], { w: 300, h: 100 });
  return { css: DIAGRAM_CSS, html: join(
    [0, 4, 8, 12].map((o, i) => text(`${p}o${i}`, [0, 187, 394, 631][i] + 6, 70, String(o), "mute")),
    f.map(([a, b, x, w], i) => box(`${p}f${i}`, x, 110, w, 130, `${nm(a)}${b ? sm(b) : ""}`, i === 0 ? "hot" : "")),
    svg(960, 820, [{ id: `${p}cb`, d: "M187 258 L187 278 L960 278 L960 258" }]),
    text(`${p}cl`, 420, 292, "dicakup CRC-32C"),
    text(`${p}ll`, 0, 470, "file log setelah tiga Put", "up mute"),
    lg.html,
    [0, 22, 44].map((o, i) => text(`${p}p${i}`, lg.xs[i] + 6, 636, String(o), "mute")),
    vline(`${p}end`, lg.end + 2, 500, 140),
    text(`${p}endl`, 880, 636, "73", "cream"),
    text(`${p}old`, 0, 700, "record lama gak pernah diubah", "say mute", { "font-size": 30 }),
  ), js: join(
    inJs(f.map((_, i) => `${p}f${i}`), 0.4, { y: 30 }, { stagger: 0.12 }),
    inJs([0, 1, 2, 3].map((i) => `${p}o${i}`), 0.6, { y: 10 }, { stagger: 0.12 }),
    drawJs(`${p}cb`, 6.0, 820, 0.6), inJs(`${p}cl`, 6.4, { y: 10 }),
    inJs(`${p}ll`, 9.8, { y: 10 }),
    lg.ids.map((id, i) => inJs([id, `${p}p${i}`], 10.0 + i * 0.5, { x: -30 })),
    inJs([`${p}end`, `${p}endl`], 11.6, { y: 0 }),
    outJs(lg.ids[0], 12.4, 0.35, 0.4), inJs(`${p}old`, 12.6, { y: 20 }),
    hotJs(lg.ids[2], 16.0, { text: true }),
  ) };
};

// opening the log gets slower as it grows (article: ~46 ms for 100,000 keys, 558 ms and 0.604 s for a million)
const openBars = (p) => {
  const row = (i, label, w, val, at) => [
    text(`${p}w${i}`, 0, 60 + i * 175, label, "say", { "font-size": 40 }),
    bar(`${p}b${i}`, 0, 118 + i * 175, w, 56),
    w > 700
      ? text(`${p}v${i}`, 0, 124 + i * 175, val, "", { "font-size": 36, width: w - 24, "text-align": "right", color: "#111111", "z-index": "3" })
      : text(`${p}v${i}`, w + 24, 122 + i * 175, val, "cream", { "font-size": 40 }),
  ];
  return { css: DIAGRAM_CSS, html: join(
    row(0, "buka 100.000 key (benchmark)", 73, "~46 ms"),
    row(1, "buka 1 juta key", 887, "~558 ms"),
    row(2, "satu Get di file 1 juta key", 960, "~0,6 detik, sebagai program terpisah"),
    box(`${p}f`, 0, 660, 960, 120, `<span class="dn" style="font-size: 40px">kill -9: 481.805 key, semuanya selamat</span>`, "fill"),
  ), js: join(
    inJs(`${p}w0`, 0.6, { y: 20 }), growJs(`${p}b0`, 0.9, 0.3), inJs(`${p}v0`, 1.1, { x: -20 }),
    inJs(`${p}w1`, 5.0, { y: 20 }), growJs(`${p}b1`, 5.3, 1.4), inJs(`${p}v1`, 6.6, { y: 10 }),
    inJs(`${p}w2`, 9.6, { y: 20 }), growJs(`${p}b2`, 9.9, 1.4), inJs(`${p}v2`, 11.2, { y: 10 }),
    inJs(`${p}f`, 14.2, { y: 30 }),
  ) };
};

const { total } = buildWide({
  series: "bikin database key-value sendiri · bagian 1",
  part: "01 / 06",
  scenes: [
    { type: "title", reg: "orange", dur: 7, still: true, chapter: "Pembuka: apa itu database key-value",
      kicker: "seri baru · bagian 1 dari 6",
      lines: [{ text: "bikin database", size: 170, light: true }, { text: "key-value sendiri", size: 170 }],
      lead: "pakai Go, cuma library standar. bagian 1: kenapa map di memori gak cukup." },
    { type: "split", dur: 13, kicker: "apa itu database key-value?", title: "key dan value", visual: ops("s02_"),
      notes: [
        { at: 1.0, html: "database yang nyimpan pasangan key dan value" },
        { at: 4.5, html: `${m("key")}: nama untuk mencari data. ${m("value")}: datanya, isinya gak dilihat` },
        { at: 8.6, html: `tiga operasi: ${m("Put")}, ${m("Get")}, dan ${m("Range")}` }] },
    { type: "split", dur: 12, kicker: "rencananya", title: "seri enam bagian", visual: parts("s03_"), chapter: "Rencana enam bagian",
      notes: [
        { at: 1.0, html: "database key-value kecil dengan Go, dari nol" },
        { at: 4.5, html: `namanya ${m("kvku")}, cuma pakai library standar Go` },
        { at: 8.0, html: "di akhir seri: key urut di B+tree, di disk" }] },
    { type: "split", dur: 20, kicker: "masalahnya", title: "map hilang saat restart", visual: restart("s04_"), chapter: "Kenapa map di memori gak cukup",
      notes: [
        { at: 1.0, html: `${m("map[string][]byte")} sudah penyimpanan key-value` },
        { at: 5.5, html: "dan paling cepat: satu Get cuma puluhan nanodetik" },
        { at: 10.5, html: "tapi isinya cuma ada selama programnya jalan" },
        { at: 15.5, html: `${m("crash")}: program berhenti karena error yang gak ditangani. isi map ikut hilang` }] },
    { type: "split", dur: 20, kicker: "pola yang sama dengan Redis", title: "data di log, map jadi index", visual: logIndex("s05_"), chapter: "Log di disk, map sebagai index",
      notes: [
        { at: 1.0, html: "Redis juga nyimpan semua datanya di memori" },
        { at: 5.5, html: `tapi tetap nulis ke disk: ${m("RDB")} dan ${m("AOF")}` },
        { at: 10.0, html: `kvku: data di ${m("log")}, map cuma jadi ${m("index")}` },
        { at: 15.0, html: "kelemahannya: tiap mulai, index dibangun ulang dari seluruh log" }] },
    { type: "split", dur: 20, kicker: "kapan data aman?", title: "write belum tentu sampai disk", visual: fsyncBars("s06_"), chapter: "Kapan data sampai ke disk",
      notes: [
        { at: 1.0, html: `${m("write")} yang berhasil: data baru masuk memori OS` },
        { at: 5.0, html: `${m("fsync")} maksa data sampai ke disk. di Go: method ${m("Sync")}` },
        { at: 10.0, html: "di laptop: ~4,3 µs lawan ~488 µs per Put" },
        { at: 15.0, html: "Redis bawaannya fsync tiap detik. kvku nyerahin pilihannya ke program" }] },
    { type: "split", dur: 19, kicker: "satu masalah lagi", title: "map gak nyimpan urutan", visual: sorted("s07_"), chapter: "Kenapa index harus urut",
      notes: [
        { at: 1.0, html: `Range ${m("user:2")} sampai ${m("user:5")}: semua key di antaranya, urut` },
        { at: 5.0, html: "di map: semua key diperiksa, lalu diurutkan" },
        { at: 10.0, html: "index yang urut: cari key pertama, lalu baca ke kanan" },
        { at: 14.5, html: `${m("Bitcask")} (Riak, 2010) juga log + hash table: urutannya gak ada` }] },
    { type: "statement", reg: "orange", dur: 7, chapter: "Anatomi record",
      lines: [{ text: "anatomi kvku", size: 230 }],
      lead: "satu file log di disk, satu map di memori sebagai index. tiap Put jadi satu record.", leadAt: 1.4 },
    { type: "split", dur: 21, kicker: "anatomi satu record", title: "header 12 byte, lalu data", visual: record("s09_"),
      notes: [
        { at: 1.0, html: `${m("CRC-32C")} 4 byte, ${m("panjang key")} 4 byte, ${m("panjang value")} 4 byte` },
        { at: 5.5, html: "CRC-32C: angka pemeriksa. satu byte berubah, angkanya hampir pasti beda" },
        { at: 10.0, html: "Put yang menimpa key: record lama tetap di file" },
        { at: 15.5, html: "Open baca ulang semua record. record setengah jadi di ujung: file dipotong" }] },
    { type: "split", dur: 18, kicker: "dari terminal", title: "tiap perintah, satu run", chapter: "Mencoba kvku dari terminal",
      cards: [{ head: "git bash · file data.kv", lines: [
        { cmd: "kvku put user:1 Andi", at: 0.8 },
        { cmd: "kvku put user:2 Budi", at: 2.0 },
        { cmd: "kvku put user:1 \"Andi Wijaya\"", at: 3.2 },
        { cmd: "kvku get user:1", at: 5.4 },
        { out: "Andi Wijaya", at: 6.3 },
        { cmd: "kvku stats", at: 10.0 },
        { out: "jumlah key  : 2", at: 10.7 },
        { out: "ukuran file : 73 byte", at: 10.9 }] }],
      notes: [
        { at: 1.0, html: "tiap baris: satu run program yang terpisah" },
        { at: 6.5, line: 4, html: `${m("Andi Wijaya")} terbaca dari Open yang membaca ulang log` },
        { at: 11.5, line: 7, html: `${m("73 byte")}: tiga record, termasuk record lama Andi` }] },
    { type: "split", dur: 18, kicker: "satu juta key", title: "buka file makin lama", visual: openBars("s11_"), chapter: "Satu juta key, dan kill -9",
      notes: [
        { at: 1.0, html: `${m("fill 1000000")}: file 40 MB` },
        { at: 5.0, html: "tiap buka, seluruh log dibaca: lebih dari setengah detik" },
        { at: 9.6, html: "satu Get pun harus nunggu selama itu" },
        { at: 14.0, html: `${m("kill -9")} setelah 2 detik: semua key yang ditulis masih ada` }] },
    { type: "bullets", dur: 13, kicker: "kelemahan versi pertama", title: "yang belum beres", chapter: "Kelemahan dan bagian berikutnya",
      items: [
        { at: 1.5, text: "waktu buka naik seiring ukuran file" },
        { at: 5.0, text: "semua key harus muat di memori" },
        { at: 8.5, text: "Range memeriksa semua key (~16 ms)" }] },
    { type: "title", reg: "orange", dur: 10,
      kicker: "lanjut bagian 2",
      lines: [{ text: "B+tree: struktur", size: 180, light: true }, { text: "dan pencarian", size: 180 }],
      lead: "Range urut tanpa memeriksa semua key. artikel lengkapnya, dengan kode Go dan test-nya: link di deskripsi.", leadAt: 1.4 },
  ],
});
console.log("bagian 1 (YouTube):", total, "s");
