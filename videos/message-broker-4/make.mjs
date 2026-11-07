// Builds the TikTok cut (9:16, ~37 s, captions only) of "Membuat Message Broker Sendiri" Bagian 4: webhook
// subscribers, which failures are retried, waits doubling from 1 s, skipping after 10 failures, the try-out, and one slow
// webhook holding up the others. Facts and logs from the article (docs/articles/membuat-message-broker-sendiri/
// seri-...-bagian-4-...md). Run: node make.mjs. Scene kit: ../message-broker-shared/tall.mjs and diagrams.mjs.
// YouTube version: ../message-broker-4-youtube.
import { buildTall } from "../message-broker-shared/tall.mjs";
import { box, text, nm, cells, arrow, svg, join, inJs, drawJs, hotJs, popJs } from "../message-broker-shared/diagrams.mjs";

// labels at 30 px or more: a phone screen shows the 1080-wide frame at about a third of its size
const mono = (s, c = "#f0ece5") => `<span style="font-family: 'IBM Plex Mono'; font-weight: 500; font-size: 30px; color: ${c}">${s}</span>`;
const say = (s, c = "#c8c4bc") => `<span class="dn" style="font-weight: 600; font-size: 34px; color: ${c}">${s}</span>`;
const dot = (id, x, y, s = 24, c = "#f0ece5") =>
  `          <div id="${id}" style="position: absolute; left: ${x - s / 2}px; top: ${y - s / 2}px; width: ${s}px; height: ${s}px; background: ${c}"></div>`;

// the broker POSTs one message to the webhook, with three Brokerku- headers and the data as body
const call = (p) => {
  const a = arrow(304, 64, 536, 64);
  return { h: 470, html: join(
    box(`${p}b`, 0, 0, 300, 128, nm("broker")),
    box(`${p}w`, 542, 0, 300, 128, nm("webhook stok"), "hot"),
    svg(842, 470, [{ id: `${p}a`, d: a.d }]),
    text(`${p}al`, 380, 0, "POST"),
    box(`${p}r`, 0, 180, 842, 280, [
      mono("Brokerku-Id: 1"), mono("Brokerku-Topic: pesanan"), mono("Brokerku-Time: waktu pesan"), mono("body: pesanan 1041 dibuat", "#a8a49c"),
    ].join(""), "l flat", { gap: 16 }),
  ), js: join(
    inJs([`${p}b`, `${p}w`], 0.3, { y: 30 }, { stagger: 0.12 }), drawJs(`${p}a`, 0.7, a.len, 0.35), inJs(`${p}al`, 0.85, { y: 10 }),
    inJs(`${p}r`, 1.2, { y: 30 }),
  ) };
};

// the status decides what happens to the message
const statuses = (p) => {
  const rows = [["2xx", "pesan sampai"], ["5xx · 408 · 429 · timeout", "dicoba lagi nanti"], ["400 · 404 · lainnya", "langsung dilewati"]];
  return { h: 560, html: join(
    rows.map(([a, b], i) => box(`${p}r${i}`, 0, i * 190, 842, 166, `${nm(a)}${say(b)}`, "l", { gap: 10 })),
  ), js: join(
    inJs(`${p}r0`, 0.4, { x: 50 }), inJs(`${p}r1`, 2.2, { x: 50 }), hotJs(`${p}r1`, 2.5), inJs(`${p}r2`, 3.7, { x: 50 }),
  ) };
};

// waits double: 1, 2, 4 ... 32 s, then 1 m 4 d, 2 m 8 d, 4 m 16 d; about 8.5 min before the message is skipped
const waits = (p) => {
  const a = cells(`${p}a`, 0, 0, ["1 d", "2 d", "4 d", "8 d", "16 d", "32 d"].map((t) => ({ t })), { w: 143, h: 100 });
  const b = cells(`${p}b`, 0, 124, ["1 m 4 d", "2 m 8 d", "4 m 16 d"].map((t) => ({ t, cls: "hot" })), { w: 282, h: 100 });
  return { h: 500, html: join(
    a.html, b.html,
    text(`${p}v`, 0, 270, "~8,5 menit", "stat", { "font-size": 150 }),
    text(`${p}s`, 0, 448, "total jeda sebelum pesan dilewati", "say mute", { "font-size": 34 }),
  ), js: join(
    inJs(a.ids, 0.3, { y: 30 }, { stagger: 0.12 }), inJs(b.ids, 1.2, { y: 30 }, { stagger: 0.15 }),
    `      tl.fromTo($("${p}v"), { scale: 0.7, opacity: 0, transformOrigin: "0% 50%" }, { scale: 1, opacity: 1, duration: 0.5, ease: "back.out(1.6)" }, 2.8);`,
    inJs(`${p}s`, 3.1, { y: 10 }),
  ) };
};

// when to give up: NATS JetStream never, RabbitMQ 4.0 after 20 deliveries, this broker after 10 failures
const giveUp = (p) => {
  const rows = [["NATS JetStream", "selamanya", "mute"], ["RabbitMQ 4.0", "20 kali", "mute"], ["brokerku", "10 kali", "fill"]];
  const row = (a, b) => `<div style="display: flex; width: 100%; justify-content: space-between; align-items: center">${nm(a)}<span class="dbig" style="font-size: 64px">${b}</span></div>`;
  return { h: 460, html: join(rows.map(([a, b, cls], i) => box(`${p}r${i}`, 0, i * 155, 842, 135, row(a, b), `l ${cls}`))), js: join(
    inJs([`${p}r0`, `${p}r1`], 0.3, { x: 50 }, { stagger: 0.2 }),
    `      tl.fromTo($("${p}r2"), { scale: 0.85, opacity: 0 }, { scale: 1, opacity: 1, duration: 0.5, ease: "back.out(1.6)" }, 2.8);`,
  ) };
};

// one webhook sender: stok takes 4 s per call, faktur waits 8 s, the pull stream email gets both at once
const slow = (p) => {
  const x0 = 180, x1 = 842, half = (x1 - x0) / 2;
  const fill = (id, x, y, w, h) => `          <div id="${id}" class="bar" style="left: ${x}px; top: ${y}px; width: ${w}px; height: ${h}px; opacity: 0.22"></div>`;
  const grow = (id, at, dur) => `      tl.fromTo($("${id}"), { scaleX: 0 }, { scaleX: 1, duration: ${dur}, ease: "none" }, ${at});`;
  const clear = { background: "transparent" };
  return { h: 470, html: join(
    [["stok", 0], ["faktur", 140], ["email", 280]].map(([n, y], i) => text(`${p}n${i}`, 0, y + 22, nm(n), "cream")),
    fill(`${p}f0`, x0, 0, half, 100), fill(`${p}f1`, x0 + half, 0, half, 100), fill(`${p}f2`, x0, 140, x1 - x0, 100),
    box(`${p}s0`, x0, 0, half, 100, nm("pesan 2"), "", clear), box(`${p}s1`, x0 + half - 3, 0, half + 3, 100, nm("pesan 3"), "", clear),
    box(`${p}w`, x0, 140, x1 - x0, 100, nm("nunggu 8 detik"), "hot", { ...clear, color: "#e85d26" }),
    dot(`${p}e`, x0 + 14, 330), text(`${p}el`, x0 + 46, 308, "langsung dapat 2 dan 3", "cream"),
    svg(842, 470, [{ id: `${p}ax`, d: `M${x0} 420 L${x1} 420 M${x0} 406 L${x0} 434 M${x0 + half} 406 L${x0 + half} 434 M${x1 - 2} 406 L${x1 - 2} 434`, cls: "cream thin" }]),
    text(`${p}t0`, x0 - 8, 440, "0", "mute"), text(`${p}t1`, x0 + half - 8, 440, "4", "mute"), text(`${p}t2`, x1 - 200, 440, "8 detik", "mute", { width: 200, "text-align": "right" }),
  ), js: join(
    inJs([`${p}n0`, `${p}n1`, `${p}n2`, `${p}ax`, `${p}t0`, `${p}t1`, `${p}t2`], 0.25, { y: 20 }, { stagger: 0.05 }),
    inJs([`${p}s0`, `${p}w`], 0.5, { y: 20 }, { stagger: 0.1 }), inJs([`${p}e`, `${p}el`], 0.7, { x: -20 }, { stagger: 0.1 }),
    grow(`${p}f0`, 0.6, 1.5), inJs(`${p}s1`, 2.1, { y: 20 }), grow(`${p}f1`, 2.1, 1.5), grow(`${p}f2`, 0.6, 3.0),
  ) };
};

const total = buildTall({
  series: "bikin message broker sendiri",
  part: "04 / 06",
  hook: { dur: 3.2, still: true, kicker: "bagian 4 / 6",
    lines: [{ text: "webhook", size: 150 }, { text: "gagal?", size: 150, light: true }, { text: "coba lagi", size: 150 }],
    lead: "jedanya berlipat dua, dari 1 detik sampai 5 menit" },
  steps: [
    { kicker: "subscriber webhook", title: "broker yang datang", dur: 4.8, visual: call("step1_"),
      marks: [
        { at: 0.8, cap: "broker kirim POST ke server HTTP milik subscriber" },
        { at: 2.7, cap: "satu pesan, tunggu jawaban, paling lama 5 detik" }] },
    { kicker: "status jawaban", title: "nasib pesannya", dur: 5.6, visual: statuses("step2_"),
      marks: [
        { at: 0.6, cap: "2xx: pesan sampai, posisi baca maju" },
        { at: 2.2, cap: "yang mungkin sembuh sendiri: dicoba lagi" },
        { at: 3.7, cap: "penerima menolak: langsung dilewati" }] },
    { kicker: "jeda berlipat dua", title: "1, 2, 4, 8 detik", dur: 5.0, visual: waits("step3_"),
      marks: [
        { at: 0.8, cap: "gagal? jangan langsung dicoba lagi" },
        { at: 2.8, cap: "jeda berlipat dua, paling lama 5 menit" }] },
    { kicker: "kapan menyerah?", title: "10 kali gagal", dur: 4.8, visual: giveUp("step4_"),
      marks: [
        { at: 0.8, cap: "tanpa batas, satu pesan rusak nahan semua pesan sesudahnya" },
        { at: 2.8, cap: "brokerku: gagal 10 kali, pesannya dilewati" }] },
    { kicker: "dicoba pakai penerima", title: "503, 503, 503, 200", dur: 5.0,
      cards: [{ head: "catatan penerima stok", lines: [
        { out: "23:17:42 panggilan 1: dijawab 503", at: 0.5 },
        { out: "23:17:43 panggilan 2: dijawab 503", at: 0.8 },
        { out: "23:17:45 panggilan 3: dijawab 503", at: 1.1 },
        { out: "23:17:49 panggilan 4: dijawab 200", at: 1.4 }] }],
      marks: [
        { at: 1.4, line: 2, cap: "jedanya 1, 2, lalu 4 detik" },
        { at: 3.0, line: 3, cap: "pesan bisa datang dua kali: webhook harus idempotent" }] },
    { kicker: "masalah berikutnya", title: "webhook lambat", dur: 5.0, visual: slow("step6_"),
      marks: [
        { at: 0.8, cap: "satu pengirim webhook: stok lambat, faktur ikut nunggu" },
        { at: 3.0, cap: "stream pull gak ikut ketahan" }] },
  ],
  end: { dur: 3.6, kicker: "lanjut bagian 5",
    lines: [{ text: "worker pool", size: 140 }, { text: "biar gak", size: 140, light: true }, { text: "ketahan", size: 140, light: true }],
    lead: "webhook yang lambat cuma nahan dirinya sendiri" },
  endSfx: { file: "ting.wav", at: 0.3, dur: 0.45, vol: 0.4 },
});
console.log("bagian 4 (TikTok):", total, "s");
