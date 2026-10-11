# [Seri Membuat VPN Sendiri Bagian 4] Membuktikan kepemilikan kunci dan membagikan daftar client

*Di bagian ini, relay berhenti percaya begitu saja. Setiap client harus membuktikan bahwa ia memegang private key-nya, relay membuktikan dirinya balik, lalu relay membagikan daftar client sehingga tidak ada lagi daftar peer yang ditulis tangan.*

Di Bagian 3, VPN kita sudah berjalan di dua komputer sungguhan. Tetapi ada dua kelemahan yang kita tinggalkan. Pertama, relay percaya begitu saja pada public key yang diakui oleh siapa pun yang terhubung. Kedua, setiap client harus menulis daftar semua client lain di file konfigurasinya sendiri.

Di Bagian 4, kita memperbaiki kedua kelemahan itu. Di akhir bagian ini, relay hanya menerima client terdaftar yang bisa membuktikan kepemilikan private key-nya. Client juga memeriksa bahwa relay-nya asli. Setelah itu, relay mengirim alamat VPN dan daftar peer ke setiap client, sehingga file konfigurasi client cukup berisi tiga baris.

## Kenapa relay VPN butuh bukti, bukan pengakuan?

Relay di Bagian 2 dan 3 mencatat setiap koneksi sesuai public key yang dikirim di pesan perkenalan. Masalahnya, public key bukan rahasia. Penyusup yang tahu public key client A bisa terhubung ke relay dan mengaku sebagai A. Relay lalu mengirim paket untuk A ke penyusup itu. Isinya tetap tidak terbaca, karena penyusup tidak punya private key A, tetapi A terputus dari VPN.

Masalahnya menjadi lebih serius di bagian ini, karena client akan menerima daftar peer dari relay. Kalau client terhubung ke relay palsu, relay palsu itu bisa menyelipkan public key milik penyerang ke dalam daftar. Client lalu menerima penyerang sebagai peer, dan penyerang bisa menjangkau client itu lewat VPN. Jadi pemeriksaannya harus berjalan dua arah. Relay memeriksa client, dan client memeriksa relay.

Ada tiga cara yang umum untuk memeriksa client. Ketiganya adalah:

- **Password bersama.** Semua client memakai satu password yang sama. Caranya sederhana, tetapi satu password yang bocor membuka pintu untuk siapa pun. Mengeluarkan satu client juga berarti mengganti password di semua komputer.
- **Token untuk setiap client.** Setiap client mendapat token, yaitu kode rahasia miliknya sendiri. Cara ini berhasil, tetapi setiap client kini menyimpan dua rahasia, yaitu private key WireGuard dan token. Keduanya harus dijaga dan diganti secara terpisah.
- **Bukti dari private key WireGuard.** Client membuktikan bahwa ia memegang private key dari public key yang ia akui, tanpa mengirim private key itu. Tidak ada rahasia baru yang perlu disimpan.

Seri ini memilih cara ketiga, karena setiap client cukup menjaga satu rahasia. Kelemahannya ada dua. Relay sekarang butuh sepasang kunci sendiri, dan setiap client harus tahu public key relay. Selain itu, kunci WireGuard tidak bisa dipakai untuk membuat digital signature, yaitu tanda pada pesan yang hanya bisa dibuat oleh pemegang private key. Jadi buktinya dibuat dengan cara lain yang dibahas di bawah.

## Anatomi bukti kepemilikan kunci

Bukti kepemilikan kunci dibangun dari empat bagian. Keempatnya adalah:

- **Shared secret.** Shared secret adalah angka rahasia yang sama, yang dihitung sendiri oleh kedua pihak. Kunci WireGuard memakai X25519, yaitu cara Diffie-Hellman di kurva Curve25519. Diffie-Hellman adalah cara dua pihak menghitung shared secret tanpa pernah mengirimnya. Client menggabungkan private key-nya dengan public key relay, dan relay menggabungkan private key relay dengan public key client. Hasilnya sama persis, dan tidak ada orang lain yang bisa menghitungnya tanpa salah satu private key itu.
- **Nonce.** Nonce adalah angka acak yang hanya dipakai sekali. Relay dan client masing-masing membuat nonce baru di setiap koneksi, sehingga bukti dari koneksi lama tidak bisa dipakai ulang.
- **HMAC.** HMAC adalah sidik jari pendek dari sebuah pesan, yang dihitung dengan sebuah kunci rahasia. Tanpa kunci itu, sidik jari yang benar tidak bisa dibuat. Kita memakai shared secret sebagai kuncinya, dan kedua nonce sebagai pesannya.
- **Label.** Bukti dari client dan bukti dari relay diberi label yang berbeda. Tanpa label, relay palsu bisa saja mengembalikan bukti milik client sebagai bukti dirinya sendiri.

![Dua garis waktu, client di kiri dan relay di kanan. Relay mengirim nonce acak. Client menjawab dengan public key, nonce client, dan bukti client, lalu relay memeriksa apakah kunci itu terdaftar dan buktinya cocok. Relay mengirim bukti relay, lalu client memeriksanya. Relay mengirim pesan teks JSON berisi alamat dan daftar peer. Setelah itu, paket WireGuard mengalir seperti di Bagian 2.](assets/mutual-proof-sequence.png)

*Private key tidak pernah dikirim. Kedua pihak cukup menunjukkan bahwa mereka bisa menghitung shared secret yang sama.*

Urutan pesannya terdiri dari lima langkah. Langkah-langkahnya adalah:

1. Relay mengirim nonce relay, yaitu 32 byte acak.
2. Client mengirim public key-nya, nonce client, dan bukti client. Bukti client adalah HMAC atas kedua nonce, dengan shared secret sebagai kuncinya dan label client di depannya.
3. Relay memeriksa dua hal, yaitu apakah public key itu terdaftar, dan apakah buktinya cocok. Kalau salah satunya gagal, relay menutup koneksi. Kalau keduanya cocok, relay mengirim bukti relay, yaitu HMAC atas kedua nonce yang sama dengan label relay.
4. Client memeriksa bukti relay. Kalau tidak cocok, client memutus koneksi. Kalau cocok, client menerima pesan sambutan dari relay.
5. Paket WireGuard mengalir seperti di Bagian 2.

Kenapa harus ada dua nonce? Nonce relay melindungi relay dari bukti client yang direkam lalu diputar ulang. Nonce client melindungi client dari relay palsu yang memutar ulang bukti relay asli dari koneksi lama.

## Daftar client VPN dibagikan oleh relay

Kelemahan pertama dari Bagian 3 adalah daftar peer yang ditulis tangan. Sekarang relay sudah tahu pasti siapa saja client-nya, jadi relay yang paling tepat menyimpan daftar itu. Daftarnya ditulis sekali di file konfigurasi relay, lengkap dengan alamat VPN setiap client.

Setelah sebuah client lolos pemeriksaan, relay mengirim pesan sambutan berisi alamat VPN client itu dan daftar client lainnya. Pesan sambutan ini dikirim sebagai pesan WebSocket jenis teks berisi JSON, bukan jenis binary. Mulai sekarang, aturannya adalah pesan teks untuk kontrol, dan pesan binary untuk paket WireGuard. Dengan begitu, paket WireGuard tidak pernah bisa terbaca sebagai perintah, dan sebaliknya.

Pilihan ini punya kelemahan. Relay kini menjadi satu-satunya pihak yang menentukan siapa client VPN. Kalau seseorang berhasil mencuri private key relay, ia bisa menambahkan client palsu. Kelemahan ini dibahas lagi di akhir artikel.

## Menulis bukti kepemilikan kunci dengan Go

Bukti dibuat di satu tempat, supaya relay dan client memakai rumus yang persis sama. Kodenya ada di package baru, `proof/proof.go`.

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

// Label membedakan bukti dari client dan bukti dari relay, supaya bukti
// yang satu tidak bisa dipakai sebagai bukti yang lain.
const (
	FromMember = "vpnku client"
	FromRelay  = "vpnku relay"
)

// Make membuat bukti untuk dua nonce. Client memanggilnya dengan private
// key-nya sendiri dan public key relay. Relay memanggilnya dengan private
// key relay dan public key client. Keduanya mendapat shared secret yang
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

`Make` menghitung shared secret dengan `curve25519.X25519`, lalu memakai rahasia itu sebagai kunci HMAC dengan SHA-256. SHA-256 adalah fungsi hash, yaitu fungsi yang mengubah data sepanjang apa pun menjadi 32 byte sidik jari. Label dan kedua nonce lalu dimasukkan sebagai pesannya. `ParseKey` adalah fungsi kecil untuk membaca kunci dari file konfigurasi, karena relay dan client sama-sama membutuhkannya.

## Menulis relay VPN yang memeriksa client

Relay kini memegang private key-nya sendiri, rentang alamat VPN, dan daftar client terdaftar. Kodenya ada di `relay/relay.go`.

```go
// Package relay meneruskan paket WireGuard di antara client VPN. Relay hanya
// menerima client terdaftar yang bisa membuktikan kepemilikan private key-nya,
// lalu membagikan daftar client kepada mereka.
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

// Member adalah satu client yang terdaftar di relay.
type Member struct {
	PublicKey [KeyLen]byte
	Address   netip.Addr
}

// Welcome dikirim relay sebagai pesan teks setelah client lolos pemeriksaan.
type Welcome struct {
	Address string `json:"address"` // alamat VPN client ini, dengan prefix
	Peers   []Peer `json:"peers"`   // semua client lainnya
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

var errNotMember = errors.New("bukan client terdaftar")

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

// check meminta client membuktikan bahwa ia memegang private key dari public
// key yang ia akui, lalu membuktikan balik bahwa relay memegang private key
// relay.
func (r *Relay) check(ctx context.Context, c *websocket.Conn) ([KeyLen]byte, error) {
	var me [KeyLen]byte
	relayNonce := make([]byte, 32)
	rand.Read(relayNonce)
	if err := c.Write(ctx, websocket.MessageBinary, relayNonce); err != nil {
		return me, err
	}

	// Jawaban client berisi public key, nonce client, lalu bukti.
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

// welcome menyusun alamat client dan daftar client lainnya.
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

1. Relay punya dua daftar. `members` berisi semua client terdaftar beserta alamatnya, dan `online` berisi client yang sedang terhubung.
2. `check` menggantikan pesan perkenalan. Fungsi ini mengirim nonce relay, membaca jawaban client, memeriksa bahwa public key-nya terdaftar dan buktinya cocok, lalu mengirim bukti relay. `hmac.Equal` membandingkan bukti dalam waktu yang sama, berapa pun byte yang cocok, supaya penyerang tidak bisa menebak bukti sedikit demi sedikit dari lamanya pemeriksaan.
3. Client yang gagal diputus dengan status `StatusPolicyViolation` dan alasan "bukan client terdaftar". Alasan ini akan muncul di layar client itu.
4. `welcome` menyusun pesan sambutan, yang dikirim sebagai JSON lewat `wsjson.Write`. Setelah itu, relay meneruskan paket persis seperti sebelumnya.

Daftar client ditulis di file konfigurasi relay, misalnya `relay.json`. Isinya adalah:

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

Kunci relay dibuat dengan program `genkey` dari Bagian 3, sama seperti kunci client. Program relay di `cmd/relay/main.go` membaca file ini. Kodenya adalah:

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
			log.Fatal("public_key client: ", err)
		}
		addr, err := netip.ParseAddr(m.Address)
		if err != nil || !network.Contains(addr) {
			log.Fatalf("alamat client %q harus berada di %s", m.Address, network)
		}
		members = append(members, relay.Member{PublicKey: pub, Address: addr})
	}

	pub, err := curve25519.X25519(key[:], curve25519.Basepoint)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("public key relay: %x", pub)
	log.Printf("relay berjalan di %s dengan %d client", *listen, len(members))
	log.Fatal(http.ListenAndServe(*listen, relay.New(key, network, members)))
}
```

Program ini memeriksa setiap alamat client, supaya tidak ada alamat di luar rentang VPN. Saat berjalan, program mencetak public key relay. Public key inilah yang perlu ditulis di konfigurasi setiap client. Alamat yang didengarkan kini bisa diatur dengan `-listen`, dan bawaannya `127.0.0.1:8080`, sesuai saran di Bagian 3 supaya relay hanya bisa dijangkau lewat Caddy.

## Mengubah Bind WebSocket di sisi client VPN

Di sisi client, Bind sekarang menyimpan private key kita dan public key relay, supaya bisa membuat bukti dan memeriksa bukti relay. Bagian yang berubah di `wsbind/wsbind.go` dimulai dari daftar import-nya. Isinya adalah:

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
// lalu menerima alamat dan daftar client.
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

// Welcome mengembalikan alamat dan daftar client dari relay.
func (b *Bind) Welcome() relay.Welcome {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.welcome
}
```

Sisanya, yaitu `Endpoint`, `Send`, `ParseEndpoint`, `Close`, `SetMark`, dan `BatchSize`, sama persis dengan Bagian 2. Perubahan di bagian ini ada tiga. Ketiganya adalah:

1. `Open` memanggil `join` sebelum Bind dipakai. Kalau relay menolak, atau relay gagal membuktikan dirinya, `Open` gagal dan wireguard-go tidak pernah mengirim paket lewat koneksi itu.
2. `join` membalik langkah-langkah di relay. Fungsi ini membaca nonce relay, mengirim public key, nonce client, dan bukti client, memeriksa bukti relay, lalu membaca pesan sambutan dengan `wsjson.Read`.
3. Fungsi penerima kini hanya memproses pesan binary. Pesan teks adalah pesan kontrol, jadi tidak pernah dianggap paket WireGuard.

## Mengubah program client VPN

File konfigurasi client sekarang jauh lebih pendek. Alamat VPN dan daftar peer tidak ada lagi di sini, karena keduanya datang dari relay. Isinya adalah:

```json
{
  "relay": "wss://relay.example.com",
  "relay_key": "isi dengan public key relay",
  "private_key": "isi dengan private key A"
}
```

Program client di `cmd/client/main.go` kini menyusun konfigurasi WireGuard dari pesan sambutan. File `os_linux.go` dan `os_windows.go` dari Bagian 3 tidak berubah. Kodenya adalah:

```go
// Program client menyalakan satu client VPN dengan virtual network card
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

// Config adalah isi file konfigurasi satu client. Alamat VPN dan daftar
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
	path := flag.String("config", "client.json", "file konfigurasi")
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

## Mencoba pemeriksaan client VPN

File test di `wsbind/wsbind_test.go` kini punya empat test. Keempatnya adalah:

- **TestClientTerdaftar.** Dua client terdaftar lolos pemeriksaan, mendapat alamat dan daftar peer dari relay, lalu saling bertukar data lewat tunnel di atas netstack, seperti di Bagian 2.
- **TestBukanClient.** Kunci yang tidak terdaftar ditolak dengan alasan "bukan client terdaftar".
- **TestPenyusup.** Penyusup yang mengaku memakai public key A, tetapi tidak memegang private key A, ditolak.
- **TestRelayPalsu.** Relay yang tidak memegang private key relay asli ditolak oleh client.

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

// Relay palsu, yang tidak memegang private key relay asli, ditolak client.
func TestRelayPalsu(t *testing.T) {
	_, relayPub := newKey(t)
	fakePriv, _ := newKey(t)
	privA, pubA := newKey(t)
	url := startRelay(t, fakePriv, pubA)

	bind := &wsbind.Bind{URL: url, PrivateKey: privA, RelayKey: relayPub}
	if _, _, err := bind.Open(0); err == nil {
		t.Fatal("client menerima relay palsu")
	}
}
```

`newKey` sama dengan di Bagian 2. `startRelay` menjalankan relay dengan client terdaftar, dan `member` menyalakan client di atas netstack lalu mengatur peer dari pesan sambutan, persis seperti program client. Hasil test-nya adalah:

```text
$ go test -v ./wsbind/
=== RUN   TestClientTerdaftar
--- PASS: TestClientTerdaftar (0.02s)
=== RUN   TestBukanClient
--- PASS: TestBukanClient (0.00s)
=== RUN   TestPenyusup
--- PASS: TestPenyusup (0.00s)
=== RUN   TestRelayPalsu
--- PASS: TestRelayPalsu (0.00s)
PASS
ok  	vpnku/wsbind	1.404s
```

Percobaan yang sama juga dijalankan dengan virtual network card sungguhan, memakai relay, Caddy, dua client terdaftar, dan satu orang asing, di dalam container pada satu mesin. Client terdaftar mendapat alamat dan peer dari relay, sedangkan orang asing ditolak. Hasilnya adalah:

```text
client A:    vpnku0 aktif dengan alamat 10.10.0.2/24 dan 1 peer. Tekan Ctrl+C untuk berhenti.
client B:    vpnku0 aktif dengan alamat 10.10.0.1/24 dan 1 peer. Tekan Ctrl+C untuk berhenti.
orang asing:  failed to get reader: received close frame: status = StatusPolicyViolation and reason = "bukan client terdaftar"
```

Setelah itu, A dan B saling ping lewat `10.10.0.x` tanpa satu pun daftar peer yang ditulis tangan.

## Kelemahan VPN versi keempat

Relay sekarang memeriksa client, dan client memeriksa relay. Tetapi masih ada empat kelemahan yang perlu kamu tahu. Keempatnya adalah:

- **Mengubah client butuh restart.** Menambah atau mengeluarkan client berarti mengubah `relay.json` lalu menjalankan ulang relay. Karena belum ada reconnect, semua client juga harus dijalankan ulang.
- **Daftar peer hanya dikirim sekali.** Pesan sambutan dikirim saat client terhubung. Perubahan sesudahnya tidak sampai ke client yang sedang berjalan.
- **Relay dipercaya menentukan client.** Siapa pun yang mencuri private key relay bisa menyamar sebagai relay dan menambahkan client palsu. Jadi private key relay harus dijaga seketat private key client. Cara yang lebih kuat adalah daftar client yang ditandatangani oleh kunci admin yang disimpan offline, tetapi seri ini belum sampai ke sana.
- **Satu client yang lambat masih bisa memperlambat pengirimnya.** Kelemahan dari Bagian 2 ini belum diperbaiki.

## Berikutnya dalam seri ini

Di Bagian 5, kita membuat VPN ini tahan banting. Client akan tersambung lagi sendiri saat koneksi putus atau relay restart. Relay akan mengirim perubahan daftar client ke semua client yang sedang terhubung, dan setiap client mendapat antrean sendiri di relay, sehingga client yang lambat tidak lagi menahan yang lain.
