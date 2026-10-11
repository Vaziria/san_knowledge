# [Seri Membuat VPN Sendiri Bagian 6] Menambah client VPN dengan undangan sekali pakai

*Di bagian ini, kita berhenti menyalin kunci dengan tangan. Admin membuat undangan dengan satu perintah, dan komputer baru bergabung sendiri tanpa private key-nya pernah meninggalkan komputer itu.*

Di Bagian 5, VPN kita menjadi tahan banting. Client tersambung lagi sendiri saat koneksi putus, dan relay mengirim perubahan daftar client tanpa restart. Tetapi menambah client baru masih pekerjaan tangan, dan pekerjaan itu mudah salah.

Di Bagian 6, kita menggantinya dengan undangan. Di akhir bagian ini, setiap langkah cukup satu perintah. Relay dibuat dengan `relay init`, client diundang dengan `relay undang`, komputer baru bergabung dengan `client gabung`, dan client dikeluarkan dengan `relay keluarkan`.

## Kenapa menambah client dengan tangan mudah salah?

Di Bagian 5, menambah satu client butuh lima langkah, dan informasinya berpindah dua arah. Langkah-langkahnya adalah:

1. Client baru menjalankan `genkey`, lalu mengirim public key-nya ke admin.
2. Admin memilih alamat VPN yang belum dipakai.
3. Admin menulis public key dan alamat itu di `relay.json`, dalam format JSON.
4. Admin mengirim alamat relay dan public key relay ke client baru.
5. Client baru menulis keduanya, bersama private key-nya, di file konfigurasinya.

Setiap langkah adalah kesempatan untuk salah. Public key ditulis sebagai 64 huruf hex, dan satu huruf yang salah membuat client itu ditolak tanpa alasan yang jelas. Dua client yang mendapat alamat sama akan saling mengganggu, dan koma yang hilang di `relay.json` membuat relay menolak seluruh perubahan.

## Tiga cara menambah client baru

Ada tiga cara yang umum untuk memasukkan komputer baru ke sebuah jaringan. Ketiganya adalah:

- **Admin membuat kunci untuk client baru.** Admin menjalankan `genkey`, mendaftarkan public key-nya, lalu mengirim file konfigurasi lengkap ke client baru. Informasinya hanya berpindah satu arah, dan client baru cukup menyimpan satu file. Masalahnya, private key ikut berpindah lewat chat atau email, dan admin juga tahu private key itu.
- **Persetujuan saat pertama terhubung.** Komputer baru membuat kuncinya sendiri lalu terhubung, dan relay mencatatnya sebagai "menunggu". Admin tinggal menyetujuinya. Masalahnya, admin harus memastikan permintaan itu benar-benar dari komputer baru, bukan dari penyerang yang terhubung pada saat yang sama, dan caranya kembali ke membandingkan 64 huruf hex.
- **Undangan sekali pakai.** Admin membuat rahasia berumur pendek yang hanya berlaku sekali. Komputer baru membuat kuncinya sendiri, lalu mengirim public key-nya bersama rahasia itu ke relay.

Seri ini memilih cara ketiga. Informasinya hanya berpindah satu arah, dari admin ke komputer baru, dan private key tidak pernah meninggalkan komputer yang membuatnya, sesuai prinsip di Bagian 1.

Kelemahannya, undangan adalah rahasia selama masih berlaku. Siapa pun yang memakainya lebih dulu akan bergabung sebagai client. Karena itu, undangan hanya bisa dipakai sekali, hanya berlaku 24 jam, dan admin bisa melihat siapa yang sudah bergabung lewat `relay daftar`.

## Anatomi undangan VPN

Undangan berisi tiga hal yang dibutuhkan komputer baru. Ketiganya adalah:

- **Alamat relay**, supaya komputer baru tahu ke mana harus terhubung.
- **Public key relay**, supaya komputer baru bisa memeriksa bukti dari relay seperti di Bagian 4. Relay palsu tidak bisa berpura-pura menjadi relay ini.
- **Token**, yaitu 32 byte acak yang menjadi rahasia sekali pakai.

Ketiganya disimpan sebagai JSON, lalu ditulis dengan base64, dan diberi awalan `vpnku-`. Base64 adalah cara menulis byte apa pun dengan huruf, angka, tanda minus, dan garis bawah, sehingga undangan menjadi satu baris teks yang mudah disalin. Contohnya, yang dipendekkan di sini, adalah:

```text
vpnku-eyJyZWxheSI6IndzOi8vMTkyLjAuMi4xOjgwODAiLCJyZWxheV9rZXkiOiI0OTU2YTZl...
```

Relay sendiri tidak menyimpan token. Relay hanya menyimpan nama calon client, hash dari token, dan masa berlakunya. Dengan begitu, siapa pun yang sempat melihat `relay.json`, misalnya dari salinan cadangan atau layar yang sedang dibagikan, tidak bisa memakai undangan yang masih berlaku.

Hash ini juga punya kelemahan, walaupun kecil. Undangan yang hilang tidak bisa ditampilkan lagi, jadi admin harus membatalkannya dan membuat undangan baru.

![Tiga garis waktu, yaitu admin, komputer baru, dan relay. Admin menjalankan relay undang kantor, yang menyimpan hash token di relay.json, lalu mengirim undangan berisi alamat relay, public key relay, dan token. Komputer baru membuat sepasang kunci, lalu mengirim token dan public key, bukan private key, ke relay lewat HTTPS. Relay memeriksa token dan masa berlakunya, menghapus undangan, dan menjawab dengan nama kantor dan alamat 10.10.0.2. Akhirnya, komputer baru terhubung dengan bukti kunci.](assets/invite-join-sequence.png)

*Yang berpindah hanya undangan dan public key.*

Urutan bergabungnya terdiri dari tujuh langkah. Langkah-langkahnya adalah:

1. Admin menjalankan `relay undang kantor` di mesin relay. Relay menyimpan hash token dan masa berlakunya di `relay.json`, lalu mencetak undangannya.
2. Admin mengirim undangan ke pemilik komputer baru, dengan cara yang sama seperti mengirim password.
3. Komputer baru menjalankan `client gabung` dengan undangan itu, dan membuat sepasang kunci baru.
4. Komputer baru mengirim token dan public key-nya ke relay, ke alamat `/gabung` lewat HTTPS.
5. Relay mencari hash token itu di daftar undangan, dan memeriksa masa berlakunya. Kalau cocok, relay memberi alamat VPN berikutnya yang masih kosong, mendaftarkan public key itu, dan menghapus undangannya.
6. Relay menjawab dengan nama dan alamat client baru.
7. Komputer baru menyimpan alamat relay, public key relay, dan private key-nya di `client.json`. Setelah itu, program client terhubung ke relay dengan bukti kunci, persis seperti di Bagian 4.

### Relay memilih alamat VPN sendiri

Relay memberi alamat VPN berikutnya yang masih kosong, mulai dari `10.10.0.1`. Ada tiga alamat yang tidak pernah dibagikan. Ketiganya adalah:

- **Alamat jaringan**, yaitu `10.10.0.0`, alamat pertama yang menjadi nama jaringan itu sendiri.
- **Alamat broadcast**, yaitu `10.10.0.255`, alamat terakhir yang berarti semua komputer di jaringan itu.
- **Alamat sebelum broadcast**, yaitu `10.10.0.254`. Alamat ini disisihkan untuk layanan di dalam VPN, dan kita memakainya di Bagian 7.

Yang dikorbankan adalah kebebasan admin memilih alamat. Kalau sebuah komputer harus punya alamat tertentu, admin masih bisa mengubah `relay.json` dengan tangan.

### Setiap client punya nama

Undangan dibuat untuk sebuah nama, misalnya `rumah` atau `kantor`, dan nama itu ikut tersimpan di daftar client. Admin memakai nama ini untuk mengeluarkan client, tanpa harus menyebut public key-nya.

Nama hanya boleh berisi huruf kecil, angka, dan tanda minus, paling panjang 32 huruf. Aturan ini sengaja sama dengan aturan nama komputer di internet, karena di Bagian 7 nama ini akan dipakai untuk menjangkau komputer itu.

## Dua program yang mengubah relay.json bersamaan

Di Bagian 5, hanya admin yang mengubah `relay.json`, dengan tangan. Sekarang ada dua jenis program yang mengubahnya. Yang pertama adalah relay sendiri, saat seseorang bergabung, dan yang kedua adalah perintah admin, seperti `relay undang` dan `relay keluarkan`.

Masalahnya, kalau dua program membaca file pada saat yang sama, lalu masing-masing mengubah dan menulisnya, tulisan yang terakhir menghapus perubahan yang pertama tanpa ada yang tahu. Seperti client yang bergabung tepat saat admin membuat undangan untuk orang lain. Undangan itu hilang, dan admin baru tahu saat undangannya ditolak.

Ada tiga cara untuk mencegahnya. Ketiganya adalah:

- **Semua perubahan lewat relay.** Perintah admin dikirim ke relay lewat HTTP, sehingga hanya relay yang menulis file. Kelemahannya sama seperti di Bagian 5, yaitu alamat admin itu butuh autentikasi sendiri.
- **Database.** Database seperti SQLite sudah mengurus banyak program yang menulis bersamaan. Kelemahannya, kita butuh library tambahan, dan konfigurasinya tidak lagi bisa dibaca atau diperbaiki dengan editor teks biasa.
- **File penanda dan penggantian sekaligus.** Sebelum mengubah `relay.json`, sebuah program membuat file `relay.json.lock`. OS hanya mengizinkan satu program membuat file baru dengan nama yang sama, jadi program lain menunggu sampai file itu dihapus.

Seri ini memilih cara ketiga, karena tidak butuh library dan file konfigurasinya tetap JSON biasa. Isi baru juga tidak ditulis langsung ke `relay.json`, tetapi ke `relay.json.tmp`, yang lalu menggantikan `relay.json` dengan satu kali rename, yaitu mengganti nama file. Rename mengganti file dalam satu langkah, jadi relay yang membaca file setiap dua detik tidak pernah melihat file setengah jadi.

Cara ini punya dua kelemahan. Kalau sebuah program mati di tengah perubahan, `relay.json.lock` tertinggal, dan program berikutnya berhenti setelah lima detik dengan pesan untuk menghapusnya. Selain itu, editor teks tidak mengenal file penanda ini, jadi mengubah `relay.json` dengan tangan sebaiknya dilakukan saat tidak ada yang sedang bergabung.

## Menulis undangan dengan Go

Undangan dipakai oleh relay dan oleh client, jadi kodenya ada di package baru, `invite/invite.go`. Kodenya adalah:

```go
// Package invite membuat dan membaca undangan sekali pakai, lalu memakainya
// untuk mendaftarkan public key client baru ke relay.
package invite

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Path adalah alamat di relay yang menerima undangan.
const Path = "/gabung"

const prefix = "vpnku-"

// ErrRefused berarti relay menerima permintaan, tetapi menolak undangannya.
var ErrRefused = errors.New("relay menolak")

// Invite berisi semua yang dibutuhkan client baru untuk bergabung.
type Invite struct {
	Relay    string `json:"relay"`     // alamat relay, misalnya wss://relay.example.com
	RelayKey string `json:"relay_key"` // public key relay, hex
	Token    string `json:"token"`     // rahasia sekali pakai, hex
}

// String menulis undangan sebagai satu baris teks yang mudah disalin.
func (i Invite) String() string {
	data, _ := json.Marshal(i)
	return prefix + base64.RawURLEncoding.EncodeToString(data)
}

// Parse membaca undangan yang ditulis oleh String.
func Parse(s string) (Invite, error) {
	var i Invite
	s, ok := strings.CutPrefix(strings.TrimSpace(s), prefix)
	data, err := base64.RawURLEncoding.DecodeString(s)
	if !ok || err != nil || json.Unmarshal(data, &i) != nil {
		return i, errors.New("undangan tidak bisa dibaca")
	}
	return i, nil
}

// Request dikirim client baru ke relay.
type Request struct {
	Token     string `json:"token"`
	PublicKey string `json:"public_key"` // hex
}

// Response adalah jawaban relay kalau undangannya diterima.
type Response struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

// Join mengirim public key client baru ke relay bersama token undangan.
// Private key tidak pernah dikirim.
func Join(ctx context.Context, i Invite, publicKey string) (Response, error) {
	var res Response
	// wss://relay.example.com menjadi https://relay.example.com/gabung.
	url := strings.Replace(strings.TrimSuffix(i.Relay, "/"), "ws", "http", 1) + Path
	body, err := json.Marshal(Request{Token: i.Token, PublicKey: publicKey})
	if err != nil {
		return res, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return res, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return res, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return res, fmt.Errorf("%w: %s", ErrRefused, strings.TrimSpace(string(msg)))
	}
	err = json.NewDecoder(resp.Body).Decode(&res)
	return res, err
}
```

Isinya terdiri dari tiga bagian. Ketiganya adalah:

1. `Invite` beserta `String` dan `Parse`, yang mengubah undangan menjadi satu baris teks dan sebaliknya.
2. `Request` dan `Response`, yaitu isi permintaan dari client baru dan jawaban dari relay.
3. `Join`, yang dipanggil di komputer baru. Alamat `wss://` diubah menjadi `https://`, karena bergabung memakai request HTTP biasa, bukan WebSocket. Kalau relay menjawab dengan selain `200 OK`, `Join` mengembalikan error yang dibungkus `ErrRefused`, supaya pemanggilnya tahu bahwa relay benar-benar menolak.

Membuat kunci kini dibutuhkan oleh relay dan oleh client, jadi isi program `genkey` dipindah menjadi fungsi `NewKey` di package `proof`. Kodenya adalah:

```go
// NewKey membuat sepasang kunci WireGuard baru, dengan cara yang sama seperti
// program genkey.
func NewKey() (priv, pub [32]byte, err error) {
	if _, err := rand.Read(priv[:]); err != nil {
		return priv, pub, err
	}
	priv[0] &= 248
	priv[31] = priv[31]&127 | 64
	p, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return priv, pub, err
	}
	copy(pub[:], p)
	return priv, pub, nil
}
```

## Menyimpan konfigurasi relay dengan aman

Konfigurasi relay kini punya tiga bagian baru, yaitu alamat relay untuk undangan, nama setiap client, dan daftar undangan yang belum dipakai. Kodenya ada di file baru, `cmd/relay/config.go`. Isinya adalah:

```go
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/netip"
	"os"
	"regexp"
	"time"

	"vpnku/proof"
	"vpnku/relay"
)

// Config adalah isi file konfigurasi relay.
type Config struct {
	PrivateKey string          `json:"private_key"` // hex
	URL        string          `json:"url"`         // alamat relay untuk client, misalnya wss://relay.example.com
	Network    string          `json:"network"`     // misalnya 10.10.0.0/24
	Members    []Member        `json:"members"`
	Invites    []PendingInvite `json:"invites"`
}

type Member struct {
	Name      string `json:"name"`
	PublicKey string `json:"public_key"` // hex
	Address   string `json:"address"`    // misalnya 10.10.0.2
}

// PendingInvite adalah undangan yang belum dipakai. Relay hanya menyimpan
// hash dari tokennya, bukan token itu sendiri.
type PendingInvite struct {
	Name    string    `json:"name"`
	Hash    string    `json:"hash"` // SHA-256 dari token, hex
	Expires time.Time `json:"expires"`
}

// decode membaca isi file konfigurasi.
func decode(data []byte) (*Config, error) {
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return decode(data)
}

// settings adalah isi konfigurasi yang sudah diperiksa, dalam bentuk yang
// dipakai relay.
type settings struct {
	key     [32]byte
	network netip.Prefix
	members []relay.Member
}

// check memeriksa setiap nilai di konfigurasi.
func (c *Config) check() (settings, error) {
	var s settings
	var err error
	if s.key, err = proof.ParseKey(c.PrivateKey); err != nil {
		return s, fmt.Errorf("private_key: %v", err)
	}
	if s.network, err = netip.ParsePrefix(c.Network); err != nil {
		return s, fmt.Errorf("network: %v", err)
	}
	for _, m := range c.Members {
		pub, err := proof.ParseKey(m.PublicKey)
		if err != nil {
			return s, fmt.Errorf("public_key client %s: %v", m.Name, err)
		}
		addr, err := netip.ParseAddr(m.Address)
		if err != nil || !s.network.Contains(addr) {
			return s, fmt.Errorf("alamat client %s harus berada di %s", m.Name, s.network)
		}
		s.members = append(s.members, relay.Member{PublicKey: pub, Address: addr})
	}
	return s, nil
}

// update mengubah file konfigurasi dengan aman. Hanya satu program yang boleh
// mengubahnya pada satu waktu, yaitu program yang berhasil membuat file .lock
// lebih dulu. Isi baru ditulis ke file sementara, lalu menggantikan file lama
// sekaligus, supaya tidak ada yang pernah membaca file setengah jadi.
func update(path string, change func(*Config) error) error {
	lock := path + ".lock"
	for try := 0; ; try++ {
		f, err := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			f.Close()
			break
		}
		if !errors.Is(err, fs.ErrExist) {
			return err
		}
		if try == 50 {
			return fmt.Errorf("%s masih ada setelah 5 detik; hapus file itu kalau tidak ada program lain yang sedang mengubah %s", lock, path)
		}
		time.Sleep(100 * time.Millisecond)
	}
	defer os.Remove(lock)

	cfg, err := load(path)
	if err != nil {
		return err
	}
	if err := change(cfg); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

var namePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,30}[a-z0-9])?$`)

// taken memeriksa apakah sebuah nama sudah dipakai client atau undangan.
func (c *Config) taken(name string) bool {
	for _, m := range c.Members {
		if m.Name == name {
			return true
		}
	}
	for _, i := range c.Invites {
		if i.Name == name {
			return true
		}
	}
	return false
}

// dropExpired menghapus undangan yang sudah lewat masa berlakunya.
func (c *Config) dropExpired() {
	var keep []PendingInvite
	for _, i := range c.Invites {
		if time.Now().Before(i.Expires) {
			keep = append(keep, i)
		}
	}
	c.Invites = keep
}

func hashToken(token []byte) string {
	sum := sha256.Sum256(token)
	return hex.EncodeToString(sum[:])
}
```

Fungsi-fungsi di file ini punya tugas masing-masing. Tugas-tugas itu adalah:

- **decode dan load** membaca isi `relay.json`.
- **check** memeriksa setiap nilai, lalu mengubahnya menjadi `settings`, yaitu bentuk yang dipakai relay. Fungsi ini menggantikan `parse` dari Bagian 5.
- **update** adalah satu-satunya cara mengubah `relay.json`. Fungsi ini membuat file `.lock` dengan `os.O_EXCL`, yang gagal kalau file itu sudah ada. Setelah itu, `update` membaca file, memanggil `change`, menulis hasilnya ke file `.tmp`, lalu mengganti `relay.json` dengan `os.Rename`.
- **taken, dropExpired, dan hashToken** adalah fungsi kecil untuk memeriksa nama, membuang undangan yang sudah kedaluwarsa, dan menghitung hash token dengan SHA-256.

File konfigurasi kini ditulis dengan izin `0o600`, yaitu hanya bisa dibaca pemiliknya di Linux. File ini berisi private key relay, jadi pengguna lain di VPS tidak boleh membacanya.

Contoh isi `relay.json` setelah satu client bergabung dan satu undangan dibuat, dengan kunci dan hash dipendekkan, adalah:

```json
{
  "private_key": "…",
  "url": "wss://relay.example.com",
  "network": "10.10.0.0/24",
  "members": [
    {"name": "rumah", "public_key": "…", "address": "10.10.0.1"}
  ],
  "invites": [
    {"name": "laptop", "hash": "…", "expires": "2026-10-12T14:39:35Z"}
  ]
}
```

## Menerima undangan di relay

Relay menerima undangan di file baru, `cmd/relay/join.go`. File ini juga berisi pembuat undangan dan perintah untuk mengeluarkan client, karena ketiganya mengubah daftar yang sama. Kodenya adalah:

```go
package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/netip"
	"time"

	"golang.org/x/crypto/curve25519"

	"vpnku/invite"
	"vpnku/proof"
	"vpnku/relay"
)

// inviteTTL adalah masa berlaku sebuah undangan.
const inviteTTL = 24 * time.Hour

var errInvalidInvite = errors.New("undangan tidak berlaku")

// createInvite membuat undangan sekali pakai untuk client bernama name.
func createInvite(path, name string) (invite.Invite, error) {
	var inv invite.Invite
	if !namePattern.MatchString(name) {
		return inv, errors.New("nama hanya boleh berisi huruf kecil, angka, dan tanda minus")
	}
	token := make([]byte, 32)
	rand.Read(token)
	err := update(path, func(c *Config) error {
		c.dropExpired()
		if c.taken(name) {
			return fmt.Errorf("nama %s sudah dipakai", name)
		}
		key, err := proof.ParseKey(c.PrivateKey)
		if err != nil {
			return err
		}
		pub, err := curve25519.X25519(key[:], curve25519.Basepoint)
		if err != nil {
			return err
		}
		c.Invites = append(c.Invites, PendingInvite{Name: name, Hash: hashToken(token), Expires: time.Now().Add(inviteTTL)})
		inv = invite.Invite{Relay: c.URL, RelayKey: hex.EncodeToString(pub), Token: hex.EncodeToString(token)}
		return nil
	})
	return inv, err
}

// joinHandler menerima undangan dari client baru. Kalau undangannya berlaku,
// public key client itu didaftarkan dengan alamat VPN berikutnya yang masih
// kosong, dan undangannya dihapus.
type joinHandler struct {
	path  string
	relay *relay.Relay
}

func (h *joinHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var in invite.Request
	if err := json.NewDecoder(io.LimitReader(req.Body, 4096)).Decode(&in); err != nil {
		http.Error(w, "permintaan tidak bisa dibaca", http.StatusBadRequest)
		return
	}
	token, err := hex.DecodeString(in.Token)
	if err != nil {
		http.Error(w, errInvalidInvite.Error(), http.StatusForbidden)
		return
	}
	pub, err := proof.ParseKey(in.PublicKey)
	if err != nil {
		http.Error(w, "public_key: "+err.Error(), http.StatusBadRequest)
		return
	}

	var out invite.Response
	var s settings
	err = update(h.path, func(c *Config) error {
		c.dropExpired()
		i := -1
		for n, p := range c.Invites {
			if p.Hash == hashToken(token) {
				i = n
			}
		}
		if i < 0 {
			return errInvalidInvite
		}
		for _, m := range c.Members {
			if m.PublicKey == hex.EncodeToString(pub[:]) {
				return errors.New("public key ini sudah terdaftar")
			}
		}
		network, err := netip.ParsePrefix(c.Network)
		if err != nil {
			return err
		}
		addr, err := nextAddress(network, c.Members)
		if err != nil {
			return err
		}
		out = invite.Response{Name: c.Invites[i].Name, Address: addr.String()}
		c.Members = append(c.Members, Member{Name: out.Name, PublicKey: hex.EncodeToString(pub[:]), Address: out.Address})
		c.Invites = append(c.Invites[:i], c.Invites[i+1:]...)
		s, err = c.check()
		return err
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	// Relay langsung mengenal client baru, tanpa menunggu file dibaca ulang.
	h.relay.SetMembers(s.members)
	log.Printf("%s bergabung dengan alamat %s", out.Name, out.Address)
	json.NewEncoder(w).Encode(out)
}

// nextAddress mengembalikan alamat pertama di jaringan VPN yang belum
// dipakai. Alamat pertama adalah alamat jaringan itu sendiri, dan alamat
// terakhir adalah alamat broadcast, jadi keduanya dilewati. Alamat sebelum
// broadcast juga dilewati, karena disisihkan untuk layanan di dalam VPN.
func nextAddress(network netip.Prefix, members []Member) (netip.Addr, error) {
	used := map[string]bool{}
	for _, m := range members {
		used[m.Address] = true
	}
	for a := network.Addr().Next(); network.Contains(a.Next().Next()); a = a.Next() {
		if !used[a.String()] {
			return a, nil
		}
	}
	return netip.Addr{}, errors.New("jaringan VPN sudah penuh")
}

// removeMember mengeluarkan client, atau membatalkan undangan, bernama name.
// Relay yang sedang berjalan memutus client itu dalam dua detik.
func removeMember(path, name string) error {
	return update(path, func(c *Config) error {
		for i, m := range c.Members {
			if m.Name == name {
				c.Members = append(c.Members[:i], c.Members[i+1:]...)
				return nil
			}
		}
		for i, p := range c.Invites {
			if p.Name == name {
				c.Invites = append(c.Invites[:i], c.Invites[i+1:]...)
				return nil
			}
		}
		return fmt.Errorf("tidak ada client atau undangan bernama %s", name)
	})
}
```

Kode ini terdiri dari empat bagian. Keempatnya adalah:

1. `createInvite` membuat token acak, menyimpan hash-nya bersama nama dan masa berlakunya, lalu menyusun undangan berisi alamat relay, public key relay, dan token.
2. `joinHandler` menerima permintaan dari client baru. Seluruh pemeriksaan dan perubahan terjadi di dalam satu panggilan `update`, jadi tidak ada program lain yang bisa mengubah daftar di tengah jalan. Undangan yang tidak ada, sudah dipakai, atau kedaluwarsa ditolak dengan pesan yang sama, "undangan tidak berlaku", supaya penebak tidak tahu bedanya.
3. Setelah berhasil, `joinHandler` langsung memanggil `SetMembers`, sehingga client baru bisa terhubung detik itu juga. Tanpa ini, client yang langsung terhubung bisa ditolak, karena relay baru membaca ulang `relay.json` dua detik kemudian.
4. `nextAddress` mencari alamat kosong pertama. Putarannya berhenti saat alamat dua langkah di depan sudah di luar jaringan, sehingga alamat sebelum broadcast dan alamat broadcast tidak pernah dibagikan. `removeMember` menghapus client atau undangan dengan nama tertentu.

Token yang bisa ditebak akan merusak seluruh cara ini. Token di sini adalah 32 byte dari `crypto/rand`, yaitu 256 bit acak, jadi menebaknya lewat internet tidak mungkin, berapa kali pun dicoba.

## Perintah-perintah program relay

Program relay kini punya lima perintah. Kodenya ada di `cmd/relay/main.go`, dan isinya adalah:

```go
// Program relay menjalankan relay VPN dan mengurus client-nya.
package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"vpnku/invite"
	"vpnku/proof"
	"vpnku/relay"
)

const usage = `Perintah:
  relay init <alamat>      membuat relay.json, misalnya relay init wss://relay.example.com
  relay run                menjalankan relay
  relay undang <nama>      membuat undangan sekali pakai untuk client baru
  relay keluarkan <nama>   mengeluarkan client, atau membatalkan undangan
  relay daftar             menampilkan client dan undangan`

func main() {
	path := flag.String("config", "relay.json", "file konfigurasi")
	listen := flag.String("listen", "127.0.0.1:8080", "alamat yang didengarkan relay")
	flag.Parse()

	var err error
	switch flag.Arg(0) {
	case "init":
		err = initConfig(*path, flag.Arg(1))
	case "run":
		err = run(*path, *listen)
	case "undang":
		var inv invite.Invite
		if inv, err = createInvite(*path, flag.Arg(1)); err == nil {
			fmt.Printf("Undangan untuk %s, sekali pakai dan berlaku %.0f jam:\n%s\n", flag.Arg(1), inviteTTL.Hours(), inv)
		}
	case "keluarkan":
		if err = removeMember(*path, flag.Arg(1)); err == nil {
			fmt.Printf("%s dikeluarkan\n", flag.Arg(1))
		}
	case "daftar":
		err = list(*path)
	default:
		err = errors.New(usage)
	}
	if err != nil {
		log.Fatal(err)
	}
}

// initConfig membuat file konfigurasi baru dengan private key relay yang baru.
func initConfig(path, url string) error {
	if !strings.HasPrefix(url, "wss://") && !strings.HasPrefix(url, "ws://") {
		return errors.New("alamat relay harus diawali wss://, misalnya wss://relay.example.com")
	}
	priv, pub, err := proof.NewKey()
	if err != nil {
		return err
	}
	cfg := Config{PrivateKey: hex.EncodeToString(priv[:]), URL: url, Network: "10.10.0.0/24"}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err // termasuk kalau file itu sudah ada
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}
	fmt.Printf("%s dibuat. Public key relay: %x\n", path, pub)
	return nil
}

// run menjalankan relay sampai program dihentikan.
func run(path, listen string) error {
	h, err := newHandler(context.Background(), path)
	if err != nil {
		return err
	}
	log.Printf("relay berjalan di %s", listen)
	return http.ListenAndServe(listen, h)
}

// newHandler menyusun relay dari file konfigurasi. Koneksi WebSocket menuju
// relay, dan undangan menuju joinHandler.
func newHandler(ctx context.Context, path string) (http.Handler, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg, err := decode(data)
	if err != nil {
		return nil, err
	}
	s, err := cfg.check()
	if err != nil {
		return nil, err
	}
	r := relay.New(s.key, s.network, s.members)
	go watch(ctx, path, data, s, r)

	mux := http.NewServeMux()
	mux.Handle("POST "+invite.Path, &joinHandler{path: path, relay: r})
	mux.Handle("/", r)
	log.Printf("relay memuat %d client dari %s", len(s.members), path)
	return mux, nil
}

// watch membaca file konfigurasi setiap dua detik. Kalau isinya berubah,
// daftar client yang baru diberikan ke relay tanpa restart.
func watch(ctx context.Context, path string, last []byte, first settings, r *relay.Relay) {
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		data, err := os.ReadFile(path)
		if err != nil || bytes.Equal(data, last) {
			continue
		}
		last = data
		cfg, err := decode(data)
		var s settings
		if err == nil {
			s, err = cfg.check()
		}
		if err == nil && (s.key != first.key || s.network != first.network) {
			err = errors.New("private_key dan network tidak bisa diubah tanpa restart")
		}
		if err != nil {
			log.Printf("perubahan %s tidak dipakai: %v", path, err)
			continue
		}
		r.SetMembers(s.members)
		log.Printf("daftar client diperbarui: %d client", len(s.members))
	}
}

// list menampilkan client dan undangan yang belum dipakai.
func list(path string) error {
	cfg, err := load(path)
	if err != nil {
		return err
	}
	for _, m := range cfg.Members {
		fmt.Printf("%-16s %-15s %.16s…\n", m.Name, m.Address, m.PublicKey)
	}
	for _, i := range cfg.Invites {
		state := "undangan, berlaku sampai " + i.Expires.Local().Format("2006-01-02 15:04")
		if time.Now().After(i.Expires) {
			state = "undangan, sudah tidak berlaku"
		}
		fmt.Printf("%-16s %s\n", i.Name, state)
	}
	return nil
}
```

Ada tiga hal yang berubah dibandingkan Bagian 5. Ketiganya adalah:

1. Perintah dipilih dari kata pertama setelah flag, lewat `flag.Arg(0)`. Flag seperti `-config` dan `-listen` ditulis sebelum perintahnya, misalnya `relay -listen 0.0.0.0:8080 run`.
2. `initConfig` membuat `relay.json` dengan private key relay yang baru. File dibuka dengan `os.O_EXCL`, jadi perintah ini tidak pernah menimpa relay yang sudah ada beserta kuncinya.
3. `newHandler` menyusun relay dan alamat `/gabung` di satu `http.ServeMux`, yaitu pembagi request milik Go. Request `POST /gabung` menuju `joinHandler`, dan semua request lain menuju relay. Caddy dari Bagian 3 meneruskan keduanya tanpa perubahan.

`watch` bekerja seperti di Bagian 5, dengan dua perbedaan. Fungsi ini kini bisa dihentikan lewat `context`, supaya test bisa mematikannya, dan perubahan private key relay juga ditolak, bukan hanya perubahan rentang alamat.

## Bergabung dari komputer baru

Di sisi client, perintah `gabung` ada di file baru, `cmd/client/join.go`. Kodenya adalah:

```go
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"vpnku/invite"
	"vpnku/proof"
)

// join bergabung ke VPN dengan sebuah undangan. Kunci baru dibuat di komputer
// ini, dan hanya public key-nya yang dikirim ke relay.
func join(path, text string) error {
	inv, err := invite.Parse(text)
	if err != nil {
		return err
	}
	priv, pub, err := proof.NewKey()
	if err != nil {
		return err
	}

	// File konfigurasi ditulis lebih dulu, supaya private key tidak hilang
	// kalau relay sudah mendaftarkan public key-nya.
	data, err := json.MarshalIndent(Config{
		Relay:      inv.Relay,
		RelayKey:   inv.RelayKey,
		PrivateKey: hex.EncodeToString(priv[:]),
	}, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%s sudah ada, jadi komputer ini sudah bergabung", path)
	}
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(path)
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := invite.Join(ctx, inv, hex.EncodeToString(pub[:]))
	if errors.Is(err, invite.ErrRefused) {
		os.Remove(path) // undangan ditolak, jadi kunci ini tidak akan dipakai
		return err
	}
	if err != nil {
		return fmt.Errorf("jawaban relay tidak sampai, jadi %s tetap disimpan. "+
			"Jalankan program client; kalau ditolak, hapus %s dan minta undangan baru: %w", path, path, err)
	}
	fmt.Printf("Bergabung sebagai %s dengan alamat %s. Konfigurasi disimpan di %s.\n", res.Name, res.Address, path)
	return nil
}
```

Urutan di fungsi ini sengaja dibuat hati-hati. File konfigurasi ditulis lebih dulu, sebelum relay dihubungi, dengan `os.O_EXCL`, sehingga komputer yang sudah bergabung tidak bisa menimpa private key-nya sendiri.

Setelah relay dihubungi, ada tiga kemungkinan hasil. Ketiganya adalah:

- **Relay menerima undangan.** File konfigurasi sudah lengkap, dan program client bisa langsung dijalankan.
- **Relay menolak undangan.** File konfigurasi dihapus, karena kunci di dalamnya tidak terdaftar dan tidak akan pernah dipakai.
- **Jawaban relay tidak sampai.** Bisa saja relay sudah mendaftarkan public key itu sebelum koneksinya putus. Jadi file konfigurasi tetap disimpan, dan pesan error menjelaskan langkah berikutnya.

Program client di `cmd/client/main.go` memanggil `join` kalau kata pertamanya adalah `gabung`. Bagian awal `main` sekarang adalah:

```go
	path := flag.String("config", "client.json", "file konfigurasi")
	flag.Parse()
	if flag.Arg(0) == "gabung" {
		if err := join(*path, flag.Arg(1)); err != nil {
			log.Fatal(err)
		}
		return
	}
```

Bergabung tidak butuh hak administrator, karena hanya menulis file. Hak administrator baru dibutuhkan saat program client dijalankan dan membuat TUN device.

## Mencoba undangan VPN

File test baru di `cmd/relay/main_test.go` berisi lima test. Kelimanya adalah:

- **TestUndanganSekaliPakai.** Undangan dipakai sekali dan berhasil, client baru langsung bisa terhubung, lalu undangan yang sama ditolak.
- **TestUndanganKedaluwarsa.** Undangan yang sudah lewat masa berlakunya ditolak.
- **TestKeluarkan.** Tiga client bergabung, lalu client kedua dikeluarkan. Relay menolaknya dalam dua detik, dan alamatnya kosong lagi untuk client berikutnya.
- **TestAlamatBerikutnya.** Di jaringan kecil `10.10.0.0/29`, relay hanya membagikan `10.10.0.1` sampai `10.10.0.5`.
- **TestUbahBersamaan.** Dua puluh goroutine membuat undangan pada saat yang sama, dan kedua puluh undangan itu tersimpan semua.

Test pertama menjalani seluruh alur undangan. Kodenya adalah:

```go
// Undangan hanya bisa dipakai sekali. Client baru langsung dikenal relay,
// tanpa menunggu file konfigurasi dibaca ulang.
func TestUndanganSekaliPakai(t *testing.T) {
	path := startRelay(t)
	inv, err := createInvite(path, "kantor")
	if err != nil {
		t.Fatal(err)
	}
	// Undangan dikirim sebagai teks, lalu dibaca lagi di komputer baru.
	inv, err = invite.Parse(inv.String())
	if err != nil {
		t.Fatal(err)
	}

	priv, pub, _ := proof.NewKey()
	res, err := invite.Join(context.Background(), inv, hex.EncodeToString(pub[:]))
	if err != nil {
		t.Fatal(err)
	}
	if res.Name != "kantor" || res.Address != "10.10.0.1" {
		t.Fatalf("jawaban relay: %+v", res)
	}

	// Client baru langsung bisa terhubung ke relay.
	relayKey, _ := proof.ParseKey(inv.RelayKey)
	bind := &wsbind.Bind{URL: inv.Relay, PrivateKey: priv, RelayKey: relayKey}
	if _, _, err := bind.Open(0); err != nil {
		t.Fatal(err)
	}
	bind.Close()
	if bind.Welcome().Address != "10.10.0.1/24" {
		t.Fatalf("alamat dari relay: %s", bind.Welcome().Address)
	}

	// Undangan yang sama tidak bisa dipakai lagi.
	_, other, _ := proof.NewKey()
	_, err = invite.Join(context.Background(), inv, hex.EncodeToString(other[:]))
	if !errors.Is(err, invite.ErrRefused) || !strings.Contains(err.Error(), "undangan tidak berlaku") {
		t.Fatalf("undangan dipakai dua kali: %v", err)
	}
}
```

`startRelay` membuat `relay.json` baru di folder sementara, lalu menjalankan relay dengan `newHandler`, persis seperti perintah `run`. Hasil test-nya, tanpa baris log, adalah:

```text
$ go test -v ./cmd/relay/
=== RUN   TestUndanganSekaliPakai
--- PASS: TestUndanganSekaliPakai (0.03s)
=== RUN   TestUndanganKedaluwarsa
--- PASS: TestUndanganKedaluwarsa (0.01s)
=== RUN   TestKeluarkan
--- PASS: TestKeluarkan (2.05s)
=== RUN   TestAlamatBerikutnya
--- PASS: TestAlamatBerikutnya (0.00s)
=== RUN   TestUbahBersamaan
--- PASS: TestUbahBersamaan (1.82s)
PASS
ok  	vpnku/cmd/relay	3.911s
```

### Percobaan undangan dengan virtual network card sungguhan

Alur yang sama juga dijalankan dengan virtual network card sungguhan, di satu mesin Linux dengan network namespace seperti di Bagian 5. Relay berada di `192.0.2.1`, dan dihubungi lewat `ws://` tanpa Caddy.

Perintah di bawah ditulis tanpa `-config` dan tanpa path folder percobaan, supaya mudah dibaca. Relay dibuat, dijalankan, lalu membuat undangan untuk `rumah`, yang dipendekkan di sini. Hasilnya adalah:

```text
$ relay init ws://192.0.2.1:8080
relay.json dibuat. Public key relay: 4956a6e350931c22ed16c3502c43325a7acb4d7648ebb63f8975054184426f1e
$ relay -listen 192.0.2.1:8080 run &
$ relay undang rumah
Undangan untuk rumah, sekali pakai dan berlaku 24 jam:
vpnku-eyJyZWxheSI6IndzOi8vMTkyLjAuMi4xOjgwODAiLCJyZWxheV9rZXkiOiI0OTU2YTZl...
```

Komputer A bergabung sebagai `rumah`, dan komputer B sebagai `kantor` dengan undangannya sendiri. Lalu komputer ketiga mencoba memakai undangan `rumah` sekali lagi. Hasilnya adalah:

```text
A$ client -config a.json gabung vpnku-eyJyZWxheSI6...
Bergabung sebagai rumah dengan alamat 10.10.0.1. Konfigurasi disimpan di a.json.
B$ client -config b.json gabung vpnku-eyJyZWxheSI6...
Bergabung sebagai kantor dengan alamat 10.10.0.2. Konfigurasi disimpan di b.json.
X$ client -config x.json gabung vpnku-eyJyZWxheSI6...
2026/10/11 14:39:33 relay menolak: undangan tidak berlaku
```

Komputer ketiga ditolak, dan tidak ada file konfigurasi yang tertinggal di sana. Setelah program client dijalankan di A dan B, A bisa melakukan ping ke `10.10.0.2` tanpa satu pun kunci yang disalin dengan tangan.

Admin lalu melihat daftar client, mengeluarkan `kantor`, dan melihat daftarnya lagi. Public key disembunyikan di sini, dan hasilnya adalah:

```text
$ relay daftar
rumah            10.10.0.1       …
kantor           10.10.0.2       …
laptop           undangan, berlaku sampai 2026-10-12 14:39
$ relay keluarkan kantor
kantor dikeluarkan
$ relay daftar
rumah            10.10.0.1       …
laptop           undangan, berlaku sampai 2026-10-12 14:39
```

Satu detik setelah `relay keluarkan`, relay membaca ulang `relay.json` dan memutus B dengan alasan "bukan client terdaftar". Ping dari A ke `10.10.0.2` pun gagal.

## Kelemahan VPN versi keenam

Menambah dan mengeluarkan client sekarang cukup satu perintah. Tetapi masih ada enam kelemahan yang perlu kamu tahu. Keenamnya adalah:

- **Undangan adalah rahasia selama berlaku.** Siapa pun yang membaca undangan lebih dulu bisa bergabung dengan nama itu. Kirim undangan lewat jalur yang kamu percaya, dan periksa `relay daftar` setelah client baru bergabung.
- **Mengubah `relay.json` dengan tangan tidak mengikuti file penanda.** Perubahan tangan yang terjadi tepat saat seseorang bergabung bisa hilang.
- **File penanda bisa tertinggal** kalau sebuah program mati di tengah perubahan. Program berikutnya berhenti dengan pesan untuk menghapusnya.
- **`relay daftar` hanya membaca file.** Daftar ini tidak menunjukkan client mana yang sedang terhubung.
- **Client masih dijangkau lewat alamat IP.** Nama seperti `kantor` hanya dipakai admin, belum oleh aplikasi.
- **Relay masih dipercaya menentukan client.** Kelemahan dari Bagian 4 ini belum berubah.

## Berikutnya dalam seri ini

Di Bagian 7, nama client mulai dipakai oleh aplikasi. Daripada mengingat `10.10.0.2`, kamu cukup menulis nama komputernya, misalnya untuk SSH atau remote desktop. Setiap client menjawab pertanyaan nama itu sendiri, dari daftar client yang sudah dikirim relay, tanpa server nama terpisah.
