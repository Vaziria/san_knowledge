# [Seri Membuat Radio Live 24 Jam Bagian 5] Menjalankan radio di Raspberry Pi dan menutup seri

*Di bagian terakhir ini, radio kita pindah ke Raspberry Pi. Kita memilih encoder hardware dengan mencobanya lebih dulu, mengukur apakah Pi tertinggal dari waktu siaran, lalu membuat radio menyala sendiri setiap kali Pi dinyalakan.*

Di Bagian 4, radio kita mulai menyapa penonton. Program Go membaca live chat lewat YouTube Data API, membagi kuota hariannya menjadi jatah dasar dan jatah tambahan, lalu mengetik `greet @nama "komentar"` di terminal. Komentar yang dihapus moderator ikut hilang dari layar.

Tetapi semuanya masih berjalan di PC. Di Bagian 5, kita memindahkan radio ke Raspberry Pi, komputer kecil yang kita pilih di Bagian 1, supaya radio berjalan 24 jam tanpa diawasi. Bagian ini juga menutup seri, dengan rangkuman kelemahan desainnya dan kapan sebaiknya memilih alat lain.

## Apa yang harus dikerjakan Raspberry Pi setiap detik?

Di PC, radio kita hampir tidak terasa. Di Raspberry Pi, setiap pekerjaan bersaing untuk empat core CPU yang lemah. Setiap detik siaran, Pi harus mengerjakan lima hal. Kelimanya adalah:

- Membuka 30 frame video latar.
- Menggambar enam lapisan teks live di atas setiap frame, jadi 180 kali menggambar teks per detik.
- Memadatkan 30 frame itu dengan H.264.
- Membuka lagu dan memadatkan suaranya dengan AAC.
- Menjalankan program Go, yaitu pengumpan suara, terminal, dan pembaca chat.

Dari kelima pekerjaan itu, memadatkan H.264 adalah yang paling berat. Raspberry Pi 3 dan 4 punya encoder H.264 hardware, jadi pekerjaan ini bisa dipindahkan dari CPU ke chip khusus. Sisanya tetap dikerjakan CPU, dan menggambar teks adalah yang terberat di antaranya.

## Encoder hardware di Raspberry Pi

Di Linux, encoder hardware Raspberry Pi bisa dipakai ffmpeg dengan nama `h264_v4l2m2m`. V4L2 adalah cara Linux berbicara dengan perangkat video, dan m2m berarti memory to memory, yaitu gambar masuk dari memori dan hasilnya kembali ke memori. Encoder ini menerima bitrate dan jarak keyframe, tetapi tidak punya preset dan batas bitrate seperti libx264.

Tapi encoder hardware tidak selalu ada. Raspberry Pi 5 tidak punya encoder H.264 hardware, dan PC juga tidak punya `h264_v4l2m2m`. Ada dua cara untuk menentukan encoder mana yang dipakai. Keduanya adalah:

- **Membaca model Raspberry Pi.** Program memeriksa model komputernya, lalu memilih encoder dari daftar. Cara ini cepat, tetapi daftar itu harus diperbarui setiap kali ada model baru, dan model yang benar belum tentu punya driver yang benar.
- **Mencoba encoder-nya.** Program meminta ffmpeg memadatkan satu detik video percobaan dengan setiap encoder, dan memakai yang pertama berhasil. Cara ini butuh beberapa detik saat program mulai, tetapi hasilnya pasti benar untuk komputer itu.

Seri ini memilih mencoba encoder-nya, karena jawaban dari komputer itu sendiri lebih bisa dipercaya daripada daftar buatan kita. Kodenya ada di `radio/encoder.go`.

```go
package radio

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// encoders adalah encoder H.264 yang dicoba, berurutan. Di Linux pada ARM,
// yang pertama adalah encoder hardware Raspberry Pi 3 dan 4.
func encoders() []string {
	if runtime.GOOS == "linux" && strings.HasPrefix(runtime.GOARCH, "arm") {
		return []string{"h264_v4l2m2m", "libx264"}
	}
	return []string{"libx264"}
}

// pickEncoder adalah encoder pertama yang berhasil membuat satu detik
// video percobaan di komputer ini.
func pickEncoder(ctx context.Context, c Config, candidates []string) (string, error) {
	for _, enc := range candidates {
		try, cancel := context.WithTimeout(ctx, 30*time.Second)
		args := []string{"-hide_banner", "-loglevel", "error", "-nostdin",
			"-f", "lavfi", "-i", fmt.Sprintf("testsrc2=size=1280x720:rate=%d", c.FPS), "-t", "1"}
		args = append(args, codecArgs(enc, c.FPS)...)
		args = append(args, "-f", "flv", "-y", os.DevNull)
		out, err := exec.CommandContext(try, c.FFmpeg, args...).CombinedOutput()
		cancel()
		if err == nil {
			return enc, nil
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		c.Log.Printf("encoder %s tidak bisa dipakai: %v: %s", enc, err, lastLine(out))
	}
	return "", fmt.Errorf("tidak ada encoder yang bisa dipakai (%s)", strings.Join(candidates, ", "))
}

// codecArgs adalah opsi encoder video: bitrate paling tinggi 3000k, dan
// keyframe setiap 2 detik.
func codecArgs(enc string, fps int) []string {
	gop := fmt.Sprint(2 * fps)
	args := []string{"-c:v", enc, "-b:v", "3000k", "-g", gop}
	if enc == "libx264" {
		// ultrafast di ARM, karena di sana setiap siklus CPU berharga.
		preset := "veryfast"
		if strings.HasPrefix(runtime.GOARCH, "arm") {
			preset = "ultrafast"
		}
		args = append(args, "-preset", preset, "-maxrate", "3000k", "-bufsize", "6000k",
			"-keyint_min", gop, "-sc_threshold", "0")
	}
	return append(args, "-pix_fmt", "yuv420p")
}

// lastLine adalah baris terakhir keluaran ffmpeg, untuk pesan error.
func lastLine(b []byte) string {
	lines := strings.Split(strings.TrimSpace(string(bytes.ReplaceAll(b, []byte("\r"), nil))), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
```

`encoders` hanya mencoba encoder hardware di Linux pada ARM, yaitu jenis CPU Raspberry Pi. `pickEncoder` menjalankan ffmpeg dengan `testsrc2`, sumber video percobaan bawaan ffmpeg, selama satu detik, lalu membuang hasilnya ke `os.DevNull`. Encoder yang gagal dicatat di log beserta alasannya, lalu encoder berikutnya dicoba.

`codecArgs` berisi opsi encoder yang dulu tertulis langsung di `encoderArgs`. Jarak keyframe sekarang dihitung dari fps, karena di Raspberry Pi kita mungkin harus menurunkan fps. Untuk libx264 di ARM, preset-nya `ultrafast`, karena di sana setiap siklus CPU berharga.

## Mengukur apakah Raspberry Pi tertinggal

Siaran harus dibuat secepat waktu berjalan. Kalau Raspberry Pi hanya sanggup membuat 0,9 detik siaran dalam satu detik, YouTube menerima siaran yang makin lama makin tertinggal, dan penonton melihat buffering. Pi tidak memberi tahu siapa pun saat ini terjadi, jadi program kita harus mengukurnya sendiri.

ffmpeg bisa melaporkan kemajuannya ke stdout dengan opsi `-progress pipe:1`. Setiap beberapa detik, ffmpeg menulis baris seperti `out_time_us=10000000`, yaitu berapa mikrodetik siaran yang sudah dibuat. Dari dua laporan, program kita bisa menghitung kecepatan siaran.

### Bukankah ffmpeg sudah melaporkan kecepatannya sendiri?

ffmpeg memang melaporkan `speed`, seperti `speed=1x`. Masalahnya, angka itu adalah rata-rata sejak ffmpeg mulai. Setelah seminggu berjalan tepat waktu, Pi yang tiba-tiba melambat hampir tidak menggeser rata-rata itu.

Karena itu, program kita menghitung kecepatan sendiri, dari selisih dua laporan yang berjarak 30 detik. Kodenya ada di `radio/progress.go`.

```go
package radio

import (
	"context"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

// progress membaca laporan -progress dari ffmpeg, yaitu baris key=value
// setiap beberapa detik, dan menyimpan berapa lama siaran yang sudah dibuat.
type progress struct {
	mu    sync.Mutex
	part  string    // awal baris yang belum lengkap
	outUS int64     // mikrodetik siaran yang sudah dibuat
	at    time.Time // kapan angka itu dilaporkan
}

func (p *progress) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.part += string(b)
	for {
		line, rest, ok := strings.Cut(p.part, "\n")
		if !ok {
			return len(b), nil
		}
		p.part = rest
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "out_time_us="); ok {
			if us, err := strconv.ParseInt(v, 10, 64); err == nil {
				p.outUS, p.at = us, time.Now()
			}
		}
	}
}

func (p *progress) get() (int64, time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.outUS, p.at
}

// speed adalah berapa detik siaran yang dibuat dalam satu detik, di antara
// dua laporan. 1,00 berarti tepat waktu.
func speed(us0 int64, at0 time.Time, us1 int64, at1 time.Time) float64 {
	return float64(us1-us0) / float64(at1.Sub(at0).Microseconds())
}

// watch memeriksa kecepatan siaran setiap 30 detik. Di bawah 0,95x,
// komputer ini tertinggal, dan penonton akan melihat siaran tersendat.
// Selain itu, kecepatan dicatat satu menit setelah mulai, lalu setiap 10
// menit.
func watch(ctx context.Context, p *progress, logger *log.Logger) {
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	var lastUS int64
	var lastAt time.Time
	report := time.Now().Add(time.Minute)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		us, at := p.get()
		if !lastAt.IsZero() && at.After(lastAt) {
			s := speed(lastUS, lastAt, us, at)
			live := (time.Duration(us) * time.Microsecond).Truncate(time.Second)
			switch {
			case s < 0.95:
				logger.Printf("tertinggal: kecepatan %.2fx dalam 30 detik terakhir; turunkan -fps", s)
			case time.Now().After(report):
				logger.Printf("live %s, kecepatan %.2fx", live, s)
				report = time.Now().Add(10 * time.Minute)
			}
		}
		lastUS, lastAt = us, at
	}
}
```

`progress` menyimpan `out_time_us` terakhir beserta waktu laporannya. `watch` memeriksanya setiap 30 detik. Kecepatan di bawah 0,95x dicatat sebagai `tertinggal`, dan selebihnya kecepatan dicatat satu menit setelah mulai, lalu setiap 10 menit, sebagai tanda bahwa radio masih sehat.

### Menyambungkan encoder dan pengukur ke radio

`Config` mendapat field `FPS`, dan `Run` memilih encoder sebelum menjalankan apa pun. Field baru itu adalah:

```go
	FPS    int    // frame per detik, 30 kalau kosong
```

Bagian awal `Run` menjadi:

```go
func Run(ctx context.Context, c Config) error {
	if c.FPS == 0 {
		c.FPS = 30
	}
```

```go
	enc, err := pickEncoder(ctx, c, encoders())
	if err != nil {
		return err
	}
	c.Log.Printf("encoder %s, %d fps", enc, c.FPS)
```

Encoder yang terpilih diteruskan ke `encode` dan `encoderArgs`. Di `encode`, stdout ffmpeg disambungkan ke `progress`, dan `watch` berjalan selama encoder hidup. Bagian yang ditambahkan adalah:

```go
	prog := &progress{}
	cmd.Stdout = prog
```

```go
	go watch(feedCtx, prog, c.Log)
```

`encoderArgs` sekarang memakai `codecArgs`, fps dari `Config`, dan laporan kemajuan. Fungsi itu menjadi:

```go
func encoderArgs(c Config, enc, filter string) []string {
	args := []string{
		"-hide_banner", "-loglevel", "warning", "-y",
		// Laporan kemajuan ke stdout setiap 5 detik, untuk watch.
		"-nostats", "-progress", "pipe:1", "-stats_period", "5",
		// Video latar dibaca secepat waktu aslinya, diulang tanpa akhir.
		"-re", "-stream_loop", "-1", "-i", c.Video,
		// Suara mentah dari stdin. Tanpa dua opsi pertama, ffmpeg menunggu
		// beberapa detik suara untuk dianalisis sebelum mulai.
		"-probesize", "32", "-analyzeduration", "0",
		"-f", "s16le", "-ar", fmt.Sprint(SampleRate), "-ch_layout", "stereo", "-i", "pipe:0",
		"-map", "0:v", "-map", "1:a", "-vf", filter, "-r", fmt.Sprint(c.FPS),
	}
	args = append(args, codecArgs(enc, c.FPS)...)
	return append(args,
		"-c:a", "aac", "-b:a", "128k",
		// Siaran selesai saat suara habis, yaitu saat stdin ditutup. Buffer
		// untuk -shortest dibuat kecil, karena dengan bawaannya, 10 detik,
		// ffmpeg menahan suara dan berhenti membacanya.
		"-shortest", "-shortest_buf_duration", "0.5",
		"-f", "flv", c.Output)
}
```

Opsi `-stats_period 5` membuat ffmpeg melapor setiap 5 detik. `-nostats` mematikan laporan biasa ffmpeg di stderr, yang tidak kita baca, supaya log tidak dipenuhi baris kemajuan.

### Hasil pengukuran di Raspberry Pi 3B+

Sebagai gambaran, saya mengukur siaran yang mirip di Raspberry Pi 3B+, dengan encoder hardware, gambar 720p, dan sepuluh lapisan teks live. Di 30 fps, siaran itu tertinggal, dengan kecepatan 0,92x. Di 24 fps, kecepatannya 1,00x, ffmpeg memakai sekitar 1,6 dari 4 core, dan suhu chip sekitar 52 °C.

Radio di seri ini hanya punya enam lapisan teks, jadi bebannya lebih ringan. Tetapi angka tadi menunjukkan cara menanganinya.

Kalau log berkata `tertinggal`, turunkan fps dengan flag `-fps`. Penonton radio hampir tidak bisa membedakan 24 fps dan 30 fps, karena gambarnya hampir diam.

## Listrik yang cukup untuk Raspberry Pi

Pengukuran tadi punya satu catatan penting. Pi 3B+ saya dipasang dengan power supply yang lemah, dan karena itu CPU-nya tertahan di 600 MHz. Raspberry Pi menurunkan kecepatan CPU-nya sendiri saat tegangan listriknya turun, dan kejadian ini disebut throttling.

Power supply yang dianjurkan untuk Raspberry Pi 3B+ adalah 5,1 V 2,5 A. Kalau tegangannya turun di bawah sekitar 4,63 V, Raspberry Pi mencatatnya di log kernel. Cara paling cepat untuk memeriksanya adalah perintah `vcgencmd get_throttled`. Di Pi saya, hasilnya adalah:

```text
throttled=0x50005
```

Angka itu adalah gabungan beberapa tanda, dan setiap tanda punya artinya sendiri. Tanda-tanda itu adalah:

- `0x1` berarti tegangan sedang terlalu rendah.
- `0x4` berarti CPU sedang diperlambat.
- `0x10000` berarti tegangan pernah terlalu rendah sejak Pi menyala.
- `0x40000` berarti CPU pernah diperlambat sejak Pi menyala.

Jumlah keempatnya adalah `0x50005`. Pi yang sehat menjawab `throttled=0x0`. Kalau Pi-mu tidak menjawab `0x0`, ganti power supply dan kabelnya sebelum menyalahkan program.

## File teks di RAM, bukan di kartu SD

Raspberry Pi menyimpan semuanya di kartu SD. Kartu SD hanya tahan ditulis dalam jumlah tertentu, dan radio kita menulis file teks setiap kali terminal mengetik satu huruf. Kursor yang berkedip saja sudah menulis satu file dua kali per detik, yaitu sekitar 170.000 kali sehari.

Linux punya folder `/dev/shm`, yang isinya ada di RAM, bukan di disk. Program kita sekarang memakai folder itu sebagai tempat file teks, kalau folder itu ada. Bagian yang berubah di `cmd/radioku/main.go` adalah:

```go
	textDir := os.TempDir()
	if fi, err := os.Stat("/dev/shm"); err == nil && fi.IsDir() {
		// /dev/shm ada di RAM. File teks ditulis ulang setiap kali satu
		// huruf diketik, dan kartu SD lebih awet tanpa tulisan sesering itu.
		textDir = "/dev/shm"
	}
	text := flag.String("teks", textDir, "folder untuk file teks yang dibaca drawtext")
	fps := flag.Int("fps", 30, "frame per detik; turunkan kalau log berkata tertinggal")
```

```go
		Font: *font, Text: *text, FPS: *fps, Log: logger}
```

Isi `/dev/shm` hilang saat Pi dimatikan, tetapi itu tidak masalah. `Run` menulis semua file teks lagi setiap kali program mulai.

## Membangun program untuk Raspberry Pi

Go bisa membangun program untuk Raspberry Pi dari PC, tanpa memasang apa pun di Pi. Pertama, periksa jenis Raspberry Pi OS-mu dengan perintah `uname -m` di Pi. Jawaban `aarch64` berarti OS 64-bit, dan `armv7l` berarti OS 32-bit.

Lalu bangun programnya di PC. Untuk OS 64-bit, perintahnya adalah:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o radioku ./cmd/radioku
```

Untuk OS 32-bit, ganti `GOARCH=arm64` dengan `GOARCH=arm GOARM=7`. `CGO_ENABLED=0` membuat Go tidak memakai compiler C, jadi program bisa dibangun dari PC mana pun. Di PowerShell, isi ketiga variabel itu dengan `$env:CGO_ENABLED=0; $env:GOOS="linux"; $env:GOARCH="arm64"`, lalu jalankan `go build` yang sama.

Di Raspberry Pi, pasang ffmpeg dan font DejaVu Sans Mono. Di Raspberry Pi OS yang berbasis Debian 13, perintahnya adalah:

```sh
sudo apt install ffmpeg fonts-dejavu-mono
```

Raspberry Pi OS memasang ffmpeg buatan Raspberry Pi sendiri, yang sudah berisi `h264_v4l2m2m`. Di Pi saya, versinya adalah 7.1.5.

Lalu salin program, video latar, folder musik, dan file kata terlarang ke Pi. Perintahnya adalah:

```sh
ssh pi@raspberrypi.local mkdir -p radioku
scp radioku latar/latar.mp4 blokir.txt jalan.sh pi@raspberrypi.local:radioku/
scp -r musik pi@raspberrypi.local:radioku/
```

`jalan.sh` adalah script yang dibahas di bagian berikutnya.

## Menyalakan radio saat Raspberry Pi menyala

Radio harus menyala sendiri setelah listrik padam, dan menyala lagi setiap kali program berhenti. Ada dua cara yang umum di Linux. Keduanya adalah:

- **Layanan systemd.** systemd adalah pengelola layanan di kebanyakan Linux, termasuk Raspberry Pi OS. systemd bisa menjalankan program saat boot, menjalankannya lagi saat berhenti, dan menyimpan log-nya. Kelemahannya, file unit-nya harus dipasang dengan `sudo`. File unit adalah file yang memberi tahu systemd cara menjalankan sebuah program.
- **Cron dengan `@reboot`.** Cron adalah penjadwal tugas di Linux, dan setiap pengguna punya jadwalnya sendiri, tanpa `sudo`. Baris `@reboot` menjalankan sebuah perintah sekali, saat komputer menyala. Kelemahannya, cron tidak menjalankan program lagi saat berhenti, jadi script kita yang harus melakukannya.

Seri ini memakai cron dengan sebuah script, karena seluruh radio bisa dipasang dan diperbarui tanpa `sudo`, selain untuk `apt install` tadi. Kalau kamu sudah terbiasa dengan systemd, pilihan itu sama baiknya. Isi `jalan.sh` adalah:

```sh
#!/bin/sh
# Menjalankan radioku, dan menjalankannya lagi setiap kali berhenti. Cron
# menjalankan script ini sekali, saat Raspberry Pi menyala.
cd "$(dirname "$0")" || exit 1

# Hanya satu salinan yang boleh berjalan, karena satu stream key hanya
# membawa satu siaran.
exec 9>.jalan.lock
flock -n 9 || exit 0

# STREAM_KEY dan YOUTUBE_API_KEY ada di rahasia.env, yang hanya bisa dibaca
# pemiliknya.
set -a
. ./rahasia.env
set +a

while true; do
	# Log yang lebih besar dari 5 MB disimpan sebagai radioku.log.1.
	if [ "$(stat -c %s radioku.log 2>/dev/null || echo 0)" -gt 5000000 ]; then
		mv radioku.log radioku.log.1
	fi
	./radioku -latar latar.mp4 -musik musik -channel UCxxxxxxxxxxxxxxxxxxxxxx \
		-blokir blokir.txt -fps 24 >>radioku.log 2>&1
	sleep 10
done
```

Script ini mengerjakan empat hal. Keempatnya adalah:

1. **Mengunci.** `flock -n 9` mengunci file `.jalan.lock`. Kalau script lain sudah memegang kunci itu, script ini langsung selesai. Satu stream key hanya membawa satu siaran, jadi dua radio yang berebut stream key yang sama hanya akan saling memutus.
2. **Membaca rahasia.** Stream key dan API key dibaca dari `rahasia.env`, dan `set -a` membuat keduanya menjadi variabel lingkungan untuk `radioku`.
3. **Memutar log.** Log yang lebih besar dari 5 MB dipindahkan ke `radioku.log.1`, supaya kartu SD tidak penuh oleh log.
4. **Menjalankan lagi.** Setiap kali `radioku` berhenti, script menunggu 10 detik, lalu menjalankannya lagi.

Kalau `radioku` mati mendadak, ffmpeg tidak tertinggal sendirian. Di Bagian 2, kita membuat ffmpeg berhenti saat stdin-nya ditutup, dan OS menutup stdin itu saat `radioku` mati. Jadi saat `jalan.sh` menjalankan `radioku` lagi, tidak ada ffmpeg lama yang masih memegang stream key.

![Diagram rantai yang menyalakan radio, dari kiri ke kanan. Cron dengan @reboot menjalankan jalan.sh sekali saat Raspberry Pi menyala. jalan.sh memegang kunci flock, membaca rahasia.env, dan menjalankan radioku. radioku menjalankan ffmpeg, dan ffmpeg mengirim siaran ke YouTube. Garis putus-putus menunjukkan jalan.sh menjalankan radioku lagi, dan radioku menjalankan ffmpeg lagi. Catatan di bawah menyebut, kalau radioku mati, OS menutup stdin milik ffmpeg, dan ffmpeg ikut berhenti.](assets/pi-boot-chain.png)

*Setiap mata rantai menyalakan lagi mata rantai sesudahnya saat berhenti.*

Di Pi, buat `rahasia.env`, lalu kunci file itu supaya hanya bisa dibaca olehmu. Lalu daftarkan `jalan.sh` ke cron. Perintahnya adalah:

```sh
cd ~/radioku
printf 'STREAM_KEY=xxxx-xxxx-xxxx-xxxx-xxxx\nYOUTUBE_API_KEY=AIza...\n' > rahasia.env
chmod 600 rahasia.env
chmod +x radioku jalan.sh
(crontab -l 2>/dev/null; echo "@reboot $HOME/radioku/jalan.sh") | crontab -
./jalan.sh &
```

Baris terakhir menjalankan radio sekarang juga, tanpa menunggu Pi dinyalakan ulang. Untuk melihat apa yang sedang terjadi, baca log-nya dengan `tail -f ~/radioku/radioku.log`.

## Mencoba bagian kelima

Test untuk encoder dan pengukur kecepatan ada di `radio/encoder_test.go`. Isinya adalah:

```go
package radio

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"
	"time"
)

func TestEncoderCadangan(t *testing.T) {
	needFFmpeg(t)
	var logs bytes.Buffer
	c := Config{FFmpeg: "ffmpeg", FPS: 30, Log: log.New(&logs, "", 0)}
	enc, err := pickEncoder(context.Background(), c, []string{"tidak_ada", "libx264"})
	if err != nil || enc != "libx264" {
		t.Fatalf("pickEncoder = %q, %v", enc, err)
	}
	if !strings.Contains(logs.String(), "encoder tidak_ada tidak bisa dipakai") {
		t.Errorf("log:\n%s", logs.String())
	}
}

func TestLaporanKemajuan(t *testing.T) {
	var p progress
	// Satu laporan bisa datang terpotong di tengah baris.
	p.Write([]byte("frame=300\nout_time_us=10000"))
	p.Write([]byte("000\nspeed=1x\nprogress=continue\n"))
	us, at := p.get()
	if us != 10_000_000 || at.IsZero() {
		t.Errorf("out_time_us = %d", us)
	}

	start := time.Now()
	if s := speed(0, start, 27_000_000, start.Add(30*time.Second)); s != 0.9 {
		t.Errorf("speed = %.2f, seharusnya 0.90", s)
	}
}
```

`TestEncoderCadangan` meminta `pickEncoder` mencoba encoder yang tidak ada, lalu libx264. Hasilnya harus libx264, dan encoder pertama harus tercatat di log sebagai tidak bisa dipakai. `TestLaporanKemajuan` memeriksa bahwa laporan yang datang terpotong di tengah baris tetap terbaca, dan bahwa 27 detik siaran dalam 30 detik dihitung sebagai 0,9x. Hasil `go test -v` di PC saya adalah:

```text
=== RUN   TestEncoderCadangan
--- PASS: TestEncoderCadangan (0.27s)
=== RUN   TestLaporanKemajuan
--- PASS: TestLaporanKemajuan (0.00s)
PASS
ok  	radioku/radio	1.096s
```

`jalan.sh` saya coba di PC dengan `sh` dan `flock` dari MSYS2, dengan program palsu sebagai pengganti `radioku`. Log yang lebih besar dari 5 MB dipindahkan ke `radioku.log.1`, program palsu dijalankan lagi setiap sekitar 11 detik, dan salinan kedua dari `jalan.sh` langsung selesai karena kuncinya sudah dipegang.

Ada hal yang belum saya coba, dan kamu perlu tahu sebelum memakainya. Program di seri ini belum saya jalankan di Raspberry Pi, karena Pi saya sedang dipakai untuk siaran. Pilihan encoder hardware, `/dev/shm`, dan cron di bagian ini mengikuti siaran yang sudah berjalan di Pi itu, tetapi kodenya sendiri baru diuji di PC.

## Apa yang sudah kita bangun dalam seri ini

Sampai di sini, radio live 24 jam kita sudah lengkap. Setiap bagian seri ini membangun satu bagiannya. Bagian-bagian itu adalah:

- **Bagian 1** membedah radio live 24 jam, anjuran YouTube untuk encoder, dan pilihan Raspberry Pi, ffmpeg, serta Go. Siaran pertama dibuat dari satu perintah ffmpeg.
- **Bagian 2** memutar playlist tanpa henti ke satu ffmpeg, dengan pengumpan yang menulis suara mengikuti jam, dan menjalankan ffmpeg lagi tanpa memotong lagu.
- **Bagian 3** menulis teks live dengan drawtext, dari file teks yang dibaca ulang di setiap frame, dengan satu file untuk setiap warna dan video latar dari HTML.
- **Bagian 4** membaca live chat lewat YouTube Data API, membagi kuota menjadi jatah dasar dan jatah tambahan, dan menyapa penonton tanpa menampilkan yang dihapus moderator.
- **Bagian 5** memilih encoder hardware dengan mencobanya, mengukur kecepatan siaran, dan menyalakan radio dengan cron tanpa `sudo`.

![Diagram radio live 24 jam yang utuh. Di dalam kotak Raspberry Pi, cron dan jalan.sh dari Bagian 5 menjalankan radioku. Di dalam radioku ada playlist dan pengumpan suara dari Bagian 2, terminal dan file teks dari Bagian 3 dan 5, pemilih encoder dari Bagian 5, dan pembaca live chat dari Bagian 4 yang mengirim sapaan ke terminal. Tiga yang pertama menuju ffmpeg. ffmpeg mengirim siaran ke YouTube, dan komentar dari live chat kembali ke pembaca live chat.](assets/radio-series-overview.png)

*Radio live 24 jam yang kita bangun dalam lima bagian. Label B menunjukkan bagian seri yang membangunnya.*

## Kelemahan desain radio di seri ini

Setiap pilihan di seri ini menjawab satu masalah dengan mengorbankan hal lain. Sebelum memakainya, ada enam kelemahan yang perlu kamu tahu. Keenamnya adalah:

- **Satu rumah, satu titik gagal.** Radio berhenti saat listrik padam atau internet rumah putus, dan baru menyala lagi setelah keduanya kembali.
- **Upload berjalan setiap detik.** Dengan gambar yang hampir diam, upload-nya sekitar 50 GB sebulan. Video latar yang banyak bergerak bisa mendekati 1 TB sebulan, jadi periksa batas pemakaian wajar dari ISP-mu.
- **Siaran terputus setiap kali ffmpeg dijalankan lagi.** Musiknya berlanjut, tetapi penonton menunggu setidaknya 5 detik, ditambah waktu YouTube untuk menerima siaran lagi.
- **Sapaan bisa terlambat hampir satu menit.** Kuota YouTube Data API membatasi seberapa sering live chat bisa dibaca.
- **Layar hanya andal di Linux.** Di Windows, drawtext gagal membaca file teks yang sedang diganti.
- **Siaran yang lebih panjang dari 12 jam tidak selalu tersimpan.** Radio ini tidak meninggalkan rekaman yang bisa ditonton ulang.

Ada juga satu kelemahan yang bukan soal teknis. Radio live 24 jam menahan penonton yang sudah datang, tetapi tidak mendatangkan penonton baru. Orang tetap harus menemukannya lebih dulu, misalnya lewat video pendek atau video biasa di channel yang sama.

## Kapan sebaiknya memilih alat lain

Radio di seri ini cocok untuk satu situasi tertentu, yaitu musik milikmu sendiri, layar yang sederhana, dan komputer kecil yang menyala terus. Untuk situasi lain, alat lain lebih tepat. Pilihan-pilihan itu adalah:

- **OBS di PC** cocok jika layarmu butuh banyak lapisan, video kamera, atau scene yang berganti, dan PC-mu memang menyala terus.
- **Layanan siaran 24 jam berbayar** cocok jika kamu tidak ingin mengurus komputer sama sekali, dan fitur mereka sudah cukup.
- **VPS** cocok jika internet atau listrik di rumahmu sering putus. Kelemahannya, sewa bulanan dan CPU yang harus memadatkan video tanpa encoder hardware.
- **Raspberry Pi 5** cocok jika video latarmu banyak bergerak. Pi 5 tidak punya encoder H.264 hardware, tetapi CPU-nya jauh lebih cepat daripada Pi 3 dan Pi 4.

## Akhir seri Membuat Radio Live 24 Jam

Seri ini dimulai dari satu perintah ffmpeg yang mengulang satu lagu. Setiap bagian sesudahnya menjawab satu kelemahan dari bagian sebelumnya, yaitu playlist yang tidak bisa berubah, layar yang diam, penonton yang tidak dijawab, dan PC yang harus menyala terus.

Kalau kamu baru bergabung di bagian ini, mulailah dari Bagian 1. Setiap bagian membangun di atas bagian sebelumnya, dan kodenya bisa diikuti dari awal sampai akhir.
