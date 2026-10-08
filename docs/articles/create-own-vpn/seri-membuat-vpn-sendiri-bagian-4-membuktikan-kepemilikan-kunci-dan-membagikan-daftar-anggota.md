# [Seri Membuat VPN Sendiri Bagian 4] Membuktikan kepemilikan kunci dan membagikan daftar anggota

*Di bagian ini, relay berhenti percaya begitu saja. Setiap anggota harus membuktikan bahwa ia memegang private key-nya, relay membuktikan dirinya balik, lalu relay membagikan daftar anggota sehingga tidak ada lagi daftar peer yang ditulis tangan.*

Di Bagian 3, VPN kita sudah berjalan di dua komputer sungguhan. Tetapi ada dua batasan yang kita tinggalkan. Pertama, relay percaya begitu saja pada public key yang diakui oleh siapa pun yang terhubung. Kedua, setiap anggota harus menulis daftar semua anggota lain di file konfigurasinya sendiri.

Di Bagian 4, kita menutup kedua batasan itu. Di akhir bagian ini, relay hanya menerima anggota terdaftar yang bisa membuktikan kepemilikan private key-nya. Anggota juga memeriksa bahwa relay-nya asli. Setelah itu, relay mengirim alamat VPN dan daftar peer ke setiap anggota, sehingga file konfigurasi anggota cukup berisi tiga baris.

## Kenapa relay VPN butuh bukti, bukan pengakuan?

Relay di Bagian 2 dan 3 mencatat setiap koneksi sesuai public key yang dikirim di pesan perkenalan. Masalahnya, public key bukan rahasia. Penyusup yang tahu public key anggota A bisa terhubung ke relay dan mengaku sebagai A. Relay lalu mengirim paket untuk A ke penyusup itu. Isinya tetap tidak terbaca, karena penyusup tidak punya private key A, tetapi A terputus dari VPN.

Masalahnya menjadi lebih serius di bagian ini, karena anggota akan menerima daftar peer dari relay. Kalau anggota terhubung ke relay palsu, relay palsu itu bisa menyelipkan public key milik penyerang ke dalam daftar. Anggota lalu menerima penyerang sebagai peer, dan penyerang bisa menjangkau anggota itu lewat VPN. Jadi pemeriksaannya harus berjalan dua arah. Relay memeriksa anggota, dan anggota memeriksa relay.

Ada tiga cara yang umum untuk memeriksa anggota. Ketiganya adalah:

- **Password bersama.** Semua anggota memakai satu password yang sama. Caranya sederhana, tetapi satu password yang bocor membuka pintu untuk siapa pun. Mengeluarkan satu anggota juga berarti mengganti password di semua komputer.
- **Token untuk setiap anggota.** Setiap anggota mendapat token, yaitu kode rahasia miliknya sendiri. Cara ini berhasil, tetapi setiap anggota kini menyimpan dua rahasia, yaitu private key WireGuard dan token. Keduanya harus dijaga dan diganti secara terpisah.
- **Bukti dari private key WireGuard.** Anggota membuktikan bahwa ia memegang private key dari public key yang ia akui, tanpa mengirim private key itu. Tidak ada rahasia baru yang perlu disimpan.

Seri ini memilih cara ketiga, karena setiap anggota cukup menjaga satu rahasia. Harganya ada dua. Relay sekarang butuh sepasang kunci sendiri, dan setiap anggota harus tahu public key relay. Selain itu, kunci WireGuard tidak bisa dipakai untuk membuat digital signature, yaitu tanda pada pesan yang hanya bisa dibuat oleh pemegang private key. Jadi buktinya dibuat dengan cara lain yang dibahas di bawah.

## Anatomi bukti kepemilikan kunci

Bukti kepemilikan kunci dibangun dari empat bagian. Keempatnya adalah:

- **Shared secret.** Shared secret adalah angka rahasia yang sama, yang dihitung sendiri oleh kedua pihak. Kunci WireGuard memakai X25519, yaitu cara Diffie-Hellman di kurva Curve25519. Diffie-Hellman adalah cara dua pihak menghitung shared secret tanpa pernah mengirimnya. Anggota menggabungkan private key-nya dengan public key relay, dan relay menggabungkan private key relay dengan public key anggota. Hasilnya sama persis, dan tidak ada orang lain yang bisa menghitungnya tanpa salah satu private key itu.
- **Nonce.** Nonce adalah angka acak yang hanya dipakai sekali. Relay dan anggota masing-masing membuat nonce baru di setiap koneksi, sehingga bukti dari koneksi lama tidak bisa dipakai ulang.
- **HMAC.** HMAC adalah sidik jari pendek dari sebuah pesan, yang dihitung dengan sebuah kunci rahasia. Tanpa kunci itu, sidik jari yang benar tidak bisa dibuat. Kita memakai shared secret sebagai kuncinya, dan kedua nonce sebagai pesannya.
- **Label.** Bukti dari anggota dan bukti dari relay diberi label yang berbeda. Tanpa label, relay palsu bisa saja mengembalikan bukti milik anggota sebagai bukti dirinya sendiri.

![Dua garis waktu, anggota di kiri dan relay di kanan. Relay mengirim nonce acak. Anggota menjawab dengan public key, nonce anggota, dan bukti anggota, lalu relay memeriksa apakah kunci itu terdaftar dan buktinya cocok. Relay mengirim bukti relay, lalu anggota memeriksanya. Relay mengirim pesan teks JSON berisi alamat dan daftar peer. Setelah itu, paket WireGuard mengalir seperti di Bagian 2.](assets/mutual-proof-sequence.png)

*Private key tidak pernah dikirim. Kedua pihak cukup menunjukkan bahwa mereka bisa menghitung shared secret yang sama.*

Urutan pesannya terdiri dari lima langkah. Langkah-langkahnya adalah:

1. Relay mengirim nonce relay, yaitu 32 byte acak.
2. Anggota mengirim public key-nya, nonce anggota, dan bukti anggota. Bukti anggota adalah HMAC atas kedua nonce, dengan shared secret sebagai kuncinya dan label anggota di depannya.
3. Relay memeriksa dua hal, yaitu apakah public key itu terdaftar, dan apakah buktinya cocok. Kalau salah satunya gagal, relay menutup koneksi. Kalau keduanya cocok, relay mengirim bukti relay, yaitu HMAC atas kedua nonce yang sama dengan label relay.
4. Anggota memeriksa bukti relay. Kalau tidak cocok, anggota memutus koneksi. Kalau cocok, anggota menerima pesan sambutan dari relay.
5. Paket WireGuard mengalir seperti di Bagian 2.

Kenapa harus ada dua nonce? Nonce relay melindungi relay dari bukti anggota yang direkam lalu diputar ulang. Nonce anggota melindungi anggota dari relay palsu yang memutar ulang bukti relay asli dari koneksi lama.

## Daftar anggota VPN dibagikan oleh relay

Batasan pertama dari Bagian 3 adalah daftar peer yang ditulis tangan. Sekarang relay sudah tahu pasti siapa saja anggotanya, jadi relay yang paling tepat menyimpan daftar itu. Daftarnya ditulis sekali di file konfigurasi relay, lengkap dengan alamat VPN setiap anggota.

Setelah seorang anggota lolos pemeriksaan, relay mengirim pesan sambutan berisi alamat VPN anggota itu dan daftar anggota lainnya. Pesan sambutan ini dikirim sebagai pesan WebSocket jenis teks berisi JSON, bukan jenis binary. Mulai sekarang, aturannya adalah pesan teks untuk kontrol, dan pesan binary untuk paket WireGuard. Dengan begitu, paket WireGuard tidak pernah bisa terbaca sebagai perintah, dan sebaliknya.

Pilihan ini punya harga. Relay kini menjadi satu-satunya pihak yang menentukan siapa anggota VPN. Kalau seseorang berhasil mencuri private key relay, ia bisa menambahkan anggota palsu. Harga ini dibahas lagi di bagian batasan.

## Menulis bukti kepemilikan kunci dengan Go

Bukti dibuat di satu tempat, supaya relay dan anggota memakai rumus yang persis sama. Kodenya ada di package baru, `proof/proof.go`.

```go
// Package proof membuktikan kepemilikan private key WireGuard tanpa pernah
// mengirim private key itu.
package proof

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"golang.org/x/crypto/curve25519"
)

// Label membedakan bukti dari anggota dan bukti dari relay, supaya bukti
// yang satu tidak bisa dipakai sebagai bukti yang lain.
const (
	FromMember = "vpnku anggota"
	FromRelay  = "vpnku relay"
)

// Make membuat bukti untuk dua nonce. Anggota memanggilnya dengan private
// key-nya sendiri dan public key relay. Relay memanggilnya dengan private
// key relay dan public key anggota. Keduanya mendapat shared secret yang
// sama, jadi keduanya bisa membuat dan memeriksa bukti yang sama.
func Make(label string, priv, peerPub [32]byte, relayNonce, memberNonce []byte) ([]byte, error) {
	shared, err := curve25519.X25519(priv[:], peerPub[:])
	if err != nil {
		return nil, err
	}
	mac := hmac.New(sha256.New, shared)
	mac.Write([]byte(label))
	mac.Write(relayNonce)
	mac.Write(memberNonce)
	return mac.Sum(nil), nil
}

// ParseKey membaca kunci WireGuard yang ditulis dalam hex.
func ParseKey(s string) ([32]byte, error) {
	var key [32]byte
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 32 {
		return key, errors.New("kunci harus 32 byte dalam hex")
	}
	copy(key[:], b)
	return key, nil
}
```

`Make` menghitung shared secret dengan `curve25519.X25519`, lalu memakai rahasia itu sebagai kunci HMAC dengan SHA-256. SHA-256 adalah fungsi hash, yaitu fungsi yang mengubah data sepanjang apa pun menjadi 32 byte sidik jari. Label dan kedua nonce lalu dimasukkan sebagai pesannya. `ParseKey` adalah fungsi kecil untuk membaca kunci dari file konfigurasi, karena relay dan anggota sama-sama membutuhkannya.

## Menulis relay VPN yang memeriksa anggota

Relay kini memegang private key-nya sendiri, rentang alamat VPN, dan daftar anggota terdaftar. Kodenya ada di `relay/relay.go`.

```go
// Package relay meneruskan paket WireGuard di antara anggota VPN. Relay hanya
// menerima anggota terdaftar yang bisa membuktikan kepemilikan private key-nya,
// lalu membagikan daftar anggota kepada mereka.
package relay

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"net/netip"
	"sync"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"vpnku/proof"
)

// KeyLen adalah panjang public key WireGuard dalam byte.
const KeyLen = 32

// Member adalah satu anggota yang terdaftar di relay.
type Member struct {
	PublicKey [KeyLen]byte
	Address   netip.Addr
}

// Welcome dikirim relay sebagai pesan teks setelah anggota lolos pemeriksaan.
type Welcome struct {
	Address string `json:"address"` // alamat VPN anggota ini, dengan prefix
	Peers   []Peer `json:"peers"`   // semua anggota lainnya
}

type Peer struct {
	PublicKey string `json:"public_key"` // hex
	Address   string `json:"address"`
}

type Relay struct {
	key     [KeyLen]byte // private key relay
	network netip.Prefix
	members map[[KeyLen]byte]netip.Addr

	mu     sync.Mutex
	online map[[KeyLen]byte]*websocket.Conn
}

func New(key [KeyLen]byte, network netip.Prefix, members []Member) *Relay {
	r := &Relay{
		key:     key,
		network: network,
		members: map[[KeyLen]byte]netip.Addr{},
		online:  map[[KeyLen]byte]*websocket.Conn{},
	}
	for _, m := range members {
		r.members[m.PublicKey] = m.Address
	}
	return r
}

var errNotMember = errors.New("bukan anggota")

func (r *Relay) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	c, err := websocket.Accept(w, req, nil)
	if err != nil {
		return
	}
	defer c.CloseNow()
	ctx := req.Context()

	me, err := r.check(ctx, c)
	if err != nil {
		c.Close(websocket.StatusPolicyViolation, errNotMember.Error())
		return
	}
	if err := wsjson.Write(ctx, c, r.welcome(me)); err != nil {
		return
	}

	r.mu.Lock()
	r.online[me] = c
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		if r.online[me] == c {
			delete(r.online, me)
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
		dst := r.online[to]
		r.mu.Unlock()
		if dst == nil {
			continue // tujuan sedang offline, jadi paketnya dibuang
		}
		copy(frame[:KeyLen], me[:])
		dst.Write(ctx, websocket.MessageBinary, frame)
	}
}

// check meminta anggota membuktikan bahwa ia memegang private key dari public
// key yang ia akui, lalu membuktikan balik bahwa relay memegang private key
// relay.
func (r *Relay) check(ctx context.Context, c *websocket.Conn) ([KeyLen]byte, error) {
	var me [KeyLen]byte
	relayNonce := make([]byte, 32)
	rand.Read(relayNonce)
	if err := c.Write(ctx, websocket.MessageBinary, relayNonce); err != nil {
		return me, err
	}

	// Jawaban anggota berisi public key, nonce anggota, lalu bukti.
	_, msg, err := c.Read(ctx)
	if err != nil {
		return me, err
	}
	if len(msg) != 3*32 {
		return me, errNotMember
	}
	copy(me[:], msg[:32])
	memberNonce, got := msg[32:64], msg[64:]
	if _, ok := r.members[me]; !ok {
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
	return me, c.Write(ctx, websocket.MessageBinary, reply)
}

// welcome menyusun alamat anggota dan daftar anggota lainnya.
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

Dibandingkan Bagian 2, ada empat perubahan. Perubahan-perubahan itu adalah:

1. Relay punya dua daftar. `members` berisi semua anggota terdaftar beserta alamatnya, dan `online` berisi anggota yang sedang terhubung.
2. `check` menggantikan pesan perkenalan. Fungsi ini mengirim nonce relay, membaca jawaban anggota, memeriksa bahwa public key-nya terdaftar dan buktinya cocok, lalu mengirim bukti relay. `hmac.Equal` membandingkan bukti dalam waktu yang sama, berapa pun byte yang cocok, supaya penyerang tidak bisa menebak bukti sedikit demi sedikit dari lamanya pemeriksaan.
3. Anggota yang gagal diputus dengan status `StatusPolicyViolation` dan alasan "bukan anggota". Alasan ini akan muncul di layar anggota itu.
4. `welcome` menyusun pesan sambutan, yang dikirim sebagai JSON lewat `wsjson.Write`. Setelah itu, relay meneruskan paket persis seperti sebelumnya.

Daftar anggota ditulis di file konfigurasi relay, misalnya `relay.json`. Isinya adalah:

```json
{
  "private_key": "isi dengan private key relay",
  "network": "10.10.0.0/24",
  "members": [
    {"public_key": "isi dengan public key A", "address": "10.10.0.2"},
    {"public_key": "isi dengan public key B", "address": "10.10.0.1"}
  ]
}
```

Kunci relay dibuat dengan program `genkey` dari Bagian 3, sama seperti kunci anggota. Program relay di `cmd/relay/main.go` membaca file ini. Kodenya adalah:

```go
package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"net/netip"
	"os"

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
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Fatal(err)
	}
	key, err := proof.ParseKey(cfg.PrivateKey)
	if err != nil {
		log.Fatal("private_key: ", err)
	}
	network, err := netip.ParsePrefix(cfg.Network)
	if err != nil {
		log.Fatal("network: ", err)
	}

	var members []relay.Member
	for _, m := range cfg.Members {
		pub, err := proof.ParseKey(m.PublicKey)
		if err != nil {
			log.Fatal("public_key anggota: ", err)
		}
		addr, err := netip.ParseAddr(m.Address)
		if err != nil || !network.Contains(addr) {
			log.Fatalf("alamat anggota %q harus berada di %s", m.Address, network)
		}
		members = append(members, relay.Member{PublicKey: pub, Address: addr})
	}

	pub, err := curve25519.X25519(key[:], curve25519.Basepoint)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("public key relay: %x", pub)
	log.Printf("relay berjalan di %s dengan %d anggota", *listen, len(members))
	log.Fatal(http.ListenAndServe(*listen, relay.New(key, network, members)))
}
```

Program ini memeriksa setiap alamat anggota, supaya tidak ada alamat di luar rentang VPN. Saat berjalan, program mencetak public key relay. Public key inilah yang perlu ditulis di konfigurasi setiap anggota. Alamat yang didengarkan kini bisa diatur dengan `-listen`, dan bawaannya `127.0.0.1:8080`, sesuai saran di Bagian 3 supaya relay hanya bisa dijangkau lewat Caddy.

## Mengubah Bind WebSocket di sisi anggota VPN

Di sisi anggota, Bind sekarang menyimpan private key kita dan public key relay, supaya bisa membuat bukti dan memeriksa bukti relay. Bagian yang berubah di `wsbind/wsbind.go` dimulai dari daftar import-nya. Isinya adalah:

```go
import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/netip"
	"sync"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"golang.org/x/crypto/curve25519"
	"golang.zx2c4.com/wireguard/conn"

	"vpnku/proof"
	"vpnku/relay"
)
```

Lalu `Bind`, `Open`, dan dua method baru, `join` dan `Welcome`. Kodenya adalah:

```go
// Bind menggantikan socket UDP milik wireguard-go.
type Bind struct {
	URL        string       // alamat relay, misalnya wss://relay.example.com
	PrivateKey [keyLen]byte // private key kita, untuk membuat bukti
	RelayKey   [keyLen]byte // public key relay, untuk memeriksa bukti relay

	mu      sync.Mutex
	ws      *websocket.Conn
	welcome relay.Welcome
}

func (b *Bind) Open(port uint16) ([]conn.ReceiveFunc, uint16, error) {
	ctx := context.Background()
	ws, _, err := websocket.Dial(ctx, b.URL, nil)
	if err != nil {
		return nil, 0, err
	}
	welcome, err := b.join(ctx, ws)
	if err != nil {
		ws.CloseNow()
		return nil, 0, err
	}
	b.mu.Lock()
	b.ws, b.welcome = ws, welcome
	b.mu.Unlock()

	receive := func(packets [][]byte, sizes []int, eps []conn.Endpoint) (int, error) {
		for {
			typ, frame, err := ws.Read(ctx)
			if err != nil {
				return 0, net.ErrClosed
			}
			if typ != websocket.MessageBinary || len(frame) <= keyLen {
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

// join menjawab pemeriksaan dari relay, memeriksa bukti balik dari relay,
// lalu menerima alamat dan daftar anggota.
func (b *Bind) join(ctx context.Context, ws *websocket.Conn) (relay.Welcome, error) {
	var w relay.Welcome
	_, relayNonce, err := ws.Read(ctx)
	if err != nil {
		return w, err
	}
	pub, err := curve25519.X25519(b.PrivateKey[:], curve25519.Basepoint)
	if err != nil {
		return w, err
	}
	memberNonce := make([]byte, 32)
	rand.Read(memberNonce)
	p, err := proof.Make(proof.FromMember, b.PrivateKey, b.RelayKey, relayNonce, memberNonce)
	if err != nil {
		return w, err
	}
	answer := append(append(pub, memberNonce...), p...)
	if err := ws.Write(ctx, websocket.MessageBinary, answer); err != nil {
		return w, err
	}

	// Relay juga harus membuktikan bahwa ia memegang private key relay.
	_, got, err := ws.Read(ctx)
	if err != nil {
		return w, err // relay menolak kita, dan alasannya ada di error ini
	}
	want, err := proof.Make(proof.FromRelay, b.PrivateKey, b.RelayKey, relayNonce, memberNonce)
	if err != nil || !hmac.Equal(got, want) {
		return w, errors.New("wsbind: relay gagal membuktikan identitasnya")
	}
	err = wsjson.Read(ctx, ws, &w)
	return w, err
}

// Welcome mengembalikan alamat dan daftar anggota dari relay.
func (b *Bind) Welcome() relay.Welcome {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.welcome
}
```

Sisanya, yaitu `Endpoint`, `Send`, `ParseEndpoint`, `Close`, `SetMark`, dan `BatchSize`, sama persis dengan Bagian 2. Perubahan di bagian ini ada tiga. Ketiganya adalah:

1. `Open` memanggil `join` sebelum Bind dipakai. Kalau relay menolak, atau relay gagal membuktikan dirinya, `Open` gagal dan wireguard-go tidak pernah mengirim paket lewat koneksi itu.
2. `join` membalik langkah-langkah di relay. Fungsi ini membaca nonce relay, mengirim public key, nonce anggota, dan bukti anggota, memeriksa bukti relay, lalu membaca pesan sambutan dengan `wsjson.Read`.
3. Fungsi penerima kini hanya memproses pesan binary. Pesan teks adalah pesan kontrol, jadi tidak pernah dianggap paket WireGuard.

## Mengubah program anggota VPN

File konfigurasi anggota sekarang jauh lebih pendek. Alamat VPN dan daftar peer tidak ada lagi di sini, karena keduanya datang dari relay. Isinya adalah:

```json
{
  "relay": "wss://relay.example.com",
  "relay_key": "isi dengan public key relay",
  "private_key": "isi dengan private key A"
}
```

Program anggota di `cmd/anggota/main.go` kini menyusun konfigurasi WireGuard dari pesan sambutan. File `os_linux.go` dan `os_windows.go` dari Bagian 3 tidak berubah. Kodenya adalah:

```go
// Program anggota menyalakan satu anggota VPN dengan virtual network card
// sungguhan.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"

	"vpnku/proof"
	"vpnku/wsbind"
)

// Config adalah isi file konfigurasi satu anggota. Alamat VPN dan daftar
// peer tidak ada di sini, karena keduanya datang dari relay.
type Config struct {
	Relay      string `json:"relay"`       // misalnya wss://relay.example.com
	RelayKey   string `json:"relay_key"`   // public key relay, hex
	PrivateKey string `json:"private_key"` // hex, dari program genkey
}

const (
	ifName = "vpnku0"
	mtu    = 1420
)

func main() {
	path := flag.String("config", "anggota.json", "file konfigurasi")
	flag.Parse()

	data, err := os.ReadFile(*path)
	if err != nil {
		log.Fatal(err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Fatal(err)
	}
	bind := &wsbind.Bind{URL: cfg.Relay}
	if bind.PrivateKey, err = proof.ParseKey(cfg.PrivateKey); err != nil {
		log.Fatal("private_key: ", err)
	}
	if bind.RelayKey, err = proof.ParseKey(cfg.RelayKey); err != nil {
		log.Fatal("relay_key: ", err)
	}

	// 1. Virtual network card sungguhan.
	tunDev, err := tun.CreateTUN(ifName, mtu)
	if err != nil {
		log.Fatalf("membuat TUN device: %v (sudah dijalankan sebagai administrator?)", err)
	}
	name, err := tunDev.Name()
	if err != nil {
		log.Fatal(err)
	}

	// 2. WireGuard dengan Bind WebSocket. Up memanggil Open, yang terhubung ke
	// relay dan menjalani pemeriksaan.
	dev := device.NewDevice(tunDev, bind, device.NewLogger(device.LogLevelError, ""))
	defer dev.Close()
	if err := dev.IpcSet("private_key=" + cfg.PrivateKey + "\n"); err != nil {
		log.Fatal(err)
	}
	if err := dev.Up(); err != nil {
		log.Fatal(err)
	}

	// 3. Alamat kita dan daftar peer datang dari relay.
	welcome := bind.Welcome()
	var conf strings.Builder
	for _, p := range welcome.Peers {
		fmt.Fprintf(&conf, "public_key=%s\nendpoint=%s\nallowed_ip=%s/32\n",
			p.PublicKey, p.PublicKey, p.Address)
	}
	if err := dev.IpcSet(conf.String()); err != nil {
		log.Fatal(err)
	}

	// 4. Beri tahu OS alamat VPN kita.
	if err := setupOS(name, welcome.Address); err != nil {
		log.Fatal(err)
	}
	log.Printf("%s aktif dengan alamat %s dan %d peer. Tekan Ctrl+C untuk berhenti.",
		name, welcome.Address, len(welcome.Peers))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
}

// run menjalankan perintah OS satu per satu, dan berhenti di perintah
// pertama yang gagal.
func run(cmds ...[]string) error {
	for _, c := range cmds {
		if out, err := exec.Command(c[0], c[1:]...).CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %v: %s", strings.Join(c, " "), err, out)
		}
	}
	return nil
}
```

Urutannya berubah dibandingkan Bagian 3. Urutan barunya adalah:

1. Hanya private key yang diatur sebelum `dev.Up()`.
2. `dev.Up()` memanggil `Open`, yang terhubung ke relay dan menjalani pemeriksaan dua arah. Kalau ditolak, program berhenti dengan alasan dari relay.
3. Setelah itu, daftar peer dari pesan sambutan diatur lewat `IpcSet`. wireguard-go menerima peer baru walaupun device sudah menyala.
4. Alamat VPN dari pesan sambutan diberikan ke OS lewat `setupOS`, sama seperti di Bagian 3.

## Mencoba pemeriksaan anggota VPN

File test di `wsbind/wsbind_test.go` kini punya empat test. Keempatnya adalah:

- **TestAnggotaTerdaftar.** Dua anggota terdaftar lolos pemeriksaan, mendapat alamat dan daftar peer dari relay, lalu saling bertukar data lewat tunnel di atas netstack, seperti di Bagian 2.
- **TestBukanAnggota.** Kunci yang tidak terdaftar ditolak dengan alasan "bukan anggota".
- **TestPenyusup.** Penyusup yang mengaku memakai public key A, tetapi tidak memegang private key A, ditolak.
- **TestRelayPalsu.** Relay yang tidak memegang private key relay asli ditolak oleh anggota.

Dua test terakhir adalah yang paling penting, karena keduanya mencoba menyerang VPN kita. Kodenya adalah:

```go
// Penyusup yang mengaku memakai public key A, tetapi tidak memegang private
// key A, ditolak relay.
func TestPenyusup(t *testing.T) {
	relayPriv, relayPub := newKey(t)
	_, pubA := newKey(t)
	url := startRelay(t, relayPriv, pubA)

	ctx := context.Background()
	ws, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.CloseNow()
	_, relayNonce, err := ws.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Penyusup membuat bukti dengan private key miliknya sendiri.
	intruderPriv, _ := newKey(t)
	memberNonce := make([]byte, 32)
	p, err := proof.Make(proof.FromMember, intruderPriv, relayPub, relayNonce, memberNonce)
	if err != nil {
		t.Fatal(err)
	}
	answer := append(append(pubA[:], memberNonce...), p...)
	if err := ws.Write(ctx, websocket.MessageBinary, answer); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ws.Read(ctx); websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("penyusup tidak ditolak: %v", err)
	}
}

// Relay palsu, yang tidak memegang private key relay asli, ditolak anggota.
func TestRelayPalsu(t *testing.T) {
	_, relayPub := newKey(t)
	fakePriv, _ := newKey(t)
	privA, pubA := newKey(t)
	url := startRelay(t, fakePriv, pubA)

	bind := &wsbind.Bind{URL: url, PrivateKey: privA, RelayKey: relayPub}
	if _, _, err := bind.Open(0); err == nil {
		t.Fatal("anggota menerima relay palsu")
	}
}
```

`newKey` sama dengan di Bagian 2. `startRelay` menjalankan relay dengan anggota terdaftar, dan `member` menyalakan anggota di atas netstack lalu mengatur peer dari pesan sambutan, persis seperti program anggota. Hasil test-nya adalah:

```text
$ go test -v ./wsbind/
=== RUN   TestAnggotaTerdaftar
--- PASS: TestAnggotaTerdaftar (0.01s)
=== RUN   TestBukanAnggota
--- PASS: TestBukanAnggota (0.00s)
=== RUN   TestPenyusup
--- PASS: TestPenyusup (0.00s)
=== RUN   TestRelayPalsu
--- PASS: TestRelayPalsu (0.00s)
PASS
ok  	vpnku/wsbind	0.083s
```

Percobaan yang sama juga dijalankan dengan virtual network card sungguhan, memakai relay, Caddy, dua anggota terdaftar, dan satu orang asing, di dalam container pada satu mesin. Anggota terdaftar mendapat alamat dan peer dari relay, sedangkan orang asing ditolak. Hasilnya adalah:

```text
anggota A:    vpnku0 aktif dengan alamat 10.10.0.2/24 dan 1 peer. Tekan Ctrl+C untuk berhenti.
anggota B:    vpnku0 aktif dengan alamat 10.10.0.1/24 dan 1 peer. Tekan Ctrl+C untuk berhenti.
orang asing:  failed to get reader: received close frame: status = StatusPolicyViolation and reason = "bukan anggota"
```

Setelah itu, A dan B saling ping lewat `10.10.0.x` tanpa satu pun daftar peer yang ditulis tangan.

## Harga dan batasan VPN versi keempat

Relay sekarang memeriksa anggota, dan anggota memeriksa relay. Tetapi masih ada batasan yang perlu kamu tahu. Batasan-batasan itu adalah:

- **Mengubah anggota butuh restart.** Menambah atau mengeluarkan anggota berarti mengubah `relay.json` lalu menjalankan ulang relay. Karena belum ada reconnect, semua anggota juga harus dijalankan ulang.
- **Daftar peer hanya dikirim sekali.** Pesan sambutan dikirim saat anggota terhubung. Perubahan sesudahnya tidak sampai ke anggota yang sedang berjalan.
- **Relay dipercaya menentukan anggota.** Siapa pun yang mencuri private key relay bisa menyamar sebagai relay dan menambahkan anggota palsu. Jadi private key relay harus dijaga seketat private key anggota. Cara yang lebih kuat adalah daftar anggota yang ditandatangani oleh kunci admin yang disimpan offline, tetapi seri ini belum sampai ke sana.
- **Satu anggota yang lambat masih bisa memperlambat pengirimnya.** Batasan dari Bagian 2 ini belum diperbaiki.

## Berikutnya dalam seri ini

Di Bagian 5, kita membuat VPN ini tahan banting. Anggota akan tersambung lagi sendiri saat koneksi putus atau relay restart. Relay akan mengirim perubahan daftar anggota ke semua anggota yang sedang terhubung, dan setiap anggota mendapat antrean sendiri di relay, sehingga anggota yang lambat tidak lagi menahan yang lain.
