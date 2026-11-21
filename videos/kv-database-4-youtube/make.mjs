// Builds the YouTube (16:9, ~3 min, captions only) video for "Membuat Database Key-Value Sendiri" Bagian 4: why
// overwriting pages in place breaks a commit, journal vs copy-on-write, copying the path to the root, dropping the leaf
// chain, the two metas A and B, values in an append-only data file with tombstones, merge and borrow on Delete, the
// power-cut and kill tests, and kvku from the terminal. The story, numbers and diagrams follow the article
// (docs/articles/membuat-database-key-value-sendiri/seri-...-bagian-4-...md). Run: node make.mjs.
// Scene kit: ../kv-database-shared/wide.mjs and diagrams.mjs.
import { buildWide } from "../kv-database-shared/wide.mjs";
import {
  DIAGRAM_CSS, box, text, nm, sm, cells, arrow, svg, bar, join,
  inJs, drawJs, hotJs, growJs,
} from "../kv-database-shared/diagrams.mjs";

const m = (s) => `<span class="m">${s}</span>`;
const nw = (s) => `<span style="white-space: nowrap">${s}</span>`;

// Bagian 3: Sync overwrote pages in place; the power fails after page 2, before pages 3 and 4
const risk = (p) => {
  const X = [0, 325, 650], W = 310;
  const top = [["2 · leaf", "m sampai r"], ["3 · root", "1 · m · 2 · s · 4"], ["4 · leaf baru", "s sampai z"]];
  const bot = [["2 · leaf", "sudah baru", ""], ["3 · root", "masih lama", "hot"], ["4 · gak ada", "belum ditulis", "dash"]];
  return { css: DIAGRAM_CSS, html: join(
    text(`${p}tl`, 0, 60, "Sync di bagian 3 · seharusnya", "up mute"),
    top.map(([a, b], i) => box(`${p}t${i}`, X[i], 110, W, 150, `${nm(a)}${sm(b)}`)),
    text(`${p}bl`, 0, 340, "di file setelah listrik padam", "up mute"),
    bot.map(([a, b, c], i) => box(`${p}b${i}`, X[i], 390, W, 150, `${nm(a)}${sm(b)}`, c)),
    text(`${p}say`, 0, 600, "tiap halaman utuh, CRC-nya cocok. tapi beda waktu: key s sampai z hilang", "say", { width: 960, color: "#e85d26" }),
  ), js: join(
    inJs(`${p}tl`, 0.5, { y: 10 }), inJs(top.map((_, i) => `${p}t${i}`), 0.7, { y: 30 }, { stagger: 0.12 }),
    inJs(`${p}bl`, 4.4, { y: 10 }), inJs(bot.map((_, i) => `${p}b${i}`), 4.6, { y: 30 }, { stagger: 0.25 }),
    inJs(`${p}say`, 8.4, { y: 20 }),
  ) };
};

// copy-on-write: commit 7's tree on the left, commit 8 copies only the path to the changed leaf
const cow = (p) => {
  const W = 140, H = 96;
  const pos = { meta: [150, 20], root: [150, 160], b0: [40, 310], b1: [260, 310], l0: [0, 460], l1: [150, 460] };
  const mk = (ox, nodes) => nodes.map(([k, a, b, cls]) => box(`${p}${ox ? "r" : "l"}${k}`, pos[k][0] + ox, pos[k][1], W, H, `${nm(a)}${sm(b)}`, cls));
  const edges = (ox, side) => [["meta", "root"], ["root", "b0"], ["root", "b1"], ["b0", "l0"], ["b0", "l1"]].map(([a, b], i) => {
    const A = arrow(pos[a][0] + ox + W / 2, pos[a][1] + H, pos[b][0] + ox + W / 2, pos[b][1] - 4, 14);
    return { id: `${p}${side}e${i}`, d: A.d, len: A.len, cls: side === "r" && i !== 2 && i !== 3 ? "" : "cream thin" };
  });
  const L = edges(0, "l"), R = edges(500, "r");
  return { css: DIAGRAM_CSS, html: join(
    svg(960, 820, [...L, ...R]),
    mk(0, [["meta", "meta A", "commit 7", ""], ["root", "hal. 9", "root", ""], ["b0", "hal. 4", "branch", ""], ["b1", "hal. 8", "branch", ""],
      ["l0", "hal. 2", "leaf", ""], ["l1", "hal. 5", "leaf lama", ""]]),
    mk(500, [["meta", "meta B", "commit 8", "hot"], ["root", "hal. 14", "root baru", "hot"], ["b0", "hal. 13", "salinan", "hot"], ["b1", "hal. 8", "sama", "mute"],
      ["l0", "hal. 2", "sama", "mute"], ["l1", "hal. 12", "+ key m", "fill"]]),
    text(`${p}tl`, 0, 610, "commit 7", "up mute"),
    text(`${p}tr`, 500, 610, "commit 8", "up"),
    text(`${p}say`, 0, 680, "cuma jalur ke leaf yang berubah yang disalin. halaman lain dipakai bersama", "say mute", { width: 960 }),
  ), js: join(
    inJs([`${p}lmeta`, `${p}lroot`, `${p}lb0`, `${p}lb1`, `${p}ll0`, `${p}ll1`, `${p}tl`], 0.6, { y: 20 }, { stagger: 0.08 }),
    L.map((e, i) => drawJs(e.id, 1.0 + i * 0.1, e.len, 0.4)),
    inJs(`${p}rl1`, 5.0, { y: 30 }), inJs(`${p}rb0`, 6.2, { y: 30 }), inJs(`${p}rroot`, 7.4, { y: 30 }), inJs([`${p}rmeta`, `${p}tr`], 8.6, { y: 30 }),
    drawJs(R[4].id, 6.6, R[4].len, 0.4), drawJs(R[1].id, 7.8, R[1].len, 0.4), drawJs(R[0].id, 9.0, R[0].len, 0.4),
    inJs([`${p}rb1`, `${p}rl0`], 10.4, { y: 20 }, { stagger: 0.15 }),
    drawJs(R[2].id, 10.6, R[2].len, 0.4), drawJs(R[3].id, 10.8, R[3].len, 0.4),
    inJs(`${p}say`, 15.2, { y: 20 }),
  ) };
};

// the leaf chain: a changed leaf moves, so every leaf to its left must be copied too
const chain = (p) => {
  const c = cells(`${p}c`, 0, 160, ["a", "c", "e", "g", "i", "k"].map((t) => ({ t: `leaf ${t}` })), { w: 163, h: 110 });
  return { css: DIAGRAM_CSS, html: join(
    text(`${p}l`, 0, 110, "rantai leaf · tiap leaf menunjuk leaf berikutnya", "up mute"),
    c.html,
    text(`${p}s`, 0, 310, "leaf k berubah, pindah ke halaman baru: leaf i masih menunjuk halaman lama, jadi ikut disalin, lalu leaf g, dan seterusnya", "say mute", { width: 960 }),
    box(`${p}f`, 0, 460, 960, 110, `<span class="dn" style="font-size: 40px">3.000 leaf: satu Put menyalin semuanya</span>`, "fill"),
    text(`${p}k`, 0, 630, "kvku melepas rantai leaf. Range turun dari root, lalu kembali ke induk saat satu leaf habis", "say", { width: 960 }),
  ), js: join(
    inJs(`${p}l`, 0.5, { y: 10 }), inJs(c.ids, 0.7, { y: 30 }, { stagger: 0.08 }),
    hotJs(c.ids[5], 3.6, { text: true }),
    [4, 3, 2, 1, 0].map((i, k) => hotJs(c.ids[i], 4.4 + k * 0.45, { text: true })),
    inJs(`${p}s`, 4.6, { y: 20 }),
    inJs(`${p}f`, 7.4, { y: 30 }),
    inJs(`${p}k`, 10.4, { y: 20 }),
  ) };
};

// two metas in page 0, and the five steps of one commit
const metas = (p) => {
  const steps = [["record", "di file data"], ["halaman", "baru di index"], ["fsync", "data, index"], ["meta B", "commit 8"], ["fsync", "index"]];
  return { css: DIAGRAM_CSS, html: join(
    text(`${p}l`, 0, 40, "halaman 0", "up mute"),
    box(`${p}a`, 0, 90, 320, 130, `${nm("meta A · byte 0")}${sm("commit 7")}`),
    box(`${p}b`, 317, 90, 320, 130, `${nm("meta B · byte 512")}${sm("commit 8")}`, "hot"),
    box(`${p}r`, 634, 90, 326, 130, `${nm("sisa")}${sm("gak dipakai")}`, "dash"),
    text(`${p}sl`, 0, 300, "satu commit", "up mute"),
    steps.map(([a, b], i) => box(`${p}s${i}`, i * 195, 350, 180, 140, `${sm(String(i + 1))}${nm(a)}${sm(b)}`, i === 3 ? "hot" : "")),
    text(`${p}say`, 0, 540, "padam sebelum langkah 4 selesai: gak ada meta yang menunjuk halaman baru. Open tetap di commit 7", "say mute", { width: 960 }),
    box(`${p}f`, 0, 680, 960, 110, `<span class="dn" style="font-size: 40px">3 fsync per commit · Put + Sync ~1,7 ms</span>`, "fill"),
  ), js: join(
    inJs(`${p}l`, 0.5, { y: 10 }), inJs([`${p}a`, `${p}b`, `${p}r`], 0.7, { y: 30 }, { stagger: 0.15 }),
    hotJs(`${p}b`, 3.0),
    inJs(`${p}sl`, 5.0, { y: 10 }), inJs(steps.map((_, i) => `${p}s${i}`), 5.2, { x: -30 }, { stagger: 0.35 }),
    inJs(`${p}say`, 10.0, { y: 20 }),
    inJs(`${p}f`, 15.2, { y: 30 }),
  ) };
};

// index and data: the leaf points at the live record; the rest of the data file is garbage
const data = (p) => {
  const rec = [["user:1 = Andi", "posisi 0 · sampah", "mute"], ["user:2 = Budi", "posisi 21 · sampah", "mute"],
    ["user:2 = Budi Santoso", "posisi 42 · hidup", "hot"], ["del user:1", "posisi 71 · tombstone", "dash"]];
  const Y = [380, 470, 560, 650], RH = 78, ry = Y[2] + RH / 2;
  const d = `M520 130 L920 130 L920 ${ry} L866 ${ry} M866 ${ry} L884 ${ry - 12} M866 ${ry} L884 ${ry + 12}`;
  return { css: DIAGRAM_CSS, html: join(
    text(`${p}il`, 0, 20, "file index", "up mute"),
    box(`${p}leaf`, 0, 70, 520, 120, `${nm("leaf · user:2")}${sm("posisi 42 · 12 byte")}`),
    svg(960, 820, [{ id: `${p}a`, d }]),
    text(`${p}dl`, 0, 330, "file data · cuma bertambah di ujung", "up mute"),
    rec.map(([a, b, c], i) => box(`${p}r${i}`, 0, Y[i], 860, RH, `${nm(a)}${sm(b)}`, `l ${c}`, { gap: 0 })),
    text(`${p}say`, 0, 750, "dari 88 byte, cuma 29 byte yang masih ditunjuk tree", "say", { width: 960, color: "#e85d26" }),
  ), js: join(
    inJs(`${p}il`, 0.5, { y: 10 }), inJs(`${p}leaf`, 0.7, { y: 30 }),
    inJs(`${p}dl`, 4.6, { y: 10 }), inJs([0, 1, 2].map((i) => `${p}r${i}`), 4.8, { x: -30 }, { stagger: 0.4 }),
    drawJs(`${p}a`, 6.2, 1100, 0.6),
    inJs(`${p}r3`, 15.0, { x: -30 }),
    inJs(`${p}say`, 16.4, { y: 20 }),
  ) };
};

// merge and borrow: a node under a quarter page merges with its sibling, and splits again if it does not fit
const merge = (p) => {
  const leaf = (id, x, y, pct, label, cls = "mute") => join(
    box(`${id}`, x, y, 290, 80, "", "flat", { background: "transparent" }),
    bar(`${id}b`, x + 3, y + 3, Math.round(284 * Math.min(pct, 100) / 100), 74, cls),
    text(`${id}t`, x, y + 92, label, "cream", { "font-size": 26 }),
  );
  const ids = (id) => [id, `${id}b`, `${id}t`];
  return { css: DIAGRAM_CSS, html: join(
    text(`${p}gl`, 0, 30, "gabung", "up"),
    leaf(`${p}g0`, 0, 80, 60, "a sampai f · 60%"), leaf(`${p}g1`, 330, 80, 10, "g · 10%"),
    leaf(`${p}g2`, 670, 80, 70, "a sampai g · 70%", ""),
    text(`${p}gs`, 0, 230, "satu halaman dilepas, pemisah g dihapus dari induk", "say mute", { width: 960, "font-size": 28 }),
    text(`${p}pl`, 0, 340, "pinjam", "up"),
    leaf(`${p}p0`, 0, 390, 95, "a sampai l · 95%"), leaf(`${p}p1`, 330, 390, 10, "m · 10%"),
    text(`${p}px`, 670, 400, "105%: gak muat", "", { "font-size": 34 }),
    leaf(`${p}p2`, 0, 560, 52, "a sampai f · 52%", ""), leaf(`${p}p3`, 330, 560, 53, "g sampai m · 53%", ""),
    text(`${p}ps`, 0, 720, "dibelah lagi di tengah, pemisah baru g naik ke induk", "say mute", { width: 960, "font-size": 28 }),
  ), js: join(
    inJs(`${p}gl`, 0.5, { y: 10 }),
    inJs([...ids(`${p}g0`), ...ids(`${p}g1`)], 0.7, { y: 20 }, { stagger: 0.05 }),
    growJs(`${p}g0b`, 1.0, 0.6), growJs(`${p}g1b`, 1.2, 0.3),
    inJs(ids(`${p}g2`), 3.0, { x: -30 }), growJs(`${p}g2b`, 3.2, 0.6), inJs(`${p}gs`, 3.6, { y: 10 }),
    inJs(`${p}pl`, 8.6, { y: 10 }),
    inJs([...ids(`${p}p0`), ...ids(`${p}p1`)], 8.8, { y: 20 }, { stagger: 0.05 }),
    growJs(`${p}p0b`, 9.1, 0.6), growJs(`${p}p1b`, 9.3, 0.3),
    inJs(`${p}px`, 10.2, { x: -30 }),
    inJs([...ids(`${p}p2`), ...ids(`${p}p3`)], 11.6, { y: 30 }, { stagger: 0.05 }),
    growJs(`${p}p2b`, 11.9, 0.6), growJs(`${p}p3b`, 12.0, 0.6),
    inJs(`${p}ps`, 12.4, { y: 10 }),
  ) };
};

// the power-cut and kill tests
const tests = (p) => {
  const kills = [[3, 4], [7, 8], [11, 12], [15, 15], [19, 20]];
  return { css: DIAGRAM_CSS, html: join(
    text(`${p}l`, 0, 40, "TestListrikPadam", "mute", { "font-size": 28 }),
    text(`${p}n`, 0, 90, "94", "stat", { "font-size": 260 }),
    text(`${p}u`, 330, 200, "titik padam", "say", { "font-size": 70 }),
    text(`${p}s`, 0, 340, "semuanya kembali ke commit lama. gak ada satu pun tree campuran", "say mute", { width: 960, "font-size": 30 }),
    text(`${p}kl`, 0, 450, "TestDihentikanPaksa · 5 putaran", "mute", { "font-size": 28 }),
    kills.map(([a, b], i) => box(`${p}k${i}`, i * 195, 500, 180, 130, `${nm(`buka ${b}`)}${sm(`lapor ${a}`)}`, a === b ? "" : "hot")),
    text(`${p}ks`, 0, 660, "dibuka di commit yang sudah selesai, walaupun belum sempat dilaporkan", "say mute", { width: 960, "font-size": 30 }),
  ), js: join(
    inJs(`${p}l`, 0.5, { y: 10 }),
    `      tl.fromTo($("${p}n"), { scale: 0.6, opacity: 0, transformOrigin: "0% 50%" }, { scale: 1, opacity: 1, duration: 0.5, ease: "back.out(1.6)" }, 0.8);`,
    inJs(`${p}u`, 1.1, { x: -30 }), inJs(`${p}s`, 5.2, { y: 20 }),
    inJs(`${p}kl`, 10.0, { y: 10 }), inJs(kills.map((_, i) => `${p}k${i}`), 10.2, { y: 30 }, { stagger: 0.15 }),
    inJs(`${p}ks`, 11.6, { y: 20 }),
  ) };
};

const { total } = buildWide({
  series: "bikin database key-value sendiri · bagian 4",
  part: "04 / 06",
  scenes: [
    { type: "title", reg: "orange", dur: 7, still: true, chapter: "Pembuka: halaman yang ditimpa",
      kicker: "bikin database key-value sendiri · bagian 4",
      lines: [{ text: "update dan delete", size: 170, light: true }, { text: "tanpa merusak data", size: 170 }],
      lead: "copy-on-write, dua meta, tombstone, lalu listrik padam 94 kali." },
    { type: "split", dur: 13, kicker: "di bagian 3", title: "halaman ditimpa di tempatnya", visual: risk("s02_"),
      notes: [
        { at: 1.0, html: "Put yang membelah leaf mengubah tiga halaman" },
        { at: 4.6, html: "listrik padam di tengah Sync: leaf baru, induk lama" },
        { at: 8.6, html: `yang dibutuhkan: ${m("commit")} yang atomik, semua atau gak sama sekali` }] },
    { type: "compare", dur: 17, kicker: "dua jawaban", title: "journal atau copy-on-write?", chapter: "Journal atau copy-on-write?",
      panels: [
        { num: "01 · SQLite", name: "journal", at: 1.2, lines: ["perubahan dicatat di file kedua sebelum halaman ditimpa",
          "rollback journal, atau WAL sejak 3.7.0", "tiap perubahan ditulis dua kali"] },
        { num: "02 · LMDB, bbolt, kvku", name: "copy-on-write", at: 6.0, pay: true, lines: ["halaman yang berubah ditulis ke halaman baru",
          "halaman lama tetap utuh", "commit selesai saat meta menunjuk root baru"] }] },
    { type: "statement", reg: "orange", dur: 7,
      lines: [{ text: "jangan pernah", size: 200, light: true }, { text: "menimpa.", size: 230 }],
      lead: "file selalu berisi tree dari commit sebelumnya.", leadAt: 1.4 },
    { type: "split", dur: 20, kicker: "copy-on-write di B+tree", title: "jalur ke root disalin", visual: cow("s05_"), chapter: "Copy-on-write di B+tree",
      notes: [
        { at: 1.0, html: "leaf yang berubah ditulis ke halaman baru" },
        { at: 5.0, html: "induknya harus menunjuk nomor baru, jadi ikut disalin" },
        { at: 10.0, html: "terus sampai root. meta B menunjuk root baru" },
        { at: 15.0, html: "sekali per halaman per commit: 1.000 Put ke leaf yang sama, satu salinan" }] },
    { type: "split", dur: 15, kicker: "harga copy-on-write", title: "rantai leaf harus dilepas", visual: chain("s06_"), chapter: "Kenapa rantai leaf dilepas",
      notes: [
        { at: 1.0, html: "di bagian 2 dan 3, Range jalan ke kanan di rantai leaf" },
        { at: 4.6, html: "dengan copy-on-write, leaf tetangga kirinya ikut disalin" },
        { at: 10.4, html: "bbolt pakai cursor dengan tumpukan halaman dari root" }] },
    { type: "split", dur: 20, kicker: "meta yang tertulis setengah", title: "dua meta, A dan B", visual: metas("s07_"), chapter: "Dua meta, A dan B",
      notes: [
        { at: 1.0, html: "commit genap ke meta A, commit ganjil ke meta B" },
        { at: 5.0, html: "Open: meta yang utuh dengan commit terbesar yang menang" },
        { at: 10.0, html: `sisa tulisan setelah commit terakhir dipotong dengan ${m("Truncate")}` },
        { at: 15.0, html: "harganya: tiga fsync. dulu Put + Sync ~1 ms" }] },
    { type: "split", dur: 20, kicker: "value pindah", title: "file data yang cuma bertambah", visual: data("s08_"), chapter: "Value di file data, dan tombstone",
      notes: [
        { at: 1.0, html: "leaf cuma nyimpan key, posisi record, dan panjang value" },
        { at: 5.0, html: "value yang diganti ditulis sebagai record baru" },
        { at: 10.0, html: `value bisa sampai 16 MB, dulu cuma ${nw("2 KB")}` },
        { at: 15.0, html: `${m("tombstone")}: record tanpa value, tanda key sudah dihapus` }] },
    { type: "split", dur: 18, kicker: "delete bikin node menyusut", title: "gabung, atau pinjam", visual: merge("s09_"), chapter: "Delete: gabung atau pinjam",
      notes: [
        { at: 1.0, html: "node di bawah seperempat halaman digabung dengan saudaranya" },
        { at: 5.0, html: "batas 25% yang sama dipakai bbolt" },
        { at: 8.8, html: "gak muat? dibelah lagi di tengah, sama dengan meminjam" },
        { at: 13.6, html: "root branch dengan satu anak diganti anaknya: tree lebih pendek" }] },
    { type: "split", dur: 20, kicker: "menguji copy-on-write", title: "listrik padam, 94 kali", visual: tests("s10_"), chapter: "Listrik padam dan kill",
      notes: [
        { at: 1.0, html: "2.000 key, lalu 60 Put dan Delete yang diakhiri Sync" },
        { at: 5.0, html: "tiap penulisan dalam commit dipotong di tengah, satu per satu" },
        { at: 10.0, html: `program anak yang terus menulis dihentikan dengan ${m("Process.Kill")}` },
        { at: 15.0, html: "meta dipaksa selalu ke slot A: test langsung gagal" }] },
    { type: "split", dur: 18, kicker: "kvku versi copy-on-write", title: "del, dan kill -9", chapter: "Mencoba kvku dari terminal",
      cards: [{ head: "git bash · kvku", lines: [
        { cmd: "kvku del user:1", at: 0.8 },
        { cmd: "kvku stats", at: 2.0 },
        { out: "commit         : 5", at: 2.8 },
        { out: "file data      : 88 byte", at: 3.0 },
        { blank: true },
        { cmd: "time kvku -db besar fill 1000000", at: 6.0 },
        { out: "real  0m6.428s", at: 7.6 },
        { cmd: "kvku -db besar fill 2000000 &", at: 10.0 },
        { out: "(kill -9 setelah 1 detik)", at: 11.2 },
        { cmd: "kvku -db besar stats", at: 12.2 },
        { out: "commit         : 2", at: 13.0 },
        { out: "file data      : 39000000 byte", at: 13.2 }] }],
      notes: [
        { at: 1.0, html: "88 byte: dua Put, satu Put pengganti, satu tombstone" },
        { at: 7.8, line: 6, html: "1 juta key ~6,4 detik, dulu ~1,1: satu WriteAt per Put" },
        { at: 13.4, line: 11, html: "fill dihentikan paksa: commit 2 utuh, file data dipotong balik" }] },
    { type: "bullets", dur: 13, kicker: "kelemahan versi ini", title: "yang belum beres", chapter: "Kelemahan dan bagian berikutnya",
      items: [
        { at: 1.5, text: "halaman lama gak pernah dipakai ulang" },
        { at: 5.0, text: "file data cuma bertambah: sampah menumpuk" },
        { at: 8.5, text: "satu Put + Sync: tiga halaman, tiga fsync" }] },
    { type: "title", reg: "orange", dur: 10,
      kicker: "lanjut bagian 5",
      lines: [{ text: "compaction dan data", size: 140, light: true }, { text: "lebih besar dari RAM", size: 140 }],
      lead: "free list, compaction, cache LRU. artikel lengkapnya, dengan kode Go dan test-nya: link di deskripsi.", leadAt: 1.4 },
  ],
});
console.log("bagian 4 (YouTube):", total, "s");
