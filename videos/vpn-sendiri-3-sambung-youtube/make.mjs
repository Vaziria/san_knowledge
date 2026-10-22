// Builds the YouTube (16:9, ~3 min, captions only) version of "Bikin VPN sendiri" part 3: connecting the office and
// home PCs, then Remote Desktop. Run: node make.mjs. Scene kit: ../vpn-sendiri-shared/wide.mjs.
import { buildWide } from "../vpn-sendiri-shared/wide.mjs";
import { vpsMap } from "../vpn-sendiri-shared/diagrams.mjs";

const m = (s) => `<span class="m">${s}</span>`;

const clientConfig = (addr, t0) => [
  { conf: "[Interface]", at: t0 },
  { conf: "PrivateKey = (otomatis)", at: t0 + 0.12 },
  { conf: `Address = ${addr}/24`, at: t0 + 0.24 },
  { blank: true },
  { conf: "[Peer]", at: t0 + 0.36 },
  { conf: "PublicKey = <isi server.pub>", at: t0 + 0.48 },
  { conf: "Endpoint = IP_VPS:51820", at: t0 + 0.6 },
  { conf: "AllowedIPs = 10.8.0.0/24", at: t0 + 0.72 },
  { conf: "PersistentKeepalive = 25", at: t0 + 0.84 },
];

const total = buildWide({
  series: "bikin vpn sendiri · bagian 3",
  part: "03 / 03",
  scenes: [
    { type: "title", reg: "orange", dur: 7, slam: true,
      lines: [{ text: "sambungkan", size: 190, light: true }, { text: "PC rumah", size: 190 }, { text: "& kantor", size: 190 }],
      lead: "lalu Remote Desktop dari rumah.", leadAt: 1.4 },
    { type: "split", dur: 13, kicker: "hari ini", title: "dua PC-nya", visual: vpsMap("s02", { at: 0.4, arrowsAt: 1.2, hot: "pcs", packetAt: 7.5 }),
      notes: [
        { at: 1.0, html: "servernya sudah nyala (bagian 2)" },
        { at: 4.0, html: `PC kantor ${m("10.8.0.3")}, PC rumah ${m("10.8.0.2")}` },
        { at: 8.5, html: "tiap PC: pasang aplikasi, isi config, daftarkan di server" }] },
    { type: "split", dur: 17, kicker: "langkah 1 / 6", title: "pasang aplikasinya",
      cards: [{ head: "aplikasi WireGuard", lines: [
        { conf: "wireguard.com/install", at: 0.9 },
        { blank: true },
        { conf: "Add Tunnel", at: 3.6 },
        { conf: "› Add empty tunnel...", at: 3.8 },
        { blank: true },
        { conf: "Public key: (salin ini)", at: 7.4 }] }],
      notes: [
        { at: 1.0, line: 0, html: "unduh di wireguard.com/install: Windows, Mac, Linux, Android, iPhone" },
        { at: 4.2, line: 3, html: `klik ${m("Add Tunnel")}, lalu ${m("Add empty tunnel")}` },
        { at: 7.8, line: 5, html: "kuncinya dibuat otomatis. salin public key-nya" },
        { at: 12.5, html: "lakukan di PC kantor dan PC rumah" }] },
    { type: "split", dur: 26, kicker: "langkah 2 / 6", title: "config PC kantor",
      cards: [{ head: "tunnel · PC kantor", lines: clientConfig("10.8.0.3", 0.6) }],
      notes: [
        { at: 2.0, line: 2, html: `${m("Address")}: alamat PC kantor di VPN` },
        { at: 6.5, line: 5, html: `${m("PublicKey")}: kunci publik server (server.pub)` },
        { at: 11.0, line: 6, html: `${m("Endpoint")}: IP VPS-mu, port 51820` },
        { at: 15.5, line: 7, html: `${m("AllowedIPs")}: cuma jaringan VPN yang lewat sini` },
        { at: 20.0, line: 8, html: `${m("PersistentKeepalive")}: biar tetap nyambung di balik CGNAT` }] },
    { type: "split", dur: 12, kicker: "langkah 3 / 6", title: "config PC rumah",
      cards: [{ head: "tunnel · PC rumah", lines: clientConfig("10.8.0.2", 0.5) }],
      notes: [
        { at: 1.6, line: 2, html: `sama persis, cuma Address-nya ${m("10.8.0.2")}` },
        { at: 6.0, html: "kunci rahasianya beda, dibuat aplikasi di PC rumah" }] },
    { type: "split", dur: 25, kicker: "langkah 4 / 6", title: "daftarkan di server",
      cards: [
        { head: "terminal · VPS", lines: [
          { cmd: "sudo nano /etc/wireguard/wg0.conf", at: 1.4 },
          { cmd: "sudo systemctl restart wg-quick@wg0", at: 19.6 }] },
        { head: "/etc/wireguard/wg0.conf", lines: [
          { conf: "[Peer]  # PC kantor", at: 3.4 },
          { conf: "PublicKey = <public key PC kantor>", at: 3.55 },
          { conf: "AllowedIPs = 10.8.0.3/32", at: 3.7 },
          { blank: true },
          { conf: "[Peer]  # PC rumah", at: 3.85 },
          { conf: "PublicKey = <public key PC rumah>", at: 4.0 },
          { conf: "AllowedIPs = 10.8.0.2/32", at: 4.15 }] }],
      notes: [
        { at: 1.0, html: "buka lagi config server, tambahkan di bawah [Interface]" },
        { at: 5.0, card: 1, line: 0, html: `satu ${m("[Peer]")} untuk tiap PC` },
        { at: 9.5, card: 1, line: 1, html: `${m("PublicKey")}: public key yang disalin dari aplikasi` },
        { at: 14.0, card: 1, line: 2, html: `${m("AllowedIPs")}: alamat PC itu, ${m("/32")}` },
        { at: 19.0, html: "simpan, lalu muat ulang" }] },
    { type: "split", dur: 18, kicker: "langkah 5 / 6", title: "activate & cek",
      cards: [{ head: "terminal · VPS", lines: [
        { cmd: "sudo wg", at: 4.0 },
        { out: "peer: (public key PC kantor)", at: 4.9 },
        { out: "  allowed ips: 10.8.0.3/32", at: 5.1 },
        { out: "  latest handshake: 5 seconds ago", at: 5.3, check: true }] }],
      notes: [
        { at: 1.0, html: `di kedua PC, klik ${m("Activate")}` },
        { at: 3.6, html: "lalu cek di VPS" },
        { at: 7.5, line: 3, html: `${m("latest handshake")} muncul = sudah nyambung` },
        { at: 12.5, html: "gak muncul? cek Endpoint, port, dan kunci" }] },
    { type: "split", dur: 22, kicker: "langkah 6 / 6", title: "remote desktop",
      cards: [
        { head: "PC kantor · settings", lines: [
          { conf: "System › Remote Desktop", at: 0.9 },
          { conf: "Remote Desktop: On", at: 1.1 }] },
        { head: "terminal · PC rumah", lines: [
          { cmd: "ping 10.8.0.3", at: 7.0 },
          { out: "Reply from 10.8.0.3", at: 8.0, check: true },
          { out: "Reply from 10.8.0.3", at: 8.35, check: true },
          { out: "Reply from 10.8.0.3", at: 8.7, check: true },
          { cmd: "mstsc /v:10.8.0.3", at: 13.0 }] }],
      notes: [
        { at: 1.4, line: 1, html: "di PC kantor: nyalakan Remote Desktop (Windows Pro)" },
        { at: 6.5, html: "dari rumah, tes dulu dengan ping" },
        { at: 12.5, html: `lalu buka Remote Desktop ke ${m("10.8.0.3")}` },
        { at: 17.0, html: "layar PC kantor kebuka di rumah" }] },
    { type: "bullets", dur: 16, kicker: "kalau belum jalan", title: "cek ini dulu",
      items: [
        { at: 1.5, text: "ping gagal? izinkan ping di firewall PC kantor" },
        { at: 5.5, text: "gak ada handshake? cek Endpoint, port, kunci" },
        { at: 9.5, text: "kantor blokir UDP? WireGuard butuh UDP" }] },
    { type: "title", reg: "orange", dur: 10,
      lines: [{ text: "VPN-mu", size: 260 }, { text: "jadi.", size: 260 }],
      lead: "mentok di langkah mana? tulis di komentar, nanti aku bantu.", leadAt: 1.3 },
  ],
});
console.log("part 3 (YouTube):", total, "s");
