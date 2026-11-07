// Builds the YouTube (16:9, ~3 min, captions only) video for "Membuat Message Broker Sendiri" Bagian 3: a subscriber
// that pulls its own messages over one HTTP/2 request. The story, numbers, logs and diagrams follow the article
// (docs/articles/membuat-message-broker-sendiri/seri-...-bagian-3-...md). Run: node make.mjs. Scene kit:
// ../message-broker-shared/wide.mjs and diagrams.mjs.
import { buildWide } from "../message-broker-shared/wide.mjs";
import { DIAGRAM_CSS, box, text, nm, sm, arrow, svg, join, inJs, drawJs, hotJs } from "../message-broker-shared/diagrams.mjs";

const m = (s) => `<span class="m">${s}</span>`;
const body = (s) => `<span class="dn" style="font-weight: 600; font-size: 30px; color: #c8c4bc">${s}</span>`;
const top = { "justify-content": "flex-start", "padding-top": 26, gap: 14 };

// the pull stream as a sequence: hello, 200, message 1, ack 1, message 2, no ack for 30 s, message 2 again, ack 2
const sequence = (p) => {
  const L = 165, R = 795, mid = (L + R) / 2;
  const msgs = [
    { y: 150, dir: 1, label: "POST /subscribers/email/stream?ack_timeout=30s", cls: "cream", at: 1.4, fs: 19 },
    { y: 240, dir: -1, label: "200 OK", cls: "cream", at: 5.3 },
    { y: 330, dir: -1, label: '{"id":1,"time":...,"data":"..."}', at: 9.3 },
    { y: 420, dir: 1, label: '{"ack":1}', cls: "cream", at: 11.0 },
    { y: 510, dir: -1, label: '{"id":2,"time":...,"data":"..."}', at: 13.8 },
    { y: 680, dir: -1, label: '{"id":2,"time":...,"data":"..."}', at: 18.3 },
    { y: 770, dir: 1, label: '{"ack":2}', cls: "cream", at: 20.0 },
  ];
  const arrows = msgs.map((g, i) => ({ ...g, id: `${p}m${i}`, a: g.dir > 0 ? arrow(L, g.y, R - 4, g.y) : arrow(R, g.y, L + 4, g.y) }));
  const w = (s, fs = 24) => s.length * fs * 0.66;
  return { css: DIAGRAM_CSS, html: join(
    box(`${p}s`, 0, 0, 330, 70, nm("subscriber email")),
    box(`${p}b`, 630, 0, 330, 70, nm("broker")),
    svg(960, 820, [
      { id: `${p}ls`, d: `M${L} 74 L${L} 810`, cls: "dash" }, { id: `${p}lb`, d: `M${R} 74 L${R} 810`, cls: "dash" },
      ...arrows.map((g) => ({ id: g.id, d: g.a.d, cls: g.cls ? `${g.cls} thin` : "thin" })),
    ]),
    arrows.map((g) => text(`${g.id}l`, Math.round(mid - w(g.label, g.fs ?? 24) / 2), g.y - 38, g.label.replace(/</g, "&lt;"), g.cls ?? "", { "font-size": g.fs ?? 24 })),
    box(`${p}gap`, R - 10, 548, 20, 96, "", "hot", { background: "transparent" }),
    text(`${p}gl`, 250, 578, "email gagal: 30 detik tanpa ack", "cream"),
  ), js: join(
    inJs([`${p}s`, `${p}b`, `${p}ls`, `${p}lb`], 0.4, { y: 20 }, { stagger: 0.1 }),
    arrows.map((g) => join(drawJs(g.id, g.at, g.a.len, 0.5), inJs(`${g.id}l`, g.at + 0.2, { y: 10 }))),
    inJs([`${p}gap`, `${p}gl`], 15.4, { y: 0 }),
  ) };
};

// one message in flight: ~140 µs per message and ack; with a 1 s ack timeout and 3 s of work, message 5 came three times
const inflight = (p) => ({ css: DIAGRAM_CSS, html: join(
  text(`${p}v`, 0, 20, "~140 µs", "stat", { "font-size": 150 }),
  text(`${p}a`, 0, 190, "satu putaran pesan dan ack, di laptop", "say mute", { "font-size": 34 }),
  text(`${p}b`, 0, 240, "sekitar 7.000 pesan per detik per subscriber", "say", { "font-size": 36 }),
  `          <div id="${p}c" class="card" style="position: absolute; left: 0; top: 400px">
            <div class="head" style="text-transform: none; letter-spacing: 0.04em; font-size: 24px">catatan pembaca · -kerja 3s -ack-timeout 1s</div>
            <div class="code">
              <div id="${p}l0" class="ln out">21:57:12 pesan 5: pesanan 1045 dibuat</div>
              <div id="${p}l1" class="ln out">21:57:15 pesan 5: pesanan 1045 dibuat</div>
              <div id="${p}l2" class="ln out">21:57:18 pesan 5: pesanan 1045 dibuat</div>
            </div>
          </div>`,
), js: join(
  `      tl.fromTo($("${p}v"), { scale: 0.7, opacity: 0, transformOrigin: "0% 50%" }, { scale: 1, opacity: 1, duration: 0.5, ease: "back.out(1.6)" }, 5.6);`,
  inJs(`${p}a`, 6.0, { y: 10 }), inJs(`${p}b`, 6.5, { y: 10 }),
  inJs(`${p}c`, 10.8, { x: 60 }),
  inJs([`${p}l0`, `${p}l1`, `${p}l2`], 16.2, { x: -18 }, { stagger: 0.5, dur: 0.3 }),
) });

// one stream per subscriber: a second one gets 409; a silently dead connection is found by HTTP/2 pings (15 s + 15 s)
const oneStream = (p) => {
  const a1 = arrow(280, 80, 614, 160);
  const ticks = [40, 300, 560, 900];
  const labels = ["paket terakhir lewat", "15 detik diam: ping dikirim", "15 detik tanpa jawaban", "koneksi ditutup, stream dilepas"];
  return { css: DIAGRAM_CSS, html: join(
    box(`${p}p1`, 0, 30, 280, 100, nm("program 1")),
    box(`${p}p2`, 0, 230, 280, 100, nm("program 2")),
    box(`${p}br`, 620, 110, 340, 150, `${nm("broker")}${sm("stream email")}`, "hot"),
    svg(960, 820, [
      { id: `${p}a1`, d: a1.d },
      { id: `${p}a2`, d: "M280 280 L612 222", cls: "dash" },
      { id: `${p}tl`, d: "M0 500 L950 500 M950 500 L932 490 M950 500 L932 510", cls: "cream thin" },
      ...ticks.map((x, i) => ({ id: `${p}t${i}`, d: `M${x} 484 L${x} 516`, cls: "thin" })),
    ]),
    text(`${p}409`, 330, 300, "409 Conflict", "", { "font-size": 30 }),
    labels.map((s, i) => text(`${p}tx${i}`, i === 3 ? 740 : Math.max(0, ticks[i] - 40), 536, s, "say", { width: 220, "font-size": 28, "text-align": i === 3 ? "right" : "left" })),
    box(`${p}ok`, 0, 690, 960, 100, body("setelah itu email bisa nyambung lagi, paling lama ~30 detik kemudian"), "l flat"),
  ), js: join(
    inJs([`${p}p1`, `${p}br`], 0.4, { y: 30 }, { stagger: 0.15 }), drawJs(`${p}a1`, 0.9, a1.len, 0.5),
    inJs(`${p}p2`, 1.4, { y: 30 }), inJs([`${p}a2`, `${p}409`], 2.0, { x: -20 }, { stagger: 0.2 }),
    inJs(`${p}tl`, 10.2, { x: -30 }), inJs([`${p}t0`, `${p}tx0`, `${p}t1`, `${p}tx1`], 10.6, { y: 10 }, { stagger: 0.25 }),
    inJs([`${p}t2`, `${p}tx2`, `${p}t3`, `${p}tx3`], 15.7, { y: 10 }, { stagger: 0.25 }),
    inJs(`${p}ok`, 17.4, { y: 30 }),
  ) };
};

// the delivery loop: woken by a publish, an ack or 100 ms; one step per subscriber, never waiting
const loop = (p) => {
  const trig = ["publish baru", "ack datang", "100 ms berlalu"];
  const tin = [arrow(145, 84, 380, 166), arrow(480, 84, 480, 166), arrow(815, 84, 580, 166)];
  const tout = [arrow(380, 296, 150, 408), arrow(480, 296, 480, 408), arrow(580, 296, 810, 408)];
  const cols = [["email", "pegang pesan 4 baru 2 detik: cek ack, belum ada, lanjut"], ["stok", "gak pegang pesan: kirim pesan 7 ke stream-nya"], ["laporan", "pesan 3 lewat 30 detik: kirim lagi"]];
  return { css: DIAGRAM_CSS, html: join(
    trig.map((t, i) => box(`${p}g${i}`, i * 335, 0, 290, 80, t)),
    box(`${p}lp`, 230, 172, 500, 120, `${nm("loop pengantar")}${sm("satu goroutine")}`, "hot"),
    svg(960, 820, [...tin.map((a, i) => ({ id: `${p}i${i}`, d: a.d, cls: "cream thin" })), ...tout.map((a, i) => ({ id: `${p}o${i}`, d: a.d }))]),
    cols.map(([n, t], i) => box(`${p}c${i}`, i * 330, 414, 300, 300,
      `<span class="ds" style="font-size: 30px; color: #e85d26; font-weight: 600">${n}</span>${body(t)}`, "l flat", top)),
  ), js: join(
    inJs(trig.map((_, i) => `${p}g${i}`), 0.4, { y: 20 }, { stagger: 0.12 }),
    tin.map((a, i) => drawJs(`${p}i${i}`, 0.9 + i * 0.1, a.len, 0.4)), inJs(`${p}lp`, 1.1, { scale: 0.9 }),
    drawJs(`${p}o0`, 6.1, tout[0].len, 0.4), drawJs(`${p}o1`, 6.3, tout[1].len, 0.4),
    inJs([`${p}c0`, `${p}c1`], 6.4, { y: 30 }, { stagger: 0.3 }),
    drawJs(`${p}o2`, 11.1, tout[2].len, 0.4), inJs(`${p}c2`, 11.3, { y: 30 }),
  ) };
};

const { total } = buildWide({
  series: "bikin message broker sendiri · bagian 3",
  part: "03 / 06",
  scenes: [
    { type: "title", reg: "orange", dur: 7, still: true, chapter: "Pembuka: push atau pull",
      kicker: "bagian 3 dari 6",
      lines: [{ text: "pesan dan ack", size: 170 }, { text: "lewat satu", size: 170, light: true }, { text: "request HTTP/2", size: 170 }],
      lead: "bagian 3: subscriber yang menarik pesannya sendiri." },
    { type: "compare", dur: 20, kicker: "dua cara ngantar pesan", title: "push atau pull?",
      panels: [
        { num: "push", name: "broker datang ke subscriber", at: 2.0,
          lines: ["broker memanggil webhook milik subscriber (bagian 4)", "kata dokumentasi Kafka: subscriber yang lambat cenderung kewalahan"] },
        { num: "pull · bagian ini", name: "subscriber datang sendiri", at: 9.0, pay: true,
          lines: ["subscriber cukup tertinggal, lalu menyusul saat mampu", "cuma butuh koneksi keluar: jalan di balik NAT atau firewall"] }] },
    { type: "split", dur: 24, kicker: "anatomi stream pull", title: "satu request, dua arah", visual: sequence("s03_"), chapter: "Anatomi stream pull",
      notes: [
        { at: 1.0, html: `request-nya sendiri adalah ${m("hello")}` },
        { at: 5.0, html: `${m("200 OK")}: stream ini milik email` },
        { at: 9.0, html: "pesan mengalir di body response, satu baris JSON" },
        { at: 13.5, html: "ack mengalir di body request yang sama" },
        { at: 18.0, html: "ack gak datang? pesan yang sama dikirim lagi" }] },
    { type: "compare", dur: 20, kicker: "kenapa satu request HTTP/2?", title: "tiga cara bawa pesan dan ack", chapter: "Kenapa satu request HTTP/2",
      panels: [
        { num: "01", name: "WebSocket", at: 2.0, lines: ["sudah teruji di banyak aplikasi", "tapi library standar Go gak punya"] },
        { num: "02", name: "dua request", at: 7.0, lines: ["jalan di HTTP/1.1, bisa dicoba pakai curl", "tapi ack lewat koneksi lain yang mungkin sudah putus"] },
        { num: "03 · dipilih", name: "satu request HTTP/2", at: 12.0, pay: true, lines: ["pesan dan ack lewat koneksi yang sama", "sejak Go 1.24: HTTP/2 tanpa TLS"] }] },
    { type: "split", dur: 24, kicker: "satu pesan dipegang sampai di-ack", title: "prefetch 1 dan timeout ack", visual: inflight("s05_"), chapter: "Prefetch 1 dan timeout ack",
      notes: [
        { at: 1.0, html: `pesan berikutnya baru dikirim setelah ack: ${m("prefetch 1")}` },
        { at: 5.5, html: "urutan terjaga, tapi throughput turun: ~7.000 pesan/detik" },
        { at: 10.5, html: "ack gak datang dalam 30 detik? dikirim lagi, seperti AckWait di JetStream" },
        { at: 16.0, html: "satu pesan bisa dikerjakan 3 kali. subscriber harus idempotent" }] },
    { type: "split", dur: 22, kicker: "satu stream per subscriber", title: "stream kedua ditolak", visual: oneStream("s06_"), chapter: "Satu stream per subscriber",
      notes: [
        { at: 1.0, html: `stream kedua dengan nama sama: ${m("409 Conflict")}` },
        { at: 5.5, html: "mirip exclusive consumer di RabbitMQ" },
        { at: 10.0, html: "koneksi mati diam-diam? broker kirim ping HTTP/2 setelah 15 detik diam" },
        { at: 15.5, html: "15 detik gak dijawab: koneksi ditutup, stream dilepas" }] },
    { type: "split", dur: 22, kicker: "loop pengantar", title: "satu goroutine, gak pernah nunggu", visual: loop("s07_"), chapter: "Loop pengantar",
      notes: [
        { at: 1.0, html: "satu goroutine keliling ke tiap subscriber yang stream-nya terbuka" },
        { at: 6.0, html: "ack datang: posisi maju. gak pegang pesan: kirim berikutnya" },
        { at: 11.0, html: "lewat timeout ack: pesan dikirim lagi" },
        { at: 16.0, html: "loop gak pernah nunggu jaringan, dan dipakai lagi untuk webhook di bagian 4" }] },
    { type: "split", dur: 20, kicker: "dicoba di Windows", title: "brokerku dan pembaca", chapter: "Mencoba brokerku dan pembaca",
      cards: [{ head: "catatan pembaca", lines: [
        { out: "pesan 1: pesanan 1041 dibuat", at: 1.0 },
        { out: "pesan 2: pesanan 1042 dibuat", at: 1.3 },
        { out: "pesan 3: pesanan 1043 dibuat", at: 1.6 },
        { out: "409 Conflict, mencoba lagi dalam 2 detik", at: 5.6 },
        { out: "pesan 4: pesanan 1044 dibuat", at: 10.0 },
        { out: "pesan 4: pesanan 1044 dibuat", at: 10.6 },
        { out: "pesan 1001: pesanan 1046 dibuat", at: 15.0 }] }],
      notes: [
        { at: 2.2, line: 2, html: "tiga pesan dikirim pakai curl, sampai berurutan" },
        { at: 6.0, line: 3, html: "pembaca kedua untuk email ditolak" },
        { at: 11.0, line: 5, html: "pembaca dimatikan sebelum ack: pesan 4 datang lagi" },
        { at: 15.4, line: 6, html: "broker dimatikan paksa: pembaca nyambung lagi, ID lanjut dari 1001" }] },
    { type: "bullets", dur: 13, kicker: "kelemahan versi ketiga", title: "yang belum beres", chapter: "Kelemahan dan bagian berikutnya",
      items: [
        { at: 1.5, text: "pesan yang gak pernah di-ack dikirim ulang selamanya" },
        { at: 5.0, text: "satu stream per subscriber: kerjanya gak bisa dibagi" },
        { at: 8.5, text: "belum ada autentikasi: broker cuma dengar di 127.0.0.1" }] },
    { type: "title", reg: "orange", dur: 10,
      kicker: "lanjut bagian 4",
      lines: [{ text: "webhook dan", size: 170, light: true }, { text: "percobaan ulang", size: 170 }],
      lead: "artikel lengkapnya, dengan kode Go dan percobaannya: link di deskripsi.", leadAt: 1.4 },
  ],
});
console.log("bagian 3 (YouTube):", total, "s");
