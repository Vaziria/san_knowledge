# [Seri Membuat VPN Sendiri Bagian 7] Menjangkau client VPN lewat nama

*Di bagian ini, setiap client VPN mendapat nama seperti kantor.vpn.internal. Nama itu dijawab oleh program client sendiri, langsung di dalam tunnel, tanpa server nama terpisah.*

Di Bagian 6, setiap client mendapat nama saat bergabung, seperti `rumah` atau `kantor`. Tetapi nama itu baru dipakai admin, untuk mengeluarkan client. Aplikasi masih harus memakai alamat IP, seperti `ssh 10.10.0.2`.

Di Bagian 7, nama itu mulai dipakai aplikasi. Di akhir bagian ini, kamu bisa menulis `ssh kantor.vpn.internal`, dan nama itu dijawab oleh program client di komputermu sendiri, dari daftar client yang sudah dikirim relay.

## Kenapa alamat IP saja tidak cukup untuk VPN?

Alamat seperti `10.10.0.2` sulit diingat, apalagi kalau client-nya belasan. Alamat itu juga bisa berubah. Client yang dikeluarkan lalu bergabung lagi mendapat alamat kosong berikutnya, dan semua konfigurasi yang menulis alamat lamanya ikut rusak.

Internet sudah lama menyelesaikan masalah yang sama dengan DNS. DNS adalah sistem yang menerjemahkan nama menjadi alamat IP. Komputer yang ingin tahu alamat `example.com` mengirim pertanyaan ke server DNS, yaitu server yang menyimpan jawaban untuk nama-nama itu, lalu memakai alamat yang dijawabnya.

VPN kita butuh hal yang sama untuk nama client-nya. Pertanyaannya adalah siapa yang menjawab, dan bagaimana OS tahu harus bertanya ke mana.

## Tiga cara memberi nama di VPN

Ada tiga cara yang umum untuk menjawab nama di jaringan privat. Ketiganya adalah:

- **File hosts.** Setiap OS punya file teks berisi pasangan nama dan alamat, yaitu `/etc/hosts` di Linux dan `C:\Windows\System32\drivers\etc\hosts` di Windows. Program client cukup menulis daftar client ke file itu. Masalahnya, program harus menulis ulang file sistem setiap kali daftar berubah, dan kalau program mati di tengah jalan, isi lamanya tertinggal.
- **Server DNS di relay.** Relay sudah tahu semua client, jadi relay bisa menjawab pertanyaan nama. Masalahnya, setiap pertanyaan menempuh internet sampai ke relay, dan semua nama berhenti bekerja saat relay tidak bisa dihubungi.
- **Setiap client menjawab sendiri.** Program client sudah memegang daftar client terbaru sejak Bagian 5, lengkap dengan nama dan alamatnya. Jadi program client bisa menjawab pertanyaan nama sendiri, tanpa menulis file sistem dan tanpa bertanya ke relay.

Seri ini memilih cara ketiga. Jawabannya datang seketika dari memori, dan tetap bekerja walaupun relay sedang restart. Kelemahannya, program client harus membaca dan menulis paket DNS sendiri, dan OS harus diatur supaya bertanya ke program itu.

## Memilih domain untuk VPN

Domain adalah bagian belakang sebuah nama, misalnya `example.com` di `www.example.com`, dan domain untuk nama client VPN tidak boleh bentrok dengan nama yang sudah ada di internet. Ada tiga pilihan yang sering dipakai. Ketiganya adalah:

- **Domain karangan sendiri, seperti `.lan`.** Domain ini tidak ada di internet hari ini. Masalahnya, ICANN, yaitu lembaga yang mengatur domain tingkat atas di internet, bisa saja membukanya suatu hari, dan nama VPN-mu akan bentrok dengan nama di internet.
- **`.local`.** Domain ini terdengar cocok, tetapi sudah dipakai untuk multicast DNS menurut RFC 6762, yaitu cara perangkat di satu LAN saling mengumumkan nama, seperti printer. Pertanyaan untuk nama `.local` dikirim ke semua perangkat di LAN, bukan ke server DNS, jadi tidak akan pernah sampai ke program client.
- **`.internal`.** Pada 29 Juli 2024, ICANN mencadangkan `.internal` secara permanen untuk jaringan privat. Domain ini tidak akan pernah dipakai di internet.

Seri ini memakai `.internal`. Tetapi jaringan kantor atau penyedia cloud bisa saja sudah memakai `.internal` untuk namanya sendiri. Jadi VPN kita tidak mengambil seluruh `.internal`, melainkan satu cabangnya saja, yaitu `vpn.internal`.

Kelemahannya, nama menjadi lebih panjang, seperti `kantor.vpn.internal`. Domain ini disimpan di `relay.json` dan bisa diganti, misalnya menjadi `rumah.internal`, sebelum relay dijalankan.

## Anatomi nama di dalam VPN

Nama di dalam VPN dibangun dari empat bagian. Keempatnya adalah:

- **Nama dan domain dari relay.** Pesan sambutan kini berisi nama client itu, domain VPN, dan nama setiap peer.
- **Alamat DNS di dalam VPN.** Di Bagian 6, relay menyisihkan alamat sebelum broadcast, yaitu `10.10.0.254`. Alamat inilah yang dipakai OS untuk bertanya nama.
- **Resolver di program client.** Resolver adalah bagian yang menjawab pertanyaan nama. Resolver kita membungkus TUN device, sehingga paket ke `10.10.0.254` dijawab sebelum sampai ke wireguard-go.
- **Aturan DNS di OS.** OS diberi tahu bahwa nama yang berakhiran `vpn.internal`, dan hanya nama itu, ditanyakan ke `10.10.0.254`. Nama lain tetap ditanyakan ke server DNS biasa. Cara ini disebut split DNS.

![Di dalam komputer A, aplikasi menjalankan ssh kantor.vpn.internal dan bertanya ke OS. OS mengirim nama di domain vpn.internal ke 10.10.0.254 lewat vpnku0, dan nama lain ke DNS biasa. Di program client, resolver menjawab pertanyaan dari vpnku0 dengan nama dari daftar client, sedangkan paket lain diteruskan ke wireguard-go lalu ke relay.](assets/names-inside-tunnel.png)

*Pertanyaan nama tidak pernah meninggalkan komputer A.*

Urutan satu pertanyaan nama terdiri dari lima langkah. Langkah-langkahnya adalah:

1. Aplikasi bertanya ke OS, berapa alamat `kantor.vpn.internal`.
2. OS melihat bahwa nama itu berakhiran `vpn.internal`, lalu mengirim pertanyaan DNS lewat UDP ke `10.10.0.254`, port 53.
3. Karena `10.10.0.254` berada di `10.10.0.0/24`, route dari Bagian 3 mengirim paket itu ke `vpnku0`.
4. Program client membaca paket itu dari `vpnku0`. Resolver melihat tujuannya `10.10.0.254` port 53, menyusun jawaban `10.10.0.2`, lalu menulis paket jawaban kembali ke `vpnku0`.
5. OS menerima jawaban itu seolah datang dari `10.10.0.254`, lalu memberikannya ke aplikasi. Setelah itu, aplikasi terhubung ke `10.10.0.2` lewat tunnel seperti biasa.

### Kenapa resolver tidak membuka port 53 saja?

Cara yang lebih umum adalah menjalankan server DNS biasa yang mendengarkan port 53 di alamat VPN. Library DNS sudah ada, dan kodenya lebih pendek. Masalahnya, banyak komputer sudah menjalankan server DNS sendiri, seperti dnsmasq atau Pi-hole, yang mendengarkan port 53 di semua alamat, sehingga program kita gagal membuka port yang sama.

Resolver di dalam tunnel tidak butuh port sama sekali. Paket ke `10.10.0.254` memang sudah masuk ke `vpnku0`, jadi program client cukup membacanya lebih dulu. Kelemahannya, kita menyusun header IPv4 dan UDP sendiri, dan resolver ini hanya mengerti IPv4 dan UDP.

### Jawaban DNS yang tepat untuk setiap pertanyaan

Resolver memberi tiga jenis jawaban. Ketiganya adalah:

- **Nama client** dijawab dengan alamatnya. Jawabannya berlaku 60 detik, jadi OS menyimpannya sebentar sebelum bertanya lagi.
- **Nama lain di domain VPN**, seperti `printer.vpn.internal`, dijawab "tidak ada". Aplikasi langsung tahu nama itu salah, tanpa menunggu.
- **Nama di luar domain VPN** ditolak. Pertanyaan seperti ini seharusnya tidak pernah sampai ke sini, karena OS hanya mengirim nama di domain VPN.

Ada satu kasus lagi. Banyak aplikasi menanyakan alamat IPv4 dan alamat IPv6 sekaligus, dan VPN kita hanya punya alamat IPv4. Pertanyaan IPv6 untuk nama client dijawab "ada, tetapi tanpa alamat", supaya aplikasi tidak menunggu jawaban yang tidak akan datang.

Batas 60 detik punya kelemahan. Kalau alamat sebuah client berubah, komputer lain bisa memakai alamat lama sampai satu menit.

### Mengatur OS supaya hanya nama VPN yang dikirim ke resolver

Setiap OS punya cara sendiri untuk split DNS. Di Linux, kita memakai systemd-resolved, yaitu layanan DNS bawaan Ubuntu dan Fedora. Perintahnya adalah:

```sh
resolvectl dns vpnku0 10.10.0.254
resolvectl domain vpnku0 ~vpn.internal
resolvectl default-route vpnku0 false
```

Perintah pertama memberi `vpnku0` server DNS. Perintah kedua menjadikan `vpn.internal` sebagai routing-only domain, yaitu domain yang ditandai `~` sehingga hanya nama di domain itu yang dikirim ke server tersebut. Perintah ketiga memastikan nama lain tidak pernah dikirim ke `vpnku0`.

Semua pengaturan ini hilang sendiri saat `vpnku0` hilang, jadi tidak ada yang perlu dibersihkan.

Di Windows, caranya adalah aturan NRPT, yaitu daftar aturan Windows yang menentukan server DNS untuk domain tertentu. Perintahnya, di PowerShell sebagai administrator, adalah:

```text
Add-DnsClientNrptRule -Namespace '.vpn.internal' -NameServers '10.10.0.254' -Comment 'vpnku'
```

Berbeda dengan Linux, aturan NRPT tetap ada setelah program berhenti. Jadi program client menghapusnya saat berhenti, dan menghapus sisa dari jalannya program sebelumnya saat mulai.

Ada satu jebakan di Windows. Perintah `nslookup` tidak mengikuti aturan NRPT, karena bertanya langsung ke server DNS biasa, jadi `nslookup kantor.vpn.internal` akan gagal walaupun namanya bekerja. Untuk mencoba, pakai `ping kantor.vpn.internal` atau `Resolve-DnsName kantor.vpn.internal`.

## Menulis resolver di dalam tunnel dengan Go

Resolver ada di package baru, `names/names.go`. Package ini memakai `golang.org/x/net/dns/dnsmessage` untuk membaca dan menyusun pesan DNS, jadi tambahkan library itu dengan `go get golang.org/x/net`. Kodenya adalah:

```go
// Package names menjawab pertanyaan DNS untuk nama client VPN, langsung di
// dalam tunnel, tanpa membuka port di OS.
package names

import (
	"encoding/binary"
	"net/netip"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/net/dns/dnsmessage"
	"golang.zx2c4.com/wireguard/tun"

	"vpnku/relay"
)

var domainPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)

// ValidDomain memeriksa domain VPN, misalnya vpn.internal.
func ValidDomain(d string) bool {
	return len(d) <= 200 && domainPattern.MatchString(d)
}

// Address mengembalikan alamat DNS di dalam VPN, yaitu alamat sebelum
// broadcast. Untuk 10.10.0.0/24, alamatnya 10.10.0.254.
func Address(network netip.Prefix) netip.Addr {
	a := network.Masked().Addr().As4()
	last := binary.BigEndian.Uint32(a[:]) | (1<<(32-network.Bits()) - 1)
	binary.BigEndian.PutUint32(a[:], last-1)
	return netip.AddrFrom4(a)
}

// Resolver menyimpan nama setiap client dan alamat VPN-nya.
type Resolver struct {
	mu     sync.Mutex
	server netip.Addr            // alamat DNS di dalam VPN
	domain string                // misalnya "vpn.internal."
	hosts  map[string]netip.Addr // misalnya "kantor.vpn.internal." → 10.10.0.2
}

// Set mengganti daftar nama dengan daftar client terbaru dari relay.
func (r *Resolver) Set(server netip.Addr, w relay.Welcome) {
	hosts := map[string]netip.Addr{}
	add := func(name, address string) {
		addr, err := netip.ParseAddr(strings.Split(address, "/")[0])
		if name != "" && err == nil {
			hosts[name+"."+w.Domain+"."] = addr
		}
	}
	add(w.Name, w.Address)
	for _, p := range w.Peers {
		add(p.Name, p.Address)
	}
	r.mu.Lock()
	r.server, r.domain, r.hosts = server, w.Domain+".", hosts
	r.mu.Unlock()
}

// answer menjawab satu pertanyaan DNS. Nama client dijawab dengan alamatnya,
// nama lain di domain VPN dijawab "tidak ada", dan nama di luar domain VPN
// ditolak.
func (r *Resolver) answer(query []byte) []byte {
	var p dnsmessage.Parser
	h, err := p.Start(query)
	if err != nil || h.Response {
		return nil
	}
	q, err := p.Question()
	if err != nil {
		return nil
	}
	name := strings.ToLower(q.Name.String())

	r.mu.Lock()
	addr, found := r.hosts[name]
	inDomain := strings.HasSuffix(name, "."+r.domain)
	r.mu.Unlock()

	rh := dnsmessage.Header{ID: h.ID, Response: true, Authoritative: true, RecursionDesired: h.RecursionDesired}
	switch {
	case found:
		rh.RCode = dnsmessage.RCodeSuccess
	case inDomain:
		rh.RCode = dnsmessage.RCodeNameError // nama ini tidak ada
	default:
		rh.RCode = dnsmessage.RCodeRefused // bukan urusan kita
	}
	b := dnsmessage.NewBuilder(nil, rh)
	b.StartQuestions()
	b.Question(q)
	b.StartAnswers()
	// Pertanyaan selain alamat IPv4, misalnya alamat IPv6, dijawab tanpa
	// isi, supaya aplikasi tidak menunggu.
	if found && q.Type == dnsmessage.TypeA && q.Class == dnsmessage.ClassINET {
		b.AResource(dnsmessage.ResourceHeader{Name: q.Name, Class: dnsmessage.ClassINET, TTL: 60},
			dnsmessage.AResource{A: addr.As4()})
	}
	msg, err := b.Finish()
	if err != nil {
		return nil
	}
	return msg
}

// handle memeriksa satu paket IPv4. Kalau paket itu pertanyaan DNS lewat UDP
// ke alamat DNS, handle mengembalikan paket jawabannya.
func (r *Resolver) handle(pkt []byte) []byte {
	if len(pkt) < 28 || pkt[0]>>4 != 4 || pkt[9] != 17 { // IPv4 dan UDP
		return nil
	}
	ihl := int(pkt[0]&0x0f) * 4
	if ihl < 20 || len(pkt) < ihl+8 {
		return nil
	}
	udpLen := int(binary.BigEndian.Uint16(pkt[ihl+4:]))
	if udpLen < 8 || len(pkt) < ihl+udpLen {
		return nil
	}
	r.mu.Lock()
	server := r.server
	r.mu.Unlock()
	udp := pkt[ihl : ihl+udpLen]
	if netip.AddrFrom4([4]byte(pkt[16:20])) != server || binary.BigEndian.Uint16(udp[2:]) != 53 {
		return nil
	}
	answer := r.answer(udp[8:])
	if answer == nil {
		return nil
	}

	// Jawaban dikirim dari alamat DNS ke penanya, dengan port ditukar.
	reply := make([]byte, 28+len(answer))
	reply[0] = 0x45 // IPv4, header 20 byte
	binary.BigEndian.PutUint16(reply[2:], uint16(len(reply)))
	reply[8] = 64 // TTL
	reply[9] = 17 // UDP
	copy(reply[12:16], pkt[16:20])
	copy(reply[16:20], pkt[12:16])
	binary.BigEndian.PutUint16(reply[10:], checksum(reply[:20]))
	copy(reply[20:22], udp[2:4])
	copy(reply[22:24], udp[0:2])
	binary.BigEndian.PutUint16(reply[24:], uint16(8+len(answer)))
	// Checksum UDP dibiarkan 0, yang berarti "tidak dihitung". IPv4
	// mengizinkannya, dan paket ini tidak pernah melewati kabel.
	copy(reply[28:], answer)
	return reply
}

// checksum menghitung checksum header IPv4.
func checksum(header []byte) uint16 {
	var sum uint32
	for i := 0; i < len(header); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(header[i:]))
	}
	for sum > 0xffff {
		sum = sum>>16 + sum&0xffff
	}
	return ^uint16(sum)
}

// Wrap membungkus TUN device. Pertanyaan DNS ke alamat DNS dijawab di sini,
// dan jawabannya dikembalikan ke OS lewat TUN device yang sama. Paket lain
// diteruskan ke wireguard-go tanpa diubah.
func Wrap(dev tun.Device, r *Resolver) tun.Device {
	return &device{Device: dev, r: r}
}

type device struct {
	tun.Device
	r *Resolver
}

func (d *device) Read(bufs [][]byte, sizes []int, offset int) (int, error) {
	for {
		n, err := d.Device.Read(bufs, sizes, offset)
		if err != nil {
			return n, err
		}
		kept := 0
		for i := range n {
			if reply := d.r.handle(bufs[i][offset : offset+sizes[i]]); reply != nil {
				out := make([]byte, offset+len(reply))
				copy(out[offset:], reply)
				d.Device.Write([][]byte{out}, offset)
				continue
			}
			if kept != i {
				copy(bufs[kept][offset:], bufs[i][offset:offset+sizes[i]])
				sizes[kept] = sizes[i]
			}
			kept++
		}
		if kept > 0 {
			return kept, nil
		}
	}
}
```

Kode ini terdiri dari empat bagian. Keempatnya adalah:

1. `ValidDomain` dan `Address`. `Address` menghitung alamat sebelum broadcast dengan menyalakan semua bit di belakang prefix, lalu menguranginya satu.
2. `Resolver` menyimpan nama lengkap setiap client, dalam huruf kecil dan diakhiri titik, seperti `kantor.vpn.internal.`. Titik di akhir adalah cara DNS menulis nama yang lengkap. `Set` dipanggil setiap kali relay mengirim daftar baru.
3. `answer` membaca satu pertanyaan DNS dan menyusun jawabannya, sesuai tiga jenis jawaban di atas. `handle` membungkus jawaban itu menjadi paket IPv4 dan UDP. Alamat dan port penanya ditukar dengan alamat dan port resolver. Checksum header IPv4, yaitu angka pemeriksa yang dihitung dari isi header, dihitung dengan fungsi `checksum`.
4. `Wrap` membungkus TUN device. Setiap kali wireguard-go membaca paket, pertanyaan DNS diambil dari kumpulan paket itu dan dijawab, dan paket lain diteruskan. Paket yang tersisa disalin ke depan, bukan ditukar tempatnya, karena wireguard-go memasangkan setiap posisi dengan memori paketnya sendiri.

Checksum UDP dibiarkan 0. Menurut RFC 768, angka 0 berarti pengirim tidak menghitung checksum, dan IPv4 mengizinkannya. Paket jawaban ini tidak pernah melewati kabel, karena langsung ditulis ke OS di komputer yang sama.

## Mengirim nama dan domain dari relay

Relay kini menyimpan nama setiap client, dan mengirimnya bersama domain VPN. Type `Member` mendapat nama, dan isinya adalah:

```go
// Member adalah satu client yang terdaftar di relay.
type Member struct {
	Name      string
	PublicKey [KeyLen]byte
	Address   netip.Addr
}
```

Pesan sambutan dan setiap peer juga mendapat nama, dan pesan sambutan mendapat domain. Kodenya adalah:

```go
// Welcome dikirim relay sebagai pesan teks saat client lolos pemeriksaan,
// dan setiap kali daftar client berubah.
type Welcome struct {
	Name    string `json:"name"`    // nama client ini
	Address string `json:"address"` // alamat VPN client ini, dengan prefix
	Domain  string `json:"domain"`  // domain VPN, misalnya vpn.internal
	Peers   []Peer `json:"peers"`   // semua client lainnya
}

type Peer struct {
	Name      string `json:"name"`
	PublicKey string `json:"public_key"` // hex
	Address   string `json:"address"`
}
```

`Relay` menyimpan domain, dan `members` kini menyimpan seluruh `Member`, bukan hanya alamatnya. Kodenya adalah:

```go
type Relay struct {
	key     [KeyLen]byte // private key relay
	network netip.Prefix
	domain  string

	mu      sync.Mutex
	members map[[KeyLen]byte]Member
	online  map[[KeyLen]byte]*conn
}

func New(key [KeyLen]byte, network netip.Prefix, domain string, members []Member) *Relay {
	r := &Relay{key: key, network: network, domain: domain, online: map[[KeyLen]byte]*conn{}}
	r.SetMembers(members)
	return r
}
```

`welcome` mengisi nama, alamat, dan domain. Kodenya adalah:

```go
// welcome menyusun alamat client dan daftar client lainnya. Dipanggil saat
// r.mu terkunci.
func (r *Relay) welcome(me [KeyLen]byte) Welcome {
	m := r.members[me]
	w := Welcome{
		Name:    m.Name,
		Address: netip.PrefixFrom(m.Address, r.network.Bits()).String(),
		Domain:  r.domain,
	}
	for key, p := range r.members {
		if key != me {
			w.Peers = append(w.Peers, Peer{Name: p.Name, PublicKey: hex.EncodeToString(key[:]), Address: p.Address.String()})
		}
	}
	return w
}
```

Di `cmd/relay/config.go`, `Config` mendapat field `Domain`, dan `check` memeriksanya. Relay dari Bagian 6 yang belum punya domain memakai `vpn.internal`. Bagian `check` yang baru adalah:

```go
	if s.domain = c.Domain; s.domain == "" {
		s.domain = "vpn.internal"
	}
	if !names.ValidDomain(s.domain) {
		return s, fmt.Errorf("domain %q tidak sah", s.domain)
	}
```

`initConfig` kini menulis `"domain": "vpn.internal"` ke `relay.json` yang baru, dan `watch` menolak perubahan domain tanpa restart, sama seperti private key dan rentang alamat.

## Mengubah program client VPN untuk nama

Program client membungkus TUN device dengan resolver, lalu mengatur DNS di OS setelah pesan sambutan pertama. Bagian `main` dari langkah kedua sampai akhir adalah:

```go
	// 2. WireGuard dengan Bind WebSocket. Up memanggil Open, yang terhubung ke
	// relay dan menjalani pemeriksaan. TUN device dibungkus resolver, yang
	// menjawab pertanyaan nama sebelum paketnya sampai ke wireguard-go.
	resolver := &names.Resolver{}
	dev := device.NewDevice(names.Wrap(tunDev, resolver), bind, device.NewLogger(device.LogLevelError, ""))
	defer dev.Close()
	if err := dev.IpcSet("private_key=" + cfg.PrivateKey + "\n"); err != nil {
		log.Fatal(err)
	}
	if err := dev.Up(); err != nil {
		log.Fatal(err)
	}

	// 3. Beri tahu OS alamat VPN kita, dari pesan sambutan pertama.
	welcome := bind.Welcome()
	network, err := netip.ParsePrefix(welcome.Address)
	if err != nil {
		log.Fatal(err)
	}
	if err := setupOS(name, welcome.Address); err != nil {
		log.Fatal(err)
	}
	log.Printf("%s aktif dengan alamat %s. Tekan Ctrl+C untuk berhenti.", name, welcome.Address)

	// 4. Minta OS menanyakan nama di domain VPN ke alamat DNS di dalam VPN.
	// Kalau gagal, VPN tetap berjalan, hanya tanpa nama.
	server := names.Address(network)
	if !names.ValidDomain(welcome.Domain) {
		log.Printf("domain %q dari relay tidak sah, jadi nama client tidak dipakai", welcome.Domain)
	} else if undo, err := setupDNS(name, server, welcome.Domain); err != nil {
		log.Printf("nama client tidak bisa dipakai: %v", err)
	} else {
		defer undo()
		log.Printf("komputer ini bernama %s.%s, dan nama dijawab di %s", welcome.Name, welcome.Domain, server)
	}

	// 5. Atur daftar peer dan daftar nama sekarang, lalu setiap kali relay
	// mengirim daftar baru, sampai program dihentikan.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	peers := map[string]bool{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-bind.Updates:
			w := bind.Welcome()
			if peers, err = setPeers(dev, peers, w.Peers); err != nil {
				log.Print(err)
			}
			resolver.Set(server, w)
		}
	}
}
```

Ada tiga hal yang perlu diperhatikan. Ketiganya adalah:

1. Resolver dibuat sebelum device, tetapi baru mendapat nama dan alamat DNS saat daftar client pertama datang. Sampai saat itu, resolver tidak menjawab apa pun.
2. Domain dari relay diperiksa dengan `ValidDomain` sebelum dipakai. Di Windows, domain itu masuk ke perintah PowerShell, jadi domain yang aneh tidak boleh lewat, walaupun relay sudah membuktikan identitasnya.
3. Kalau OS tidak bisa diatur, misalnya karena Linux-nya tidak memakai systemd-resolved, program hanya mencetak alasannya. VPN tetap berjalan, dan client tetap bisa dijangkau lewat alamat IP.

`setupDNS` di Linux ada di `cmd/client/os_linux.go`, dan isinya adalah:

```go
// setupDNS meminta systemd-resolved menanyakan nama di domain VPN ke alamat
// DNS di dalam VPN, dan hanya nama itu. Pengaturan ini hilang sendiri saat
// TUN device hilang, jadi tidak ada yang perlu dibersihkan.
func setupDNS(name string, server netip.Addr, domain string) (func(), error) {
	err := run(
		[]string{"resolvectl", "dns", name, server.String()},
		[]string{"resolvectl", "domain", name, "~" + domain},
		[]string{"resolvectl", "default-route", name, "false"},
	)
	return func() {}, err
}
```

`setupDNS` di Windows ada di `cmd/client/os_windows.go`, dan isinya adalah:

```go
// setupDNS membuat aturan NRPT, supaya Windows menanyakan nama di domain VPN
// ke alamat DNS di dalam VPN. Aturan itu dihapus lagi saat program berhenti.
func setupDNS(_ string, server netip.Addr, domain string) (func(), error) {
	remove := []string{"powershell", "-NoProfile", "-Command",
		"Get-DnsClientNrptRule | Where-Object Comment -eq 'vpnku' | Remove-DnsClientNrptRule -Force"}
	undo := func() { exec.Command(remove[0], remove[1:]...).Run() }
	undo() // aturan lama dari jalannya program sebelumnya, kalau ada

	err := run([]string{"powershell", "-NoProfile", "-Command", fmt.Sprintf(
		"Add-DnsClientNrptRule -Namespace '.%s' -NameServers '%s' -Comment 'vpnku'", domain, server)})
	return undo, err
}
```

Kedua file juga mendapat import `net/netip`.

## Mencoba nama client VPN

Package `names` punya tiga test sendiri. Dua yang pertama menguji jawaban dan paket, yaitu `TestJawabanDNS` untuk setiap jenis pertanyaan, dan `TestPaketDNS` untuk alamat, port, dan checksum paket jawaban. Yang ketiga, `TestAlamatDNS`, menguji `Address` untuk beberapa ukuran jaringan.

Test jawaban memakai tabel, yaitu daftar pertanyaan beserta jawaban yang diharapkan. Kodenya adalah:

```go
// Setiap jenis pertanyaan mendapat jawaban yang tepat.
func TestJawabanDNS(t *testing.T) {
	r := newResolver()
	tests := []struct {
		name  string
		typ   dnsmessage.Type
		rcode dnsmessage.RCode
		addr  string // kosong kalau tidak ada alamat di jawaban
	}{
		{"kantor.vpn.internal.", dnsmessage.TypeA, dnsmessage.RCodeSuccess, "10.10.0.2"},
		{"KANTOR.Vpn.Internal.", dnsmessage.TypeA, dnsmessage.RCodeSuccess, "10.10.0.2"},
		{"rumah.vpn.internal.", dnsmessage.TypeA, dnsmessage.RCodeSuccess, "10.10.0.1"},
		{"kantor.vpn.internal.", dnsmessage.TypeAAAA, dnsmessage.RCodeSuccess, ""},
		{"printer.vpn.internal.", dnsmessage.TypeA, dnsmessage.RCodeNameError, ""},
		{"example.com.", dnsmessage.TypeA, dnsmessage.RCodeRefused, ""},
	}
	for _, tt := range tests {
		var p dnsmessage.Parser
		h, err := p.Start(r.answer(query(t, tt.name, tt.typ)))
		if err != nil {
			t.Fatal(err)
		}
		p.SkipAllQuestions()
		answers, _ := p.AllAnswers()
		got := ""
		if len(answers) == 1 {
			got = netip.AddrFrom4(answers[0].Body.(*dnsmessage.AResource).A).String()
		}
		if h.ID != 7 || h.RCode != tt.rcode || got != tt.addr {
			t.Errorf("%s %v: rcode %v, alamat %q", tt.name, tt.typ, h.RCode, got)
		}
	}
}
```

Di `wsbind`, helper `member` kini membungkus netstack dengan resolver yang sama, dan netstack diberi `10.10.0.254` sebagai server DNS. Test barunya memanggil web server B lewat nama. Kodenya adalah:

```go
package wsbind_test

import "testing"

// Client saling menjangkau lewat nama. Pertanyaan nama dijawab di dalam
// tunnel, dari daftar client yang dikirim relay.
func TestNamaClient(t *testing.T) {
	relayPriv, relayPub := newKey(t)
	privA, pubA := newKey(t)
	privB, pubB := newKey(t)
	_, url := startRelay(t, relayPriv, pubA, pubB) // rumah dan kantor

	netA, _ := member(t, url, privA, relayPub)
	netB, _ := member(t, url, privB, relayPub)
	serve(t, netB, "10.10.0.2")
	get(t, netA, "http://kantor.vpn.internal/")

	if _, err := netA.LookupHost("printer.vpn.internal"); err == nil {
		t.Fatal("nama yang tidak ada mendapat alamat")
	}
}
```

Hasil test-nya adalah:

```text
$ go test -v ./names/
=== RUN   TestJawabanDNS
--- PASS: TestJawabanDNS (0.00s)
=== RUN   TestPaketDNS
--- PASS: TestPaketDNS (0.00s)
=== RUN   TestAlamatDNS
--- PASS: TestAlamatDNS (0.00s)
PASS
ok  	vpnku/names	0.569s
$ go test -v -run TestNamaClient ./wsbind/
=== RUN   TestNamaClient
--- PASS: TestNamaClient (0.02s)
PASS
ok  	vpnku/wsbind	1.378s
```

### Percobaan nama dengan virtual network card sungguhan

Nama juga dicoba dengan virtual network card sungguhan, di satu mesin Linux dengan systemd-resolved. Client `rumah` berjalan langsung di mesin itu, supaya systemd-resolved melihat `vpnku0`-nya. Client `kantor` berjalan di network namespace sendiri, dan memakai `10.10.0.254` sebagai server DNS biasanya.

Dari `kantor`, `dig` bertanya langsung ke `10.10.0.254`. Nama client mendapat alamatnya, nama yang tidak ada dijawab `NXDOMAIN`, dan nama di luar domain ditolak dengan `REFUSED`. Hanya baris status dan jawaban yang ditampilkan, dan hasilnya adalah:

```text
$ dig rumah.vpn.internal
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 22024
rumah.vpn.internal.	60	IN	A	10.10.0.1
$ dig printer.vpn.internal
;; ->>HEADER<<- opcode: QUERY, status: NXDOMAIN, id: 44607
$ dig example.com
;; ->>HEADER<<- opcode: QUERY, status: REFUSED, id: 13369
$ ping rumah.vpn.internal
PING rumah.vpn.internal (10.10.0.1) 56(84) bytes of data.
64 bytes from 10.10.0.1: icmp_seq=1 ttl=64 time=7.12 ms
```

Dari `rumah`, `resolvectl` menunjukkan split DNS yang diatur program client. Nama di `vpn.internal` dijawab lewat `vpnku0`, sedangkan `example.com` tetap dijawab server DNS biasa. Potongan hasilnya adalah:

```text
$ resolvectl status vpnku0
Link 6 (vpnku0)
     Protocols: -DefaultRoute +LLMNR -mDNS -DNSOverTLS DNSSEC=no/unsupported
   DNS Servers: 10.10.0.254
    DNS Domain: ~vpn.internal
$ resolvectl query kantor.vpn.internal
kantor.vpn.internal: 10.10.0.2                              -- link: vpnku0
$ resolvectl query --cache=no example.com
example.com: 2606:4700:10::ac42:93f3
             2606:4700:10::6814:179a
             104.20.23.154
```

Setelah program client `rumah` dihentikan, `resolvectl status vpnku0` menjawab `No such device`. `vpnku0` hilang, dan pengaturan DNS-nya ikut hilang.

Bagian Windows, yaitu aturan NRPT, sudah diperiksa dengan `go vet` dan berhasil dibangun, tetapi belum dijalankan di Windows sungguhan. Sama seperti `os_windows.go` di Bagian 3, coba sekali dengan terminal administrator sebelum mengandalkannya.

## Kelemahan VPN versi ketujuh

Client VPN sekarang bisa dijangkau lewat nama. Tetapi masih ada enam kelemahan yang perlu kamu tahu. Keenamnya adalah:

- **Hanya IPv4 dan UDP.** Resolver tidak menjawab pertanyaan DNS lewat TCP. Jawaban kita selalu kecil, jadi OS tidak pernah perlu mengulang lewat TCP.
- **Hanya nama client.** Komputer lain di LAN kantor, di belakang sebuah client, tidak punya nama di VPN, karena VPN ini memang hanya menghubungkan client-nya.
- **Linux tanpa systemd-resolved tidak mendapat nama.** VPN tetap berjalan lewat alamat IP.
- **Aturan NRPT di Windows belum dijalankan.** Kodenya baru diperiksa, belum dicoba di Windows sungguhan.
- **Perubahan alamat terlambat sampai satu menit** di komputer yang sudah menyimpan jawaban lama.
- **Nama dan domain datang dari relay.** Relay yang dicuri kuncinya bisa memberi nama palsu, sama seperti bisa menambah client palsu sejak Bagian 4.

## Berikutnya dalam seri ini

Sejak Bagian 3, relay berjalan di VPS yang punya IP publik, domain, dan Caddy. Tidak semua orang punya ketiganya. Di Bagian 8, kita menjalankan relay tanpa server sendiri, yaitu di belakang layanan tunnel atau di layanan container, dan menangani satu masalah baru yang datang bersamanya, yaitu koneksi yang diputus setiap jam.
