// Builds the TikTok cut (9:16, ~37 s, captions only) of "Membuat Message Broker Sendiri" Bagian 2: the read position
// as one number, slots instead of mmap, the 512-byte slot, the write order, what the broker checks on reopen, IDs
// reserved per 1,000. Facts from the article (docs/articles/membuat-message-broker-sendiri/seri-...-bagian-2-...md).
// Run: node make.mjs. Scene kit: ../message-broker-shared/tall.mjs and diagrams.mjs. YouTube: ../message-broker-2-youtube.
import { buildTall } from "../message-broker-shared/tall.mjs";
import { box, text, nm, sm, big, cells, pointer, arrow, svg, vline, join, inJs, drawJs, hotJs, moveJs } from "../message-broker-shared/diagrams.mjs";

const m = (s) => `<span class="m">${s}</span>`;

// the read position is a byte position: email's 72 is where record 3 starts
const bytePos = (p) => {
  const c = cells(`${p}c`, 0, 50, ["ID 1", "ID 2", "ID 3"].map((t) => ({ t })), { w: 250, h: 110 });
  const ox = [...c.xs, c.end - 3], offs = ["0", "36", "72", "108"];
  return { h: 350, html: join(
    text(`${p}l`, 0, 0, "log topic pesanan", "up mute"),
    c.html,
    offs.map((o, i) => text(`${p}o${i}`, ox[i] - (i ? o.length * 9 : 0), 172, o, "mute")),
    pointer(`${p}pe`, c.xs[2] + 1, 222, "email: 72", { len: 60, w: 230, h: 64 }),
  ), js: join(
    inJs(`${p}l`, 0.3, { y: 10 }), inJs(c.ids, 0.4, { y: 30 }, { stagger: 0.1 }),
    inJs(offs.map((_, i) => `${p}o${i}`), 2.6, { y: 10 }, { stagger: 0.08 }), inJs(`${p}pe`, 2.9, { y: 40 }), hotJs(c.ids[2], 3.1, { text: true }),
  ) };
};

// WriteAt into a slot vs a memory-mapped file (measured: ~2.8 µs vs ~2.8 ns per 8-byte write); the slot is chosen
const speeds = (p) => {
  const val = (s) => `<span class="dbig" style="font-size: 72px; color: #f0ece5">${s}</span>`;
  return { h: 420, html: join(
    box(`${p}a`, 0, 0, 842, 190, `${nm("slot + WriteAt")}${val("~2,8 µs")}`),
    box(`${p}b`, 0, 220, 842, 190, `${nm("mmap")}${val("~2,8 ns")}${sm("seribu kali lebih cepat")}`),
    text(`${p}t`, 650, 16, "dipilih", "", { "font-size": 30, "z-index": "3" }),
  ), js: join(
    inJs(`${p}a`, 0.3, { y: 30 }), inJs(`${p}b`, 0.8, { y: 30 }),
    hotJs(`${p}a`, 2.8), inJs(`${p}t`, 2.9, { x: 20 }),
  ) };
};

// one subscriber slot (512 bytes): an ack writes only the first 8
const slot = (p) => {
  const f = [["0-7", "posisi baca"], ["8", "dipakai"], ["9", "jenis"], ["10-11", "panjang nama"], ["12-75", "nama topic"], ["76-139", "nama subscriber"], ["140-511", "cadangan"]];
  const row = (a, b) => `<div style="display: flex; width: 100%; gap: 22px; align-items: baseline"><span class="ds" style="width: 150px">${a}</span>${nm(b)}</div>`;
  return { h: 520, html: join(f.map(([a, b], i) => box(`${p}f${i}`, 0, i * 73, 842, 76, row(a, b), "l"))),
    js: join(inJs(f.map((_, i) => `${p}f${i}`), 0.3, { x: 50 }, { stagger: 0.07 }), hotJs(`${p}f0`, 2.6)) };
};

// every second: fsync the logs first, then the metadata
const order = (p) => {
  const a = arrow(421, 318, 421, 362);
  return { h: 490, html: join(
    box(`${p}w`, 0, 0, 842, 110, `${nm("publish dan ack")}${sm("ditulis ke OS, tanpa fsync")}`),
    text(`${p}l`, 0, 150, "setiap detik", "up mute"),
    box(`${p}f1`, 0, 200, 842, 110, nm("1 · fsync semua log"), "hot"),
    svg(842, 490, [{ id: `${p}a`, d: a.d }]),
    box(`${p}f2`, 0, 370, 842, 110, nm("2 · fsync metadata"), "hot"),
  ), js: join(
    inJs(`${p}w`, 0.3, { y: 30 }), inJs([`${p}l`, `${p}f1`], 0.8, { y: 30 }, { stagger: 0.1 }), drawJs(`${p}a`, 1.3, a.len, 0.3), inJs(`${p}f2`, 1.5, { y: 30 }),
  ) };
};

// on reopen: a position behind the log's end resends, one ahead of it moves back
const reopen = (p) => {
  const cw = { w: 118, h: 100 };
  const a = cells(`${p}a`, 0, 50, [1, 2, 3, 4, 5].map((t) => ({ t })), cw);
  const b = cells(`${p}b`, 0, 350, [{ t: 1 }, { t: 2 }, { t: 3, cls: "dash" }], cw);
  return { h: 580, html: join(
    text(`${p}al`, 0, 0, "posisi di belakang ujung log", "", { "font-size": 30 }), a.html, vline(`${p}ae`, a.end + 8, 36, 130),
    pointer(`${p}ap`, a.xs[2] + 59, 152, "email", { len: 48, w: 150, h: 58 }),
    text(`${p}al2`, a.end + 30, 70, "dikirim<br>lagi", "say", { "font-size": 38 }),
    text(`${p}bl`, 0, 300, "posisi di depan ujung log", "", { "font-size": 30 }), b.html, vline(`${p}be`, b.xs[2] + 2, 336, 130),
    pointer(`${p}bp`, b.end + 6, 452, "email", { len: 48, w: 150, h: 58 }),
    text(`${p}bl2`, b.end + 60, 380, "dimundurkan", "say", { "font-size": 38 }),
  ), js: join(
    inJs([`${p}al`, ...a.ids, `${p}ae`], 0.3, { y: 20 }, { stagger: 0.05 }), inJs(`${p}ap`, 0.8, { y: 30 }), inJs(`${p}al2`, 1.2, { x: -20 }),
    inJs([`${p}bl`, ...b.ids, `${p}be`], 2.6, { y: 20 }, { stagger: 0.05 }), inJs(`${p}bp`, 3.0, { y: 30 }),
    moveJs(`${p}bp`, 3.5, -(b.end + 6 - (b.xs[2] + 2)), 0.5), inJs(`${p}bl2`, 3.8, { x: -20 }),
  ) };
};

// IDs reserved per 1,000: after a crash the next message gets 1001
const reserve = (p) => {
  const c = cells(`${p}c`, 0, 0, [1, 2, 3].map((t) => ({ t })), { w: 110, h: 100 });
  return { h: 330, html: join(
    c.html,
    text(`${p}r`, c.end + 34, 30, "cadangan: ID 1000", "cream", { "font-size": 34 }),
    box(`${p}n`, 0, 160, 842, 160, `${big("1001")}${sm("ID pesan berikutnya")}`, "fill"),
  ), js: join(
    inJs(c.ids, 0.3, { y: 30 }, { stagger: 0.08 }), inJs(`${p}r`, 0.9, { x: -20 }),
    `      tl.fromTo($("${p}n"), { scale: 0.85, opacity: 0 }, { scale: 1, opacity: 1, duration: 0.5, ease: "back.out(1.6)" }, 2.9);`,
  ) };
};

const total = buildTall({
  series: "bikin message broker sendiri",
  part: "02 / 06",
  hook: { dur: 3.2, still: true, kicker: "bagian 2 / 6",
    lines: [{ text: "broker mati,", size: 130 }, { text: "lanjut", size: 130, light: true }, { text: "dari mana?", size: 130, light: true }],
    lead: "mengingat posisi baca setiap subscriber" },
  steps: [
    { kicker: "posisi baca", title: "cukup satu angka", dur: 4.8, visual: bytePos("step1_"),
      marks: [
        { at: 0.8, cap: "pesan di log selalu berurutan" },
        { at: 2.6, cap: "posisi baca: letak byte record berikutnya" }] },
    { kicker: "nyimpan di mana?", title: "slot, bukan mmap", dur: 5.2, visual: speeds("step2_"),
      marks: [
        { at: 0.8, cap: "mmap seribu kali lebih cepat" },
        { at: 2.8, cap: "tapi OS yang pegang kendali. broker pilih slot" }] },
    { kicker: "file metadata", title: "slot 512 byte", dur: 4.8, visual: slot("step3_"),
      marks: [
        { at: 0.6, cap: "satu slot untuk satu subscriber" },
        { at: 2.5, cap: "satu ack cuma nulis 8 byte di awalnya" }] },
    { kicker: "tiap detik", title: "log, lalu metadata", dur: 5.0, visual: order("step4_"),
      marks: [
        { at: 0.8, cap: "log selalu di-fsync sebelum metadata" },
        { at: 2.8, cap: "listrik padam: hilang paling lama ~1 detik" }] },
    { kicker: "saat dibuka lagi", title: "ikuti ujung log", dur: 5.2, visual: reopen("step5_"),
      marks: [
        { at: 0.8, cap: "ack belum sampai ke disk? pesannya dikirim lagi" },
        { at: 2.8, cap: "record gak sampai ke disk? posisi dimundurkan" }] },
    { kicker: "ID gak boleh kembar", title: "cadangan per 1.000", dur: 4.8, visual: reserve("step6_"),
      marks: [
        { at: 0.8, cap: "ID dicadangkan per 1.000, dicatat di disk dulu" },
        { at: 2.8, cap: "broker mati mendadak? ID lanjut dari 1001" }] },
  ],
  end: { dur: 3.6, kicker: "lanjut bagian 3",
    lines: [{ text: "subscriber", size: 140, light: true }, { text: "pull", size: 230 }],
    lead: "subscriber yang menarik pesannya sendiri" },
  endSfx: { file: "ting.wav", at: 0.3, dur: 0.45, vol: 0.4 },
});
console.log("bagian 2 (TikTok):", total, "s");
