# [Seri Membuat VPN Sendiri Bagian 9] Merawat VPN dan menutup seri

*Di bagian terakhir ini, VPN kita menyala sendiri saat komputer menyala, menjadi satu file untuk setiap OS, dan bisa diperbarui dengan aman. Lalu kita melihat kembali kelemahan dari seluruh desainnya, dan kapan sebaiknya memilih alat lain.*

Delapan bagian sebelumnya membangun VPN yang lengkap. Client saling menjangkau lewat nama, bergabung dengan undangan, tersambung lagi sendiri, dan relay-nya bisa berjalan di tiga tempat. Tetapi program client masih harus dijalankan dengan tangan di terminal, seperti yang kita catat di daftar kelemahan Bagian 3.

Di Bagian 9, kita menutup urusan perawatan itu. Program client menyala sendiri saat komputer menyala dan menunggu relay kalau belum siap, `wintun.dll` masuk ke dalam program, dan program bisa memperbarui dirinya sendiri. Setelah itu, kita melihat kembali seluruh seri ini.

## Kenapa VPN harus menyala sendiri?

VPN baru berguna kalau sedang menyala. Bayangkan PC kantor yang menyala ulang setelah listrik padam atau setelah pembaruan Windows. Kalau tidak ada yang menjalankan program client, PC itu tidak bisa dijangkau tepat saat kamu membutuhkannya dari rumah.

Ada dua cara untuk menjalankan program saat komputer menyala. Keduanya adalah:

- **Jalankan saat pengguna login.** Seperti shortcut di folder Startup Windows. Masalahnya, PC kantor yang menyala ulang berhenti di layar login, dan program tidak pernah jalan sampai seseorang login.
- **Jalankan oleh pengelola layanan milik OS.** Di Linux, pengelolanya adalah systemd, dan di Windows, Task Scheduler bisa menjalankan program saat komputer menyala. Program berjalan tanpa ada yang login, sebagai root atau sebagai akun SYSTEM.

Seri ini memilih cara kedua, karena hanya cara ini yang bekerja tanpa ada orang di depan komputer. Kelemahannya, program berjalan dengan hak tertinggi di OS, jadi file konfigurasinya, yang berisi private key, harus disimpan di folder yang hanya bisa dibaca administrator.

### Program client menunggu relay saat komputer menyala

Di Bagian 5, percobaan pertama ke relay tidak diulang, supaya client yang ditolak langsung melihat alasannya. Tetapi saat komputer baru menyala, jaringan sering belum siap, dan program akan langsung berhenti. Kita bisa menyerahkan urusan ini ke pengelola layanan, yang menjalankan ulang program yang berhenti, atau membuat program client menunggu sendiri.

Seri ini melakukan keduanya, dengan satu pengecualian. Program client terus mencoba dengan jeda yang makin panjang, seperti di Bagian 5, kecuali kalau relay menolaknya. Penolakan tidak akan berubah dengan mencoba lagi, jadi program berhenti dan mencetak alasannya.

Untuk membedakan keduanya, Bind mendapat fungsi kecil `Rejected`. Relay menutup client yang ditolak dengan status `StatusPolicyViolation` sejak Bagian 4, jadi status itulah yang diperiksa. Kodenya, di `wsbind/wsbind.go`, adalah:

```go
// Rejected memeriksa apakah error dari Open berarti relay menolak kita, bukan
// relay yang belum bisa dihubungi.
func Rejected(err error) bool {
	return websocket.CloseStatus(err) == websocket.StatusPolicyViolation
}
```

Di `cmd/client/main.go`, `dev.Up()` kini dipanggil berulang kali. wireguard-go mengembalikan device ke keadaan mati kalau `Up` gagal, jadi `Up` aman dipanggil lagi. Kodenya adalah:

```go
	// Saat komputer baru menyala, jaringan atau relay mungkin belum siap.
	// Jadi program terus mencoba, kecuali relay menolak kita.
	for wait := time.Second; ; wait = min(wait*2, 30*time.Second) {
		err := dev.Up()
		if err == nil {
			break
		}
		if wsbind.Rejected(err) {
			log.Fatal(err)
		}
		log.Printf("relay belum bisa dihubungi, coba lagi dalam %v: %v", wait, err)
		time.Sleep(wait)
	}
```

Kelemahannya, alamat relay yang salah ketik membuat program menunggu selamanya, bukan berhenti. Alasannya tetap tercetak di log setiap kali percobaan gagal.

### Menjalankan client VPN sebagai layanan di Linux

Di Linux, systemd membaca file unit, yaitu file teks yang menjelaskan cara menjalankan sebuah layanan. Simpan file ini sebagai `/etc/systemd/system/vpnku.service`. Isinya adalah:

```ini
[Unit]
Description=Client VPN vpnku
Wants=network-online.target
After=network-online.target

[Service]
ExecStart=/usr/local/bin/client -config /etc/vpnku/client.json
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Setiap baris punya tugasnya sendiri. Tugas-tugas itu adalah:

- **Wants dan After `network-online.target`** meminta systemd menunggu jaringan menyala sebelum menjalankan program. Dokumentasi systemd menyarankan keduanya dipakai bersama.
- **Restart=always** menjalankan ulang program setiap kali berhenti, apa pun alasannya, dan **RestartSec=5** memberi jeda lima detik sebelum menjalankannya lagi.
- **WantedBy=multi-user.target** membuat layanan ini ikut menyala saat komputer menyala dalam mode biasa.

Lalu salin program untuk OS itu sebagai `/usr/local/bin/client`, pindahkan konfigurasinya, dan nyalakan layanannya. Perintahnya adalah:

```sh
sudo cp rilis/client-linux-amd64 /usr/local/bin/client
sudo mkdir -p /etc/vpnku
sudo mv client.json /etc/vpnku/
sudo chmod 700 /etc/vpnku
sudo systemctl enable --now vpnku
journalctl -u vpnku -f
```

`systemctl enable --now` menyalakan layanan sekarang dan setiap kali komputer menyala, dan `journalctl -u vpnku -f` menampilkan log-nya. Karena `Restart=always`, client yang ditolak relay akan dijalankan ulang setiap lima detik dan ditolak lagi. Hentikan dengan `sudo systemctl disable --now vpnku` sampai masalahnya selesai.

### Menjalankan client VPN saat Windows menyala

Di Windows, salin `client-windows-amd64.exe` sebagai `C:\vpnku\client.exe`, bersama `client.json`. Task Scheduler bisa menjalankannya saat komputer menyala, sebagai akun SYSTEM, dan perintahnya, di terminal administrator, adalah:

```text
schtasks /create /tn vpnku /tr "C:\vpnku\client.exe -config C:\vpnku\client.json" /sc onstart /ru System
```

`/sc onstart` berarti setiap kali komputer menyala, dan `/ru System` berarti sebagai akun SYSTEM, yang tidak butuh password dan tidak butuh siapa pun login. Hanya administrator yang boleh membuat task seperti ini.

Perintah ini tidak mengatur apa yang terjadi kalau program berhenti di tengah jalan. Program client sudah menunggu relay sendiri, jadi yang tersisa hanya program yang benar-benar rusak. Pastikan juga folder `C:\vpnku` hanya bisa dibaca administrator, karena `client.json` berisi private key.

## Satu file program untuk setiap OS

Di Bagian 3, `wintun.dll` diunduh terpisah dan diletakkan di sebelah `client.exe`. Dua file yang harus selalu bersama mudah terpisah, dan program tanpa DLL itu gagal membuat TUN device. Tiga cara untuk mengatasinya adalah:

- **Installer**, yaitu program pemasang yang menaruh semua file di tempatnya. Cara ini rapi, tetapi membuat installer adalah pekerjaan tersendiri.
- **Zip berisi dua file.** Cara ini mudah, tetapi kedua file tetap bisa terpisah setelah zip dibuka.
- **DLL di dalam program.** Go bisa menyimpan file apa pun di dalam program dengan `go:embed`. Saat mulai, program menulis DLL itu di sebelah dirinya sendiri.

Seri ini memilih cara ketiga. Lisensi Wintun mengizinkan DLL-nya disebarkan bersama program yang memakainya lewat API Wintun, yaitu daftar fungsi yang disediakan Wintun untuk program lain, asalkan DLL-nya tidak diubah. Kelemahannya, program menjadi sekitar 427 KB lebih besar, dan folder program harus bisa ditulis saat program mulai, yang selalu benar untuk akun SYSTEM.

Unduh zip Wintun dari situs wintun.net, lalu simpan file `bin/amd64/wintun.dll` di folder `cmd/client`, sebelum membangun program untuk Windows. Di `cmd/client/os_windows.go`, DLL itu disimpan di sebuah variabel. Kodenya adalah:

```go
// wintunDLL adalah isi wintun.dll dari wintun.net, yang disimpan di folder
// cmd/client sebelum program dibangun.
//
//go:embed wintun.dll
var wintunDLL []byte
```

`//go:embed` membutuhkan import `embed`, yang ditulis sebagai `_ "embed"` karena tidak ada fungsinya yang dipanggil langsung. Fungsi yang menulis DLL itu, di file yang sama, adalah:

```go
// installWintun menulis wintun.dll di sebelah program, karena Windows mencari
// DLL di folder program. File yang isinya sudah sama tidak ditulis ulang.
func installWintun() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	path := filepath.Join(filepath.Dir(exe), "wintun.dll")
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, wintunDLL) {
		return nil
	}
	return os.WriteFile(path, wintunDLL, 0o644)
}
```

Di Linux, TUN device disediakan kernel, jadi `os_linux.go` cukup berisi fungsi kosong. Isinya adalah:

```go
// installWintun tidak dibutuhkan di Linux, karena TUN device disediakan
// kernel.
func installWintun() error { return nil }
```

Program client memanggil `installWintun` tepat sebelum membuat TUN device, karena wireguard-go baru memuat `wintun.dll` saat itu. Kodenya adalah:

```go
	// 1. Virtual network card sungguhan. Di Windows, wintun.dll ditulis dulu
	// di sebelah program.
	if err := installWintun(); err != nil {
		log.Fatal(err)
	}
	tunDev, err := tun.CreateTUN(ifName, mtu)
	if err != nil {
		log.Fatalf("membuat TUN device: %v (sudah dijalankan sebagai administrator?)", err)
	}
```

`os_windows.go` hanya dipakai saat membangun untuk Windows. Jadi program untuk Linux tetap bisa dibangun tanpa `wintun.dll`.

### Program client VPN untuk Raspberry Pi

Raspberry Pi memakai prosesor ARM, bukan prosesor seperti di PC biasa, dan Go bisa membangun program untuknya dari komputer mana pun dengan `GOARCH`. Ada dua pilihan. Keduanya adalah:

- **`GOARCH=arm64`** untuk Raspberry Pi dengan OS 64-bit, yang menjawab `aarch64` saat ditanya dengan `uname -m`.
- **`GOARCH=arm` dengan `GOARM=6`** untuk Raspberry Pi 1 dan Zero, yang menjawab `armv6l`. Keduanya memakai prosesor ARMv6, dan Go wiki memetakan ARMv6 ke `GOARM=6`.

Nama client dari Bagian 7 butuh systemd-resolved. Kalau OS di Raspberry Pi-mu tidak memakainya, program mencetak alasannya, dan VPN tetap berjalan lewat alamat IP.

### Membangun semua program sekaligus

Semua program dibangun dari satu komputer, lalu hash setiap file dicatat di `SHA256SUMS`. Hash ini akan dipakai untuk pembaruan di bawah. Perintahnya adalah:

```sh
export CGO_ENABLED=0
V="-ldflags=-X main.version=v1.0.0"
mkdir -p rilis
GOOS=linux GOARCH=amd64 go build "$V" -o rilis/client-linux-amd64 ./cmd/client
GOOS=linux GOARCH=arm64 go build "$V" -o rilis/client-linux-arm64 ./cmd/client
GOOS=linux GOARCH=arm GOARM=6 go build "$V" -o rilis/client-linux-arm ./cmd/client
GOOS=windows GOARCH=amd64 go build "$V" -o rilis/client-windows-amd64.exe ./cmd/client
cd rilis && sha256sum client-* > SHA256SUMS
```

`CGO_ENABLED=0` memastikan program tidak bergantung pada library C di komputer tujuan. `-X main.version=v1.0.0` mengisi variabel `version` di program client saat dibangun, supaya `client -version` menjawab versi yang benar. Di `cmd/client/main.go`, variabel itu dan perintah barunya adalah:

```go
// version diisi saat program dibangun, dengan -ldflags "-X main.version=v1.0.0".
var version = "dev"

func main() {
	path := flag.String("config", "client.json", "file konfigurasi")
	showVersion := flag.Bool("version", false, "tampilkan versi program ini")
	flag.Parse()
	switch flag.Arg(0) {
	case "gabung":
		if err := join(*path, flag.Arg(1)); err != nil {
			log.Fatal(err)
		}
		return
	case "perbarui":
		if err := selfUpdate(flag.Arg(1)); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *showVersion {
		fmt.Println(version)
		return
	}
```

## Memperbarui program client dengan aman

Saat versi baru keluar, program di setiap komputer harus diganti. Mengganti dengan tangan berarti menyalin file yang tepat untuk setiap OS ke setiap komputer. Di Windows, file program yang sedang berjalan juga tidak bisa ditimpa.

Ada tiga cara yang umum. Ketiganya adalah:

- **Menyalin dengan tangan.** Cara ini sederhana untuk dua komputer, tetapi melelahkan untuk sepuluh.
- **Pengelola paket milik OS**, seperti repository apt di Linux atau winget di Windows. Cara ini rapi, tetapi setiap OS butuh pengaturan sendiri, dan menyiapkannya adalah pekerjaan besar.
- **Program memperbarui dirinya sendiri** dari alamat yang kamu kendalikan, setelah memeriksa hash file yang diunduh.

Seri ini memilih cara ketiga. Admin cukup menaruh isi folder `rilis` di web server mana pun yang memakai HTTPS, lalu menjalankan `sudo client perbarui https://example.com/vpnku/v1.1.0` di setiap komputer, atau perintah yang sama di terminal administrator di Windows.

### Bukankah hash dari server yang sama tidak membuktikan apa-apa?

Pemeriksaan hash membuktikan bahwa file yang diunduh sama persis dengan file yang dicatat di `SHA256SUMS`. File yang rusak di jalan, terpotong, atau salah versi akan ditolak. Tetapi `SHA256SUMS` diunduh dari server yang sama dengan programnya.

Orang yang berhasil menguasai server itu bisa mengganti program dan `SHA256SUMS` sekaligus, dan hash-nya akan tetap cocok. Jadi hash di sini melindungi dari kerusakan, bukan dari server yang dikuasai orang lain. Perlindungan yang lebih kuat adalah digital signature, yaitu tanda tangan dari kunci admin yang disimpan offline, seperti yang disinggung di daftar kelemahan Bagian 4.

Jadi cara ini cocok kalau server rilisnya milikmu sendiri dan dijaga sama baiknya dengan relay. Cara ini kurang cocok untuk program yang dipakai banyak orang yang tidak mengenalmu.

### Mengganti program yang sedang berjalan

Windows tidak mengizinkan file program yang sedang berjalan ditimpa atau dihapus, tetapi mengizinkannya diganti namanya. Jadi program lama diberi nama `.old` lebih dulu, lalu program baru mengambil nama aslinya. Di Linux, cara yang sama juga berhasil, dan file `.old` bisa langsung dihapus.

Kodenya ada di package baru, `update/update.go`. Isinya adalah:

```go
// Package update mengunduh versi baru program client, memeriksa hash-nya,
// lalu mengganti file program.
package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
)

// FileName adalah nama file program untuk OS dan prosesor komputer ini,
// misalnya client-linux-arm64 atau client-windows-amd64.exe.
func FileName() string {
	name := "client-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// Download mengambil SHA256SUMS dan file name dari base, lalu memastikan hash
// file itu sama dengan yang tertulis di SHA256SUMS.
func Download(ctx context.Context, client *http.Client, base, name string) ([]byte, error) {
	if !strings.HasPrefix(base, "https://") {
		return nil, errors.New("alamat pembaruan harus diawali https://")
	}
	base = strings.TrimSuffix(base, "/")
	sums, err := get(ctx, client, base+"/SHA256SUMS")
	if err != nil {
		return nil, err
	}
	want := ""
	for _, line := range strings.Split(string(sums), "\n") {
		// Setiap baris berisi hash, spasi, lalu nama file, seperti hasil sha256sum.
		f := strings.Fields(line)
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == name {
			want = f[0]
		}
	}
	if want == "" {
		return nil, fmt.Errorf("%s tidak tercantum di SHA256SUMS", name)
	}
	data, err := get(ctx, client, base+"/"+name)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != want {
		return nil, fmt.Errorf("hash %s tidak cocok dengan SHA256SUMS", name)
	}
	return data, nil
}

func get(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 100<<20)) // paling besar 100 MB
}

// Replace mengganti file program di path dengan isi baru. Windows tidak
// mengizinkan file program yang sedang berjalan ditimpa atau dihapus, tetapi
// mengizinkannya diganti namanya. Jadi file lama diberi nama .old lebih dulu,
// lalu file baru mengambil nama aslinya.
func Replace(path string, data []byte) error {
	old, tmp := path+".old", path+".new"
	os.Remove(old) // sisa pembaruan sebelumnya, kalau programnya sudah berhenti
	if err := os.WriteFile(tmp, data, 0o755); err != nil {
		return err
	}
	if err := os.Rename(path, old); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Rename(old, path) // kembalikan file lama
		return err
	}
	os.Remove(old) // berhasil di Linux, gagal di Windows selama program lama masih berjalan
	return nil
}
```

Package ini punya tiga fungsi. Ketiganya adalah:

1. `FileName` menyusun nama file untuk OS dan prosesor komputer ini dari `runtime.GOOS` dan `runtime.GOARCH`, dengan aturan nama yang sama seperti perintah build di atas.
2. `Download` menolak alamat tanpa HTTPS, mengunduh `SHA256SUMS`, mencari baris untuk file itu, mengunduh filenya, lalu membandingkan hash-nya. Tanda `*` di depan nama file dibuang, karena `sha256sum` di beberapa OS menulisnya untuk file binary.
3. `Replace` menulis program baru sebagai `.new`, mengganti nama program lama menjadi `.old`, lalu mengganti nama `.new` menjadi nama aslinya. Kalau langkah terakhir gagal, program lama dikembalikan.

Perintah `perbarui` di program client ada di file baru, `cmd/client/update.go`. Kodenya adalah:

```go
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"vpnku/update"
)

// selfUpdate mengganti program ini dengan versi baru dari base, misalnya
// https://example.com/vpnku/v1.1.0, setelah hash-nya diperiksa. Program yang
// sedang berjalan tetap memakai versi lama sampai dijalankan ulang.
func selfUpdate(base string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	data, err := update.Download(ctx, http.DefaultClient, base, update.FileName())
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := update.Replace(exe, data); err != nil {
		return err
	}
	fmt.Printf("%s diperbarui dari versi %s. Jalankan ulang programnya untuk memakai versi baru.\n", exe, version)
	return nil
}
```

Program yang sedang berjalan sebagai layanan tetap memakai versi lama sampai dijalankan ulang. Di Linux, jalankan `sudo systemctl restart vpnku`. Di Windows, jalankan ulang task-nya atau nyalakan ulang komputernya.

## Mencoba bagian kesembilan

Test di bagian ini menguji dua hal, yaitu menunggu relay saat mulai, dan pembaruan program. Test pertama ada di `wsbind/mulai_test.go`. Kodenya adalah:

```go
// Client yang mulai sebelum relay hidup gagal menyala, tetapi bukan karena
// ditolak, jadi program boleh mencoba lagi. Begitu relay hidup, percobaan
// berikutnya berhasil. Kunci yang tidak terdaftar ditolak.
func TestMulaiSebelumRelay(t *testing.T) {
	relayPriv, relayPub := newKey(t)
	privA, pubA := newKey(t)
	strangerPriv, _ := newKey(t)

	// Alamat relay dipesan dulu, tetapi relay belum mendengarkan.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	url := "ws://" + addr

	devA := newDevice(t, url, privA, relayPub)
	err = devA.Up()
	if err == nil || wsbind.Rejected(err) {
		t.Fatalf("relay mati seharusnya bukan penolakan: %v", err)
	}

	// Relay hidup di alamat itu.
	ln, err = net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	r := relay.New(relayPriv, netip.MustParsePrefix("10.10.0.0/24"), "vpn.internal", members(pubA))
	srv := httptest.NewUnstartedServer(r)
	srv.Listener.Close()
	srv.Listener = ln
	srv.Start()
	t.Cleanup(srv.Close)

	if err := devA.Up(); err != nil {
		t.Fatalf("percobaan kedua gagal: %v", err)
	}
	if err := newDevice(t, url, strangerPriv, relayPub).Up(); !wsbind.Rejected(err) {
		t.Fatalf("orang asing seharusnya ditolak: %v", err)
	}
}
```

`newDevice` membuat device wireguard-go di atas netstack tanpa menyalakannya. Test ini membuktikan tiga hal yang diandalkan program client. Ketiganya adalah `Up` yang gagal karena relay mati tidak dianggap penolakan, `Up` berhasil saat dicoba lagi setelah relay hidup, dan kunci yang tidak terdaftar dianggap penolakan.

Package `update` punya empat test, yaitu `TestUnduh`, `TestUnduhRusak`, `TestBukanHTTPS`, dan `TestGanti`. Semua test itu memakai server HTTPS kecil yang meniru folder `rilis`. Kodenya adalah:

```go
// startRelease menjalankan server HTTPS berisi satu file program dan
// SHA256SUMS. sumOf adalah isi yang hash-nya ditulis di SHA256SUMS.
func startRelease(t *testing.T, program, sumOf string) *httptest.Server {
	sums := fmt.Sprintf("%x  client-test\n", sha256.Sum256([]byte(sumOf)))
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/SHA256SUMS", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, sums) })
	mux.HandleFunc("/v2/client-test", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, program) })
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	return srv
}
```

`TestUnduhRusak` menyajikan program yang berbeda dari yang dicatat di `SHA256SUMS`, seperti server yang disusupi di tengah jalan. Kodenya adalah:

```go
// File yang berubah di jalan, atau di server, ditolak.
func TestUnduhRusak(t *testing.T) {
	srv := startRelease(t, "program yang disusupi", "program baru")
	_, err := Download(context.Background(), srv.Client(), srv.URL+"/v2", "client-test")
	if err == nil || !strings.Contains(err.Error(), "tidak cocok") {
		t.Fatalf("file rusak diterima: %v", err)
	}
}
```

Hasil test-nya adalah:

```text
$ go test -v -run TestMulaiSebelumRelay ./wsbind/
=== RUN   TestMulaiSebelumRelay
--- PASS: TestMulaiSebelumRelay (0.01s)
PASS
ok  	vpnku/wsbind	0.908s
$ go test -v ./update/
=== RUN   TestUnduh
--- PASS: TestUnduh (0.01s)
=== RUN   TestUnduhRusak
--- PASS: TestUnduhRusak (0.00s)
=== RUN   TestBukanHTTPS
--- PASS: TestBukanHTTPS (0.00s)
=== RUN   TestGanti
--- PASS: TestGanti (0.01s)
PASS
ok  	vpnku/update	0.827s
```

Keempat program di folder `rilis` juga berhasil dibangun dari satu komputer Windows, dan `client-windows-amd64.exe -version` menjawab `v1.0.0`. `Replace` juga dicoba di Windows dengan program kecil yang mengganti dirinya sendiri saat berjalan. File baru mengambil nama aslinya, dan file `.old` tertinggal sampai program itu berhenti, persis seperti yang dijelaskan di atas.

Ada yang belum dijalankan untuk artikel ini. File unit systemd, task di Task Scheduler, dan `wintun.dll` di dalam program belum dicoba di komputer sungguhan, karena butuh hak administrator. Coba sekali sebelum mengandalkannya.

## Apa yang sudah kita bangun dalam seri ini

Sembilan bagian ini membangun VPN dari nol. Isinya adalah:

1. **Bagian 1** menjelaskan apa itu VPN, paket di dalam paket, dan kenapa CGNAT membuat kita memilih relay.
2. **Bagian 2** membawa WireGuard lewat WebSocket ke sebuah relay, dengan Bind buatan sendiri.
3. **Bagian 3** menjalankan VPN dengan virtual network card sungguhan di Linux dan Windows, dengan relay di belakang Caddy.
4. **Bagian 4** membuat relay dan client saling membuktikan kepemilikan kunci, lalu relay membagikan daftar client.
5. **Bagian 5** membuat VPN tersambung lagi sendiri, mengikuti perubahan client tanpa restart, dan memberi setiap client antrean sendiri.
6. **Bagian 6** mengganti penyalinan kunci dengan undangan sekali pakai.
7. **Bagian 7** memberi setiap client nama, yang dijawab di dalam tunnel.
8. **Bagian 8** menjalankan relay di belakang layanan tunnel atau di layanan container, dan memperbarui koneksi sebelum diputus.
9. **Bagian 9** membuat VPN menyala sendiri, menjadi satu file untuk setiap OS, dan bisa diperbarui dengan aman.

![Client di kiri berisi aplikasi, vpnku0 dan resolver nama, wireguard-go, Bind WebSocket, serta undangan dan layanan, masing-masing dengan nomor bagian yang membangunnya. Bind WebSocket terhubung lewat HTTPS, yaitu Caddy, layanan tunnel, atau layanan container, ke relay di kanan. Relay berisi bukti kunci dua arah, relay.json dengan client dan undangan, daftar client untuk semua, dan antrean per client. Relay meneruskan paket ke client lain yang susunannya sama.](assets/vpn-series-overview.png)

*Setiap kotak menunjukkan bagian seri yang membangunnya.*

## Kelemahan desain VPN di seri ini

Setiap bagian ditutup dengan daftar kelemahannya. Kalau dikumpulkan, desain ini secara keseluruhan punya tujuh kelemahan. Ketujuhnya adalah:

- **Semua paket lewat relay.** Perjalanan paket lebih panjang daripada koneksi langsung, dan kecepatan seluruh VPN dibatasi relay. Inilah yang dikorbankan saat memilih relay di Bagian 1, supaya VPN tetap berjalan di balik CGNAT.
- **TCP di dalam TCP.** WebSocket berjalan di atas TCP, jadi di jaringan yang sering kehilangan paket, aplikasi TCP di dalam tunnel melambat lebih banyak daripada di WireGuard biasa.
- **Relay dipercaya menentukan client.** Relay tidak bisa membaca isi paket, tetapi relay yang dicuri kuncinya bisa menambah client palsu. Daftar client yang ditandatangani kunci admin yang disimpan offline akan menutup celah ini, tetapi seri ini tidak membangunnya.
- **Relay dan pihak di depannya melihat siapa berbicara dengan siapa.** Public key, ukuran, dan waktu setiap paket terlihat, walaupun isinya tidak.
- **Satu relay adalah satu titik mati.** Kalau relay mati, seluruh VPN berhenti sampai relay hidup lagi.
- **Hanya IPv4, dan hanya client.** Komputer lain di LAN sebuah client tidak ikut masuk ke VPN.
- **Bagian Windows belum dijalankan.** Wintun dari Bagian 3, aturan NRPT dari Bagian 7, dan Task Scheduler dari bagian ini baru diperiksa dan dibangun, belum dijalankan di Windows sungguhan.

## Kapan sebaiknya memilih alat lain

VPN di seri ini bukan satu-satunya jawaban. Ada empat pilihan lain yang sering dipakai, masing-masing dengan kondisinya sendiri. Keempatnya adalah:

- **WireGuard biasa.** Kalau salah satu sisi punya IP publik dan UDP tidak diblokir, WireGuard biasa lebih cepat, karena paket berjalan langsung lewat UDP dan bisa diproses di dalam kernel. Pilihan ini kurang cocok kalau semua komputer berada di balik CGNAT.
- **Tailscale.** Tailscale mencoba koneksi UDP langsung lebih dulu, dengan hole punching. Kalau gagal, Tailscale memakai relay bernama DERP lewat HTTPS, dan relay itu juga tidak bisa membaca isi paket. Pilihan ini cocok kalau kamu ingin koneksi langsung secepat mungkin dan tidak keberatan memakai server koordinasi milik Tailscale.
- **Headscale.** Headscale adalah implementasi open source dari server koordinasi Tailscale, yang dijalankan sendiri, untuk satu jaringan. Pilihan ini cocok kalau kamu ingin cara kerja Tailscale dengan server di bawah kendalimu sendiri.
- **ZeroTier.** ZeroTier menyebut dirinya switch Ethernet yang bisa diprogram. Kebanyakan traffic-nya berjalan langsung antarkomputer, dengan relay gratis tetapi lambat untuk sisanya.

VPN di seri ini cocok untuk tiga keadaan. Yang pertama adalah belajar cara kerja VPN dari dalam, yang kedua adalah beberapa komputer milik sendiri, dan yang ketiga adalah jaringan yang hanya mengizinkan HTTPS keluar. VPN ini kurang cocok untuk kecepatan setinggi mungkin, untuk ratusan pengguna, atau untuk layanan yang tidak boleh berhenti.

## Akhir seri Membuat VPN Sendiri

Seri ini dimulai dengan satu masalah, yaitu VPN butuh satu sisi yang bisa dijangkau, padahal di balik CGNAT tidak ada. Jawabannya adalah relay yang dijangkau lewat HTTPS, dan setiap bagian sesudahnya memperbaiki satu kelemahan dari jawaban itu, dengan mengorbankan hal lain.

Kalau kamu baru bergabung di bagian ini, mulailah dari Bagian 1. Setiap bagian membangun di atas bagian sebelumnya, dan kodenya bisa diikuti dari awal sampai akhir.
