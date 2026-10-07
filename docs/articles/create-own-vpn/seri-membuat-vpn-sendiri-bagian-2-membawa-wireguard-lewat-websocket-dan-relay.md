# [Seri Membuat VPN Sendiri Bagian 2] Membawa WireGuard lewat WebSocket dan relay

*Di bagian ini, kita menulis jalur pengiriman VPN dengan Go. Paket WireGuard tidak lagi lewat UDP, tetapi lewat WebSocket ke sebuah relay, sehingga tetap berjalan di balik CGNAT.*

Di Bagian 1, kita sudah melihat apa itu VPN. VPN menghubungkan komputer di tempat yang berbeda lewat tunnel, yaitu jalur tertutup di atas internet. Kita memilih WireGuard sebagai protokolnya. Kita juga memilih relay, yaitu server yang meneruskan paket di antara anggota, karena komputer di balik CGNAT tidak bisa dijangkau dari internet.

Di Bagian 2, kita mulai menulis kodenya. Fokusnya adalah jalur pengiriman, yaitu cara paket WireGuard berpindah dari satu anggota ke anggota lain. Di akhir bagian ini, dua anggota VPN akan bertukar data lewat relay. Semuanya bisa dicoba di satu komputer, tanpa hak administrator.

## Kenapa WireGuard perlu dibawa lewat WebSocket?

WireGuard biasanya mengirim paket lewat UDP, langsung ke IP publik dan port milik peer-nya. Peer adalah komputer lain yang menjadi lawan bicara di tunnel. Cara ini cepat dan sederhana, selama salah satu peer bisa dijangkau dari internet.

Masalahnya, di balik CGNAT tidak ada peer yang bisa dijangkau. Firewall kantor juga sering hanya mengizinkan HTTPS keluar. HTTPS adalah koneksi web terenkripsi, yang biasanya memakai port 443. Jadi kita butuh jalur lain yang membawa paket WireGuard tanpa koneksi masuk. Ada tiga pilihan yang masuk akal. Ketiganya adalah:

- **Hole punching lewat UDP.** Kedua peer saling mengirim paket pada saat yang sama, supaya masing-masing NAT membuka jalan. Cara ini sering berhasil, tetapi sebagian CGNAT menggagalkannya, dan firewall yang memblokir UDP tetap menghalangi.
- **Koneksi TCP biasa ke relay.** TCP adalah cara mengirim yang menjamin data sampai lengkap dan berurutan. Koneksi keluar memang diizinkan, tetapi firewall kantor sering hanya membuka port tertentu. Seperti port 443 untuk HTTPS.
- **WebSocket di atas HTTPS.** WebSocket adalah koneksi web yang tetap terbuka, sehingga kedua sisi bisa saling mengirim pesan kapan saja. Karena berjalan di atas HTTPS, bagi firewall koneksi ini terlihat seperti traffic web biasa, dan hampir selalu diizinkan keluar.

Seri ini memilih WebSocket di atas HTTPS, karena hanya pilihan ini yang lolos di balik CGNAT maupun firewall kantor. Pilihan ini punya tiga harga. Ketiganya adalah:

- **Semua paket lewat relay.** Perjalanannya lebih jauh, dan kecepatan seluruh VPN dibatasi oleh relay.
- **TCP di dalam TCP.** WebSocket berjalan di atas TCP, dan aplikasi di dalam VPN sering memakai TCP juga. Saat jaringan sering kehilangan paket, dua lapisan TCP ini bisa saling memperlambat, karena keduanya sama-sama mengirim ulang data yang hilang.
- **Enkripsi dua lapis.** HTTPS mengenkripsi perjalanan sampai ke relay, lalu WireGuard mengenkripsi isinya sekali lagi dari ujung ke ujung. Harganya sedikit kerja tambahan untuk CPU dan beberapa byte header tambahan.

Untuk menghubungkan beberapa komputer milik sendiri, harga ini masih masuk akal. Jika kamu butuh kecepatan setinggi mungkin, jalur UDP langsung tetap lebih baik, dan relay bisa menjadi cadangan saat jalur langsung gagal.

## Anatomi jalur pengiriman VPN

Sebelum menulis kode, kita lihat dulu bagian-bagian jalur pengiriman. Di Bagian 1, setiap komputer memegang sepasang kunci. Kunci yang dibagikan ke anggota lain disebut public key, dan kunci yang dirahasiakan disebut private key. Public key ini akan menjadi alamat setiap anggota. Ada empat bagian yang bekerja di jalur pengiriman. Keempatnya adalah:

- **wireguard-go** mengurus semua urusan WireGuard, yaitu enkripsi, handshake, dan pergantian kunci. Saat sebuah paket siap dikirim, wireguard-go menyerahkannya ke Bind.
- **Bind** adalah bagian wireguard-go yang mengirim dan menerima paket ke jaringan. Bind bawaannya memakai UDP. Kita menggantinya dengan Bind buatan sendiri yang memakai WebSocket.
- **Endpoint** adalah alamat tujuan sebuah paket menurut wireguard-go. Untuk UDP, endpoint berupa IP dan port. Untuk kita, endpoint adalah public key milik peer tujuan.
- **Relay** menyimpan daftar anggota yang sedang terhubung, yaitu pasangan public key dan koneksi WebSocket-nya. Setiap pesan yang masuk diteruskan ke koneksi milik public key tujuan.

![Anggota A dan anggota B masing-masing punya wireguard-go dan Bind WebSocket yang menggantikan UDP. Kedua Bind membuka koneksi WebSocket keluar ke relay di tengah. Relay menyimpan daftar anggota, yaitu public key dan koneksinya. Relay hanya membaca public key di depan setiap pesan, sedangkan paket WireGuard di belakangnya tetap terenkripsi.](assets/delivery-path-anatomy.png)

*Bind WebSocket menggantikan UDP, dan relay mengantar paket berdasarkan public key.*

Public key dipakai sebagai alamat karena dua alasan. Keduanya adalah:

- **Public key sudah unik.** Setiap anggota punya public key yang berbeda, jadi kita tidak butuh nomor atau nama tambahan.
- **Alamatnya tidak berubah saat komputer pindah jaringan.** Seperti laptop yang pindah dari Wi-Fi rumah ke hotspot ponsel. Laptop itu cukup membuat koneksi WebSocket baru, dan relay tetap mengenalinya dari public key yang sama.

## Format pesan antara anggota VPN dan relay

Relay perlu tahu ke mana setiap paket harus diantar, dan penerima perlu tahu dari siapa paket itu datang. Karena itu, setiap paket WireGuard dikirim sebagai satu pesan WebSocket jenis binary, dengan public key sepanjang 32 byte di depannya. Ada tiga jenis pesan. Ketiganya adalah:

1. **Pesan perkenalan.** Begitu terhubung, anggota mengirim public key-nya sendiri. Relay mencatat koneksi itu sebagai milik public key tersebut.
2. **Pesan dari anggota ke relay.** 32 byte pertamanya adalah public key tujuan, lalu disusul paket WireGuard.
3. **Pesan dari relay ke anggota.** Relay mengganti 32 byte pertama dengan public key pengirim, lalu meneruskannya. Paket WireGuard di belakangnya tidak diubah.

![Tiga baris. Pertama, pesan perkenalan berisi public key A sepanjang 32 byte, dan relay mencatat koneksi itu sebagai milik A. Kedua, pesan dari A ke relay berisi tujuan, yaitu public key B, lalu paket WireGuard yang terenkripsi. Ketiga, pesan dari relay ke B berisi pengirim, yaitu public key A, lalu paket WireGuard yang sama tanpa diubah.](assets/relay-message-format.png)

*Relay hanya membaca dan mengganti 32 byte pertama setiap pesan.*

Kenapa relay yang mengisi public key pengirim, bukan anggota itu sendiri? Kalau anggota yang mengisinya, anggota nakal bisa mengaku sebagai anggota lain di setiap pesan. Relay tahu pasti dari koneksi mana sebuah pesan datang, jadi relay yang paling layak mengisinya. Siapa yang boleh mengaku memakai sebuah public key saat perkenalan adalah soal lain, yang dibahas di bagian batasan.

## Menyiapkan project Go untuk VPN

Kode di bagian ini butuh Go 1.26 atau lebih baru. Kita membuat satu module Go bernama `vpnku`. Module adalah satu project Go beserta daftar library yang dipakainya. Perintah untuk membuatnya adalah:

```sh
mkdir vpnku && cd vpnku
go mod init vpnku
go get github.com/coder/websocket golang.zx2c4.com/wireguard golang.org/x/crypto
```

Ketiga library itu punya tugas masing-masing. `github.com/coder/websocket` mengurus WebSocket, `golang.zx2c4.com/wireguard` adalah wireguard-go, dan `golang.org/x/crypto` dipakai untuk membuat kunci di test. Susunan foldernya adalah:

```text
vpnku/
  relay/relay.go          relay yang meneruskan pesan
  cmd/relay/main.go       program untuk menjalankan relay
  wsbind/wsbind.go        Bind WebSocket untuk wireguard-go
  wsbind/wsbind_test.go   test dua anggota lewat relay
```

## Menulis relay VPN dengan Go

Relay adalah server HTTP yang menerima koneksi WebSocket dari setiap anggota. Kodenya ada di `relay/relay.go`.

```go
// Package relay meneruskan paket WireGuard di antara anggota VPN.
package relay

import (
	"net/http"
	"sync"

	"github.com/coder/websocket"
)

// KeyLen adalah panjang public key WireGuard dalam byte.
const KeyLen = 32

type Relay struct {
	mu      sync.Mutex
	members map[[KeyLen]byte]*websocket.Conn
}

func New() *Relay {
	return &Relay{members: map[[KeyLen]byte]*websocket.Conn{}}
}

func (r *Relay) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	c, err := websocket.Accept(w, req, nil)
	if err != nil {
		return
	}
	defer c.CloseNow()
	ctx := req.Context()

	// Pesan pertama dari anggota adalah public key-nya.
	_, first, err := c.Read(ctx)
	if err != nil || len(first) != KeyLen {
		return
	}
	var me [KeyLen]byte
	copy(me[:], first)

	r.mu.Lock()
	r.members[me] = c
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		if r.members[me] == c {
			delete(r.members, me)
		}
		r.mu.Unlock()
	}()

	for {
		typ, frame, err := c.Read(ctx)
		if err != nil {
			return
		}
		if typ != websocket.MessageBinary || len(frame) <= KeyLen {
			continue
		}
		var to [KeyLen]byte
		copy(to[:], frame[:KeyLen])

		r.mu.Lock()
		dst := r.members[to]
		r.mu.Unlock()
		if dst == nil {
			continue // tujuan sedang offline, jadi paketnya dibuang
		}

		// Ganti tujuan dengan pengirim, supaya penerima tahu dari siapa
		// paket ini datang.
		copy(frame[:KeyLen], me[:])
		dst.Write(ctx, websocket.MessageBinary, frame)
	}
}
```

Kode ini bekerja dalam enam langkah. Langkah-langkahnya adalah:

1. `websocket.Accept` mengubah request HTTP menjadi koneksi WebSocket.
2. Pesan pertama dibaca sebagai public key anggota, lalu koneksinya dicatat di `members`.
3. Setiap pesan berikutnya dibaca 32 byte pertamanya untuk mencari koneksi tujuan.
4. Kalau tujuannya tidak ditemukan, pesan itu dibuang. Paket yang hilang bukan masalah besar, karena WireGuard dan TCP di dalamnya akan mengirim ulang, persis seperti saat UDP kehilangan paket.
5. Kalau tujuannya ditemukan, 32 byte pertama diganti dengan public key pengirim, lalu pesan dikirim ke tujuan.
6. Saat koneksi putus, anggota itu dihapus dari daftar.

Setiap koneksi berjalan di goroutine-nya sendiri, jadi beberapa goroutine bisa membaca dan mengubah `members` pada saat yang sama. Karena itu, `members` dijaga oleh `sync.Mutex`. Mutex memastikan hanya satu goroutine yang menyentuh daftar itu pada satu waktu.

Program untuk menjalankan relay ada di `cmd/relay/main.go`, dan dijalankan dengan `go run ./cmd/relay`.

```go
package main

import (
	"log"
	"net/http"

	"vpnku/relay"
)

func main() {
	log.Println("relay berjalan di :8080")
	log.Fatal(http.ListenAndServe(":8080", relay.New()))
}
```

## Menulis Bind WebSocket untuk wireguard-go

Bind adalah sebuah interface di wireguard-go. Interface di Go adalah daftar method yang harus dimiliki sebuah type. Siapa pun yang punya keenam method ini bisa menjadi Bind. Isi interface itu, tanpa komentarnya, adalah:

```go
type Bind interface {
	Open(port uint16) (fns []ReceiveFunc, actualPort uint16, err error)
	Close() error
	SetMark(mark uint32) error
	Send(bufs [][]byte, ep Endpoint) error
	ParseEndpoint(s string) (Endpoint, error)
	BatchSize() int
}

type ReceiveFunc func(packets [][]byte, sizes []int, eps []Endpoint) (n int, err error)
```

Setiap method punya tugasnya sendiri. Tugas-tugas itu adalah:

- **Open** menyalakan Bind, lalu mengembalikan fungsi penerima. wireguard-go memanggil fungsi penerima berulang kali untuk mengambil paket yang datang.
- **Close** mematikan Bind. Setelah itu, fungsi penerima harus mengembalikan `net.ErrClosed`.
- **SetMark** memberi tanda pada paket untuk kernel Linux. WebSocket tidak membutuhkannya, jadi method ini tidak melakukan apa-apa.
- **Send** mengirim satu atau beberapa paket ke sebuah endpoint.
- **ParseEndpoint** mengubah tulisan dari konfigurasi WireGuard menjadi endpoint.
- **BatchSize** memberi tahu berapa paket yang diproses sekaligus. Kita memproses satu per satu.

Endpoint juga sebuah interface, dengan enam method kecil. Yang penting bagi kita adalah `DstToString` dan `DstToBytes`, yang mengembalikan public key peer. Method lainnya boleh mengembalikan nilai kosong, karena kita tidak memakai IP dan port. Kodenya ada di `wsbind/wsbind.go`.

```go
// Package wsbind membawa paket WireGuard lewat WebSocket ke sebuah relay,
// menggantikan UDP.
package wsbind

import (
	"context"
	"encoding/hex"
	"errors"
	"net"
	"net/netip"
	"sync"

	"github.com/coder/websocket"
	"golang.zx2c4.com/wireguard/conn"
)

const keyLen = 32

// Endpoint adalah alamat sebuah peer. Di sini alamatnya adalah public key
// peer itu, karena relay mengantar paket berdasarkan public key.
type Endpoint [keyLen]byte

func (e Endpoint) ClearSrc()           {}
func (e Endpoint) SrcToString() string { return "" }
func (e Endpoint) DstToString() string { return hex.EncodeToString(e[:]) }
func (e Endpoint) DstToBytes() []byte  { return e[:] }
func (e Endpoint) DstIP() netip.Addr   { return netip.Addr{} }
func (e Endpoint) SrcIP() netip.Addr   { return netip.Addr{} }

// Bind menggantikan socket UDP milik wireguard-go.
type Bind struct {
	URL string       // alamat relay, misalnya wss://relay.example.com
	Key [keyLen]byte // public key milik kita sendiri

	mu sync.Mutex
	ws *websocket.Conn
}

func (b *Bind) Open(port uint16) ([]conn.ReceiveFunc, uint16, error) {
	ctx := context.Background()
	ws, _, err := websocket.Dial(ctx, b.URL, nil)
	if err != nil {
		return nil, 0, err
	}
	// Perkenalkan diri ke relay dengan public key kita.
	if err := ws.Write(ctx, websocket.MessageBinary, b.Key[:]); err != nil {
		ws.CloseNow()
		return nil, 0, err
	}
	b.mu.Lock()
	b.ws = ws
	b.mu.Unlock()

	receive := func(packets [][]byte, sizes []int, eps []conn.Endpoint) (int, error) {
		for {
			_, frame, err := ws.Read(ctx)
			if err != nil {
				return 0, net.ErrClosed
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

func (b *Bind) Send(bufs [][]byte, ep conn.Endpoint) error {
	to, ok := ep.(Endpoint)
	if !ok {
		return errors.New("wsbind: endpoint bukan public key")
	}
	b.mu.Lock()
	ws := b.ws
	b.mu.Unlock()
	if ws == nil {
		return net.ErrClosed
	}
	for _, buf := range bufs {
		frame := append(to[:], buf...)
		if err := ws.Write(context.Background(), websocket.MessageBinary, frame); err != nil {
			return err
		}
	}
	return nil
}

func (b *Bind) ParseEndpoint(s string) (conn.Endpoint, error) {
	key, err := hex.DecodeString(s)
	if err != nil || len(key) != keyLen {
		return nil, errors.New("wsbind: endpoint harus public key dalam hex")
	}
	var e Endpoint
	copy(e[:], key)
	return e, nil
}

func (b *Bind) Close() error {
	b.mu.Lock()
	ws := b.ws
	b.ws = nil
	b.mu.Unlock()
	if ws != nil {
		return ws.CloseNow()
	}
	return nil
}

func (b *Bind) SetMark(uint32) error { return nil }
func (b *Bind) BatchSize() int       { return 1 }
```

Kode ini bekerja dalam lima langkah. Langkah-langkahnya adalah:

1. `Open` membuka koneksi WebSocket ke relay, lalu mengirim pesan perkenalan berisi public key kita.
2. `Open` mengembalikan satu fungsi penerima. Setiap kali dipanggil, fungsi itu membaca satu pesan, memisahkan 32 byte pengirim, lalu menyalin paket WireGuard ke `packets[0]`.
3. `Send` menempelkan public key tujuan di depan setiap paket, lalu mengirimnya sebagai pesan WebSocket.
4. `ParseEndpoint` mengubah public key yang ditulis dalam hex menjadi `Endpoint`. Hex adalah cara menulis byte dengan angka 0 sampai 9 dan huruf a sampai f.
5. `Close` menutup koneksi WebSocket. Fungsi penerima lalu gagal membaca dan mengembalikan `net.ErrClosed`, tanda bagi wireguard-go bahwa Bind sudah ditutup.

Library WebSocket ini mengizinkan beberapa goroutine menulis ke satu koneksi pada saat yang sama. Jadi `Send` tidak butuh mutex tambahan, walaupun wireguard-go bisa memanggilnya dari banyak goroutine.

## Mencoba dua anggota VPN tanpa hak administrator

Di Bagian 1, virtual network card butuh hak administrator. Untuk mencoba jalur pengiriman, kita belum membutuhkannya. wireguard-go punya netstack, yaitu network stack yang berjalan di dalam program. Network stack adalah bagian OS yang mengurus IP dan TCP. Dengan netstack, setiap anggota punya alamat VPN sendiri tanpa membuat virtual network card sungguhan.

Test di bawah menjalankan relay dan dua anggota, A dan B, di satu program. B menjalankan web server kecil di `10.10.0.1`, dan A memanggilnya lewat tunnel. Kodenya ada di `wsbind/wsbind_test.go`.

```go
package wsbind_test

import (
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/curve25519"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/netstack"

	"vpnku/relay"
	"vpnku/wsbind"
)

func TestDuaAnggota(t *testing.T) {
	srv := httptest.NewServer(relay.New())
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")

	privA, pubA := newKey(t)
	privB, pubB := newKey(t)
	netA := member(t, url, privA, pubA, pubB, "10.10.0.2", "10.10.0.1")
	netB := member(t, url, privB, pubB, pubA, "10.10.0.1", "10.10.0.2")

	// B menjalankan web server kecil di alamat VPN-nya.
	ln, err := netB.ListenTCP(&net.TCPAddr{IP: net.ParseIP("10.10.0.1"), Port: 80})
	if err != nil {
		t.Fatal(err)
	}
	go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "halo dari B")
	}))

	// A memanggil web server itu lewat tunnel.
	client := http.Client{
		Transport: &http.Transport{DialContext: netA.DialContext},
		Timeout:   10 * time.Second,
	}
	resp, err := client.Get("http://10.10.0.1/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "halo dari B" {
		t.Fatalf("jawaban B: %q", body)
	}
}

// newKey membuat sepasang kunci WireGuard.
func newKey(t *testing.T) (priv, pub [32]byte) {
	rand.Read(priv[:])
	priv[0] &= 248
	priv[31] = priv[31]&127 | 64
	p, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		t.Fatal(err)
	}
	copy(pub[:], p)
	return priv, pub
}

// member menyalakan satu anggota VPN. Virtual network card-nya adalah
// netstack, yang berjalan di dalam program, jadi tes ini tidak butuh hak
// administrator.
func member(t *testing.T, url string, priv, pub, peer [32]byte, self, peerIP string) *netstack.Net {
	tunDev, tnet, err := netstack.CreateNetTUN([]netip.Addr{netip.MustParseAddr(self)}, nil, 1420)
	if err != nil {
		t.Fatal(err)
	}
	bind := &wsbind.Bind{URL: url, Key: pub}
	dev := device.NewDevice(tunDev, bind, device.NewLogger(device.LogLevelError, ""))
	t.Cleanup(dev.Close)

	conf := fmt.Sprintf("private_key=%x\npublic_key=%x\nendpoint=%x\nallowed_ip=%s/32\n",
		priv, peer, peer, peerIP)
	if err := dev.IpcSet(conf); err != nil {
		t.Fatal(err)
	}
	if err := dev.Up(); err != nil {
		t.Fatal(err)
	}
	return tnet
}
```

Test ini berjalan dalam empat langkah. Langkah-langkahnya adalah:

1. `httptest.NewServer` menjalankan relay di port acak.
2. `newKey` membuat private key acak, lalu menghitung public key-nya dengan fungsi X25519 dari `golang.org/x/crypto`.
3. `member` membuat netstack, Bind WebSocket, dan device wireguard-go, lalu mengaturnya lewat `IpcSet`. `IpcSet` menerima konfigurasi WireGuard dalam bentuk teks, satu pengaturan per baris. Endpoint peer diisi dengan public key peer itu sendiri, ditulis dalam hex. `allowed_ip` adalah alamat VPN yang boleh dikirim ke dan diterima dari peer itu.
4. B membuka web server di alamat VPN-nya, lalu A memanggilnya. Jawaban "halo dari B" hanya bisa sampai kalau seluruh jalur bekerja, dari wireguard-go A, Bind A, relay, Bind B, sampai wireguard-go B.

Jalankan test-nya dengan perintah di bawah. Angka waktunya bisa berbeda di komputermu.

```text
$ go test ./wsbind/
ok  	vpnku/wsbind	0.084s
```

## Harga dan batasan jalur pengiriman versi pertama

Jalur pengiriman ini sudah bekerja, tetapi masih versi pertama. Ada empat batasan yang perlu kamu tahu sebelum memakainya. Keempatnya adalah:

- **Relay percaya begitu saja pada pesan perkenalan.** Siapa pun bisa mengaku memakai public key anggota lain. Isi paket tetap aman, karena tanpa private key, penyusup tidak bisa mendekripsi atau membuat paket WireGuard yang sah. Tetapi penyusup bisa menerima paket yang seharusnya untuk anggota itu, dan mengganggu koneksinya. Versi berikutnya perlu meminta bukti bahwa anggota benar-benar memegang private key-nya.
- **Satu anggota yang lambat bisa memperlambat pengirimnya.** Relay menulis ke tujuan di goroutine milik pengirim. Kalau tujuannya lambat, pengirim ikut tertahan. Versi yang lebih baik memberi setiap anggota antrean sendiri, lalu membuang paket saat antrean penuh, seperti UDP.
- **Koneksi yang putus tidak tersambung lagi.** Kalau relay restart atau jaringan putus, Bind ini tidak membuka koneksi baru sendiri. wireguard-go baru memanggil `Open` lagi saat device dinyalakan ulang.
- **Contoh ini memakai HTTP biasa.** Di internet, relay harus berada di belakang HTTPS, supaya lolos firewall kantor dan supaya isi koneksi ke relay tidak terlihat di jalan. Caranya bisa lewat reverse proxy, yaitu server yang menerima HTTPS lalu meneruskannya ke relay. Anggota lalu memakai alamat `wss://`, yaitu WebSocket di atas HTTPS.

## Berikutnya dalam seri ini

Di Bagian 3, kita mengganti netstack dengan virtual network card sungguhan. Kita membuat TUN device di Linux dan Wintun di Windows, memberi alamat dan route, lalu menghubungkan dua komputer nyata lewat relay ini. Setelah itu, kita kembali ke batasan di atas, dimulai dari bukti kepemilikan public key.
