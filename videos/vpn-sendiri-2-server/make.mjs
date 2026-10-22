// Builds part 2 of "Bikin VPN sendiri": the WireGuard server on a VPS in 5 steps, as code cards.
// Run: node make.mjs (writes index.html and compositions/). The cards and their timing are ../vpn-sendiri-shared/cards.mjs.
import { buildVideo } from "../vpn-sendiri-shared/cards.mjs";

const m = (s) => `<span class="m">${s}</span>`;

const total = buildVideo({
  series: "bikin vpn sendiri",
  part: "02 / 03",
  hook: { dur: 2.4, kicker: "bagian 2 · 5 langkah", slam: true,
    lines: [{ text: "bikin server", size: 132 }, { text: "VPN sendiri", size: 132 }] },
  steps: [
    { kicker: "langkah 1 / 5", title: "install WireGuard", dur: 4.6,
      cards: [{ head: "terminal · VPS", lines: [{ cmd: "sudo apt install wireguard", at: 0.9 }] }],
      marks: [{ at: 2.2, cap: "di VPS Ubuntu atau Debian" }] },
    { kicker: "langkah 2 / 5", title: "bikin kunci", dur: 6.6,
      cards: [{ head: "terminal · VPS", lines: [
        { cmd: "wg genkey | tee server.key \\", at: 0.9 },
        { cont: "| wg pubkey > server.pub", at: 1.95 }] }],
      marks: [
        { at: 3.0, cap: `${m("server.key")} kunci rahasia, jangan dibagi` },
        { at: 4.6, cap: `${m("server.pub")} kunci publik, buat PC lain` }] },
    { kicker: "langkah 3 / 5", title: "tulis config-nya", dur: 7.6,
      cards: [{ head: "/etc/wireguard/wg0.conf", lines: [
        { conf: "[Interface]", at: 0.7 },
        { conf: "Address = 10.8.0.1/24", at: 0.85 },
        { conf: "ListenPort = 51820", at: 1.0 },
        { conf: "PrivateKey = <isi server.key>", at: 1.15 },
        { conf: "PostUp = sysctl -w net.ipv4.ip_forward=1", at: 1.3 }] }],
      marks: [
        { at: 1.9, line: 1, cap: "alamat server di jaringan VPN" },
        { at: 3.3, line: 2, cap: "port ini yang nanti dibuka" },
        { at: 4.7, line: 3, cap: "isi dengan kunci rahasia tadi" },
        { at: 6.1, line: 4, cap: "biar rumah ↔ kantor bisa lewat VPS" }] },
    { kicker: "langkah 4 / 5", title: "buka port-nya", dur: 6.0,
      cards: [{ head: "terminal · VPS", lines: [
        { cmd: "sudo ufw allow 51820/udp", at: 0.9 },
        { cmd: "sudo ufw route allow \\", at: 2.1 },
        { cont: "in on wg0 out on wg0", at: 2.95 }] }],
      marks: [{ at: 3.9, cap: `buka juga ${m("UDP 51820")} di firewall panel VPS-mu` }] },
    { kicker: "langkah 5 / 5", title: "nyalakan", dur: 6.4,
      cards: [{ head: "terminal · VPS", lines: [
        { cmd: "sudo systemctl enable --now wg-quick@wg0", at: 0.9 },
        { cmd: "sudo wg", at: 2.7 },
        { out: "interface: wg0", at: 3.4 },
        { out: "  listening port: 51820", at: 3.55, check: true }] }],
      marks: [{ at: 4.2, cap: "server siap, dan nyala lagi sendiri tiap VPS restart" }] },
  ],
  end: { dur: 3.4, kicker: "lanjut bagian 3",
    lines: [{ text: "sambungkan", size: 130, light: true }, { text: "PC rumah", size: 130 }, { text: "& kantor", size: 130 }] },
});
console.log("part 2:", total, "s");
