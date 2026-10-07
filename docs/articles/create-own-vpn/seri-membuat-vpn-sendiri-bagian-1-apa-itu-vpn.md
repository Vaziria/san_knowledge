# [Seri Membuat VPN Sendiri Bagian 1] Apa itu VPN (Virtual Private Network) ?

*Sebelum membuatnya, kita pahami dulu apa yang sebenarnya dilakukan VPN. Termasuk kenapa setiap paketnya adalah paket di dalam paket, dan satu masalah yang membuat VPN buatan sendiri lebih sulit daripada kelihatannya.*

Dalam seri ini, kita membangun VPN dari nol. VPN ini adalah alat kecil yang menggabungkan komputer di rumah, di kantor, dan di sebuah VPS ke dalam satu jaringan privat. VPS adalah server virtual yang disewa di data center. Sebelum masuk ke kode, bagian ini menjelaskan apa itu VPN dan bagaimana cara kerjanya.

## Dua hal yang sama-sama disebut "VPN"

Kata VPN dipakai untuk dua produk berbeda yang berbagi satu mekanisme. Kedua produk itu adalah:

- **Privacy VPN** adalah aplikasi yang kamu pasang untuk menyembunyikan alamat IP-mu, yaitu alamat yang dipakai perangkatmu di internet. Aplikasi ini juga bisa membuatmu terlihat berada di negara lain. Perangkatmu mengirim semua traffic, yaitu semua data yang keluar dan masuk, ke server milik provider VPN, lalu server itu meneruskannya ke internet. Kamu meminjam jalan keluar ke internet milik orang lain.
- **Private network VPN** adalah arti aslinya. VPN ini menggabungkan komputer-komputer milikmu di tempat yang berbeda ke dalam satu jaringan, seolah semuanya tercolok ke switch yang sama. Switch adalah alat yang menghubungkan komputer-komputer di satu ruangan. Laptop di rumah bisa menjangkau PC kantor di alamat seperti `10.10.0.1`, dan tidak ada orang lain di internet yang bisa.

Seri ini membahas jenis kedua. Di baliknya, keduanya bekerja dengan cara yang sama. Keduanya memakai tunnel, yaitu jalur tertutup yang membawa data dari satu komputer ke komputer lain melewati internet. Cara tunnel bekerja dibahas di bawah.

## Apa masalah yang diselesaikan oleh VPN?

Untuk memahami masalahnya, kita mulai dari cara data berjalan di jaringan. Data dikirim dalam potongan kecil yang disebut paket. Setiap paket terdiri dari header, yaitu bagian depan yang berisi alamat pengirim dan alamat tujuan, dan payload, yaitu isi yang dibawanya. Router adalah perangkat yang menyambungkan satu jaringan dengan jaringan lain. Router membaca alamat tujuan di header, lalu meneruskan paket sampai tiba di tujuannya.

Meski begitu, komputer di jaringan yang berbeda tidak bisa begitu saja saling terhubung. Alasannya adalah:

- **Private IP tidak bisa menyeberang.** PC di rumahmu kemungkinan punya alamat seperti `192.168.1.20`. Rentang `10.0.0.0/8`, `172.16.0.0/12`, dan `192.168.0.0/16` dicadangkan untuk LAN, yaitu jaringan di dalam satu rumah atau kantor. Router di internet tidak meneruskan paket ke alamat-alamat itu.
- **NAT menyembunyikan komputer.** NAT (Network Address Translation) membuat router di rumah bisa membagi satu IP publik, yaitu alamat yang terlihat dari internet, untuk semua perangkat. Balasan untuk koneksi yang kamu mulai bisa kembali. Koneksi yang dimulai dari luar tidak tahu harus diteruskan ke perangkat mana.
- **Firewall memblokir sisanya.** Kantor biasanya memasang firewall, yaitu penyaring yang mengizinkan koneksi keluar dan menolak koneksi masuk.

VPN memberi setiap komputer alamat kedua di sebuah jaringan virtual bersama. Jaringan ini disebut virtual karena tidak punya kabel sendiri dan berjalan di atas internet. Alamat itu berlaku sama di mana pun komputernya berada. Seperti di rumah, di LAN kantor, atau di Wi-Fi hotel.

![Dua panel. Tanpa VPN: PC rumah (192.168.1.20) di belakang router NAT, PC kantor (192.168.0.15) di belakang firewall, dan internet di antaranya. Koneksi masuk ke rumah berhenti di NAT, koneksi masuk ke kantor diblokir firewall, dan paket ke private IP dibuang di internet. Dengan VPN: kedua PC mendapat alamat kedua, 10.10.0.2 dan 10.10.0.1, yang terhubung lewat tunnel di jaringan virtual 10.10.0.0/24.](assets/networks-without-and-with-vpn.png)

*Tanpa VPN, koneksi masuk berhenti di jalan. Dengan VPN, kedua PC punya alamat yang bisa saling dijangkau.*

## Anatomi VPN dan peran tunnel di dalamnya

Di bagian sebelumnya, kita melihat VPN memberi setiap komputer alamat kedua di sebuah jaringan virtual. Untuk memahami bagaimana VPN melakukannya, kita bedah dulu bagian-bagiannya. Di setiap komputer anggota VPN, ada empat bagian yang bekerja bersama. Keempatnya adalah:

- **Aplikasi** adalah program yang kamu pakai sehari-hari. Seperti browser, remote desktop, atau database. Aplikasi tidak tahu ada VPN. Ia hanya mengirim paket ke sebuah alamat, misalnya `10.10.0.1`.
- **Virtual network card** adalah pintu masuk ke jaringan virtual. Setiap komputer terhubung ke jaringan lewat network card, yaitu hardware untuk kabel LAN atau Wi-Fi. VPN menambahkan satu network card lagi yang hanya berupa software, lengkap dengan alamat VPN-nya, misalnya `10.10.0.2`. OS, yaitu software dasar seperti Windows atau Linux, melihatnya sebagai network interface biasa. Network interface adalah sebutan OS untuk sebuah network card. Di Linux, interface buatan VPN ini disebut TUN device, dan di Windows, Wintun melakukan tugas yang sama.
- **Program VPN** bekerja di balik virtual network card. Program ini membaca paket yang masuk ke virtual network card, membungkusnya, lalu mengirimnya ke komputer tujuan. Paket yang datang dari komputer lain dibuka bungkusnya, lalu diserahkan kembali ke virtual network card.
- **Kunci** dipegang oleh setiap komputer. Kunci membuktikan identitas komputer itu dan dipakai untuk mengenkripsi paket. Perannya dibahas lebih lanjut di bagian tentang apa yang membuat VPN privat.

Bagian-bagian itu baru menjadi jaringan setelah dihubungkan oleh tunnel. Tunnel adalah jalur antara program VPN di satu komputer dan program VPN di komputer lain. Bagi aplikasi, tunnel bekerja seperti kabel yang langsung menyambungkan dua virtual network card. Paket yang masuk di satu ujung keluar di ujung lain. Padahal, paket itu sebenarnya menempuh jalan biasa, lewat network card asli, router, dan internet.

Kumpulan tunnel inilah yang membentuk jaringan virtual. Karena berjalan di atas jaringan lain, jaringan virtual disebut overlay. Jaringan asli di bawahnya, yaitu internet, disebut underlay. Aplikasi hanya melihat overlay, sedangkan program VPN yang mengurus perjalanan di underlay.

![Dua kolom, komputer A dan komputer B. Di setiap komputer ada empat bagian bertumpuk, yaitu aplikasi, virtual network card (10.10.0.2 dan 10.10.0.1), program VPN yang memegang kunci, dan network card asli (192.168.1.20 dan 192.168.0.15). Garis putus-putus biru menghubungkan kedua virtual network card sebagai jaringan virtual 10.10.0.0/24 yang dilihat aplikasi. Pipa biru menghubungkan kedua program VPN sebagai tunnel terenkripsi, yang dibawa oleh internet. Garis abu-abu menunjukkan jalan asli paket dari network card ke internet.](assets/vpn-anatomy.png)

*Aplikasi melihat kabel langsung. Paketnya sebenarnya lewat tunnel di atas internet.*

## Cara kerja VPN adalah "paket di dalam paket"

Sekarang kita ikuti satu paket melewati anatomi tadi. Misalnya aplikasi di komputer A (`10.10.0.2`) mengirim data ke komputer B (`10.10.0.1`). Langkah-langkahnya adalah:

1. Aplikasi mengirim paket ke `10.10.0.1`. OS melihat alamat itu milik jaringan virtual, sehingga paketnya diarahkan ke virtual network card.
2. Program VPN membaca paket itu dari virtual network card, lalu mengenkripsinya, yaitu mengacak isinya dengan kunci yang hanya diketahui A dan B. Tanpa kunci itu, isinya tidak bisa dibaca.
3. Program VPN membungkus paket terenkripsi itu menjadi payload dari paket UDP biasa, yang ditujukan ke IP publik komputer B. UDP adalah cara paling sederhana untuk mengirim paket di internet.
4. Paket bungkusan itu keluar lewat network card asli dan menempuh internet seperti paket lainnya.
5. Program VPN di komputer B membuka bungkusnya, mendekripsinya, lalu menyerahkan paket aslinya ke virtual network card B.
6. OS di komputer B mengantarkan paket itu ke aplikasi, seolah paket itu datang langsung dari `10.10.0.2`.

Gambar di bawah memakai WireGuard sebagai contoh. WireGuard adalah salah satu protokol VPN yang dibahas di bagian berikutnya.

![Dua baris. Di atas, paket yang dikirim aplikasi di dalam tunnel: header IP dari 10.10.0.2 ke 10.10.0.1, lalu data aplikasi. Di bawah, paket yang dilihat jaringan: header IP di antara dua IP publik, header UDP, header WireGuard, dan seluruh paket dalam, terenkripsi, sebagai payload-nya.](assets/vpn-packet-inside-a-packet.png)

*Apa yang dikirim aplikasi, dan apa yang dilihat jaringan.*

Pembungkusan ini disebut encapsulation, dan ada harganya. Setiap paket membawa satu set header tambahan, sehingga ruang untuk data berkurang. WireGuard menambah 60 byte di IPv4 dan 80 byte di IPv6, dua versi alamat IP yang dipakai di internet. Karena itu, MTU default WireGuard, yaitu ukuran paket terbesar di tunnel, adalah 1420, bukan 1500 seperti biasanya. Angka itu adalah 1500 dikurangi 80 byte untuk kasus IPv6.

## Apa yang membuat VPN privat

Di bagian sebelumnya, kita melihat tunnel membungkus paket di dalam paket lain supaya bisa menyeberangi internet. Tetapi pembungkusan saja belum membuat jaringan itu privat. Paket yang hanya dibungkus masih bisa dibaca, dipalsukan, atau diubah oleh siapa pun yang dilewatinya di internet. Karena itu, VPN memberi tunnel tiga sifat yang menjadikannya jaringan privat. Ketiga sifat itu adalah:

- **Enkripsi mencegah paket dibaca.** Inilah langkah mengenkripsi yang terjadi sebelum paket dibungkus. Siapa pun di sepanjang jalur hanya melihat data acak di antara dua IP publik. Seperti ISP-mu, yaitu perusahaan yang menjual koneksi internet kepadamu, atau pemilik router di kafe.
- **Autentikasi mencegah paket dipalsukan.** Setiap komputer memegang sepasang kunci. Satu kunci dirahasiakan, dan satu lagi dibagikan ke anggota lain untuk mengenalinya. Komputer tanpa kunci yang diterima anggota lain tidak bisa bergabung, dan tidak bisa menyusupkan paket.
- **Integritas mencegah paket diubah.** Paket yang diubah di tengah jalan gagal diperiksa, lalu dibuang.

## Tiga protokol yang akan kamu temui di dalam VPN

Protokol VPN adalah aturan tentang cara dua komputer membuat tunnel dan bertukar paket di dalamnya. Ada tiga protokol yang akan sering kamu temui. Mereka sama-sama membuat tunnel, tetapi berbeda dalam usia, ukuran, dan cara memakainya. Ketiganya adalah:

- **IPsec** sudah menjadi standar sejak tahun 1990-an. Protokol ini tertanam di kebanyakan router dan OS, dan umum dipakai untuk menghubungkan jaringan antarkantor. Konfigurasinya juga terkenal rumit.
- **OpenVPN** muncul pada 2001. OpenVPN dibangun di atas TLS, protokol yang juga mengamankan HTTPS, dan berjalan sebagai program biasa. OpenVPN bisa memakai UDP maupun TCP, yaitu cara mengirim yang menjamin data sampai lengkap dan berurutan. Fleksibilitasnya menjadikannya pilihan utama banyak setup self-hosted, yaitu VPN yang dijalankan sendiri di server milik penggunanya.
- **WireGuard** adalah yang terbaru. Ukurannya kecil. Implementasi Linux aslinya sekitar 4.000 baris kode, dan sudah menjadi bagian dari kernel Linux, yaitu inti dari OS Linux, sejak 2020. WireGuard memakai satu set kriptografi modern yang tetap (Curve25519, ChaCha20-Poly1305, dan BLAKE2s), jadi tidak ada pilihan cipher, yaitu algoritma enkripsi, yang bisa salah diatur. WireGuard hanya berjalan di atas UDP.

Seri ini dibangun di atas WireGuard, karena ukurannya kecil dan tidak ada pilihan cipher yang bisa salah diatur. Harganya, WireGuard hanya berjalan di atas UDP. Batasan ini kita jawab di bagian akhir artikel ini.

## Tiga bentuk jaringan VPN

Tunnel yang sama bisa disusun dalam tiga bentuk, tergantung siapa terhubung ke siapa. Ketiganya adalah:

- **Remote access.** Satu laptop terhubung ke jaringan kantor. Kantor menjalankan server VPN, dan setiap laptop menjadi client-nya, yaitu pihak yang menghubungi server.
- **Site to site.** Dua kantor menghubungkan seluruh jaringannya lewat router masing-masing.
- **Mesh.** Setiap komputer menjadi peer bagi semua komputer lain. Artinya, setiap komputer setara dan bisa langsung terhubung ke komputer mana pun, tanpa server pusat yang memegang kunci. Bentuk inilah yang akan kita bangun, karena komputer di rumah, di kantor, dan di VPS dalam seri ini sama-sama perlu saling menjangkau.

## Masalah saat membuat VPN sendiri adalah "harus ada yang bisa dijangkau"

Di gambar paket tadi, paket luar dikirim ke IP publik komputer tujuan. Artinya, setiap tunnel dimulai dengan satu komputer mengirim paket ke IP publik komputer lain, dan komputer itu harus bisa dijangkau dari internet. Server VPN klasik butuh IP publik dan port yang terbuka. Port adalah nomor yang menentukan program mana di sebuah komputer yang menerima koneksi. Di situlah membuat VPN sendiri menjadi sulit.

Banyak koneksi rumah sekarang tidak punya IP publik sama sekali. Dengan carrier-grade NAT (CGNAT), ISP memasang lapisan NAT kedua di depan router-mu dan membagi satu IP publik untuk banyak pelanggan. ISP di Indonesia memakainya. Sisi WAN router-mu, yaitu sisi yang menghadap ke internet, mendapat alamat dari rentang bersama `100.64.0.0/10`, bukan IP publik. Kamu juga tidak bisa melakukan port forwarding, yaitu mengatur router agar meneruskan koneksi masuk ke satu komputer, di router yang bukan milikmu.

Ada tiga cara yang umum untuk mengatasinya:

- **Sewa server dengan IP publik**, lalu hubungkan semuanya ke server itu. Cara ini berhasil, tetapi setiap paket melewati server tersebut, dan kamu harus membayarnya.
- **Hole punching.** Kedua komputer mengetahui IP publik dan port satu sama lain dari server ketiga, lalu saling mengirim paket pada saat yang sama, sehingga masing-masing NAT mengira paket dari pihak lain adalah balasan. Cara ini sering berhasil, tetapi sebagian CGNAT menggagalkannya.
- **Relay.** Kedua komputer terhubung keluar ke titik ketiga, yang meneruskan paket di antara keduanya. Cara ini berjalan di mana pun koneksi keluar diizinkan. Harganya, setiap paket melewati relay, sehingga perjalanannya lebih jauh dan kecepatannya dibatasi oleh relay itu.

Seri ini memilih relay, karena hanya cara ini yang tetap berjalan di balik CGNAT dan firewall kantor, selama koneksi HTTPS keluar diizinkan.

## Apa yang dibutuhkan untuk membuat VPN sendiri

Sampai di sini, kita sudah tahu anatomi VPN, cara kerjanya, dan masalah yang harus dihadapi. Sekarang kita tentukan dengan bahasa apa VPN ini ditulis, dan komponen apa saja yang dibutuhkan.

### Bahasa pemrograman untuk VPN ini adalah "Go"

VPN di seri ini ditulis dengan Go, bahasa pemrograman buatan Google. Alasannya adalah:

- **WireGuard sudah tersedia dalam Go.** wireguard-go adalah implementasi resmi WireGuard yang berjalan sebagai program biasa, di luar kernel. Kita bisa memakainya sebagai library, yaitu kode siap pakai yang dipanggil dari program kita. Dengan begitu, kita tidak perlu menulis kriptografi sendiri.
- **Satu file untuk setiap OS.** Go bisa membuat program untuk Windows dan Linux dari satu komputer yang sama. Hasilnya satu file executable yang langsung bisa dijalankan, tanpa runtime yang harus dipasang lebih dulu. Runtime adalah program pendukung yang harus terpasang supaya sebuah bahasa bisa berjalan.
- **Mudah mengerjakan banyak hal bersamaan.** Go punya goroutine, yaitu cara ringan untuk menjalankan banyak pekerjaan sekaligus. VPN membutuhkannya, karena VPN membaca virtual network card, mengirim ke jaringan, dan melayani banyak anggota pada saat yang sama.

Tapi kenapa tidak memakai bahasa lain? Ada tiga pilihan lain yang masuk akal, dan masing-masing punya harganya. Ketiganya adalah:

- **C** dipakai oleh WireGuard di dalam kernel Linux, sehingga paling cepat. Harganya, kesalahan kecil dalam mengelola memori bisa menjadi celah keamanan, dan kode di dalam kernel lebih sulit dibuat dan diuji.
- **Rust** juga cepat, dan melindungi program dari kesalahan memori. Cloudflare punya implementasi WireGuard dalam Rust bernama boringtun. Harganya, Rust lebih sulit dipelajari.
- **Python atau JavaScript** paling mudah ditulis. Harganya, keduanya lebih lambat untuk memproses paket satu per satu, dan butuh runtime yang terpasang di setiap komputer.

Go juga punya harganya. Program Go berjalan di luar kernel dan memakai garbage collector, yaitu pembersih memori otomatis, sehingga lebih lambat daripada WireGuard di dalam kernel. Jadi, Go cocok jika yang dihubungkan adalah beberapa komputer milik sendiri. Go kurang cocok jika VPN harus melayani ribuan pengguna dengan kecepatan setinggi mungkin.

### Komponen yang dibutuhkan untuk membuat VPN sendiri

Setiap bagian di anatomi VPN tadi butuh komponen yang nyata, ditambah beberapa komponen untuk menjawab masalah CGNAT. Komponen-komponen itu adalah:

- **Virtual network card.** Di Linux, kita memakai TUN device bawaan kernel. Di Windows, kita memakai Wintun, yaitu driver TUN dari proyek WireGuard. Driver adalah software yang mengajari OS cara memakai sebuah perangkat. Keduanya hanya bisa dibuat dengan hak administrator, jadi program VPN harus dijalankan sebagai administrator di Windows atau sebagai root, yaitu akun administrator, di Linux.
- **Program VPN.** wireguard-go mengurus enkripsi, handshake, dan pergantian kunci. Handshake adalah saat dua komputer saling mengenali dan menyepakati kunci di awal koneksi. Bagian yang kita tulis sendiri adalah penghubung antara wireguard-go, virtual network card, dan jalur pengiriman.
- **Kunci.** Setiap komputer membuat sepasang kuncinya sendiri saat bergabung. Kunci yang dirahasiakan tidak pernah meninggalkan komputer itu. Hanya kunci yang dibagikan yang dikirim ke anggota lain.
- **Jalur pengiriman.** WireGuard biasanya mengirim paket lewat UDP. Masalahnya, di balik CGNAT tidak ada komputer yang bisa dijangkau, dan firewall kantor sering hanya mengizinkan HTTPS keluar. Jadi kita mengganti jalur UDP itu dengan WebSocket, yaitu koneksi web yang tetap terbuka, di atas HTTPS. wireguard-go memungkinkan bagian pengirim paketnya diganti tanpa mengubah WireGuard itu sendiri.
- **Relay.** Semua anggota terhubung keluar ke relay, yang meneruskan paket di antara mereka. Relay hanya meneruskan paket yang sudah terenkripsi, sehingga ia tidak bisa membaca isinya. Relay butuh alamat HTTPS yang bisa diakses publik. Seperti sebuah VPS dengan IP publik.
- **Daftar anggota.** Harus ada yang mencatat siapa saja anggotanya, kunci yang dibagikan oleh masing-masing, dan alamat VPN mereka. Catatan ini dipegang relay dan dikirim ke setiap anggota, supaya program VPN tahu ke mana harus mengirim setiap paket.
- **Pengaturan OS.** Setelah virtual network card dibuat, OS harus diberi tahu alamatnya dan route ke jaringan virtual. Route adalah aturan yang memberi tahu OS lewat network card mana sebuah paket harus keluar. Di Windows, firewall juga perlu diatur supaya paket dari anggota lain boleh masuk.

### Yang perlu kamu siapkan untuk mengikuti seri ini

Untuk mengikuti seri ini dan mencoba VPN-nya sendiri, ada empat hal yang perlu kamu siapkan. Keempatnya adalah:

- Go yang sudah terpasang, untuk menulis dan membangun programnya.
- Minimal dua komputer dengan Windows atau Linux, untuk dihubungkan.
- Hak administrator atau root di komputer-komputer itu.
- Satu server yang bisa diakses lewat HTTPS, untuk menjalankan relay.

## Berikutnya dalam seri ini

Di Bagian 2, kita mulai menulis kodenya. Kita mulai dari jalur pengiriman, yaitu cara membawa WireGuard lewat WebSocket melalui relay. Dengan begitu, VPN ini bisa berjalan di mana pun koneksi HTTPS keluar bisa berjalan, termasuk di balik CGNAT.
