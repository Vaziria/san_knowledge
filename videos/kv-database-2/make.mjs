// Builds the TikTok cut (9:16, ~37 s, captions only) of "Membuat Database Key-Value Sendiri" Bagian 2: Range on a map
// index, fan-out, the B+tree's anatomy and Get, node splits, Range on the leaf chain, and the 300x faster Range. Facts
// from the article (docs/articles/membuat-database-key-value-sendiri/seri-...-bagian-2-...md). Run: node make.mjs.
// Scene kit: ../kv-database-shared/tall.mjs and diagrams.mjs. YouTube version: ../kv-database-2-youtube.
import { buildTall } from "../kv-database-shared/tall.mjs";
import { box, text, nm, arrow, svg, bar, join, inJs, drawJs, hotJs, growJs } from "../kv-database-shared/diagrams.mjs";

const m = (s) => `<span class="m">${s}</span>`;
const kn = (s) => nm(s.split(" ").join("&nbsp;&nbsp;&nbsp;"));

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

// fan-out
const fanout = (p) => ({ h: 380, html: join(
  text(`${p}n`, 0, 0, "64 × 64 × 64", "stat", { "font-size": 150 }),
  text(`${p}r`, 0, 170, "= 262.144 leaf", "say", { "font-size": 64 }),
  text(`${p}a`, 0, 290, "fan-out: jumlah anak per node", "cream", { "font-size": 34 }),
), js: join(
  inJs(`${p}n`, 0.35, { y: 30 }), inJs(`${p}r`, 1.4, { x: -30 }), inJs(`${p}a`, 2.8, { y: 20 }),
) });

// the article's order-3 tree, in 842 px: root g; branches [c e], [i]; leaves ab cd ef gh ij
const LX = [0, 173, 346, 519, 692], LW = 150, LY = 330;
function tree(p, hi) {
  const br = [{ id: `${p}b0`, x: 150, w: 200, t: "c e" }, { id: `${p}b1`, x: 560, w: 160, t: "i" }];
  const cx = (b) => b.x + b.w / 2;
  const edges = [
    arrow(421, 90, cx(br[0]), 176, 12), arrow(421, 90, cx(br[1]), 176, 12),
    ...[0, 1, 2].map((i) => arrow(cx(br[0]), 260, LX[i] + LW / 2, LY - 4, 12)),
    ...[3, 4].map((i) => arrow(cx(br[1]), 260, LX[i] + LW / 2, LY - 4, 12)),
  ];
  const chain = [0, 1, 2, 3].map((i) => arrow(LX[i] + LW, LY + 42, LX[i + 1] - 2, LY + 42, 9));
  const hot = hi.edges.map((e) => (e.startsWith("ch") ? chain[+e.slice(2)] : edges[+e.slice(1)]));
  return { h: 420, html: join(
    svg(842, 420, [...edges.map((a, i) => ({ id: `${p}e${i}`, d: a.d, cls: "mute thin" })), ...chain.map((a, i) => ({ id: `${p}c${i}`, d: a.d, cls: "cream thin" })),
      ...hot.map((a, i) => ({ id: `${p}h${i}`, d: a.d }))]),
    box(`${p}r`, 331, 0, 180, 86, kn("g")),
    br.map((b) => box(b.id, b.x, 180, b.w, 82, kn(b.t))),
    ["a b", "c d", "e f", "g h", "i j"].map((t, i) => box(`${p}l${i}`, LX[i], LY, LW, 84, kn(t))),
  ), js: join(
    inJs(`${p}r`, 0.3, { y: 20 }), inJs(br.map((b) => b.id), 0.5, { y: 20 }, { stagger: 0.08 }),
    inJs([0, 1, 2, 3, 4].map((i) => `${p}l${i}`), 0.7, { y: 20 }, { stagger: 0.06 }),
    edges.map((a, i) => drawJs(`${p}e${i}`, 0.8, a.len, 0.3)),
    chain.map((a, i) => drawJs(`${p}c${i}`, 1.1 + i * 0.1, a.len, 0.25)),
    hi.nodes.map((id, i) => hotJs(`${p}${id}`, hi.at + i * 0.35, { text: true })),
    hot.map((a, i) => drawJs(`${p}h${i}`, hi.at + 0.15 + i * 0.35, a.len, 0.3)),
  ) };
}

// Put j: the leaf splits (i copied up), then the branch (g moves up)
const split = (p) => {
  const row = (r, y, from, a, b, note) => {
    const ar = arrow(270, y + 42, 350, y + 42, 12);
    return { html: join(
      box(`${p}f${r}`, 0, y, 260, 84, kn(from), "dash"),
      svg(842, 420, [{ id: `${p}a${r}`, d: ar.d, cls: "cream thin" }]),
      box(`${p}x${r}`, 362, y, 230, 84, kn(a)),
      box(`${p}y${r}`, 612, y, 230, 84, kn(b)),
      text(`${p}n${r}`, 362, y + 96, note, "cream"),
    ), len: ar.len };
  };
  const r0 = row(0, 0, "g h i j", "g h", "i j", "i disalin ke induk");
  const r1 = row(1, 210, "c e g i", "c e", "i", "g naik ke induk");
  return { h: 360, html: join(r0.html, r1.html), js: join(
    inJs(`${p}f0`, 0.3, { y: 20 }), drawJs(`${p}a0`, 0.8, r0.len, 0.3), inJs([`${p}x0`, `${p}y0`, `${p}n0`], 1.0, { x: -30 }, { stagger: 0.1 }),
    inJs(`${p}f1`, 2.7, { y: 20 }), drawJs(`${p}a1`, 3.0, r1.len, 0.3), inJs([`${p}x1`, `${p}y1`, `${p}n1`], 3.2, { x: -30 }, { stagger: 0.1 }),
  ) };
};

// Range 10 keys: 16 ms with the map, 54 µs with the B+tree
const bars = (p) => ({ h: 420, html: join(
  text(`${p}w1`, 0, 0, "map (Bagian 1)", "say", { "font-size": 46 }),
  bar(`${p}b1`, 0, 70, 842, 70, "mute"),
  text(`${p}v1`, 0, 155, "~16 ms", "cream", { "font-size": 52 }),
  text(`${p}w2`, 0, 245, "B+tree", "say", { "font-size": 46 }),
  bar(`${p}b2`, 0, 315, 12, 70),
  text(`${p}v2`, 36, 321, "~54 µs", "", { "font-size": 52 }),
), js: join(
  inJs(`${p}w1`, 0.3, { y: 20 }), growJs(`${p}b1`, 0.5, 1.0), inJs(`${p}v1`, 1.3, { y: 10 }),
  inJs(`${p}w2`, 1.6, { y: 20 }), growJs(`${p}b2`, 1.8, 0.3), inJs(`${p}v2`, 2.0, { x: -20 }),
) });

const total = buildTall({
  series: "bikin database key-value sendiri",
  part: "02 / 06",
  hook: { dur: 3.2, still: true, kicker: "bagian 2 / 6",
    lines: [{ text: "struktur dan", size: 130, light: true }, { text: "pencarian", size: 160 }, { text: "B+tree", size: 160 }],
    lead: "database key-value sendiri, pakai Go" },
  steps: [
    { kicker: "di bagian 1", title: "map gak urut", dur: 5.0,
      visual: stat("step1_", "16", "ms", 330, "Range 10 key dari 100.000 key", "semua key diperiksa, lalu diurutkan"),
      marks: [
        { at: 0.8, cap: "map jadi index: cepat, tapi gak nyimpan urutan" },
        { at: 2.8, cap: `bagian 2: map diganti ${m("B+tree")}` }] },
    { kicker: "B+tree", title: "banyak key per node", dur: 5.0, visual: fanout("step2_"),
      marks: [
        { at: 0.8, cap: "satu node nyimpan banyak key, misalnya 64" },
        { at: 2.8, cap: "jadi tree-nya pendek, walau key-nya banyak" }] },
    { kicker: "anatomi", title: "Get(\"h\")", dur: 5.2, visual: tree("step3_", { at: 2.8, nodes: ["r", "b1", "l3"], edges: ["e1", "e5"] }),
      marks: [
        { at: 0.8, cap: "root, branch, leaf. value cuma ada di leaf" },
        { at: 2.8, cap: "Get: satu node per tingkat, dari root ke leaf" }] },
    { kicker: "Put j", title: "node penuh dibelah", dur: 5.4, visual: split("step4_"),
      marks: [
        { at: 0.8, cap: "leaf kelebihan key: dibelah dua" },
        { at: 2.9, cap: "branch ikut penuh: dibelah, key tengah naik" }] },
    { kicker: "Range(\"d\", \"h\")", title: "rantai leaf", dur: 5.2,
      visual: tree("step5_", { at: 1.6, nodes: ["r", "b0", "l1", "l2", "l3"], edges: ["e0", "e3", "ch1", "ch2"] }),
      marks: [
        { at: 0.8, cap: "turun sekali ke leaf yang berisi d" },
        { at: 3.0, cap: `lalu jalan ke kanan: ${m("d, e, f, g")}` }] },
    { kicker: "hasilnya", title: "300 kali lebih cepat", dur: 5.0, visual: bars("step6_"),
      marks: [
        { at: 0.8, cap: "Range 10 key dari 100.000 key" },
        { at: 2.8, cap: "harganya: Get ~414 ns, di map cuma ~38 ns" }] },
  ],
  end: { dur: 3.6, kicker: "lanjut bagian 3",
    lines: [{ text: "B+tree", size: 170 }, { text: "di file", size: 150, light: true }],
    lead: "halaman 16 KB, lalu menimbang mmap" },
  endSfx: { file: "ting.wav", at: 0.3, dur: 0.45, vol: 0.4 },
});
console.log("bagian 2 (TikTok):", total, "s");
