# Foreign terms wordlist

Which words stay English and which stay Indonesian in the articles. The rule is in [guideline.md](guideline.md), under "Language and tone": technical terms keep their original form, untranslated, while everyday words stay Indonesian.

- **Before handing over an article:** check every line of it against this list, not only the lines that changed.
- **A term not listed yet:** follow the rule, then add the term here. If it could go either way, ask the user and record their answer.

## Keep in English

Write these as they are, with no Indonesian equivalent next to them.

| Term | Not | Note |
|---|---|---|
| privacy VPN | VPN privasi | the app that hides your IP; not the kind this series builds |
| private network VPN | VPN jaringan privat | joins your own machines into one network; the kind this series builds |
| private IP | IP privat, alamat privat | `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16` |
| tunnel | terowongan, terowongan (tunnel) | |
| payload | muatan | "payload-nya" with a suffix |
| traffic | lalu lintas | |
| header | | |
| encapsulation | enkapsulasi | the wrapping itself can be described as "pembungkusan" |
| port | | |
| port forwarding | membuka port | "melakukan port forwarding" |
| router | | "router-mu" with a suffix |
| switch | | |
| firewall | | |
| LAN | jaringan lokal | |
| WAN | sisi internet | "sisi WAN router-mu" |
| NAT, carrier-grade NAT (CGNAT) | | |
| MTU | | |
| hole punching | | |
| cipher | | |
| network interface | antarmuka jaringan | "interface" on its own after first use |
| network card, virtual network card | kartu jaringan, kartu jaringan virtual | |
| TUN device | perangkat TUN | |
| OS | sistem operasi | |
| server | | |
| client | klien, anggota | a computer in the VPN, or its program, which connects to the relay. "client-nya" with a suffix; "sebuah client", not "seorang" |
| peer | rekan, rekan (peer) | |
| provider | penyedia | "provider VPN" |
| remote access | akses jarak jauh | |
| site to site | antarlokasi | |
| mesh | | |
| overlay | | the virtual network that runs on top of another |
| underlay | | the real network underneath, usually the internet |
| relay | | the server that every client connects to; introduced as "relay, yaitu server yang …" |
| self-hosted | pemasangan mandiri | "setup self-hosted" |
| setup | pemasangan | |
| library | pustaka | code ready to call from your own program |
| executable | | "file executable" |
| runtime | | |
| goroutine | | |
| garbage collector | pengumpul sampah | |
| driver | | |
| root | | the administrator account on Linux |
| handshake | jabat tangan | |
| route | rute | |
| WebSocket | | |
| public key | kunci publik | the key a member shares; introduced in Part 2 as the name for "kunci yang dibagikan" |
| private key | kunci privat | the key a member keeps secret |
| module | modul | a Go project and the libraries it uses |
| interface | antarmuka | the Go meaning: a list of methods a type must have |
| method | metode | |
| type | tipe | the Go meaning |
| test | uji, pengujian | "test", "go test" |
| mutex | | |
| network stack | tumpukan jaringan | |
| netstack | | wireguard-go's network stack inside the program |
| reverse proxy | | |
| hex | heksadesimal | |
| request | permintaan | an HTTP request |
| binary | biner | a WebSocket message type |
| offline | | |
| device | perangkat | the wireguard-go device |
| prefix | awalan | as in `/24` |
| mask | | the Windows form of a prefix, `255.255.255.0` |
| adapter | | the Windows name for a network card, as in "adapter Wintun" |
| netlink | | |
| profil Public | | the Windows firewall profile for untrusted networks |
| shared secret | rahasia bersama | the number both sides compute with Diffie-Hellman |
| digital signature | tanda tangan digital | |
| Diffie-Hellman | | |
| nonce | | a random number used once |
| HMAC | | |
| hash | | "fungsi hash", as in SHA-256 |
| token | | |
| password | kata sandi | |
| hardware | perangkat keras | |
| software | perangkat lunak | |
| data center | pusat data | |
| VPS | | explained at first use as a server virtual rented in a data center |
| keepalive | | TCP's own check on an idle connection |
| ping, pong | | a WebSocket ping and its answer; "ping" is also the command |
| channel | saluran | the Go meaning; explained at first use as "saluran milik Go" |
| proxy | | a program that forwards connections, as in "proxy TCP" |
| network namespace | | the Linux feature; "namespace" on its own after first use |
| log | catatan | "log dari Bind" |
| hotspot | | "hotspot ponsel" |
| base64 | | explained at first use as a way to write any bytes with letters and digits |
| flag | | a command-line flag, as in "flag `-config`" |
| rename | mengganti nama | explained at first use as "mengganti nama file" |
| broadcast | | "alamat broadcast" |
| domain | | as in `vpn.internal`; explained at first use as the end part of a name |
| server DNS | DNS server | Indonesian word order, like "server nama" |
| resolver | | the part that answers name questions |
| split DNS | | only one domain goes to a given DNS server |
| routing-only domain | | systemd-resolved's `~domain` |
| multicast DNS | | what `.local` belongs to |
| checksum | | explained at first use as "angka pemeriksa" |
| field | | the Go meaning, as in "field `Domain`" |
| container | | explained at first use as a package holding a program and all it needs |
| preview | | a service still in preview |
| file unit | | a systemd unit file; explained at first use |
| installer | | explained at first use as a program that puts files in place |
| shortcut | | "shortcut di folder Startup" |
| login | | "login Microsoft", "layar login" |
| open source | | |
| repository | | "repository apt" |
| API | | explained at first use as the list of functions a library offers |

## Keep in Indonesian

Everyday words that Indonesian readers use, including Indonesian spellings of borrowed words.

| Word | English | Note |
|---|---|---|
| paket | packet | |
| paket dalam | inner packet | |
| jaringan | network | |
| jaringan virtual | virtual network | |
| alamat | address | |
| koneksi | connection | "koneksi masuk", "koneksi keluar" |
| pesan | message | a WebSocket message |
| fungsi | function | "fungsi penerima" |
| konfigurasi | configuration | |
| sertifikat | certificate | "sertifikat HTTPS" |
| IP publik | public IP | not "alamat publik" |
| kunci, sepasang kunci | key, key pair | |
| data | data | |
| enkripsi, mengenkripsi | encryption, encrypt | |
| dekripsi, mendekripsi | decryption, decrypt | |
| autentikasi | authentication | |
| integritas | integrity | |
| kriptografi | cryptography | |
| protokol | protocol | |
| rentang | range | as in "rentang `10.0.0.0/8`" |
| antrean | queue | the relay's queue for one member |
| sinyal | signal | an OS signal, as in "sinyal `SIGHUP`" |
| konstanta | constant | the Go meaning |
| jeda | delay, backoff | "jeda yang makin panjang", not "backoff" |
| variabel lingkungan | environment variable | as in "variabel lingkungan `PORT`" |
| akses anonim | anonymous access | |
| layanan | service | an OS service, as in "menjalankan client sebagai layanan" |
| pengelola layanan | service manager | systemd, Task Scheduler |
| kelemahan, hal yang dikorbankan | cost, price, trade-off, limitation | a design's cost; not "harga", "harganya" or "batasan", which are calques. "harga" and "biaya" only for money. See [dialectical_writing.md](dialectical_writing.md) |
