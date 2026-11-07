// Builds the TikTok cut (9:16, ~37 s, captions only) of "Membuat Message Broker Sendiri" Bagian 3: pull instead of
// push, one HTTP/2 request carrying messages and acks, one message in flight, the ack timeout, one stream per
// subscriber, the delivery loop. Facts and logs from the article (docs/articles/membuat-message-broker-sendiri/
// seri-...-bagian-3-...md). Run: node make.mjs. Scene kit: ../message-broker-shared/tall.mjs and diagrams.mjs.
// YouTube version: ../message-broker-3-youtube.
import { buildTall } from "../message-broker-shared/tall.mjs";
import { box, text, nm, sm, arrow, svg, join, inJs, drawJs, hotJs } from "../message-broker-shared/diagrams.mjs";

// push vs pull
const pushPull = (p) => ({ h: 440, html: join(
  box(`${p}a`, 0, 0, 842, 200, `<span class="dbig" style="font-size: 96px; color: #888880">push</span>${sm("broker datang ke subscriber")}`, "mute", { "font-size": 30 }),
  box(`${p}b`, 0, 230, 842, 200, `<span class="dbig" style="font-size: 96px">pull</span>${sm("subscriber datang sendiri")}`),
), js: join(inJs(`${p}a`, 0.3, { y: 30 }), inJs(`${p}b`, 0.6, { y: 30 }), hotJs(`${p}b`, 2.6)) });

// the pull stream: hello, 200 OK, a message in the response body, its ack in the request body
const sequence = (p) => {
  const L = 150, R = 692, mid = (L + R) / 2;
  const msgs = [
    { y: 160, dir: 1, label: "POST .../email/stream", cls: "cream", at: 0.7 },
    { y: 270, dir: -1, label: "200 OK", cls: "cream", at: 1.4 },
    { y: 380, dir: -1, label: '{"id":1,...}', at: 2.5 },
    { y: 490, dir: 1, label: '{"ack":1}', cls: "cream", at: 4.0 },
  ];
  const ar = msgs.map((g, i) => ({ ...g, id: `${p}m${i}`, a: g.dir > 0 ? arrow(L, g.y, R - 4, g.y) : arrow(R, g.y, L + 4, g.y) }));
  return { h: 540, html: join(
    box(`${p}s`, 0, 0, 300, 76, nm("email")),
    box(`${p}b`, 542, 0, 300, 76, nm("broker")),
    svg(842, 540, [
      { id: `${p}ls`, d: `M${L} 80 L${L} 540`, cls: "dash" }, { id: `${p}lb`, d: `M${R} 80 L${R} 540`, cls: "dash" },
      ...ar.map((g) => ({ id: g.id, d: g.a.d, cls: g.cls ?? "" })),
    ]),
    ar.map((g) => text(`${g.id}l`, Math.round(mid - g.label.length * 30 * 0.31), g.y - 48, g.label, g.cls ?? "", { "font-size": 30 })),
  ), js: join(
    inJs([`${p}s`, `${p}b`, `${p}ls`, `${p}lb`], 0.25, { y: 20 }, { stagger: 0.06 }),
    ar.map((g) => join(drawJs(g.id, g.at, g.a.len, 0.4), inJs(`${g.id}l`, g.at + 0.15, { y: 10 }))),
  ) };
};

// one message in flight: ~140 µs per message and ack on a laptop, about 7,000 a second per subscriber
const stat = (p) => ({ h: 360, html: join(
  text(`${p}v`, 0, 0, "~140 µs", "stat", { "font-size": 200 }),
  text(`${p}a`, 0, 210, "per pesan + ack", "say mute", { "font-size": 44 }),
  text(`${p}b`, 0, 280, "~7.000 pesan/detik per subscriber", "say", { "font-size": 42 }),
), js: join(
  `      tl.fromTo($("${p}v"), { scale: 0.7, opacity: 0, transformOrigin: "0% 50%" }, { scale: 1, opacity: 1, duration: 0.5, ease: "back.out(1.6)" }, 0.3);`,
  inJs(`${p}a`, 0.7, { y: 10 }), inJs(`${p}b`, 2.8, { y: 10 }),
) });

// one stream per subscriber: the second gets 409 Conflict
const conflict = (p) => {
  const a1 = arrow(300, 55, 538, 130);
  return { h: 440, html: join(
    box(`${p}p1`, 0, 0, 300, 110, nm("program 1")),
    box(`${p}br`, 542, 90, 300, 150, `${nm("broker")}${sm("stream email")}`, "hot"),
    box(`${p}p2`, 0, 240, 300, 110, nm("program 2")),
    svg(842, 440, [{ id: `${p}a1`, d: a1.d }, { id: `${p}a2`, d: "M300 295 L538 205", cls: "dash" }]),
    text(`${p}x`, 330, 330, "409 Conflict", "", { "font-size": 38 }),
  ), js: join(
    inJs([`${p}p1`, `${p}br`], 0.3, { y: 30 }, { stagger: 0.12 }), drawJs(`${p}a1`, 0.6, a1.len, 0.4),
    inJs(`${p}p2`, 1.0, { y: 30 }), inJs([`${p}a2`, `${p}x`], 1.4, { x: -20 }, { stagger: 0.2 }),
  ) };
};

// the delivery loop: one goroutine, one step per subscriber
const loop = (p) => {
  const cols = [["email", "cek ack"], ["stok", "kirim pesan 7"], ["laporan", "kirim lagi"]];
  const ar = [arrow(421, 134, 135, 256), arrow(421, 134, 421, 256), arrow(421, 134, 707, 256)];
  return { h: 500, html: join(
    box(`${p}lp`, 171, 0, 500, 130, `${nm("loop pengantar")}${sm("satu goroutine")}`, "hot"),
    svg(842, 500, ar.map((a, i) => ({ id: `${p}a${i}`, d: a.d }))),
    cols.map(([n, t], i) => box(`${p}c${i}`, i * 286, 262, 270, 200, `<span class="ds" style="font-size: 32px; color: #e85d26; font-weight: 600">${n}</span>${nm(t)}`)),
  ), js: join(
    inJs(`${p}lp`, 0.3, { scale: 0.9 }), ar.map((a, i) => drawJs(`${p}a${i}`, 0.7 + i * 0.1, a.len, 0.35)),
    inJs(cols.map((_, i) => `${p}c${i}`), 1.0, { y: 30 }, { stagger: 0.15 }),
  ) };
};

const total = buildTall({
  series: "bikin message broker sendiri",
  part: "03 / 06",
  hook: { dur: 3.2, still: true, kicker: "bagian 3 / 6",
    lines: [{ text: "pesan + ack", size: 130 }, { text: "lewat satu", size: 130, light: true }, { text: "request HTTP/2", size: 110 }],
    lead: "subscriber yang menarik pesannya sendiri" },
  steps: [
    { kicker: "push atau pull?", title: "pull, bukan push", dur: 4.8, visual: pushPull("step1_"),
      marks: [
        { at: 0.8, cap: "push: subscriber yang lambat bisa kewalahan" },
        { at: 2.6, cap: "pull: tertinggal, lalu nyusul. jalan di balik NAT" }] },
    { kicker: "anatomi stream pull", title: "satu request HTTP/2", dur: 5.6, visual: sequence("step2_"),
      marks: [
        { at: 0.6, cap: "request-nya sendiri adalah hello" },
        { at: 2.4, cap: "pesan mengalir di body response" },
        { at: 3.9, cap: "ack di body request yang sama" }] },
    { kicker: "prefetch 1", title: "satu pesan, satu ack", dur: 4.6, visual: stat("step3_"),
      marks: [
        { at: 0.8, cap: "pesan berikutnya baru dikirim setelah ack" },
        { at: 2.7, cap: "urutan terjaga, throughput turun" }] },
    { kicker: "timeout ack", title: "dikirim lagi", dur: 5.0,
      cards: [{ head: "catatan pembaca", lines: [
        { out: "21:57:12 pesan 5: pesanan 1045 dibuat", at: 0.6 },
        { out: "21:57:15 pesan 5: pesanan 1045 dibuat", at: 1.0 },
        { out: "21:57:18 pesan 5: pesanan 1045 dibuat", at: 1.4 }] }],
      marks: [
        { at: 1.6, line: 2, cap: "kerja 3 detik, timeout ack 1 detik" },
        { at: 3.2, cap: "satu pesan dikerjakan 3 kali. subscriber harus idempotent" }] },
    { kicker: "satu stream per subscriber", title: "yang kedua: 409", dur: 5.0, visual: conflict("step5_"),
      marks: [
        { at: 0.8, cap: "stream kedua dengan nama yang sama ditolak" },
        { at: 2.8, cap: "koneksi mati diam-diam? ping HTTP/2, 15 + 15 detik" }] },
    { kicker: "loop pengantar", title: "satu goroutine", dur: 4.6, visual: loop("step6_"),
      marks: [
        { at: 0.8, cap: "keliling ke tiap subscriber, satu langkah" },
        { at: 2.6, cap: "gak pernah nunggu jaringan" }] },
  ],
  end: { dur: 3.6, kicker: "lanjut bagian 4",
    lines: [{ text: "webhook &", size: 140 }, { text: "percobaan", size: 140, light: true }, { text: "ulang", size: 140, light: true }],
    lead: "jeda 1 detik, berlipat dua, paling lama 5 menit" },
  endSfx: { file: "ting.wav", at: 0.3, dur: 0.45, vol: 0.4 },
});
console.log("bagian 3 (TikTok):", total, "s");
