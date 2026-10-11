# [Seri Membuat VPN Sendiri Bagian 8] Menjalankan relay VPN tanpa server sendiri

*Di bagian ini, kita melihat dua tempat lain untuk relay selain VPS, yaitu layanan tunnel dan layanan container, beserta biaya dan kelemahannya masing-masing. Lalu kita mengajari VPN membuka koneksi baru sebelum koneksi lamanya diputus.*

Sejak Bagian 3, relay berjalan di sebuah VPS yang punya IP publik, domain, dan Caddy untuk HTTPS. Itu cara yang paling sederhana dan paling murah. Tetapi tidak semua orang punya VPS, tidak semua orang punya domain, dan tidak semua orang mau mengurus server.

Di Bagian 8, kita membandingkan tiga tempat untuk menjalankan relay. Di akhir bagian ini, relay bisa berjalan di belakang layanan tunnel atau di layanan container, dan client memperbarui koneksinya sendiri sebelum diputus oleh layanan itu.

## Apa yang sebenarnya dibutuhkan relay VPN?

Di Bagian 2, kita sepakat bahwa client hanya membuka koneksi keluar ke relay, lewat WebSocket di atas HTTPS. Jadi yang dibutuhkan relay hanyalah satu alamat HTTPS yang bisa dijangkau dari internet, seperti `wss://relay.example.com`. Relay sendiri tidak harus punya IP publik, selama ada pihak lain yang memegang alamat itu dan meneruskan koneksinya ke relay.

Ada tiga cara untuk mendapatkan alamat itu. Ketiganya adalah:

- **VPS sendiri.** VPS punya IP publik, domain mengarah ke IP itu, dan Caddy mengurus sertifikat HTTPS. Inilah cara di Bagian 3.
- **Layanan tunnel.** Relay berjalan di komputer mana pun, misalnya PC kantor yang selalu menyala. Sebuah program di sebelahnya membuka koneksi keluar ke penyedia layanan, dan penyedia memberi alamat HTTPS publik yang diteruskan lewat koneksi itu.
- **Layanan container.** Container adalah paket berisi program beserta semua yang dibutuhkannya. Penyedia cloud menjalankan container berisi relay, dan memberi alamat HTTPS publik untuknya.

![Tiga baris. Di baris VPS sendiri, client terhubung lewat wss ke Caddy, yang meneruskan ke relay di VPS yang sama, seharga sekitar 4 sampai 5 dolar sebulan. Di baris layanan tunnel, client terhubung ke layanan tunnel milik penyedia, dan relay di PC kantor membuka koneksi keluar ke layanan itu, gratis dengan batas pemakaian. Di baris layanan container, client terhubung ke layanan container, yang meneruskan ke relay dengan umur koneksi paling lama 60 menit, seharga sekitar 44 dolar sebulan.](assets/relay-hosting-options.png)

*Client tidak peduli relay berjalan di mana. Yang berubah hanya alamat di undangan.*

## Relay di belakang layanan tunnel

Layanan tunnel memecahkan masalah yang sama dengan relay kita, tetapi untuk server web. Komputer di balik CGNAT tidak bisa menerima koneksi masuk, jadi program tunnel membuka koneksi keluar ke penyedia, dan penyedia meneruskan request dari internet lewat koneksi itu. Dua layanan yang terkenal adalah Cloudflare Tunnel dan Microsoft dev tunnels.

Dengan Cloudflare, cara tercepatnya adalah quick tunnel, yang tidak butuh akun maupun domain. Perintahnya, di komputer yang menjalankan relay, adalah:

```sh
cloudflared tunnel --url http://localhost:8080
```

Program `cloudflared` lalu mencetak alamat acak di `trycloudflare.com`. Masalahnya, alamat itu berganti setiap kali quick tunnel dibuat, dan Cloudflare menyebut quick tunnel hanya untuk percobaan. Untuk alamat yang tetap, Cloudflare menyarankan Cloudflare Tunnel biasa, yang terikat ke akun Cloudflare-mu.

Dengan Microsoft dev tunnels, kita membuat tunnel yang tetap, lalu menjalankannya. Perintahnya adalah:

```sh
devtunnel user login
devtunnel create -a
devtunnel port create -p 8080
devtunnel host
```

`devtunnel host` mencetak alamat seperti `https://abcd1234-8080.usw2.devtunnels.ms`, yang tetap sama selama tunnel itu tidak dihapus. Pilihan `-a` membuka akses anonim, yaitu siapa pun boleh terhubung tanpa login Microsoft atau GitHub. Microsoft memperingatkan bahwa dengan akses anonim, siapa pun yang menebak alamatnya bisa menjangkau server kita.

Untuk relay kita, akses anonim itu aman. Relay hanya menerima client yang bisa membuktikan private key-nya sejak Bagian 4, dan hanya menerima undangan dengan token yang benar sejak Bagian 6. Setelah tunnel berjalan, relay dibuat dengan alamat itu, misalnya `relay init wss://abcd1234-8080.usw2.devtunnels.ms`, dan dijalankan dengan `relay run` seperti biasa.

### Kelemahan relay di belakang layanan tunnel

Layanan tunnel menghapus tiga kebutuhan sekaligus, yaitu IP publik, domain, dan Caddy, dan biasanya gratis. Tetapi ada empat hal yang dikorbankan. Keempatnya adalah:

- **Penyedia melihat siapa berbicara dengan siapa.** HTTPS berakhir di penyedia, jadi penyedia melihat setiap pesan WebSocket. Isi paketnya tetap aman, karena WireGuard mengenkripsinya dari ujung ke ujung, tetapi public key di depan setiap pesan, ukurannya, dan waktunya terlihat.
- **Batas pemakaian.** Dev tunnels dibatasi 5 GB per pengguna per bulan dan 20 MB per detik per tunnel. Layanannya masih preview tanpa jaminan, tunnel yang 30 hari tidak dipakai dihapus, dan login di program `devtunnel` hanya berlaku beberapa hari.
- **Komputer relay harus selalu menyala.** Kalau PC kantor mati, seluruh VPN ikut mati.
- **Alamat tidak boleh berubah.** Alamat relay tertulis di setiap undangan dan di setiap `client.json`. Alamat yang berubah, seperti quick tunnel, berarti mengubah konfigurasi semua client.

Ada satu hal lagi. Cloudflare menutup koneksi WebSocket yang terlalu lama diam, dan bisa memutus koneksi saat servernya diperbarui. Ping setiap 15 detik dari Bagian 5 menjaga koneksi tidak pernah diam, dan Bind yang tersambung lagi sendiri menangani pemutusan saat pembaruan.

Jadi layanan tunnel cocok untuk beberapa komputer dengan traffic kecil, seperti SSH dan remote desktop, kalau ada satu komputer yang selalu menyala. Layanan ini kurang cocok untuk memindahkan file besar setiap hari.

## Relay di layanan container

Layanan container menghapus kebutuhan terakhir, yaitu komputer yang selalu menyala, dan contohnya adalah Google Cloud Run, yang menjalankan container dan memberi alamat HTTPS untuknya. Cloud Run mendukung WebSocket tanpa pengaturan tambahan, tetapi relay kita bertemu empat aturan baru di sana. Keempatnya adalah:

- **Umur koneksi dibatasi.** Setiap koneksi WebSocket dihitung sebagai satu request, dan request dibatasi waktunya, bawaannya 5 menit dan paling lama 60 menit. Jadi setiap client diputus paling tidak sekali setiap jam.
- **Port ditentukan penyedia.** Container harus mendengarkan di `0.0.0.0`, yaitu semua alamat, di port yang diberikan lewat variabel lingkungan `PORT`. Bawaan relay kita, `127.0.0.1:8080`, tidak akan pernah menerima koneksi.
- **File tidak bertahan.** File di dalam container disimpan di memori dan hilang saat container berhenti. Padahal `relay.json` berisi client dan undangan yang berubah saat relay berjalan.
- **Jumlah container bisa bertambah sendiri.** Bawaannya, Cloud Run menjalankan sampai 100 container saat ramai. Relay menyimpan daftar client yang terhubung di memorinya, jadi client di container yang berbeda tidak bisa saling menjangkau. Jumlahnya harus dibatasi satu, dan Cloud Run menyebut batas itu bisa terlewati sebentar, misalnya saat traffic melonjak, sehingga client bisa tersambung lagi beberapa kali.

Aturan pertama dan kedua dijawab dengan kode di bawah. Aturan ketiga tidak, karena caranya berbeda di setiap penyedia. `relay.json` harus disimpan di luar container, misalnya di penyimpanan file milik penyedia cloud, dan file penanda dari Bagian 6 tidak bekerja di sana, karena ia hanya menjaga satu disk di satu mesin.

### Biaya relay di layanan container

Cloud Run menghitung biaya selama container aktif, dan container yang memegang koneksi WebSocket terbuka selalu dihitung aktif. Karena client VPN selalu terhubung, container relay aktif sepanjang bulan. Dengan harga Cloud Run di us-central1, satu vCPU dan 512 MiB memori selama 30 hari, setelah jatah gratis, perhitungannya adalah:

```text
CPU     (2.592.000 − 240.000) vCPU-detik × US$0,000018 = US$42,34
memori  (1.296.000 − 450.000) GiB-detik  × US$0,000002 = US$1,69
jumlah                                                   US$44,03
```

Biaya itu belum termasuk data yang keluar ke internet, yang dihitung untuk setiap byte yang diteruskan relay. Sebagai pembanding, VPS termurah di DigitalOcean berharga US$4 sebulan, dan Nanode 1 GB di Akamai US$5 sebulan.

Jadi layanan container cocok kalau kamu sama sekali tidak mau mengurus mesin dan rela membayar sekitar sepuluh kali harga VPS. Layanan ini kurang cocok kalau tujuannya hemat.

## Memperbarui koneksi sebelum diputus

Di Bagian 5, Bind sudah tersambung lagi sendiri setiap kali koneksinya putus. Jadi pemutusan setiap jam di layanan container tidak mematikan VPN.

### Bukankah tersambung lagi sudah cukup?

Tersambung lagi memang menyelamatkan VPN, tetapi tidak gratis. Selama Bind tersambung lagi, paket dibuang, dan TCP di dalam tunnel harus mengirim ulang, jadi setiap client mengalami jeda sesaat setiap jam. Kalau semua client terhubung pada menit yang sama, semuanya terputus pada menit yang sama juga.

Cara yang lebih baik adalah membuka koneksi baru sebelum koneksi lama diputus, tetapi harus ada yang tahu kapan koneksi itu akan diputus. Ada dua pilihan. Keduanya adalah:

- **Setiap client memperbarui koneksinya sendiri** setelah waktu tertentu. Masalahnya, setiap client harus diatur, dan setiap client harus tahu layanan apa yang ada di depan relay.
- **Relay meminta client memperbaruinya.** Batas umur koneksi adalah sifat tempat relay dijalankan, jadi relay yang paling tahu. Pengaturannya cukup satu, di satu tempat.

Seri ini memilih cara kedua. Relay diberi flag `-renew`, misalnya `-renew 50m` untuk batas 60 menit di Cloud Run, sehingga masih ada sepuluh menit untuk berpindah.

### Anatomi pembaruan koneksi

Pembaruan koneksi berjalan dalam empat langkah. Langkah-langkahnya adalah:

1. Setiap koneksi di relay punya pengatur waktunya sendiri. Setelah koneksi berumur sepanjang `-renew`, relay mengirim daftar client seperti biasa, dengan tambahan `"renew": true`.
2. Client membuka koneksi baru dan menjalani pemeriksaan dua arah dari Bagian 4. Begitu koneksi baru lolos, relay mencatatnya sebagai koneksi client itu, dan mulai mengirim lewat koneksi baru.
3. Client memindahkan pengirimannya ke koneksi baru, lalu menutup koneksi lama. Fungsi penerima pun pindah membaca koneksi baru.
4. Batas umur koneksi lama tiba, tetapi koneksi itu sudah tidak dipakai.

![Dua garis waktu, koneksi lama dan koneksi baru. Pada langkah 1, relay meminta koneksi baru. Koneksi baru diperiksa, lalu pada langkah 2 lolos pemeriksaan dan relay mulai mengirim lewatnya. Pada langkah 3, client pindah dan menutup koneksi lama. Pada langkah 4, batas umur koneksi lama tiba, tetapi koneksi itu sudah tidak dipakai.](assets/renew-before-cut.png)

*Tidak ada saat di mana client tanpa koneksi.*

Ada satu perubahan kecil di relay agar cara ini berhasil. Di Bagian 5, relay langsung menutup koneksi lama begitu koneksi baru dari client yang sama masuk. Kalau itu tetap terjadi, fungsi penerima di client bisa melihat koneksi lamanya mati sebelum sempat pindah, lalu menganggapnya putus dan membuka koneksi ketiga.

Karena itu, relay kini memberi koneksi lama waktu 10 detik sebelum menutupnya. Kelemahannya kecil. Koneksi mati dari laptop yang pindah jaringan, seperti di Bagian 5, bertahan 10 detik lebih lama di relay, tetapi pesan sudah dikirim lewat koneksi baru.

Pembaruan ini hanya membantu batas yang bisa diperkirakan. Pemutusan yang datang tiba-tiba, seperti saat Cloudflare memperbarui servernya, tetap ditangani dengan tersambung lagi.

## Mengubah relay VPN untuk pembaruan koneksi

Di `relay/relay.go`, ada konstanta baru untuk waktu pindah. Isinya adalah:

```go
const (
	queueLen     = 256              // pesan yang boleh menunggu untuk satu client
	writeTimeout = 10 * time.Second // client yang tidak membaca selama ini diputus
	renewGrace   = 10 * time.Second // waktu bagi client untuk pindah ke koneksi barunya
)
```

`Welcome` mendapat field `Renew`. Field ini hanya ditulis kalau bernilai `true`, berkat `omitempty`, jadi daftar client biasa tidak berubah. Kodenya adalah:

```go
// Welcome dikirim relay sebagai pesan teks saat client lolos pemeriksaan,
// dan setiap kali daftar client berubah.
type Welcome struct {
	Name    string `json:"name"`            // nama client ini
	Address string `json:"address"`         // alamat VPN client ini, dengan prefix
	Domain  string `json:"domain"`          // domain VPN, misalnya vpn.internal
	Peers   []Peer `json:"peers"`           // semua client lainnya
	Renew   bool   `json:"renew,omitempty"` // minta client membuka koneksi baru
}
```

`Relay` mendapat field `RenewAfter`. Kodenya adalah:

```go
type Relay struct {
	// RenewAfter, kalau tidak nol, meminta setiap client membuka koneksi
	// baru setelah koneksinya berumur selama ini. Pakai ini kalau ada pihak
	// di depan relay yang memutus koneksi yang terlalu lama. Atur sebelum
	// relay mulai menerima koneksi.
	RenewAfter time.Duration

	key     [KeyLen]byte // private key relay
	network netip.Prefix
	domain  string

	mu      sync.Mutex
	members map[[KeyLen]byte]Member
	online  map[[KeyLen]byte]*conn
}
```

Di `ServeHTTP`, koneksi lama ditutup dengan `time.AfterFunc`, yaitu fungsi Go yang menjalankan sesuatu setelah waktu tertentu, dan setiap koneksi baru mendapat pengatur waktunya sendiri. Bagian yang berubah adalah:

```go
		if old := r.online[me]; old != nil {
			// Koneksi lama dari client yang sama ditutup sebentar lagi,
			// supaya client sempat pindah ke koneksi barunya.
			time.AfterFunc(renewGrace, func() { old.ws.CloseNow() })
		}
		r.online[me] = o
		r.sendWelcome(me, o, false)
```

Setelah penulis dijalankan, pengatur waktu pembaruan dimulai. Kodenya adalah:

```go
	go o.write(ctx)
	if r.RenewAfter > 0 {
		go r.askRenew(ctx, me, o)
	}
```

`askRenew` menunggu sampai koneksi cukup tua, lalu mengirim daftar client dengan `Renew`, tetapi hanya kalau koneksi itu masih koneksi aktif client tersebut. `sendWelcome` kini menerima satu parameter tambahan. Kodenya adalah:

```go
// askRenew menunggu sampai koneksi berumur RenewAfter, lalu meminta client
// membuka koneksi baru.
func (r *Relay) askRenew(ctx context.Context, me [KeyLen]byte, o *conn) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(r.RenewAfter):
	}
	r.mu.Lock()
	if r.online[me] == o {
		r.sendWelcome(me, o, true)
	}
	r.mu.Unlock()
}

// sendWelcome menaruh daftar client terbaru di antrean client itu. Kalau
// antreannya penuh, client itu diputus, karena saat tersambung lagi ia
// mendapat daftar yang lengkap. Dipanggil saat r.mu terkunci.
func (r *Relay) sendWelcome(me [KeyLen]byte, o *conn, renew bool) {
	w := r.welcome(me)
	w.Renew = renew
	data, err := json.Marshal(w)
	if err != nil {
		return
	}
	select {
	case o.queue <- message{websocket.MessageText, data}:
	default:
		o.ws.CloseNow()
	}
}
```

Semua pemanggilan `sendWelcome` yang lama memakai `false`.

## Mengubah Bind WebSocket untuk pembaruan koneksi

Di `wsbind/wsbind.go`, fungsi penerima memulai pembaruan saat daftar client membawa `Renew`. Bagian yang berubah adalah:

```go
			if typ == websocket.MessageText {
				var w relay.Welcome
				if json.Unmarshal(frame, &w) == nil {
					b.setWelcome(w)
					if w.Renew {
						go b.renew(ctx, ws)
					}
				}
				continue
			}
```

`drop` kini membedakan koneksi yang putus dari koneksi lama yang sudah digantikan, dan `renew` adalah fungsi baru. Kodenya adalah:

```go
// drop melepas koneksi yang sudah putus, supaya fungsi penerima tersambung
// lagi. Koneksi lama yang sudah digantikan renew dilepas tanpa tersambung
// lagi.
func (b *Bind) drop(ws *websocket.Conn) {
	ws.CloseNow()
	b.mu.Lock()
	current := b.ws == ws
	if current {
		b.ws = nil
	}
	b.mu.Unlock()
	if current {
		log.Printf("wsbind: koneksi ke relay putus")
	}
}

// renew membuka koneksi baru ke relay selagi koneksi lama masih hidup, lalu
// memindahkan pengiriman dan penerimaan ke koneksi baru. Kalau gagal,
// koneksi lama tetap dipakai sampai diputus.
func (b *Bind) renew(ctx context.Context, old *websocket.Conn) {
	ws, err := b.dial(ctx)
	if err != nil {
		log.Printf("wsbind: koneksi baru belum bisa dibuka: %v", err)
		return
	}
	b.mu.Lock()
	swapped := b.ws == old
	if swapped {
		b.ws = ws
	}
	b.mu.Unlock()
	if !swapped {
		ws.CloseNow() // koneksi lama sudah putus duluan dan sudah diganti
		return
	}
	old.CloseNow() // fungsi penerima lalu pindah membaca koneksi baru
}
```

Urutan di `renew` adalah inti dari seluruh cara ini. Koneksi baru dibuka dan diperiksa lebih dulu, lalu `b.ws` diganti dalam satu langkah di bawah mutex, dan baru setelah itu koneksi lama ditutup. Kalau koneksi lama ternyata sudah putus duluan dan sudah diganti oleh `redial`, koneksi baru dari `renew` ditutup, supaya tidak ada dua koneksi yang dipakai bersamaan.

## Menjalankan relay di layanan container

Program relay di `cmd/relay/main.go` mendapat flag `-renew`, dan membaca variabel lingkungan `PORT`. Bagian yang berubah adalah:

```go
	path := flag.String("config", "relay.json", "file konfigurasi")
	listen := flag.String("listen", "127.0.0.1:8080", "alamat yang didengarkan relay")
	renew := flag.Duration("renew", 0, "minta client membuka koneksi baru setiap selang ini, misalnya 50m")
	flag.Parse()
	// Layanan container seperti Cloud Run memberi tahu port lewat PORT, dan
	// relay harus mendengarkan di semua alamat.
	if port := os.Getenv("PORT"); port != "" && !flagSet("listen") {
		*listen = ":" + port
	}
```

`PORT` hanya dipakai kalau `-listen` tidak ditulis di baris perintah, supaya pilihan yang ditulis tangan selalu menang. Fungsi `flagSet` memeriksanya dengan `flag.Visit`, yang hanya mengunjungi flag yang benar-benar ditulis. Kodenya adalah:

```go
// flagSet memeriksa apakah sebuah flag ditulis di baris perintah.
func flagSet(name string) bool {
	set := false
	flag.Visit(func(f *flag.Flag) { set = set || f.Name == name })
	return set
}
```

`run` dan `newHandler` meneruskan nilai `-renew` ke `RenewAfter`. Di layanan container, setelah `relay.json` punya tempat di luar container, relay dijalankan dengan `relay -renew 50m run`, dengan batas waktu request diatur 60 menit dan jumlah container paling banyak satu.

## Mencoba pembaruan koneksi

Test baru di `wsbind/sesi_test.go` meniru layanan yang membatasi umur koneksi dengan `front`, yaitu proxy TCP yang memutus setiap koneksi tepat satu detik setelah dibuka. Relay meminta pembaruan setelah setengah detik, dan A memanggil web server B setiap 100 milidetik selama tiga detik. Kodenya adalah:

```go
// Pihak di depan relay memutus setiap koneksi satu detik setelah dibuka,
// seperti layanan yang membatasi umur koneksi. Relay meminta client
// membuka koneksi baru setelah setengah detik, jadi koneksi lama selalu
// sudah ditinggalkan sebelum diputus, dan tunnel tidak pernah terganggu.
func TestPerbaruiKoneksi(t *testing.T) {
	logs := captureLog(t)
	relayPriv, relayPub := newKey(t)
	privA, pubA := newKey(t)
	privB, pubB := newKey(t)

	r := relay.New(relayPriv, netip.MustParsePrefix("10.10.0.0/24"), "vpn.internal", members(pubA, pubB))
	r.RenewAfter = 500 * time.Millisecond
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	front, url := startFront(t, "ws"+strings.TrimPrefix(srv.URL, "http"), time.Second)

	netA, _ := member(t, url, privA, relayPub)
	netB, _ := member(t, url, privB, relayPub)
	serve(t, netB, "10.10.0.2")
	for end := time.Now().Add(3 * time.Second); time.Now().Before(end); time.Sleep(100 * time.Millisecond) {
		get(t, netA, "http://10.10.0.2/")
	}

	if n := front.opened.Load(); n < 10 {
		t.Fatalf("hanya %d koneksi yang dibuka; koneksi tidak diperbarui", n)
	}
	if strings.Contains(logs.String(), "koneksi ke relay putus") {
		t.Fatalf("koneksi sempat putus:\n%s", logs.String())
	}
}
```

Test ini menuntut dua hal. Yang pertama, `front` harus menerima paling sedikit 10 koneksi, tanda bahwa koneksi benar-benar diperbarui. Yang kedua, log tidak boleh berisi "koneksi ke relay putus", tanda bahwa tidak ada satu pun client yang sempat terputus.

`captureLog` menampung log selama test, dan `front` dibuat seperti `cutter` dari Bagian 5, dengan `time.AfterFunc` untuk memutus koneksi. Hasilnya adalah:

```text
$ go test -v -run TestPerbaruiKoneksi ./wsbind/
=== RUN   TestPerbaruiKoneksi
--- PASS: TestPerbaruiKoneksi (3.09s)
PASS
ok  	vpnku/wsbind	4.237s
```

Test yang sama dijalankan sekali lagi dengan `RenewAfter` bernilai 0, yaitu tanpa pembaruan. Kali ini setiap koneksi diputus `front`, dan test gagal karena log berisi "koneksi ke relay putus". Pesan gagalnya adalah:

```text
--- FAIL: TestPerbaruiKoneksi (3.08s)
    sesi_test.go:47: koneksi sempat putus:
```

Percobaan di bagian ini memakai `front` sebagai tiruan layanan di depan relay. Relay belum dijalankan di Cloudflare, dev tunnels, maupun Cloud Run sungguhan untuk artikel ini.

## Kelemahan VPN versi kedelapan

Relay kini bisa berjalan di tiga tempat, dan client memperbarui koneksinya sebelum diputus. Tetapi masih ada enam kelemahan yang perlu kamu tahu. Keenamnya adalah:

- **Layanan tunnel melihat siapa berbicara dengan siapa.** Isi paket tetap terenkripsi, tetapi public key, ukuran, dan waktu setiap pesan terlihat oleh penyedia.
- **Relay di layanan container butuh penyimpanan di luar container.** Seri ini tidak membangunnya, karena caranya berbeda di setiap penyedia.
- **Layanan container sekitar sepuluh kali lebih mahal daripada VPS kecil**, belum termasuk data yang keluar.
- **Pembaruan hanya membantu batas yang diketahui.** Pemutusan mendadak tetap memakai tersambung lagi dari Bagian 5, dengan jedanya.
- **Semua paket masih lewat relay**, dan kini juga lewat penyedia di depannya. Perjalanan paket bertambah panjang.
- **Relay masih dipercaya menentukan client.** Kelemahan dari Bagian 4 ini belum berubah.

## Berikutnya dalam seri ini

Di Bagian 9, bagian terakhir seri ini, kita merawat VPN yang sudah jadi. Program client menyala sendiri saat komputer menyala, menjadi satu file untuk setiap OS termasuk Raspberry Pi, dan bisa diperbarui dengan aman. Setelah itu, kita menutup seri ini dengan melihat kembali kelemahan dari seluruh desainnya, dan kapan sebaiknya memilih alat lain.
