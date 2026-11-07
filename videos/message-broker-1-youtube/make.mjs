// Builds the YouTube (16:9, ~3 min, captions only) video for "Membuat Message Broker Sendiri" Bagian 1: a broker's
// own protocol or AMQP, then storing messages in a log. The story, numbers and diagrams follow the article
// (docs/articles/membuat-message-broker-sendiri/seri-...-bagian-1-...md). Run: node make.mjs. Scene kit:
// ../message-broker-shared/wide.mjs and diagrams.mjs.
import { buildWide } from "../message-broker-shared/wide.mjs";
import {
  DIAGRAM_CSS, box, text, nm, sm, cells, pointer, arrow, svg, vline, bar, join,
  inJs, outJs, drawJs, hotJs, moveJs, growJs, packetJs,
} from "../message-broker-shared/diagrams.mjs";

const m = (s) => `<span class="m">${s}</span>`;

// producer -> broker -> consumer, and the ack coming back
const flow = (p) => {
  const a1 = arrow(250, 375, 352, 375), a2 = arrow(608, 345, 707, 345), ak = arrow(707, 415, 612, 415);
  return { css: DIAGRAM_CSS, html: join(
    box(`${p}pr`, 0, 300, 250, 150, `${nm("producer")}${sm("pengirim")}`),
    box(`${p}br`, 355, 270, 250, 210, `${nm("broker")}${sm("terima, simpan,")}${sm("antar")}`, "hot"),
    box(`${p}co`, 710, 300, 250, 150, `${nm("consumer")}${sm("penerima")}`),
    svg(960, 820, [{ id: `${p}a1`, d: a1.d }, { id: `${p}a2`, d: a2.d }, { id: `${p}ak`, d: ak.d, cls: "cream thin" }]),
    text(`${p}l1`, 262, 322, "pesan"), text(`${p}l2`, 618, 292, "pesan"), text(`${p}lk`, 632, 432, "ack", "cream"),
    `          <div id="${p}pk" class="pkt" style="opacity: 0"></div>`,
    text(`${p}say`, 0, 560, "pesan disimpan di broker sampai consumer mengirim ack", "say mute", { width: 960 }),
  ), js: join(
    inJs([`${p}pr`, `${p}br`, `${p}co`], 0.4, { y: 40 }, { stagger: 0.12 }),
    drawJs(`${p}a1`, 4.6, a1.len, 0.4), inJs(`${p}l1`, 4.8, { y: 10 }),
    packetJs(`${p}pk`, 5.0, [[250, 375], [352, 375]], 0.6),
    drawJs(`${p}a2`, 5.9, a2.len, 0.4), inJs(`${p}l2`, 6.1, { y: 10 }),
    packetJs(`${p}pk`, 6.3, [[608, 345], [707, 345]], 0.6),
    drawJs(`${p}ak`, 8.6, ak.len, 0.4), inJs(`${p}lk`, 8.8, { y: 10 }),
    inJs(`${p}say`, 10.0, { y: 20 }),
  ) };
};

// the series map: six parts, this one highlighted
const parts = (p) => {
  const rows = ["protokol dan log pesan", "posisi baca", "subscriber pull", "webhook dan percobaan ulang", "worker pool", "menjalankan broker"];
  return { css: DIAGRAM_CSS + `
    .prow { display: flex; width: 100%; align-items: baseline; gap: 30px; }
    .prow .ds { font-size: 26px; }`, html: join(rows.map((r, i) =>
    box(`${p}r${i}`, 0, i * 134 + 12, 960, 112,
      `<div class="prow">${sm(`0${i + 1}`)}${nm(r)}${i === 0 ? `<span class="ds" style="margin-left: auto">VIDEO INI</span>` : ""}</div>`,
      i === 0 ? "l fill" : "l flat"))),
  js: inJs(rows.map((_, i) => `${p}r${i}`), 0.4, { x: 60 }, { stagger: 0.12 }) };
};

// RabbitMQ copies a message into every queue; the broker we build writes it once and keeps one number per subscriber
const logVsQueue = (p) => {
  const qy = [60, 170, 280];
  const qa = qy.map((y) => arrow(232, 200, 356, y + 42));
  const c = cells(`${p}c`, 0, 500, [1, 2, 3, 4, 5, 6].map((n) => ({ t: n })), { w: 120, h: 96 });
  const rabbit = [`${p}rl`, `${p}ex`, ...qy.map((_, i) => `${p}q${i}`), ...qy.map((_, i) => `${p}qa${i}`)];
  const copies = [];
  qy.forEach((y, i) => [0, 1, 2].forEach((k) => copies.push({ id: `${p}cp${i}${k}`, x: 724 + k * 56, y: y + 22 })));
  return { css: DIAGRAM_CSS, html: join(
    text(`${p}rl`, 0, 0, "RabbitMQ", "up mute"),
    box(`${p}ex`, 0, 150, 232, 100, nm("exchange")),
    qy.map((y, i) => box(`${p}q${i}`, 356, y, 340, 84, nm(["queue email", "queue stok", "queue invoice"][i]), "l")),
    svg(960, 820, qa.map((a, i) => ({ id: `${p}qa${i}`, d: a.d, cls: "cream thin" }))),
    copies.map((k) => bar(k.id, k.x, k.y, 40, 40)),
    text(`${p}cl`, 724, 372, "3 salinan", "mute"),
    text(`${p}bl`, 0, 440, "brokerku · log topic pesanan", "up mute"),
    c.html,
    vline(`${p}end`, c.end + 12, 478, 140),
    text(`${p}endl`, c.end + 28, 530, "ujung"),
    pointer(`${p}pe`, c.xs[3] + 60, 596, "email", { len: 62, w: 170 }),
    pointer(`${p}ps`, c.end + 12, 596, "stok", { len: 62, w: 150 }),
  ), js: join(
    inJs([`${p}rl`, `${p}ex`, `${p}q0`, `${p}q1`, `${p}q2`], 0.4, { y: 30 }, { stagger: 0.1 }),
    qa.map((a, i) => drawJs(`${p}qa${i}`, 1.0 + i * 0.1, a.len, 0.4)),
    inJs(copies.map((k) => k.id), 1.7, { scale: 0 }, { dur: 0.3, stagger: 0.06 }), inJs(`${p}cl`, 2.4, { y: 10 }),
    outJs([...rabbit, ...copies.map((k) => k.id), `${p}cl`], 6.0, 0.35),
    inJs(`${p}bl`, 6.0, { y: 10 }), inJs(c.ids, 6.3, { y: 30 }, { stagger: 0.1 }),
    inJs([`${p}end`, `${p}endl`], 7.0, { y: 0 }),
    inJs(`${p}pe`, 11.2, { y: 40 }), inJs(`${p}ps`, 11.5, { y: 40 }),
    moveJs(`${p}pe`, 16.4, 117, 0.6),
  ) };
};

// AMQP 0-9-1, counted from its spec: the handshake, channels, 6 classes and 53 methods, four frame types
const amqp = (p) => {
  const rows = [
    ["koneksi TCP dibuka dengan handshake", "start, start-ok, tune, tune-ok, open, open-ok"],
    ["channel di dalam koneksi", "channel.open, channel.open-ok"],
    ["6 class, 53 method", "connection, channel, exchange, queue, basic, tx"],
    ["4 jenis frame biner", "method, header, body, heartbeat"],
  ];
  return { css: DIAGRAM_CSS, html: join(rows.map(([a, b], i) => box(`${p}r${i}`, 0, 30 + i * 196, 960, 170, `${nm(a)}${sm(b)}`, "l"))),
    js: join(inJs(rows.map((_, i) => `${p}r${i}`), 0.4, { x: 60 }, { stagger: 0.15 }), hotJs(`${p}r2`, 5.6), hotJs(`${p}r0`, 10.1)) };
};

// one record: 24-byte header (length, CRC-32C, ID, time) and the data; the CRC covers ID to the end
const record = (p) => {
  const y = 300, f = [["panjang", "4 byte", 0, 153], ["CRC-32C", "4 byte", 150, 153], ["ID", "8 byte", 300, 193], ["waktu", "8 byte", 490, 193], ["data", "isi pesan", 680, 280]];
  return { css: DIAGRAM_CSS, html: join(
    svg(960, 820, [{ id: `${p}hb`, d: "M0 222 L0 202 L683 202 L683 222", cls: "cream thin" }, { id: `${p}cb`, d: `M300 ${y + 165} L300 ${y + 185} L960 ${y + 185} L960 ${y + 165}` }]),
    text(`${p}hl`, 250, 160, "header 24 byte", "cream"),
    [0, 4, 8, 16, 24].map((o, i) => text(`${p}o${i}`, [0, 150, 300, 490, 680][i] + 6, y - 40, String(o), "mute")),
    f.map(([a, b, x, w], i) => box(`${p}f${i}`, x, y, w, 140, `${nm(a)}${sm(b)}`, i === 1 ? "hot" : "")),
    text(`${p}cl`, 500, y + 200, "dicakup CRC-32C"),
    text(`${p}say`, 0, y + 290, "panjang gak ikut dihitung CRC. kalau panjangnya rusak, CRC dari byte yang salah gak akan cocok", "say mute", { width: 960 }),
  ), js: join(
    inJs(f.map((_, i) => `${p}f${i}`), 0.4, { y: 30 }, { stagger: 0.12 }),
    inJs([0, 1, 2, 3, 4].map((i) => `${p}o${i}`), 0.6, { y: 10 }, { stagger: 0.12 }),
    drawJs(`${p}hb`, 4.8, 740, 0.5), inJs(`${p}hl`, 5.1, { y: 10 }),
    hotJs(`${p}f1`, 10.0), drawJs(`${p}cb`, 10.3, 720, 0.6), inJs(`${p}cl`, 10.7, { y: 10 }),
    inJs(`${p}say`, 15.4, { y: 20 }),
  ) };
};

// write vs write + fsync, measured on the author's laptop (3556 and 470182 ns/op)
const fsyncBars = (p) => ({ css: DIAGRAM_CSS, html: join(
  text(`${p}w1`, 0, 150, "write", "say", { "font-size": 44 }),
  bar(`${p}b1`, 0, 215, 12, 70),
  text(`${p}v1`, 36, 218, "~3,6 µs", "cream", { "font-size": 52 }),
  text(`${p}w2`, 0, 350, "write + fsync", "say", { "font-size": 44 }),
  bar(`${p}b2`, 0, 415, 960, 70),
  text(`${p}v2`, 0, 505, "~470 µs", "", { "font-size": 52 }),
  text(`${p}x`, 300, 510, "sekitar 130 kali lebih lama", "say mute", { "font-size": 36 }),
  box(`${p}f`, 0, 640, 960, 120, `<span class="dn" style="font-size: 46px">fsync sekali setiap detik</span>`, "fill"),
), js: join(
  inJs(`${p}w1`, 1.2, { y: 20 }), growJs(`${p}b1`, 1.5, 0.3), inJs(`${p}v1`, 1.7, { x: -20 }),
  inJs(`${p}w2`, 5.2, { y: 20 }), growJs(`${p}b2`, 5.5, 1.6), inJs(`${p}v2`, 7.0, { y: 10 }),
  inJs(`${p}x`, 10.2, { y: 10 }),
  inJs(`${p}f`, 15.2, { y: 30 }),
) });

// a torn write at the end of the log, and the cut at the last whole record
const torn = (p) => ({ css: DIAGRAM_CSS, html: join(
  text(`${p}fl`, 0, 120, "file log", "up mute"),
  box(`${p}r1`, 0, 170, 303, 120, nm("record ID 1")),
  box(`${p}r2`, 300, 170, 303, 120, nm("record ID 2")),
  box(`${p}r3`, 600, 170, 303, 120, `${nm("ID 3")}${sm("terpotong")}`, "dash"),
  vline(`${p}cut`, 598, 140, 190),
  text(`${p}cutl`, 612, 312, "ujung log setelah dibuka"),
  text(`${p}q0`, 0, 430, "/ panjangnya masuk akal?", "say", { "font-size": 36 }),
  text(`${p}q1`, 0, 495, "/ datanya lengkap?", "say", { "font-size": 36 }),
  text(`${p}q2`, 0, 560, "/ CRC-nya cocok?", "say", { "font-size": 36 }),
  text(`${p}res`, 0, 660, "record ID 3 gagal: file dipotong di situ", "say", { "font-size": 36, color: "#e85d26" }),
), js: join(
  inJs([`${p}fl`, `${p}r1`, `${p}r2`], 0.4, { y: 30 }, { stagger: 0.12 }),
  `      tl.fromTo($("${p}r3"), { scaleX: 0, transformOrigin: "0% 50%" }, { scaleX: 1, duration: 1.2, ease: "power1.in" }, 1.4);`,
  inJs([`${p}q0`, `${p}q1`, `${p}q2`], 10.2, { x: -30 }, { stagger: 0.5 }),
  inJs(`${p}res`, 12.0, { y: 20 }),
  outJs(`${p}r3`, 15.2, 0.15, 0.5), inJs([`${p}cut`, `${p}cutl`], 15.3, { y: -20 }),
) });

const { total } = buildWide({
  series: "bikin message broker sendiri · bagian 1",
  part: "01 / 06",
  scenes: [
    { type: "title", reg: "orange", dur: 7, still: true, chapter: "Pembuka: apa itu message broker",
      kicker: "seri baru · bagian 1 dari 6",
      lines: [{ text: "bikin message", size: 180, light: true }, { text: "broker sendiri", size: 180 }],
      lead: "pakai Go, cuma library standar. bagian 1: protokol dan log pesan." },
    { type: "split", dur: 14, kicker: "kenalan dulu", title: "apa itu message broker?", visual: flow("s02_"),
      notes: [
        { at: 1.0, html: "server yang menerima, menyimpan, dan mengantar pesan antar program" },
        { at: 4.6, html: `yang mengirim: ${m("producer")}. yang menerima: ${m("consumer")}` },
        { at: 8.6, html: `${m("ack")}: laporan consumer bahwa pesan sudah selesai dikerjakan` }] },
    { type: "split", dur: 12, kicker: "rencananya", title: "seri enam bagian", visual: parts("s03_"), chapter: "Rencana enam bagian",
      notes: [
        { at: 1.0, html: "message broker kecil dengan Go, dari nol" },
        { at: 4.5, html: "tiap bagian nambah satu kemampuan" },
        { at: 8.0, html: `bagian 1: ${m("protokol")} dan ${m("log pesan")}` }] },
    { type: "split", dur: 22, kicker: "anatomi", title: "satu log, satu posisi baca", visual: logVsQueue("s04_"), chapter: "Satu log, satu posisi baca",
      notes: [
        { at: 1.0, html: "RabbitMQ: exchange nyalin pesan ke setiap queue" },
        { at: 6.0, html: `broker kita: tiap pesan ditulis sekali di ${m("log")} milik topic-nya` },
        { at: 11.0, html: `tiap subscriber cuma punya satu angka: ${m("posisi baca")}` },
        { at: 16.0, html: "ack cukup mindahin angka itu" }] },
    { type: "split", dur: 20, kicker: "protokol yang sudah ada", title: "AMQP 0-9-1: lengkap, tapi besar", visual: amqp("s05_"), chapter: "AMQP 0-9-1",
      notes: [
        { at: 1.0, html: "protokol asli RabbitMQ. client-nya sudah ada untuk banyak bahasa" },
        { at: 5.5, html: `spesifikasinya: ${m("6 class")}, ${m("53 method")}` },
        { at: 10.0, html: "6 langkah pembukaan sebelum pesan pertama" },
        { at: 14.5, html: "dan model-nya queue yang dihapus setelah di-ack, bukan log" }] },
    { type: "split", dur: 22, kicker: "pilihan kita", title: "protokol sendiri, di atas HTTP", chapter: "Protokol sendiri di atas HTTP",
      cards: [{ head: "4 alamat brokerku", lines: [
        { conf: "PUT  /topics/{topic}", at: 0.7 },
        { conf: "POST /topics/{topic}", at: 0.85 },
        { conf: "PUT  /subscribers/{name}", at: 1.0 },
        { conf: "POST /subscribers/{name}/stream", at: 1.15 }] }],
      notes: [
        { at: 1.0, html: "Kafka dan NATS juga pakai protokol sendiri, sesuai model masing-masing" },
        { at: 6.0, line: 1, html: `tiap perintah satu request HTTP. producer bisa kirim pakai ${m("curl")}` },
        { at: 11.0, line: 3, html: `stream pull lewat ${m("HTTP/2")}: pesan dan ack di satu request` },
        { at: 16.0, html: "kelemahannya: gak ada client siap pakai, program RabbitMQ gak bisa langsung pindah" }] },
    { type: "statement", reg: "orange", dur: 7, chapter: "Kenapa log, dan anatomi satu record",
      lines: [{ text: "kenapa log?", size: 260 }],
      lead: "log: file yang cuma bertambah di ujungnya. cara paling sederhana nyimpan pesan yang berurutan.", leadAt: 1.4 },
    { type: "split", dur: 20, kicker: "anatomi satu record", title: "header 24 byte, lalu data", visual: record("s08_"),
      notes: [
        { at: 1.0, html: "tiap pesan disimpan sebagai satu record" },
        { at: 5.0, html: `${m("panjang")} 4 byte, ${m("CRC-32C")} 4 byte, ${m("ID")} 8 byte, ${m("waktu")} 8 byte` },
        { at: 10.0, html: "CRC-32C: angka pemeriksa. satu byte berubah, angkanya hampir pasti beda" },
        { at: 15.0, html: `Kafka dan PostgreSQL pakai ${m("CRC-32C")} untuk tugas yang sama` }] },
    { type: "split", dur: 20, kicker: "kapan pesan aman?", title: "write belum tentu sampai ke disk", visual: fsyncBars("s09_"), chapter: "write lawan fsync",
      notes: [
        { at: 1.0, html: `${m("write")} yang berhasil: data baru ada di memori OS` },
        { at: 5.0, html: `${m("fsync")} maksa data sampai ke disk, tapi lambat` },
        { at: 10.0, html: "di laptop: ~3,6 µs lawan ~470 µs per pesan" },
        { at: 15.0, html: "jadi broker fsync sekali tiap detik. listrik padam: hilang ~1 detik terakhir" }] },
    { type: "split", dur: 20, kicker: "listrik padam", title: "record yang terpotong dibuang", visual: torn("s10_"), chapter: "Record yang terpotong",
      notes: [
        { at: 1.0, html: "listrik bisa padam pas record lagi ditulis" },
        { at: 5.0, html: "ujung log bisa berisi record setengah jadi, atau sampah" },
        { at: 10.0, html: "saat log dibuka, semua record diperiksa dari awal" },
        { at: 15.0, html: "di record pertama yang gagal, file dipotong" }] },
    { type: "bullets", dur: 13, kicker: "kelemahan versi pertama", title: "yang belum beres", chapter: "Kelemahan dan bagian berikutnya",
      items: [
        { at: 1.5, text: "belum ada fsync berkala (tugas bagian 2)" },
        { at: 5.0, text: "satu file terus membesar, gak ada yang dihapus" },
        { at: 8.5, text: "rusak di tengah file? sesudahnya ikut dibuang" }] },
    { type: "title", reg: "orange", dur: 10,
      kicker: "lanjut bagian 2",
      lines: [{ text: "mengingat", size: 200, light: true }, { text: "posisi baca", size: 200 }],
      lead: "artikel lengkapnya, dengan kode Go dan test-nya: link di deskripsi.", leadAt: 1.4 },
  ],
});
console.log("bagian 1 (YouTube):", total, "s");
