// Builds part 3 of "Bikin VPN sendiri": connecting the office and home PCs to the server, as code cards.
// Run: node make.mjs (writes index.html and compositions/). The cards and their timing are ../vpn-sendiri-shared/cards.mjs.
import { buildVideo } from "../vpn-sendiri-shared/cards.mjs";

const m = (s) => `<span class="m">${s}</span>`;

const total = buildVideo({
  series: "bikin vpn sendiri",
  part: "03 / 03",
  hook: { dur: 2.4, kicker: "bagian 3 · 4 langkah", slam: true,
    lines: [{ text: "sambungkan", size: 140 }, { text: "PC rumah", size: 140 }, { text: "& kantor", size: 140 }] },
  steps: [
    { kicker: "langkah 1 / 4", title: "pasang di tiap PC", dur: 5.6,
      cards: [{ head: "aplikasi WireGuard", lines: [
        { conf: "wireguard.com/install", at: 0.7 },
        { blank: true },
        { conf: "Add Tunnel", at: 1.4 },
        { conf: "› Add empty tunnel...", at: 1.6 }] }],
      marks: [
        { at: 2.2, line: 0, cap: "Windows, Mac, Linux, Android, iPhone" },
        { at: 3.6, line: 3, cap: `kuncinya dibuat otomatis, salin ${m("public key")}-nya` }] },
    { kicker: "langkah 2 / 4", title: "config PC kantor", dur: 10.4,
      cards: [{ head: "tunnel · PC kantor", lines: [
        { conf: "[Interface]", at: 0.6 },
        { conf: "PrivateKey = (otomatis)", at: 0.72 },
        { conf: "Address = 10.8.0.3/24", at: 0.84 },
        { blank: true },
        { conf: "[Peer]", at: 0.96 },
        { conf: "PublicKey = <isi server.pub>", at: 1.08 },
        { conf: "Endpoint = IP_VPS:51820", at: 1.2 },
        { conf: "AllowedIPs = 10.8.0.0/24", at: 1.32 },
        { conf: "PersistentKeepalive = 25", at: 1.44 }] }],
      marks: [
        { at: 1.9, line: 2, cap: "alamat PC kantor di VPN" },
        { at: 3.3, line: 5, cap: "kunci publik server dari bagian 2" },
        { at: 4.7, line: 6, cap: "ganti dengan IP VPS-mu" },
        { at: 6.1, line: 7, cap: "cuma jaringan VPN yang lewat sini" },
        { at: 7.5, line: 8, cap: "biar tetap nyambung di balik CGNAT" },
        { at: 8.9, line: 2, cap: `PC rumah sama, Address-nya ${m("10.8.0.2")}` }] },
    { kicker: "langkah 3 / 4", title: "daftarkan di server", dur: 7.4,
      cards: [
        { head: "/etc/wireguard/wg0.conf", lines: [
          { conf: "[Peer]  # PC kantor", at: 0.6 },
          { conf: "PublicKey = <public key PC kantor>", at: 0.72 },
          { conf: "AllowedIPs = 10.8.0.3/32", at: 0.84 },
          { blank: true },
          { conf: "[Peer]  # PC rumah", at: 0.96 },
          { conf: "PublicKey = <public key PC rumah>", at: 1.08 },
          { conf: "AllowedIPs = 10.8.0.2/32", at: 1.2 }] },
        { head: "terminal · VPS", lines: [{ cmd: "sudo systemctl restart wg-quick@wg0", at: 4.6 }] }],
      marks: [
        { at: 1.8, line: 2, cap: `satu ${m("[Peer]")} untuk tiap PC` },
        { at: 3.3, line: 6, cap: `tambahkan di bawah ${m("[Interface]")}` },
        { at: 5.9, cap: "lalu muat ulang" }] },
    { kicker: "langkah 4 / 4", title: "tes dari rumah", dur: 8.4,
      cards: [{ head: "terminal · PC rumah", lines: [
        { cmd: "ping 10.8.0.3", at: 1.6 },
        { out: "Reply from 10.8.0.3", at: 2.4, check: true },
        { out: "Reply from 10.8.0.3", at: 2.75, check: true },
        { out: "Reply from 10.8.0.3", at: 3.1, check: true },
        { blank: true },
        { cmd: "mstsc /v:10.8.0.3", at: 4.4 }] }],
      marks: [
        { at: 0.5, cap: `klik ${m("Activate")} di kedua PC` },
        { at: 5.6, cap: "layar PC kantor kebuka di rumah" }] },
  ],
  end: { dur: 4.6, kicker: "bikin vpn sendiri · selesai", slam: true,
    lines: [{ text: "VPN-mu", size: 200 }, { text: "jadi.", size: 200 }],
    lead: "mentok di langkah mana? sawer di <b>Saweria</b> (link di bio), lalu <b>DM aku</b>. aku bantu sampai jalan." },
  endSfx: { file: "ting.wav", at: 0.1, dur: 0.45, vol: 0.4 },
});
console.log("part 3:", total, "s");
