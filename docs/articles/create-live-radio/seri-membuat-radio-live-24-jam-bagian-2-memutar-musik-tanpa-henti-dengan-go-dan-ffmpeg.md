# [Seri Membuat Radio Live 24 Jam Bagian 2] Memutar musik tanpa henti dengan Go dan ffmpeg

*Di bagian ini, kita menulis pemutar musik untuk radio live 24 jam. Satu ffmpeg terus bersiaran, sementara program Go membuka lagu demi lagu dan menulis suaranya tepat waktu, termasuk saat ffmpeg harus dijalankan lagi.*

Di Bagian 1, kita sudah melihat anatomi radio live 24 jam. Musik, video latar, dan teks live digabungkan oleh encoder, yaitu ffmpeg, lalu dikirim ke YouTube lewat RTMP. Kita juga sudah mencoba siaran dari satu perintah ffmpeg, dan melihat tiga masalahnya, yaitu hanya satu lagu, tidak ada yang menyalakannya lagi, dan layar yang diam.

Di Bagian 2, kita menjawab dua masalah pertama dengan program Go. Program ini memutar semua lagu di sebuah folder tanpa henti, dan menjalankan ffmpeg lagi saat ia berhenti, tanpa memotong lagu yang sedang diputar. Semuanya bisa dicoba di satu komputer, dengan siaran ke sebuah file.

## Kenapa tidak memakai playlist bawaan ffmpeg?

ffmpeg punya concat, yaitu cara membaca beberapa file berurutan dari sebuah daftar. Cara ini bekerja baik untuk lagu yang sudah pasti, seperti satu mix berdurasi satu jam. Masalahnya, daftar itu dibaca sekali saat ffmpeg mulai, dan semua file di dalamnya harus punya format yang sama.

Ada dua cara lain untuk memutar playlist yang bisa berubah. Keduanya adalah:

- **Satu ffmpeg untuk setiap lagu.** Setiap lagu disiarkan oleh ffmpeg-nya sendiri, dan program kita menjalankan ffmpeg berikutnya saat lagu selesai. Cara ini sederhana, tetapi setiap pergantian lagu memutus koneksi ke YouTube, dan penonton melihat siaran berhenti sebentar.
- **Satu ffmpeg yang terus berjalan.** Encoder hanya dijalankan sekali, dan suaranya datang dari program kita lewat stdin, yaitu input standar sebuah program. Program kita yang membuka setiap lagu, lalu menulis suaranya ke stdin itu.

Seri ini memilih satu ffmpeg yang terus berjalan, karena hanya cara ini yang tidak memutus siaran di antara lagu. Kelemahannya, program kita harus menulis suara dengan kecepatan yang tepat, dan bagian berikut menunjukkan kenapa itu tidak boleh salah.

## Suara adalah jam bagi siaran

Encoder di Bagian 1 membaca video latar dengan `-re`, yaitu secepat waktu aslinya. Tetapi suara dari stdin tidak punya waktu asli. Suara itu datang secepat program kita menulisnya.

Saya mencoba apa yang terjadi kalau suara berhenti datang. Program kecil menulis suara ke encoder selama 5 detik, berhenti 5 detik, lalu menulis lagi. Selama suara berhenti, waktu siaran yang dilaporkan ffmpeg ikut berhenti di 5,6 detik, dan baru berjalan lagi saat suara datang.

Artinya, encoder tidak membuat gambar tanpa suara. Bagi YouTube, siaran yang berhenti mengirim data terlihat seperti siaran yang macet. Jadi ada dua aturan untuk program kita. Keduanya adalah:

- **Suara tidak boleh berhenti.** Di antara dua lagu, atau saat folder musik kosong, program kita menulis hening, yaitu suara mentah yang semua angkanya nol.
- **Suara ditulis mengikuti jam.** Program kita tahu kapan hening dibutuhkan hanya kalau ia tahu berapa banyak suara yang seharusnya sudah ditulis sekarang. Untuk itu, setiap detik program kita menulis tepat satu detik suara, setengah detik lebih awal, supaya encoder tidak pernah menunggu.

![Dua garis waktu sepanjang 15 detik. Di atas, tanpa hening: suara dari program berhenti di detik 5 sampai 10, dan garis waktu siaran ikut datar di antara keduanya, ditandai siaran macet. Di bawah, dengan hening: celah di antara dua lagu diisi hening berwarna biru, dan garis waktu siaran naik terus tanpa putus.](assets/audio-is-the-clock.png)

*Tanpa suara, encoder berhenti membuat siaran. Hening menjaga siaran tetap berjalan.*

### Bukankah cukup menyambung lagu satu per satu ke stdin?

Menyambung lagu satu per satu memang terlihat cukup. Saat lagu pertama habis, lagu kedua langsung ditulis ke stdin yang sama. Tetapi membuka lagu kedua butuh waktu, dan selama itu tidak ada suara yang ditulis.

Tanpa jam, program kita tidak tahu kapan celah itu mulai terlihat oleh encoder. Dengan jam, program kita tahu persis berapa banyak suara yang sudah terlambat, dan mengisinya dengan hening.

## Anatomi pemutar musik

Sebelum menulis kode, kita lihat dulu bagian-bagian pemutar musik. Suara mengalir dari folder musik ke encoder melewati empat bagian. Keempatnya adalah:

- **Playlist** membaca folder musik sebelum setiap lagu, lalu menjalankan satu ffmpeg kecil untuk lagu itu. ffmpeg kecil ini adalah decoder, yaitu program yang membuka lagu menjadi PCM, suara mentah yang dibahas di Bagian 1.
- **Channel Go** menampung potongan PCM sepanjang 50 milidetik, paling banyak 40 potong, yaitu 2 detik suara. Channel adalah saluran milik Go untuk mengirim data dari satu goroutine ke goroutine lain.
- **Pengumpan** mengambil potongan dari channel dan menulisnya ke stdin encoder, mengikuti jam. Saat channel kosong, pengumpan menulis hening.
- **Encoder** adalah ffmpeg dengan perintah dari Bagian 1, kecuali suaranya datang dari stdin. Saat encoder berhenti, program kita menjalankannya lagi, dengan jeda yang makin panjang.

![Diagram aliran suara. Folder musik berisi file .wav. Playlist menjalankan satu ffmpeg decoder untuk setiap lagu, yang menghasilkan PCM. PCM masuk ke channel Go berukuran 2 detik, lalu ke pengumpan yang menulis mengikuti jam dan menambahkan hening, lalu ke stdin ffmpeg encoder. Encoder juga membaca video latar yang diulang, lalu mengirim siaran ke YouTube atau ke file. Panah putus-putus dari encoder kembali ke program menunjukkan encoder dijalankan lagi saat berhenti.](assets/music-player-anatomy.png)

*Suara berjalan dari folder musik ke encoder, dan hanya encoder yang berbicara dengan YouTube.*

Setiap bagian ini dipilih untuk satu alasan. Alasan-alasan itu adalah:

- **Satu decoder untuk setiap lagu** membuat format lagu tidak penting. Lagu 48 kHz mono dan lagu 22 kHz stereo sama-sama dibuka menjadi 44,1 kHz stereo, format yang dianjurkan YouTube.
- **Folder dibaca ulang sebelum setiap lagu,** jadi lagu yang baru disalin ke folder ikut diputar tanpa restart.
- **Channel sepanjang 2 detik** membuat lagu berikutnya sudah mulai dibuka saat lagu sekarang masih terdengar. Celah di antara lagu tertutup oleh 2 detik itu.
- **Encoder yang berhenti tidak menghentikan musik.** Selama encoder mati, channel penuh dan playlist ikut menunggu. Saat encoder menyala lagi, lagu berlanjut tepat dari tempatnya.

## Menyiapkan project Go untuk radio

Kode di bagian ini butuh Go 1.25 atau lebih baru, dan ffmpeg 7 atau lebih baru. Kita membuat satu module Go bernama `radioku`. Module adalah satu project Go beserta daftar library yang dipakainya. Perintah untuk membuatnya adalah:

```sh
mkdir radioku && cd radioku
go mod init radioku
```

Program ini hanya memakai library bawaan Go, jadi tidak ada library yang perlu diunduh. Susunan foldernya adalah:

```text
radioku/
  radio/playlist.go       membaca folder musik dan membuka setiap lagu
  radio/feeder.go         menulis suara ke encoder mengikuti jam
  radio/stream.go         menjalankan encoder, dan menjalankannya lagi saat berhenti
  radio/radio_test.go     test
  cmd/radioku/main.go     program yang dijalankan
```

## Menulis playlist untuk radio

Playlist membaca folder musik, memilih lagu berikutnya, lalu membuka lagu itu dengan ffmpeg. Kodenya ada di `radio/playlist.go`.

```go
// Package radio menyiarkan musik tanpa henti ke sebuah live stream.
package radio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Suara siaran selalu 44,1 kHz, stereo, 16 bit, apa pun format file lagunya.
const (
	SampleRate = 44100
	Channels   = 2
	frameBytes = Channels * 2                 // satu frame berisi satu sampel untuk setiap channel
	chunkBytes = SampleRate * frameBytes / 20 // 50 ms suara
)

// Playlist memutar file .wav di sebuah folder, urut nama, berulang-ulang.
// Setiap lagu didekode oleh ffmpeg-nya sendiri menjadi PCM, lalu dikirim
// ke Out dalam potongan 50 ms.
type Playlist struct {
	Dir    string
	FFmpeg string
	Log    *log.Logger
	Out    chan []byte
}

// Run memutar lagu sampai ctx selesai. Folder dibaca ulang sebelum setiap
// lagu, jadi lagu yang baru ditambahkan ikut diputar tanpa restart.
func (p *Playlist) Run(ctx context.Context) {
	last := ""
	quiet := false // sudah dicatat di log bahwa tidak ada lagu
	for ctx.Err() == nil {
		files, err := wavFiles(p.Dir)
		if err != nil || len(files) == 0 {
			if !quiet {
				p.Log.Printf("tidak ada lagu di %s, siaran berisi hening", p.Dir)
				quiet = true
			}
			sleep(ctx, 5*time.Second)
			continue
		}
		quiet = false
		last = nextTrack(files, last)
		if err := p.play(ctx, last); err != nil && ctx.Err() == nil {
			p.Log.Printf("gagal memutar %s: %v", last, err)
			sleep(ctx, 2*time.Second)
		}
	}
}

// play mendekode satu lagu ke Out.
func (p *Playlist) play(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, p.FFmpeg, "-loglevel", "error", "-nostdin",
		"-i", filepath.Join(p.Dir, name), "-vn",
		"-f", "s16le", "-ar", fmt.Sprint(SampleRate), "-ac", fmt.Sprint(Channels), "pipe:1")
	// Kalau ffmpeg dijalankan lewat program perantara, mematikan perantara
	// itu tidak ikut mematikan ffmpeg yang asli. WaitDelay memastikan Wait
	// tetap selesai.
	cmd.WaitDelay = 5 * time.Second
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	p.Log.Printf("memutar %s", name)
	for {
		buf := make([]byte, chunkBytes)
		n, err := io.ReadFull(out, buf)
		if n > 0 {
			select {
			case p.Out <- buf[:n]:
			case <-ctx.Done():
				cmd.Wait()
				return ctx.Err()
			}
		}
		if err != nil {
			break // lagu selesai
		}
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("%w: %s", err, bytes.TrimSpace(stderr.Bytes()))
	}
	return nil
}

// wavFiles adalah daftar file .wav di dir, urut nama. File yang namanya
// diawali titik dilewati, jadi lagu bisa disalin dengan nama tersembunyi
// lalu diganti namanya setelah selesai.
func wavFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir) // sudah urut nama
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		name := e.Name()
		if e.Type().IsRegular() && !strings.HasPrefix(name, ".") &&
			strings.EqualFold(filepath.Ext(name), ".wav") {
			files = append(files, name)
		}
	}
	return files, nil
}

// nextTrack adalah lagu sesudah last menurut urutan nama, atau lagu pertama
// setelah lagu terakhir. Kalau last sudah dihapus, lagu sesudah tempatnya
// tetap ditemukan.
func nextTrack(files []string, last string) string {
	i := sort.SearchStrings(files, last) // lagu pertama yang namanya >= last
	if i < len(files) && files[i] == last {
		i++
	}
	if i == len(files) {
		i = 0
	}
	return files[i]
}

func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
```

Kode ini bekerja dalam lima langkah. Langkah-langkahnya adalah:

1. `Run` membaca daftar file `.wav` di folder musik dengan `wavFiles`, sebelum setiap lagu.
2. Kalau folder kosong, `Run` menunggu 5 detik lalu membaca lagi. Selama itu, pengumpan menulis hening, jadi siaran tetap berjalan.
3. `nextTrack` memilih lagu sesudah lagu terakhir menurut urutan nama. Setelah lagu terakhir, playlist kembali ke lagu pertama.
4. `play` menjalankan ffmpeg untuk membuka lagu itu menjadi PCM 44,1 kHz stereo 16 bit, lalu membaca hasilnya dari stdout, yaitu output standar ffmpeg.
5. Hasil itu dipotong menjadi potongan 50 milidetik dan dikirim ke channel `Out`. Saat channel penuh, `play` menunggu, dan ffmpeg ikut menunggu di belakangnya.

Ada tiga hal kecil di kode itu yang punya alasan sendiri. Ketiganya adalah:

- **File yang namanya diawali titik dilewati.** Menyalin lagu besar ke folder butuh waktu, dan lagu yang baru setengah tersalin tidak boleh diputar. Salin dengan nama seperti `.04 lagu baru.wav`, lalu ganti namanya setelah selesai.
- **`nextTrack` mencari dengan `sort.SearchStrings`,** yaitu pencarian di daftar yang sudah urut. Kalau lagu terakhir sudah dihapus dari folder, lagu yang namanya sesudah lagu itu tetap ditemukan.
- **`WaitDelay` membatasi waktu menunggu ffmpeg.** Di Windows, ffmpeg yang dipasang lewat package manager kadang dijalankan oleh program perantara. Mematikan perantara itu tidak ikut mematikan ffmpeg yang asli, dan tanpa `WaitDelay`, `Wait` bisa menunggu selamanya.

## Menulis pengumpan suara

Pengumpan menulis suara ke encoder mengikuti jam, dan menulis hening saat tidak ada lagu yang siap. Kodenya ada di `radio/feeder.go`.

```go
package radio

import (
	"context"
	"io"
	"time"
)

// lead adalah seberapa jauh suara ditulis di depan jam, supaya encoder
// tidak pernah menunggu suara.
const lead = 500 * time.Millisecond

// Feeder menulis suara ke encoder mengikuti jam: setiap detik, tepat satu
// detik suara. Saat tidak ada lagu yang siap, yang ditulis adalah hening.
type Feeder struct {
	In   <-chan []byte
	rest []byte // sisa potongan yang belum ditulis, tetap ada saat encoder diganti
}

// Feed menulis ke w sampai ctx selesai atau penulisan gagal. Penulisan yang
// gagal adalah tanda bahwa encoder sudah berhenti.
func (f *Feeder) Feed(ctx context.Context, w io.Writer) error {
	silence := make([]byte, chunkBytes)
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	start := time.Now()
	var written int64
	for {
		due := bytesFor(time.Since(start) + lead)
		for written < due {
			if len(f.rest) == 0 {
				select {
				case f.rest = <-f.In:
				default: // lagu berikutnya belum siap
				}
			}
			b := silence
			if len(f.rest) > 0 {
				b = f.rest
			}
			b = b[:min(int64(len(b)), due-written)]
			if _, err := w.Write(b); err != nil {
				return err
			}
			if len(f.rest) > 0 {
				f.rest = f.rest[len(b):]
			}
			written += int64(len(b))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}

// bytesFor adalah jumlah byte suara untuk durasi d, dalam frame utuh. d
// dipecah menjadi detik dan sisanya, karena nanodetik dikali 44.100 sudah
// melewati batas int64 setelah sekitar 2,4 hari.
func bytesFor(d time.Duration) int64 {
	sec, rest := int64(d/time.Second), int64(d%time.Second)
	frames := sec*SampleRate + rest*SampleRate/int64(time.Second)
	return frames * frameBytes
}
```

Setiap 20 milidetik, pengumpan menjalankan tiga langkah. Langkah-langkahnya adalah:

1. Menghitung `due`, yaitu berapa byte suara yang seharusnya sudah ditulis, dari waktu sejak pengumpan mulai ditambah `lead`, setengah detik.
2. Selama yang sudah ditulis kurang dari `due`, mengambil potongan lagu dari channel. Kalau channel kosong, yang ditulis adalah hening.
3. Menulis potongan itu ke encoder, paling banyak sampai `due`, lalu menunggu 20 milidetik berikutnya.

Satu frame suara adalah satu sampel untuk setiap channel suara, yaitu 4 byte untuk stereo 16 bit. Semua angka di pengumpan selalu kelipatan 4 byte, jadi hening selalu disisipkan di antara dua frame. Kalau hening pernah disisipkan di tengah sebuah frame, semua suara sesudahnya bergeser, suara kiri dan kanan tertukar, dan musiknya berubah menjadi bising.

Dua hal lain menjaga siaran yang berjalan berbulan-bulan. Keduanya adalah:

- **Sisa potongan disimpan di `rest`.** Saat encoder berhenti, penulisan gagal, dan bagian lagu yang belum tertulis tetap ada di `rest`. Encoder berikutnya melanjutkan lagu tepat dari byte itu.
- **`bytesFor` memecah waktu menjadi detik dan sisanya.** Waktu di Go dihitung dalam nanodetik. Nanodetik dikali 44.100 melewati batas angka `int64` setelah sekitar 2,4 hari, dan siaran kita berjalan jauh lebih lama dari itu.

## Menjalankan encoder dan menjalankannya lagi saat berhenti

Encoder adalah ffmpeg dengan perintah dari Bagian 1, kecuali suaranya dari stdin. `Run` menjalankan playlist, lalu menjalankan encoder berulang-ulang sampai program dihentikan. Kodenya ada di `radio/stream.go`.

```go
package radio

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Config adalah apa yang disiarkan, dan ke mana.
type Config struct {
	FFmpeg string // ffmpeg yang dijalankan
	Video  string // video latar, diulang terus
	Music  string // folder lagu .wav
	Output string // URL RTMP beserta stream key, atau nama file untuk mencoba
	Secret string // bagian Output yang tidak boleh masuk log, yaitu stream key
	Log    *log.Logger
}

// Run bersiaran sampai ctx selesai. Saat ffmpeg berhenti, misalnya karena
// jaringan putus, ffmpeg dijalankan lagi, dan musik berlanjut dari tempatnya.
func Run(ctx context.Context, c Config) error {
	pl := &Playlist{Dir: c.Music, FFmpeg: c.FFmpeg, Log: c.Log, Out: make(chan []byte, 40)}
	var wg sync.WaitGroup
	wg.Go(func() { pl.Run(ctx) })
	defer wg.Wait()
	feeder := &Feeder{In: pl.Out}

	wait := 5 * time.Second
	for {
		started := time.Now()
		err := encode(ctx, c, feeder)
		if ctx.Err() != nil {
			return nil
		}
		// Siaran yang sempat berjalan lama dianggap sehat, jadi jedanya
		// kembali pendek.
		if time.Since(started) > 2*time.Minute {
			wait = 5 * time.Second
		}
		c.Log.Printf("siaran berhenti (%v), mulai lagi dalam %s", err, wait)
		sleep(ctx, wait)
		wait = min(2*wait, time.Minute)
	}
}

// encode menjalankan satu ffmpeg sampai berhenti.
func encode(ctx context.Context, c Config, f *Feeder) error {
	cmd := exec.CommandContext(ctx, c.FFmpeg, encoderArgs(c)...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	// Saat ctx selesai, stdin ditutup. ffmpeg melihat suaranya habis dan,
	// karena -shortest, menutup siaran dengan rapi. Kalau dalam 10 detik
	// belum berhenti juga, ffmpeg dimatikan.
	cmd.Cancel = stdin.Close
	cmd.WaitDelay = 10 * time.Second
	ffLog := &ffmpegLog{log: c.Log, secret: c.Secret}
	cmd.Stderr = ffLog
	if err := cmd.Start(); err != nil {
		return err
	}
	c.Log.Printf("bersiaran ke %s", mask(c.Output, c.Secret))

	feedCtx, stopFeed := context.WithCancel(ctx)
	fed := make(chan struct{})
	go func() {
		defer close(fed)
		f.Feed(feedCtx, stdin)
	}()
	err = cmd.Wait()
	stopFeed()
	<-fed
	if err == nil {
		err = errors.New("ffmpeg selesai")
	}
	if ffLog.last != "" {
		err = fmt.Errorf("%w: %s", err, ffLog.last)
	}
	return err
}

// encoderArgs adalah perintah ffmpeg yang membuat siaran: video latar yang
// diulang, suara dari stdin, lalu H.264 dan AAC di dalam FLV.
func encoderArgs(c Config) []string {
	return []string{
		"-hide_banner", "-loglevel", "warning", "-y",
		// Video latar dibaca secepat waktu aslinya, diulang tanpa akhir.
		"-re", "-stream_loop", "-1", "-i", c.Video,
		// Suara mentah dari stdin. Tanpa dua opsi pertama, ffmpeg menunggu
		// beberapa detik suara untuk dianalisis sebelum mulai.
		"-probesize", "32", "-analyzeduration", "0",
		"-f", "s16le", "-ar", fmt.Sprint(SampleRate), "-ch_layout", "stereo", "-i", "pipe:0",
		"-map", "0:v", "-map", "1:a",
		// Bitrate paling tinggi 3000k, dan keyframe setiap 2 detik sesuai
		// anjuran YouTube.
		"-c:v", "libx264", "-preset", "veryfast", "-r", "30",
		"-b:v", "3000k", "-maxrate", "3000k", "-bufsize", "6000k",
		"-g", "60", "-keyint_min", "60", "-sc_threshold", "0", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "128k",
		// Siaran selesai saat suara habis, yaitu saat stdin ditutup. Buffer
		// untuk -shortest dibuat kecil, karena dengan bawaannya, 10 detik,
		// ffmpeg menahan suara dan berhenti membacanya.
		"-shortest", "-shortest_buf_duration", "0.5",
		"-f", "flv", c.Output,
	}
}

// ffmpegLog meneruskan keluhan ffmpeg ke log, baris demi baris, dengan
// stream key disamarkan.
type ffmpegLog struct {
	log    *log.Logger
	secret string
	part   string // awal baris yang belum lengkap
	last   string // baris terakhir, untuk pesan error
}

func (l *ffmpegLog) Write(b []byte) (int, error) {
	l.part += string(b)
	for {
		line, rest, ok := strings.Cut(l.part, "\n")
		if !ok {
			return len(b), nil
		}
		l.part = rest
		if line = strings.TrimSpace(line); line != "" {
			l.last = mask(line, l.secret)
			l.log.Print("ffmpeg: ", l.last)
		}
	}
}

func mask(s, secret string) string {
	if secret == "" {
		return s
	}
	return strings.ReplaceAll(s, secret, "****")
}
```

`Run` menjalankan encoder dalam sebuah perulangan. Setiap putarannya adalah:

1. `encode` menjalankan ffmpeg dan menyambungkan stdin-nya ke pengumpan, lalu menunggu sampai ffmpeg berhenti.
2. Kalau program sedang dihentikan, `Run` selesai.
3. Kalau tidak, `Run` mencatat alasannya di log, menunggu, lalu menjalankan encoder lagi. Jedanya mulai dari 5 detik, dua kali lebih panjang setiap kali gagal, sampai paling lama 1 menit.
4. Encoder yang sempat berjalan lebih dari 2 menit dianggap sehat, jadi jedanya kembali ke 5 detik.

Jeda yang makin panjang dipakai karena alasan ini. Saat internet putus satu jam, program kita tidak perlu mencoba ratusan kali per menit. Tetapi saat internet hanya putus sebentar, siaran kembali dalam beberapa detik.

### Opsi encoder yang ditemukan lewat percobaan

Tiga opsi di `encoderArgs` tidak ada di perintah Bagian 1. Masing-masing ditemukan saat mencoba suara dari stdin. Ketiganya adalah:

- **`-probesize 32 -analyzeduration 0`.** Biasanya ffmpeg menganalisis beberapa detik pertama dari setiap input sebelum mulai. Suara dari stdin datang mengikuti jam, jadi analisis itu ikut menunggu. Di percobaan saya, siaran baru mulai sekitar 10 detik setelah ffmpeg dijalankan, dan dengan kedua opsi ini, sekitar 1,6 detik.
- **`-ch_layout stereo`.** Suara mentah tidak membawa keterangan jumlah channel. Opsi ini memberi tahu ffmpeg bahwa suaranya stereo, supaya ffmpeg tidak menebak dan menulis peringatan di log.
- **`-shortest -shortest_buf_duration 0.5`.** `-shortest` mengakhiri siaran saat suara habis, yaitu saat stdin ditutup. Tanpa opsi ini, ffmpeg tetap menyiarkan video latar tanpa suara setelah stdin ditutup.

Opsi `-shortest` punya buffer bawaan sepanjang 10 detik. Di percobaan saya, dengan buffer itu, penulisan suara ke ffmpeg tertahan sampai 11 detik, dan siaran menjadi tersendat. Dengan buffer 0,5 detik, siaran berjalan tepat waktu selama 60 detik percobaan, dan ffmpeg berhenti sekitar 1,7 detik setelah stdin ditutup.

Opsi `-y` di awal perintah mengizinkan ffmpeg menimpa file tujuan. Tanpa opsi ini, saat kita mencoba ke sebuah file, encoder yang dijalankan lagi langsung berhenti, karena file `coba.flv` sudah ada.

### Menghentikan encoder dengan menutup stdin

Ada dua cara untuk menghentikan ffmpeg. Cara pertama adalah mematikan prosesnya, dan cara kedua adalah memintanya berhenti. ffmpeg yang dimatikan tidak sempat menutup siaran dengan rapi.

Di Linux, ffmpeg bisa diminta berhenti dengan sinyal interrupt, sama seperti menekan Ctrl+C. Windows tidak punya sinyal itu untuk program lain. Karena itu, `encode` memakai cara yang berlaku di mana saja, yaitu menutup stdin, dan `-shortest` membuat ffmpeg berhenti dengan rapi.

Cara ini juga menjaga satu hal penting. Kalau program kita mati mendadak, OS menutup stdin milik ffmpeg, dan ffmpeg ikut berhenti. Tanpa itu, ffmpeg yang tertinggal akan terus mengirim video tanpa suara ke stream key yang sama.

### Stream key tidak boleh masuk log

Saat bersiaran ke YouTube, alamat tujuan ffmpeg berisi stream key. ffmpeg menyebut alamat itu di pesan error-nya, dan pesan itu masuk ke log kita. `ffmpegLog` mengganti stream key dengan `****` di setiap baris sebelum dicatat.

## Program radioku untuk memutar musik

Program yang dijalankan membaca flag, lalu memanggil `Run`. Flag adalah pilihan yang ditulis di baris perintah, seperti `-musik musik`. Kodenya ada di `cmd/radioku/main.go`.

```go
// Program radioku menyiarkan lagu .wav di sebuah folder tanpa henti, di
// atas video latar yang diulang.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"radioku/radio"
)

// youtubeURL adalah alamat server RTMP YouTube, seperti yang tertera di
// YouTube Studio.
const youtubeURL = "rtmp://a.rtmp.youtube.com/live2/"

func main() {
	video := flag.String("latar", "latar.mp4", "video latar, diulang terus")
	music := flag.String("musik", "musik", "folder lagu .wav")
	out := flag.String("ke", "", "file tujuan untuk mencoba, misalnya coba.flv; tanpa ini, siaran dikirim ke YouTube")
	ffmpeg := flag.String("ffmpeg", "ffmpeg", "ffmpeg yang dijalankan")
	flag.Parse()

	logger := log.New(os.Stderr, "", log.LstdFlags)
	c := radio.Config{FFmpeg: *ffmpeg, Video: *video, Music: *music, Output: *out, Log: logger}
	if c.Output == "" {
		key := os.Getenv("STREAM_KEY")
		if key == "" {
			logger.Fatal("isi variabel lingkungan STREAM_KEY dengan stream key dari YouTube Studio, atau coba dulu dengan -ke coba.flv")
		}
		c.Output, c.Secret = youtubeURL+key, key
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := radio.Run(ctx, c); err != nil {
		logger.Fatal(err)
	}
}
```

Program ini punya empat flag. Flag `-latar` adalah video latar, `-musik` adalah folder lagu, `-ffmpeg` adalah ffmpeg yang dijalankan, dan `-ke` adalah file tujuan untuk mencoba. Tanpa `-ke`, siaran dikirim ke YouTube dengan stream key dari variabel lingkungan `STREAM_KEY`.

`signal.NotifyContext` membuat `ctx` selesai saat kamu menekan Ctrl+C, atau saat OS meminta program berhenti. `Run` lalu menutup stdin encoder, menunggu playlist berhenti, dan program selesai dengan rapi.

## Mencoba pemutar musik

Kita mulai dari test. Test di `radio/radio_test.go` memeriksa pemutar musik dari bagian terkecilnya. Bagian yang memeriksa pengumpan adalah:

```go
// frameWriter mencatat apa yang ditulis Feeder.
type frameWriter struct {
	data    []byte
	failAt  int // penulisan ke berapa yang gagal; 0 berarti tidak pernah
	writes  int
	partial bool // ada penulisan yang memotong frame
}

func (w *frameWriter) Write(b []byte) (int, error) {
	w.writes++
	if w.writes == w.failAt {
		return 0, errors.New("encoder berhenti")
	}
	if len(b)%frameBytes != 0 {
		w.partial = true
	}
	w.data = append(w.data, b...)
	return len(b), nil
}

func TestPengumpanMengikutiJam(t *testing.T) {
	in := make(chan []byte, 1)
	in <- bytes.Repeat([]byte{1}, chunkBytes) // 50 ms lagu, lalu tidak ada lagi
	f := &Feeder{In: in}
	var w frameWriter

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	start := time.Now()
	f.Feed(ctx, &w)
	elapsed := time.Since(start)

	// Setelah satu detik, yang tertulis adalah satu detik suara ditambah lead.
	want := bytesFor(elapsed + lead)
	if got := int64(len(w.data)); got > want || want-got > bytesFor(50*time.Millisecond) {
		t.Errorf("tertulis %d byte setelah %s, seharusnya sekitar %d", got, elapsed, want)
	}
	if w.partial {
		t.Error("ada penulisan yang memotong frame")
	}
	// Lagu ditulis lebih dulu, lalu hening.
	if !bytes.Equal(w.data[:chunkBytes], bytes.Repeat([]byte{1}, chunkBytes)) ||
		bytes.ContainsRune(w.data[chunkBytes:], 1) {
		t.Error("urutan lagu dan hening salah")
	}
}

func TestMusikBerlanjutSetelahEncoderBerhenti(t *testing.T) {
	song := make([]byte, 3*SampleRate*frameBytes) // 3 detik
	for i := range song {
		song[i] = byte(i%251 + 1)
	}
	in := make(chan []byte, 1)
	in <- song
	f := &Feeder{In: in}

	// Encoder pertama berhenti pada penulisan kedua.
	first := frameWriter{failAt: 2}
	if err := f.Feed(context.Background(), &first); err == nil {
		t.Fatal("Feed seharusnya berhenti saat penulisan gagal")
	}
	// Encoder kedua melanjutkan lagu tepat dari tempatnya.
	second := frameWriter{}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	f.Feed(ctx, &second)
	n := len(first.data)
	if !bytes.Equal(second.data[:1000], song[n:n+1000]) {
		t.Error("lagu tidak berlanjut dari tempatnya")
	}
}
```

Kedua test itu memeriksa dua janji pengumpan. Keduanya adalah:

- **`TestPengumpanMengikutiJam`** memberi pengumpan 50 milidetik lagu, lalu membiarkannya berjalan satu detik. Yang tertulis harus satu detik suara ditambah `lead`, tanpa ada penulisan yang memotong frame, dengan lagu lebih dulu lalu hening.
- **`TestMusikBerlanjutSetelahEncoderBerhenti`** membuat encoder pertama gagal di penulisan kedua. Encoder kedua harus menerima lagu tepat dari byte tempat encoder pertama berhenti.

Sisa file test berisi lima test lain. `TestLaguBerikutnya` dan `TestDaftarLagu` memeriksa urutan lagu dan file yang dilewati, `TestBytesForTigaHari` memeriksa hitungan byte untuk siaran 3 hari, dan `TestStreamKeyDisamarkan` memeriksa bahwa stream key tidak muncul di log maupun di pesan error. `TestSiaranKeFile` menjalankan semuanya dengan ffmpeg sungguhan selama 4 detik, ke sebuah file. Hasil `go test -v` di komputer saya adalah:

```text
=== RUN   TestLaguBerikutnya
--- PASS: TestLaguBerikutnya (0.00s)
=== RUN   TestDaftarLagu
--- PASS: TestDaftarLagu (0.00s)
=== RUN   TestBytesForTigaHari
--- PASS: TestBytesForTigaHari (0.00s)
=== RUN   TestPengumpanMengikutiJam
--- PASS: TestPengumpanMengikutiJam (1.00s)
=== RUN   TestMusikBerlanjutSetelahEncoderBerhenti
--- PASS: TestMusikBerlanjutSetelahEncoderBerhenti (0.12s)
=== RUN   TestStreamKeyDisamarkan
--- PASS: TestStreamKeyDisamarkan (0.18s)
=== RUN   TestSiaranKeFile
--- PASS: TestSiaranKeFile (9.33s)
PASS
ok  	radioku/radio	10.998s
```

### Menjalankan radio dengan dua lagu percobaan

Sekarang kita jalankan programnya. Pakai video latar dari Bagian 1, lalu buat dua lagu percobaan berupa nada 20 detik. Perintahnya adalah:

```sh
mkdir musik
ffmpeg -f lavfi -i "sine=frequency=440:duration=20" "musik/01 nada rendah.wav"
ffmpeg -f lavfi -i "sine=frequency=660:duration=20" "musik/02 nada tinggi.wav"
go run ./cmd/radioku -latar latar.mp4 -musik musik -ke coba.flv
```

Setelah 45 detik, log-nya adalah:

```text
2026/10/13 00:52:21 memutar 01 nada rendah.wav
2026/10/13 00:52:21 bersiaran ke coba.flv
2026/10/13 00:52:39 memutar 02 nada tinggi.wav
2026/10/13 00:52:59 memutar 01 nada rendah.wav
```

Lagu kedua tercatat 18 detik setelah lagu pertama, bukan 20 detik. Log mencatat saat sebuah lagu mulai dibuka, dan lagu dibuka sekitar 2 detik sebelum terdengar, karena channel menampung 2 detik suara di depannya. Setelah lagu kedua, playlist kembali ke lagu pertama.

Untuk bersiaran ke YouTube, isi variabel lingkungan `STREAM_KEY`, lalu jalankan program tanpa `-ke`. Di Linux, perintahnya adalah:

```sh
export STREAM_KEY=xxxx-xxxx-xxxx-xxxx-xxxx
go run ./cmd/radioku -latar latar.mp4 -musik musik
```

Di PowerShell di Windows, ganti baris pertama dengan `$env:STREAM_KEY = "xxxx-xxxx-xxxx-xxxx-xxxx"`. Saat siaran berjalan, salin lagu baru ke folder `musik`, dan lagu itu akan diputar setelah lagu terakhir di urutan nama.

## Kelemahan pemutar musik versi kedua

Pemutar musik ini sudah memutar playlist tanpa henti, dan menyalakan encoder lagi saat berhenti. Tetapi masih ada empat kelemahan yang perlu kamu tahu. Keempatnya adalah:

- **Layarnya masih diam.** Penonton tidak tahu lagu apa yang sedang diputar.
- **Encoder yang dijalankan lagi tetap memutus siaran.** Musiknya berlanjut, tetapi YouTube melihat koneksi yang putus. Penonton menunggu setidaknya 5 detik, ditambah waktu YouTube untuk menerima siaran lagi.
- **Hanya file `.wav`.** File `.wav` besar, sekitar 10 MB untuk setiap menit stereo 44,1 kHz. ffmpeg juga bisa membuka MP3 atau FLAC, jadi cukup ubah `wavFiles` kalau kamu ingin file yang lebih kecil.
- **Lagu berganti tanpa transisi.** Lagu berikutnya langsung dimulai setelah lagu sebelumnya habis, tanpa crossfade, yaitu suara lagu lama yang memudar sambil lagu baru masuk.

## Berikutnya dalam seri ini

Di Bagian 3, kita menghidupkan layar. Kita menulis teks live di atas video latar dengan filter drawtext dari ffmpeg, yaitu jam, judul lagu yang sedang diputar, dan sebuah terminal yang mengetik `play <lagu>` setiap kali lagu berganti. Video latarnya kita buat dari sebuah halaman HTML.
