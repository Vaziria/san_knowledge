// Builds the TikTok cut (9:16, ~37 s, captions only) of "Membuat Database Key-Value Sendiri" Bagian 4: overwriting pages
// in place, copy-on-write copying the path to the root, the two metas A and B, values in an append-only data file with
// tombstones, merge on Delete, and 94 simulated power cuts. Facts from the article
// (docs/articles/membuat-database-key-value-sendiri/seri-...-bagian-4-...md).
// Run: node make.mjs. Scene kit: ../kv-database-shared/tall.mjs and diagrams.mjs. YouTube version: ../kv-database-4-youtube.
import { buildTall } from "../kv-database-shared/tall.mjs";
import { box, text, nm, arrow, svg, bar, join, inJs, drawJs, hotJs, growJs } from "../kv-database-shared/diagrams.mjs";

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

// what Sync should write, and what is in the file after the power fails
const risk = (p) => {
  const X = [0, 286, 572], W = 270;
  const top = [["hal. 2", "leaf baru"], ["hal. 3 root", "+ pemisah s"], ["hal. 4 baru", "s sampai z"]];
  const bot = [["hal. 2", "sudah baru", ""], ["hal. 3 root", "masih lama", "hot"], ["hal. 4", "belum ada", "dash"]];
  return { h: 400, html: join(
    text(`${p}tl`, 0, 0, "seharusnya", "up mute"),
    top.map(([a, b], i) => box(`${p}t${i}`, X[i], 44, W, 130, `${nm(a)}${lab(b)}`)),
    text(`${p}bl`, 0, 214, "setelah listrik padam", "up mute"),
    bot.map(([a, b, c], i) => box(`${p}b${i}`, X[i], 258, W, 130, `${nm(a)}${lab(b)}`, c)),
  ), js: join(
    inJs(`${p}tl`, 0.3, { y: 10 }), inJs(top.map((_, i) => `${p}t${i}`), 0.4, { y: 30 }, { stagger: 0.1 }),
    inJs(`${p}bl`, 2.6, { y: 10 }), inJs(bot.map((_, i) => `${p}b${i}`), 2.7, { y: 30 }, { stagger: 0.2 }),
  ) };
};

// copy-on-write: the old path on the left, the copied path on the right, page 8 shared
const cow = (p) => {
  const W = 250, H = 110;
  const nodes = [["o0", 0, 0, "hal. 9", "root lama", ""], ["o1", 0, 170, "hal. 4", "branch lama", ""], ["o2", 0, 340, "hal. 5", "leaf lama", ""],
    ["n0", 592, 0, "hal. 14", "root baru", "hot"], ["n1", 592, 170, "hal. 13", "salinan", "hot"], ["n2", 592, 340, "hal. 12", "+ key m", "fill"],
    ["s", 271, 255, "hal. 8", "milik bersama", "mute"]];
  const up = [arrow(717, 340, 717, 284, 14), arrow(717, 170, 717, 114, 14)];
  return { h: 460, html: join(
    svg(842, 460, up.map((a, i) => ({ id: `${p}a${i}`, d: a.d }))),
    nodes.map(([k, x, y, a, b, c]) => box(`${p}${k}`, x, y, k === "s" ? 300 : W, H, `${nm(a)}${lab(b)}`, c)),
  ), js: join(
    inJs([`${p}o0`, `${p}o1`, `${p}o2`], 0.3, { y: 20 }, { stagger: 0.1 }),
    inJs(`${p}n2`, 1.2, { x: -40 }), drawJs(`${p}a0`, 1.6, 120, 0.3),
    inJs(`${p}n1`, 1.9, { x: -40 }), drawJs(`${p}a1`, 2.3, 120, 0.3),
    inJs(`${p}n0`, 2.6, { x: -40 }), inJs(`${p}s`, 3.2, { y: 20 }),
  ) };
};

// page 0 holds meta A and meta B
const metas = (p) => ({ h: 400, html: join(
  text(`${p}l`, 0, 0, "halaman 0", "up mute"),
  box(`${p}a`, 0, 50, 420, 150, `${nm("meta A")}${lab("commit 7")}`),
  box(`${p}b`, 417, 50, 425, 150, `${nm("meta B")}${lab("commit 8")}`),
  text(`${p}s`, 0, 250, "padam saat menulis meta B?", "say", { width: 842 }),
  text(`${p}t`, 0, 310, "meta A masih utuh: Open pakai commit 7", "say mute", { width: 842 }),
), js: join(
  inJs(`${p}l`, 0.3, { y: 10 }), inJs([`${p}a`, `${p}b`], 0.4, { y: 30 }, { stagger: 0.15 }),
  hotJs(`${p}b`, 1.4),
  inJs(`${p}s`, 2.9, { y: 20 }), inJs(`${p}t`, 3.4, { y: 20 }),
) });

// the data file: records only ever appended; the leaf points at the live one
const data = (p) => {
  const rec = [["user:1 = Andi", "mute"], ["user:2 = Budi", "mute"], ["user:2 = Budi Santoso", "hot"], ["tombstone user:1", "dash"]];
  return { h: 510, html: join(
    text(`${p}l`, 0, 0, "file data · 88 byte", "up mute"),
    rec.map(([a, c], i) => box(`${p}r${i}`, 0, 50 + i * 100, 842, 86, nm(a), `l ${c}`)),
    text(`${p}s`, 0, 462, "cuma 29 byte yang masih ditunjuk tree", "", { "font-size": 32 }),
  ), js: join(
    inJs(`${p}l`, 0.3, { y: 10 }), inJs([0, 1, 2].map((i) => `${p}r${i}`), 0.4, { x: -40 }, { stagger: 0.3 }),
    inJs(`${p}r3`, 2.9, { x: -40 }), inJs(`${p}s`, 3.6, { y: 10 }),
  ) };
};

// merge: a leaf under a quarter page joins its sibling
const merge = (p) => {
  const leaf = (id, x, y, w, pct, label, cls = "mute") => join(
    box(id, x, y, w, 90, "", "flat", { background: "transparent" }),
    bar(`${id}b`, x + 3, y + 3, Math.round((w - 6) * pct / 100), 84, cls),
    text(`${id}t`, x, y + 104, label, "cream", { "font-size": 32 }),
  );
  return { h: 420, html: join(
    leaf(`${p}a`, 0, 0, 400, 60, "a sampai f · 60%"), leaf(`${p}g`, 442, 0, 400, 10, "g · 10%"),
    leaf(`${p}c`, 0, 240, 842, 70, "digabung: a sampai g · 70%", ""),
  ), js: join(
    inJs([`${p}a`, `${p}at`, `${p}g`, `${p}gt`], 0.3, { y: 20 }, { stagger: 0.05 }),
    growJs(`${p}ab`, 0.5, 0.5), growJs(`${p}gb`, 0.7, 0.3),
    inJs([`${p}c`, `${p}ct`], 2.6, { y: 30 }), growJs(`${p}cb`, 2.8, 0.6),
  ) };
};

const total = buildTall({
  series: "bikin database key-value sendiri",
  part: "04 / 06",
  hook: { dur: 3.2, still: true, kicker: "bagian 4 / 6",
    lines: [{ text: "update,", size: 130, light: true }, { text: "delete,", size: 160 }, { text: "gak rusak", size: 160 }],
    lead: "database key-value sendiri, pakai Go" },
  steps: [
    { kicker: "di bagian 3", title: "menimpa halaman", dur: 5.0, visual: risk("step1_"),
      marks: [
        { at: 0.8, cap: "Sync menimpa halaman di tempatnya" },
        { at: 2.8, cap: "listrik padam di tengah: leaf baru, root lama" }] },
    { kicker: "copy-on-write", title: "jangan menimpa", dur: 5.4, visual: cow("step2_"),
      marks: [
        { at: 0.8, cap: "halaman yang berubah ditulis ke halaman baru" },
        { at: 3.0, cap: "induknya ikut disalin, sampai root" }] },
    { kicker: "dua meta", title: "A dan B", dur: 5.2, visual: metas("step3_"),
      marks: [
        { at: 0.8, cap: "commit genap ke A, commit ganjil ke B" },
        { at: 2.9, cap: "Open pilih meta utuh dengan commit terbesar" }] },
    { kicker: "value pindah", title: "file data", dur: 5.2, visual: data("step4_"),
      marks: [
        { at: 0.8, cap: "file data cuma bertambah, leaf nyimpan posisinya" },
        { at: 2.9, cap: `delete menulis ${m("tombstone")}` }] },
    { kicker: "delete", title: "node digabung", dur: 5.0, visual: merge("step5_"),
      marks: [
        { at: 0.8, cap: "node di bawah 25% digabung dengan saudaranya" },
        { at: 2.8, cap: "gak muat? dibelah lagi di tengah" }] },
    { kicker: "listrik padam", title: "94 kali", dur: 5.0,
      visual: stat("step6_", "94", "titik padam", 340, "tiap penulisan dalam commit dipotong", "semuanya kembali ke commit lama"),
      marks: [
        { at: 0.8, cap: "disimulasikan di setiap penulisan" },
        { at: 2.8, cap: "gak ada satu pun tree campuran" }] },
  ],
  end: { dur: 3.6, kicker: "lanjut bagian 5",
    lines: [{ text: "compaction,", size: 130, light: true }, { text: "data 9 GB", size: 150 }],
    lead: "free list, compaction, data lebih besar dari RAM" },
  endSfx: { file: "ting.wav", at: 0.3, dur: 0.45, vol: 0.4 },
});
console.log("bagian 4 (TikTok):", total, "s");
