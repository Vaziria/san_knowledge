# [Seri Membuat Message Broker Sendiri Bagian 6] Menjalankan broker dan menutup seri

*Di bagian terakhir ini, broker yang sama dijalankan dengan tiga cara, yaitu tertanam di program Go, dipasang di server HTTP milik aplikasi, dan berdiri sendiri dengan CLI. Lalu kita merangkum seri ini, mendaftar kelemahan seluruh rancangannya, dan membahas kapan RabbitMQ, NATS, atau Kafka adalah pilihan yang lebih baik.*

Seri ini membangun `brokerku`, message broker kecil dengan Go, dari nol. Message broker adalah server yang menerima, menyimpan, dan mengantar pesan di antara program. Producer mengirim pesan ke sebuah topic, yaitu nama sebuah log pesan seperti `pesanan`, dan broker mengantarnya ke setiap subscriber, yaitu penerima bernama yang terdaftar di topic itu.

Di Bagian 5, satu pengirim webhook diganti dengan worker pool, yaitu sekumpulan goroutine, yang disebut worker, yang bergantian memanggil webhook. Webhook adalah alamat HTTP milik subscriber yang dipanggil broker untuk setiap pesan, dan goroutine adalah fungsi Go yang berjalan bersamaan dengan fungsi lain.

Di Bagian 6, kita menjalankan broker itu dengan tiga cara, menulis CLI, yaitu program yang dipakai lewat perintah di terminal, dan menambah daftar subscriber beserta percobaan ulangnya. Setelah itu, kita merangkum keenam bagian, mendaftar kelemahan rancangannya, dan membandingkannya dengan RabbitMQ, NATS, dan Kafka.

## Tiga cara menjalankan broker yang sama

Sejak Bagian 2, broker kita adalah package Go bernama `broker`, dan API HTTP dari Bagian 3 hanyalah satu cara memakainya. `broker.Open` membuka folder data, method seperti `CreateTopic` dan `Publish` bisa dipanggil langsung, dan `Handler` mengembalikan `http.Handler`, yaitu nilai Go yang menjawab request HTTP, berisi seluruh API broker.

Dari situ, ada tiga cara menjalankannya. Ketiganya adalah:

- **Tertanam.** Program Go membuka broker di dalam dirinya sendiri, lalu memanggil `Publish` sebagai fungsi biasa. Tidak ada jaringan antara kode program dan broker.
- **Dipasang di server aplikasi.** Aplikasi yang sudah punya server HTTP memasang `Handler` di bawah sebuah alamat, misalnya `/broker/`. Program lain bisa memakai broker lewat port aplikasi itu.
- **Berdiri sendiri.** Program `brokerku server` menjalankan broker sebagai server tersendiri, dan program lain, termasuk CLI `brokerku`, memanggilnya lewat HTTP.

![Tiga kolom. Tertanam, kode toko memanggil b.Publish ke broker di dalam program yang sama, lalu broker memanggil webhook stok, dan broker hidup dan mati bersama programnya. Dipasang di server aplikasi, POST /pesanan di server HTTP aplikasi port 8090 memanggil b.Publish, dan broker di /broker/ dipakai pembaca dan CLI lewat port yang sama. Berdiri sendiri, brokerku server di port 8080 berisi broker, dan toko, pembaca, serta CLI memanggilnya lewat HTTP.](assets/run-modes.png)

*Kodenya sama. Yang berbeda hanya siapa yang memanggil `Open`, dan di mana `Handler` dipasang.*

### Bukankah broker seharusnya selalu berdiri sendiri?

RabbitMQ, NATS, dan Kafka dijalankan sebagai server tersendiri, dan ada alasan yang kuat untuk itu. Broker yang berdiri sendiri bisa di-restart, yaitu dimatikan lalu dinyalakan lagi, diperbarui, dan diberi mesin sendiri, tanpa menyentuh program yang memakainya.

Tetapi untuk aplikasi kecil dengan satu server, broker yang berdiri sendiri berarti satu layanan lagi yang harus dijalankan, dipantau, dan dicadangkan. Broker yang tertanam ikut dijalankan bersama aplikasinya, dan `Publish` hanyalah panggilan fungsi yang menulis ke file, tanpa koneksi yang bisa putus.

Ada hal yang dikorbankan. Broker yang tertanam hidup dan mati bersama programnya, jadi program yang mati mendadak juga menghentikan pengantaran pesan, dan program itu harus berbagi CPU dan memori dengan broker. Karena itu, seri ini tidak memilih salah satu, tetapi membuat ketiganya mungkin dengan package yang sama.

## Tertanam di program Go

Program contoh `tertanam` adalah program toko yang mencatat tiga pesanan. Untuk setiap pesanan, ia memanggil `Publish` secara langsung, dan broker di dalamnya memanggil webhook `stok`, tanpa server broker yang terpisah. Seluruh isi `contoh/tertanam/main.go` adalah:

```go
// Program tertanam menunjukkan broker yang dipasang di dalam program lain.
// Program toko ini mencatat pesanan, lalu menerbitkan pesannya dengan
// memanggil fungsi Publish, tanpa server broker yang terpisah.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"time"

	"brokerku/broker"
)

func main() {
	webhook := flag.String("webhook", "http://127.0.0.1:9001/", "webhook untuk subscriber stok")
	flag.Parse()

	b, err := broker.Open("data-toko")
	if err != nil {
		log.Fatal(err)
	}
	defer b.Close()

	// Topic dan subscriber cukup dibuat sekali. Saat program dijalankan
	// lagi, keduanya sudah ada, jadi ErrExists diabaikan.
	if err := b.CreateTopic("pesanan"); err != nil && !errors.Is(err, broker.ErrExists) {
		log.Fatal(err)
	}
	err = b.CreateWebhook("stok", "pesanan", *webhook)
	if err != nil && !errors.Is(err, broker.ErrExists) {
		log.Fatal(err)
	}

	for n := 1041; n <= 1043; n++ {
		// ... pesanan n disimpan di database toko ...
		id, err := b.Publish("pesanan", fmt.Appendf(nil, "pesanan %d dibuat", n))
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("pesanan %d dicatat, pesan %d diterbitkan", n, id)
	}

	// Program contoh ini berhenti sebentar lagi. Pesan yang belum sempat
	// diantar tetap ada di log, dan diantar saat program berjalan lagi.
	time.Sleep(2 * time.Second)
}
```

Topic dan subscriber cukup dibuat sekali, karena keduanya disimpan di folder data. Saat program dijalankan lagi, `CreateTopic` dan `CreateWebhook` mengembalikan `ErrExists`, dan error itu diabaikan.

Program ini dijalankan dua kali berturut-turut, dengan `penerima` dari Bagian 4 sebagai webhook `stok` di port 9001. Catatan yang ditulis program itu adalah:

```text
$ tertanam
2026/11/07 23:31:59 pesanan 1041 dicatat, pesan 1 diterbitkan
2026/11/07 23:31:59 pesanan 1042 dicatat, pesan 2 diterbitkan
2026/11/07 23:31:59 pesanan 1043 dicatat, pesan 3 diterbitkan
$ tertanam
2026/11/07 23:32:01 pesanan 1041 dicatat, pesan 4 diterbitkan
2026/11/07 23:32:01 pesanan 1042 dicatat, pesan 5 diterbitkan
2026/11/07 23:32:01 pesanan 1043 dicatat, pesan 6 diterbitkan
```

Catatan `penerima` untuk kedua putaran itu adalah:

```text
2026/11/07 23:31:59 pesan 1, panggilan 1: pesanan 1041 dibuat, dijawab 200
2026/11/07 23:31:59 pesan 2, panggilan 1: pesanan 1042 dibuat, dijawab 200
2026/11/07 23:31:59 pesan 3, panggilan 1: pesanan 1043 dibuat, dijawab 200
2026/11/07 23:32:01 pesan 4, panggilan 1: pesanan 1041 dibuat, dijawab 200
2026/11/07 23:32:01 pesan 5, panggilan 1: pesanan 1042 dibuat, dijawab 200
2026/11/07 23:32:01 pesan 6, panggilan 1: pesanan 1043 dibuat, dijawab 200
```

Putaran kedua melanjutkan ID dari 4, dan `stok` tidak menerima pesan 1 sampai 3 lagi. Posisi baca `stok`, yaitu letak pesan berikutnya untuk subscriber itu di dalam log, tersimpan di folder data `data-toko` sejak Bagian 2.

Ada satu hal yang perlu diingat. `Publish` kembali setelah pesan ditulis ke file, tetapi seperti di Bagian 1, file itu baru dipaksa ke disk dengan `fsync` setiap detik. Kalau listrik padam tepat setelah `Publish`, pesan yang diterbitkan dalam detik terakhir bisa hilang, walaupun programnya sudah mencatat pesanan itu.

## Dipasang di server HTTP milik aplikasi

Program contoh `aplikasi` adalah server HTTP milik sebuah aplikasi. Ia punya alamatnya sendiri, `POST /pesanan`, yang mencatat pesanan lalu memanggil `Publish`, dan memasang seluruh API broker di bawah `/broker/`. Seluruh isi `contoh/aplikasi/main.go` adalah:

```go
// Program aplikasi menunjukkan broker yang menjadi satu bagian dari server
// HTTP milik aplikasi. Alamat aplikasi sendiri, POST /pesanan, berada di
// samping API broker, yang dipasang di bawah /broker/.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"

	"brokerku/broker"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:8090", "alamat yang didengarkan")
	flag.Parse()

	b, err := broker.Open("data-aplikasi")
	if err != nil {
		log.Fatal(err)
	}
	if err := b.CreateTopic("pesanan"); err != nil && !errors.Is(err, broker.ErrExists) {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /pesanan", func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			http.Error(w, "pesanan terlalu besar", http.StatusRequestEntityTooLarge)
			return
		}
		// ... pesanan disimpan di database aplikasi ...
		id, err := b.Publish("pesanan", data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "pesanan diterima, pesan %d\n", id)
	})
	// API broker dipasang di bawah /broker/. StripPrefix membuang awalan
	// itu, jadi /broker/topics/pesanan sampai di broker sebagai
	// /topics/pesanan.
	mux.Handle("/broker/", http.StripPrefix("/broker", b.Handler()))

	srv := &http.Server{Addr: *listen, Handler: mux}
	broker.ConfigureServer(srv) // HTTP/2 tanpa TLS dan ping, untuk stream pull

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		srv.Close()
	}()
	log.Printf("aplikasi berjalan di %s, broker di %s/broker", *listen, *listen)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Print(err)
	}
	if err := b.Close(); err != nil {
		log.Fatal(err)
	}
}
```

Menurut dokumentasi Go, `http.StripPrefix` membuang awalan dari alamat request, lalu meneruskannya ke handler, yaitu fungsi atau nilai yang menjawab request. Jadi `/broker/topics/pesanan` sampai di broker sebagai `/topics/pesanan`, dan kode broker tidak perlu tahu di mana ia dipasang.

`broker.ConfigureServer` dari Bagian 3 tetap dibutuhkan. Fungsi itu menyalakan HTTP/2 tanpa TLS dan ping di server aplikasi, karena stream pull, yaitu satu request HTTP/2 yang tetap terbuka tempat broker mengirim pesan satu per satu, membutuhkan keduanya.

Percobaannya memakai CLI yang ditulis di bawah, `pembaca` dari Bagian 3, dan `curl`. Variabel lingkungan `BROKERKU` membuat CLI memanggil broker di dalam aplikasi, dan perintahnya adalah:

```text
$ export BROKERKU=http://127.0.0.1:8090/broker
$ brokerku subscriber create -topic pesanan email
subscriber email dibuat, jenis pull, membaca topic pesanan
$ curl -X POST --data-binary "pesanan 1041 dibuat" http://127.0.0.1:8090/pesanan
pesanan diterima, pesan 1
$ brokerku publish pesanan "pesanan 1042 dibuat"
pesan 2 terkirim ke topic pesanan
$ brokerku subscriber list
NAMA   TOPIC    JENIS            GAGAL  DICOBA LAGI  WEBHOOK
email  pesanan  pull, terhubung  0      -            -
```

Pesan pertama diterbitkan oleh alamat milik aplikasi, dan pesan kedua lewat API broker di port yang sama. Catatan `pembaca` untuk subscriber pull `email` adalah:

```text
2026/11/07 23:32:06 pesan 1: pesanan 1041 dibuat
2026/11/07 23:32:06 pesan 2: pesanan 1042 dibuat
```

Kelemahannya ada di keamanan. API broker kita tidak punya autentikasi, jadi siapa pun yang bisa menjangkau port aplikasi kini bisa membuat topic, membuat subscriber, dan mengirim pesan. Karena `Handler` hanyalah `http.Handler`, aplikasi bisa membungkusnya dengan pemeriksaan login miliknya sendiri, tetapi seri ini tidak membangunnya.

## Berdiri sendiri, dengan CLI

Program `brokerku` dari Bagian 3 hanya bisa menjalankan server. Kini ia menjadi satu program dengan beberapa perintah, dan perintah selain `server` memanggil broker yang sedang berjalan lewat HTTP. Perintahnya adalah:

- **`brokerku server`** menjalankan broker, dengan flag `-listen`, `-data`, dan `-workers` dari bagian sebelumnya.
- **`brokerku topic create <topic>`** membuat topic.
- **`brokerku subscriber create -topic <topic> [-webhook <alamat>] <nama>`** membuat subscriber pull, atau subscriber webhook kalau `-webhook` diisi.
- **`brokerku subscriber list`** mendaftar semua subscriber, beserta percobaan ulangnya.
- **`brokerku publish <topic> <pesan>`** mengirim satu pesan.

Bukankah `curl` sudah cukup, karena API-nya HTTP biasa? Untuk membuat topic dan mengirim pesan, `curl` memang cukup, dan seri ini memakainya sejak Bagian 1. CLI memberi dua hal yang tidak diberikan `curl`, yaitu error yang mudah dibaca dan daftar subscriber dalam bentuk tabel.

### Daftar subscriber dan percobaan ulangnya

Sejak Bagian 4, subscriber webhook bisa sedang menunggu jeda percobaan ulang, tetapi satu-satunya cara melihatnya adalah membaca catatan broker. Kini broker punya method `Subscribers`, di file baru `broker/info.go`, yang seluruh isinya adalah:

```go
package broker

import (
	"cmp"
	"slices"
	"time"
)

// SubscriberInfo menggambarkan satu subscriber dan keadaan pengantarannya.
type SubscriberInfo struct {
	Name      string    `json:"name"`
	Topic     string    `json:"topic"`
	Webhook   string    `json:"webhook,omitempty"` // kosong untuk subscriber pull
	Connected bool      `json:"connected"`         // subscriber pull dengan stream yang terbuka
	Failures  int       `json:"failures"`          // percobaan yang gagal untuk pesan di posisi baca
	RetryAt   time.Time `json:"retry_at,omitzero"` // kapan pesan itu dicoba lagi
}

// Subscribers mengembalikan semua subscriber, urut menurut nama.
func (b *Broker) Subscribers() []SubscriberInfo {
	b.mu.Lock()
	defer b.mu.Unlock()
	list := make([]SubscriberInfo, 0, len(b.subs))
	for _, s := range b.subs {
		list = append(list, SubscriberInfo{
			Name:      s.name,
			Topic:     s.topic.name,
			Webhook:   s.webhook,
			Connected: b.streams[s.name] != nil,
			Failures:  s.failures,
			RetryAt:   s.retryAt,
		})
	}
	slices.SortFunc(list, func(x, y SubscriberInfo) int { return cmp.Compare(x.Name, y.Name) })
	return list
}
```

`Connected` bernilai true untuk subscriber pull yang stream-nya sedang terbuka. `Failures` dan `RetryAt` adalah jumlah percobaan yang gagal dan waktu percobaan berikutnya untuk pesan di posisi baca, yang disimpan di slot subscriber sejak Bagian 4.

Daftar itu dijawab oleh alamat baru, `GET /subscribers`. Di `broker/http.go`, tambahannya adalah:

```go
	mux.HandleFunc("GET /subscribers", func(w http.ResponseWriter, r *http.Request) {
		reply(w, http.StatusOK, b.Subscribers(), nil)
	})
```

Selebihnya, `http.go` sama persis dengan Bagian 4.

### Menulis CLI

Perintah dipilih dari dua kata pertama, dan setiap perintah punya `FlagSet`-nya sendiri, yaitu kumpulan flag milik satu perintah dari package `flag`. Seluruh isi `cmd/brokerku/main.go` kini adalah:

```go
// Program brokerku menjalankan broker sebagai server tersendiri, dan mengatur
// broker yang sedang berjalan lewat alamat HTTP-nya. Perintahnya adalah:
//
//	brokerku server [-listen 127.0.0.1:8080] [-data data] [-workers 1]
//	brokerku topic create <topic>
//	brokerku subscriber create -topic <topic> [-webhook <alamat>] <nama>
//	brokerku subscriber list
//	brokerku publish <topic> <pesan>
//
// Perintah selain server memanggil broker di -broker, atau di variabel
// lingkungan BROKERKU, atau di http://127.0.0.1:8080.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"

	"brokerku/broker"
)

const usage = `pemakaian:
  brokerku server [-listen alamat] [-data folder] [-workers n]
  brokerku topic create <topic>
  brokerku subscriber create -topic <topic> [-webhook alamat] <nama>
  brokerku subscriber list
  brokerku publish <topic> <pesan>`

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "brokerku:", err)
		os.Exit(1)
	}
}

// run menjalankan satu perintah dan menulis hasilnya ke out.
func run(args []string, out io.Writer) error {
	switch cmd := strings.Join(args[:min(2, len(args))], " "); {
	case strings.HasPrefix(cmd, "server"):
		return serve(args[1:])
	case cmd == "topic create":
		return createTopic(args[2:], out)
	case cmd == "subscriber create":
		return createSubscriber(args[2:], out)
	case cmd == "subscriber list":
		return listSubscribers(args[2:], out)
	case strings.HasPrefix(cmd, "publish"):
		return publish(args[1:], out)
	}
	return errors.New(usage)
}

// serve menjalankan broker sebagai server HTTP sampai Ctrl+C ditekan.
func serve(args []string) error {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	listen := fs.String("listen", "127.0.0.1:8080", "alamat yang didengarkan")
	dir := fs.String("data", "data", "folder data broker")
	workers := fs.Int("workers", 1, "jumlah worker yang memanggil webhook")
	if err := fs.Parse(args); err != nil {
		return err
	}
	b, err := broker.OpenWith(*dir, broker.Options{Workers: *workers})
	if err != nil {
		return err
	}
	srv := &http.Server{Addr: *listen, Handler: b.Handler()}
	broker.ConfigureServer(srv)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		srv.Close() // memutus semua koneksi, termasuk stream yang terbuka
	}()

	log.Printf("brokerku berjalan di %s, data di %s, %d worker", *listen, *dir, *workers)
	err = srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
	err = errors.Join(err, b.Close())
	log.Print("brokerku berhenti")
	return err
}
```

`serve` adalah isi `main` dari Bagian 5, dengan dua perbedaan. Flag-nya dibaca dari `FlagSet` milik perintah `server`, dan error dikembalikan ke `main`, bukan langsung menghentikan program.

Perintah yang memanggil broker ada di `cmd/brokerku/client.go`. Seluruh isinya adalah:

```go
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/tabwriter"

	"brokerku/broker"
)

// clientFlags membuat FlagSet untuk perintah yang memanggil broker yang
// sedang berjalan, lengkap dengan flag -broker.
func clientFlags(name string) (*flag.FlagSet, *string) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	addr := os.Getenv("BROKERKU")
	if addr == "" {
		addr = "http://127.0.0.1:8080"
	}
	return fs, fs.String("broker", addr, "alamat broker")
}

func createTopic(args []string, out io.Writer) error {
	fs, addr := clientFlags("topic create")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("pemakaian: brokerku topic create <topic>")
	}
	if _, err := call(http.MethodPut, *addr+"/topics/"+fs.Arg(0), nil); err != nil {
		return err
	}
	fmt.Fprintf(out, "topic %s dibuat\n", fs.Arg(0))
	return nil
}

func createSubscriber(args []string, out io.Writer) error {
	fs, addr := clientFlags("subscriber create")
	topic := fs.String("topic", "", "topic yang dibaca subscriber")
	webhook := fs.String("webhook", "", "alamat webhook; kosong untuk subscriber pull")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 || *topic == "" {
		return errors.New("pemakaian: brokerku subscriber create -topic <topic> [-webhook alamat] <nama>")
	}
	body, _ := json.Marshal(map[string]string{"topic": *topic, "webhook": *webhook})
	if _, err := call(http.MethodPut, *addr+"/subscribers/"+fs.Arg(0), body); err != nil {
		return err
	}
	kind := "pull"
	if *webhook != "" {
		kind = "webhook"
	}
	fmt.Fprintf(out, "subscriber %s dibuat, jenis %s, membaca topic %s\n", fs.Arg(0), kind, *topic)
	return nil
}

func listSubscribers(args []string, out io.Writer) error {
	fs, addr := clientFlags("subscriber list")
	if err := fs.Parse(args); err != nil {
		return err
	}
	body, err := call(http.MethodGet, *addr+"/subscribers", nil)
	if err != nil {
		return err
	}
	var list []broker.SubscriberInfo
	if err := json.Unmarshal(body, &list); err != nil {
		return err
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAMA\tTOPIC\tJENIS\tGAGAL\tDICOBA LAGI\tWEBHOOK")
	for _, s := range list {
		kind, hook, retry := "pull", "-", "-"
		if s.Connected {
			kind = "pull, terhubung"
		}
		if s.Webhook != "" {
			kind, hook = "webhook", s.Webhook
		}
		if !s.RetryAt.IsZero() {
			retry = s.RetryAt.Format("15:04:05")
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\t%s\n", s.Name, s.Topic, kind, s.Failures, retry, hook)
	}
	return tw.Flush()
}

func publish(args []string, out io.Writer) error {
	fs, addr := clientFlags("publish")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return errors.New("pemakaian: brokerku publish <topic> <pesan>")
	}
	body, err := call(http.MethodPost, *addr+"/topics/"+fs.Arg(0), []byte(fs.Arg(1)))
	if err != nil {
		return err
	}
	var r struct {
		ID uint64 `json:"id"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return err
	}
	fmt.Fprintf(out, "pesan %d terkirim ke topic %s\n", r.ID, fs.Arg(0))
	return nil
}

// call mengirim satu request ke broker. Jawaban selain 2xx menjadi error
// yang berisi pesan dari broker.
func call(method, url string, body []byte) ([]byte, error) {
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("%s: %s", resp.Status, bytes.TrimSpace(data))
	}
	return data, nil
}
```

Alamat broker dicari dari flag `-broker`, lalu dari variabel lingkungan `BROKERKU`, lalu `http://127.0.0.1:8080`. `call` mengubah setiap jawaban selain 2xx menjadi error yang berisi status dan pesan dari broker, dan `listSubscribers` memakai `tabwriter` untuk merapikan kolomnya.

### Mencoba broker yang berdiri sendiri

Percobaan berikut dijalankan di Windows dengan program hasil `go build`. Broker dijalankan dengan `brokerku server -workers 2`, webhook `stok` dengan `penerima -listen 127.0.0.1:9001 -gagal 2`, yang menjawab 503 untuk dua panggilan pertama setiap pesan, dan subscriber pull `email` dengan `pembaca`. Perintah dan hasilnya adalah:

```text
$ brokerku topic create pesanan
topic pesanan dibuat
$ brokerku subscriber create -topic pesanan -webhook http://127.0.0.1:9001/ stok
subscriber stok dibuat, jenis webhook, membaca topic pesanan
$ brokerku subscriber create -topic pesanan email
subscriber email dibuat, jenis pull, membaca topic pesanan
$ brokerku topic create pesanan
brokerku: 409 Conflict: broker: sudah ada: topic pesanan
$ brokerku publish pesanan "pesanan 1041 dibuat"
pesan 1 terkirim ke topic pesanan
```

Topic kedua dengan nama yang sama ditolak dengan `409 Conflict`, dan CLI menampilkan pesan dari broker apa adanya. Daftar subscriber pukul 23.31.53, sekitar dua detik setelah pesan dikirim, adalah:

```text
$ brokerku subscriber list
NAMA   TOPIC    JENIS            GAGAL  DICOBA LAGI  WEBHOOK
email  pesanan  pull, terhubung  0      -            -
stok   pesanan  webhook          2      23:31:54     http://127.0.0.1:9001/
```

`stok` sudah gagal dua kali untuk pesan 1, dan akan dicoba lagi pukul 23.31.54. Daftar yang sama pukul 23.31.56 adalah:

```text
$ brokerku subscriber list
NAMA   TOPIC    JENIS            GAGAL  DICOBA LAGI  WEBHOOK
email  pesanan  pull, terhubung  0      -            -
stok   pesanan  webhook          0      -            http://127.0.0.1:9001/
```

Hitungan kegagalan sudah kembali ke nol, karena percobaan ketiga berhasil. Catatan `penerima` untuk `stok` adalah:

```text
2026/11/07 23:31:51 pesan 1, panggilan 1: pesanan 1041 dibuat, dijawab 503
2026/11/07 23:31:52 pesan 1, panggilan 2: pesanan 1041 dibuat, dijawab 503
2026/11/07 23:31:54 pesan 1, panggilan 3: pesanan 1041 dibuat, dijawab 200
```

## Menguji ketiga cara

Ada dua test baru. Keduanya adalah:

- **TestPerintah.** Perintah CLI dijalankan terhadap broker sungguhan di port acak, dengan webhook yang selalu menjawab 503. Setiap perintah harus menulis teks yang diharapkan, topic kedua dengan nama yang sama harus ditolak, dan daftar subscriber harus menunjukkan percobaan yang gagal.
- **TestDipasangDiServerAplikasi.** Broker dipasang di bawah `/broker/` di server milik aplikasi. Pesan yang diterbitkan oleh alamat aplikasi harus sampai di subscriber pull lewat stream di `/broker/`.

Seluruh isi `cmd/brokerku/main_test.go` adalah:

```go
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"brokerku/broker"
)

// Perintah client dijalankan terhadap broker sungguhan di port acak. Webhook
// stok selalu menjawab 503, jadi daftar subscriber menunjukkan percobaan
// yang gagal dan kapan pesan itu dicoba lagi.
func TestPerintah(t *testing.T) {
	b, err := broker.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(b.Handler())
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(func() {
		b.Close()
		srv.Close()
		hook.Close()
	})
	t.Setenv("BROKERKU", srv.URL)

	steps := []struct {
		args []string
		want string
	}{
		{[]string{"topic", "create", "pesanan"}, "topic pesanan dibuat\n"},
		{[]string{"subscriber", "create", "-topic", "pesanan", "-webhook", hook.URL, "stok"},
			"subscriber stok dibuat, jenis webhook, membaca topic pesanan\n"},
		{[]string{"subscriber", "create", "-topic", "pesanan", "email"},
			"subscriber email dibuat, jenis pull, membaca topic pesanan\n"},
		{[]string{"publish", "pesanan", "pesanan 1041 dibuat"}, "pesan 1 terkirim ke topic pesanan\n"},
	}
	for _, s := range steps {
		var out strings.Builder
		if err := run(s.args, &out); err != nil {
			t.Fatalf("%v: %v", s.args, err)
		}
		if out.String() != s.want {
			t.Fatalf("%v menulis %q, seharusnya %q", s.args, out.String(), s.want)
		}
	}

	err = run([]string{"topic", "create", "pesanan"}, &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "409 Conflict") {
		t.Fatalf("topic kedua dengan nama sama seharusnya ditolak, error-nya %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		var out strings.Builder
		if err := run([]string{"subscriber", "list"}, &out); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), "webhook  1") || time.Now().After(deadline) {
			t.Logf("brokerku subscriber list\n%s", out.String())
			if !strings.Contains(out.String(), "webhook  1") {
				t.Fatal("percobaan yang gagal tidak muncul di daftar")
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}
```

Test itu memanggil `run`, bukan menjalankan program `brokerku.exe`, jadi ia bisa menangkap tulisan setiap perintah tanpa proses terpisah. Seluruh isi `pull/mount_test.go` adalah:

```go
package pull_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"brokerku/broker"
)

// Broker dipasang di bawah /broker/ di server milik aplikasi, di samping
// alamat aplikasi itu sendiri. Topic dan subscriber dibuat lewat /broker/,
// pesan diterbitkan oleh handler aplikasi dengan memanggil Publish, dan
// subscriber pull menerimanya lewat stream di /broker/.
func TestDipasangDiServerAplikasi(t *testing.T) {
	b, err := broker.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /pesanan", func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		id, err := b.Publish("pesanan", data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "pesanan diterima, pesan %d\n", id)
	})
	mux.Handle("/broker/", http.StripPrefix("/broker", b.Handler()))
	srv := httptest.NewUnstartedServer(mux)
	broker.ConfigureServer(srv.Config)
	srv.Start()
	t.Cleanup(func() {
		b.Close()
		srv.Close()
	})

	api := srv.URL + "/broker"
	create(t, api, "pesanan", "email")
	do(t, http.MethodPost, srv.URL+"/pesanan", "pesanan 1041 dibuat", http.StatusOK)
	m := receive(t, api, "email", 0)
	if m.ID != 1 || string(m.Data) != "pesanan 1041 dibuat" {
		t.Fatalf("email menerima pesan %d %q", m.ID, m.Data)
	}
}
```

`create`, `do`, dan `receive` adalah fungsi bantu dari `pull_test.go` di Bagian 3. Hasil kedua test adalah:

```text
$ go test -v ./cmd/brokerku/
=== RUN   TestPerintah
2026/11/07 23:33:26 broker: stok gagal menerima pesan 1 (percobaan 1): 503 Service Unavailable, dicoba lagi dalam 1s
    main_test.go:65: brokerku subscriber list
        NAMA   TOPIC    JENIS    GAGAL  DICOBA LAGI  WEBHOOK
        email  pesanan  pull     0      -            -
        stok   pesanan  webhook  1      23:33:27     http://127.0.0.1:63210
--- PASS: TestPerintah (0.07s)
PASS
ok  	brokerku/cmd/brokerku	1.631s
$ go test -v -run Dipasang ./pull/
=== RUN   TestDipasangDiServerAplikasi
--- PASS: TestDipasangDiServerAplikasi (0.01s)
PASS
ok  	brokerku/pull	1.758s
```

Semua test di module ini, dari Bagian 1 sampai 6, juga dijalankan 20 kali dengan `-count=20` dan lima kali dengan `-race`, dan semuanya lulus.

## Rangkuman seri Membuat Message Broker Sendiri

Keenam bagian seri ini membangun satu broker, sepotong demi sepotong. Ringkasnya adalah:

1. **Protokol sendiri atau AMQP, lalu menyimpan pesan di log.** Broker kita berbicara lewat beberapa alamat HTTP, bukan AMQP 0-9-1, dan menyimpan pesan setiap topic di log, yaitu daftar pesan yang hanya bisa ditambah di ujungnya. Setiap pesan adalah record, yaitu header tetap ditambah datanya, dengan CRC-32C untuk mengenali tulisan yang terpotong.
2. **Mengingat posisi baca setiap subscriber.** Posisi baca disimpan di slot berukuran tetap di file metadata, dan ditulis dengan `WriteAt`. Log dipaksa ke disk sebelum metadata setiap detik, dan ID dicadangkan per 1.000, supaya tidak ada ID yang dipakai dua kali setelah mati mendadak.
3. **Subscriber yang menarik pesannya sendiri.** Subscriber pull membuka stream pull dan mengirim ack, yaitu laporan bahwa sebuah pesan sudah selesai dikerjakan. Satu loop pengantar berkeliling ke setiap subscriber dan memegang satu pesan per subscriber, dengan timeout ack, yaitu waktu yang diberikan kepada subscriber untuk mengirim ack, selama 30 detik.
4. **Webhook dan percobaan ulang dengan jeda yang makin panjang.** Broker memanggil webhook untuk setiap pesan, mencoba lagi timeout, koneksi yang putus, 5xx, 408, dan 429 dengan jeda dari 1 detik yang berlipat dua, dan melewati pesan setelah 10 kali gagal. Batas yang sama berlaku untuk stream pull.
5. **Worker pool agar webhook yang lambat tidak menahan subscriber lain.** Beberapa worker memanggil webhook bersamaan, setiap subscriber memegang paling banyak satu worker supaya urutannya terjaga, dan putaran dimulai dari subscriber yang menunggu.
6. **Menjalankan broker dan menutup seri.** Broker yang sama bisa tertanam, dipasang di server aplikasi, atau berdiri sendiri dengan CLI.

![Producer mengirim POST /topics ke API HTTP, dari Bagian 1, 3, dan 6. Di disk ada log per topic dengan record dan CRC-32C dari Bagian 1, dan slot metadata berisi posisi baca dan percobaan ulang dari Bagian 2 dan 4. Loop pengantar dari Bagian 3 mengantar pesan lewat stream pull dengan ack dan timeout, dan lewat worker pool dari Bagian 5 yang memanggil webhook dengan jeda dari Bagian 4. Kotak putus-putus mendaftar tiga cara menjalankannya dari Bagian 6.](assets/series-overview.png)

*Seluruh broker muat di satu module Go, hanya dengan library standar.*

Library standar adalah kumpulan package yang ikut terpasang bersama Go. Module `brokerku` tidak memakai package dari luar, jadi semua yang dibahas di seri ini bisa dibaca sampai ke dasarnya.

## Kelemahan seluruh rancangan brokerku

Setiap bagian ditutup dengan kelemahan versi brokernya. Sebagian sudah ditutup oleh bagian berikutnya, tetapi beberapa adalah bagian dari rancangannya sendiri, dan perlu kamu tahu sebelum memakai rancangan seperti ini.

### Hanya satu server, tanpa salinan

Semua pesan dan semua posisi baca ada di satu folder di satu mesin. Dokumentasi NATS menyebut satu server sebagai satu titik kegagalan. Menurut dokumentasi itu, stream dengan satu salinan tidak punya toleransi kegagalan, dan kehilangan server itu berarti kehilangan setiap pesan dan posisi setiap consumer, yaitu program yang membaca pesan.

Broker lain menyalin datanya ke beberapa server. Menurut dokumentasinya, quorum queue RabbitMQ adalah queue yang disalin dengan algoritma konsensus Raft, dan stream JetStream di NATS dengan tiga salinan, R=3, tidak kehilangan apa pun saat satu server hilang. Dokumentasi Kafka menyebut faktor replikasi 3, yaitu tiga salinan data, sebagai pengaturan yang umum di produksi.

Seri ini tidak membangun salinan. Menyalin log ke server lain dengan aman butuh algoritma konsensus seperti Raft, dan itu cukup untuk satu seri tersendiri.

### Pesan dalam detik terakhir bisa hilang

Log dan metadata dipaksa ke disk sekali setiap detik, karena `fsync` untuk setiap pesan, di laptop yang diukur di Bagian 1, sekitar 130 kali lebih lambat daripada `write` saja. Akibatnya, listrik yang padam bisa menghilangkan pesan dalam detik terakhir, walaupun `Publish` sudah kembali tanpa error.

Seri ini juga tidak pernah mematikan listrik sungguhan. Mati mendadak hanya disimulasikan, dengan menyalin folder data yang sedang dipakai dan memotong ujung log.

### Log terus membesar

Pesan tidak dihapus setelah di-ack, dan broker kita belum punya cara untuk membuang pesan lama. Kafka menyelesaikannya dengan retensi. Menurut dokumentasinya, pesan di Kafka tidak dihapus setelah dibaca, dan setiap topic punya pengaturan berapa lama pesan disimpan sebelum dibuang.

### Pesan bisa sampai lebih dari sekali

Ack yang hilang, timeout, dan broker yang mati sebelum posisi baca tersimpan semuanya bisa membuat pesan yang sama dikirim lagi. Jadi setiap subscriber harus idempotent, yaitu aman menerima pesan yang sama lebih dari sekali, misalnya dengan mencatat ID pesan yang sudah dikerjakan.

### Satu subscriber, satu pesan pada satu waktu

Urutan pesan dijaga dengan memberi setiap subscriber paling banyak satu pesan yang sedang dikerjakan. Akibatnya, throughput, yaitu jumlah pesan per detik, untuk satu subscriber dibatasi oleh lama satu pesan, sekitar 7.000 pesan per detik untuk stream pull di laptop yang diukur di Bagian 3, dan jauh lebih sedikit untuk webhook.

Pekerjaan satu subscriber juga tidak bisa dibagi ke beberapa program. Kafka membagi topic menjadi partisi, yaitu potongan topic yang masing-masing punya urutannya sendiri, sehingga beberapa consumer bisa membaca topic yang sama bersamaan.

### Hal-hal lain yang belum dibangun

Ada beberapa kekurangan lain yang lebih kecil. Kekurangan itu adalah:

- **Tanpa autentikasi, dan TLS belum pernah dicoba.** Siapa pun yang bisa menjangkau port broker bisa memakainya, dan hanya HTTP/2 tanpa TLS yang pernah dijalankan.
- **Folder data tidak dikunci.** Tidak ada yang mencegah dua program membuka folder data yang sama, dan keduanya bisa saling menimpa slot dan log.
- **Pesan yang dilewati hanya dicatat.** Tidak ada dead-letter, yaitu tempat khusus untuk pesan yang gagal diantar, dan belum ada cara untuk memundurkan posisi baca.
- **Routing hanya lewat nama topic.** Tidak ada exchange seperti di RabbitMQ atau wildcard seperti di NATS, jadi subscriber hanya bisa membaca satu topic utuh.
- **Client pull hanya untuk Go.** Program dalam bahasa lain harus menulis client stream pull-nya sendiri, atau memakai webhook, yang cukup berupa server HTTP biasa.
- **Belum pernah dijalankan di Linux.** Semua test dan percobaan seri ini berjalan di Windows.

## Kapan RabbitMQ, NATS, atau Kafka lebih tepat?

Broker yang dibangun sendiri tidak menggantikan broker yang sudah matang. Setiap broker di bawah ini menjawab kebutuhan yang tidak dijawab `brokerku`.

### RabbitMQ, untuk routing dan protokol standar

Pilih RabbitMQ kalau pesan harus diarahkan dengan aturan, atau kalau program yang memakainya ditulis dalam banyak bahasa. Exchange di RabbitMQ, seperti yang dibahas di Bagian 1, mengarahkan pesan ke queue menurut aturannya, dan menurut dokumentasinya, RabbitMQ mendukung AMQP 0-9-1, AMQP 1.0, MQTT, dan STOMP.

RabbitMQ juga sudah punya hal-hal yang kita tandai sebagai kelemahan. Quorum queue disalin ke beberapa server, dan menurut dokumentasinya, sejak RabbitMQ 4.0, pesan yang gagal diantar lebih dari 20 kali dibuang atau dipindahkan ke dead-letter exchange, yaitu exchange untuk pesan yang gagal diantar.

### NATS, untuk pesan yang ringan dan alamat yang fleksibel

Pilih NATS kalau banyak layanan perlu saling mengirim pesan dengan cepat, termasuk tanya-jawab. Menurut dokumentasinya, subject di NATS, yaitu nama alamat sebuah pesan, bisa bertingkat dengan titik, seperti `orders.retail.placed`, dan subscriber bisa memakai wildcard untuk menerima pesan dari banyak subject sekaligus.

NATS juga mendukung request-reply, yaitu pesan yang dikirim sambil menunggu jawaban. JetStream, lapisan penyimpanan di NATS, menambahkan pesan yang tersimpan, ack, dan salinan di beberapa server.

### Kafka, untuk throughput besar dan riwayat yang panjang

Pilih Kafka kalau pesannya sangat banyak, atau kalau riwayat pesan perlu dibaca ulang. Partisi membagi satu topic ke beberapa broker, dan menurut dokumentasinya, Kafka menjamin setiap consumer membaca pesan sebuah partisi dalam urutan penulisannya.

Kafka juga dikelilingi alat yang besar. Menurut dokumentasinya, Kafka Connect membawa data masuk dan keluar dari sistem lain, dan library Kafka Streams memproses aliran pesan saat pesan itu datang.

### brokerku, untuk satu server dan program Go

Rancangan seperti `brokerku` masuk akal kalau satu server sudah cukup, dan broker lebih baik ikut di dalam program Go daripada menjadi layanan tersendiri. Seluruh datanya ada di satu folder, dan broker-nya sendiri, yaitu package `eventlog` dan `broker` tanpa test, hanya sekitar 1.400 baris Go.

Rancangan ini juga masuk akal untuk belajar. Setiap keputusan di seri ini, dari `fsync` sekali sedetik sampai giliran worker, adalah keputusan yang juga diambil RabbitMQ, NATS, dan Kafka, hanya dengan jawaban yang berbeda.

![Empat baris. brokerku, berwarna biru, cocok kalau satu server sudah cukup, dan broker ingin dipasang di dalam program Go. RabbitMQ cocok kalau butuh routing, protokol standar, dan queue yang disalin ke beberapa server. NATS cocok kalau butuh subject dengan wildcard, request-reply, dan JetStream dengan R=3. Kafka cocok kalau butuh throughput besar lewat partisi, dan membaca ulang riwayat yang panjang.](assets/choose-broker.png)

*Setiap broker menjawab kebutuhan yang berbeda.*

## Akhir seri Membuat Message Broker Sendiri

Seri ini dimulai dari satu pertanyaan, yaitu memakai AMQP atau membuat protokol sendiri. Enam bagian kemudian, kita punya broker yang menyimpan pesan di log, mengingat posisi baca setiap subscriber, mengantar pesan lewat stream pull dan webhook, mencoba lagi kegagalan dengan jeda yang makin panjang, dan membagi worker dengan adil.

Kalau kamu ingin melanjutkannya sendiri, kelemahan di atas adalah daftar pekerjaan yang baik. Autentikasi dan kunci folder data adalah pekerjaan kecil, retensi butuh log yang dipecah menjadi beberapa file, dan salinan di beberapa server adalah pekerjaan yang jauh lebih besar.

Yang paling penting bukan broker-nya, tetapi alasan di balik setiap keputusannya. Saat nanti kamu memilih RabbitMQ, NATS, atau Kafka, kamu sudah tahu pertanyaan apa yang harus diajukan, mulai dari di mana posisi baca disimpan, kapan data dipaksa ke disk, sampai apa yang terjadi pada pesan yang terus gagal.
