// Builds the YouTube (16:9, ~3 min, captions only) video for "Membuat Database Key-Value Sendiri" Bagian 5: where the
// garbage comes from, the free list with its one-commit wait, keeping the free list in the file, the 75% compaction
// threshold, compaction that does not stop readers (two locks), the test numbers, the LRU cache, 9 GB of data on a
// ~7.4 GB RAM laptop, and mmap weighed again with numbers. The story, numbers and diagrams follow the article
// (docs/articles/membuat-database-key-value-sendiri/seri-...-bagian-5-...md). Run: node make.mjs.
// Scene kit: ../kv-database-shared/wide.mjs and diagrams.mjs.
import { buildWide } from "../kv-database-shared/wide.mjs";
import {
  DIAGRAM_CSS, box, text, nm, sm, cells, arrow, svg, bar, join,
  inJs, drawJs, hotJs, growJs,
} from "../kv-database-shared/diagrams.mjs";

const m = (s) => `<span class="m">${s}</span>`;

// Bagian 4 left garbage in two places
const garbage = (p) => ({ css: DIAGRAM_CSS, html: join(
  text(`${p}il`, 0, 60, "file index · test bagian 4", "up mute"),
  text(`${p}in`, 0, 110, "2.093", "stat", { "font-size": 180 }),
  text(`${p}iu`, 480, 180, "halaman untuk 2.609 key", "say", { "font-size": 40, width: 480 }),
  text(`${p}dl`, 0, 380, "file data · try-out bagian 4", "up mute"),
  text(`${p}dn`, 0, 430, "29 / 88", "stat", { "font-size": 180 }),
  text(`${p}du`, 600, 500, "byte masih hidup", "say", { "font-size": 40, width: 360 }),
  text(`${p}s`, 0, 690, "halaman berukuran sama: cukup dicatat, lalu dipakai lagi. record beda panjang: dipadatkan", "say mute", { width: 960 }),
), js: join(
  inJs(`${p}il`, 0.5, { y: 10 }), inJs(`${p}in`, 0.8, { y: 30 }), inJs(`${p}iu`, 1.2, { x: -30 }),
  inJs(`${p}dl`, 4.6, { y: 10 }), inJs(`${p}dn`, 4.9, { y: 30 }), inJs(`${p}du`, 5.3, { x: -30 }),
  inJs(`${p}s`, 8.8, { y: 20 }),
) });

// page 5's life: used by commit 7, left by commit 8 (pending), free in commit 9, reused by a Put
const freelist = (p) => {
  const st = [["commit 7", "hal. 5 dipakai tree", "ditunjuk meta A", ""], ["commit 8", "hal. 5 ditinggalkan", "masuk pending", "dash"],
    ["commit 9", "hal. 5 di free list", "boleh dipakai lagi", "hot"], ["commit 9 · Put", "leaf baru", "ditulis di hal. 5", "fill"]];
  const ar = [0, 1, 2].map((i) => arrow(470, 190 + i * 160, 470, 222 + i * 160, 12));
  return { css: DIAGRAM_CSS, html: join(
    svg(960, 820, ar.map((a, i) => ({ id: `${p}a${i}`, d: a.d, cls: "cream thin" }))),
    st.map(([a, b, c, cls], i) => box(`${p}s${i}`, 0, 60 + i * 160, 960, 128, `${nm(`${a} · ${b}`)}${sm(c)}`, cls)),
    text(`${p}say`, 0, 720, "free list disimpan di halaman file index, ditunjuk meta", "say mute", { width: 960 }),
  ), js: join(
    inJs(`${p}s0`, 0.6, { y: 30 }),
    drawJs(`${p}a0`, 4.6, 60, 0.3), inJs(`${p}s1`, 4.8, { y: 30 }),
    drawJs(`${p}a1`, 9.6, 60, 0.3), inJs(`${p}s2`, 9.8, { y: 30 }),
    drawJs(`${p}a2`, 12.6, 60, 0.3), inJs(`${p}s3`, 12.8, { y: 30 }),
    inJs(`${p}say`, 15.4, { y: 20 }),
  ) };
};

// 50% (Badger's suggestion) vs 75% (kvku): how big the data file may grow, and how often a value is written
const threshold = (p) => {
  const rows = [["g = 50% · saran Badger", [["file data paling besar", 240, "2× data hidup"], ["byte ditulis seumur value", 480, "~2×"]], "mute"],
    ["g = 75% · kvku", [["file data paling besar", 480, "4× data hidup"], ["byte ditulis seumur value", 320, "~1,33×"]], ""]];
  return { css: DIAGRAM_CSS, html: join(
    rows.map(([t, bars, cls], r) => join(
      text(`${p}t${r}`, 0, 36 + r * 340, t, r ? "" : "mute", { "font-size": 28 }),
      bars.map(([l, w, v], i) => join(
        text(`${p}l${r}${i}`, 0, 90 + r * 340 + i * 120, l, "say mute", { "font-size": 28 }),
        bar(`${p}b${r}${i}`, 0, 132 + r * 340 + i * 120, w, 52, cls),
        text(`${p}v${r}${i}`, w + 24, 136 + r * 340 + i * 120, v, r ? "" : "cream", { "font-size": 36 }),
      )),
    )),
    text(`${p}f`, 0, 720, "1 + (1-g) + (1-g)² + ...", "cream", { "font-size": 34 }),
  ), js: join(
    [0, 1].map((r) => join(
      inJs(`${p}t${r}`, r ? 5.0 : 0.6, { y: 10 }),
      [0, 1].map((i) => join(inJs(`${p}l${r}${i}`, (r ? 5.2 : 0.8) + i * 0.6, { y: 10 }), growJs(`${p}b${r}${i}`, (r ? 5.4 : 1.0) + i * 0.6, 0.6),
        inJs(`${p}v${r}${i}`, (r ? 6.0 : 1.6) + i * 0.6, { x: -20 }))),
    )),
    inJs(`${p}f`, 10.4, { y: 20 }),
  ) };
};

// compaction copies the live records to a new file, while readers keep the old tree
const compaction = (p) => {
  const c = cells(`${p}c`, 0, 90, ["a=1", "b=1", "a=2", "c=1", "b=2", "del a", "c=2"].map((t, i) => ({ t, cls: i === 4 || i === 6 ? "" : "dash" })), { w: 140, h: 90 });
  const n = cells(`${p}n`, 0, 290, ["b=2", "c=2"].map((t) => ({ t, cls: "hot" })), { w: 140, h: 90 });
  const a1 = arrow(c.xs[4] + 70, 184, n.xs[0] + 70, 286, 14), a2 = arrow(c.xs[6] + 70, 184, n.xs[1] + 70, 286, 14);
  const lanes = [["pembaca", [[0, 500, "tree lama", "mute"], [520, 440, "tree baru", "mute"]]], ["compaction", [[0, 500, "menyalin tanpa Lock", ""], [520, 130, "commit", ""]]],
    ["penulis", [[0, 640, "menunggu", "dash"]]]];
  return { css: DIAGRAM_CSS, html: join(
    text(`${p}ol`, 0, 40, "data-000001", "mute"),
    c.html, n.html,
    svg(960, 820, [{ id: `${p}a1`, d: a1.d }, { id: `${p}a2`, d: a2.d }]),
    text(`${p}nl`, 300, 320, "data-000002 · urut menurut key", "mute"),
    lanes.map(([l, segs], r) => join(
      text(`${p}ll${r}`, 0, 450 + r * 110, l, "up mute", { "font-size": 22 }),
      segs.map(([x, w, t, cls], i) => box(`${p}g${r}${i}`, x, 485 + r * 110, w, 60, sm(t), cls)),
    )),
    box(`${p}lk`, 500, 470, 20, 300, "", "fill"),
  ), js: join(
    inJs(`${p}ol`, 0.5, { y: 10 }), inJs(c.ids, 0.6, { y: 20 }, { stagger: 0.07 }),
    drawJs(`${p}a1`, 4.6, a1.len, 0.4), drawJs(`${p}a2`, 4.8, a2.len, 0.4),
    inJs(n.ids, 5.2, { y: 20 }, { stagger: 0.1 }), inJs(`${p}nl`, 5.4, { x: -20 }),
    inJs([`${p}ll0`, `${p}g00`, `${p}ll1`, `${p}g10`, `${p}ll2`, `${p}g20`], 10.0, { x: -20 }, { stagger: 0.12 }),
    inJs(`${p}lk`, 13.0, { scaleY: 0 }),
    inJs([`${p}g01`, `${p}g11`], 13.4, { x: -20 }, { stagger: 0.12 }),
  ) };
};

// the compaction test's numbers
const results = (p) => {
  const rows = [["951 ms", "compaction"], ["58.542 Get", "selesai selama itu"], ["2,5 ms", "Get paling lama"], ["941 ms", "Put menunggu"]];
  return { css: DIAGRAM_CSS, html: join(
    text(`${p}l`, 0, 30, "TestPembacaSaatCompaction", "mute", { "font-size": 28 }),
    rows.map(([a, b], i) => join(
      box(`${p}r${i}`, 0, 90 + i * 170, 960, 145, "", i === 3 ? "hot" : ""),
      text(`${p}n${i}`, 36, 108 + i * 170, a, "stat", { "font-size": 96, "z-index": "3", color: i === 3 ? "#e85d26" : "#f0ece5" }),
      text(`${p}t${i}`, 560, 140 + i * 170, b, "say", { "font-size": 38, "z-index": "3" }),
    )),
  ), js: join(
    inJs(`${p}l`, 0.5, { y: 10 }),
    [0, 1, 2, 3].map((i) => inJs([`${p}r${i}`, `${p}n${i}`, `${p}t${i}`], [0.7, 4.8, 8.8, 13.2][i], { x: 50 }, { stagger: 0.08 })),
  ) };
};

// bars: [label, width, value, at]
const barRows = (p, rows, { y0 = 60, gap = 180, inside = 700 } = {}) => ({
  html: join(rows.map(([l, w, v, , cls], i) => join(
    text(`${p}w${i}`, 0, y0 + i * gap, l, "say", { "font-size": 36 }),
    bar(`${p}b${i}`, 0, y0 + 56 + i * gap, w, 56, cls ?? ""),
    w > inside ? text(`${p}v${i}`, 0, y0 + 62 + i * gap, v, "", { "font-size": 36, width: w - 24, "text-align": "right", color: cls === "mute" ? "#f0ece5" : "#111111", "z-index": "3" })
      : text(`${p}v${i}`, w + 24, y0 + 60 + i * gap, v, "cream", { "font-size": 40 }),
  ))),
  js: join(rows.map(([, w, , at], i) => join(inJs(`${p}w${i}`, at, { y: 20 }), growJs(`${p}b${i}`, at + 0.3, w > 300 ? 1.0 : 0.3), inJs(`${p}v${i}`, at + (w > 300 ? 1.2 : 0.6), { y: 10 })))),
});

// Get with a big and a tiny LRU cache
const cache = (p) => {
  const b = barRows(p, [["Get acak · cache 4.096 node", 112, "~6,7 µs", 0.6], ["Get acak · cache 64 node", 960, "~57 µs", 4.6, "mute"]], { y0: 120, gap: 220 });
  return { css: DIAGRAM_CSS, html: join(
    text(`${p}l`, 0, 50, "benchmark · 100.000 key", "up mute"), b.html,
    text(`${p}s`, 0, 600, "cache kecil: leaf harus dibaca lagi dari file, delapan kali lebih lambat", "say mute", { width: 960 }),
  ), js: join(inJs(`${p}l`, 0.4, { y: 10 }), b.js, inJs(`${p}s`, 8.6, { y: 20 })) };
};

// mmap vs ReadAt on random 4 KB reads: from disk (round 1) and from the page cache (round 2)
const mmap = (p) => {
  const panel = (k, y, title, rows, scale, at) => join(
    text(`${p}h${k}`, 0, y, title, "up mute"),
    rows.map(([l, v, num, hot], i) => {
      const w = Math.max(6, Math.round(560 * num / scale));
      return join(
        text(`${p}l${k}${i}`, 0, y + 50 + i * 76, l, "say", { "font-size": 30 }),
        bar(`${p}b${k}${i}`, 300, y + 50 + i * 76, w, 44, hot ? "" : "mute"),
        text(`${p}v${k}${i}`, 300 + w + 18, y + 52 + i * 76, v, hot ? "" : "cream", { "font-size": 32 }),
      );
    }),
  );
  const ids = (k) => [0, 1, 2, 3].map((i) => [`${p}l${k}${i}`, `${p}b${k}${i}`, `${p}v${k}${i}`]).flat();
  return { css: DIAGRAM_CSS, html: join(
    panel(0, 20, "putaran 1 · dari disk · µs per 4 KB", [["1,2 GB · ReadAt", "101,9", 101.9], ["1,2 GB · mmap", "91", 91, 1], ["9,3 GB · ReadAt", "179,7", 179.7], ["9,3 GB · mmap", "259,7", 259.7, 1]], 300),
    panel(1, 410, "putaran 2 · dari page cache · µs per 4 KB", [["1,2 GB · ReadAt", "7,2", 7.2], ["1,2 GB · mmap", "0,6", 0.6, 1], ["9,3 GB · ReadAt", "8,8", 8.8], ["9,3 GB · mmap", "0,5", 0.5, 1]], 10),
  ), js: join(
    inJs(`${p}h0`, 0.5, { y: 10 }), inJs(ids(0), 0.7, { x: -20 }, { stagger: 0.06 }),
    inJs(`${p}h1`, 5.0, { y: 10 }), inJs(ids(1), 5.2, { x: -20 }, { stagger: 0.06 }),
  ) };
};

const { total } = buildWide({
  series: "bikin database key-value sendiri · bagian 5",
  part: "05 / 06",
  scenes: [
    { type: "title", reg: "orange", dur: 7, still: true, chapter: "Pembuka: sampah di kvku",
      kicker: "bikin database key-value sendiri · bagian 5",
      lines: [{ text: "compaction dan data", size: 160, light: true }, { text: "lebih besar dari RAM", size: 160 }],
      lead: "free list, compaction tanpa menghentikan pembaca, lalu data 9 GB di laptop ber-RAM ~7,4 GB." },
    { type: "split", dur: 13, kicker: "setelah bagian 4", title: "sampah di dua tempat", visual: garbage("s02_"),
      notes: [
        { at: 1.0, html: "copy-on-write meninggalkan jalur lama di file index" },
        { at: 4.6, html: "value yang diganti atau dihapus tertinggal di file data" },
        { at: 8.6, html: "dua sampah, dua cara membereskannya" }] },
    { type: "split", dur: 20, kicker: "free list", title: "halaman yang boleh dipakai lagi", visual: freelist("s03_"), chapter: "Free list dan daftar tunggu",
      notes: [
        { at: 1.0, html: `${m("free list")}: nomor halaman yang gak lagi ditunjuk tree` },
        { at: 5.0, html: "halaman yang ditinggalkan commit 8 masih dipakai commit 7, cadangannya" },
        { at: 10.0, html: `jadi menunggu satu commit dulu di daftar tunggu, ${m("pending")}` },
        { at: 15.0, html: "Open cuma pernah mundur satu commit, jadi satu jeda cukup" }] },
    { type: "compare", dur: 15, kicker: "free list setelah program ditutup", title: "disimpan, atau dihitung ulang?",
      panels: [
        { num: "01 · bbolt NoFreelistSync", name: "dihitung ulang", at: 1.2, lines: ["Open menuruni seluruh tree", "halaman yang gak dikunjungi dianggap bebas",
          "1 juta key: lebih dari 3.000 halaman dibaca"] },
        { num: "02 · kvku", name: "disimpan di file", at: 5.6, pay: true, lines: ["rantai halaman free list, ditunjuk meta",
          "Open tetap cuma baca halaman 0 dan rantai itu", "harganya: tiap commit menulis satu halaman lagi"] }] },
    { type: "statement", reg: "orange", dur: 7, chapter: "Kapan file data dipadatkan",
      lines: [{ text: "kapan dipadatkan?", size: 200 }],
      lead: "compaction: salin record yang hidup ke file data baru, lalu buang file lama.", leadAt: 1.4 },
    { type: "split", dur: 20, kicker: "ambang compaction", title: "75%, bukan 50%", visual: threshold("s06_"),
      notes: [
        { at: 1.0, html: "kvku menghitung byte sampah di meta, tanpa membaca file data" },
        { at: 5.0, html: "Sync mulai compaction saat sampah lewat 75% dan file lebih dari 1 MB" },
        { at: 10.0, html: `dokumentasi ${m("RunValueLogGC")} Badger menyarankan 0,5` },
        { at: 15.0, html: "75%: lebih sedikit penulisan, file data boleh empat kali lebih besar" }] },
    { type: "split", dur: 20, kicker: "copy-on-write membantu lagi", title: "compaction, pembaca jalan terus", visual: compaction("s07_"), chapter: "Compaction tanpa menghentikan pembaca",
      notes: [
        { at: 1.0, html: "cuma b=2 dan c=2 yang hidup. tombstone gak ikut disalin" },
        { at: 5.0, html: "tree baru ditulis ke halaman bebas, value urut menurut key" },
        { at: 10.0, html: `dua kunci: ${m("wmu")} untuk penulis, ${m("RWMutex")} untuk tree` },
        { at: 15.0, html: "Lock singkat untuk menukar root dan file data" }] },
    { type: "split", dur: 18, kicker: "hasil test", title: "pembaca gak berhenti", visual: results("s08_"), chapter: "Hasil test compaction",
      notes: [
        { at: 1.0, html: "empat goroutine terus membaca selama compaction" },
        { at: 4.8, html: "file index berhenti di 140 halaman, dari commit 10 sampai 80" },
        { at: 8.8, html: "file data 1,1 MB, 90% sampah, jadi ~107 KB" },
        { at: 13.2, html: "harganya: penulis menunggu sampai compaction selesai" }] },
    { type: "split", dur: 13, kicker: "cache LRU", title: "membuang node yang lama", visual: cache("s09_"), chapter: "Cache LRU",
      notes: [
        { at: 1.0, html: `${m("LRU")}: buang node yang paling lama gak dipakai` },
        { at: 4.6, html: "root dan branch hampir selalu tetap di cache" },
        { at: 8.6, html: "node baru belum di file, jadi fill Sync tiap 100.000 key" }] },
    { type: "split", dur: 20, kicker: "laptop RAM ~7,4 GB", title: "data 9,3 GB", chapter: "Data lebih besar dari RAM",
      cards: [{ head: "git bash · ukur", lines: [
        { cmd: "ukur -db besar -isi 8000000 -value 1200", at: 0.8 },
        { out: "mengisi 8000000 key: 1m56s", at: 2.4 },
        { out: "file index 412 MB, file data 9338 MB", at: 2.6 },
        { blank: true },
        { cmd: "ukur -db besar -baca 20000 -cache 4096", at: 5.6 },
        { out: "putaran 1: ... 440.8µs per Get", at: 7.2 },
        { out: "putaran 2: ... 50.1µs per Get", at: 7.6 },
        { out: "heap Go 114 MB", at: 8.0 },
        { blank: true },
        { cmd: "ukur -db besar -baca 20000 -cache 64", at: 13.0 },
        { out: "putaran 1: ... 95.9µs per Get", at: 14.6 },
        { out: "heap Go 3 MB", at: 14.8 }] }],
      notes: [
        { at: 1.0, html: "8 juta key, value 1.200 byte, ~2 menit" },
        { at: 7.4, line: 5, html: "dari disk ~441 µs per Get, dari memori ~50 µs" },
        { at: 15.0, line: 11, html: "heap Go ditentukan cache, bukan besar datanya" }] },
    { type: "split", dur: 20, kicker: "mmap, sekali lagi", title: "kali ini dengan angka", visual: mmap("s11_"), chapter: "Menimbang mmap dengan angka",
      notes: [
        { at: 1.0, html: "potongan 4 KB di posisi acak, ReadAt dan mmap" },
        { at: 5.0, html: "dari disk: mmap gak menang, malah lebih lambat di 9,3 GB" },
        { at: 10.0, html: "dari page cache: mmap belasan kali lebih cepat" },
        { at: 15.0, html: "kvku tetap ReadAt: root dan branch sudah di cache sendiri" }] },
    { type: "bullets", dur: 13, kicker: "kelemahan versi ini", title: "yang belum beres", chapter: "Kelemahan dan bagian berikutnya",
      items: [
        { at: 1.5, text: "penulis menunggu selama compaction" },
        { at: 5.0, text: "compaction menyalin seluruh data dan tree" },
        { at: 8.5, text: "file index gak pernah mengecil" }] },
    { type: "title", reg: "orange", dur: 10,
      kicker: "lanjut bagian 6 · bagian terakhir",
      lines: [{ text: "kvku lawan", size: 150, light: true }, { text: "bbolt dan Badger", size: 150 }],
      lead: "diukur di laptop yang sama, lalu menutup seri. artikel lengkapnya, dengan kode Go dan test-nya: link di deskripsi.", leadAt: 1.4 },
  ],
});
console.log("bagian 5 (YouTube):", total, "s");
