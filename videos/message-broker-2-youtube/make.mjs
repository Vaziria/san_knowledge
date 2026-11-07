// Builds the YouTube (16:9, ~3 min, captions only) video for "Membuat Message Broker Sendiri" Bagian 2: remembering
// every subscriber's read position, also after the broker stops. The story, numbers and diagrams follow the article
// (docs/articles/membuat-message-broker-sendiri/seri-...-bagian-2-...md). Run: node make.mjs. Scene kit:
// ../message-broker-shared/wide.mjs and diagrams.mjs.
import { buildWide } from "../message-broker-shared/wide.mjs";
import {
  DIAGRAM_CSS, box, text, nm, sm, cells, pointer, arrow, svg, vline, xmark, join,
  inJs, drawJs, popJs, hotJs, moveJs,
} from "../message-broker-shared/diagrams.mjs";

const m = (s) => `<span class="m">${s}</span>`;
const body = (s, c = "#a8a49c") => `<span class="dn" style="font-weight: 600; font-size: 32px; color: ${c}">${s}</span>`;
const top = { "justify-content": "flex-start", "padding-top": 30, gap: 16 };

// the read position is a byte position in the log file; a new subscriber starts at the end
const bytePos = (p) => {
  const y = 220, c = cells(`${p}c`, 0, y, ["ID 1", "ID 2", "ID 3", "ID 4"].map((t) => ({ t })), { w: 200, h: 110 });
  const offs = ["0", "36", "72", "108", "144"], ox = [...c.xs, c.end - 3];
  return { css: DIAGRAM_CSS, html: join(
    text(`${p}l`, 0, y - 60, "log topic pesanan", "up mute"),
    c.html,
    offs.map((o, i) => text(`${p}o${i}`, ox[i] - (i ? o.length * 8 : 0), y + 124, o, "mute")),
    vline(`${p}e`, c.end + 10, y - 20, 150), text(`${p}el`, c.end - 70, y - 60, "ujung"),
    pointer(`${p}pe`, c.xs[2] + 1, y + 170, "email: 72", { len: 60, w: 200 }),
    pointer(`${p}pn`, c.end + 10, y + 170, "baru", { len: 60, w: 130 }),
  ), js: join(
    inJs(`${p}l`, 0.3, { y: 10 }), inJs(c.ids, 0.4, { y: 30 }, { stagger: 0.1 }),
    inJs(`${p}pe`, 1.4, { y: 40 }),
    inJs(offs.map((_, i) => `${p}o${i}`), 5.7, { y: 10 }, { stagger: 0.1 }),
    hotJs(c.ids[2], 10.2, { text: true }),
    inJs([`${p}e`, `${p}el`], 14.2, { y: 0 }), inJs(`${p}pn`, 14.4, { y: 40 }),
  ) };
};

// subscribers.meta: fixed 512-byte slots; inside one slot, the read position is the first 8 bytes
const slots = (p) => {
  const fields = [["0-7", "posisi baca"], ["8", "dipakai"], ["9", "jenis"], ["10-11", "panjang nama"], ["12-75", "nama topic"], ["76-139", "nama subscriber"], ["140-511", "cadangan"]];
  const row = (a, b) => `<div style="display: flex; width: 100%; gap: 24px; align-items: baseline"><span class="ds" style="width: 170px">byte ${a}</span>${nm(b)}</div>`;
  const sl = [["slot 0", "email"], ["slot 1", "stok"], ["slot 2", "laporan"]];
  return { css: DIAGRAM_CSS, html: join(
    text(`${p}h`, 0, 0, "subscribers.meta · slot 512 byte", "cream"),
    sl.map(([a, b], i) => box(`${p}s${i}`, i * 240, 44, 243, 90, `${nm(a)}${sm(b)}`)),
    box(`${p}s3`, 720, 44, 240, 90, sm("slot baru"), "dash"),
    ["0", "512", "1024", "1536"].map((o, i) => text(`${p}so${i}`, i * 240 + 4, 142, o, "mute")),
    text(`${p}r`, 0, 206, "isi satu slot subscriber", "up mute"),
    fields.map(([a, b], i) => box(`${p}f${i}`, 0, 250 + i * 61, 960, 64, row(a, b), "l")),
    text(`${p}t`, 0, 692, "topics.meta · slot 128 byte: ID yang dicadangkan, dipakai, panjang, nama topic, cadangan", "say mute", { width: 960 }),
  ), js: join(
    inJs([`${p}h`, `${p}s0`, `${p}s1`, `${p}s2`, `${p}s3`], 0.4, { y: 20 }, { stagger: 0.1 }),
    inJs(fields.map((_, i) => `${p}f${i}`), 1.4, { x: 40 }, { stagger: 0.08 }), inJs(`${p}r`, 1.3, { y: 0 }),
    inJs(["0", "1", "2", "3"].map((i) => `${p}so${i}`), 5.7, { y: 10 }, { stagger: 0.1 }),
    hotJs(`${p}f0`, 10.7),
    inJs(`${p}t`, 15.6, { y: 20 }),
  ) };
};

// the write order: every publish and ack goes to the OS; every second, fsync the logs first, then the metadata
const writeOrder = (p) => {
  const a = arrow(424, 300, 536, 300);
  return { css: DIAGRAM_CSS, html: join(
    box(`${p}pu`, 0, 20, 465, 130, `${nm("publish")}${sm("record ditulis ke OS")}`),
    box(`${p}ak`, 495, 20, 465, 130, `${nm("ack")}${sm("posisi ditulis ke OS")}`),
    text(`${p}sd`, 0, 196, "setiap detik", "up mute"),
    box(`${p}f1`, 0, 240, 420, 120, nm("1 · fsync semua log"), "hot"),
    svg(960, 820, [{ id: `${p}a`, d: a.d }]),
    box(`${p}f2`, 540, 240, 420, 120, nm("2 · fsync metadata"), "hot"),
    box(`${p}pa`, 0, 430, 465, 270, `${nm("program mati mendadak")}${body("memori OS tetap ada: semua yang sudah ditulis selamat, walau belum di-fsync")}`, "l flat", top),
    box(`${p}pb`, 495, 430, 465, 270, `${nm("listrik padam atau OS mati")}${body("cuma yang sudah di-fsync yang selamat: paling lama ~1 detik terakhir hilang")}`, "l flat", top),
  ), js: join(
    inJs([`${p}pu`, `${p}ak`], 0.4, { y: 30 }, { stagger: 0.15 }),
    inJs([`${p}sd`, `${p}f1`], 5.0, { y: 30 }, { stagger: 0.1 }), drawJs(`${p}a`, 5.5, a.len, 0.4), inJs(`${p}f2`, 5.8, { x: -30 }),
    inJs(`${p}pa`, 10.1, { y: 40 }), inJs(`${p}pb`, 15.1, { y: 40 }),
  ) };
};

// on reopen the log is the truth: three cases
const restart = (p) => {
  const cw = { w: 90, h: 76 };
  const a = cells(`${p}a`, 0, 40, [1, 2, 3, 4, 5].map((t) => ({ t })), cw);
  const b = cells(`${p}b`, 0, 320, [{ t: 1 }, { t: 2 }, { t: 3, cls: "dash" }], cw);
  const c = cells(`${p}c`, 0, 600, [1, 2, 3].map((t) => ({ t })), cw);
  const say = (id, y, s) => text(id, 500, y, s, "say", { width: 460 });
  return { css: DIAGRAM_CSS, html: join(
    text(`${p}al`, 0, 0, "posisi di belakang ujung log"), a.html, vline(`${p}ae`, a.end + 8, 30, 96),
    pointer(`${p}ap`, a.xs[2] + 45, 118, "email", { len: 44, w: 130, h: 50 }),
    say(`${p}as`, 40, "email lanjut dari pesan 3. yang belum di-ack dikirim lagi"),
    text(`${p}bl`, 0, 280, "posisi di depan ujung log"), b.html, vline(`${p}be`, b.xs[2] + 2, 310, 96),
    pointer(`${p}bp`, b.end + 6, 398, "email", { len: 44, w: 130, h: 50 }),
    say(`${p}bs`, 320, "posisinya dimundurkan ke ujung log. pesan 3 hilang bersama listriknya"),
    text(`${p}cl`, 0, 560, "cadangan ID di depan log"), c.html,
    text(`${p}cr`, c.end + 24, 620, "cadangan:", "cream"),
    text(`${p}cr2`, c.end + 24, 652, "ID 1000", "cream"),
    box(`${p}cn`, 0, 706, 460, 64, "pesan berikutnya: ID 1001", "fill", { "font-size": 24 }),
    say(`${p}cs`, 600, "ID 4 sampai 1000 gak pernah dipakai, jadi gak ada ID kembar"),
  ), js: join(
    inJs([`${p}al`, ...a.ids, `${p}ae`], 1.0, { y: 20 }, { stagger: 0.06 }), inJs(`${p}ap`, 1.8, { y: 30 }), inJs(`${p}as`, 2.4, { y: 20 }),
    inJs([`${p}bl`, ...b.ids, `${p}be`], 8.0, { y: 20 }, { stagger: 0.06 }), inJs(`${p}bp`, 8.6, { y: 30 }),
    moveJs(`${p}bp`, 9.6, -(b.end + 6 - (b.xs[2] + 2)), 0.6), inJs(`${p}bs`, 10.0, { y: 20 }),
    inJs([`${p}cl`, ...c.ids], 15.0, { y: 20 }, { stagger: 0.06 }), inJs([`${p}cr`, `${p}cr2`], 15.6, { x: -20 }), inJs(`${p}cn`, 16.2, { y: 20 }), inJs(`${p}cs`, 16.6, { y: 20 }),
  ) };
};

// why an ID must never be used twice: the article's message 118 story, then the fix
const idStory = (p) => {
  const steps = ["1 · broker mengantar pesan 118", "2 · consumer mencatat: ID 118 sudah diproses", "3 · listrik padam: log berakhir di 117", "4 · pesan baru dapat ID 118 lagi"];
  return { css: DIAGRAM_CSS, html: join(
    steps.map((s, i) => box(`${p}r${i}`, 0, i * 124, 960, 100, nm(s), i === 3 ? "l hot" : "l")),
    xmark(`${p}x`, 860, 3 * 124 + 15),
    text(`${p}xl`, 590, 3 * 124 + 108, "dilewati diam-diam", "", { "font-size": 30 }),
    box(`${p}fix`, 0, 580, 960, 200, `${nm("cadangkan ID per 1.000")}${body("batas bloknya ditulis dan di-fsync dulu, sebelum ID pertamanya dipakai", "rgba(17, 17, 17, 0.8)")}`, "l fill", top),
  ), js: join(
    inJs(steps.slice(0, 3).map((_, i) => `${p}r${i}`), 1.2, { x: 50 }, { stagger: 1.0 }),
    inJs(`${p}r3`, 6.0, { x: 50 }), popJs(`${p}x`, 6.5), inJs(`${p}xl`, 6.8, { y: 10 }),
    inJs(`${p}fix`, 11.2, { y: 40 }),
  ) };
};

const { total } = buildWide({
  series: "bikin message broker sendiri · bagian 2",
  part: "02 / 06",
  scenes: [
    { type: "title", reg: "orange", dur: 7, still: true, chapter: "Pembuka: yang harus diingat broker",
      kicker: "bagian 2 dari 6",
      lines: [{ text: "broker mati,", size: 160 }, { text: "subscriber lanjut", size: 160, light: true }, { text: "dari mana?", size: 160, light: true }],
      lead: "bagian 2: mengingat posisi baca setiap subscriber." },
    { type: "bullets", dur: 14, kicker: "selain pesan", title: "tiga hal yang harus diingat",
      items: [
        { at: 1.6, text: "daftar topic, dan file log miliknya" },
        { at: 5.4, text: "daftar subscriber, topic-nya, dan posisi bacanya" },
        { at: 9.2, text: "penghitung ID, biar ID gak dipakai dua kali" }] },
    { type: "split", dur: 18, kicker: "posisi baca", title: "cukup satu angka per subscriber", visual: bytePos("s03_"), chapter: "Posisi baca cukup satu angka",
      notes: [
        { at: 1.0, html: "pesan di log selalu berurutan, jadi posisi cukup satu angka" },
        { at: 5.5, html: `bedanya dengan ${m("offset")} Kafka: ini posisi byte di file` },
        { at: 10.0, html: "broker langsung baca record di posisi itu, tanpa nyari" },
        { at: 14.0, html: "subscriber baru mulai di ujung log: cuma dapat pesan baru" }] },
    { type: "compare", dur: 20, kicker: "di mana posisi disimpan?", title: "tiga cara nyimpan posisi baca", chapter: "Tiga cara menyimpan posisi",
      panels: [
        { num: "01", name: "JSON + rename", at: 2.0, lines: ["tulis ulang seluruh daftar, lalu ganti nama file", "aman, tapi berat untuk tiap ack"] },
        { num: "02 · dipilih", name: "slot + WriteAt", at: 7.0, pay: true, lines: ["ubah 8 byte di tempatnya", "sekitar 2,8 µs per tulis"] },
        { num: "03", name: "memory-mapped file", at: 12.0, lines: ["sekitar 2,8 ns per tulis, seribu kali lebih cepat", "tapi OS yang pegang kendali"] }] },
    { type: "bullets", dur: 15, kicker: "kenapa bukan mmap?", title: "cepat, tapi kendalinya hilang", chapter: "Kenapa bukan mmap",
      items: [
        { at: 1.6, text: "kapan data sampai ke file, OS yang menentukan" },
        { at: 5.4, text: "salah akses jadi sinyal SIGBUS: program Go berhenti" },
        { at: 9.2, text: "library standar Go gak punya Mmap untuk Windows" }] },
    { type: "split", dur: 22, kicker: "anatomi file metadata", title: "slot berukuran tetap", visual: slots("s06_"), chapter: "Anatomi file metadata",
      notes: [
        { at: 1.0, html: `dua file: ${m("topics.meta")} dan ${m("subscribers.meta")}` },
        { at: 5.5, html: "slot ke-i mulai di byte i × 512, jadi letaknya bisa dihitung" },
        { at: 10.5, html: "satu ack cuma nulis 8 byte di awal slot" },
        { at: 15.5, html: `log tiap topic dinamai nomor slot-nya: ${m("topic-0.log")}` }] },
    { type: "split", dur: 20, kicker: "urutan nulis ke disk", title: "log dulu, baru metadata", visual: writeOrder("s07_"), chapter: "Urutan menulis ke disk",
      notes: [
        { at: 1.0, html: "tiap publish dan ack: tulis ke OS, tanpa fsync" },
        { at: 5.0, html: "tiap detik: fsync semua log dulu, baru metadata" },
        { at: 10.0, html: "program mati mendadak: OS tetap nyimpan semuanya" },
        { at: 15.0, html: "listrik padam: yang belum di-fsync hilang" }] },
    { type: "statement", reg: "orange", dur: 7, chapter: "Saat broker dibuka lagi",
      lines: [{ text: "metadata", size: 220, light: true }, { text: "ngikutin log.", size: 220 }],
      lead: "saat dibuka lagi, setiap posisi baca dan penghitung ID dicocokkan dengan ujung log.", leadAt: 1.4 },
    { type: "split", dur: 24, kicker: "saat broker dibuka lagi", title: "tiga keadaan", visual: restart("s09_"),
      notes: [
        { at: 1.0, html: "posisi di belakang ujung log: ack terakhir belum sampai ke disk" },
        { at: 8.0, html: "posisi di depan ujung log: record-nya gak sampai ke disk" },
        { at: 15.0, html: "cadangan ID di depan log: ID lanjut setelah cadangan" },
        { at: 19.5, html: `pesan bisa terkirim dua kali, tapi gak hilang: ${m("at-least-once")}` }] },
    { type: "split", dur: 20, kicker: "kenapa ID gak boleh kembar?", title: "cadangan ID per 1.000", visual: idStory("s10_"), chapter: "Kenapa ID tidak boleh kembar",
      notes: [
        { at: 1.0, html: `consumer yang ${m("idempotent")} nyatet ID yang sudah diproses` },
        { at: 6.0, html: "kalau ID dipakai ulang, pesan baru dilewati diam-diam" },
        { at: 11.0, html: "jadi broker mencadangkan ID per 1.000, dicatat di disk dulu" },
        { at: 15.5, html: "kelemahannya: ID melompat, misalnya dari 3 ke 1001" }] },
    { type: "bullets", dur: 13, kicker: "kelemahan versi kedua", title: "yang belum beres", chapter: "Kelemahan dan bagian berikutnya",
      items: [
        { at: 1.5, text: "listrik padam bisa menghilangkan ~1 detik terakhir" },
        { at: 5.0, text: "ID melompat setelah broker mati mendadak" },
        { at: 8.5, text: "belum ada program lain yang bisa menerima pesan" }] },
    { type: "title", reg: "orange", dur: 10,
      kicker: "lanjut bagian 3",
      lines: [{ text: "subscriber yang", size: 150, light: true }, { text: "menarik pesannya", size: 150 }, { text: "sendiri", size: 150 }],
      lead: "artikel lengkapnya, dengan kode Go dan test-nya: link di deskripsi.", leadAt: 1.4 },
  ],
});
console.log("bagian 2 (YouTube):", total, "s");
