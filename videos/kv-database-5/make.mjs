// Builds the TikTok cut (9:16, ~37 s, captions only) of "Membuat Database Key-Value Sendiri" Bagian 5: the garbage
// copy-on-write leaves, the free list with its one-commit wait, the 75% compaction threshold, compaction that keeps
// readers going, 9 GB of data on a ~7.4 GB RAM laptop, and mmap weighed with numbers. Facts from the article
// (docs/articles/membuat-database-key-value-sendiri/seri-...-bagian-5-...md).
// Run: node make.mjs. Scene kit: ../kv-database-shared/tall.mjs and diagrams.mjs. YouTube version: ../kv-database-5-youtube.
import { buildTall } from "../kv-database-shared/tall.mjs";
import { box, text, nm, cells, arrow, svg, bar, join, inJs, drawJs, growJs } from "../kv-database-shared/diagrams.mjs";

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

// two kinds of garbage
const garbage = (p) => ({ h: 400, html: join(
  box(`${p}i`, 0, 0, 842, 170, `${nm("file index")}${lab("2.093 halaman untuk 2.609 key")}`),
  box(`${p}d`, 0, 210, 842, 170, `${nm("file data")}${lab("29 dari 88 byte masih hidup")}`),
), js: join(inJs(`${p}i`, 0.3, { y: 30 }), inJs(`${p}d`, 2.8, { y: 30 })) });

// page 5: used, pending, free
const freelist = (p) => {
  const st = [["commit 7", "hal. 5 dipakai", ""], ["commit 8", "hal. 5 menunggu", "dash"], ["commit 9", "hal. 5 bebas", "hot"]];
  return { h: 440, html: join(
    st.map(([a, b, c], i) => box(`${p}s${i}`, 0, i * 150, 842, 120, `${nm(a)}${lab(b)}`, c)),
  ), js: join(inJs(`${p}s0`, 0.3, { y: 30 }), inJs(`${p}s1`, 1.4, { y: 30 }), inJs(`${p}s2`, 2.9, { y: 30 })) };
};

// 50% vs 75%: how often a value is written over its life
const threshold = (p) => ({ h: 420, html: join(
  text(`${p}l0`, 0, 0, "ambang 50% · ditulis ~2×", "say", { "font-size": 40 }),
  bar(`${p}b0`, 0, 64, 842, 64, "mute"),
  text(`${p}l1`, 0, 200, "ambang 75% · ditulis ~1,33×", "say", { "font-size": 40 }),
  bar(`${p}b1`, 0, 264, 561, 64),
  text(`${p}v1`, 585, 270, "kvku", "", { "font-size": 48 }),
), js: join(
  inJs(`${p}l0`, 0.3, { y: 20 }), growJs(`${p}b0`, 0.5, 0.8),
  inJs(`${p}l1`, 1.4, { y: 20 }), growJs(`${p}b1`, 1.6, 0.6), inJs(`${p}v1`, 2.2, { x: -20 }),
) });

// compaction: only b=2 and c=2 move to the new file
const compaction = (p) => {
  const c = cells(`${p}c`, 0, 40, ["a=1", "b=1", "a=2", "b=2", "del a", "c=2"].map((t, i) => ({ t, cls: i === 3 || i === 5 ? "" : "dash" })), { w: 142, h: 100 });
  const n = cells(`${p}n`, 0, 300, ["b=2", "c=2"].map((t) => ({ t, cls: "hot" })), { w: 160, h: 100 });
  const a1 = arrow(c.xs[3] + 71, 144, n.xs[0] + 80, 296, 14), a2 = arrow(c.xs[5] + 71, 144, n.xs[1] + 80, 296, 14);
  return { h: 420, html: join(
    text(`${p}ol`, 0, 0, "file data lama", "mute"),
    c.html, n.html,
    svg(842, 420, [{ id: `${p}a1`, d: a1.d }, { id: `${p}a2`, d: a2.d }]),
    text(`${p}nl`, 340, 334, "file data baru", "mute"),
  ), js: join(
    inJs(`${p}ol`, 0.3, { y: 10 }), inJs(c.ids, 0.4, { y: 20 }, { stagger: 0.06 }),
    drawJs(`${p}a1`, 1.3, a1.len, 0.4), drawJs(`${p}a2`, 1.4, a2.len, 0.4),
    inJs(n.ids, 1.7, { y: 20 }, { stagger: 0.1 }), inJs(`${p}nl`, 1.9, { x: -20 }),
  ) };
};

// mmap vs ReadAt per 4 KB on the 9.3 GB file
const mmap = (p) => ({ h: 470, html: join(
  text(`${p}h0`, 0, 0, "dari disk", "up mute"),
  text(`${p}r0`, 0, 44, "ReadAt 179,7 µs · mmap 259,7 µs", "say", { "font-size": 40 }),
  text(`${p}h1`, 0, 160, "dari page cache", "up mute"),
  text(`${p}l1`, 0, 204, "ReadAt", "say", { "font-size": 40 }),
  bar(`${p}b1`, 0, 260, 760, 60, "mute"),
  text(`${p}v1`, 0, 266, "8,8 µs", "", { "font-size": 40, width: 736, "text-align": "right", color: "#f0ece5", "z-index": "3" }),
  text(`${p}l2`, 0, 340, "mmap", "say", { "font-size": 40 }),
  bar(`${p}b2`, 0, 396, 43, 60),
  text(`${p}v2`, 67, 400, "0,5 µs", "", { "font-size": 44 }),
), js: join(
  inJs([`${p}h0`, `${p}r0`], 0.3, { y: 20 }, { stagger: 0.15 }),
  inJs([`${p}h1`, `${p}l1`], 2.6, { y: 20 }, { stagger: 0.1 }), growJs(`${p}b1`, 2.8, 0.7), inJs(`${p}v1`, 3.4, { y: 10 }),
  inJs(`${p}l2`, 3.0, { y: 20 }), growJs(`${p}b2`, 3.2, 0.2), inJs(`${p}v2`, 3.4, { x: -20 }),
) });

const total = buildTall({
  series: "bikin database key-value sendiri",
  part: "05 / 06",
  hook: { dur: 3.2, still: true, kicker: "bagian 5 / 6",
    lines: [{ text: "compaction,", size: 130, light: true }, { text: "data 9 GB,", size: 150 }, { text: "RAM 7,4 GB", size: 150 }],
    lead: "database key-value sendiri, pakai Go" },
  steps: [
    { kicker: "setelah bagian 4", title: "sampah di file", dur: 5.0, visual: garbage("step1_"),
      marks: [
        { at: 0.8, cap: "copy-on-write ninggalin halaman lama" },
        { at: 2.8, cap: "value yang diganti ninggalin record lama" }] },
    { kicker: "free list", title: "dipakai lagi", dur: 5.2, visual: freelist("step2_"),
      marks: [
        { at: 0.8, cap: "commit 7 masih butuh halaman itu sebagai cadangan" },
        { at: 2.9, cap: "jadi menunggu satu commit, lalu dipakai lagi" }] },
    { kicker: "compaction", title: "ambang 75%", dur: 5.2, visual: threshold("step3_"),
      marks: [
        { at: 0.8, cap: "Sync mulai compaction saat sampah lewat 75%" },
        { at: 2.9, cap: "Badger menyarankan 50%: lebih sering menyalin" }] },
    { kicker: "compaction", title: "pembaca jalan terus", dur: 5.2, visual: compaction("step4_"),
      marks: [
        { at: 0.8, cap: "cuma value yang hidup yang disalin" },
        { at: 2.9, cap: "58.542 Get selesai selama compaction" }] },
    { kicker: "data 9,3 GB", title: "Get dari disk", dur: 5.0,
      visual: stat("step5_", "441", "µs", 560, "per Get, putaran pertama, dari disk", "putaran kedua, dari memori: ~50 µs"),
      marks: [
        { at: 0.8, cap: "8 juta key di laptop ber-RAM ~7,4 GB" },
        { at: 2.8, cap: "heap Go ~114 MB, ikut ukuran cache" }] },
    { kicker: "mmap lagi", title: "dengan angka", dur: 5.2, visual: mmap("step6_"),
      marks: [
        { at: 0.8, cap: `dari disk: ${m("mmap")} gak menang` },
        { at: 2.9, cap: "dari memori: belasan kali lebih cepat" }] },
  ],
  end: { dur: 3.6, kicker: "lanjut bagian 6 · terakhir",
    lines: [{ text: "kvku lawan", size: 130, light: true }, { text: "bbolt", size: 160 }, { text: "dan Badger", size: 140 }],
    lead: "diukur di laptop yang sama" },
  endSfx: { file: "ting.wav", at: 0.3, dur: 0.45, vol: 0.4 },
});
console.log("bagian 5 (TikTok):", total, "s");
