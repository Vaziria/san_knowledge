// Builds the YouTube (16:9, ~3 min, captions only) video for "Membuat Message Broker Sendiri" Bagian 4: webhook
// subscribers, which failures are retried, waits from 1 s doubling to at most 5 min, skipping after 10 failures, retry
// state in the subscriber slot, the same limit for pull streams, and one slow webhook holding up the others. The story,
// numbers, logs and diagrams follow the article (docs/articles/membuat-message-broker-sendiri/seri-...-bagian-4-...md).
// Run: node make.mjs. Scene kit: ../message-broker-shared/wide.mjs and diagrams.mjs.
import { buildWide } from "../message-broker-shared/wide.mjs";
import { DIAGRAM_CSS, box, text, nm, sm, cells, arrow, svg, xmark, join, inJs, drawJs, hotJs, popJs, growJs } from "../message-broker-shared/diagrams.mjs";

const m = (s) => `<span class="m">${s}</span>`;
const body = (s, c = "#c8c4bc") => `<span class="dn" style="font-weight: 600; font-size: 30px; color: ${c}">${s}</span>`;
const head = (s, c = "#f0ece5") => `<span class="dn" style="font-size: 38px; color: ${c}">${s}</span>`;
// a flat square marker on a timeline, centred on (x, y)
const dot = (id, x, y, s = 18, c = "#e85d26") =>
  `          <div id="${id}" style="position: absolute; left: ${x - s / 2}px; top: ${y - s / 2}px; width: ${s}px; height: ${s}px; background: ${c}"></div>`;
const tint = { background: "rgba(232, 93, 38, 0.16)", color: "#e85d26" };

// one webhook call: the loop hands the message to the webhook sender, which POSTs it with three Brokerku- headers
const anatomy = (p) => {
  const go = arrow(404, 36, 556, 36), back = arrow(556, 86, 404, 86);
  const post = arrow(620, 126, 620, 236), ans = arrow(900, 236, 900, 126);
  return { css: DIAGRAM_CSS, html: join(
    box(`${p}lp`, 0, 0, 400, 120, `${nm("loop pengantar")}${sm("satu goroutine")}`),
    box(`${p}ps`, 560, 0, 400, 120, `${nm("pengirim webhook")}${sm("satu goroutine")}`),
    svg(960, 820, [
      { id: `${p}a1`, d: go.d }, { id: `${p}a2`, d: back.d, cls: "cream thin" },
      { id: `${p}a3`, d: post.d }, { id: `${p}a4`, d: ans.d, cls: "cream thin" },
    ]),
    text(`${p}a1l`, 442, -2, "pesan"),
    text(`${p}a2l`, 444, 96, "hasil", "cream"),
    text(`${p}a3l`, 520, 164, "POST"),
    text(`${p}a4l`, 668, 164, "jawaban", "cream"),
    `          <div id="${p}c" class="card" style="position: absolute; left: 0; top: 250px">
            <div class="head">webhook stok · POST /</div>
            <div class="code">
              <div id="${p}l0" class="ln">Brokerku-Id: 1</div>
              <div id="${p}l1" class="ln">Brokerku-Topic: pesanan</div>
              <div id="${p}l2" class="ln">Brokerku-Time: waktu pesan</div>
              <div id="${p}l3" class="ln out">body: pesanan 1041 dibuat</div>
            </div>
          </div>`,
    box(`${p}r`, 0, 640, 960, 120, body("jawabannya cukup status HTTP, paling lama 5 detik. body jawaban diabaikan."), "l flat"),
  ), js: join(
    inJs([`${p}lp`, `${p}ps`], 0.4, { y: 30 }, { stagger: 0.15 }),
    drawJs(`${p}a1`, 1.3, go.len, 0.4), inJs(`${p}a1l`, 1.5, { y: 10 }),
    hotJs(`${p}ps`, 5.3),
    drawJs(`${p}a3`, 9.6, post.len, 0.4), inJs(`${p}a3l`, 9.8, { y: 10 }), inJs(`${p}c`, 9.9, { y: 30 }),
    inJs([`${p}l0`, `${p}l1`, `${p}l2`, `${p}l3`], 10.4, { x: -18 }, { stagger: 0.25, dur: 0.3 }),
    drawJs(`${p}a4`, 14.6, ans.len, 0.4), inJs(`${p}a4l`, 14.8, { y: 10 }),
    drawJs(`${p}a2`, 15.3, back.len, 0.4), inJs(`${p}a2l`, 15.5, { y: 10 }),
    inJs(`${p}r`, 15.8, { y: 30 }),
  ) };
};

// the answer decides: 2xx delivered, failures that may heal are retried, anything else is skipped
const statuses = (p) => {
  const rows = [
    ["2xx", "pesan sampai: posisi baca maju, pesan berikutnya dikirim", ""],
    ["timeout · koneksi putus · 5xx · 408 · 429", "mungkin sembuh sendiri: dicoba lagi setelah jeda yang berlipat dua", ""],
    ["status lain: 400 · 404 · 302", "penerima menolak pesan: langsung dilewati, alasannya dicatat", ""],
  ];
  const ys = [30, 290, 550];
  return { css: DIAGRAM_CSS, html: join(
    rows.map(([a, b], i) => box(`${p}r${i}`, 0, ys[i], 960, 210, `${head(a, i === 1 ? "#e85d26" : "#f0ece5")}${body(b)}`, "l", { gap: 14 })),
  ), js: join(
    inJs(`${p}r0`, 1.2, { x: 50 }), inJs(`${p}r1`, 5.2, { x: 50 }), hotJs(`${p}r1`, 5.8), inJs(`${p}r2`, 10.2, { x: 50 }),
  ) };
};

// the retry schedule drawn to time: attempts 1-7 within 63 s, then waits of 1m4s, 2m8s, 4m16s; skipped at 8m31s
const schedule = (p) => {
  const X = (t) => Math.round(8 + t * (944 / 511)), Y = 200;
  const at = [0, 1, 3, 7, 15, 31, 63, 127, 255, 511];
  const waits = [[63, 127, "1 m 4 d"], [127, 255, "2 m 8 d"], [255, 511, "4 m 16 d"]];
  const lab = (id, x, y, s, cls, extra = {}) => text(id, x, y, s, cls, { "font-size": 24, ...extra });
  return { css: DIAGRAM_CSS, html: join(
    text(`${p}h`, 0, 0, "10 percobaan, digambar sesuai waktu", "up mute"),
    waits.map(([a, b, s], i) => box(`${p}w${i}`, X(a) + 3, 112, X(b) - X(a) - 6, 56, s, "hot", { ...tint, "font-size": 22 })),
    svg(960, 820, [{ id: `${p}ax`, d: `M0 ${Y} L960 ${Y}`, cls: "cream thin" }]),
    at.slice(0, 9).map((t, i) => dot(`${p}d${i}`, X(t), Y, i < 7 ? 14 : 20)),
    dot(`${p}d9`, X(511), Y, 24, "#f0ece5"),
    lab(`${p}t0`, 0, 226, "percobaan 1-7", "cream"), lab(`${p}u0`, 0, 262, "0:00-1:03", "mute"),
    lab(`${p}t1`, X(127) - 8, 226, "8", "cream"), lab(`${p}u1`, X(127) - 8, 262, "2:07", "mute"),
    lab(`${p}t2`, X(255) - 8, 226, "9", "cream"), lab(`${p}u2`, X(255) - 8, 262, "4:15", "mute"),
    lab(`${p}t3`, 600, 226, "percobaan 10 gagal", "cream", { width: 360, "text-align": "right" }),
    lab(`${p}u3`, 600, 262, "8:31 dilewati", "mute", { width: 360, "text-align": "right" }),
    box(`${p}j`, 0, 340, 960, 150, `${nm("jeda setelah gagal ke-1 sampai ke-9")}<span class="ds" style="font-size: 26px; color: #f0ece5">1, 2, 4, 8, 16, 32 d · 1 m 4 d · 2 m 8 d · 4 m 16 d</span>`, "l flat", { gap: 12 }),
    text(`${p}v`, 0, 540, "~8,5 menit", "stat", { "font-size": 130 }),
    text(`${p}vs`, 0, 690, "sebelum pesan menyerah. batas 5 menit belum pernah kepakai", "say mute", { width: 960 }),
  ), js: join(
    inJs(`${p}h`, 0.6, { y: 10 }), inJs(`${p}ax`, 1.0, { x: -40 }),
    inJs(at.slice(0, 7).map((_, i) => `${p}d${i}`), 1.4, { scale: 0 }, { stagger: 0.12, dur: 0.3 }),
    inJs([`${p}t0`, `${p}u0`], 2.4, { y: 10 }),
    inJs(`${p}w0`, 5.6, { x: -30 }), inJs([`${p}d7`, `${p}t1`, `${p}u1`], 6.0, { y: 10 }, { stagger: 0.1 }),
    inJs(`${p}w1`, 6.6, { x: -30 }), inJs([`${p}d8`, `${p}t2`, `${p}u2`], 7.0, { y: 10 }, { stagger: 0.1 }),
    inJs(`${p}w2`, 7.6, { x: -30 }), inJs([`${p}d9`, `${p}t3`, `${p}u3`], 8.0, { y: 10 }, { stagger: 0.1 }),
    inJs(`${p}j`, 11.2, { y: 30 }),
    `      tl.fromTo($("${p}v"), { scale: 0.7, opacity: 0, transformOrigin: "0% 50%" }, { scale: 1, opacity: 1, duration: 0.5, ease: "back.out(1.6)" }, 15.6);`,
    inJs(`${p}vs`, 16.0, { y: 10 }),
  ) };
};

// retry state lives in the subscriber slot (bytes 140-405); pull streams count an ack timeout as one failure
const slotPull = (p) => {
  const f = [["0-139", "posisi baca, jenis, nama", "mute"], ["140-147", "waktu percobaan berikutnya", ""], ["148-149", "jumlah percobaan yang gagal", ""],
    ["150-405", "alamat webhook", ""], ["406-511", "cadangan", "dash"]];
  const row = (a, b) => `<div style="display: flex; width: 100%; gap: 24px; align-items: baseline"><span class="ds" style="width: 150px">byte ${a}</span>${nm(b)}</div>`;
  const c = cells(`${p}c`, 0, 556, [{ t: "1" }, { t: "1" }, { t: "1" }, { t: "2", cls: "hot" }], { w: 110, h: 90 });
  return { css: DIAGRAM_CSS, html: join(
    text(`${p}h`, 0, 0, "slot subscriber · 512 byte", "up mute"),
    f.map(([a, b, cls], i) => box(`${p}f${i}`, 0, 44 + i * 71, 960, 74, row(a, b), `l ${cls}`)),
    text(`${p}ph`, 0, 506, "stream pull: pesan 1 gak pernah di-ack", "up mute"),
    c.html,
    text(`${p}ps`, c.end + 36, 562, "tiga timeout ack, tiga percobaan gagal: pesan 1 dilewati, pesan 2 sampai", "say", { width: 960 - c.end - 36 }),
    box(`${p}ok`, 0, 690, 960, 100, body("lambat tapi sehat? minta timeout ack lebih panjang, sampai 24 jam"), "l flat"),
  ), js: join(
    inJs(`${p}h`, 0.4, { y: 10 }), inJs(f.map((_, i) => `${p}f${i}`), 0.6, { x: 40 }, { stagger: 0.08 }),
    hotJs(`${p}f1`, 1.6), hotJs(`${p}f2`, 1.8),
    inJs([`${p}ph`, ...c.ids], 10.2, { y: 20 }, { stagger: 0.25 }), inJs(`${p}ps`, 11.4, { x: -20 }),
    inJs(`${p}ok`, 14.2, { y: 30 }),
  ) };
};

// one webhook sender: stok takes 4 s per call, so faktur and arsip get messages 2 and 3 after 8 s; pull email at once
const holds = (p) => {
  const x0 = 240, x1 = 850, px = (x1 - x0) / 8;
  const rows = [["stok", "webhook, lambat", 40], ["faktur", "webhook", 180], ["arsip", "webhook", 320], ["email", "pull", 460]];
  // time passes for real: a fill grows inside each box (the box and its text stay still)
  const fill = (id, x, y, w, h) => `          <div id="${id}" class="bar" style="left: ${x}px; top: ${y}px; width: ${w}px; height: ${h}px; opacity: 0.22"></div>`;
  const grow = (id, at, dur) => `      tl.fromTo($("${id}"), { scaleX: 0 }, { scaleX: 1, duration: ${dur}, ease: "none" }, ${at});`;
  const clear = { background: "transparent" };
  return { css: DIAGRAM_CSS, html: join(
    rows.map(([n, s, y], i) => join(text(`${p}n${i}`, 0, y + 2, nm(n), "cream"), text(`${p}k${i}`, 0, y + 46, s, "mute", { "font-size": 21 }))),
    fill(`${p}f2`, x0, 40, 4 * px, 80), fill(`${p}f3`, x0 + 4 * px, 40, 4 * px, 80),
    [1, 2].map((i) => fill(`${p}g${i}`, x0, rows[i][2], x1 - x0, 80)),
    box(`${p}s2`, x0, 40, 4 * px, 80, nm("pesan 2"), "", clear),
    box(`${p}s3`, x0 + 4 * px - 3, 40, 4 * px + 3, 80, nm("pesan 3"), "", clear),
    [1, 2].map((i) => box(`${p}b${i}`, x0, rows[i][2], x1 - x0, 80, "nunggu pengirim webhook, 8 detik", "hot", { ...clear, color: "#e85d26", "font-size": 24 })),
    [1, 2].map((i) => join(dot(`${p}e${i}`, x1 + 22, rows[i][2] + 40, 20, "#f0ece5"), text(`${p}el${i}`, x1 + 40, rows[i][2] + 26, "2, 3", "cream"))),
    dot(`${p}m`, x0 + 10, 500, 20, "#f0ece5"),
    text(`${p}ml`, x0 + 40, 476, "pesan 2 dan 3 langsung sampai: stream pull gak lewat pengirim webhook", "say", { width: 960 - x0 - 40 }),
    svg(960, 820, [{ id: `${p}ax`, d: `M${x0} 640 L${x1} 640 M${x0} 626 L${x0} 654 M${x0 + 4 * px} 626 L${x0 + 4 * px} 654 M${x1} 626 L${x1} 654`, cls: "cream thin" }]),
    text(`${p}x0`, x0 - 60, 670, "23:17:54", "mute"), text(`${p}x1`, x0 + 4 * px - 60, 670, "23:17:58", "mute"), text(`${p}x2`, x1 - 60, 670, "23:18:02", "mute"),
  ), js: join(
    inJs([`${p}n0`, `${p}k0`, `${p}n1`, `${p}k1`, `${p}n2`, `${p}k2`, `${p}n3`, `${p}k3`, `${p}ax`, `${p}x0`, `${p}x1`, `${p}x2`], 0.4, { y: 20 }, { stagger: 0.08 }),
    inJs([`${p}m`, `${p}ml`], 1.4, { x: -20 }, { stagger: 0.2 }),
    inJs([`${p}s2`, `${p}b1`, `${p}b2`], 1.2, { y: 20 }, { stagger: 0.1 }), inJs(`${p}s3`, 5.4, { y: 20 }),
    grow(`${p}f2`, 1.4, 4), grow(`${p}f3`, 5.4, 4), grow(`${p}g1`, 1.4, 8), grow(`${p}g2`, 1.4, 8),
    popJs(`${p}e1`, 9.4), popJs(`${p}e2`, 9.5), inJs([`${p}el1`, `${p}el2`], 9.6, { x: -10 }),
  ) };
};

const { total } = buildWide({
  series: "bikin message broker sendiri · bagian 4",
  part: "04 / 06",
  scenes: [
    { type: "title", reg: "orange", dur: 7, still: true, chapter: "Pembuka: pull lawan webhook",
      kicker: "bagian 4 dari 6",
      lines: [{ text: "webhook gagal?", size: 170 }, { text: "coba lagi, tapi", size: 170, light: true }, { text: "jedanya berlipat", size: 170 }],
      lead: "bagian 4: webhook dan percobaan ulang dengan jeda yang makin panjang." },
    { type: "compare", dur: 17, kicker: "subscriber jenis kedua", title: "pull lawan webhook",
      panels: [
        { num: "pull · bagian 3", name: "subscriber datang sendiri", at: 2.0,
          lines: ["harus buka stream HTTP/2 dan nyambung terus", "pesan tanpa ack dikirim ulang selamanya"] },
        { num: "webhook · bagian ini", name: "broker datang ke subscriber", at: 8.0, pay: true,
          lines: ["cukup server HTTP biasa: broker kirim POST tiap pesan", "satu pesan, tunggu jawaban: webhook gak kewalahan"] }] },
    { type: "split", dur: 20, kicker: "anatomi satu panggilan", title: "satu POST per pesan", visual: anatomy("s03_"), chapter: "Anatomi satu panggilan webhook",
      notes: [
        { at: 1.0, html: "loop pengantar nyerahin pesan, gak nunggu hasilnya" },
        { at: 5.0, html: "pengirim webhook: goroutine kedua, satu panggilan tiap kali" },
        { at: 9.5, html: `body: data pesan apa adanya. ID, topic, waktu di header ${m("Brokerku-")}` },
        { at: 14.5, html: "timeout 5 detik (GitHub: 10 detik). redirect gak diikuti" }] },
    { type: "split", dur: 19, kicker: "gak semua gagal layak dicoba lagi", title: "status jawabannya yang nentuin", visual: statuses("s04_"), chapter: "Kegagalan mana yang dicoba lagi",
      notes: [
        { at: 1.0, html: "2xx: pesan sampai, posisi baca maju" },
        { at: 5.0, html: "timeout, koneksi putus, 5xx, 408, 429: mungkin sembuh sendiri" },
        { at: 10.0, html: "400 atau 404 bakal ditolak lagi persis sama: langsung dilewati" },
        { at: 14.5, html: "Stripe nyoba lagi semua selain 2xx. brokerku ikut arti status di RFC 9110" }] },
    { type: "statement", reg: "orange", dur: 7,
      lines: [{ text: "pesan bisa", size: 190, light: true }, { text: "sampai dua kali.", size: 190 }],
      lead: "timeout gak bilang apakah webhook sudah ngerjain pesannya. jadi webhook harus <b>idempotent</b>: catat Brokerku-Id yang sudah diproses.", leadAt: 1.2 },
    { type: "split", dur: 21, kicker: "gagal? jangan langsung dicoba lagi", title: "jeda yang berlipat dua", visual: schedule("s06_"), chapter: "Jeda yang berlipat dua",
      notes: [
        { at: 1.0, html: "kata AWS: percobaan ulang itu egois, bisa memperburuk server" },
        { at: 5.5, html: "jeda 1 detik, berlipat dua tiap gagal, paling lama 5 menit" },
        { at: 11.0, html: "angka awalnya sama dengan library C++ Google Cloud Storage" },
        { at: 15.5, html: `10 percobaan: ~8,5 menit. tanpa ${m("jitter")}, diterima sebagai kelemahan` }] },
    { type: "compare", dur: 17, kicker: "kapan broker menyerah?", title: "kenapa dilewati setelah 10 kali?", chapter: "Kenapa menyerah setelah 10 kali",
      panels: [
        { num: "NATS JetStream", name: "selamanya", at: 2.0, lines: ["MaxDeliver bawaan -1: tanpa batas", "satu pesan rusak nahan semua sesudahnya"] },
        { num: "RabbitMQ 4.0", name: "20 kali", at: 6.5, lines: ["batas bawaan quorum queue", "lalu dibuang, atau ke <span style=\"white-space: nowrap\">dead-letter</span> exchange"] },
        { num: "brokerku", name: "10 kali", at: 11.0, pay: true, lines: ["~8,5 menit: cukup buat lewati gangguan singkat", "yang dilewati dicatat, tetap ada di log"] }] },
    { type: "split", dur: 18, kicker: "di mana percobaan ulang diingat?", title: "di slot, juga untuk pull", visual: slotPull("s08_"), chapter: "Tahan restart, berlaku juga untuk pull",
      notes: [
        { at: 1.0, html: "jumlah gagal dan waktu percobaan berikutnya: 10 byte di slot" },
        { at: 5.5, html: "broker restart: hitungannya gak balik ke nol, jedanya tetap ditunggu" },
        { at: 10.0, html: "stream pull juga: timeout ack dihitung satu percobaan gagal" },
        { at: 14.0, html: "stream yang putus gak dihitung" }] },
    { type: "split", dur: 19, kicker: "dicoba di Windows", title: "stok, faktur, dan arsip", chapter: "Mencoba dengan penerima",
      cards: [
        { head: "catatan penerima stok · -gagal 3", lines: [
          { out: "23:17:42 panggilan 1: dijawab 503", at: 1.0 },
          { out: "23:17:43 panggilan 2: dijawab 503", at: 1.4 },
          { out: "23:17:45 panggilan 3: dijawab 503", at: 1.8 },
          { out: "23:17:49 panggilan 4: dijawab 200", at: 5.8 }] },
        { head: "faktur dan arsip", lines: [
          { out: "23:17:42 faktur: dijawab 200", at: 9.8 },
          { out: "23:17:42 arsip: dijawab 400", at: 13.6 },
          { out: "arsip melewati pesan 1: ditolak", at: 14.0 }] }],
      notes: [
        { at: 2.4, card: 0, line: 2, html: "stok jawab 503 tiga kali: jedanya 1, 2, 4 detik" },
        { at: 6.2, card: 0, line: 3, html: "panggilan keempat dijawab 200: pesan sampai" },
        { at: 10.2, card: 1, line: 0, html: "faktur dapat pesan itu di detik yang sama" },
        { at: 14.4, card: 1, line: 2, html: "arsip selalu jawab 400: langsung dilewati" }] },
    { type: "split", dur: 17, kicker: "masalah yang tersisa", title: "satu webhook lambat, yang lain nunggu", visual: holds("s10_"), chapter: "Webhook lambat menahan yang lain",
      notes: [
        { at: 1.0, html: "pengirim webhook cuma satu goroutine" },
        { at: 5.0, html: "stok butuh 4 detik per pesan" },
        { at: 9.6, html: "faktur dan arsip baru dapat pesan 2 dan 3 setelah 8 detik" },
        { at: 13.4, html: "stream pull email gak ikut ketahan" }] },
    { type: "bullets", dur: 13, kicker: "kelemahan versi keempat", title: "yang belum beres", chapter: "Kelemahan dan bagian berikutnya",
      items: [
        { at: 1.5, text: "satu pengirim webhook buat semua subscriber" },
        { at: 5.0, text: "jeda tanpa jitter, Retry-After diabaikan" },
        { at: 8.5, text: "webhook belum bisa mastiin siapa pengirimnya" }] },
    { type: "title", reg: "orange", dur: 10,
      kicker: "lanjut bagian 5",
      lines: [{ text: "worker pool", size: 180 }, { text: "biar webhook lambat", size: 130, light: true }, { text: "gak nahan yang lain", size: 130, light: true }],
      lead: "artikel lengkapnya, dengan kode Go, test, dan percobaannya: link di deskripsi.", leadAt: 1.4 },
  ],
});
console.log("bagian 4 (YouTube):", total, "s");
