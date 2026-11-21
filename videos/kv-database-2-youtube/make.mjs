// Builds the YouTube (16:9, ~3 min, captions only) video for "Membuat Database Key-Value Sendiri" Bagian 2: why a B+tree
// (not a sorted slice or a binary search tree), its anatomy, Get, Put with node splits, Range on the leaf chain, the test
// with a map as the answer key, and the benchmarks. The story, numbers and diagrams follow the article
// (docs/articles/membuat-database-key-value-sendiri/seri-...-bagian-2-...md). Run: node make.mjs.
// Scene kit: ../kv-database-shared/wide.mjs and diagrams.mjs.
import { buildWide } from "../kv-database-shared/wide.mjs";
import {
  DIAGRAM_CSS, box, text, nm, sm, cells, arrow, svg, bar, join,
  inJs, outJs, drawJs, hotJs, moveJs, growJs,
} from "../kv-database-shared/diagrams.mjs";

const m = (s) => `<span class="m">${s}</span>`;
// keys inside a node, spaced apart (a plain space is too narrow in Barlow)
const kn = (s) => nm(s.split(" ").join("&nbsp;&nbsp;&nbsp;"));

// Range on Bagian 1's map index: 16 ms for 10 keys out of 100,000
const recap = (p) => ({ css: DIAGRAM_CSS, html: join(
  text(`${p}l`, 0, 140, "Bagian 1 · Range 10 key dari 100.000 key", "up mute"),
  text(`${p}n`, 0, 200, "~16", "stat", { "font-size": 300 }),
  text(`${p}u`, 430, 330, "ms", "say", { "font-size": 90 }),
  text(`${p}s`, 0, 520, "semua key diperiksa, lalu diurutkan, untuk 10 key saja", "say mute", { width: 960, "font-size": 36 }),
), js: join(
  inJs(`${p}l`, 0.5, { y: 10 }),
  `      tl.fromTo($("${p}n"), { scale: 0.6, opacity: 0, transformOrigin: "0% 50%" }, { scale: 1, opacity: 1, duration: 0.5, ease: "back.out(1.6)" }, 4.6);`,
  inJs(`${p}u`, 4.9, { x: -30 }), inJs(`${p}s`, 8.6, { y: 20 }),
) });

// a sorted slice shifts every key on Put; a binary search tree needs ~20 levels for a million keys
const options = (p) => {
  const c = cells(`${p}c`, 0, 100, ["a", "b", "d", "e", "f", "g"].map((t) => ({ t })), { w: 120, h: 90 });
  const lv = cells(`${p}lv`, 0, 560, Array.from({ length: 20 }, (_, i) => ({ t: String(i + 1) })), { w: 50, h: 60 });
  return { css: DIAGRAM_CSS + `\n    #${p}lv0, ${lv.ids.map((id) => `#${id}`).join(", ")} { font-size: 20px; }`, html: join(
    text(`${p}sl`, 0, 50, "slice yang diurutkan", "up mute"),
    c.html,
    box(`${p}new`, 234, 220, 120, 90, nm("c"), "hot"),
    text(`${p}sx`, 0, 330, "Put di tengah: semua key di kanannya bergeser", "say mute", { width: 960, "font-size": 30 }),
    text(`${p}tl`, 0, 500, "binary search tree · 1 juta key", "up mute"),
    lv.html,
    text(`${p}tx`, 0, 650, "setidaknya 20 tingkat. di disk, tiap tingkat bisa satu kali baca", "say mute", { width: 960, "font-size": 30 }),
  ), js: join(
    inJs(`${p}sl`, 0.4, { y: 10 }), inJs(c.ids, 0.6, { y: 30 }, { stagger: 0.07 }),
    inJs(`${p}new`, 5.6, { y: 30 }),
    c.ids.slice(2).map((id) => moveJs(id, 6.4, 117, 0.6)),
    `      tl.to($("${p}new"), { y: -120, duration: 0.5, ease: "power3.inOut" }, 7.1);`,
    inJs(`${p}sx`, 7.4, { y: 20 }),
    inJs(`${p}tl`, 10.6, { y: 10 }), inJs(lv.ids, 10.8, { x: -20 }, { stagger: 0.05 }),
    inJs(`${p}tx`, 15.0, { y: 20 }),
  ) };
};

// fan-out: many keys per node, so three levels under the root reach 262,144 leaves
const fanout = (p) => {
  const kids = [0, 1, 2, 3].map((i) => i * 240);
  const ed = kids.map((x) => arrow(480, 190, x + 100, 300, 14));
  return { css: DIAGRAM_CSS, html: join(
    box(`${p}r`, 330, 90, 300, 100, `${nm("root")}${sm("64 key")}`, "hot"),
    svg(960, 820, ed.map((a, i) => ({ id: `${p}e${i}`, d: a.d, cls: "cream thin" }))),
    kids.map((x, i) => box(`${p}k${i}`, x, 300, 200, 90, i === 3 ? nm("... 64") : nm(`anak ${i + 1}`), i === 3 ? "dash" : "")),
    text(`${p}f`, 0, 430, "fan-out: jumlah anak per node", "cream"),
    text(`${p}n`, 0, 520, "64 × 64 × 64", "stat", { "font-size": 120 }),
    text(`${p}n2`, 0, 660, "= 262.144 leaf, cukup tiga tingkat di bawah root", "say", { "font-size": 36, width: 960 }),
  ), js: join(
    inJs(`${p}r`, 0.5, { y: 30 }),
    ed.map((a, i) => drawJs(`${p}e${i}`, 1.0 + i * 0.1, a.len, 0.4)),
    inJs(kids.map((_, i) => `${p}k${i}`), 1.3, { y: 30 }, { stagger: 0.1 }),
    inJs(`${p}f`, 4.6, { y: 10 }),
    inJs(`${p}n`, 7.0, { y: 30 }), inJs(`${p}n2`, 7.6, { y: 20 }),
  ) };
};

// The article's order-3 tree: root g; branches [c e] and [i]; leaves ab cd ef gh ij, chained left to right.
// hi: { at, path: [root?, branch, leaf ids], leaves: [...] } highlights a Get path or a Range.
const LX = [0, 198, 396, 594, 792], LW = 168, LY = 560;
function tree(p, { at = 0, hi, label }) {
  const leaves = ["a b", "c d", "e f", "g h", "i j"];
  const br = [{ id: `${p}b0`, x: 170, w: 240, t: "c e" }, { id: `${p}b1`, x: 640, w: 180, t: "i" }];
  const cx = (b) => b.x + b.w / 2;
  const edges = [
    { id: `${p}e0`, a: arrow(480, 190, cx(br[0]), 326, 14) }, { id: `${p}e1`, a: arrow(480, 190, cx(br[1]), 326, 14) },
    ...[0, 1, 2].map((i) => ({ id: `${p}e${i + 2}`, a: arrow(cx(br[0]), 420, LX[i] + LW / 2, LY - 4, 14) })),
    ...[3, 4].map((i) => ({ id: `${p}e${i + 2}`, a: arrow(cx(br[1]), 420, LX[i] + LW / 2, LY - 4, 14) })),
  ];
  const chain = [0, 1, 2, 3].map((i) => ({ id: `${p}ch${i}`, a: arrow(LX[i] + LW, LY + 45, LX[i + 1] - 2, LY + 45, 10) }));
  const hot = (hi?.edges ?? []).map((e) => ({ id: `${p}h${e}`, a: e.startsWith("ch") ? chain[+e.slice(2)].a : edges[+e.slice(1)].a }));
  return { css: DIAGRAM_CSS, html: join(
    text(`${p}lr`, 0, 115, "root", "up mute"), text(`${p}lb`, 0, 290, "branch", "up mute"), text(`${p}ll`, 0, 690, "leaf", "up mute"),
    svg(960, 820, [...edges.map((e) => ({ id: e.id, d: e.a.d, cls: "mute thin" })), ...chain.map((e) => ({ id: e.id, d: e.a.d, cls: "cream thin" })),
      ...hot.map((e) => ({ id: e.id, d: e.a.d }))]),
    box(`${p}r`, 380, 100, 200, 90, nm("g")),
    br.map((b) => box(b.id, b.x, 330, b.w, 90, kn(b.t))),
    leaves.map((t, i) => box(`${p}l${i}`, LX[i], LY, LW, 90, kn(t))),
    label ? text(`${p}lab`, 600, 120, label, "cream", { "font-size": 30 }) : "",
  ), js: join(
    inJs(`${p}r`, at + 0.4, { y: 30 }),
    edges.slice(0, 2).map((e) => drawJs(e.id, at + 0.7, e.a.len, 0.4)),
    inJs(br.map((b) => b.id), at + 0.9, { y: 30 }, { stagger: 0.1 }),
    edges.slice(2).map((e) => drawJs(e.id, at + 1.2, e.a.len, 0.4)),
    inJs([0, 1, 2, 3, 4].map((i) => `${p}l${i}`), at + 1.4, { y: 30 }, { stagger: 0.08 }),
    inJs([`${p}lr`, `${p}lb`, `${p}ll`], at + 1.6, { x: -20 }, { stagger: 0.1 }),
    chain.map((e, i) => drawJs(e.id, (hi?.chainAt ?? at + 2.2) + i * 0.15, e.a.len, 0.3)),
    hi ? join(
      label ? inJs(`${p}lab`, hi.at, { y: 10 }) : "",
      hi.nodes.map((id, i) => hotJs(`${p}${id}`, hi.at + i * 0.6, { text: true })),
      hot.map((e, i) => drawJs(e.id, hi.at + 0.2 + i * 0.6, e.a.len, 0.4)),
    ) : "",
  ) };
}

// Put j into an order-3 tree: the leaf splits (i copied up), the branch splits (g moves up), a new root
const splitViz = (p) => {
  const row = (r, y, from, a, b, note) => {
    const ar = arrow(320, y + 45, 400, y + 45, 14);
    return { html: join(
      text(`${p}t${r}`, 0, y - 40, ["1 · leaf kelebihan", "2 · branch kelebihan"][r], "up mute"),
      box(`${p}f${r}`, 0, y, 300, 90, kn(from), "dash"),
      svg(960, 820, [{ id: `${p}a${r}`, d: ar.d, cls: "cream thin" }]),
      box(`${p}x${r}`, 420, y, 230, 90, kn(a)),
      box(`${p}y${r}`, 680, y, 230, 90, kn(b)),
      text(`${p}n${r}`, 420, y + 104, note, "cream"),
    ), ids: [`${p}t${r}`, `${p}f${r}`], len: ar.len };
  };
  const r0 = row(0, 60, "g h i j", "g h", "i j", "i disalin ke induk");
  const r1 = row(1, 330, "c e g i", "c e", "i", "g naik ke induk, gak disalin");
  const e1 = arrow(480, 690, 310, 736, 12), e2 = arrow(480, 690, 650, 736, 12);
  return { css: DIAGRAM_CSS, html: join(
    r0.html, r1.html,
    text(`${p}t2`, 0, 560, "3 · root baru", "up mute"),
    box(`${p}r`, 380, 600, 200, 90, nm("g"), "hot"),
    svg(960, 820, [{ id: `${p}e1`, d: e1.d, cls: "cream thin" }, { id: `${p}e2`, d: e2.d, cls: "cream thin" }]),
    box(`${p}c1`, 200, 740, 220, 80, kn("c e")),
    box(`${p}c2`, 540, 740, 220, 80, nm("i")),
  ), js: join(
    inJs(r0.ids, 1.0, { y: 20 }, { stagger: 0.1 }),
    drawJs(`${p}a0`, 5.6, r0.len, 0.4), inJs([`${p}x0`, `${p}y0`], 5.9, { x: -30 }, { stagger: 0.12 }), inJs(`${p}n0`, 6.4, { y: 10 }),
    inJs(r1.ids, 10.4, { y: 20 }, { stagger: 0.1 }),
    drawJs(`${p}a1`, 11.0, r1.len, 0.4), inJs([`${p}x1`, `${p}y1`], 11.3, { x: -30 }, { stagger: 0.12 }), inJs(`${p}n1`, 11.8, { y: 10 }),
    inJs([`${p}t2`, `${p}r`], 15.4, { y: 20 }, { stagger: 0.1 }),
    drawJs(`${p}e1`, 15.8, e1.len, 0.3), drawJs(`${p}e2`, 15.8, e2.len, 0.3),
    inJs([`${p}c1`, `${p}c2`], 16.0, { y: 20 }, { stagger: 0.1 }),
  ) };
};

// two horizontal bar pairs, scaled to the longer one
const pairBars = (p, rows, foot) => {
  const html = [], js = [];
  rows.forEach((r, i) => {
    const y = 40 + i * 190;
    html.push(text(`${p}w${i}`, 0, y, r.label, "say", { "font-size": 38 }), bar(`${p}b${i}`, 0, y + 58, Math.max(12, r.w), 56, r.mute ? "mute" : ""));
    html.push(r.w > 700
      ? text(`${p}v${i}`, 0, y + 64, r.val, "", { "font-size": 36, width: r.w - 24, "text-align": "right", color: r.mute ? "#f0ece5" : "#111111", "z-index": "3" })
      : text(`${p}v${i}`, Math.max(12, r.w) + 24, y + 62, r.val, "cream", { "font-size": 40 }));
    js.push(inJs(`${p}w${i}`, r.at, { y: 20 }), growJs(`${p}b${i}`, r.at + 0.3, r.w > 300 ? 1.2 : 0.3), inJs(`${p}v${i}`, r.at + (r.w > 300 ? 1.4 : 0.6), { y: 10 }));
  });
  if (foot) { html.push(box(`${p}f`, 0, 680, 960, 110, `<span class="dn" style="font-size: 40px">${foot.text}</span>`, "fill")); js.push(inJs(`${p}f`, foot.at, { y: 30 })); }
  return { css: DIAGRAM_CSS, html: join(html), js: join(js) };
};

const { total } = buildWide({
  series: "bikin database key-value sendiri · bagian 2",
  part: "02 / 06",
  scenes: [
    { type: "title", reg: "orange", dur: 7, still: true, chapter: "Pembuka: Range di Bagian 1",
      kicker: "bikin database key-value sendiri · bagian 2",
      lines: [{ text: "struktur dan", size: 170, light: true }, { text: "pencarian B+tree", size: 170 }],
      lead: "B+tree di memori dengan Go: node, leaf, pembelahan node, dan Range yang urut." },
    { type: "split", dur: 13, kicker: "di bagian 1", title: "map gak nyimpan urutan", visual: recap("s02_"),
      notes: [
        { at: 1.0, html: "data di log, map di memori jadi index" },
        { at: 4.6, html: "Range harus memeriksa semua key, lalu mengurutkannya" },
        { at: 8.6, html: `bagian 2: map diganti ${m("B+tree")}` }] },
    { type: "split", dur: 20, kicker: "pilihan index yang urut", title: "slice urut, atau pohon?", visual: options("s03_"), chapter: "Slice urut atau binary search tree?",
      notes: [
        { at: 1.0, html: `slice urut: ${m("binary search")}, 1 juta key cukup ~20 kali periksa` },
        { at: 5.5, html: "tapi Put di tengah menggeser semua key di kanannya" },
        { at: 10.5, html: "binary search tree: Put gak menggeser, tapi satu key per node" },
        { at: 15.0, html: "tree-nya tinggi. di Bagian 3 tree pindah ke disk" }] },
    { type: "split", dur: 15, kicker: "B+tree", title: "banyak key di setiap node", visual: fanout("s04_"), chapter: "Fan-out: banyak key per node",
      notes: [
        { at: 1.0, html: "satu node nyimpan banyak key, misalnya 64" },
        { at: 4.6, html: `${m("fan-out")}: jumlah anak per node` },
        { at: 9.6, html: `${m("bbolt")} dan SQLite juga pakai b-tree di halaman file` }] },
    { type: "statement", reg: "orange", dur: 7, chapter: "Anatomi B+tree dan Get",
      lines: [{ text: "anatomi B+tree", size: 230 }],
      lead: "root di atas, branch di tengah, leaf di bawah. value cuma ada di leaf.", leadAt: 1.4 },
    { type: "split", dur: 20, kicker: "order 3: paling banyak 3 key per node", title: "dari root sampai leaf",
      visual: tree("s06_", { label: "Get(\"h\")", hi: { at: 10.2, nodes: ["r", "b1", "l3"], edges: ["e1", "e5"] } }),
      notes: [
        { at: 1.0, html: `node paling atas: ${m("root")}. paling bawah: ${m("leaf")}. di antaranya: ${m("branch")}` },
        { at: 5.0, html: "leaf berisi key dan value, dan menunjuk ke leaf berikutnya" },
        { at: 10.0, html: "Get h: satu node per tingkat, dari root sampai leaf" },
        { at: 15.0, html: "branch cuma berisi pemisah. di B-tree biasa, value boleh ada di branch" }] },
    { type: "split", dur: 21, kicker: "Put j", title: "node penuh dibelah dua", visual: splitViz("s07_"), chapter: "Put dan pembelahan node",
      notes: [
        { at: 1.0, html: "Put masuk ke leaf yang benar, leaf tetap urut" },
        { at: 5.5, html: "lebih dari order: leaf dibelah, key pertama kanan disalin ke induk" },
        { at: 10.5, html: "branch kelebihan: dibelah, key tengahnya naik" },
        { at: 15.5, html: "root dibelah: root baru, tree tambah tinggi satu tingkat" }] },
    { type: "split", dur: 16, kicker: "Range(\"d\", \"h\")", title: "turun sekali, lalu ke kanan", chapter: "Range di rantai leaf",
      visual: tree("s08_", { hi: { at: 4.6, chainAt: 2.2, nodes: ["r", "b0", "l1", "l2", "l3"], edges: ["e0", "e3", "ch1", "ch2"] } }),
      notes: [
        { at: 1.0, html: "Range turun ke leaf yang berisi key pertama" },
        { at: 4.6, html: "lalu jalan ke kanan lewat rantai leaf, tanpa kembali ke root" },
        { at: 10.0, html: `berhenti di h. hasilnya ${m("d, e, f, g")}` }] },
    { type: "split", dur: 20, kicker: "menguji B+tree", title: "map sebagai kunci jawaban", chapter: "Menguji dengan map sebagai kunci jawaban",
      cards: [{ head: "git bash · kvku", lines: [
        { cmd: "go test -v ./btree/", at: 0.8 },
        { out: "--- PASS: TestContohKecil", at: 1.8 },
        { out: "--- PASS: TestTimpaValue", at: 2.0 },
        { out: "--- PASS: TestAcakDibandingkanMap", at: 2.2 },
        { out: "  order3, 4, 5, 8, 64", at: 2.4 },
        { out: "ok  kvku/btree  1.217s", at: 2.6, check: true },
        { blank: true },
        { cmd: "go test ./btree/", at: 10.6 },
        { out: "--- FAIL: TestAcakDibandingkanMap", at: 11.4 },
        { out: "FAIL  kvku/btree  0.592s", at: 11.6 }] }],
      notes: [
        { at: 1.0, html: "20.000 key acak ditulis ke B+tree dan ke map sekaligus" },
        { at: 5.0, line: 4, html: "jawabannya harus selalu sama, di lima order" },
        { at: 8.5, html: "check: leaf sedalam sama, key urut, node minimal setengah penuh" },
        { at: 12.5, line: 8, html: `baris ${m("i++")} di fungsi child dihapus: test langsung gagal` }] },
    { type: "split", dur: 17, kicker: "benchmark, 100.000 key", title: "B+tree lawan map",
      visual: pairBars("s10_", [
        { label: "Get di map", w: 88, val: "~38 ns", at: 1.0 },
        { label: "Get di B+tree", w: 960, val: "~414 ns", at: 2.0 },
        { label: "isi 100.000 key: map", w: 302, val: "~14 ms", at: 6.0, mute: true },
      ], { text: "isi 100.000 key di B+tree: ~45 ms", at: 7.0 }), chapter: "B+tree lawan map",
      notes: [
        { at: 1.0, html: "Get di map tetap lebih cepat, sekitar 10 kali" },
        { at: 6.0, html: "mengisi 100.000 key juga sekitar 3 kali lebih lama" },
        { at: 11.0, html: "itu harga urutan: B+tree membandingkan string di beberapa node" }] },
    { type: "split", dur: 17, kicker: "B+tree jadi index kvku", title: "Range 300 kali lebih cepat",
      visual: pairBars("s11_", [
        { label: "Range 10 key · Bagian 1 (map)", w: 960, val: "~16 ms", at: 1.0, mute: true },
        { label: "Range 10 key · Bagian 2 (B+tree)", w: 12, val: "~54 µs", at: 2.6 },
        { label: "buka 1 juta key di terminal", w: 527, val: "~306 ms", at: 9.6 },
      ]), chapter: "Range 300 kali lebih cepat",
      notes: [
        { at: 1.0, html: `${m("logkv")} cuma berubah beberapa baris: index jadi B+tree` },
        { at: 5.0, html: "Range 10 key dari 100.000: ~16 ms jadi ~54 µs" },
        { at: 9.6, html: "sisanya 10 kali baca record dari file. buka 1 juta key: ~0,3 detik" }] },
    { type: "bullets", dur: 13, kicker: "kelemahan B+tree versi memori", title: "yang belum beres", chapter: "Kelemahan dan bagian berikutnya",
      items: [
        { at: 1.5, text: "tree dibangun ulang tiap program mulai" },
        { at: 5.0, text: "semua key masih harus muat di memori" },
        { at: 8.5, text: "Get lebih lambat dari map, belum ada delete" }] },
    { type: "title", reg: "orange", dur: 10,
      kicker: "lanjut bagian 3",
      lines: [{ text: "B+tree di", size: 190, light: true }, { text: "halaman 16 KB", size: 190 }],
      lead: "lalu menimbang mmap. artikel lengkapnya, dengan kode Go dan test-nya: link di deskripsi.", leadAt: 1.4 },
  ],
});
console.log("bagian 2 (YouTube):", total, "s");
