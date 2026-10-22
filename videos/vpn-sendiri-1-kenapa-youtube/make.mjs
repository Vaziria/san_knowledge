// Builds the YouTube (16:9, ~3 min, captions only) version of "Bikin VPN sendiri" part 1: why the office PC can't be
// reached, the options, the VPS meeting point, WireGuard. Run: node make.mjs. Scene kit: ../vpn-sendiri-shared/wide.mjs.
import { buildWide } from "../vpn-sendiri-shared/wide.mjs";
import { DIAGRAM_CSS, box, inJs, drawJs, popJs, vpsMap } from "../vpn-sendiri-shared/diagrams.mjs";

const m = (s) => `<span class="m">${s}</span>`;

// Why "masuk" fails: the PC sits behind the office router; out works, in is dropped.
const router = (p) => ({
  css: DIAGRAM_CSS,
  html: [
    box(`${p}net`, 330, 40, 300, 130, "internet"),
    box(`${p}rt`, 330, 330, 300, 130, "router"),
    box(`${p}pc`, 330, 620, 300, 130, "PC kantor", true),
    `          <svg class="dg" viewBox="0 0 960 820" aria-hidden="true">
            <path id="${p}out" d="M400 620 L400 460 M400 330 L400 176 M400 176 L386 206 M400 176 L414 206" />
            <path id="${p}in" class="dim" d="M560 172 L560 322" />
          </svg>`,
    `          <div id="${p}lo" class="dlabel" style="left: 70px; top: 230px">keluar: bisa</div>`,
    `          <div id="${p}li" class="dlabel cream" style="left: 650px; top: 230px">masuk: ditolak</div>`,
    `          <div id="${p}x" class="xmark" style="left: 515px; top: 200px"><i></i><i></i></div>`,
  ].join("\n"),
  js: [inJs(`${p}net`, 0.4), inJs(`${p}rt`, 0.55), inJs(`${p}pc`, 0.7),
    drawJs(`${p}out`, 5.3, 460), inJs(`${p}lo`, 5.9, { x: -30 }),
    inJs(`${p}in`, 10.3, { y: -30 }), popJs(`${p}x`, 10.9), inJs(`${p}li`, 11.1, { x: 30 })].join("\n"),
});

// CGNAT: one public IP shared by many homes; your router has none of its own.
const cgnat = (p) => ({
  css: DIAGRAM_CSS,
  html: [
    box(`${p}net`, 330, 30, 300, 110, "internet"),
    box(`${p}isp`, 180, 250, 600, 140, `ISP<span class="ip">1 IP publik, dipakai bersama</span>`),
    box(`${p}r1`, 40, 560, 260, 120, "rumah A"),
    box(`${p}r2`, 350, 560, 260, 120, "rumahmu", true),
    box(`${p}r3`, 660, 560, 260, 120, "rumah C"),
    `          <svg class="dg" viewBox="0 0 960 820" aria-hidden="true">
            <path id="${p}up" d="M480 250 L480 140" />
            <path id="${p}l1" d="M170 560 L300 390" />
            <path id="${p}l2" d="M480 560 L480 390" />
            <path id="${p}l3" d="M790 560 L660 390" />
          </svg>`,
    `          <div id="${p}pf" class="dlabel cream" style="left: 300px; top: 728px">port forwarding</div>`,
    `          <div id="${p}st" class="strike" style="left: 290px; top: 744px; width: 380px"></div>`,
  ].join("\n"),
  js: [inJs(`${p}net`, 0.4), inJs(`${p}isp`, 0.55), inJs(`${p}r1`, 0.7), inJs(`${p}r2`, 0.8), inJs(`${p}r3`, 0.9),
    drawJs(`${p}up`, 4.8, 120, 0.3), drawJs(`${p}l1`, 5.0, 240, 0.4), drawJs(`${p}l2`, 5.1, 180, 0.4), drawJs(`${p}l3`, 5.2, 240, 0.4),
    `      tl.fromTo($("${p}r2"), { scale: 1 }, { scale: 1.06, duration: 0.25, ease: "power2.out", yoyo: true, repeat: 1 }, 9.3);`,
    inJs(`${p}pf`, 13.7, { y: 20 }),
    `      tl.fromTo($("${p}st"), { scaleX: 0 }, { scaleX: 1, duration: 0.4, ease: "power3.inOut" }, 14.3);`].join("\n"),
});

const total = buildWide({
  series: "bikin vpn sendiri · bagian 1",
  part: "01 / 03",
  scenes: [
    { type: "title", reg: "orange", dur: 6, slam: true,
      lines: [{ text: "PC kantor", size: 200 }, { text: "mau diremote", size: 200, light: true }, { text: "dari rumah?", size: 200, light: true }] },
    { type: "statement", reg: "dark", dur: 8, lines: [{ text: "gak bisa.", size: 300, acc: true }],
      lead: "bukan salah PC-nya. ini soal cara internet nyambung.", leadAt: 1.6 },
    { type: "split", dur: 20, kicker: "kenapa", title: "PC di belakang router", visual: router("s03"),
      notes: [
        { at: 1.0, html: "PC kantor ada di belakang router kantor" },
        { at: 5.0, html: `keluar ke internet: ${m("bisa")}` },
        { at: 10.0, html: `masuk dari luar: ${m("ditolak")}. router gak tahu harus diteruskan ke PC mana` },
        { at: 15.0, html: "makanya PC kantor gak bisa ditelpon dari rumah" }] },
    { type: "split", dur: 18, kicker: "di rumah lebih parah", title: "CGNAT", visual: cgnat("s04"),
      notes: [
        { at: 1.0, html: "banyak ISP rumahan pakai CGNAT" },
        { at: 4.5, html: "satu IP publik dipakai bareng banyak pelanggan" },
        { at: 9.0, html: "router-mu gak punya IP publik sendiri" },
        { at: 13.5, html: "jadi port forwarding di router rumah gak ada gunanya" }] },
    { type: "statement", reg: "orange", dur: 8,
      lines: [{ text: "dua-duanya", size: 150, light: true }, { text: "bisa nelpon keluar,", size: 150, light: true }, { text: "gak bisa ditelpon.", size: 150 }],
      lead: "itu kuncinya.", leadAt: 1.8 },
    { type: "compare", dur: 28, kicker: "pilihannya", title: "tiga cara remote PC kantor",
      panels: [
        { num: "01", name: "port forwarding", at: 2.0, lines: ["buka port di router kantor", "butuh IP publik dan akses ke router", "gak jalan kalau kena CGNAT"] },
        { num: "02", name: "aplikasi remote", at: 9.0, lines: ["pasang aplikasi, langsung jalan", "tapi bergantung pada server pihak lain"] },
        { num: "03", name: "VPN sendiri", at: 16.0, pay: true, lines: ["servernya punyamu sendiri", "semua PC jadi satu jaringan", "gratis, open source"] }] },
    { type: "split", dur: 24, kicker: "caranya", title: "kasih titik tengah",
      visual: vpsMap("s07", { at: 0.4, arrowsAt: 5.2, packetAt: 16.5 }),
      notes: [
        { at: 1.0, html: "sewa VPS kecil. dia punya IP publik" },
        { at: 5.0, html: "PC rumah dan PC kantor sama-sama nyambung ke VPS. arahnya keluar, jadi gak diblokir" },
        { at: 11.0, html: `tiap perangkat dapat alamat VPN: ${m("10.8.0.x")}` },
        { at: 16.0, html: "rumah ke kantor lewat VPS, rasanya satu jaringan" }] },
    { type: "bullets", dur: 20, kicker: "kenalan dengan WireGuard", title: "VPN modern yang ringan",
      items: [
        { at: 1.6, text: "tiap perangkat punya sepasang kunci" },
        { at: 6.8, text: "yang ditukar cuma kunci publik" },
        { at: 12.0, text: "jalan lewat UDP, biasanya port 51820" }] },
    { type: "bullets", dur: 18, kicker: "yang kamu butuhkan", title: "modalnya",
      items: [
        { at: 1.5, text: "VPS kecil dengan IP publik (Ubuntu / Debian)" },
        { at: 6.0, text: "aplikasi WireGuard di PC rumah dan kantor" },
        { at: 10.5, text: "akses admin di kedua PC" }] },
    { type: "bullets", dur: 16, kicker: "rencananya", title: "tiga bagian",
      items: [
        { at: 1.5, text: "bagian 1: idenya (video ini)" },
        { at: 5.5, text: "bagian 2: bikin server di VPS" },
        { at: 9.5, text: "bagian 3: sambungkan PC + Remote Desktop" }] },
    { type: "title", reg: "orange", dur: 10,
      lines: [{ text: "VPN-mu", size: 260 }, { text: "sendiri", size: 260 }],
      lead: "lanjut bagian 2: bikin servernya di VPS.", leadAt: 1.4 },
  ],
});
console.log("part 1 (YouTube):", total, "s");
