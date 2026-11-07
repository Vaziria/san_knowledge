// Builds the TikTok cut (9:16, ~37 s, captions only) of "Membuat Message Broker Sendiri" Bagian 1: one log per topic,
// AMQP vs four HTTP addresses, the record, write vs fsync, torn writes. Facts from the article
// (docs/articles/membuat-message-broker-sendiri/seri-...-bagian-1-...md). Run: node make.mjs (writes index.html and
// compositions/). Scene kit: ../message-broker-shared/tall.mjs and diagrams.mjs. YouTube version: ../message-broker-1-youtube.
import { buildTall } from "../message-broker-shared/tall.mjs";
import { box, text, nm, sm, cells, pointer, vline, bar, join, inJs, outJs, hotJs, growJs } from "../message-broker-shared/diagrams.mjs";

const m = (s) => `<span class="m">${s}</span>`;

// one log, one read position per subscriber
const log = (p) => {
  const c = cells(`${p}c`, 0, 60, [1, 2, 3, 4, 5, 6].map((n) => ({ t: n })), { w: 118, h: 110 });
  return { h: 330, html: join(
    text(`${p}l`, 0, 0, "log topic pesanan", "up mute"),
    c.html,
    vline(`${p}e`, c.end + 12, 40, 150),
    text(`${p}el`, c.end + 26, 96, "ujung"),
    pointer(`${p}pe`, c.xs[3] + 59, 172, "email", { len: 70, w: 170, h: 64 }),
    pointer(`${p}ps`, c.end + 12, 172, "stok", { len: 70, w: 150, h: 64 }),
  ), js: join(
    inJs(`${p}l`, 0.3, { y: 10 }), inJs(c.ids, 0.4, { y: 30 }, { stagger: 0.08 }), inJs([`${p}e`, `${p}el`], 0.9, { y: 0 }),
    inJs(`${p}pe`, 2.9, { y: 40 }), inJs(`${p}ps`, 3.1, { y: 40 }),
  ) };
};

// AMQP 0-9-1 counted from its spec
const amqp = (p) => ({ h: 400, html: join(
  text(`${p}n`, 0, 0, "53", "stat", { "font-size": 280 }),
  text(`${p}u`, 330, 120, "method", "say", { "font-size": 76 }),
  text(`${p}a`, 0, 290, "6 class · 4 jenis frame biner", "cream", { "font-size": 34 }),
  text(`${p}b`, 0, 345, "6 langkah sebelum pesan pertama", "cream", { "font-size": 34 }),
), js: join(
  `      tl.fromTo($("${p}n"), { scale: 0.6, opacity: 0, transformOrigin: "0% 50%" }, { scale: 1, opacity: 1, duration: 0.5, ease: "back.out(1.6)" }, 0.35);`,
  inJs(`${p}u`, 0.6, { x: -30 }), inJs([`${p}a`, `${p}b`], 1.2, { y: 20 }, { stagger: 0.3 }),
) });

// the record: five fields stacked like memory, the CRC highlighted
const record = (p) => {
  const f = [["0", "panjang", "4 byte"], ["4", "CRC-32C", "4 byte"], ["8", "ID", "8 byte"], ["16", "waktu", "8 byte"], ["24", "data", "isi pesan"]];
  const row = (a, b) => `<div style="display: flex; width: 100%; justify-content: space-between; align-items: baseline">${nm(a)}${sm(b)}</div>`;
  return { h: 440, html: join(
    f.map(([o, a, b], i) => join(text(`${p}o${i}`, 0, i * 85 + 26, o, "mute"), box(`${p}f${i}`, 90, i * 85, 752, 88, row(a, b), "l"))),
  ), js: join(
    inJs(f.map((_, i) => `${p}f${i}`), 0.35, { x: 60 }, { stagger: 0.1 }), inJs(f.map((_, i) => `${p}o${i}`), 0.5, { y: 0 }, { stagger: 0.1 }),
    hotJs(`${p}f1`, 2.7),
  ) };
};

// write vs write + fsync (3556 vs 470182 ns per message on the author's laptop)
const bars = (p) => ({ h: 420, html: join(
  text(`${p}w1`, 0, 0, "write", "say", { "font-size": 46 }),
  bar(`${p}b1`, 0, 70, 12, 70),
  text(`${p}v1`, 36, 76, "~3,6 µs", "cream", { "font-size": 52 }),
  text(`${p}w2`, 0, 190, "write + fsync", "say", { "font-size": 46 }),
  bar(`${p}b2`, 0, 260, 842, 70),
  text(`${p}v2`, 0, 345, "~470 µs", "", { "font-size": 52 }),
), js: join(
  inJs(`${p}w1`, 0.3, { y: 20 }), growJs(`${p}b1`, 0.5, 0.3), inJs(`${p}v1`, 0.6, { x: -20 }),
  inJs(`${p}w2`, 1.0, { y: 20 }), growJs(`${p}b2`, 1.2, 1.2), inJs(`${p}v2`, 2.2, { y: 10 }),
) });

// a torn write: the third record is cut off, the log ends after record 2
const torn = (p) => ({ h: 300, html: join(
  box(`${p}r1`, 0, 30, 283, 130, nm("ID 1")),
  box(`${p}r2`, 280, 30, 283, 130, nm("ID 2")),
  box(`${p}r3`, 560, 30, 282, 130, `${nm("ID 3")}${sm("terpotong")}`, "dash"),
  vline(`${p}cut`, 558, 0, 200),
  text(`${p}cl`, 300, 225, "ujung log setelah dibuka"),
), js: join(
  inJs([`${p}r1`, `${p}r2`], 0.3, { y: 30 }, { stagger: 0.12 }),
  `      tl.fromTo($("${p}r3"), { scaleX: 0, transformOrigin: "0% 50%" }, { scaleX: 1, duration: 0.9, ease: "power1.in" }, 0.7);`,
  outJs(`${p}r3`, 2.9, 0.15, 0.4), inJs([`${p}cut`, `${p}cl`], 3.0, { y: -20 }),
) });

const total = buildTall({
  series: "bikin message broker sendiri",
  part: "01 / 06",
  hook: { dur: 3.2, still: true, kicker: "seri baru · bagian 1 / 6",
    lines: [{ text: "bikin", size: 150, light: true }, { text: "message broker", size: 112 }, { text: "sendiri", size: 150, light: true }],
    lead: "pakai Go, cuma library standar" },
  steps: [
    { kicker: "anatomi", title: "satu log, satu angka", dur: 5.0, visual: log("step1_"),
      marks: [
        { at: 0.8, cap: "pesan ditulis sekali di log milik topic-nya" },
        { at: 2.8, cap: `tiap subscriber cuma punya satu angka: ${m("posisi baca")}` }] },
    { kicker: "AMQP 0-9-1?", title: "kebesaran", dur: 4.6, visual: amqp("step2_"),
      marks: [
        { at: 0.8, cap: "protokol asli RabbitMQ: client-nya banyak, tapi besar" },
        { at: 2.6, cap: "dan model-nya queue, bukan log" }] },
    { kicker: "pilihan kita", title: "protokol sendiri", dur: 5.2,
      cards: [{ head: "4 alamat · di atas HTTP", lines: [
        { conf: "PUT  /topics/{topic}", at: 0.6 },
        { conf: "POST /topics/{topic}", at: 0.72 },
        { conf: "PUT  /subscribers/{name}", at: 0.84 },
        { conf: "POST /subscribers/{name}/stream", at: 0.96 }] }],
      marks: [
        { at: 1.4, line: 1, cap: `producer bisa kirim pesan pakai ${m("curl")}` },
        { at: 3.2, line: 3, cap: "stream pull lewat HTTP/2" }] },
    { kicker: "anatomi record", title: "header 24 byte", dur: 5.0, visual: record("step4_"),
      marks: [
        { at: 0.8, cap: "tiap pesan disimpan sebagai satu record" },
        { at: 2.7, cap: `${m("CRC-32C")}: angka pemeriksa dari isi record` }] },
    { kicker: "kapan pesan aman?", title: "write lawan fsync", dur: 5.0, visual: bars("step5_"),
      marks: [
        { at: 0.8, cap: "write belum tentu sampai ke disk" },
        { at: 2.8, cap: "jadi broker fsync sekali tiap detik" }] },
    { kicker: "listrik padam", title: "record terpotong", dur: 5.0, visual: torn("step6_"),
      marks: [
        { at: 0.8, cap: "ujung log bisa berisi record setengah jadi" },
        { at: 2.8, cap: "saat dibuka: CRC gak cocok, file dipotong" }] },
  ],
  end: { dur: 3.6, kicker: "lanjut bagian 2",
    lines: [{ text: "mengingat", size: 140, light: true }, { text: "posisi baca", size: 140 }],
    lead: "tiap subscriber, juga setelah restart" },
  endSfx: { file: "ting.wav", at: 0.3, dur: 0.45, vol: 0.4 },
});
console.log("bagian 1 (TikTok):", total, "s");
