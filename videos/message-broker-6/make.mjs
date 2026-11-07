// Builds the TikTok cut (9:16, ~37 s, captions only) of "Membuat Message Broker Sendiri" Bagian 6, the series
// finale: one broker package run three ways (embedded, mounted in an application, standalone with a CLI), the biggest
// weakness, and when RabbitMQ, NATS or Kafka fits better; the end card closes the series. Facts from the article
// (docs/articles/membuat-message-broker-sendiri/seri-...-bagian-6-...md). Run: node make.mjs. Scene kit:
// ../message-broker-shared/tall.mjs and diagrams.mjs. YouTube version: ../message-broker-6-youtube.
import { buildTall } from "../message-broker-shared/tall.mjs";
import { box, nm, join, inJs, hotJs } from "../message-broker-shared/diagrams.mjs";

const m = (s) => `<span class="m">${s}</span>`;
// labels at 30 px or more: a phone screen shows the 1080-wide frame at about a third of its size
const say = (s, c = "#c8c4bc") => `<span class="dn" style="font-weight: 600; font-size: 34px; color: ${c}">${s}</span>`;

// one package, three ways to run it
const modes = (p) => {
  const rows = [["tertanam", "program Go manggil Publish sebagai fungsi"], ["di server aplikasi", "API broker di bawah /broker/"], ["berdiri sendiri", "brokerku server, plus CLI"]];
  return { h: 500, html: join(rows.map(([a, b], i) => box(`${p}r${i}`, 0, i * 170, 842, 150, `${nm(a)}${say(b)}`, "l", { gap: 8 }))), js: join(
    inJs(rows.map((_, i) => `${p}r${i}`), 0.3, { x: 50 }, { stagger: 0.25 }),
    hotJs(`${p}r0`, 2.9), hotJs(`${p}r1`, 3.05), hotJs(`${p}r2`, 3.2),
  ) };
};

// the biggest weakness: one server without copies, then three smaller ones
const weakness = (p) => {
  const small = ["detik terakhir bisa hilang", "log terus membesar", "pesan bisa sampai dua kali"];
  return { h: 520, html: join(
    box(`${p}a`, 0, 0, 842, 170, `${nm("satu server, tanpa salinan")}${say("server hilang: pesan dan posisi baca ikut hilang", "#f0ece5")}`, "l hot", { gap: 8 }),
    small.map((s, i) => box(`${p}s${i}`, 0, 196 + i * 110, 842, 92, say(s), "l mute")),
  ), js: join(
    `      tl.fromTo($("${p}a"), { scale: 0.9, opacity: 0 }, { scale: 1, opacity: 1, duration: 0.5, ease: "back.out(1.6)" }, 0.3);`,
    inJs(small.map((_, i) => `${p}s${i}`), 2.0, { x: 40 }, { stagger: 0.2 }),
  ) };
};

// when to choose which broker
const choose = (p) => {
  const rows = [["RabbitMQ", "routing, protokol standar"], ["NATS", "wildcard, request-reply"], ["Kafka", "partisi, riwayat panjang"], ["brokerku", "satu server, di dalam Go"]];
  const row = (a, b, dark) => `<div style="display: flex; width: 100%; align-items: center; gap: 24px"><span class="dn" style="width: 220px; flex: none">${a}</span>${say(b, dark ? "#111111" : "#f0ece5")}</div>`;
  return { h: 540, html: join(rows.map(([a, b], i) => box(`${p}r${i}`, 0, i * 136, 842, 120, row(a, b, i === 3), i === 3 ? "l fill" : "l"))), js: join(
    inJs([`${p}r0`, `${p}r1`, `${p}r2`], 0.3, { x: 50 }, { stagger: 0.3 }),
    `      tl.fromTo($("${p}r3"), { scale: 0.85, opacity: 0 }, { scale: 1, opacity: 1, duration: 0.5, ease: "back.out(1.6)" }, 2.9);`,
  ) };
};

const total = buildTall({
  series: "bikin message broker sendiri",
  part: "06 / 06",
  hook: { dur: 3.2, still: true, kicker: "bagian 6 / 6 · terakhir",
    lines: [{ text: "tiga cara", size: 150 }, { text: "menjalankan", size: 130, light: true }, { text: "broker", size: 150 }],
    lead: "lalu kapan pakai RabbitMQ, NATS, atau Kafka" },
  steps: [
    { kicker: "satu package", title: "tiga cara", dur: 5.0, visual: modes("step1_"),
      marks: [
        { at: 0.8, cap: "broker kita cuma package Go" },
        { at: 2.9, cap: "kodenya sama, beda cara masangnya" }] },
    { kicker: "cara 1 · tertanam", title: "Publish = fungsi", dur: 4.8,
      cards: [{ head: "contoh/tertanam/main.go", lines: [
        { conf: "b, err := broker.Open(\"data-toko\")", at: 0.5 },
        { conf: "id, err := b.Publish(\"pesanan\", data)", at: 0.7 }] }],
      marks: [
        { at: 1.2, line: 1, cap: "gak ada jaringan antara kode dan broker" },
        { at: 3.0, cap: "tapi broker hidup dan mati bareng programnya" }] },
    { kicker: "cara 2 · di aplikasi", title: "di bawah /broker/", dur: 5.0,
      cards: [{ head: "contoh/aplikasi/main.go", lines: [
        { conf: "mux.Handle(\"/broker/\",", at: 0.5 },
        { conf: "  http.StripPrefix(\"/broker\",", at: 0.7 },
        { conf: "    b.Handler()))", at: 0.9 }] }],
      marks: [
        { at: 1.2, line: 0, cap: "satu proses, satu port buat aplikasi dan broker" },
        { at: 3.0, line: 1, cap: `${m("StripPrefix")}: broker gak perlu tahu dipasang di mana` }] },
    { kicker: "cara 3 · berdiri sendiri", title: "brokerku + CLI", dur: 5.0,
      cards: [{ head: "terminal", lines: [
        { cmd: "brokerku subscriber list", at: 0.5 },
        { out: "NAMA   JENIS    GAGAL  DICOBA LAGI", at: 1.4 },
        { out: "email  pull     0      -", at: 1.55 },
        { out: "stok   webhook  2      23:31:54", at: 1.7 }] }],
      marks: [
        { at: 2.0, line: 3, cap: "stok gagal 2 kali, dicoba lagi 23:31:54" },
        { at: 3.5, cap: "tanpa harus baca catatan broker" }] },
    { kicker: "kelemahan terbesar", title: "satu server", dur: 5.0, visual: weakness("step5_"),
      marks: [
        { at: 0.8, cap: "satu titik kegagalan, kata dokumentasi NATS" },
        { at: 2.9, cap: "RabbitMQ, NATS, Kafka bisa nyalin ke beberapa server" }] },
    { kicker: "kapan pakai yang mana?", title: "pilih sesuai butuh", dur: 5.2, visual: choose("step6_"),
      marks: [
        { at: 0.8, cap: "broker matang buat kebutuhan yang lebih besar" },
        { at: 3.0, cap: "brokerku: cukup satu server, ikut di program Go" }] },
  ],
  end: { dur: 3.6, kicker: "akhir seri · 6 / 6",
    lines: [{ text: "enam bagian,", size: 130 }, { text: "satu broker.", size: 130, light: true }],
    lead: "yang paling penting: alasan di balik tiap keputusannya" },
  endSfx: { file: "ting.wav", at: 0.3, dur: 0.45, vol: 0.4 },
});
console.log("bagian 6 (TikTok):", total, "s");
