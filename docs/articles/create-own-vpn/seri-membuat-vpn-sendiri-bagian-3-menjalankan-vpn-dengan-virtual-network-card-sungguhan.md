# [Seri Membuat VPN Sendiri Bagian 3] Menjalankan VPN dengan virtual network card sungguhan

*Di bagian ini, VPN kita keluar dari test dan berjalan di komputer sungguhan. Kita membuat TUN device di Linux dan Wintun di Windows, mengatur OS-nya, lalu menghubungkan dua komputer lewat relay di internet.*

Di Bagian 2, kita sudah menulis jalur pengiriman VPN. wireguard-go membawa paket lewat Bind WebSocket ke sebuah relay, dan relay meneruskannya ke anggota tujuan berdasarkan public key. Semuanya kita coba di dalam satu program memakai netstack, yaitu network stack yang berjalan di dalam program.

Di Bagian 3, kita mengganti netstack dengan virtual network card sungguhan. Di akhir bagian ini, dua komputer di tempat berbeda bisa saling ping lewat alamat VPN `10.10.0.x`, melalui relay di sebuah VPS. Kode dari Bagian 2, yaitu relay dan Bind WebSocket, dipakai lagi tanpa diubah.

## Apa bedanya netstack dengan virtual network card sungguhan?

netstack sangat praktis untuk mencoba, karena tidak butuh hak administrator. Tetapi netstack hanya bisa dipakai oleh program yang membuatnya. Aplikasi lain di komputer itu, seperti browser, SSH, atau remote desktop, tidak tahu netstack itu ada.

Virtual network card sungguhan adalah network card yang dikenal oleh OS. Setelah dibuat, semua aplikasi di komputer itu bisa memakai alamat VPN tanpa perlu diubah. Inilah yang kita butuhkan supaya VPN benar-benar berguna.

Pilihan ini punya harga. Harga-harga itu adalah:

- **Butuh hak administrator.** Membuat network card baru adalah urusan OS, jadi program anggota harus dijalankan sebagai administrator di Windows, atau dengan `sudo` di Linux.
- **OS harus diatur.** Network card baru belum punya alamat, belum menyala, dan belum punya route. Kita harus mengaturnya sendiri.
- **Setiap OS berbeda.** Linux dan Windows punya cara yang berbeda untuk membuat dan mengatur network card. Kodenya harus ditulis untuk masing-masing OS.

## Anatomi VPN di sisi OS

Supaya OS mau mengirim paket ke VPN, ada empat hal yang harus disiapkan. Keempatnya adalah:

- **TUN device** adalah virtual network card yang dibuat lewat wireguard-go. Di Linux, kernel menyediakannya langsung. Di Windows, wireguard-go memakai Wintun, yaitu driver TUN dari proyek WireGuard. Kita menamainya `vpnku0`.
- **Alamat dengan prefix.** TUN device diberi alamat VPN, misalnya `10.10.0.2/24`. Prefix `/24` artinya 24 bit pertama dari alamat itu adalah nomor jaringan, jadi seluruh alamat `10.10.0.0` sampai `10.10.0.255` dianggap satu jaringan.
- **Route.** Route adalah aturan yang memberi tahu OS lewat network card mana sebuah paket harus keluar. Karena alamatnya memakai prefix `/24`, OS otomatis membuat route untuk seluruh `10.10.0.0/24` lewat `vpnku0`.
- **MTU.** MTU adalah ukuran paket terbesar. Kita memakai 1420, seperti WireGuard, supaya paket yang sudah dibungkus tetap muat di jaringan biasa.

Di Windows, ada satu hal tambahan, yaitu firewall. Windows memasukkan network card baru ke profil Public, yaitu profil untuk jaringan yang belum dipercaya. Profil ini menolak semua koneksi masuk, termasuk ping. Karena itu, kita perlu satu aturan firewall yang mengizinkan paket dari anggota VPN lain.

![Di dalam komputer A, aplikasi mengirim ping ke 10.10.0.1. Route di OS mengarahkan paket ke 10.10.0.0/24 ke vpnku0, yaitu TUN device dengan alamat 10.10.0.2/24 dan MTU 1420, sedangkan paket lain langsung ke network card. Dari vpnku0, paket dibaca program anggota, yaitu wireguard-go dan Bind WebSocket, dibungkus, lalu dikirim sebagai pesan WebSocket lewat network card ke relay di internet.](assets/os-route-to-tun.png)

*Hanya paket ke 10.10.0.0/24 yang masuk ke VPN. Traffic internet lainnya tetap lewat jalan biasa.*

Perhatikan bahwa VPN ini hanya menangkap paket ke `10.10.0.0/24`. Membuka website tetap lewat jalan biasa, tidak lewat relay. Inilah bedanya dengan privacy VPN di Bagian 1, yang membawa semua traffic ke server provider-nya.

## Membuat kunci untuk setiap anggota VPN

Setiap anggota butuh sepasang kunci. Private key dirahasiakan dan hanya disimpan di komputer anggota itu, sedangkan public key dibagikan ke anggota lain. Program kecil di `cmd/genkey/main.go` membuatnya.

```go
// Program genkey membuat sepasang kunci WireGuard untuk satu anggota.
package main

import (
	"crypto/rand"
	"fmt"
	"log"

	"golang.org/x/crypto/curve25519"
)

func main() {
	var priv [32]byte
	if _, err := rand.Read(priv[:]); err != nil {
		log.Fatal(err)
	}
	priv[0] &= 248
	priv[31] = priv[31]&127 | 64
	pub, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("private_key: %x\npublic_key:  %x\n", priv, pub)
}
```

Program ini membuat 32 byte acak sebagai private key. Tiga bit di dalamnya lalu diatur, sesuai aturan kunci Curve25519 yang dipakai WireGuard. Public key dihitung dari private key dengan fungsi X25519. Hasilnya dicetak dalam hex, format yang sama dengan Bagian 2. Contoh hasilnya adalah:

```text
$ go run ./cmd/genkey
private_key: c057d3cb423870ed52376dd3a28f3609d754f2be1f70dbeb37325738fd08c566
public_key:  7b4423ef95851316dad4166a3c7078827f22bbe8178be7633b21e6103bed7b71
```

Jalankan program ini sekali untuk setiap anggota. Jangan pakai kunci contoh di atas, karena private key-nya sudah tidak rahasia.

## Menulis program anggota VPN

Program anggota menyatukan semua bagian. Program ini membaca file konfigurasi, membuat TUN device, menyalakan wireguard-go dengan Bind WebSocket dari Bagian 2, lalu mengatur OS. File konfigurasinya berbentuk JSON. Contoh untuk anggota A adalah:

```json
{
  "relay": "wss://relay.example.com",
  "private_key": "isi dengan private key A",
  "address": "10.10.0.2/24",
  "peers": [
    {"public_key": "isi dengan public key B", "address": "10.10.0.1"}
  ]
}
```

`relay` adalah alamat relay. `address` adalah alamat VPN anggota ini, lengkap dengan prefix-nya. `peers` adalah daftar anggota lain yang boleh dihubungi, masing-masing dengan public key dan alamat VPN-nya. Kodenya ada di `cmd/anggota/main.go`.

```go
// Program anggota menyalakan satu anggota VPN dengan virtual network card
// sungguhan.
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"golang.org/x/crypto/curve25519"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"

	"vpnku/wsbind"
)

// Config adalah isi file konfigurasi satu anggota.
type Config struct {
	Relay      string `json:"relay"`       // misalnya wss://relay.example.com
	PrivateKey string `json:"private_key"` // hex, dari program genkey
	Address    string `json:"address"`     // alamat VPN anggota ini, misalnya 10.10.0.2/24
	Peers      []Peer `json:"peers"`
}

type Peer struct {
	PublicKey string `json:"public_key"` // hex
	Address   string `json:"address"`    // alamat VPN peer, misalnya 10.10.0.1
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

	// Public key kita dihitung dari private key.
	priv, err := hex.DecodeString(cfg.PrivateKey)
	if err != nil || len(priv) != 32 {
		log.Fatal("private_key harus 32 byte dalam hex")
	}
	pub, err := curve25519.X25519(priv, curve25519.Basepoint)
	if err != nil {
		log.Fatal(err)
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

	// 2. WireGuard dengan Bind WebSocket dari Bagian 2.
	bind := &wsbind.Bind{URL: cfg.Relay}
	copy(bind.Key[:], pub)
	dev := device.NewDevice(tunDev, bind, device.NewLogger(device.LogLevelError, ""))
	defer dev.Close()

	var conf strings.Builder
	fmt.Fprintf(&conf, "private_key=%s\n", cfg.PrivateKey)
	for _, p := range cfg.Peers {
		fmt.Fprintf(&conf, "public_key=%s\nendpoint=%s\nallowed_ip=%s/32\n",
			p.PublicKey, p.PublicKey, p.Address)
	}
	if err := dev.IpcSet(conf.String()); err != nil {
		log.Fatal(err)
	}
	if err := dev.Up(); err != nil {
		log.Fatal(err)
	}

	// 3. Beri tahu OS alamat VPN kita.
	if err := setupOS(name, cfg.Address); err != nil {
		log.Fatal(err)
	}
	log.Printf("%s aktif dengan alamat %s. Tekan Ctrl+C untuk berhenti.", name, cfg.Address)

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

Program ini bekerja dalam lima langkah. Langkah-langkahnya adalah:

1. File konfigurasi dibaca, lalu public key kita dihitung dari private key. Dengan begitu, file konfigurasi cukup menyimpan private key.
2. `tun.CreateTUN` membuat TUN device bernama `vpnku0` dengan MTU 1420. Di sinilah hak administrator dibutuhkan.
3. wireguard-go dinyalakan dengan Bind WebSocket dari Bagian 2. Setiap peer diatur lewat `IpcSet`, sama seperti di test Bagian 2. Bedanya, kali ini daftar peer diambil dari file konfigurasi.
4. `setupOS` memberi tahu OS alamat VPN kita. Isinya berbeda untuk setiap OS, dan dibahas di bawah.
5. Program menunggu sampai kamu menekan Ctrl+C. Saat program berhenti, wireguard-go ditutup dan TUN device-nya ikut hilang.

`setupOS` ditulis dua kali, di file `os_linux.go` dan `os_windows.go`. Go memilih file berdasarkan akhiran namanya. Saat membangun program untuk Linux, Go hanya memakai `os_linux.go`, dan saat membangun untuk Windows, Go hanya memakai `os_windows.go`.

### Pengaturan OS untuk VPN di Linux

Di Linux, TUN device yang baru dibuat belum punya alamat dan masih mati. Perintah untuk memberinya alamat, lalu menyalakannya, adalah:

```sh
sudo ip addr add 10.10.0.2/24 dev vpnku0
sudo ip link set vpnku0 up
```

Tapi kenapa tidak menyuruh pengguna mengetik perintah ini sendiri? Mengetik sendiri memang membuat kita paham apa yang terjadi. Masalahnya, perintah itu harus diulang setiap kali program dijalankan, karena TUN device hilang saat program berhenti. Jadi program anggota menjalankan perintah yang sama lewat `run`. Kodenya ada di `cmd/anggota/os_linux.go`.

```go
package main

// setupOS memberi alamat ke TUN device, lalu menyalakannya. Karena alamatnya
// memakai prefix /24, Linux otomatis menambahkan route ke seluruh jaringan
// VPN lewat device ini.
func setupOS(name, address string) error {
	return run(
		[]string{"ip", "addr", "add", address, "dev", name},
		[]string{"ip", "link", "set", name, "up"},
	)
}
```

Menjalankan perintah `ip` dari program punya harga. Program jadi bergantung pada perintah `ip` yang terpasang di OS. Program yang lebih matang memanggil OS secara langsung, misalnya lewat netlink, yaitu cara program di Linux berbicara dengan bagian jaringan di kernel. Untuk seri ini, perintah `ip` lebih mudah dibaca dan diperiksa.

### Pengaturan OS untuk VPN di Windows

Di Windows, wireguard-go membuat TUN device lewat Wintun. Wintun butuh file `wintun.dll`. Unduh dari situs wintun.net, lalu ambil file `bin/amd64/wintun.dll` dari dalam zip-nya. Simpan file itu di folder yang sama dengan `anggota.exe`. Lisensi Wintun mengizinkan file DLL itu disertakan tanpa diubah bersama program yang memakainya.

Setelah adapter dibuat, Windows butuh tiga pengaturan. Perintah-perintahnya adalah:

```text
netsh interface ipv4 set address name=vpnku0 source=static address=10.10.0.2 mask=255.255.255.0
netsh interface ipv4 set subinterface vpnku0 mtu=1420 store=active
netsh advfirewall firewall add rule name=vpnku dir=in action=allow remoteip=10.10.0.0/24
```

Perintah pertama memberi alamat. Mask `255.255.255.0` adalah cara Windows menulis prefix `/24`, dan Windows juga otomatis membuat route-nya. Perintah kedua mengatur MTU. Perintah ketiga membuka firewall untuk paket dari `10.10.0.0/24`. Kodenya ada di `cmd/anggota/os_windows.go`.

```go
package main

import (
	"fmt"
	"net"
	"net/netip"
	"os/exec"
)

// setupOS memberi alamat dan MTU ke adapter Wintun, lalu membuka firewall
// untuk paket dari anggota lain. Windows memasukkan adapter baru ke profil
// Public, yang menolak semua koneksi masuk.
func setupOS(name, address string) error {
	prefix, err := netip.ParsePrefix(address)
	if err != nil {
		return err
	}
	mask := net.IP(net.CIDRMask(prefix.Bits(), 32)).String()

	// Hapus aturan lama dari jalannya program sebelumnya, kalau ada.
	exec.Command("netsh", "advfirewall", "firewall", "delete", "rule", "name=vpnku").Run()

	return run(
		[]string{"netsh", "interface", "ipv4", "set", "address", "name=" + name,
			"source=static", "address=" + prefix.Addr().String(), "mask=" + mask},
		[]string{"netsh", "interface", "ipv4", "set", "subinterface", name,
			fmt.Sprintf("mtu=%d", mtu), "store=active"},
		[]string{"netsh", "advfirewall", "firewall", "add", "rule", "name=vpnku",
			"dir=in", "action=allow", "remoteip=" + prefix.Masked().String()},
	)
}
```

Aturan firewall lama dihapus dulu, karena tanpa itu setiap kali program dijalankan akan menambah aturan yang sama. Aturan ini mengizinkan semua jenis paket dari anggota VPN. Artinya, kita percaya pada semua anggota. Kalau tidak, aturannya bisa dipersempit, misalnya hanya untuk port remote desktop.

## Menyiapkan relay VPN di VPS dengan HTTPS

Relay dari Bagian 2 sekarang perlu berjalan di internet. Kita memakai sebuah VPS dengan IP publik dan sebuah domain, misalnya `relay.example.com`. Domain itu harus mengarah ke IP publik VPS.

Di Bagian 2, kita sudah sepakat bahwa relay harus berada di belakang HTTPS. Caranya adalah dengan Caddy, sebuah web server yang bisa menjadi reverse proxy dan otomatis mengambil sertifikat HTTPS untuk domain kita. Sertifikat HTTPS adalah bukti dari pihak tepercaya bahwa server itu benar pemilik domainnya. Caddy juga meneruskan koneksi WebSocket tanpa pengaturan tambahan. Pengaturannya cukup satu file bernama `Caddyfile`. Isinya adalah:

```text
relay.example.com {
	reverse_proxy localhost:8080
}
```

Ada satu perubahan kecil di `cmd/relay/main.go`. Ganti alamat `":8080"` menjadi `"127.0.0.1:8080"`, supaya relay hanya bisa dijangkau lewat Caddy, bukan langsung dari internet. Setelah itu, program untuk VPS bisa dibangun dari komputermu sendiri. Perintahnya adalah:

```sh
GOOS=linux GOARCH=amd64 go build -o relay ./cmd/relay
```

`GOOS` dan `GOARCH` memberi tahu Go untuk membuat program bagi Linux 64-bit, walaupun komputermu memakai OS lain. Salin file `relay` ke VPS, jalankan, lalu jalankan Caddy di folder yang berisi `Caddyfile` dengan perintah `caddy run`. Pastikan port 80 dan 443 di VPS terbuka, karena Caddy memakainya untuk HTTPS dan untuk mengambil sertifikat.

## Mencoba dua komputer lewat VPN

Sekarang semua bagian sudah siap. Langkah-langkah untuk menghubungkan dua komputer adalah:

1. Jalankan `go run ./cmd/genkey` dua kali, satu untuk anggota A dan satu untuk anggota B.
2. Buat `a.json` untuk A dengan alamat `10.10.0.2/24`, dan `b.json` untuk B dengan alamat `10.10.0.1/24`. Masing-masing berisi private key-nya sendiri, dan public key anggota lainnya di `peers`.
3. Bangun program anggota. Untuk Linux, pakai `GOOS=linux GOARCH=amd64 go build -o anggota ./cmd/anggota`. Untuk Windows, pakai `GOOS=windows GOARCH=amd64 go build -o anggota.exe ./cmd/anggota`, lalu letakkan `wintun.dll` di sebelahnya.
4. Di Linux, jalankan `sudo ./anggota -config a.json`. Di Windows, buka terminal sebagai administrator, lalu jalankan `anggota.exe -config b.json`.
5. Dari komputer A, jalankan `ping 10.10.0.1`.

Sebelum ping, kamu bisa memeriksa hasil `setupOS` di Linux dengan `ip addr show vpnku0` dan `ip route`. Hasilnya menunjukkan alamat, MTU 1420, dan route ke `10.10.0.0/24` yang dibuat otomatis. Contohnya adalah:

```text
3: vpnku0: <POINTOPOINT,MULTICAST,NOARP,UP,LOWER_UP> mtu 1420 qdisc fq_codel state UNKNOWN qlen 500
    inet 10.10.0.2/24 scope global vpnku0
10.10.0.0/24 dev vpnku0 scope link  src 10.10.0.2
```

Hasil ping di bawah berasal dari percobaan dengan dua komputer Linux, relay, dan Caddy di dalam container pada satu mesin. Lewat internet, angka waktunya akan lebih besar, karena setiap paket menempuh perjalanan ke relay lalu kembali lagi.

```text
--- 10.10.0.1 ping statistics ---
3 packets transmitted, 3 packets received, 0% packet loss
round-trip min/avg/max = 1.943/3.668/4.591 ms
```

Kalau ping berhasil, aplikasi apa pun di A kini bisa menjangkau B di `10.10.0.1`. Seperti SSH, remote desktop, atau database.

## Harga dan batasan VPN versi ketiga

VPN ini sudah berjalan di komputer sungguhan, tetapi masih punya batasan. Batasan-batasan itu adalah:

- **Daftar peer ditulis tangan.** Setiap anggota harus mencantumkan semua anggota lain di file konfigurasinya. Menambah satu anggota berarti mengubah file di semua komputer.
- **Relay masih percaya begitu saja pada pesan perkenalan.** Batasan dari Bagian 2 ini belum diperbaiki. Isi paket tetap aman, tetapi orang yang tahu public key seorang anggota bisa mengganggu pengirimannya.
- **Koneksi yang putus tidak tersambung lagi.** Kalau relay restart, ping berhenti berhasil. Untuk sementara, jalankan ulang program anggota di setiap komputer.
- **Program berjalan di terminal.** VPN mati saat terminalnya ditutup, dan tidak menyala sendiri saat komputer dinyalakan.

## Berikutnya dalam seri ini

Di Bagian 4, kita menutup dua batasan pertama. Relay akan meminta bukti bahwa setiap anggota benar-benar memegang private key dari public key yang ia akui. Setelah itu, relay membagikan daftar anggota ke semua anggota, sehingga daftar peer tidak perlu lagi ditulis tangan.
