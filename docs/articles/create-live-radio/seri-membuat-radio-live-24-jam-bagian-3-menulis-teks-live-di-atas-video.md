# [Seri Membuat Radio Live 24 Jam Bagian 3] Menulis teks live di atas video

*Di bagian ini, kita menghidupkan layar radio. ffmpeg menggambar jam, judul lagu, dan sebuah terminal yang mengetik sendiri di atas video latar, dengan membaca file teks di setiap frame.*

Di Bagian 2, kita menulis pemutar musik. Satu ffmpeg terus bersiaran, sementara program Go membuka lagu demi lagu dan menulis suaranya mengikuti jam. Saat ffmpeg berhenti, program kita menjalankannya lagi, dan lagu berlanjut dari tempatnya.

Tetapi layarnya masih diam. Di Bagian 3, kita menulis teks live di atas video latar, yaitu jam, judul lagu yang sedang diputar, dan sebuah terminal yang mengetik `play <lagu>` setiap kali lagu berganti. Video latarnya kita buat dari sebuah halaman HTML.

![Layar radio dalam bentuk jendela terminal gelap. Di title bar ada tiga titik warna, tulisan radioku, dan jam 21:04 di kanan. Di dalam jendela, prompt berwarna merah muda radio@radioku, lalu baris berpanah hijau play 01 hujan di jendela.wav dan play 02 kopi pagi.wav, dan baris input yang sedang mengetik play 03 lampu ko dengan kursor kotak. Di baris status bawah tertulis 03 lampu kota.wav dengan tanda not balok.](assets/terminal-screen.png)

*Layar yang kita bangun di bagian ini. Semua teks yang berwarna adalah teks live.*

## Kenapa teks digambar oleh ffmpeg di setiap frame?

Teks di layar harus berubah saat lagu berganti, saat menit berganti, dan saat terminal mengetik huruf berikutnya. Ada tiga cara untuk memasukkan teks yang berubah ke siaran. Ketiganya adalah:

- **Membuat video latar baru untuk setiap lagu.** Judul lagu digambar ke video latar, lalu encoder dijalankan lagi dengan video itu. Cara ini memutus siaran setiap kali lagu berganti, dan tidak bisa membuat terminal yang mengetik.
- **Memakai aplikasi penyusun layar,** seperti OBS dengan sumber teks atau halaman web. Di Bagian 1, kita sudah melihat kenapa aplikasi desktop tidak cocok untuk Raspberry Pi tanpa layar.
- **Memakai filter drawtext dari ffmpeg.** drawtext menggambar tulisan di atas setiap frame, dan bisa membaca tulisan itu dari sebuah file. Kalau file itu dibaca ulang di setiap frame, program kita cukup mengganti isi file untuk mengubah layar.

Seri ini memilih drawtext, karena teksnya berubah tanpa memutus siaran, dan program kita hanya perlu menulis file. Ada dua hal yang dikorbankan. Keduanya adalah:

- **CPU di setiap frame.** Setiap drawtext menggambar tulisannya 30 kali per detik, berubah atau tidak. Di Bagian 5, kita mengukur seberapa berat beban ini bagi Raspberry Pi.
- **Satu warna untuk setiap drawtext.** Baris terminal yang berwarna-warni butuh beberapa drawtext yang saling mengisi. Caranya dibahas di bawah.

## Anatomi layar siaran

Sebelum menulis kode, kita lihat dulu bagian-bagian layar. Layar siaran disusun dari lapisan-lapisan yang ditumpuk oleh ffmpeg di setiap frame. Lapisan-lapisan itu adalah:

- **Video latar** berisi semua yang tidak pernah berubah. Seperti latar belakang, jendela terminal, title bar, dan baris status. Video ini dibuat sekali dari sebuah halaman HTML, sebelum siaran dimulai.
- **Empat lapisan terminal** menggambar isi terminal, satu lapisan untuk setiap warna. Warnanya adalah merah muda untuk prompt, hijau untuk tanda panah, putih untuk perintah, dan kuning untuk keluaran perintah.
- **Satu lapisan status** menggambar judul lagu yang sedang diputar di baris status.
- **Satu lapisan jam** menggambar jam di kanan title bar.

Setiap lapisan teks adalah satu drawtext yang membaca satu file teks. Program kita menulis file-file itu, dan ffmpeg membacanya ulang di setiap frame. Program kita dan ffmpeg tidak pernah saling memanggil, dan file teks adalah satu-satunya penghubung di antara keduanya.

![Diagram lapisan layar. Di kiri, program Go menulis enam file teks ke sebuah folder, yaitu prompt, arrow, command, output, status, dan clock. Panah reload=1 menuju tumpukan lapisan di tengah, dengan video latar di paling bawah dan enam lapisan drawtext di atasnya, masing-masing dengan contoh warnanya. Panah biru dari tumpukan menuju frame siaran yang sudah jadi.](assets/screen-layers.png)

*Program Go hanya menulis file. ffmpeg membaca file itu dan menumpuk lapisannya di setiap frame.*

## Video latar dibuat dari halaman HTML

Video latar berisi semua yang diam, dan biasanya dibuat di aplikasi gambar. Tetapi radio ini butuh satu hal yang sulit dijamin oleh aplikasi gambar. Ruang kosong untuk teks live harus berada di koordinat yang sama persis dengan angka di program Go.

Halaman HTML memberi koordinat itu, dalam piksel. Kita menulis jendela terminal dengan CSS, dengan sudut membulat, bayangan, dan latar bergradasi, lalu meminta browser headless memotret halaman itu. Browser headless adalah browser yang berjalan tanpa jendela, dan bisa disuruh dari baris perintah.

Tapi kenapa tidak menggambar jendela itu langsung dengan filter ffmpeg? ffmpeg punya filter drawbox untuk menggambar kotak. Masalahnya, setiap filter dijalankan di setiap frame dan memakan CPU, sedangkan gambar yang tidak berubah cukup digambar sekali.

Isi `latar/latar.html` adalah:

```html
<!doctype html>
<html>
<head>
<meta charset="utf-8">
<style>
  /* Kanvas 1280x720, sama dengan ukuran siaran. Tempat teks live
     dibiarkan kosong; angkanya sama dengan konstanta di overlay.go. */
  html, body { margin: 0; width: 1280px; height: 720px; overflow: hidden; }
  body {
    background: radial-gradient(circle at 15% 10%, #2b2552, #0d0e1a 70%);
    font-family: "Segoe UI", sans-serif;
  }
  .window {
    position: absolute; left: 40px; top: 32px; width: 1200px; height: 656px;
    background: #16172a; border: 1px solid #34364f; border-radius: 14px;
    box-shadow: 0 24px 60px rgba(0, 0, 0, 0.55); overflow: hidden;
  }
  .titlebar {
    height: 48px; background: #20213a;
    display: flex; align-items: center; padding: 0 18px;
  }
  .dot { width: 13px; height: 13px; border-radius: 50%; margin-right: 8px; }
  .title {
    position: absolute; left: 0; right: 0; top: 14px;
    text-align: center; color: #8b8fb5; font-size: 15px;
  }
  .status {
    position: absolute; left: 0; right: 0; bottom: 0; height: 52px;
    background: #20213a;
  }
  .label {
    position: absolute; right: 24px; top: 16px;
    color: #8b8fb5; font-size: 15px;
  }
</style>
</head>
<body>
  <div class="window">
    <div class="titlebar">
      <div class="dot" style="background: #ff5f57"></div>
      <div class="dot" style="background: #febc2e"></div>
      <div class="dot" style="background: #28c840"></div>
      <div class="title">radioku</div>
    </div>
    <div class="status"><div class="label">musik 24 jam</div></div>
  </div>
</body>
</html>
```

Jendela itu mulai di titik (40, 32) dan berukuran 1200 × 656 piksel. Title bar-nya setinggi 48 piksel, dan baris statusnya setinggi 52 piksel. Panel terminal di antara keduanya dibiarkan kosong, karena seluruh isinya adalah teks live.

Untuk memotretnya, kita memakai Microsoft Edge dalam mode headless. Chrome dan Chromium menerima flag yang sama. Di PowerShell, dari folder `latar`, perintahnya adalah:

```powershell
& "C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe" --headless=new --hide-scrollbars --force-device-scale-factor=1 `
  --user-data-dir="$env:TEMP\edge-latar" --window-size=1280,720 --screenshot="$PWD\latar.png" "file:///$PWD/latar.html"
```

Flag `--user-data-dir` memberi browser headless folder profil sendiri, supaya tidak bertabrakan dengan Edge yang sedang kamu pakai. Hasilnya adalah `latar.png` berukuran 1280 × 720. Lalu ffmpeg mengubah gambar itu menjadi video 10 detik, dan perintahnya adalah:

```sh
ffmpeg -loop 1 -framerate 30 -i latar.png -t 10 -c:v libx264 -tune stillimage -pix_fmt yuv420p -g 60 latar.mp4
```

`-tune stillimage` memberi tahu encoder bahwa gambarnya diam, jadi video 10 detik itu hanya sekitar 54 KB. Video sekecil ini juga ringan untuk dibuka berulang-ulang oleh Raspberry Pi.

## drawtext membaca ulang file teks di setiap frame

Setiap lapisan teks adalah satu filter drawtext dengan beberapa opsi. Opsi-opsi itu adalah:

- `fontfile` adalah file font yang dipakai. Seri ini memakai DejaVu Sans Mono, font monospace yang gratis, yaitu font yang setiap hurufnya sama lebar.
- `textfile` adalah file teks yang digambar.
- `reload=1` membaca ulang file itu setiap 1 frame, yaitu di setiap frame.
- `expansion=none` menggambar tanda `%` apa adanya. Tanpa ini, drawtext menganggap `%` sebagai awal sebuah perintah, seperti `%{localtime}`.
- `text_shaping=0` mematikan penyusunan huruf untuk tulisan seperti Arab, yang bisa menggeser posisi huruf.
- `y_align=font` mengukur tinggi baris dari ukuran font, bukan dari huruf yang kebetulan ada di file. Tanpa ini, file berisi huruf kecil saja digambar sedikit lebih tinggi daripada file berisi huruf besar.
- `fontsize`, `fontcolor`, `x`, dan `y` adalah ukuran huruf, warnanya, dan titik kiri atas tulisan.

### Mengganti isi file tanpa terbaca setengah jadi

Program kita mengganti isi file kapan saja, sedangkan ffmpeg membacanya 30 kali per detik. Kalau ffmpeg membaca tepat saat file sedang ditulis, ffmpeg mendapat isi yang baru setengah jadi. Satu frame menampilkan teks yang terpotong.

Cara yang biasa untuk mencegahnya adalah menulis isi baru ke file sementara, lalu mengganti nama file sementara itu menjadi nama file aslinya. Di Linux, penggantian nama ini terjadi dalam satu langkah, jadi ffmpeg selalu membaca isi lama atau isi baru yang lengkap.

Masalahnya, Windows tidak bekerja seperti itu. Di percobaan saya, ffmpeg di Windows gagal membuka file tepat saat file itu sedang diganti namanya, dengan error `Permission denied`, lalu siaran berhenti dalam beberapa detik. Menulis langsung ke file juga gagal, karena ffmpeg di Windows memetakan file itu ke memori, dan Windows menolak menulis ke file yang sedang dipetakan.

Jadi, layar siaran di seri ini dibuat untuk Linux, tempat ia akan berjalan di Bagian 5. Untuk mencobanya di Windows, pakai WSL, yaitu Linux yang berjalan di dalam Windows. Bagian lain dari program, seperti test dan pemutar musik, tetap berjalan di Windows.

## Satu baris berwarna-warni adalah "beberapa file yang saling mengisi"

Setiap drawtext hanya punya satu warna. Padahal satu baris terminal bisa berisi tiga warna sekaligus, seperti tanda panah hijau dan perintah putih. Karena itu, satu baris dipecah menjadi beberapa file, satu file untuk setiap warna.

Di setiap file, huruf berwarna itu ditulis di tempatnya, dan huruf berwarna lain diganti spasi. Semua file digambar dari titik yang sama. Karena setiap huruf DejaVu Sans Mono sama lebar, spasi di satu file tepat menutupi tempat huruf di file lain.

![Dua baris terminal dipecah menjadi tiga file warna, digambar sebagai deretan kotak huruf. File arrow berisi tanda panah di baris pertama. File command berisi dua spasi lalu play a.wav. File output berisi Hello! di baris kedua. Kotak krem adalah spasi. Ketiga file digambar di titik yang sama, dan hasilnya di layar adalah baris pertama berisi panah dan play a.wav, dan baris kedua berisi Hello!.](assets/colour-files.png)

*Satu baris dengan dua warna adalah dua file yang digambar di tempat yang sama.*

Baris yang tidak punya huruf berwarna itu tetap ditulis sebagai baris kosong. Dengan begitu, setiap file punya jumlah baris yang sama, dan baris ke-5 di file mana pun selalu jatuh di ketinggian yang sama.

### Ukuran panel terminal dihitung dari ukuran huruf

Terminal harus tahu berapa huruf yang muat di satu baris, supaya baris yang panjang bisa dipotong ke baris berikutnya. Di DejaVu Sans Mono, setiap huruf selebar 0,602 kali ukuran font, dan drawtext memajukan setiap baris setinggi 1,164 kali ukuran font.

Dengan font 20 piksel, setiap huruf selebar sekitar 12 piksel, dan setiap baris setinggi sekitar 23 piksel. Panel terminal kita berukuran 1152 × 520 piksel, jadi muat 95 huruf per baris dan 22 baris. Kalau kamu mengganti font, kedua angka itu ikut berubah.

## Menulis file teks untuk drawtext

`TextFile` adalah satu file yang dibaca drawtext. Kodenya ada di `radio/textfile.go`.

```go
package radio

import (
	"os"
	"path/filepath"
	"sync"
)

// TextFile adalah satu file yang dibaca drawtext di setiap frame.
type TextFile struct {
	Path string

	mu   sync.Mutex
	last *string // isi terakhir yang ditulis
}

func NewTextFile(dir, name string) *TextFile {
	return &TextFile{Path: filepath.Join(dir, "radioku-"+name+".txt")}
}

// Write mengganti isi file, hanya kalau isinya berubah. Isi baru ditulis
// ke file sementara, lalu file itu diganti namanya. Di Linux, rename
// mengganti file dalam satu langkah, jadi drawtext selalu membaca isi lama
// atau isi baru yang lengkap.
func (f *TextFile) Write(s string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.last != nil && *f.last == s {
		return nil
	}
	tmp := f.Path + ".tmp"
	if err := os.WriteFile(tmp, []byte(s), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, f.Path); err != nil {
		return err
	}
	f.last = &s
	return nil
}
```

`Write` hanya menulis kalau isinya berubah. Jam, misalnya, dihitung setiap detik, tetapi file-nya hanya berubah sekali semenit. Isi terakhir disimpan sebagai pointer, supaya file yang isinya kosong tetap ditulis sekali di awal.

## Menyusun drawtext di atas video latar

`overlay.go` berisi tempat setiap teks, warna-warnanya, cara memecah baris menjadi file per warna, dan rangkaian drawtext untuk ffmpeg. Kodenya ada di `radio/overlay.go`.

```go
package radio

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Tempat teks live di video latar 1280x720. Angka yang sama dipakai di
// latar.html: kalau salah satu digeser, geser juga yang lain.
const (
	fontSize = 20
	paneX    = 64 // panel terminal
	paneY    = 104
	paneW    = 1152
	paneH    = 520
	statusX  = 64 // baris status di bawah jendela
	statusY  = 650
	clockX   = 1160 // jam di kanan title bar
	clockY   = 44
)

// Ukuran huruf DejaVu Sans Mono. Setiap huruf selebar 0,602 kali ukuran
// font, dan drawtext memajukan setiap baris setinggi 1,164 kali ukuran
// font.
const (
	charWidth  = 0.602
	lineHeight = 1.164
)

// paneSize adalah jumlah kolom dan baris yang muat di panel terminal.
func paneSize() (cols, rows int) {
	w, h := float64(paneW), float64(paneH)
	return int(w / (fontSize * charWidth)), int(h / (fontSize * lineHeight))
}

// Warna teks di panel terminal. Setiap warna punya file dan drawtext
// sendiri.
const (
	colorPrompt  = iota // radio@radioku ~
	colorArrow          // ❯
	colorCommand        // perintah yang diketik
	colorOutput         // keluaran perintah
	numColors
)

var colorValues = [numColors]string{"0xff7ac6", "0x9ece6a", "0xe6e6f0", "0xe0af68"}

// cell adalah satu huruf di layar, dengan warnanya.
type cell struct {
	r     rune
	color int
}

func cells(s string, color int) []cell {
	var cs []cell
	for _, r := range s {
		cs = append(cs, cell{r, color})
	}
	return cs
}

// wrap memotong satu baris setiap cols huruf, seperti terminal.
func wrap(line []cell, cols int) [][]cell {
	var lines [][]cell
	for len(line) > cols {
		lines = append(lines, line[:cols])
		line = line[cols:]
	}
	return append(lines, line)
}

// colorText adalah isi file untuk satu warna: huruf berwarna itu di
// tempatnya, dan huruf berwarna lain diganti spasi. Dengan font monospace,
// semua warna jatuh tepat di kolomnya masing-masing.
func colorText(lines [][]cell, color int) string {
	out := make([]string, len(lines))
	for i, line := range lines {
		var b strings.Builder
		for _, c := range line {
			if c.color == color {
				b.WriteRune(c.r)
			} else {
				b.WriteByte(' ')
			}
		}
		out[i] = strings.TrimRight(b.String(), " ")
	}
	return strings.Join(out, "\n")
}

// overlay adalah satu drawtext: isi sebuah file, satu warna, di satu tempat.
type overlay struct {
	file  *TextFile
	color string
	x, y  int
}

// videoFilter adalah rangkaian drawtext di atas video latar. reload=1
// membaca ulang file di setiap frame. expansion=none membuat tanda % tampil
// apa adanya. text_shaping=0 dan y_align=font menjaga setiap huruf di
// kolom dan barisnya, apa pun isi file-nya.
func videoFilter(font string, overlays []overlay) string {
	var parts []string
	for _, o := range overlays {
		parts = append(parts, fmt.Sprintf(
			"drawtext=fontfile=%s:textfile=%s:reload=1:expansion=none:text_shaping=0:y_align=font:fontsize=%d:fontcolor=%s:x=%d:y=%d",
			filterPath(font), filterPath(o.file.Path), fontSize, o.color, o.x, o.y))
	}
	return strings.Join(parts, ",")
}

// Path di dalam filter ffmpeg di-escape dua kali. Sekali untuk nilai opsi,
// karena ':' memisahkan opsi, dan sekali lagi untuk rangkaian filter,
// karena ',' memisahkan filter.
var (
	escapeOption = strings.NewReplacer(`\`, `\\`, `'`, `\'`, `:`, `\:`)
	escapeGraph  = strings.NewReplacer(`\`, `\\`, `'`, `\'`, `[`, `\[`, `]`, `\]`, `,`, `\,`, `;`, `\;`)
)

// filterPath menyiapkan path untuk filter ffmpeg. Garis miring terbalik
// diganti garis miring biasa, yang juga diterima Windows.
func filterPath(p string) string {
	return escapeGraph.Replace(escapeOption.Replace(filepath.ToSlash(p)))
}
```

Ada empat bagian di file ini. Keempatnya adalah:

- **Konstanta tempat** adalah koordinat teks live di video latar. Angkanya sama dengan ruang kosong di `latar.html`.
- **`cell`, `cells`, dan `wrap`** menyusun isi terminal sebagai deretan huruf yang masing-masing tahu warnanya, lalu memotong baris yang lebih panjang dari 95 huruf.
- **`colorText`** menulis isi satu file warna. Huruf berwarna lain diganti spasi, dan spasi di akhir baris dibuang.
- **`videoFilter`** menyusun semua drawtext menjadi satu rangkaian filter, dipisahkan koma.

Path file di dalam filter ffmpeg perlu di-escape, yaitu diberi garis miring terbalik di depan tanda baca yang punya arti khusus. Escape dilakukan dua kali, karena filter punya dua lapis aturan. Lapis pertama adalah nilai opsi, tempat `:` memisahkan opsi, dan lapis kedua adalah rangkaian filter, tempat `,` memisahkan filter.

Di Windows, path seperti `C:\Users\...` berisi `:`, jadi tanpa escape, ffmpeg membaca `C` sebagai nilai opsi dan sisanya sebagai opsi berikutnya.

## Menulis terminal yang mengetik sendiri

`Screen` adalah panel terminal. Setiap perintah masuk ke antrean, diketik huruf demi huruf di baris input, lalu naik ke riwayat. Kodenya ada di `radio/screen.go`.

```go
package radio

import (
	"context"
	"log"
	"sync"
	"time"
)

const (
	host      = "radio@radioku"
	maxQueue  = 10  // perintah yang boleh menunggu; yang tertua dibuang
	keepLines = 100 // perintah yang disimpan di riwayat
	cursor    = "█"
)

// entry adalah satu perintah di terminal, beserta keluarannya.
type entry struct {
	command string
	output  []string
}

// Screen adalah panel terminal di layar siaran. Setiap perintah diketik
// huruf demi huruf di baris input, lalu naik ke riwayat bersama
// keluarannya. Di antara perintah, kursor berkedip.
type Screen struct {
	cols, rows int
	files      [numColors]*TextFile
	log        *log.Logger
	wake       chan struct{}

	mu       sync.Mutex
	history  []entry
	queue    []entry
	typed    string // bagian perintah yang sudah diketik
	cursorOn bool

	renderMu sync.Mutex // menjaga urutan tulisan ke file
}

func NewScreen(dir string, logger *log.Logger) *Screen {
	s := &Screen{log: logger, wake: make(chan struct{}, 1), cursorOn: true}
	s.cols, s.rows = paneSize()
	names := [numColors]string{"prompt", "arrow", "command", "output"}
	for i, name := range names {
		s.files[i] = NewTextFile(dir, name)
	}
	return s
}

// overlays adalah drawtext untuk panel terminal: satu untuk setiap warna,
// semuanya di titik yang sama.
func (s *Screen) overlays() []overlay {
	var os []overlay
	for i, f := range s.files {
		os = append(os, overlay{f, colorValues[i], paneX, paneY})
	}
	return os
}

// Play mengantrekan "play <lagu>" untuk lagu yang mulai diputar.
func (s *Screen) Play(name string) {
	s.add(entry{command: "play " + name})
}

func (s *Screen) add(e entry) {
	s.mu.Lock()
	if len(s.queue) == maxQueue {
		s.queue = s.queue[1:]
	}
	s.queue = append(s.queue, e)
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// Run mengetik perintah yang mengantre sampai ctx selesai.
func (s *Screen) Run(ctx context.Context) {
	blink := time.NewTicker(500 * time.Millisecond)
	defer blink.Stop()
	for ctx.Err() == nil {
		s.mu.Lock()
		var next *entry
		if len(s.queue) > 0 {
			next = &s.queue[0]
			s.queue = s.queue[1:]
		}
		waiting := len(s.queue)
		s.mu.Unlock()

		if next == nil {
			select {
			case <-ctx.Done():
			case <-s.wake:
			case <-blink.C:
				s.mu.Lock()
				s.cursorOn = !s.cursorOn
				s.mu.Unlock()
				s.render()
			}
			continue
		}
		s.typeOut(ctx, *next, waiting)
	}
}

// typeOut mengetik satu perintah, sekitar 25 huruf per detik, atau dua kali
// lebih cepat kalau ada perintah lain yang menunggu, lalu memindahkannya ke
// riwayat.
func (s *Screen) typeOut(ctx context.Context, e entry, waiting int) {
	step := time.Second / 25
	pause := 2 * time.Second
	if waiting > 0 {
		step, pause = time.Second/50, 700*time.Millisecond
	}
	s.mu.Lock()
	s.cursorOn = true
	s.mu.Unlock()
	typed := ""
	for _, r := range e.command {
		typed += string(r)
		s.mu.Lock()
		s.typed = typed
		s.mu.Unlock()
		s.render()
		sleep(ctx, step)
	}
	sleep(ctx, 300*time.Millisecond) // Enter

	s.mu.Lock()
	s.history = append(s.history, e)
	if len(s.history) > keepLines {
		s.history = s.history[1:]
	}
	s.typed = ""
	s.mu.Unlock()
	s.render()
	sleep(ctx, pause)
}

// lines adalah isi panel: riwayat, lalu prompt dan baris input, dipotong
// dari bawah supaya muat.
func (s *Screen) lines() [][]cell {
	s.mu.Lock()
	defer s.mu.Unlock()
	var lines [][]cell
	prompt := cells(host+" ~", colorPrompt)
	command := func(text string) [][]cell {
		return wrap(append(cells("❯ ", colorArrow), cells(text, colorCommand)...), s.cols)
	}
	for _, e := range s.history {
		lines = append(lines, prompt)
		lines = append(lines, command(e.command)...)
		for _, o := range e.output {
			lines = append(lines, wrap(cells(o, colorOutput), s.cols)...)
		}
	}
	input := s.typed
	if s.cursorOn {
		input += cursor
	}
	lines = append(lines, prompt)
	lines = append(lines, command(input)...)
	return lines[max(0, len(lines)-s.rows):]
}

// render menulis file setiap warna.
func (s *Screen) render() {
	s.renderMu.Lock()
	defer s.renderMu.Unlock()
	lines := s.lines()
	for i, f := range s.files {
		if err := f.Write(colorText(lines, i)); err != nil {
			s.log.Printf("gagal menulis %s: %v", f.Path, err)
		}
	}
}
```

`Run` mengulang tiga langkah sampai program berhenti. Langkah-langkahnya adalah:

1. Mengambil perintah pertama dari antrean.
2. Kalau antrean kosong, menunggu perintah baru, sambil membalik kursor setiap 500 milidetik supaya terlihat berkedip.
3. Kalau ada perintah, mengetiknya dengan `typeOut`, lalu kembali ke langkah 1.

`typeOut` meniru orang yang mengetik. Kecepatannya 25 huruf per detik, atau 50 huruf per detik kalau ada perintah lain yang menunggu. Setelah huruf terakhir, ada jeda 300 milidetik seperti menekan Enter, lalu perintah naik ke riwayat, dan layar dibiarkan 2 detik supaya bisa dibaca.

Antrean dibatasi 10 perintah, dan riwayat dibatasi 100 perintah. Kalau antrean penuh, perintah tertua dibuang, karena mengetik semua perintah lama hanya membuat layar makin tertinggal. Riwayat yang tampil hanya 22 baris terakhir, karena `lines` memotong isi panel dari bawah.

`render` mengubah isi panel menjadi empat file warna, lalu menulisnya. `renderMu` menjaga supaya dua render tidak menulis bersamaan. Di bagian ini, hanya `Run` yang memanggil `render`, tetapi di Bagian 4, pembaca live chat juga memanggilnya dari goroutine lain.

## Menyambungkan layar ke pemutar musik

Playlist dari Bagian 2 sekarang memberi tahu layar setiap kali lagu berganti. `Playlist` mendapat satu field baru, dan `play` memanggilnya. Bagian yang berubah adalah:

```go
	// OnTrack, kalau diisi, dipanggil dengan nama setiap lagu yang mulai
	// diputar.
	OnTrack func(name string)
}
```

```go
	p.Log.Printf("memutar %s", name)
	if p.OnTrack != nil {
		p.OnTrack(name)
	}
```

Di `radio/stream.go`, `Config` mendapat dua field baru, dan `Run` menyiapkan layar sebelum encoder dijalankan. Bagian awal file itu menjadi:

```go
// Config adalah apa yang disiarkan, dan ke mana.
type Config struct {
	FFmpeg string // ffmpeg yang dijalankan
	Video  string // video latar, diulang terus
	Music  string // folder lagu .wav
	Output string // URL RTMP beserta stream key, atau nama file untuk mencoba
	Secret string // bagian Output yang tidak boleh masuk log, yaitu stream key
	Font   string // file font DejaVu Sans Mono untuk teks live
	Text   string // folder untuk file teks yang dibaca drawtext
	Log    *log.Logger
}

// Run bersiaran sampai ctx selesai. Saat ffmpeg berhenti, misalnya karena
// jaringan putus, ffmpeg dijalankan lagi, dan musik berlanjut dari tempatnya.
func Run(ctx context.Context, c Config) error {
	screen := NewScreen(c.Text, c.Log)
	status := NewTextFile(c.Text, "status")
	clock := NewTextFile(c.Text, "clock")
	overlays := append(screen.overlays(),
		overlay{status, colorValues[colorCommand], statusX, statusY},
		overlay{clock, colorValues[colorCommand], clockX, clockY})

	// Semua file teks harus sudah ada sebelum ffmpeg mulai, karena drawtext
	// gagal kalau file yang dibacanya tidak ada.
	screen.render()
	status.Write("")
	clock.Write(time.Now().Format("15:04"))
	for _, o := range overlays {
		if _, err := os.Stat(o.file.Path); err != nil {
			return fmt.Errorf("teks live: %w", err)
		}
	}

	pl := &Playlist{Dir: c.Music, FFmpeg: c.FFmpeg, Log: c.Log, Out: make(chan []byte, 40)}
	pl.OnTrack = func(name string) {
		screen.Play(name)
		if err := status.Write("♪ " + name); err != nil {
			c.Log.Print(err)
		}
	}
	var wg sync.WaitGroup
	wg.Go(func() { pl.Run(ctx) })
	wg.Go(func() { screen.Run(ctx) })
	wg.Go(func() { runClock(ctx, clock, c.Log) })
	defer wg.Wait()
	feeder := &Feeder{In: pl.Out}
	filter := videoFilter(c.Font, overlays)
```

`Run` menulis semua file teks sekali sebelum encoder dijalankan, lalu memeriksa bahwa semuanya ada. drawtext langsung gagal kalau file yang harus dibacanya tidak ada, dan tanpa pemeriksaan ini, encoder akan gagal berulang-ulang dengan pesan yang membingungkan.

Rangkaian filter dibuat sekali, lalu diteruskan ke `encode` dan `encoderArgs`. Di `encoderArgs`, filter itu dipasang dengan `-vf`, dan baris itu menjadi:

```go
		"-map", "0:v", "-map", "1:a", "-vf", filter,
```

Jam diperbarui oleh `runClock`, yang ditambahkan di akhir file yang sama:

```go
// runClock memperbarui jam di title bar. File-nya hanya berubah saat
// menitnya berganti.
func runClock(ctx context.Context, f *TextFile, logger *log.Logger) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tick.C:
			if err := f.Write(now.Format("15:04")); err != nil {
				logger.Print(err)
			}
		}
	}
}
```

Selebihnya, `stream.go` sama dengan Bagian 2, kecuali `encode` dan `encoderArgs` yang sekarang menerima `filter`. Di `cmd/radioku/main.go`, ada dua flag baru, yaitu `-font` untuk file font dan `-teks` untuk folder file teks. Bagian yang berubah adalah:

```go
	font := flag.String("font", "/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf", "file font DejaVu Sans Mono")
	text := flag.String("teks", os.TempDir(), "folder untuk file teks yang dibaca drawtext")
	flag.Parse()

	logger := log.New(os.Stderr, "", log.LstdFlags)
	c := radio.Config{FFmpeg: *ffmpeg, Video: *video, Music: *music, Output: *out,
		Font: *font, Text: *text, Log: logger}
```

## Mencoba layar siaran

Test layar memeriksa dua hal yang paling mudah salah, yaitu pemecahan warna dan escape path. Test pemecahan warna di `radio/screen_test.go` adalah:

```go
func TestWarnaSejajar(t *testing.T) {
	lines := [][]cell{
		append(cells("❯ ", colorArrow), cells("play a.wav", colorCommand)...),
		cells("Hello!", colorOutput),
	}
	want := map[int]string{
		colorArrow:   "❯\n",
		colorCommand: "  play a.wav\n",
		colorOutput:  "\nHello!",
		colorPrompt:  "\n",
	}
	for color, w := range want {
		if got := colorText(lines, color); got != w {
			t.Errorf("warna %d: %q, seharusnya %q", color, got, w)
		}
	}
}
```

Tiga test lain ada di file yang sama. `TestAntreanPenuh` memeriksa bahwa perintah tertua dibuang saat antrean penuh, dan `TestLayarMengetik` menjalankan terminal selama 1,5 detik lalu membaca isi file-file warnanya. `TestFilterPathDenganTandaBaca` menaruh file teks di folder bernama `teks, ['a'];b`, lalu meminta ffmpeg sungguhan menggambar satu frame dari file itu.

Test butuh font DejaVu Sans Mono. Di Raspberry Pi OS dan Debian 13, pasang dengan `sudo apt install fonts-dejavu-mono`, dan file-nya ada di `/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf`. Di Windows, unduh dari situs DejaVu Fonts, lalu isi variabel lingkungan `RADIOKU_FONT` dengan lokasi file `DejaVuSansMono.ttf`.

Di Windows, hasil `go test -v` adalah:

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
--- PASS: TestStreamKeyDisamarkan (0.19s)
=== RUN   TestSiaranKeFile
    radio_test.go:187: di Windows, drawtext gagal membuka file yang sedang di-rename
--- SKIP: TestSiaranKeFile (0.00s)
=== RUN   TestWarnaSejajar
--- PASS: TestWarnaSejajar (0.00s)
=== RUN   TestAntreanPenuh
--- PASS: TestAntreanPenuh (0.00s)
=== RUN   TestLayarMengetik
--- PASS: TestLayarMengetik (1.50s)
=== RUN   TestFilterPathDenganTandaBaca
--- PASS: TestFilterPathDenganTandaBaca (0.10s)
PASS
ok  	radioku/radio	3.299s
```

`TestSiaranKeFile` dilewati di Windows, karena alasan rename tadi. Di Linux, test itu juga memeriksa bahwa file status berisi lagu yang sedang diputar, dan bahwa `play 01.wav` sudah diketik.

Untuk menjalankan radionya di Linux, tambahkan flag font kalau font-mu tidak di tempat bawaannya. Perintahnya adalah:

```sh
go run ./cmd/radioku -latar latar/latar.mp4 -musik musik -ke coba.flv
```

Gambar di awal artikel ini adalah satu frame dari layar itu. Gambar itu dibuat dengan filter yang sama, dari file-file teks yang isinya ditentukan, sehingga hasilnya persis seperti yang dilihat penonton.

## Kelemahan layar versi ketiga

Layar ini sudah hidup, dengan jam, judul lagu, dan terminal yang mengetik sendiri. Tetapi masih ada empat kelemahan yang perlu kamu tahu. Keempatnya adalah:

- **Hanya berjalan andal di Linux.** Di Windows, drawtext gagal membaca file yang sedang diganti, dan siaran berhenti.
- **Judul lagu muncul sekitar 2 detik lebih awal.** Layar diberi tahu saat lagu mulai dibuka, dan lagu dibuka 2 detik sebelum terdengar, karena channel Go menampung 2 detik suara di depannya. Karena terminal butuh hampir satu detik untuk mengetik perintahnya, selisih ini jarang terasa.
- **Huruf lebar merusak kolom.** Huruf Cina dan Jepang selebar dua huruf biasa, jadi kolom sesudahnya bergeser, dan warna di baris itu tidak lagi sejajar. Huruf yang tidak ada di font tampil sebagai kotak.
- **Layarnya belum menjawab penonton.** Terminal hanya mengetik nama lagu, dan komentar penonton di live chat belum dibaca.

## Berikutnya dalam seri ini

Di Bagian 4, kita membaca live chat dan menyapa setiap penonton yang berkomentar. Terminal mengetik `greet @nama "komentar"` dan menjawab `Hello, @nama!`. Kita juga membahas cara membaca live chat yang resmi dan yang tidak resmi, kuota harian YouTube Data API, dan komentar apa saja yang tidak boleh tampil di layar.
