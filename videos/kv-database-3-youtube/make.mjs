// Builds the YouTube (16:9, ~3 min, captions only) video for "Membuat Database Key-Value Sendiri" Bagian 3: the B+tree
// moves into a file of fixed 16 KB pages (meta page, node page layout), ReadAt vs mmap and why kvku does not map yet,
// the Sync order and the risk of overwriting pages in place, and a million keys in a file. The story, numbers and
// diagrams follow the article (docs/articles/membuat-database-key-value-sendiri/seri-...-bagian-3-...md).
// Run: node make.mjs. Scene kit: ../kv-database-shared/wide.mjs and diagrams.mjs.
import { buildWide } from "../kv-database-shared/wide.mjs";
import {
  DIAGRAM_CSS, box, text, nm, sm, cells, svg, bar, join,
  inJs, outJs, drawJs, hotJs, growJs,
} from "../kv-database-shared/diagrams.mjs";

const m = (s) => `<span class="m">${s}</span>`;
// a path with an arrow head at its last point, going in direction dir ("up", "down")
const head = (x, y, dir) => (dir === "down" ? ` M${x} ${y} L${x - 12} ${y - 18} M${x} ${y} L${x + 12} ${y - 18}` : ` M${x} ${y} L${x - 12} ${y + 18} M${x} ${y} L${x + 12} ${y + 18}`);

// Bagian 2 still rebuilt the tree on every start: 0.36 s for one Get
const recap = (p) => ({ css: DIAGRAM_CSS, html: join(
  text(`${p}l`, 0, 140, "Bagian 2 · satu Get, file 1 juta key", "up mute"),
  text(`${p}n`, 0, 200, "0,36", "stat", { "font-size": 300 }),
  text(`${p}u`, 600, 330, "detik", "say", { "font-size": 90 }),
  text(`${p}s`, 0, 520, "sebagai program terpisah: Open membaca seluruh log dulu", "say mute", { width: 960, "font-size": 36 }),
), js: join(
  inJs(`${p}l`, 0.5, { y: 10 }),
  `      tl.fromTo($("${p}n"), { scale: 0.6, opacity: 0, transformOrigin: "0% 50%" }, { scale: 1, opacity: 1, duration: 0.5, ease: "back.out(1.6)" }, 1.0);`,
  inJs(`${p}u`, 1.3, { x: -30 }), inJs(`${p}s`, 4.6, { y: 20 }),
) });

// a file of fixed pages: page n starts at n x 16,384
const pages = (p) => {
  const c = cells(`${p}c`, 0, 200, ["halaman 0", "halaman 1", "halaman 2", "halaman 3"].map((t) => ({ t })), { w: 243, h: 120 });
  return { css: DIAGRAM_CSS, html: join(
    text(`${p}l`, 0, 150, "satu file, halaman 16 KB", "up mute"),
    c.html,
    ["0", "16.384", "32.768", "49.152"].map((o, i) => text(`${p}o${i}`, c.xs[i] + 6, 340, o, "mute")),
    box(`${p}f`, 0, 430, 960, 110, `<span class="dn" style="font-size: 40px">halaman n mulai di byte n × 16.384</span>`, "fill"),
    text(`${p}x`, 0, 610, "node yang ditulis sepanjang isinya: membesar, gak muat lagi, harus dipindah, dan semua yang menunjuk ke situ ikut diubah", "say mute", { width: 960 }),
  ), js: join(
    inJs(`${p}l`, 0.5, { y: 10 }), inJs(c.ids, 0.7, { y: 30 }, { stagger: 0.1 }),
    inJs([0, 1, 2, 3].map((i) => `${p}o${i}`), 4.6, { y: 10 }, { stagger: 0.12 }),
    hotJs(c.ids[3], 5.4, { text: true }),
    inJs(`${p}f`, 5.8, { y: 30 }),
    inJs(`${p}x`, 13.0, { y: 20 }),
  ) };
};

// page sizes in other databases (from their docs)
const sizes = (p) => {
  const rows = [["InnoDB (MySQL)", "16 KB bawaan, bisa 4 KB sampai 64 KB"], ["PostgreSQL", "biasanya 8 kB"],
    ["SQLite", "pangkat dua, 512 byte sampai 64 KB"], ["kvku", "16 KB, dipilih saat file dibuat: 1 KB sampai 64 KB"]];
  return { css: DIAGRAM_CSS, html: join(rows.map(([a, b], i) => box(`${p}r${i}`, 0, 50 + i * 190, 960, 160, `${nm(a)}${sm(b)}`, i === 3 ? "l fill" : "l"))),
    js: join(inJs(rows.map((_, i) => `${p}r${i}`), 0.4, { x: 60 }, { stagger: 0.2 }), hotJs(`${p}r0`, 4.6)) };
};

// the file: meta page 0 points at root page 3, which points at leaves 1 and 2
const file = (p) => {
  const X = [0, 245, 490, 735], W = 225, Y = 250, H = 210;
  const cxs = X.map((x) => x + W / 2);
  const toRoot = `M${cxs[0]} ${Y - 4} L${cxs[0]} 170 L${cxs[3]} 170 L${cxs[3]} ${Y - 4}` + head(cxs[3], Y - 4, "down");
  const to2 = `M${cxs[3] - 40} ${Y + H + 4} L${cxs[3] - 40} 530 L${cxs[2]} 530 L${cxs[2]} ${Y + H + 4}` + head(cxs[2], Y + H + 4, "up");
  const to1 = `M${cxs[3] + 40} ${Y + H + 4} L${cxs[3] + 40} 600 L${cxs[1]} 600 L${cxs[1]} ${Y + H + 4}` + head(cxs[1], Y + H + 4, "up");
  const pg = [["meta", ["root = 3", "halaman = 4"]], ["leaf", ["a sampai l", "berikutnya: 2"]], ["leaf", ["m sampai z", ""]], ["branch, root", ["1 · m · 2", ""]]];
  return { css: DIAGRAM_CSS, html: join(
    svg(960, 820, [{ id: `${p}a0`, d: toRoot }, { id: `${p}a1`, d: to1 }, { id: `${p}a2`, d: to2 }]),
    pg.map(([t, ls], i) => join(
      box(`${p}p${i}`, X[i], Y, W, H, `${sm(`halaman ${i}`)}${nm(t)}${ls.filter(Boolean).map((l) => sm(l)).join("")}`, i === 0 ? "hot" : ""),
    )),
    text(`${p}say`, 0, 680, "Open cukup baca halaman 0. halaman lain dibaca saat pencarian melewatinya", "say mute", { width: 960 }),
  ), js: join(
    inJs(`${p}p0`, 0.6, { y: 30 }),
    [1, 2, 3].map((i) => inJs(`${p}p${i}`, 4.8 + (i - 1) * 0.5, { x: -30 })),
    drawJs(`${p}a0`, 7.0, 1100, 0.7),
    drawJs(`${p}a1`, 7.8, 800, 0.6), drawJs(`${p}a2`, 8.0, 800, 0.6),
    inJs(`${p}say`, 10.4, { y: 20 }),
  ) };
};

// the byte layout of a node page: a 12-byte header, then entries, then zeros to the end
const layout = (p) => {
  const hdr = [["CRC-32C", "0-3", 0, 220], ["jenis", "4", 217, 150], ["-", "5", 364, 120], ["jumlah key", "6-7", 481, 210], ["leaf berikutnya", "8-11", 688, 272]];
  const ent = [["panjang key", "2 byte", 0, 215], ["panjang value", "2 byte", 212, 245], ["key", "", 454, 160], ["value", "", 611, 160], ["... nol", "", 768, 192]];
  return { css: DIAGRAM_CSS, html: join(
    text(`${p}hl`, 0, 60, "header 12 byte · halaman leaf", "up mute"),
    hdr.map(([a, b, x, w], i) => box(`${p}h${i}`, x, 110, w, 130, `${nm(a)}${sm(`byte ${b}`)}`, i === 0 ? "hot" : i === 2 ? "mute" : "")),
    text(`${p}el`, 0, 330, "lalu entri, satu per key", "up mute"),
    ent.map(([a, b, x, w], i) => box(`${p}e${i}`, x, 380, w, 130, `${nm(a)}${b ? sm(b) : ""}`, i === 4 ? "dash" : "")),
    text(`${p}z`, 0, 525, "sampai byte 16.383", "mute", { width: 960, "text-align": "right" }),
    text(`${p}lim`, 0, 640, "key paling panjang 1.024 byte, value paling besar 2.048 byte: setiap belahan pasti muat", "say mute", { width: 960 }),
  ), js: join(
    inJs(`${p}hl`, 0.5, { y: 10 }), inJs(hdr.map((_, i) => `${p}h${i}`), 0.7, { y: 30 }, { stagger: 0.1 }),
    hotJs(`${p}h0`, 5.2),
    inJs(`${p}el`, 10.0, { y: 10 }), inJs(ent.map((_, i) => `${p}e${i}`), 10.2, { y: 30 }, { stagger: 0.1 }), inJs(`${p}z`, 10.8, { y: 10 }),
    inJs(`${p}lim`, 15.2, { y: 20 }),
  ) };
};

// ReadAt vs mmap, measured on random 16 KB pages of a 64 MB file in the page cache
const readBars = (p) => {
  const rows = [["ReadAt satu halaman", 960, "~7,5 µs", 1.0], ["mmap, salin satu halaman", 128, "~1,0 µs", 5.4], ["mmap, baca satu byte", 12, "~49 ns", 6.4]];
  return { css: DIAGRAM_CSS, html: join(
    rows.map(([l, w, v], i) => join(
      text(`${p}w${i}`, 0, 60 + i * 180, l, "say", { "font-size": 38 }),
      bar(`${p}b${i}`, 0, 118 + i * 180, w, 56, i === 0 ? "mute" : ""),
      w > 700 ? text(`${p}v${i}`, 0, 124 + i * 180, v, "", { "font-size": 36, width: w - 24, "text-align": "right", color: "#f0ece5", "z-index": "3" })
        : text(`${p}v${i}`, w + 24, 122 + i * 180, v, "cream", { "font-size": 40 }),
    )),
    box(`${p}f`, 0, 650, 960, 120, `<span class="dn" style="font-size: 40px">bbolt: baca lewat mmap, tulis lewat WriteAt</span>`, "fill"),
  ), js: join(
    rows.map(([, w], i) => join(inJs(`${p}w${i}`, rows[i][3], { y: 20 }), growJs(`${p}b${i}`, rows[i][3] + 0.3, w > 300 ? 1.2 : 0.3), inJs(`${p}v${i}`, rows[i][3] + (w > 300 ? 1.4 : 0.6), { y: 10 }))),
    inJs(`${p}f`, 15.2, { y: 30 }),
  ) };
};

// four problems with mmap and Go's standard library
const why = (p) => {
  const rows = [["kode beda untuk tiap OS", "syscall.Mmap ada di Linux, gak ada di Windows"], ["ukuran file sulit diubah", "Windows: gak bisa diperkecil selama dipetakan"],
    ["salah baca jadi crash", "SIGBUS di Linux, bukan error biasa"], ["kapan tulisan sampai ke file?", "cuma msync yang bisa mengatur"]];
  return { css: DIAGRAM_CSS, html: join(rows.map(([a, b], i) => box(`${p}r${i}`, 0, 30 + i * 196, 960, 170, `${nm(a)}${sm(b)}`, "l"))),
    js: join(inJs(rows.map((_, i) => `${p}r${i}`), 0.6, { x: 60 }, { stagger: 0.3 }), hotJs(`${p}r1`, 5.2), hotJs(`${p}r2`, 10.2)) };
};

// overwriting pages in place: power fails after page 2 is written, before pages 3 and 4
const risk = (p) => {
  const X = [0, 325, 650], W = 310;
  const top = [["2 · leaf", "m sampai r"], ["3 · root", "1 · m · 2 · s · 4"], ["4 · leaf baru", "s sampai z"]];
  const bot = [["2 · leaf", "m sampai r", ""], ["3 · root lama", "1 · m · 2", "hot"], ["4 · gak ada", "belum ditulis", "dash"]];
  return { css: DIAGRAM_CSS, html: join(
    text(`${p}tl`, 0, 60, "yang seharusnya ditulis Sync", "up mute"),
    top.map(([a, b], i) => box(`${p}t${i}`, X[i], 110, W, 150, `${nm(a)}${sm(b)}`)),
    text(`${p}bl`, 0, 340, "di file setelah listrik padam", "up mute"),
    bot.map(([a, b, c], i) => box(`${p}b${i}`, X[i], 390, W, 150, `${nm(a)}${sm(b)}`, c)),
    text(`${p}say`, 0, 600, "key s sampai z hilang: halaman 2 sudah gak berisi key itu, dan root gak tahu halaman 4", "say", { width: 960, color: "#e85d26" }),
  ), js: join(
    inJs(`${p}tl`, 0.5, { y: 10 }), inJs(top.map((_, i) => `${p}t${i}`), 0.7, { y: 30 }, { stagger: 0.12 }),
    inJs(`${p}bl`, 10.0, { y: 10 }), inJs(bot.map((_, i) => `${p}b${i}`), 10.2, { y: 30 }, { stagger: 0.25 }),
    inJs(`${p}say`, 15.2, { y: 20 }),
  ) };
};

const { total } = buildWide({
  series: "bikin database key-value sendiri · bagian 3",
  part: "03 / 06",
  scenes: [
    { type: "title", reg: "orange", dur: 7, still: true, chapter: "Pembuka: B+tree yang masih di memori",
      kicker: "bikin database key-value sendiri · bagian 3",
      lines: [{ text: "B+tree di", size: 180, light: true }, { text: "halaman 16 KB", size: 180 }],
      lead: "B+tree pindah ke file, lalu kita menimbang mmap." },
    { type: "split", dur: 13, kicker: "di bagian 2", title: "tree dibangun ulang tiap mulai", visual: recap("s02_"),
      notes: [
        { at: 1.0, html: "B+tree di memori, dibangun dari seluruh log" },
        { at: 4.6, html: "satu Get sebagai program terpisah masih lama" },
        { at: 8.6, html: "bagian 3: B+tree-nya sendiri disimpan di file" }] },
    { type: "split", dur: 17, kicker: "halaman", title: "potongan file berukuran tetap", visual: pages("s03_"), chapter: "Kenapa halaman berukuran tetap",
      notes: [
        { at: 1.0, html: "tiap node B+tree disimpan di satu halaman" },
        { at: 5.0, html: "posisi node dihitung dari nomor halamannya" },
        { at: 9.0, html: "nomor halaman menggantikan pointer di Bagian 2" },
        { at: 13.0, html: "harganya: ruang kosong di tiap halaman" }] },
    { type: "split", dur: 13, kicker: "kenapa 16 KB?", title: "ukuran halaman database lain", visual: sizes("s04_"),
      notes: [
        { at: 1.0, html: "halaman besar: fan-out besar, tree tetap pendek" },
        { at: 4.6, html: "kvku pakai 16 KB, seperti bawaan InnoDB" },
        { at: 8.6, html: "kelemahannya: perubahan kecil menulis ulang satu halaman penuh" }] },
    { type: "statement", reg: "orange", dur: 7, chapter: "Anatomi file kvku",
      lines: [{ text: "anatomi file", size: 230 }],
      lead: "halaman 0: meta. halaman 1 dan seterusnya: satu node per halaman.", leadAt: 1.4 },
    { type: "split", dur: 20, kicker: "dari meta sampai leaf", title: "halaman 0 menunjuk ke root", visual: file("s06_"),
      notes: [
        { at: 1.0, html: `${m("halaman meta")}: nomor halaman root, jumlah halaman, jumlah key` },
        { at: 5.0, html: "halaman baru selalu ditambahkan di ujung file" },
        { at: 10.0, html: "branch nyimpan nomor halaman anak-anaknya" },
        { at: 15.0, html: "leaf nyimpan nomor halaman leaf berikutnya" }] },
    { type: "split", dur: 20, kicker: "susunan byte", title: "satu halaman node", visual: layout("s07_"), chapter: "Susunan byte di halaman",
      notes: [
        { at: 1.0, html: "tiap halaman node: header 12 byte, lalu entri" },
        { at: 5.0, html: `${m("CRC-32C")}: halaman yang tertulis setengah dikenali saat dibaca` },
        { at: 10.0, html: "entri leaf: panjang key, panjang value, key, value" },
        { at: 15.0, html: "node dibelah menurut byte, bukan jumlah key" }] },
    { type: "split", dur: 20, kicker: "membaca halaman", title: "ReadAt, atau mmap?", visual: readBars("s08_"), chapter: "ReadAt atau mmap?",
      notes: [
        { at: 1.0, html: `${m("ReadAt")}: salin byte dari page cache ke memori program` },
        { at: 5.0, html: `${m("page cache")}: memori OS untuk isi file yang baru dibaca` },
        { at: 10.0, html: `${m("mmap")}: file dipetakan ke memori, dibaca seperti slice` },
        { at: 15.0, html: "tanpa salinan, tanpa system call" }] },
    { type: "split", dur: 20, kicker: "dengan library standar Go", title: "kenapa kvku belum pakai mmap", visual: why("s09_"), chapter: "Kenapa kvku belum pakai mmap",
      notes: [
        { at: 1.0, html: "empat masalah muncul saat mmap dipakai" },
        { at: 5.0, html: "test kecil di Windows: Truncate ditolak selama file dipetakan" },
        { at: 10.0, html: "kesalahan baca jadi crash, bukan error" },
        { at: 15.0, html: "jadi kvku: ReadAt + cache sendiri. kode sama di semua OS" }] },
    { type: "split", dur: 20, kicker: "kapan halaman ditulis?", title: "menimpa halaman itu berisiko", visual: risk("s10_"), chapter: "Sync dan risiko menimpa halaman",
      notes: [
        { at: 1.0, html: "Put cuma mengubah node di cache. halaman ditulis saat Sync" },
        { at: 5.0, html: `Sync: halaman node, ${m("fsync")}, halaman meta, ${m("fsync")}` },
        { at: 10.0, html: "tapi halaman ditimpa di tempatnya sendiri" },
        { at: 15.0, html: "CRC-nya cocok semua, cuma beda waktu. diselesaikan di Bagian 4" }] },
    { type: "split", dur: 18, kicker: "satu juta key", title: "Get baca tiga halaman", chapter: "Satu juta key di file",
      cards: [{ head: "git bash · kvku", lines: [
        { cmd: "kvku -db besar.kvt stats", at: 0.8 },
        { out: "jumlah key     : 1000000", at: 1.6 },
        { out: "tinggi tree    : 3", at: 1.8 },
        { cmd: "kvku -db besar.kvt -v get key:000777777", at: 4.4 },
        { out: "nilai 000777777", at: 5.6 },
        { out: "halaman dibaca dari file: 3", at: 5.8 },
        { blank: true },
        { cmd: "ls -l besar.kvt", at: 12.4 },
        { out: "64176128 besar.kvt", at: 13.0 }] }],
      notes: [
        { at: 1.0, html: "buka file 100.000 key: ~0,1 ms. log di Bagian 2: ~40 ms" },
        { at: 6.2, line: 5, html: "satu Get: root, branch, leaf. ~0,03 detik, dulu ~0,36" },
        { at: 13.4, line: 8, html: "64 MB untuk isi ~32 MB: key urut bikin leaf setengah penuh" }] },
    { type: "bullets", dur: 13, kicker: "kelemahan B+tree di file", title: "yang belum beres", chapter: "Kelemahan dan bagian berikutnya",
      items: [
        { at: 1.5, text: "halaman ditimpa di tempatnya" },
        { at: 5.0, text: "Put hilang kalau program mati sebelum Sync" },
        { at: 8.5, text: "cache gak pernah dikosongkan, belum ada delete" }] },
    { type: "title", reg: "orange", dur: 10,
      kicker: "lanjut bagian 4",
      lines: [{ text: "update dan delete", size: 150, light: true }, { text: "tanpa merusak data", size: 150 }],
      lead: "copy-on-write dan dua halaman meta. artikel lengkapnya, dengan kode Go dan test-nya: link di deskripsi.", leadAt: 1.4 },
  ],
});
console.log("bagian 3 (YouTube):", total, "s");
