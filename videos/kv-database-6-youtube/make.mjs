// Builds the YouTube (16:9, ~3 min, captions only) video for "Membuat Database Key-Value Sendiri" Bagian 6, the series
// finale: kvku measured side by side with bbolt and Badger on the same laptop (three designs, a fair setup, the results,
// Badger's SyncWrites on Windows, file sizes), the Range fix that made it ten times faster, kvku's weaknesses, when to use
// SQLite, bbolt, Badger or Pebble, and the series recap. The story, numbers and diagrams follow the article
// (docs/articles/membuat-database-key-value-sendiri/seri-...-bagian-6-...md). Run: node make.mjs.
// Scene kit: ../kv-database-shared/wide.mjs and diagrams.mjs.
import { buildWide } from "../kv-database-shared/wide.mjs";
import {
  DIAGRAM_CSS, box, text, nm, sm, arrow, svg, bar, join,
  inJs, drawJs, hotJs, growJs,
} from "../kv-database-shared/diagrams.mjs";

const m = (s) => `<span class="m">${s}</span>`;

// five parts so far
const recap = (p) => {
  const rows = [["1", "log di disk, map sebagai index"], ["2", "B+tree di memori"], ["3", "halaman 16 KB di file"], ["4", "copy-on-write, dua meta, tombstone"], ["5", "free list, compaction, cache LRU"]];
  return { css: DIAGRAM_CSS, html: join(rows.map(([a, b], i) => box(`${p}r${i}`, 0, 60 + i * 140, 960, 116, `${nm(`bagian ${a} · ${b}`)}`, i === 4 ? "l hot" : "l"))),
    js: inJs(rows.map((_, i) => `${p}r${i}`), 0.6, { x: 60 }, { stagger: 0.25 }) };
};

// three ways to store key-value data
const designs = (p) => {
  const cols = [["kvku", ["file index: B+tree, halaman 16 KB", "file data: value, cuma bertambah", "dibaca dengan ReadAt"], "hot"],
    ["bbolt", ["satu file: B+tree", "value di dalam leaf", "dibaca lewat mmap"], ""],
    ["Badger", ["memtable di memori + WAL", "SSTable bertingkat, urut", "value log untuk value besar"], ""]];
  return { css: DIAGRAM_CSS, html: join(cols.map(([t, ls, c], i) => join(
    box(`${p}c${i}`, i * 325, 40, 310, 110, nm(t), c === "hot" ? "fill" : ""),
    ls.map((l, k) => box(`${p}c${i}l${k}`, i * 325, 170 + k * 170, 310, 150, `<span class="dn" style="font-size: 28px">${l}</span>`, c, { padding: "0 18px" })),
  ))), js: join(cols.map((_, i) => inJs([`${p}c${i}`, `${p}c${i}l0`, `${p}c${i}l1`, `${p}c${i}l2`], [0.6, 5.0, 10.0][i], { y: 30 }, { stagger: 0.12 }))) };
};

// grouped bars: groups [{ t, unit, rows: [[name, value number, label]] }], each group with its own scale
const groups = (p, gs, { y0 = 0, gh = 205, at = [] } = {}) => {
  const names = ["kvku", "bbolt", "badger"];
  return {
    html: join(gs.map((g, k) => {
      const max = Math.max(...g.rows.map((r) => r[0]));
      return join(
        text(`${p}g${k}`, 0, y0 + k * gh, g.t, "mute", { "font-size": 26 }),
        g.rows.map(([v, l], i) => {
          const w = Math.max(6, Math.round(480 * v / max));
          return join(
            text(`${p}n${k}${i}`, 0, y0 + k * gh + 40 + i * 52, names[i], i === 0 ? "" : "cream", { "font-size": 28 }),
            bar(`${p}b${k}${i}`, 140, y0 + k * gh + 40 + i * 52, w, 40, i === 0 ? "" : "mute"),
            text(`${p}v${k}${i}`, 140 + w + 16, y0 + k * gh + 42 + i * 52, l, i === 0 ? "" : "cream", { "font-size": 30 }),
          );
        }),
      );
    })),
    js: join(gs.map((g, k) => join(
      inJs(`${p}g${k}`, at[k], { y: 10 }),
      g.rows.map((_, i) => join(inJs(`${p}n${k}${i}`, at[k] + 0.1 + i * 0.12, { x: -20 }), growJs(`${p}b${k}${i}`, at[k] + 0.2 + i * 0.12, 0.5),
        inJs(`${p}v${k}${i}`, at[k] + 0.6 + i * 0.12, { x: -10 }))),
    ))),
  };
};

const results = (p) => {
  const g = groups(p, [
    { t: "1.000 Put, satu commit · ms", rows: [[9.74, "9,74"], [3.80, "3,80"], [2.06, "2,06"]] },
    { t: "Get acak · µs", rows: [[7.4, "7,4"], [2.0, "2,0"], [3.6, "3,6"]] },
    { t: "Range 100 key · µs", rows: [[26.8, "26,8"], [5.8, "5,8"], [183.4, "183,4"]] },
    { t: "Open, Get, Close · ms", rows: [[0.43, "0,43"], [9.0, "9,0"], [59.4, "59,4"]] },
  ], { y0: 0, gh: 205, at: [0.6, 5.0, 10.0, 15.0] });
  return { css: DIAGRAM_CSS, html: g.html, js: g.js };
};

// Badger's PutSync on Windows, and why
const badger = (p) => {
  const g = groups(p, [{ t: "Put lalu commit · ms", rows: [[1.71, "1,71"], [1.30, "1,30"], [0.016, "0,016"]] }], { y0: 40, at: [0.6] });
  return { css: DIAGRAM_CSS, html: join(
    g.html,
    box(`${p}code`, 0, 330, 960, 220, `${sm("ristretto · msync untuk Windows")}<span class="dn" style="font-size: 34px; font-family: 'IBM Plex Mono'">// TODO: Figure out how to do<br>// msync on Windows.<br>return nil</span>`, "l hot"),
    text(`${p}say`, 0, 610, "di Windows, SyncWrites Badger gak memaksa data ke disk. angka ini gak bisa dibandingkan", "say", { width: 960, color: "#e85d26" }),
  ), js: join(g.js, inJs(`${p}code`, 9.6, { y: 30 }), inJs(`${p}say`, 14.6, { y: 20 })) };
};

// file sizes for one million keys, then after rewriting every key
const sizes = (p) => {
  const g = groups(p, [
    { t: "1 juta key · MB", rows: [[169, "169"], [512, "512, terpakai 264"], [112, "112"]] },
    { t: "setelah semua key ditulis ulang · MB", rows: [[288, "288"], [512, "512"], [223, "223"]] },
  ], { y0: 80, gh: 300, at: [0.6, 5.6] });
  return { css: DIAGRAM_CSS, html: g.html, js: g.js };
};

// Range: one ReadAt per key, then one ReadAt for adjacent records
const range = (p) => {
  const recs = Array.from({ length: 8 }, (_, i) => box(`${p}r${i}`, i * 118, 470, 112, 80, sm(`k${i + 1}`), "", { "font-size": 26 }));
  return { css: DIAGRAM_CSS, html: join(
    text(`${p}l`, 0, 30, "Range 100 key · µs", "mute", { "font-size": 26 }),
    text(`${p}w0`, 0, 80, "kvku bagian 5", "say", { "font-size": 34 }),
    bar(`${p}b0`, 0, 130, 960, 56, "mute"),
    text(`${p}v0`, 0, 136, "~270", "", { "font-size": 38, width: 936, "text-align": "right", color: "#f0ece5", "z-index": "3" }),
    text(`${p}w1`, 0, 220, "kvku bagian 6", "say", { "font-size": 34 }),
    bar(`${p}b1`, 0, 270, 96, 56),
    text(`${p}v1`, 120, 272, "~27", "", { "font-size": 44 }),
    text(`${p}rl`, 0, 410, "record bersebelahan di file data", "mute", { "font-size": 26 }),
    recs,
    box(`${p}one`, 0, 570, 942, 70, `<span class="dn" style="font-size: 32px">satu ReadAt, paling banyak 64 KB</span>`, "fill"),
    text(`${p}say`, 0, 680, "tiap record tetap diperiksa CRC dan key-nya", "say mute", { width: 960 }),
  ), js: join(
    inJs(`${p}l`, 0.5, { y: 10 }), inJs(`${p}w0`, 0.7, { y: 20 }), growJs(`${p}b0`, 1.0, 1.0), inJs(`${p}v0`, 1.9, { y: 10 }),
    inJs(`${p}rl`, 5.0, { y: 10 }), inJs([...Array(8).keys()].map((i) => `${p}r${i}`), 5.2, { y: 20 }, { stagger: 0.25 }),
    inJs(`${p}one`, 10.0, { scaleX: 0 }),
    inJs(`${p}w1`, 12.0, { y: 20 }), growJs(`${p}b1`, 12.3, 0.3), inJs(`${p}v1`, 12.6, { x: -20 }),
    inJs(`${p}say`, 15.4, { y: 20 }),
  ) };
};

// kvku's weaknesses as a whole
const weak = (p) => {
  const rows = ["tiga fsync per commit", "satu system call per Put", "ReadAt, tanpa mmap", "penulis menunggu compaction", "tanpa transaksi, tanpa kunci folder", "cuma diuji di Windows"];
  return { css: DIAGRAM_CSS, html: join(rows.map((t, i) => box(`${p}r${i}`, 0, 20 + i * 132, 960, 112, nm(t), "l"))),
    js: join(inJs(rows.map((_, i) => `${p}r${i}`), 0.6, { x: 60 }, { stagger: 0.3 }), hotJs(`${p}r1`, 5.2), hotJs(`${p}r4`, 10.2)) };
};

// which database to use
const choose = (p) => {
  const rows = [["SQLite", "datamu punya bentuk: SQL, index, transaksi, satu file"], ["bbolt", "key-value, bacaan jauh lebih banyak dari tulisan"],
    ["Badger", "tulisan banyak, data besar"], ["Pebble", "LSM tree di produksi besar, mesin CockroachDB"]];
  return { css: DIAGRAM_CSS, html: join(rows.map(([a, b], i) => box(`${p}r${i}`, 0, 20 + i * 190, 960, 164, `${nm(a)}${sm(b)}`, "l"))),
    js: join(rows.map((_, i) => inJs(`${p}r${i}`, [0.6, 4.0, 7.4, 10.8][i], { x: 60 }))) };
};

// the series: six parts, each born from the previous part's weakness
const series = (p) => {
  const t = [["1", "log dan map"], ["2", "B+tree di memori"], ["3", "halaman 16 KB"], ["4", "copy-on-write"], ["5", "compaction"], ["6", "mengukur"]];
  const X = [0, 335, 670], W = 290, H = 170;
  const pos = t.map((_, i) => [X[i % 3], i < 3 ? 120 : 420]);
  const ar = [0, 1, 3, 4].map((i) => arrow(pos[i][0] + W + 6, pos[i][1] + H / 2, pos[i + 1][0] - 8, pos[i + 1][1] + H / 2, 14));
  const turn = { d: `M${X[2] + W / 2} ${120 + H + 6} L${X[2] + W / 2} 370 L${W / 2} 370 L${W / 2} 412 M${W / 2} 412 L${W / 2 - 12} 394 M${W / 2} 412 L${W / 2 + 12} 394` };
  return { css: DIAGRAM_CSS, html: join(
    svg(960, 820, [...ar.map((a, i) => ({ id: `${p}a${i}`, d: a.d, cls: "cream thin" })), { id: `${p}t`, d: turn.d, cls: "cream thin" }]),
    t.map(([n, l], i) => box(`${p}b${i}`, pos[i][0], pos[i][1], W, H, `${sm(`bagian ${n}`)}${nm(l)}`, i === 5 ? "fill" : "")),
    text(`${p}say`, 0, 670, "tiap bagian lahir dari kelemahan bagian sebelumnya", "say mute", { width: 960 }),
  ), js: join(
    [0, 1, 2, 3, 4, 5].map((i) => inJs(`${p}b${i}`, 0.5 + i * 0.7, { y: 20 })),
    ar.map((a, k) => drawJs(`${p}a${k}`, [0.9, 1.6, 3.0, 3.7][k], a.len, 0.3)), drawJs(`${p}t`, 2.3, 900, 0.4),
    inJs(`${p}say`, 8.4, { y: 20 }),
  ) };
};

const { total } = buildWide({
  series: "bikin database key-value sendiri · bagian 6",
  part: "06 / 06",
  scenes: [
    { type: "title", reg: "orange", dur: 7, still: true, chapter: "Pembuka: seberapa jauh kvku?",
      kicker: "bikin database key-value sendiri · bagian 6",
      lines: [{ text: "kvku lawan", size: 180, light: true }, { text: "bbolt dan Badger", size: 180 }],
      lead: "bagian terakhir: mengukur, membandingkan, lalu menutup seri." },
    { type: "split", dur: 13, kicker: "lima bagian", title: "kvku sudah lengkap", visual: recap("s02_"),
      notes: [
        { at: 1.0, html: "test cuma menjawab: apakah kvku benar?" },
        { at: 4.6, html: "bagian 6: seberapa jauh dari database sungguhan?" },
        { at: 8.6, html: "pembandingnya bbolt dan Badger, ditulis dengan Go" }] },
    { type: "split", dur: 20, kicker: "tiga desain", title: "B+tree, atau LSM tree?", visual: designs("s03_"), chapter: "Tiga cara menyimpan key-value",
      notes: [
        { at: 1.0, html: "kvku: B+tree seperti bbolt, value terpisah seperti Badger" },
        { at: 5.0, html: "bbolt: B+tree copy-on-write, awalnya port LMDB ke Go" },
        { at: 10.0, html: `Badger: ${m("LSM tree")}, tulisan ditampung di ${m("memtable")}` },
        { at: 15.0, html: `value 100 byte tetap di LSM: ${m("ValueThreshold")} 1 MB` }] },
    { type: "split", dur: 13, kicker: "cara mengukur yang adil", title: "beban yang sama", chapter: "Cara mengukur yang adil",
      cards: [{ head: "/banding/go.mod", lines: [
        { conf: "module banding", at: 0.6 },
        { blank: true },
        { conf: "require (", at: 0.8 },
        { conf: "  github.com/dgraph-io/badger/v4 v4.9.6", at: 1.0 },
        { conf: "  go.etcd.io/bbolt v1.4.0", at: 1.2 },
        { conf: ")", at: 1.4 },
        { conf: "replace kvku => ../bagian-6", at: 1.6 }] }],
      notes: [
        { at: 1.0, html: "value 100 byte, tiap commit harus aman di disk" },
        { at: 4.6, line: 3, html: `Badger dibuka dengan ${m("SyncWrites")}, bawaannya false` },
        { at: 8.6, line: 6, html: "module terpisah: kvku tetap cuma library standar" }] },
    { type: "statement", reg: "orange", dur: 7, chapter: "Hasil: Put, Get, Range, Open",
      lines: [{ text: "hasilnya", size: 230 }],
      lead: "satu run, satu laptop: AMD Ryzen 7 5700U, RAM ~7,4 GB, SSD, Windows.", leadAt: 1.4 },
    { type: "split", dur: 20, kicker: "kvku, bbolt, badger", title: "kvku menang di Open", visual: results("s06_"),
      notes: [
        { at: 1.0, html: "1.000 Put: satu WriteAt per Put, bbolt menunggu commit" },
        { at: 5.0, html: "Get: selisihnya satu ReadAt ke file data" },
        { at: 10.0, html: "Range: iterator LSM menggabung memtable dan SSTable" },
        { at: 15.0, html: "Open: kvku cuma baca halaman 0 dan free list" }] },
    { type: "split", dur: 18, kicker: "angka yang paling mencolok", title: "Badger: 16 µs per commit?", visual: badger("s07_"), chapter: "Badger di Windows gak menunggu disk",
      notes: [
        { at: 1.0, html: "~80 kali lebih cepat dari bbolt" },
        { at: 4.6, html: "fsync di laptop ini butuh ratusan µs. gak mungkin" },
        { at: 9.6, html: `SyncWrites memanggil ${m("msync")} dari ristretto` },
        { at: 14.6, html: "angka Badger di Linux: pertanyaan terbuka" }] },
    { type: "split", dur: 13, kicker: "ukuran file", title: "satu juta key", visual: sizes("s08_"), chapter: "Ukuran file",
      notes: [
        { at: 1.0, html: "Badger paling kecil: SSTable ditulis padat" },
        { at: 4.6, html: "bbolt memperbesar file dulu sampai 512 MB" },
        { at: 8.6, html: "kvku 288 MB: sampah 50%, belum lewat ambang 75%" }] },
    { type: "split", dur: 20, kicker: "perbaikan setelah mengukur", title: "Range sepuluh kali lebih cepat", visual: range("s09_"), chapter: "Range sepuluh kali lebih cepat",
      notes: [
        { at: 1.0, html: "Range pertama hampir 50 kali bbolt: satu ReadAt per key" },
        { at: 5.0, html: "record key berurutan sering bersebelahan di file data" },
        { at: 10.0, html: "jadi dibaca sekaligus dengan satu ReadAt" },
        { at: 15.0, html: "dari ~270 jadi ~27 µs. masih ~4,5 kali bbolt" }] },
    { type: "split", dur: 16, kicker: "setelah enam bagian", title: "kelemahan kvku", visual: weak("s10_"), chapter: "Kelemahan kvku",
      notes: [
        { at: 1.0, html: "benar dan tahan listrik padam, kodenya bisa dibaca dalam satu sore" },
        { at: 5.2, html: "1.000 Put dalam satu commit ~2,5 kali lebih lambat dari bbolt" },
        { at: 10.2, html: "beberapa Put gak bisa berhasil atau gagal bersama" }] },
    { type: "split", dur: 16, kicker: "kvku dibuat untuk belajar", title: "untuk data sungguhan", visual: choose("s11_"), chapter: "Kapan SQLite, bbolt, Badger, atau Pebble",
      notes: [
        { at: 1.0, html: "SQLite bersaing dengan fopen(), bukan dengan database server" },
        { at: 5.6, html: "pilihannya tergantung bentuk data dan cara ia ditulis" },
        { at: 10.8, html: "sudah punya PostgreSQL? tabel dua kolom sering cukup" }] },
    { type: "split", dur: 13, kicker: "rekap seri", title: "enam bagian, satu kvku", visual: series("s12_"), chapter: "Rekap enam bagian",
      notes: [
        { at: 1.0, html: "dimulai dari: kenapa map di memori gak cukup?" },
        { at: 4.6, html: "semua kode cuma pakai library standar Go" },
        { at: 8.6, html: "diuji dengan go test, -race, dan try-out" }] },
    { type: "bullets", dur: 13, kicker: "mau lanjut sendiri?", title: "latihan pertama", chapter: "Penutup seri",
      items: [
        { at: 1.5, text: "kumpulkan record di memori sampai commit" },
        { at: 5.0, text: "lihat hasilnya di benchmark Put1000" },
        { at: 8.5, text: "lalu pilih kelemahan berikutnya di daftar tadi" }] },
    { type: "title", reg: "orange", dur: 10,
      kicker: "akhir seri · bikin database key-value sendiri",
      lines: [{ text: "alasan di balik", size: 180, light: true }, { text: "tiap pilihan.", size: 180 }],
      lead: "itu yang paling berharga dari seri ini. makasih sudah ngikutin enam bagian. artikel lengkapnya, dengan kode Go dan test-nya: link di deskripsi.", leadAt: 1.6 },
  ],
});
console.log("bagian 6 (YouTube):", total, "s");
