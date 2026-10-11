# [Seri Membuat VPN Sendiri Bagian 5] Membuat VPN yang tahan banting

*Di bagian ini, VPN kita belajar bertahan. Client tersambung lagi sendiri saat koneksi putus, relay mengirim perubahan daftar client tanpa restart, dan client yang lambat tidak lagi menahan client lain.*

Di Bagian 4, relay mulai memeriksa setiap client, dan client memeriksa relay. Relay juga membagikan alamat VPN dan daftar peer, sehingga file konfigurasi client cukup berisi tiga baris. Tetapi VPN itu masih rapuh, dan ada tiga kelemahan yang kita tinggalkan.

Kelemahan pertama, kalau koneksi ke relay putus, VPN berhenti sampai program client dijalankan ulang. Kelemahan kedua, daftar client hanya dikirim sekali, jadi menambah client berarti menjalankan ulang relay. Kelemahan ketiga, satu client yang lambat bisa memperlambat semua client yang mengirim kepadanya.

Di Bagian 5, kita menutup ketiganya. Di akhir bagian ini, client tersambung lagi sendiri, perubahan daftar client sampai ke semua client dalam beberapa detik, dan setiap client punya antrean sendiri di relay.

## Kenapa koneksi ke relay pasti putus suatu saat?

Koneksi ke relay adalah satu koneksi WebSocket di atas TCP, yang dibuka sekali saat program client mulai, dan semua paket WireGuard lewat di dalamnya. Koneksi seperti ini tidak bertahan selamanya. Contohnya adalah:

- **Relay restart**, misalnya karena programnya diperbarui atau VPS-nya dinyalakan ulang.
- **Laptop pindah jaringan**, seperti dari Wi-Fi rumah ke hotspot ponsel. Koneksi lama terikat pada alamat jaringan yang lama.
- **Laptop ditutup lalu dibuka lagi** beberapa jam kemudian. Selama laptop tidur, relay dan jaringan di tengah jalan sudah melupakan koneksinya.
- **Perangkat di tengah jalan membuang koneksi yang lama diam.** NAT di router dan reverse proxy menyimpan catatan setiap koneksi, dan catatan yang lama tidak dipakai akhirnya dihapus.

Putusnya koneksi bisa terjadi dengan dua cara. Kalau relay restart, OS di relay menutup koneksinya, dan program client langsung mendapat error saat membaca. Kalau jaringan putus di tengah jalan, tidak ada yang memberi tahu siapa pun, dan program client terus menunggu paket yang tidak akan pernah datang.

Jadi Bind punya dua tugas baru. Bind harus menemukan koneksi yang mati diam-diam, lalu tersambung lagi sendiri.

### Bukankah TCP sudah punya keepalive?

Memang TCP bisa memeriksa koneksinya sendiri. Pemeriksaan ini disebut keepalive, yaitu paket kecil yang dikirim saat koneksi lama diam, dan Go menyalakannya untuk koneksi WebSocket kita. Hanya saja, keepalive TCP punya dua kelemahan.

Kelemahan pertama adalah lamanya. Koneksi baru dinyatakan mati setelah beberapa kali pemeriksaan tidak dijawab, dengan jeda puluhan detik di antaranya, jadi butuh beberapa menit. Kelemahan kedua lebih penting, yaitu keepalive TCP hanya memeriksa koneksi sampai pihak terdekat.

Di Bagian 3, relay berada di belakang Caddy. Koneksi TCP client berakhir di Caddy, dan Caddy membuka koneksi lain ke relay. Keepalive TCP hanya membuktikan bahwa Caddy masih menjawab, bukan bahwa relay di belakangnya masih hidup.

Karena itu, Bind memakai ping milik WebSocket. Ping adalah pesan kecil yang meminta balasan, dan balasannya disebut pong.

Ping WebSocket berjalan di dalam koneksi WebSocket sampai ke relay, dan relay sendiri yang menjawabnya. Jadi pong yang datang membuktikan seluruh jalurnya hidup. Yang dikorbankan hanya sedikit traffic, yaitu beberapa byte setiap 15 detik untuk setiap client.

### Bukankah WireGuard sudah mencoba lagi sendiri?

WireGuard memang mencoba lagi. Kalau handshake tidak dijawab, wireguard-go mengirim handshake baru setiap 5 detik. Masalahnya, semua percobaan itu diserahkan ke Bind, dan Bind mengirimnya ke koneksi yang sudah mati.

wireguard-go tidak tahu apa-apa tentang WebSocket. wireguard-go hanya memanggil `Open` saat device dinyalakan, dan berhenti memanggil fungsi penerima setelah fungsi itu mengembalikan `net.ErrClosed` atau gagal beberapa kali berturut-turut. Jadi tersambung lagi adalah tugas Bind, dan harus terjadi di dalam fungsi penerima, tanpa wireguard-go perlu tahu.

## Anatomi Bind yang tersambung lagi sendiri

Bind yang tersambung lagi sendiri dibangun dari empat bagian. Keempatnya adalah:

- **Fungsi penerima.** wireguard-go memanggil fungsi ini berulang kali dari satu goroutine untuk mengambil paket. Karena hanya ada satu, fungsi inilah yang memegang koneksi. Kalau membaca gagal, fungsi ini membuka koneksi baru, lalu membaca lagi.
- **Penjaga koneksi.** Setiap koneksi punya goroutine kecil yang mengirim ping setiap 15 detik. Kalau pong tidak datang dalam 10 detik, goroutine ini menutup koneksinya, sehingga fungsi penerima mendapat error dan tersambung lagi.
- **Percobaan tersambung lagi.** Setiap percobaan terdiri dari membuka koneksi dan menjalani pemeriksaan dua arah dari Bagian 4, dan harus selesai dalam 10 detik. Kalau gagal, Bind menunggu sebelum mencoba lagi, mulai dari 1 detik, lalu 2, 4, 8, dan seterusnya, paling lama 30 detik.
- **Pengirim.** Selama Bind belum tersambung, paket yang mau dikirim dibuang tanpa error, seperti UDP saat jaringan putus. WireGuard dan TCP di dalam tunnel akan mengirim ulang.

![Garis waktu satu koneksi ke relay. Koneksi hidup, dan ping setiap 15 detik dijawab pong. Jaringan putus tanpa tanda, lalu sebuah ping tidak dijawab. Setelah 10 detik tanpa pong, koneksi ditutup. Bind mencoba tersambung lagi dengan jeda 1, 2, 4, 8 sampai 30 detik, lalu berhasil, diperiksa, dan menerima daftar client terbaru.](assets/reconnect-timeline.png)

*Ping menemukan koneksi yang mati, lalu Bind tersambung lagi dengan jeda yang makin panjang.*

Sesi WireGuard tidak ikut hilang saat koneksi WebSocket putus. Kunci dan sesi disimpan di wireguard-go, bukan di koneksi, jadi begitu tersambung lagi, paket langsung mengalir. Kunci sesi WireGuard hanya berlaku paling lama tiga menit, jadi kalau putusnya lama, wireguard-go membuat handshake baru sendiri.

### Kenapa jeda percobaan makin panjang?

Ada dua pilihan yang lebih sederhana. Yang pertama adalah mencoba lagi terus-menerus tanpa jeda, dan yang kedua adalah jeda tetap yang panjang, misalnya 30 detik. Keduanya punya masalah.

Mencoba terus-menerus berarti ribuan percobaan per detik saat relay mati lama. CPU dan log penuh, dan begitu relay hidup lagi, semua client menyerbunya bersamaan. Jeda tetap 30 detik lebih tenang, tetapi relay yang restart dalam dua detik membuat VPN mati setengah menit.

Jeda yang berlipat dua menggabungkan kelebihan keduanya. Gangguan singkat pulih dalam hitungan detik, dan gangguan panjang hanya dicoba dua kali per menit. Kelemahannya, setelah gangguan panjang, Bind bisa menunggu sampai 30 detik walaupun relay sudah hidup lagi.

Untuk ratusan client, jeda ini biasanya ditambah sedikit angka acak, supaya client tidak mencoba pada detik yang sama. Untuk beberapa komputer milik sendiri, itu belum dibutuhkan.

## Daftar client VPN berubah tanpa restart

Di Bagian 4, relay membaca daftar client dari `relay.json` sekali, saat mulai, lalu mengirimnya ke setiap client sekali, saat client itu terhubung. Menambah atau mengeluarkan client berarti menjalankan ulang relay.

### Bukankah restart relay sekarang sudah cukup?

Dengan Bind yang tersambung lagi sendiri, restart relay memang sudah cukup. Semua client tersambung lagi, dan setiap client menerima daftar terbaru di pesan sambutan. Hanya saja, semua client ikut menanggung akibatnya.

Restart relay memutus semua client selama beberapa detik, padahal yang berubah hanya satu client. Di percobaan di akhir artikel ini, satu restart relay membuat 13 dari 30 ping hilang. Jadi relay perlu menerapkan perubahan saat berjalan.

### Relay membaca ulang konfigurasinya setiap dua detik

Relay perlu tahu kapan `relay.json` berubah. Ada tiga cara yang umum. Ketiganya adalah:

- **Perintah admin lewat HTTP.** Relay membuka alamat khusus untuk mengubah client. Cara ini cepat, tetapi alamat itu butuh autentikasi sendiri, karena siapa pun yang bisa memanggilnya bisa menambah client.
- **Sinyal dari OS.** Di Linux, program bisa diberi tahu lewat sinyal, misalnya `SIGHUP`. Windows tidak punya sinyal ini, padahal relay bisa saja berjalan di PC Windows.
- **Membaca file secara berkala.** Relay membaca `relay.json` setiap dua detik, lalu membandingkannya dengan isi sebelumnya.

Seri ini memilih cara ketiga, karena paling sederhana dan berjalan di semua OS. Kelemahannya, perubahan baru berlaku dalam dua detik, dan file kecil itu dibaca terus-menerus. File yang sedang setengah ditulis gagal dibaca sebagai JSON, jadi relay mengabaikannya dan mencoba lagi dua detik kemudian.

Hanya daftar client yang dibaca ulang. Private key relay dan rentang alamat VPN tetap butuh restart, karena mengubah keduanya mengubah identitas relay dan alamat semua client.

### Relay mengirim daftar lengkap, bukan perubahannya

Ada dua cara memberi tahu client. Cara pertama, setiap client bertanya ke relay secara berkala. Cara ini sederhana, tetapi perubahan terlambat sampai, dan setiap client terus bertanya walaupun tidak ada yang berubah.

Cara kedua, relay mengirim daftar baru lewat koneksi WebSocket yang sudah terbuka. Perubahan sampai seketika, tanpa koneksi tambahan. Seri ini memilih cara kedua.

Yang dikirim selalu daftar lengkap, bukan hanya perubahannya. Alasannya, client yang sempat terputus mungkin melewatkan beberapa perubahan, sedangkan daftar lengkap selalu benar, kapan pun diterima. Kelemahannya, pesan itu makin besar seiring jumlah client, sekitar 100 byte per client, dan itu masih kecil untuk puluhan client.

Client yang dikeluarkan langsung diputus dengan alasan "bukan client terdaftar". Ia akan terus mencoba tersambung lagi, dan terus ditolak, sampai programnya dihentikan atau ia didaftarkan lagi.

### Client hanya mengubah peer yang berubah

Di sisi client, daftar baru harus diterapkan ke wireguard-go. Cara paling mudah adalah `replace_peers=true`, yaitu pengaturan wireguard-go yang menghapus semua peer lalu menambahkan daftar baru. Masalahnya, setiap peer yang dihapus kehilangan sesinya, jadi setiap perubahan kecil memutus tunnel ke semua client sesaat.

Karena itu, program client membandingkan daftar baru dengan daftar yang sedang dipakai. Peer baru ditambahkan, peer yang hilang dihapus dengan `remove=true`, dan peer yang tetap ada hanya diperbarui alamatnya dengan `replace_allowed_ips=true`, tanpa kehilangan sesinya.

## Antrean untuk setiap client di relay

Di Bagian 2, relay meneruskan pesan dengan menulis ke koneksi tujuan di goroutine milik pengirim. Kalau tujuannya lambat membaca, misalnya karena sinyal ponselnya buruk, penulisan itu menunggu. Selama menunggu, goroutine pengirim tidak membaca pesan berikutnya, jadi semua paket dari pengirim itu tertahan, termasuk paket untuk client lain yang cepat.

Ada tiga cara untuk mengatasinya. Ketiganya adalah:

- **Antrean tanpa batas.** Pengirim tidak pernah menunggu, karena pesan disimpan dulu. Masalahnya, untuk client yang tidak pernah membaca, antrean itu tumbuh terus sampai memori relay habis.
- **Tetap menunggu.** Inilah perilaku relay sekarang, dan inilah masalah yang mau kita selesaikan.
- **Antrean terbatas yang membuang paket saat penuh.** Pengirim tidak pernah menunggu, dan memori yang dipakai ada batasnya.

Seri ini memilih cara ketiga, karena begitulah jaringan biasa bekerja. Router yang sedang sibuk juga membuang paket saat antreannya penuh, dan TCP di dalam tunnel memakai paket yang hilang itu sebagai tanda untuk memperlambat kirimannya. Kelemahannya, paket yang dibuang harus dikirim ulang.

![Di dalam relay, pembaca A menerima pesan dari client A dan menaruhnya di antrean tujuan tanpa menunggu. Antrean B hanya berisi dua pesan, dan penulis B mengirimnya ke client B yang membaca dengan cepat. Antrean S penuh karena client S tidak pernah membaca, jadi pesan baru untuk S dibuang, dan penulis S macet.](assets/relay-queue-per-member.png)

*Pembaca tidak pernah menunggu penerima, jadi client yang lambat hanya merugikan dirinya sendiri.*

Setiap client yang terhubung kini punya dua hal di relay. Keduanya adalah:

- **Antrean** berkapasitas 256 pesan. Antrean ini dibuat dengan channel, yaitu saluran milik Go untuk mengirim nilai dari satu goroutine ke goroutine lain. Pembaca milik pengirim menaruh pesan di sini tanpa menunggu, dan kalau antreannya penuh, pesan itu dibuang.
- **Penulis**, yaitu satu goroutine yang mengambil pesan dari antrean dan menulisnya ke client itu. Hanya penulis ini yang menulis ke koneksi client tersebut. Kalau satu penulisan tidak selesai dalam 10 detik, client itu diputus, lalu ia tersambung lagi dengan antrean kosong.

Daftar client juga lewat antrean yang sama, supaya urutannya terjaga dan hanya ada satu penulis. Bedanya, daftar client tidak boleh dibuang, karena TCP tidak bisa mengirim ulang pesan yang dibuang relay. Jadi kalau antrean penuh saat daftar baru harus dikirim, relay memutus client itu, dan ia mendapat daftar lengkap saat tersambung lagi.

Batas 256 pesan juga membatasi memori. Paket WireGuard paling besar sekitar 1,5 KB, jadi satu antrean yang penuh memakai sekitar 384 KB.

## Menulis relay VPN yang mengikuti perubahan client

Karena relay berubah cukup banyak, berikut seluruh isi `relay/relay.go`. Kodenya adalah:

```go
// Package relay meneruskan paket WireGuard di antara client VPN. Relay hanya
// menerima client terdaftar yang bisa membuktikan kepemilikan private key-nya,
// membagikan daftar client kepada mereka, dan memberi setiap client antrean
// sendiri.
package relay

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/netip"
	"sync"
	"time"

	"github.com/coder/websocket"

	"vpnku/proof"
)

// KeyLen adalah panjang public key WireGuard dalam byte.
const KeyLen = 32

const (
	queueLen     = 256              // pesan yang boleh menunggu untuk satu client
	writeTimeout = 10 * time.Second // client yang tidak membaca selama ini diputus
)

// Member adalah satu client yang terdaftar di relay.
type Member struct {
	PublicKey [KeyLen]byte
	Address   netip.Addr
}

// Welcome dikirim relay sebagai pesan teks saat client lolos pemeriksaan,
// dan setiap kali daftar client berubah.
type Welcome struct {
	Address string `json:"address"` // alamat VPN client ini, dengan prefix
	Peers   []Peer `json:"peers"`   // semua client lainnya
}

type Peer struct {
	PublicKey string `json:"public_key"` // hex
	Address   string `json:"address"`
}

// conn adalah koneksi satu client yang sedang terhubung, dengan antreannya
// sendiri.
type conn struct {
	ws    *websocket.Conn
	queue chan message
}

type message struct {
	typ  websocket.MessageType
	data []byte
}

type Relay struct {
	key     [KeyLen]byte // private key relay
	network netip.Prefix

	mu      sync.Mutex
	members map[[KeyLen]byte]netip.Addr
	online  map[[KeyLen]byte]*conn
}

func New(key [KeyLen]byte, network netip.Prefix, members []Member) *Relay {
	r := &Relay{key: key, network: network, online: map[[KeyLen]byte]*conn{}}
	r.SetMembers(members)
	return r
}

// SetMembers mengganti daftar client saat relay berjalan. Client yang
// dikeluarkan langsung diputus, dan client lain yang sedang terhubung
// menerima daftar baru.
func (r *Relay) SetMembers(members []Member) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.members = map[[KeyLen]byte]netip.Addr{}
	for _, m := range members {
		r.members[m.PublicKey] = m.Address
	}
	for key, o := range r.online {
		if _, ok := r.members[key]; !ok {
			go o.ws.Close(websocket.StatusPolicyViolation, errNotMember.Error())
			continue
		}
		r.sendWelcome(key, o)
	}
}

var errNotMember = errors.New("bukan client terdaftar")

func (r *Relay) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ws, err := websocket.Accept(w, req, nil)
	if err != nil {
		return
	}
	defer ws.CloseNow()
	ctx := req.Context()

	me, err := r.check(ctx, ws)
	if err != nil {
		ws.Close(websocket.StatusPolicyViolation, errNotMember.Error())
		return
	}

	o := &conn{ws: ws, queue: make(chan message, queueLen)}
	r.mu.Lock()
	_, ok := r.members[me] // client bisa saja dikeluarkan saat pemeriksaan berjalan
	if ok {
		if old := r.online[me]; old != nil {
			old.ws.CloseNow() // koneksi lama dari client yang sama
		}
		r.online[me] = o
		r.sendWelcome(me, o)
	}
	r.mu.Unlock()
	if !ok {
		ws.Close(websocket.StatusPolicyViolation, errNotMember.Error())
		return
	}
	defer func() {
		r.mu.Lock()
		if r.online[me] == o {
			delete(r.online, me)
		}
		r.mu.Unlock()
	}()
	go o.write(ctx)

	for {
		typ, frame, err := ws.Read(ctx)
		if err != nil {
			return
		}
		if typ != websocket.MessageBinary || len(frame) <= KeyLen {
			continue
		}
		var to [KeyLen]byte
		copy(to[:], frame[:KeyLen])

		r.mu.Lock()
		dst := r.online[to]
		r.mu.Unlock()
		if dst == nil {
			continue // tujuan sedang offline, jadi paketnya dibuang
		}
		copy(frame[:KeyLen], me[:])
		select {
		case dst.queue <- message{websocket.MessageBinary, frame}:
		default:
			// Antrean tujuan penuh, jadi paketnya dibuang, seperti UDP.
		}
	}
}

// write mengirim isi antrean ke client, satu per satu. Hanya goroutine ini
// yang menulis ke koneksi client itu. Client yang tidak membaca selama
// writeTimeout diputus.
func (o *conn) write(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case m := <-o.queue:
			wctx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := o.ws.Write(wctx, m.typ, m.data)
			cancel()
			if err != nil {
				o.ws.CloseNow()
				return
			}
		}
	}
}

// sendWelcome menaruh daftar client terbaru di antrean client itu. Kalau
// antreannya penuh, client itu diputus, karena saat tersambung lagi ia
// mendapat daftar yang lengkap. Dipanggil saat r.mu terkunci.
func (r *Relay) sendWelcome(me [KeyLen]byte, o *conn) {
	data, err := json.Marshal(r.welcome(me))
	if err != nil {
		return
	}
	select {
	case o.queue <- message{websocket.MessageText, data}:
	default:
		o.ws.CloseNow()
	}
}

// check meminta client membuktikan bahwa ia memegang private key dari public
// key yang ia akui, lalu membuktikan balik bahwa relay memegang private key
// relay.
func (r *Relay) check(ctx context.Context, ws *websocket.Conn) ([KeyLen]byte, error) {
	var me [KeyLen]byte
	relayNonce := make([]byte, 32)
	rand.Read(relayNonce)
	if err := ws.Write(ctx, websocket.MessageBinary, relayNonce); err != nil {
		return me, err
	}

	// Jawaban client berisi public key, nonce client, lalu bukti.
	_, msg, err := ws.Read(ctx)
	if err != nil {
		return me, err
	}
	if len(msg) != 3*32 {
		return me, errNotMember
	}
	copy(me[:], msg[:32])
	memberNonce, got := msg[32:64], msg[64:]
	r.mu.Lock()
	_, ok := r.members[me]
	r.mu.Unlock()
	if !ok {
		return me, errNotMember
	}
	want, err := proof.Make(proof.FromMember, r.key, me, relayNonce, memberNonce)
	if err != nil || !hmac.Equal(got, want) {
		return me, errNotMember
	}

	// Bukti balik dari relay, atas dua nonce yang sama.
	reply, err := proof.Make(proof.FromRelay, r.key, me, relayNonce, memberNonce)
	if err != nil {
		return me, err
	}
	return me, ws.Write(ctx, websocket.MessageBinary, reply)
}

// welcome menyusun alamat client dan daftar client lainnya. Dipanggil saat
// r.mu terkunci.
func (r *Relay) welcome(me [KeyLen]byte) Welcome {
	w := Welcome{Address: netip.PrefixFrom(r.members[me], r.network.Bits()).String()}
	for key, addr := range r.members {
		if key != me {
			w.Peers = append(w.Peers, Peer{PublicKey: hex.EncodeToString(key[:]), Address: addr.String()})
		}
	}
	return w
}
```

Dibandingkan Bagian 4, ada enam perubahan. Perubahan-perubahan itu adalah:

1. `members` kini bisa berubah saat relay berjalan, jadi `check` membacanya sambil memegang mutex.
2. `SetMembers` mengganti daftar client. Client yang dikeluarkan ditutup dengan alasan "bukan client terdaftar". Penutupan ini berjalan di goroutine sendiri, karena `Close` menunggu balasan dari client itu. Client lain mendapat daftar baru lewat `sendWelcome`.
3. `conn` menyimpan koneksi satu client beserta antreannya. Setelah lolos pemeriksaan, relay memeriksa sekali lagi apakah client itu masih terdaftar, karena bisa saja ia dikeluarkan saat pemeriksaan berjalan. Koneksi lama dari client yang sama ditutup, misalnya milik laptop yang baru pindah jaringan.
4. Pesan untuk client lain ditaruh di antrean tujuan dengan `select` dan `default`. Kalau antreannya penuh, `default` yang dijalankan, dan pesannya dibuang.
5. `write` adalah penulis untuk satu client. Setiap penulisan diberi batas 10 detik lewat `context.WithTimeout`.
6. `sendWelcome` mengubah daftar client menjadi JSON, lalu menaruhnya di antrean sebagai pesan teks. Kalau antreannya penuh, client itu diputus.

## Membaca ulang konfigurasi relay saat file berubah

Program relay di `cmd/relay/main.go` kini menyimpan cara membaca konfigurasi di fungsi `parse`, supaya bisa dipakai saat mulai dan saat file berubah. Kodenya adalah:

```go
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"os"
	"time"

	"golang.org/x/crypto/curve25519"

	"vpnku/proof"
	"vpnku/relay"
)

// Config adalah isi file konfigurasi relay.
type Config struct {
	PrivateKey string   `json:"private_key"` // hex, dari program genkey
	Network    string   `json:"network"`     // misalnya 10.10.0.0/24
	Members    []Member `json:"members"`
}

type Member struct {
	PublicKey string `json:"public_key"` // hex
	Address   string `json:"address"`    // misalnya 10.10.0.2
}

func main() {
	path := flag.String("config", "relay.json", "file konfigurasi")
	listen := flag.String("listen", "127.0.0.1:8080", "alamat yang didengarkan relay")
	flag.Parse()

	data, err := os.ReadFile(*path)
	if err != nil {
		log.Fatal(err)
	}
	key, network, members, err := parse(data)
	if err != nil {
		log.Fatal(err)
	}
	pub, err := curve25519.X25519(key[:], curve25519.Basepoint)
	if err != nil {
		log.Fatal(err)
	}
	r := relay.New(key, network, members)
	go watch(*path, data, network, r)

	log.Printf("public key relay: %x", pub)
	log.Printf("relay berjalan di %s dengan %d client", *listen, len(members))
	log.Fatal(http.ListenAndServe(*listen, r))
}

// watch membaca file konfigurasi setiap dua detik. Kalau isinya berubah,
// daftar client yang baru diberikan ke relay tanpa restart.
func watch(path string, last []byte, network netip.Prefix, r *relay.Relay) {
	for range time.Tick(2 * time.Second) {
		data, err := os.ReadFile(path)
		if err != nil || bytes.Equal(data, last) {
			continue
		}
		last = data
		_, n, members, err := parse(data)
		if err == nil && n != network {
			err = fmt.Errorf("network tidak bisa diubah tanpa restart")
		}
		if err != nil {
			log.Printf("perubahan %s tidak dipakai: %v", path, err)
			continue
		}
		r.SetMembers(members)
		log.Printf("daftar client diperbarui: %d client", len(members))
	}
}

// parse membaca isi file konfigurasi dan memeriksa setiap nilainya.
func parse(data []byte) ([32]byte, netip.Prefix, []relay.Member, error) {
	var cfg Config
	var key [32]byte
	var network netip.Prefix
	if err := json.Unmarshal(data, &cfg); err != nil {
		return key, network, nil, err
	}
	key, err := proof.ParseKey(cfg.PrivateKey)
	if err != nil {
		return key, network, nil, fmt.Errorf("private_key: %v", err)
	}
	network, err = netip.ParsePrefix(cfg.Network)
	if err != nil {
		return key, network, nil, fmt.Errorf("network: %v", err)
	}

	var members []relay.Member
	for _, m := range cfg.Members {
		pub, err := proof.ParseKey(m.PublicKey)
		if err != nil {
			return key, network, nil, fmt.Errorf("public_key client: %v", err)
		}
		addr, err := netip.ParseAddr(m.Address)
		if err != nil || !network.Contains(addr) {
			return key, network, nil, fmt.Errorf("alamat client %q harus berada di %s", m.Address, network)
		}
		members = append(members, relay.Member{PublicKey: pub, Address: addr})
	}
	return key, network, members, nil
}
```

`watch` membaca `relay.json` setiap dua detik dengan `time.Tick`. Kalau isinya sama dengan sebelumnya, tidak ada yang dilakukan. Kalau berbeda dan isinya benar, daftar client yang baru diberikan ke `SetMembers`, dan kalau salah, alasannya dicetak dan relay tetap memakai daftar lama.

## Menulis Bind WebSocket yang tersambung lagi sendiri

Bind di `wsbind/wsbind.go` mendapat beberapa import baru, yaitu `encoding/json`, `log`, dan `time`. Daftar import-nya sekarang adalah:

```go
import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"golang.org/x/crypto/curve25519"
	"golang.zx2c4.com/wireguard/conn"

	"vpnku/proof"
	"vpnku/relay"
)
```

Angka-angka yang dibahas di atas ditulis sebagai konstanta di bawah `keyLen`. Isinya adalah:

```go
const (
	pingEvery   = 15 * time.Second // jeda antar-ping ke relay
	pingTimeout = 10 * time.Second // pong harus datang dalam waktu ini
	dialTimeout = 10 * time.Second // terhubung dan lolos pemeriksaan dalam waktu ini
	maxWait     = 30 * time.Second // jeda terlama sebelum mencoba tersambung lagi
)
```

Lalu `Bind`, `Open`, dan empat fungsi baru, yaitu `dial`, `redial`, `drop`, dan `keepAlive`. Kodenya adalah:

```go
// Bind menggantikan socket UDP milik wireguard-go. Kalau koneksi ke relay
// putus, Bind tersambung lagi sendiri.
type Bind struct {
	URL        string       // alamat relay, misalnya wss://relay.example.com
	PrivateKey [keyLen]byte // private key kita, untuk membuat bukti
	RelayKey   [keyLen]byte // public key relay, untuk memeriksa bukti relay

	// Updates menerima tanda setiap kali relay mengirim daftar client,
	// lalu Welcome mengembalikan daftar terbarunya. Buat dengan
	// make(chan struct{}, 1), atau biarkan nil kalau tidak dibutuhkan.
	Updates chan struct{}

	mu      sync.Mutex
	ws      *websocket.Conn
	cancel  context.CancelFunc
	welcome relay.Welcome
}

func (b *Bind) Open(port uint16) ([]conn.ReceiveFunc, uint16, error) {
	ctx, cancel := context.WithCancel(context.Background())
	ws, err := b.dial(ctx)
	if err != nil {
		cancel()
		return nil, 0, err
	}
	b.mu.Lock()
	b.ws, b.cancel = ws, cancel
	b.mu.Unlock()

	receive := func(packets [][]byte, sizes []int, eps []conn.Endpoint) (int, error) {
		for {
			b.mu.Lock()
			ws := b.ws
			b.mu.Unlock()
			if ws == nil {
				if ws = b.redial(ctx); ws == nil {
					return 0, net.ErrClosed // Bind ditutup
				}
			}
			typ, frame, err := ws.Read(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return 0, net.ErrClosed
				}
				b.drop(ws)
				continue
			}
			if typ == websocket.MessageText {
				var w relay.Welcome
				if json.Unmarshal(frame, &w) == nil {
					b.setWelcome(w)
				}
				continue
			}
			if len(frame) <= keyLen {
				continue
			}
			var from Endpoint
			copy(from[:], frame[:keyLen])
			sizes[0] = copy(packets[0], frame[keyLen:])
			eps[0] = from
			return 1, nil
		}
	}
	return []conn.ReceiveFunc{receive}, port, nil
}

// dial terhubung ke relay, menjalani pemeriksaan, lalu menjaga koneksinya
// tetap hidup dengan ping.
func (b *Bind) dial(ctx context.Context) (*websocket.Conn, error) {
	dctx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()
	ws, _, err := websocket.Dial(dctx, b.URL, nil)
	if err != nil {
		return nil, err
	}
	welcome, err := b.join(dctx, ws)
	if err != nil {
		ws.CloseNow()
		return nil, err
	}
	b.setWelcome(welcome)
	go keepAlive(ctx, ws)
	return ws, nil
}

// redial mencoba tersambung lagi sampai berhasil. Jedanya dimulai dari satu
// detik dan berlipat dua setiap kali gagal, sampai paling lama maxWait.
func (b *Bind) redial(ctx context.Context) *websocket.Conn {
	wait := time.Second
	for {
		ws, err := b.dial(ctx)
		if err == nil {
			b.mu.Lock()
			b.ws = ws
			b.mu.Unlock()
			log.Printf("wsbind: tersambung lagi ke relay")
			return ws
		}
		if ctx.Err() != nil {
			return nil
		}
		log.Printf("wsbind: relay belum bisa dihubungi, coba lagi dalam %v: %v", wait, err)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(wait):
		}
		wait = min(wait*2, maxWait)
	}
}

// drop melepas koneksi yang sudah putus, supaya fungsi penerima tersambung
// lagi.
func (b *Bind) drop(ws *websocket.Conn) {
	ws.CloseNow()
	b.mu.Lock()
	if b.ws == ws {
		b.ws = nil
	}
	b.mu.Unlock()
	log.Printf("wsbind: koneksi ke relay putus")
}

// keepAlive mengirim ping ke relay secara berkala. Kalau pong tidak datang
// tepat waktu, koneksinya dianggap mati dan ditutup, sehingga fungsi
// penerima tersambung lagi.
func keepAlive(ctx context.Context, ws *websocket.Conn) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(pingEvery):
		}
		pctx, cancel := context.WithTimeout(ctx, pingTimeout)
		err := ws.Ping(pctx)
		cancel()
		if err != nil {
			ws.CloseNow()
			return
		}
	}
}
```

Fungsi penerima kini bekerja dalam empat langkah. Langkah-langkahnya adalah:

1. Fungsi ini mengambil koneksi yang sedang dipakai. Kalau belum ada, `redial` mencoba tersambung lagi sampai berhasil, atau sampai Bind ditutup.
2. Kalau membaca gagal karena Bind ditutup, fungsi ini mengembalikan `net.ErrClosed`, tanda bagi wireguard-go untuk berhenti. Kalau gagal karena koneksinya putus, `drop` melepas koneksi itu, dan putaran berikutnya tersambung lagi.
3. Pesan teks adalah daftar client, jadi disimpan lewat `setWelcome`.
4. Pesan binary adalah paket WireGuard, dan diserahkan ke wireguard-go seperti sebelumnya.

`dial` membatasi setiap percobaan dengan `dialTimeout`. Tanpa batas ini, percobaan ke jaringan yang membuang paket diam-diam bisa menunggu sampai dua menit, karena TCP terus mengulang permintaan koneksinya. `keepAlive` memakai `ws.Ping`, yang menunggu pong lewat fungsi penerima, karena library WebSocket ini hanya membaca pong saat ada yang sedang membaca koneksinya.

Setelah `join`, yang tidak berubah sejak Bagian 4, ada `setWelcome`, `Welcome`, dan `Send`. Kodenya adalah:

```go
// setWelcome menyimpan daftar client terbaru, lalu memberi tanda lewat
// Updates. Kalau tanda sebelumnya belum dibaca, tanda baru tidak perlu
// dikirim, karena Welcome selalu mengembalikan daftar terbaru.
func (b *Bind) setWelcome(w relay.Welcome) {
	b.mu.Lock()
	b.welcome = w
	b.mu.Unlock()
	select {
	case b.Updates <- struct{}{}:
	default:
	}
}

// Welcome mengembalikan alamat dan daftar client terbaru dari relay.
func (b *Bind) Welcome() relay.Welcome {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.welcome
}

func (b *Bind) Send(bufs [][]byte, ep conn.Endpoint) error {
	to, ok := ep.(Endpoint)
	if !ok {
		return errors.New("wsbind: endpoint bukan public key")
	}
	b.mu.Lock()
	ws := b.ws
	b.mu.Unlock()
	if ws == nil {
		return nil // sedang tersambung lagi, jadi paketnya dibuang, seperti UDP
	}
	for _, buf := range bufs {
		frame := append(to[:], buf...)
		if err := ws.Write(context.Background(), websocket.MessageBinary, frame); err != nil {
			return err
		}
	}
	return nil
}
```

`setWelcome` memberi tanda lewat `Updates` dengan `select` dan `default`. Kalau tanda sebelumnya belum dibaca, tanda baru tidak dikirim, karena `Welcome` selalu mengembalikan daftar terbaru. Dengan begitu, Bind tidak pernah menunggu program client.

Kenapa Bind tidak langsung mengatur peer di wireguard-go? Daftar pertama datang di dalam `Open`, yang dipanggil wireguard-go saat device dinyalakan, sebelum program client siap. Jadi Bind cukup menyimpan daftar dan memberi tanda, lalu program client yang mengatur peer di goroutine-nya sendiri.

`Close` kini juga menghentikan usaha tersambung lagi. Kodenya adalah:

```go
func (b *Bind) Close() error {
	b.mu.Lock()
	ws, cancel := b.ws, b.cancel
	b.ws, b.cancel = nil, nil
	b.mu.Unlock()
	if cancel != nil {
		cancel() // hentikan usaha tersambung lagi
	}
	if ws != nil {
		return ws.CloseNow()
	}
	return nil
}
```

Sisanya, yaitu `Endpoint`, `ParseEndpoint`, `SetMark`, dan `BatchSize`, sama persis dengan Bagian 2, dan `join` sama persis dengan Bagian 4.

## Mengubah program client VPN

Program client di `cmd/client/main.go` kini membuat Bind dengan channel `Updates`. Barisnya adalah:

```go
	bind := &wsbind.Bind{URL: cfg.Relay, Updates: make(chan struct{}, 1)}
```

Daftar import-nya bertambah satu, yaitu `vpnku/relay`, untuk type `relay.Peer`. Bagian akhir `main` dan fungsi baru `setPeers` adalah:

```go
	// 3. Beri tahu OS alamat VPN kita, dari pesan sambutan pertama.
	welcome := bind.Welcome()
	if err := setupOS(name, welcome.Address); err != nil {
		log.Fatal(err)
	}
	log.Printf("%s aktif dengan alamat %s. Tekan Ctrl+C untuk berhenti.", name, welcome.Address)

	// 4. Atur daftar peer sekarang, lalu setiap kali relay mengirim daftar
	// baru, sampai program dihentikan.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	peers := map[string]bool{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-bind.Updates:
			if peers, err = setPeers(dev, peers, bind.Welcome().Peers); err != nil {
				log.Print(err)
			}
		}
	}
}

// setPeers menyamakan peer di wireguard-go dengan daftar dari relay. Peer
// baru ditambahkan, peer yang hilang dihapus, dan peer yang tetap ada tidak
// diganggu, supaya sesi WireGuard-nya tidak putus.
func setPeers(dev *device.Device, current map[string]bool, list []relay.Peer) (map[string]bool, error) {
	var conf strings.Builder
	next := map[string]bool{}
	for _, p := range list {
		next[p.PublicKey] = true
		fmt.Fprintf(&conf, "public_key=%s\nendpoint=%s\nreplace_allowed_ips=true\nallowed_ip=%s/32\n",
			p.PublicKey, p.PublicKey, p.Address)
	}
	for key := range current {
		if !next[key] {
			fmt.Fprintf(&conf, "public_key=%s\nremove=true\n", key)
		}
	}
	if err := dev.IpcSet(conf.String()); err != nil {
		return current, err
	}
	log.Printf("daftar peer dari relay: %d peer", len(next))
	return next, nil
}
```

Program client kini tidak lagi sekadar menunggu Ctrl+C. Program ini menunggu dua hal sekaligus dengan `select`, yaitu Ctrl+C dan tanda dari `Updates`. Setiap tanda membuat `setPeers` menyamakan peer di wireguard-go dengan daftar terbaru.

Tanda pertama sudah menunggu sejak `Open`, jadi daftar peer pertama diatur lewat jalan yang sama dengan perubahan sesudahnya. Alamat VPN client hanya dibaca sekali, dari pesan sambutan pertama. File `os_linux.go` dan `os_windows.go` dari Bagian 3 tidak berubah.

## Mencoba VPN yang tahan banting

File test di `wsbind` kini punya tujuh test, yaitu empat dari Bagian 4 dan tiga yang baru. Ketiga test baru itu adalah:

- **TestTersambungLagi.** Semua koneksi ke relay diputus di tengah jalan, lalu A memanggil web server B lagi lewat tunnel.
- **TestDaftarClientBerubah.** Client C didaftarkan saat relay berjalan dan langsung bisa dijangkau A. Lalu B dikeluarkan, dan B harus hilang dari daftar peer A.
- **TestClientLambat.** Client S terhubung tetapi tidak pernah membaca. A mengirim 20.000 paket ke S, lalu satu paket ke B, dan paket untuk B harus tetap sampai.

Untuk memutus koneksi, test memakai proxy TCP kecil bernama `cutter` di antara client dan relay. Proxy adalah program yang meneruskan koneksi ke tujuan lain, dan `cutter` bisa menutup semua koneksi yang sedang diteruskannya, seperti jaringan yang putus sesaat. Kodenya adalah:

```go
// Koneksi ke relay putus di tengah jalan. Kedua client tersambung lagi
// sendiri, dan tunnel bekerja lagi tanpa program dijalankan ulang.
func TestTersambungLagi(t *testing.T) {
	relayPriv, relayPub := newKey(t)
	privA, pubA := newKey(t)
	privB, pubB := newKey(t)
	_, url := startRelay(t, relayPriv, pubA, pubB)
	network, url := startCutter(t, url)

	netA, _ := member(t, url, privA, relayPub)
	netB, _ := member(t, url, privB, relayPub)
	serve(t, netB, "10.10.0.2")
	get(t, netA, "http://10.10.0.2/")

	network.cut() // semua koneksi ke relay putus
	get(t, netA, "http://10.10.0.2/")
}
```

Test client lambat memakai `dialRaw`, yang menjalani pemeriksaan tanpa wireguard-go, supaya test bisa mengatur sendiri kapan membaca. Kodenya adalah:

```go
// Client S tidak pernah membaca. A mengirim banyak paket ke S, lalu satu
// paket ke B. Paket untuk B tetap sampai, karena setiap client punya
// antrean sendiri di relay.
func TestClientLambat(t *testing.T) {
	relayPriv, relayPub := newKey(t)
	privA, pubA := newKey(t)
	privB, pubB := newKey(t)
	privS, pubS := newKey(t)
	_, url := startRelay(t, relayPriv, pubA, pubB, pubS)
	a := dialRaw(t, url, privA, relayPub)
	b := dialRaw(t, url, privB, relayPub)
	dialRaw(t, url, privS, relayPub) // S terhubung, tetapi tidak pernah membaca

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	packet := make([]byte, 1400)
	for range 20000 {
		if err := a.Write(ctx, websocket.MessageBinary, append(pubS[:], packet...)); err != nil {
			t.Fatal("relay berhenti membaca dari A: ", err)
		}
	}
	if err := a.Write(ctx, websocket.MessageBinary, append(pubB[:], "halo B"...)); err != nil {
		t.Fatal(err)
	}
	_, got, err := b.Read(ctx)
	if err != nil {
		t.Fatal("paket untuk B tidak sampai: ", err)
	}
	if !bytes.Equal(got, append(pubA[:], "halo B"...)) {
		t.Fatalf("B menerima %q", got)
	}
}
```

Test ini benar-benar menguji antrean. Dengan relay dari Bagian 4, relay berhenti membaca dari A begitu koneksi ke S penuh, dan test ini gagal setelah 20 detik. Pesan gagalnya adalah:

```text
--- FAIL: TestClientLambat (20.01s)
    tahan_test.go:91: relay berhenti membaca dari A:  failed to write msg: failed to write frame: context deadline exceeded
```

Dengan relay versi kelima, semua test lulus. Baris yang diawali `wsbind:` adalah log dari Bind, dan angka waktunya bisa berbeda di komputermu. Hasilnya adalah:

```text
$ go test -v ./wsbind/
=== RUN   TestTersambungLagi
2026/10/11 15:56:34 wsbind: koneksi ke relay putus
2026/10/11 15:56:34 wsbind: koneksi ke relay putus
2026/10/11 15:56:34 wsbind: tersambung lagi ke relay
2026/10/11 15:56:34 wsbind: tersambung lagi ke relay
--- PASS: TestTersambungLagi (1.02s)
=== RUN   TestDaftarClientBerubah
2026/10/11 15:56:35 wsbind: koneksi ke relay putus
2026/10/11 15:56:35 wsbind: relay belum bisa dihubungi, coba lagi dalam 1s: failed to get reader: received close frame: status = StatusPolicyViolation and reason = "bukan client terdaftar"
--- PASS: TestDaftarClientBerubah (0.07s)
=== RUN   TestClientLambat
--- PASS: TestClientLambat (0.30s)
=== RUN   TestClientTerdaftar
--- PASS: TestClientTerdaftar (0.02s)
=== RUN   TestBukanClient
--- PASS: TestBukanClient (0.00s)
=== RUN   TestPenyusup
--- PASS: TestPenyusup (0.00s)
=== RUN   TestRelayPalsu
--- PASS: TestRelayPalsu (0.00s)
PASS
ok  	vpnku/wsbind	2.881s
```

### Percobaan dengan virtual network card sungguhan

Percobaan yang sama juga dijalankan dengan virtual network card sungguhan, di satu mesin Linux. Setiap client berjalan di network namespace-nya sendiri, yaitu fitur Linux yang memberi sekelompok program network stack terpisah, seolah mereka berada di komputer lain. Relay berjalan di luar namespace itu, dan tiga client, A, B, dan C, terhubung kepadanya.

Relay dihubungi lewat `ws://` biasa tanpa Caddy, karena bagian HTTPS tidak berubah sejak Bagian 3. Percobaannya terdiri dari empat langkah. Langkah-langkahnya adalah:

1. **Relay restart.** Relay dimatikan, lalu dijalankan lagi tiga detik kemudian, sementara A terus melakukan ping ke B.
2. **Jaringan putus diam-diam.** Semua paket A ke relay dibuang selama 40 detik, tanpa memberi tahu siapa pun.
3. **C didaftarkan.** `relay.json` diubah saat relay berjalan, lalu C dinyalakan dan A melakukan ping ke C.
4. **B dikeluarkan.** `relay.json` diubah lagi tanpa B.

Log client A menunjukkan keempat langkah itu. Relay hidup lagi pada 14:25:09, tetapi A baru mencoba lagi empat detik kemudian, karena jeda percobaannya sudah sampai 4 detik.

Saat jaringan putus diam-diam pada 14:25:21, A menemukan koneksi yang mati 17 detik kemudian. Setiap percobaan selama jaringan masih putus gagal setelah 10 detik, dan A tersambung lagi satu detik setelah jaringan pulih. Potongan lognya adalah:

```text
2026/10/11 14:25:06 wsbind: koneksi ke relay putus
2026/10/11 14:25:13 wsbind: tersambung lagi ke relay
2026/10/11 14:25:13 daftar peer dari relay: 1 peer
2026/10/11 14:25:38 wsbind: koneksi ke relay putus
2026/10/11 14:25:48 wsbind: relay belum bisa dihubungi, coba lagi dalam 1s: failed to WebSocket dial: failed to send handshake request: Get "http://192.0.2.1:8080": context deadline exceeded
2026/10/11 14:26:02 wsbind: tersambung lagi ke relay
2026/10/11 14:26:02 daftar peer dari relay: 1 peer
2026/10/11 14:26:23 daftar peer dari relay: 2 peer
2026/10/11 14:26:29 daftar peer dari relay: 1 peer
```

Selama relay restart, 13 dari 30 ping A ke B hilang, dan sisanya berhasil tanpa program mana pun dijalankan ulang. Setelah C didaftarkan, A bisa melakukan ping ke C.

Setelah B dikeluarkan, ping ke B gagal dan ping ke C tetap berhasil. B sendiri mencatat penolakan dari relay. Log B adalah:

```text
2026/10/11 14:26:29 wsbind: koneksi ke relay putus
2026/10/11 14:26:29 wsbind: relay belum bisa dihubungi, coba lagi dalam 1s: failed to get reader: received close frame: status = StatusPolicyViolation and reason = "bukan client terdaftar"
```

## Kelemahan VPN versi kelima

VPN ini sekarang tersambung lagi sendiri dan mengikuti perubahan client. Tetapi masih ada enam kelemahan yang perlu kamu tahu. Keenamnya adalah:

- **Relay harus bisa dihubungi saat program client mulai.** Percobaan pertama di `Open` tidak diulang, supaya client yang ditolak langsung melihat alasannya. Kalau relay mati saat itu, program client berhenti dan harus dijalankan lagi.
- **Menambah client masih berarti menyalin kunci dengan tangan.** Client baru menjalankan `genkey` dan mengirim public key-nya ke admin. Admin memilih alamat yang masih kosong, menulisnya di `relay.json`, lalu mengirim alamat relay dan public key relay kembali. Satu huruf yang salah dari 64 huruf hex membuat client itu ditolak.
- **Alamat VPN client hanya dibaca saat mulai.** Kalau alamat sebuah client diubah, client lain langsung memakai alamat baru, tetapi client itu sendiri harus dijalankan ulang.
- **Private key relay dan rentang alamat VPN tetap butuh restart relay.**
- **Paket dibuang saat antrean penuh dan saat tersambung lagi.** Aplikasi yang memakai TCP mengirim ulang sendiri, tetapi aplikasi yang memakai UDP, seperti panggilan suara, akan merasakan gangguannya.
- **Relay masih dipercaya menentukan client.** Kelemahan dari Bagian 4 ini belum berubah.

## Berikutnya dalam seri ini

Di Bagian 6, kita berhenti menyalin kunci dengan tangan. Admin membuat undangan sekali pakai dengan satu perintah, dan client baru bergabung dengan undangan itu tanpa private key-nya pernah meninggalkan komputernya. Mengeluarkan client juga cukup dengan satu perintah.
