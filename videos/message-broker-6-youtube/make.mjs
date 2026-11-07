// Builds the YouTube (16:9, ~3 min, captions only) video for "Membuat Message Broker Sendiri" Bagian 6, the series
// finale: the same broker package run three ways (embedded, mounted in an application's HTTP server, standalone with a
// CLI), the series recap, the weaknesses of the whole design, and when RabbitMQ, NATS or Kafka is the better choice. The
// story, logs and diagrams follow the article (docs/articles/membuat-message-broker-sendiri/seri-...-bagian-6-...md).
// Run: node make.mjs. Scene kit: ../message-broker-shared/wide.mjs and diagrams.mjs.
import { buildWide } from "../message-broker-shared/wide.mjs";
import { DIAGRAM_CSS, box, text, nm, sm, arrow, svg, join, inJs, drawJs, hotJs } from "../message-broker-shared/diagrams.mjs";

const m = (s) => `<span class="m">${s}</span>`;
const body = (s, c = "#c8c4bc") => `<span class="dn" style="font-weight: 600; font-size: 30px; color: ${c}">${s}</span>`;
const code = (s) => `<span style="font-family: 'IBM Plex Mono'; font-weight: 600; font-size: 22px; color: #f0ece5">${s}</span>`;

// one package, three ways to run it
const runModes = (p) => {
  const col = (i) => i * 330;
  const small = { "font-size": 22 }, tiny = { "font-size": 20 };
  const up = (x) => arrow(x, 562, x, 486);
  const ups = [up(col(1) + 70), up(col(1) + 230), up(col(2) + 45), up(col(2) + 150), up(col(2) + 255)];
  const pub = [arrow(col(0) + 60, 204, col(0) + 60, 286), arrow(col(1) + 60, 204, col(1) + 60, 286)];
  const hook = arrow(150, 484, 150, 560);
  return { css: DIAGRAM_CSS, html: join(
    ["tertanam", "di server aplikasi", "berdiri sendiri"].map((s, i) => text(`${p}h${i}`, col(i), 0, nm(s), "cream")),
    [0, 1, 2].map((i) => box(`${p}c${i}`, col(i), 50, 300, 430, "", "flat", { background: "#161614" })),
    text(`${p}l0`, 20, 66, "program toko", "mute", small), text(`${p}l1`, col(1) + 16, 66, "server aplikasi :8090", "mute", tiny),
    text(`${p}l2`, col(2) + 16, 66, "brokerku server :8080", "mute", tiny),
    box(`${p}k0`, 20, 110, 260, 90, nm("kode toko")),
    box(`${p}k1`, col(1) + 20, 110, 260, 90, code("POST /pesanan")),
    box(`${p}b0`, 20, 290, 260, 100, nm("broker"), "hot"),
    box(`${p}b1`, col(1) + 20, 290, 260, 100, nm("broker di /broker/"), "hot"),
    box(`${p}b2`, col(2) + 20, 110, 260, 280, nm("broker"), "hot"),
    svg(960, 820, [
      ...pub.map((a, i) => ({ id: `${p}p${i}`, d: a.d })), { id: `${p}wh`, d: hook.d, cls: "cream thin" },
      ...ups.map((a, i) => ({ id: `${p}u${i}`, d: a.d, cls: "cream thin" })),
    ]),
    text(`${p}pl0`, 76, 230, "b.Publish(...)", "", small), text(`${p}pl1`, col(1) + 76, 230, "b.Publish(...)", "", small),
    box(`${p}w`, 20, 566, 260, 90, nm("webhook stok")),
    box(`${p}r0`, col(1), 566, 140, 90, "pembaca", "", small), box(`${p}r1`, col(1) + 160, 566, 140, 90, "CLI", "", small),
    box(`${p}s0`, col(2), 566, 90, 90, "toko", "", small), box(`${p}s1`, col(2) + 100, 566, 110, 90, "pembaca", "", small),
    box(`${p}s2`, col(2) + 220, 566, 80, 90, "CLI", "", small),
    ["tanpa jaringan, hidup dan mati bareng programnya", "satu proses, satu port buat aplikasi dan broker", "semua program manggil broker lewat HTTP"]
      .map((s, i) => text(`${p}t${i}`, col(i), 690, s, "say mute", { width: 300, "font-size": 26 })),
  ), js: join(
    inJs([`${p}h0`, `${p}c0`, `${p}l0`, `${p}k0`], 1.2, { y: 20 }, { stagger: 0.08 }), drawJs(`${p}p0`, 1.7, pub[0].len, 0.3),
    inJs([`${p}pl0`, `${p}b0`], 1.9, { y: 10 }, { stagger: 0.1 }), drawJs(`${p}wh`, 2.3, hook.len, 0.3), inJs([`${p}w`, `${p}t0`], 2.5, { y: 20 }, { stagger: 0.15 }),
    inJs([`${p}h1`, `${p}c1`, `${p}l1`, `${p}k1`], 5.0, { y: 20 }, { stagger: 0.08 }), drawJs(`${p}p1`, 5.5, pub[1].len, 0.3),
    inJs([`${p}pl1`, `${p}b1`], 5.7, { y: 10 }, { stagger: 0.1 }), inJs([`${p}r0`, `${p}r1`], 6.1, { y: 20 }, { stagger: 0.1 }),
    drawJs(`${p}u0`, 6.4, ups[0].len, 0.3), drawJs(`${p}u1`, 6.5, ups[1].len, 0.3), inJs(`${p}t1`, 6.7, { y: 20 }),
    inJs([`${p}h2`, `${p}c2`, `${p}l2`, `${p}b2`], 9.6, { y: 20 }, { stagger: 0.08 }),
    inJs([`${p}s0`, `${p}s1`, `${p}s2`], 10.2, { y: 20 }, { stagger: 0.1 }),
    [2, 3, 4].map((i) => drawJs(`${p}u${i}`, 10.5 + (i - 2) * 0.1, ups[i].len, 0.3)), inJs(`${p}t2`, 10.9, { y: 20 }),
    hotJs(`${p}b0`, 14.2), hotJs(`${p}b1`, 14.4), hotJs(`${p}b2`, 14.6),
  ) };
};

// the six parts, one row each
const recap = (p) => {
  const parts = [
    ["log per topic", "protokol 4 alamat HTTP, record dengan CRC-32C"],
    ["posisi baca", "slot berukuran tetap, fsync log dulu baru metadata"],
    ["stream pull", "satu request HTTP/2, satu pesan sampai di-ack"],
    ["webhook", "jeda berlipat dua, dilewati setelah 10 kali gagal"],
    ["worker pool", "urutan terjaga, giliran yang adil"],
    ["tiga cara", "tertanam, di server aplikasi, berdiri sendiri"],
  ];
  const row = (i, a, b) => `<div style="display: flex; gap: 22px; align-items: baseline"><span class="ds" style="font-size: 26px; color: #e85d26; font-weight: 600">0${i + 1}</span>${nm(a)}</div>${body(b)}`;
  return { css: DIAGRAM_CSS, html: join(parts.map(([a, b], i) => box(`${p}r${i}`, 0, 8 + i * 132, 960, 116, row(i, a, b), "l", { gap: 6 }))), js: join(
    inJs([`${p}r0`, `${p}r1`], 1.2, { x: 50 }, { stagger: 0.6 }),
    inJs([`${p}r2`, `${p}r3`, `${p}r4`], 6.2, { x: 50 }, { stagger: 0.6 }),
    inJs(`${p}r5`, 11.2, { x: 50 }), hotJs(`${p}r5`, 11.6),
  ) };
};

// the weaknesses of the whole design
const weak = (p) => {
  const rows = [
    ["satu server, tanpa salinan", "server hilang: semua pesan dan posisi baca ikut hilang"],
    ["pesan di detik terakhir bisa hilang", "fsync cuma sekali tiap detik"],
    ["log terus membesar", "belum ada retensi"],
    ["pesan bisa sampai lebih dari sekali", "subscriber harus idempotent"],
    ["satu subscriber, satu pesan sekaligus", "throughput dibatasi lama satu pesan"],
    ["tanpa autentikasi, gak ada dead-letter", "dan belum pernah dijalankan di Linux"],
  ];
  return { css: DIAGRAM_CSS, html: join(rows.map(([a, b], i) => box(`${p}r${i}`, 0, 10 + i * 132, 960, 116, `${nm(a)}${body(b)}`, i ? "l" : "l hot", { gap: 6 }))), js: join(
    inJs(`${p}r0`, 1.2, { x: 50 }),
    inJs([`${p}r1`, `${p}r2`, `${p}r3`, `${p}r4`], 11.2, { x: 50 }, { stagger: 0.5 }),
    inJs(`${p}r5`, 15.7, { x: 50 }),
  ) };
};

// when to choose which broker
const choose = (p) => {
  const rows = [
    ["RabbitMQ", "routing pakai exchange, AMQP, MQTT, STOMP", "quorum queue disalin ke beberapa server"],
    ["NATS", "subject dengan wildcard, request-reply", "JetStream: pesan tersimpan, ack, R=3"],
    ["Kafka", "throughput besar lewat partisi", "retensi, riwayat panjang bisa dibaca ulang"],
    ["brokerku", "satu server sudah cukup", "broker ikut di dalam program Go"],
  ];
  const row = (n, a, b, dark) => `<div style="display: flex; width: 100%; align-items: center; gap: 28px"><span class="dn" style="width: 200px; flex: none; font-size: 40px">${n}</span><div style="display: flex; flex-direction: column; gap: 6px">${body(a, dark ? "#111111" : "#f0ece5")}${body(b, dark ? "rgba(17, 17, 17, 0.75)" : "#a8a49c")}</div></div>`;
  return { css: DIAGRAM_CSS, html: join(rows.map(([n, a, b], i) => box(`${p}r${i}`, 0, 12 + i * 198, 960, 176, row(n, a, b, i === 3), i === 3 ? "l fill" : "l"))), js: join(
    rows.map((_, i) => inJs(`${p}r${i}`, [1.2, 6.2, 11.2, 15.7][i], { x: 50 })),
  ) };
};

const { total } = buildWide({
  series: "bikin message broker sendiri · bagian 6",
  part: "06 / 06",
  scenes: [
    { type: "title", reg: "orange", dur: 7, still: true, chapter: "Pembuka: satu package, tiga cara",
      kicker: "bagian 6 dari 6 · terakhir",
      lines: [{ text: "satu broker,", size: 170 }, { text: "tiga cara", size: 170, light: true }, { text: "menjalankannya", size: 170 }],
      lead: "bagian terakhir: menjalankan broker, lalu kapan RabbitMQ, NATS, atau Kafka lebih tepat." },
    { type: "split", dur: 18, kicker: "broker kita cuma package Go", title: "satu package, tiga cara", visual: runModes("s02_"),
      notes: [
        { at: 1.0, html: "sejak bagian 2, broker kita adalah package Go" },
        { at: 5.0, html: `${m("Open")} buka folder data, ${m("Handler")} berisi seluruh API HTTP` },
        { at: 9.5, html: "tertanam, dipasang di server aplikasi, atau berdiri sendiri" },
        { at: 14.0, html: "kodenya sama: bedanya siapa yang manggil Open, dan di mana Handler dipasang" }] },
    { type: "compare", dur: 16, kicker: "bukankah broker harus berdiri sendiri?", title: "berdiri sendiri atau tertanam?", chapter: "Harus selalu berdiri sendiri?",
      panels: [
        { num: "berdiri sendiri", name: "layanan tersendiri", at: 1.8, lines: ["bisa di-restart dan diperbarui tanpa nyentuh aplikasinya", "tapi satu layanan lagi: dijalankan, dipantau, dicadangkan"] },
        { num: "tertanam", name: "di dalam program", at: 6.0, lines: ["Publish cuma panggilan fungsi, tanpa koneksi yang bisa putus", "tapi hidup dan mati bareng programnya"] },
        { num: "seri ini", name: "ketiganya bisa", at: 10.2, pay: true, lines: ["gak milih salah satu", "ketiganya mungkin, dengan package yang sama"] }] },
    { type: "split", dur: 18, kicker: "cara 1 · tertanam", title: "Publish cuma panggilan fungsi", chapter: "Tertanam di program Go",
      cards: [
        { head: "contoh/tertanam/main.go", lines: [
          { conf: "b, err := broker.Open(\"data-toko\")", at: 0.8 },
          { conf: "id, err := b.Publish(\"pesanan\", data)", at: 1.0 }] },
        { head: "dijalankan dua kali", lines: [
          { cmd: "tertanam", at: 5.6 },
          { out: "pesanan 1041 dicatat, pesan 1 diterbitkan", at: 6.4 },
          { out: "pesanan 1043 dicatat, pesan 3 diterbitkan", at: 6.6 },
          { cmd: "tertanam", at: 9.8 },
          { out: "pesanan 1041 dicatat, pesan 4 diterbitkan", at: 10.6 },
          { out: "pesanan 1043 dicatat, pesan 6 diterbitkan", at: 10.8 }] }],
      notes: [
        { at: 1.6, card: 0, line: 1, html: "program toko manggil Publish sebagai fungsi biasa, tanpa jaringan" },
        { at: 6.8, card: 1, line: 2, html: "tanpa server broker terpisah: webhook stok tetap dapat pesannya" },
        { at: 11.2, card: 1, line: 5, html: "dijalankan lagi: ID lanjut dari 4, stok gak dapat pesan lama" },
        { at: 14.8, html: "broker hidup dan mati bareng programnya" }] },
    { type: "split", dur: 18, kicker: "cara 2 · dipasang di aplikasi", title: "API broker di bawah /broker/", chapter: "Dipasang di server aplikasi",
      cards: [
        { head: "contoh/aplikasi/main.go", lines: [
          { conf: "mux.HandleFunc(\"POST /pesanan\", ...)", at: 0.8 },
          { conf: "mux.Handle(\"/broker/\",", at: 1.0 },
          { conf: "  http.StripPrefix(\"/broker\",", at: 1.2 },
          { conf: "    b.Handler()))", at: 1.4 }] },
        { head: "terminal", lines: [
          { cmd: "curl -X POST -d ... :8090/pesanan", at: 6.0 },
          { out: "pesanan diterima, pesan 1", at: 7.2 },
          { cmd: "brokerku publish pesanan ...", at: 8.6 },
          { out: "pesan 2 terkirim ke topic pesanan", at: 9.8 }] }],
      notes: [
        { at: 1.8, card: 0, line: 1, html: "aplikasi masang seluruh API broker di bawah /broker/" },
        { at: 5.2, card: 0, line: 2, html: `${m("StripPrefix")} buang awalannya: broker gak perlu tahu dipasang di mana` },
        { at: 10.2, card: 1, line: 3, html: "pesan 1 lewat alamat aplikasi, pesan 2 lewat API broker, port yang sama" },
        { at: 14.4, html: "kelemahannya: tanpa autentikasi, siapa pun yang bisa jangkau port bisa pakai" }] },
    { type: "split", dur: 19, kicker: "cara 3 · berdiri sendiri", title: "brokerku server dan CLI-nya", chapter: "Berdiri sendiri, dengan CLI",
      cards: [{ head: "terminal", lines: [
        { cmd: "brokerku topic create pesanan", at: 1.0 },
        { out: "topic pesanan dibuat", at: 2.2 },
        { cmd: "brokerku topic create pesanan", at: 4.4 },
        { out: "409 Conflict: sudah ada", at: 5.6 },
        { cmd: "brokerku subscriber list", at: 9.0 },
        { out: "NAMA   JENIS    GAGAL  DICOBA LAGI", at: 10.0 },
        { out: "email  pull     0      -", at: 10.2 },
        { out: "stok   webhook  2      23:31:54", at: 10.4 }] }],
      notes: [
        { at: 2.6, line: 1, html: "satu program: brokerku server, plus perintah yang manggil lewat HTTP" },
        { at: 6.0, line: 3, html: "error dari broker ditampilkan apa adanya" },
        { at: 10.8, line: 7, html: "subscriber list: stok sudah gagal 2 kali, dicoba lagi 23:31:54" },
        { at: 15.0, html: `${m("curl")} juga cukup. CLI ngasih error yang kebaca dan tabel` }] },
    { type: "statement", reg: "orange", dur: 7, chapter: "Rangkuman enam bagian",
      lines: [{ text: "enam bagian,", size: 220 }, { text: "satu broker.", size: 220, light: true }],
      lead: "broker-nya sendiri cuma sekitar 1.400 baris Go, pakai library standar saja.", leadAt: 1.2 },
    { type: "split", dur: 20, kicker: "rangkuman seri", title: "yang kita bangun", visual: recap("s08_"),
      notes: [
        { at: 1.0, html: "bagian 1-2: nyimpan pesan dan posisi baca, tahan mati mendadak" },
        { at: 6.0, html: "bagian 3-5: ngantar pesan, lewat pull dan webhook" },
        { at: 11.0, html: "bagian 6: tiga cara menjalankannya" },
        { at: 15.0, html: "semuanya satu module Go, tanpa package dari luar" }] },
    { type: "split", dur: 20, kicker: "sebelum dipakai", title: "kelemahan seluruh rancangan", visual: weak("s09_"), chapter: "Kelemahan seluruh rancangan",
      notes: [
        { at: 1.0, html: "satu server tanpa salinan: kata NATS, satu titik kegagalan" },
        { at: 6.0, html: "Kafka umumnya 3 salinan. nyalin dengan aman butuh Raft: satu seri sendiri" },
        { at: 11.0, html: "fsync sekali sedetik, log gak pernah dibuang, pesan bisa dobel" },
        { at: 15.5, html: "tanpa autentikasi, gak ada <span style=\"white-space: nowrap\">dead-letter</span>, belum dicoba di Linux" }] },
    { type: "split", dur: 20, kicker: "broker yang sudah matang", title: "kapan pakai yang mana?", visual: choose("s10_"), chapter: "Kapan RabbitMQ, NATS, atau Kafka",
      notes: [
        { at: 1.0, html: "RabbitMQ: routing pakai aturan, banyak bahasa, protokol standar" },
        { at: 6.0, html: "NATS: banyak layanan saling kirim pesan cepat, termasuk tanya-jawab" },
        { at: 11.0, html: "Kafka: pesan sangat banyak, riwayat yang perlu dibaca ulang" },
        { at: 15.5, html: "brokerku: satu server cukup, broker ikut di dalam program Go" }] },
    { type: "bullets", dur: 13, kicker: "mau lanjut sendiri?", title: "daftar pekerjaan berikutnya", chapter: "Penutup seri",
      items: [
        { at: 1.5, text: "kecil: autentikasi dan kunci folder data" },
        { at: 5.0, text: "sedang: retensi, log dipecah jadi beberapa file" },
        { at: 8.5, text: "besar: salinan di beberapa server" }] },
    { type: "title", reg: "orange", dur: 10,
      kicker: "akhir seri · bikin message broker sendiri",
      lines: [{ text: "yang paling penting:", size: 120, light: true }, { text: "alasan di balik", size: 160 }, { text: "tiap keputusan.", size: 160 }],
      lead: "makasih sudah ngikutin enam bagian. artikel lengkapnya, dengan kode Go dan test-nya: link di deskripsi.", leadAt: 1.6 },
  ],
});
console.log("bagian 6 (YouTube):", total, "s");
