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
| log | catatan | "log dari Bind"; also Kafka's log, explained at first use as a list of messages that only grows at its end. Where both meanings could meet, write "catatan yang ditulis program" for the first |
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
| live stream | siaran langsung (as the term) | YouTube's feature; explained at first use. In running text the broadcast itself is "siaran" |
| live chat | obrolan langsung | the comment column beside a live stream |
| Super Chat | | YouTube's paid comment |
| stream key | kunci stream | the secret code that sends a broadcast to a channel |
| channel | kanal | a YouTube channel, always "channel YouTube" or clear from context; also the Go meaning ("channel Go") and audio channels ("channel suara"), so qualify it |
| encoder, decoder | penyandi | "encoder hardware", "encoder software" |
| codec | | explained at first use as a way to compress picture or sound and open it again |
| frame | bingkai | a video frame; "frame suara" for one sample per audio channel |
| fps | | "30 fps, yaitu 30 frame per detik" |
| keyframe | | |
| bitrate | laju bit | "bitrate video" |
| CBR | | "bitrate yang tetap, yang disebut CBR" |
| latency | latensi | the delay before viewers see a moment of the broadcast |
| buffering | | |
| RTMP, RTMPS, FLV, H.264, AAC, PCM | | PCM is explained at first use as "suara mentah" |
| playlist | daftar putar | |
| crossfade | | |
| stdin, stdout | | explained at first use as "input standar sebuah program" and "output standar" |
| preset | | the libx264 option |
| filter | | the ffmpeg meaning, as in "filter drawtext" |
| font, monospace | huruf (for font) | "font monospace" |
| escape | | "di-escape" |
| browser headless | | explained at first use |
| upload | unggah | "di-upload", "meng-upload" |
| Shorts | | YouTube's short videos |
| scene | adegan | OBS's arrangement of pictures, text and sound |
| throttling | | the Raspberry Pi slowing its CPU |
| power supply | catu daya, adaptor | |
| core | inti | a CPU core |
| link, emoji, spammer | tautan | |
| variation selector | | the invisible character that changes how an emoji looks |
| cron | | "baris `@reboot`" |
| message broker | perantara pesan | explained at first use as a server that receives, keeps and delivers messages; "broker" on its own after that |
| producer, consumer | pengirim, penerima (as the term) | the programs that send to and take from a broker; "consumer-nya" with a suffix. "pengirim" and "penerima" stay for the plain idea, before the terms are introduced |
| exchange | | the broker's entry point; the kinds keep Indonesian word order, like "server DNS": "exchange direct", "exchange fanout", "exchange topic", "exchange headers" |
| queue | antrean | the broker's named queue, beside exchange and binding: "queue `email`", "queue-nya". The VPN series keeps "antrean" for the relay's per-client queue |
| binding | ikatan | the rule from an exchange to a queue; "binding-nya". "terikat dengan" is fine for the verb |
| routing key | kunci routing | the label a message is sent with, as in `pesanan.dibuat` |
| ack, acknowledgement | | "di-ack", "ack-nya"; "ack otomatis", "ack manual" |
| at-most-once, at-least-once, exactly-once | | delivery guarantees; explained at first use |
| idempotent | | a consumer that can process the same message twice safely |
| publisher confirm | | the broker's ack to the producer |
| durable, persistent | | a queue kept on disk, and a message written to disk: "queue durable", "pesan persistent" |
| restart | | "di-restart", explained at first use as switched off and on again |
| disk | cakram | |
| cluster | | several servers that each keep a copy |
| prefetch | | how many unacked messages a consumer may hold |
| throughput | | messages processed per second |
| event | peristiwa | Kafka's name for a message |
| topic | | Kafka's log; not the same as "exchange topic" |
| offset | | a consumer's position in a Kafka log |
| partition | partisi | a piece of a Kafka topic |
| consumer group, share group | | Kafka's groups of consumers |
| subject | subjek | NATS's address for a message |
| core NATS, JetStream | | NATS without storage, and its storage layer |
| stream | | RabbitMQ's queue that keeps messages after they are read |
| flash sale | | explained at first use as a big discount for a short time |
| timeout | batas waktu (as the term) | explained at first use as "batas waktu menunggu jawaban" |
| invoice | faktur, tagihan | |
| database | basis data | |
| backup | cadangan | "di-backup" |
| aset game | | an object made to be used in a game (Blender article); "aset" is the Indonesian spelling |
| game engine | mesin game | the program that runs a game, such as Godot; explained at first use |
| low poly | | a model with as few faces as possible |
| face | bidang, sisi | one of the small surfaces a 3D model is made of; explained at first use |
| mesh | jaring | the points, edges and faces of an object; "Mesh" with a capital is also one of the collision shapes |
| origin | titik asal | an object's pivot dot in Blender; "origin-nya" |
| Object Mode, Edit Mode, Sculpt Mode | mode objek, mode edit | Blender's modes, written as the interface names them |
| rigid body | benda kaku | Blender's physics for solid objects |
| Active, Passive | aktif, pasif | the rigid body types, as Blender names them |
| collision shape | bentuk tabrakan | the invisible shape physics uses; "collision shape-nya" |
| collision | | in compounds such as "collision statis" (a game engine's static collision) |
| convex, Convex Hull | cembung | explained at first use as having no dent or hole |
| modifier | pengubah | "Mirror modifier"; explained at first use |
| viewport | | Blender's 3D area |
| 3D cursor | kursor 3D | Blender's red and white ring where new objects appear; "kursor" stays for the mouse pointer |
| apply | menerapkan | "di-apply", as in Blender's Object, Apply |
| bake | memanggang | "di-bake", storing a material as an image |
| node | simpul | Blender's material nodes |
| render | | "di-render", explained at first use |
| UV unwrap | | explained at first use |
| glTF | | the 3D file format |
| Blender's interface names | | Sidebar, Properties, Physics, Modifier, Add Modifier, Randomize, Amount, Random Seed, Clipping, Merge, Apply Modifiers and the like stay exactly as Blender shows them |

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
| pesan | message | a WebSocket message; also a message sent through a broker |
| lonjakan | burst, spike | a sudden rise in orders or messages |
| asinkron, sinkron | asynchronous, synchronous | |
| salinan | copy | "satu salinan pesan di setiap queue" |
| memori | memory | |
| transaksi | transaction | a database transaction; explained at first use |
| tabel, baris | table, row | database tables and rows |
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
| siaran | broadcast, stream | the broadcast itself, in running text; "live stream" names YouTube's feature |
| penonton | viewer | |
| sapaan, menyapa | greeting, greet | the terminal's answer to a comment |
| komentar | comment | a live chat message |
| hening | silence | sound whose samples are all zero |
| sampel | sample | "44.100 sampel per detik" |
| kuota | quota | YouTube Data API's daily units |
| jatah dasar, jatah tambahan | base and extra allowance | the two parts of the chat budget |
| video latar | background video | |
| teks live | live text | the text drawn over the background on every frame |
| kartu SD | SD card | |
| kursor | cursor | |
| pengumpan | feeder | the part that writes sound to the encoder on the clock |
| fisika | physics | "fisika di Blender" |
| gravitasi | gravity | |
| tabrakan, bertabrakan | collision, collide | the plain idea; "collision shape" is the term |
| tekstur | texture | |
| ekspor, diekspor, mengimpor | export, import | |
| simetris | symmetric | |
| cermin, garis cermin | mirror, mirror line | the plain idea; "Mirror modifier" is the term |
| titik | point, vertex | the points of a mesh, in running text |
| kubus, bola, tiang, batu, tanah | cube, sphere, pillar, stone, ground | object names in running text; Blender's own object names (Cube, Plane) stay as shown |
