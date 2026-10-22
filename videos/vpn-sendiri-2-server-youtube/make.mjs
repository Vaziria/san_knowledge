// Builds the YouTube (16:9, ~3 min, captions only) version of "Bikin VPN sendiri" part 2: the WireGuard server on a
// VPS in 6 steps. Run: node make.mjs. Scene kit: ../vpn-sendiri-shared/wide.mjs.
import { buildWide } from "../vpn-sendiri-shared/wide.mjs";
import { vpsMap } from "../vpn-sendiri-shared/diagrams.mjs";

const m = (s) => `<span class="m">${s}</span>`;

const total = buildWide({
  series: "bikin vpn sendiri · bagian 2",
  part: "02 / 03",
  scenes: [
    { type: "title", reg: "orange", dur: 7, slam: true,
      lines: [{ text: "bikin server", size: 190 }, { text: "VPN sendiri", size: 190 }],
      lead: "6 langkah di VPS, dari nol sampai nyala.", leadAt: 1.3 },
    { type: "split", dur: 12, kicker: "hari ini", title: "si titik tengah", visual: vpsMap("s02", { at: 0.4, arrowsAt: 1.2, hot: "vps" }),
      notes: [
        { at: 1.0, html: `VPS dengan IP publik. di VPN, alamatnya ${m("10.8.0.1")}` },
        { at: 5.5, html: "PC rumah dan kantor disambungkan di bagian 3" }] },
    { type: "split", dur: 17, kicker: "langkah 1 / 6", title: "masuk ke VPS",
      cards: [{ head: "terminal · laptop", lines: [{ cmd: "ssh user@IP_VPS", at: 1.6 }] }],
      notes: [
        { at: 1.0, html: "dari laptop, buka terminal atau PowerShell" },
        { at: 4.5, html: `${m("IP_VPS")} = IP publik dari penyedia VPS` },
        { at: 8.5, html: `${m("user")} = akun yang punya akses sudo` },
        { at: 12.5, html: "langkah berikutnya diketik di VPS" }] },
    { type: "split", dur: 16, kicker: "langkah 2 / 6", title: "install WireGuard",
      cards: [{ head: "terminal · VPS", lines: [{ cmd: "sudo apt update", at: 1.5 }, { cmd: "sudo apt install wireguard", at: 3.6 }] }],
      notes: [
        { at: 1.0, html: "VPS Ubuntu atau Debian" },
        { at: 6.0, html: `${m("apt update")} menyegarkan daftar paket` },
        { at: 10.5, html: `paketnya sudah termasuk ${m("wg")} dan ${m("wg-quick")}` }] },
    { type: "split", dur: 22, kicker: "langkah 3 / 6", title: "bikin sepasang kunci",
      cards: [{ head: "terminal · VPS", lines: [
        { cmd: "umask 077", at: 1.5 },
        { cmd: "wg genkey | tee server.key \\", at: 3.4 },
        { cont: "| wg pubkey > server.pub", at: 4.6 },
        { cmd: "cat server.pub", at: 10.0 },
        { out: "(kunci publik server, 44 karakter)", at: 11.0 }] }],
      notes: [
        { at: 2.2, line: 0, html: `${m("umask 077")}: file kunci cuma bisa dibaca kamu` },
        { at: 6.4, html: `${m("server.key")} kunci rahasia, jangan pernah dibagi` },
        { at: 11.6, line: 4, html: `${m("server.pub")} kunci publik, nanti dipakai di PC` },
        { at: 17.0, html: "dua-duanya tersimpan di VPS" }] },
    { type: "split", dur: 30, kicker: "langkah 4 / 6", title: "tulis config-nya",
      cards: [
        { head: "terminal · VPS", lines: [{ cmd: "sudo nano /etc/wireguard/wg0.conf", at: 1.4 }] },
        { head: "/etc/wireguard/wg0.conf", lines: [
          { conf: "[Interface]", at: 3.4 },
          { conf: "Address = 10.8.0.1/24", at: 3.55 },
          { conf: "ListenPort = 51820", at: 3.7 },
          { conf: "PrivateKey = <isi server.key>", at: 3.85 },
          { conf: "PostUp = sysctl -w net.ipv4.ip_forward=1", at: 4.0 }] }],
      notes: [
        { at: 1.0, html: "buka file config dengan nano" },
        { at: 5.0, card: 1, line: 1, html: `${m("Address")}: alamat server di VPN` },
        { at: 9.5, card: 1, line: 2, html: `${m("ListenPort")}: port yang nanti dibuka` },
        { at: 14.0, card: 1, line: 3, html: `${m("PrivateKey")}: isi dengan server.key` },
        { at: 18.5, card: 1, line: 4, html: `${m("PostUp")}: biar rumah ↔ kantor bisa lewat VPS` },
        { at: 24.0, html: `simpan ${m("Ctrl+O")}, keluar ${m("Ctrl+X")}` }] },
    { type: "split", dur: 19, kicker: "langkah 5 / 6", title: "buka port-nya",
      cards: [{ head: "terminal · VPS", lines: [
        { cmd: "sudo ufw allow 51820/udp", at: 1.5 },
        { cmd: "sudo ufw route allow \\", at: 4.6 },
        { cont: "in on wg0 out on wg0", at: 5.5 }] }],
      notes: [
        { at: 1.0, line: 0, html: `${m("ufw")}: firewall bawaan Ubuntu` },
        { at: 6.8, line: 2, html: "aturan kedua: izinkan rumah ↔ kantor lewat VPS" },
        { at: 11.0, html: `buka juga ${m("UDP 51820")} di firewall panel penyedia VPS` },
        { at: 15.0, html: "untuk WireGuard, cuma port ini yang perlu dibuka" }] },
    { type: "split", dur: 18, kicker: "langkah 6 / 6", title: "nyalakan & cek",
      cards: [{ head: "terminal · VPS", lines: [
        { cmd: "sudo systemctl enable --now wg-quick@wg0", at: 1.5 },
        { cmd: "sudo wg", at: 5.4 },
        { out: "interface: wg0", at: 6.3 },
        { out: "  public key: (isi server.pub)", at: 6.5 },
        { out: "  private key: (hidden)", at: 6.7 },
        { out: "  listening port: 51820", at: 6.9, check: true }] }],
      notes: [
        { at: 1.0, line: 0, html: `${m("enable --now")}: nyala sekarang, dan tiap VPS restart` },
        { at: 8.0, line: 5, html: `${m("listening port: 51820")} artinya server siap` },
        { at: 13.0, line: 3, html: "public key di sini sama dengan server.pub" }] },
    { type: "bullets", dur: 16, kicker: "kalau gagal", title: "cek tiga hal ini",
      items: [
        { at: 1.5, text: "isi PrivateKey: kunci rahasia, utuh tanpa spasi" },
        { at: 5.5, text: "UDP 51820 sudah dibuka di panel VPS" },
        { at: 9.5, text: "nama file persis /etc/wireguard/wg0.conf" }] },
    { type: "title", reg: "orange", dur: 9,
      lines: [{ text: "server", size: 240 }, { text: "siap.", size: 240 }],
      lead: "lanjut bagian 3: sambungkan PC rumah dan kantor, lalu Remote Desktop.", leadAt: 1.3 },
  ],
});
console.log("part 2 (YouTube):", total, "s");
