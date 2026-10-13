# [Seri Membuat Radio Live 24 Jam Bagian 4] Menyapa penonton dari live chat

*Di bagian ini, radio kita mulai menjawab penonton. Program Go membaca live chat lewat YouTube Data API, membagi kuota hariannya dengan hati-hati, lalu mengetik sapaan untuk setiap komentar di terminal.*

Di Bagian 3, kita menghidupkan layar radio. ffmpeg menggambar jam, judul lagu, dan sebuah terminal yang mengetik `play <lagu>`, dengan membaca file teks di setiap frame. Program Go cukup menulis file-file itu untuk mengubah layar.

Tetapi layar itu belum menjawab siapa pun. Di Bagian 4, kita membaca live chat, yaitu kolom komentar di samping siaran, lalu menyapa setiap penonton yang berkomentar. Terminal mengetik `greet @nama "komentar"`, dan menjawab `Hello, @nama!`.

![Layar radio yang sama dengan Bagian 3, sekarang berisi sapaan. Di antara perintah play, terminal mengetik greet @budi lagunya enak buat kerja malam, lalu menjawab Hello, @budi! berwarna kuning. Sapaan yang sama muncul untuk @sari dan @andi, dan baris input sedang mengetik greet @rina semangat ker dengan kursor kotak. Baris status menunjukkan 02 kopi pagi.wav.](assets/greeting-screen.png)

*Setiap komentar diketik sebagai perintah greet, dan terminal menjawab dengan nama penulisnya.*

## Kenapa radio perlu menyapa penonton?

Radio live 24 jam biasanya diputar sambil bekerja, dan penontonnya jarang berinteraksi. Sapaan di layar memberi alasan untuk menulis komentar. Penonton melihat namanya diketik di layar, dan tahu bahwa siaran ini benar-benar live.

Sapaan itu hanya terasa kalau datangnya cepat. Di YouTube Studio, atur latency siaran ke Ultra-low, yaitu jeda kurang dari 5 detik untuk kebanyakan penonton. Kelemahannya, YouTube menyebut penonton lebih mungkin mengalami buffering, tetapi untuk radio yang ingin menjawab penontonnya, jeda yang pendek lebih penting.

## Dua cara membaca live chat YouTube

Program kita harus membaca live chat dari luar YouTube. Ada dua cara yang dipakai orang, dan masing-masing punya kelemahan. Keduanya adalah:

- **YouTube Data API.** API adalah daftar fungsi yang disediakan sebuah layanan untuk program lain. YouTube Data API adalah cara resmi, dengan dokumentasi dan format yang dijaga. Kelemahannya, setiap panggilan memakai kuota, dan kuota bawaannya hanya 10.000 unit sehari.
- **Membaca chat seperti jendela chat di youtube.com.** Jendela chat di browser mengambil komentar dari server YouTube setiap beberapa detik, dan program lain bisa meniru caranya. Cara ini tidak butuh kuota dan secepat browser. Kelemahannya, formatnya tidak resmi dan bisa berubah kapan saja, dan Persyaratan Layanan YouTube melarang akses otomatis ke layanannya tanpa izin tertulis dari YouTube.

Seri ini memakai YouTube Data API, karena caranya resmi dan tidak akan berhenti bekerja saat YouTube mengubah halaman web-nya. Yang dikorbankan adalah kecepatan. Kuota harian membatasi seberapa sering chat bisa dibaca, dan bagian berikutnya membahas cara membaginya.

## Kuota harian adalah "anggaran" untuk membaca chat

Satu panggilan untuk membaca live chat, yaitu `liveChatMessages.list`, memakai 5 unit kuota. Dengan kuota 10.000 unit sehari, program kita hanya boleh membaca chat 2.000 kali sehari. Kalau dibagi rata, satu kali setiap 43,2 detik.

Membagi rata adalah cara paling sederhana, tetapi tidak cocok dengan cara orang mengobrol. Live chat radio kebanyakan sepi, lalu tiba-tiba ada beberapa komentar berturut-turut. Penonton yang membalas sapaan ingin melihat balasannya cepat, bukan 43 detik kemudian.

Karena itu, kuota dibagi menjadi dua jatah. Keduanya adalah:

- **Jatah dasar** adalah 1.600 panggilan sehari, yaitu satu panggilan setiap 54 detik. Jatah ini berjalan terus, sepi atau ramai.
- **Jatah tambahan** adalah 300 panggilan sehari, yang terkumpul satu setiap 4,8 menit, sampai paling banyak 30. Selama 2 menit setelah ada komentar baru, program memakai jatah ini untuk membaca chat secepat yang diminta YouTube.

Dalam sehari, chat yang sepi memakai 1.600 panggilan, yaitu 8.000 unit. Chat yang terus ramai memakai paling banyak 1.930 panggilan, yaitu 9.650 unit. Sisa kuotanya dipakai untuk mencari siaran dan live chat-nya, yang dibahas di bagian anatomi di bawah.

![Garis waktu 10 menit live chat. Di atas, titik abu-abu adalah panggilan jatah dasar, satu setiap 54 detik. Di menit ke-4 sebuah komentar datang, dan terbaca 40 detik kemudian di panggilan berikutnya. Sesudahnya, titik biru yang rapat adalah panggilan jatah tambahan selama 2 menit. Di bawah, area jatah tambahan penuh di 30, turun selama 2 menit itu, lalu naik perlahan, satu setiap 4,8 menit.](assets/quota-budget.png)

*Jatah dasar menjaga chat tetap terbaca. Jatah tambahan dipakai saat penonton sedang mengobrol.*

### Bukankah 54 detik terlalu lama untuk sebuah sapaan?

Untuk percakapan, 54 detik memang lama. Komentar pertama setelah chat sepi bisa menunggu selama itu sebelum terbaca, ditambah latency siaran. Komentar sesudahnya terbaca jauh lebih cepat, karena jatah tambahan sudah dipakai.

Ada tiga cara untuk mempercepatnya, dan masing-masing punya syarat. Ketiganya adalah:

- **Meminta kuota tambahan.** YouTube menyediakan formulir permintaan kuota. Kuota tambahan baru berlaku setelah permintaan itu disetujui YouTube.
- **Memakai `liveChatMessages.streamList`.** Cara resmi ini membuka satu koneksi yang terus terbuka, dan YouTube mengirim komentar baru begitu ada. Cara ini lebih cocok untuk siaran 24 jam, tetapi saya belum mencobanya, jadi tidak dipakai di seri ini.
- **Membaca seperti jendela chat di youtube.com.** Cara ini paling cepat, dengan kelemahan yang sudah dibahas di atas.

Untuk radio yang menyapa, bukan untuk percakapan, jeda itu masih sepadan. Penonton yang berkomentar melihat namanya di layar dalam waktu kurang dari satu menit.

## Anatomi pembaca live chat

Sebelum menulis kode, kita lihat dulu bagian-bagian pembaca live chat. Live chat milik sebuah siaran punya ID sendiri, dan ID itu harus dicari dari ID channel. Langkah-langkahnya adalah:

1. **Mencari siaran yang sedang live** dengan `search.list`, berdasarkan ID channel. `search.list` punya kuota sendiri, yaitu 100 panggilan sehari, terpisah dari 10.000 unit tadi.
2. **Mencari ID live chat** milik siaran itu dengan `videos.list`, yang memakai 1 unit kuota.
3. **Membaca live chat berulang-ulang** dengan `liveChatMessages.list`. Setiap jawaban berisi komentar baru, `nextPageToken` untuk panggilan berikutnya, dan `pollingIntervalMillis`, yaitu jeda yang diminta YouTube.
4. **Meneruskan setiap komentar ke terminal,** setelah dibersihkan dan diperiksa.

Kalau channel belum live, langkah pertama diulang. Jedanya mulai dari 1 menit, dua kali lebih panjang setiap kali gagal, sampai 15 menit. Dengan jeda 15 menit, pencarian paling banyak 96 kali sehari, masih di bawah kuota 100 panggilan.

Jawaban pertama dari live chat berisi komentar yang ditulis sebelum program kita mulai. Komentar lama itu dilewati, karena menyapa orang yang sudah pergi hanya membuat layar ramai. Saat siaran selesai, YouTube mengisi `offlineAt`, dan pembaca chat kembali ke langkah pertama.

![Diagram alur pembaca live chat. Kotak search.list dengan kuota 100 panggilan sehari menghasilkan ID video, lalu videos.list menghasilkan ID live chat, lalu liveChatMessages.list diulang selama siaran live dengan pageToken dan jeda dari budget. Komentar lalu diperiksa dan masuk ke antrean terminal. Tombstone dan ban menghapus komentar dari antrean. Garis putus-putus kembali ke search.list saat siaran selesai atau belum live.](assets/chat-reader-anatomy.png)

*Tiga panggilan API, dan hanya yang terakhir yang diulang terus.*

## Komentar apa saja yang boleh tampil di layar?

Semua yang tampil di layar adalah wajah channel-mu, 24 jam sehari. Menampilkan komentar apa adanya memang paling jujur, tetapi satu komentar berisi link penipuan akan ikut diketik di layar. Karena itu, setiap komentar melewati lima aturan. Kelimanya adalah:

- **Komentar yang dihapus moderator ikut hilang.** YouTube mengirim pesan `tombstone` untuk komentar yang dihapus, dan terminal membuangnya dari antrean, dari riwayat, bahkan dari baris input kalau sedang diketik.
- **Penulis yang diblokir hilang seluruhnya.** Untuk pesan `userBannedEvent`, semua komentar dari channel penulis itu dibuang.
- **Kata terlarang tidak pernah diketik.** Daftar kata terlarang dibaca dari sebuah file, dan komentar yang mengandung salah satunya hanya dicatat di log.
- **Link dibuang.** Siaran 24 jam yang menampilkan link adalah papan iklan gratis bagi spammer.
- **Emoji dan simbol bergambar dibuang,** karena font monospace kita tidak punya gambar untuk emoji. Komentar yang lebih panjang dari 120 huruf juga dipotong.

Kelemahannya, sebagian komentar yang wajar kehilangan emoji atau link-nya. Untuk radio yang berjalan tanpa diawasi, itu masih sepadan.

## Menulis pembaca live chat

Pembaca live chat ada di `radio/chat.go`. Kodenya adalah:

```go
package radio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const youtubeAPI = "https://www.googleapis.com/youtube/v3"

// Chat membaca live chat sebuah channel lewat YouTube Data API, lalu
// menyapa setiap komentar di layar.
type Chat struct {
	Key     string   // API key dari Google Cloud Console
	Channel string   // ID channel, diawali "UC"
	Blocked []string // kata, huruf kecil, yang membuat komentar tidak disapa
	Log     *log.Logger

	// API adalah alamat YouTube Data API, dan Every adalah jarak rata-rata
	// di antara panggilan saat chat sepi. Test mengganti keduanya.
	API   string
	Every time.Duration
}

// Run membaca chat sampai ctx selesai. Saat channel belum live, atau
// siarannya berganti video, live chat dicari lagi.
func (c *Chat) Run(ctx context.Context, s *Screen) {
	b := newBudget(c.Every)
	wait := time.Minute
	for ctx.Err() == nil {
		chatID, err := c.findChat(ctx)
		if err != nil {
			c.Log.Printf("chat: %v; mencari lagi dalam %s", err, wait)
			sleep(ctx, wait)
			// search.list hanya boleh dipanggil 100 kali sehari, jadi
			// pencarian paling sering 15 menit sekali.
			wait = min(2*wait, 15*time.Minute)
			continue
		}
		wait = time.Minute
		c.Log.Printf("chat: membaca live chat %s", chatID)
		err = c.read(ctx, chatID, s, b)
		if ctx.Err() == nil {
			c.Log.Printf("chat: %v", err)
			sleep(ctx, time.Minute)
		}
	}
}

// findChat mencari live chat milik siaran yang sedang live di channel ini.
func (c *Chat) findChat(ctx context.Context) (string, error) {
	var found struct {
		Items []struct {
			ID struct {
				VideoID string `json:"videoId"`
			} `json:"id"`
		} `json:"items"`
	}
	err := c.get(ctx, "search", url.Values{"part": {"id"}, "channelId": {c.Channel},
		"eventType": {"live"}, "type": {"video"}}, &found)
	if err != nil {
		return "", err
	}
	if len(found.Items) == 0 {
		return "", errors.New("channel sedang tidak live")
	}
	var videos struct {
		Items []struct {
			Live struct {
				ChatID string `json:"activeLiveChatId"`
			} `json:"liveStreamingDetails"`
		} `json:"items"`
	}
	err = c.get(ctx, "videos", url.Values{"part": {"liveStreamingDetails"},
		"id": {found.Items[0].ID.VideoID}}, &videos)
	if err != nil {
		return "", err
	}
	if len(videos.Items) == 0 || videos.Items[0].Live.ChatID == "" {
		return "", errors.New("siaran ini tidak punya live chat")
	}
	return videos.Items[0].Live.ChatID, nil
}

// chatMessage adalah bagian dari satu pesan live chat yang kita pakai.
type chatMessage struct {
	ID      string `json:"id"`
	Snippet struct {
		Type string `json:"type"`
		Text struct {
			MessageText string `json:"messageText"`
		} `json:"textMessageDetails"`
		SuperChat struct {
			Comment string `json:"userComment"`
		} `json:"superChatDetails"`
		Banned struct {
			User struct {
				ChannelID string `json:"channelId"`
			} `json:"bannedUserDetails"`
		} `json:"userBannedDetails"`
	} `json:"snippet"`
	Author struct {
		ChannelID string `json:"channelId"`
		Name      string `json:"displayName"`
	} `json:"authorDetails"`
}

// read membaca satu live chat sampai siarannya selesai atau terjadi error.
func (c *Chat) read(ctx context.Context, chatID string, s *Screen, b *budget) error {
	token := ""
	for first := true; ; first = false {
		params := url.Values{"liveChatId": {chatID}, "part": {"snippet,authorDetails"}}
		if token != "" {
			params.Set("pageToken", token)
		}
		var page struct {
			Items                 []chatMessage `json:"items"`
			NextPageToken         string        `json:"nextPageToken"`
			PollingIntervalMillis int           `json:"pollingIntervalMillis"`
			OfflineAt             string        `json:"offlineAt"`
		}
		if err := c.get(ctx, "liveChat/messages", params, &page); err != nil {
			return err
		}
		if page.OfflineAt != "" {
			return errors.New("siaran sudah selesai")
		}
		// Halaman pertama berisi komentar lama, yang tidak perlu disapa.
		if !first {
			for _, m := range page.Items {
				c.handle(m, s)
			}
		}
		token = page.NextPageToken
		active := !first && len(page.Items) > 0
		interval := time.Duration(page.PollingIntervalMillis) * time.Millisecond
		sleep(ctx, b.next(time.Now(), active, interval))
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

// handle meneruskan satu pesan ke layar. Komentar dan Super Chat disapa.
// Komentar yang dihapus moderator, dan semua komentar dari penulis yang
// diblokir, hilang dari layar.
func (c *Chat) handle(m chatMessage, s *Screen) {
	switch m.Snippet.Type {
	case "textMessageEvent", "superChatEvent":
		text := m.Snippet.Text.MessageText
		if m.Snippet.Type == "superChatEvent" {
			text = m.Snippet.SuperChat.Comment
		}
		name, comment := atName(m.Author.Name), cleanComment(text)
		if name == "" {
			return
		}
		if w := blockedWord(c.Blocked, name+" "+comment); w != "" {
			c.Log.Printf("chat: %s tidak disapa, ada kata %q", name, w)
			return
		}
		s.Greet(m.ID, m.Author.ChannelID, name, comment)
	case "tombstone":
		s.Remove(func(e entry) bool { return e.id == m.ID })
	case "userBannedEvent":
		banned := m.Snippet.Banned.User.ChannelID
		s.Remove(func(e entry) bool { return e.channel == banned })
	}
}

// get memanggil satu method YouTube Data API dan membaca jawabannya.
func (c *Chat) get(ctx context.Context, method string, params url.Values, out any) error {
	api := c.API
	if api == "" {
		api = youtubeAPI
	}
	params.Set("key", c.Key)
	req, err := http.NewRequestWithContext(ctx, "GET", api+"/"+method+"?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// Error dari http menyebut URL-nya, dan URL itu berisi API key.
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return fmt.Errorf("%s: %w", method, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("%s: %s %s", method, resp.Status, e.Error.Message)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// budget membagi kuota harian di antara panggilan liveChatMessages.list.
// Kuota bawaan adalah 10.000 unit sehari, dan setiap panggilan memakai 5
// unit, jadi paling banyak 2.000 panggilan sehari. Budget memakai paling
// banyak 1.930 di antaranya:
//
//   - jatah dasar, 1.600 panggilan, yaitu satu panggilan setiap 54 detik;
//   - jatah tambahan, 300 panggilan, yang terkumpul satu setiap 4,8 menit
//     sampai paling banyak 30. Jatah ini dipakai untuk membaca chat secepat
//     yang diminta YouTube, selama 2 menit setelah ada komentar.
//
// Sisa kuotanya dipakai untuk videos.list.
type budget struct {
	every  time.Duration // jarak jatah dasar
	extra  float64       // jatah tambahan yang terkumpul
	last   time.Time     // panggilan terakhir
	recent time.Time     // komentar terakhir
}

const (
	extraEvery = 288 * time.Second // 300 panggilan tambahan sehari
	maxExtra   = 30
)

func newBudget(every time.Duration) *budget {
	if every == 0 {
		every = 54 * time.Second
	}
	return &budget{every: every, extra: 10}
}

// next adalah jeda sebelum panggilan berikutnya. active berarti panggilan
// yang baru selesai membawa komentar baru, dan interval adalah jeda yang
// diminta YouTube.
func (b *budget) next(now time.Time, active bool, interval time.Duration) time.Duration {
	if !b.last.IsZero() {
		b.extra = min(maxExtra, b.extra+float64(now.Sub(b.last))/float64(extraEvery))
	}
	b.last = now
	if active {
		b.recent = now
	}
	if now.Sub(b.recent) < 2*time.Minute && b.extra >= 1 {
		b.extra--
		return max(interval, time.Second)
	}
	return max(interval, b.every)
}

var linkPattern = regexp.MustCompile(`(?i)(https?://|www\.)\S+`)

// cleanComment membuang yang tidak bisa atau tidak boleh tampil di layar:
// link, emoji dan simbol bergambar, serta karakter kontrol. Komentar yang
// terlalu panjang dipotong.
func cleanComment(s string) string {
	s = linkPattern.ReplaceAllString(s, " ")
	s = strings.Map(func(r rune) rune {
		switch {
		case unicode.IsSpace(r):
			return ' '
		case !drawable(r):
			return -1
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 120 {
		s = string(r[:119]) + "…"
	}
	return s
}

// drawable menentukan apakah sebuah huruf bisa digambar oleh font
// monospace kita. Emoji ada di atas U+1F000 atau termasuk simbol lain
// (So), seperti ☕ dan ❤.
func drawable(r rune) bool {
	return unicode.IsPrint(r) && r < 0x1F000 &&
		!unicode.Is(unicode.So, r) && !unicode.Is(unicode.Variation_Selector, r)
}

// atName adalah nama penulis sebagai "@nama".
func atName(name string) string {
	name = cleanComment(name)
	if name != "" && !strings.HasPrefix(name, "@") {
		name = "@" + name
	}
	return name
}

// blockedWord adalah kata terlarang pertama yang ada di s.
func blockedWord(blocked []string, s string) string {
	s = strings.ToLower(s)
	for _, w := range blocked {
		if strings.Contains(s, w) {
			return w
		}
	}
	return ""
}
```

File ini punya enam bagian. Keenamnya adalah:

- **`Chat`** menyimpan API key, ID channel, dan kata terlarang. `API` dan `Every` hanya diganti oleh test, supaya test memakai server palsu dan jeda yang pendek.
- **`Run`** mencari live chat, membacanya dengan `read` sampai siaran selesai, lalu mencari lagi.
- **`findChat`** menjalankan langkah 1 dan 2 dari anatomi tadi.
- **`read`** membaca live chat berulang-ulang dengan `pageToken`, melewati halaman pertama, lalu menunggu jeda dari `budget`.
- **`handle`** meneruskan komentar dan Super Chat ke terminal, dan menghapus komentar yang dihapus atau diblokir. Super Chat adalah komentar berbayar, dan pesannya ada di field `userComment`.
- **`get`** memanggil satu method API. API key dikirim sebagai parameter `key`, jadi URL-nya berisi API key. Error dari library `net/http` menyebut URL itu, jadi `get` hanya meneruskan error di dalamnya, supaya API key tidak masuk log.

`drawable` memakai dua aturan untuk mengenali emoji. Hampir semua emoji ada di atas U+1F000, dan sisanya, seperti ☕ dan ❤, termasuk golongan simbol lain, yang disebut `So` di Unicode. Variation selector, yaitu karakter tak terlihat yang mengubah tampilan emoji, juga dibuang.

## Terminal yang menyapa dan menghapus komentar

Terminal dari Bagian 3 mendapat dua kemampuan baru, yaitu menyapa dan menghapus. Setiap perintah sekarang mengingat dari komentar mana ia berasal. Bagian yang berubah di `radio/screen.go` adalah:

```go
// entry adalah satu perintah di terminal, beserta keluarannya.
type entry struct {
	command string
	output  []string
	id      string // ID komentar di live chat; kosong untuk lagu
	channel string // ID channel penulis komentar
}
```

`Screen` juga mendapat field `current`, yaitu perintah yang sedang diketik. Field-field yang dijaga mutex di `Screen` sekarang adalah:

```go
	mu       sync.Mutex
	history  []entry
	queue    []entry
	current  *entry // perintah yang sedang diketik
	typed    string // bagian perintah yang sudah diketik
	cursorOn bool
```

`Greet` dan `Remove` adalah:

```go
// Greet mengantrekan sapaan untuk satu komentar di live chat.
func (s *Screen) Greet(id, channel, name, comment string) {
	command := "greet " + name
	if comment != "" {
		command += ` "` + strings.ReplaceAll(comment, `"`, `'`) + `"`
	}
	s.add(entry{command: command, output: []string{"Hello, " + name + "!"}, id: id, channel: channel})
}

// Remove menghapus komentar dari antrean, dari riwayat, dan dari baris
// input kalau sedang diketik. Dipakai saat moderator menghapus komentar
// atau memblokir penulisnya.
func (s *Screen) Remove(match func(entry) bool) {
	s.mu.Lock()
	keep := func(list []entry) []entry {
		var kept []entry
		for _, e := range list {
			if !match(e) {
				kept = append(kept, e)
			}
		}
		return kept
	}
	s.queue = keep(s.queue)
	s.history = keep(s.history)
	if s.current != nil && match(*s.current) {
		s.current = nil
	}
	s.mu.Unlock()
	s.render()
}
```

`Greet` mengganti tanda kutip ganda di komentar dengan tanda kutip tunggal, supaya perintah `greet` di layar tetap terbaca sebagai satu kalimat dalam tanda kutip. `Remove` menerima fungsi pencocok, jadi satu fungsi yang sama dipakai untuk komentar yang dihapus dan untuk penulis yang diblokir.

`typeOut` sekarang memeriksa, di setiap huruf, apakah komentar yang sedang diketik baru saja dihapus. Kalau iya, baris input dikosongkan, dan perintah itu tidak pernah naik ke riwayat. Kodenya menjadi:

```go
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
	s.current, s.cursorOn = &e, true
	s.mu.Unlock()
	typed := ""
	for _, r := range e.command {
		typed += string(r)
		s.mu.Lock()
		gone := s.current == nil // komentar dihapus di tengah jalan
		if gone {
			s.typed = ""
		} else {
			s.typed = typed
		}
		s.mu.Unlock()
		s.render()
		if gone {
			return
		}
		sleep(ctx, step)
	}
	sleep(ctx, 300*time.Millisecond) // Enter

	s.mu.Lock()
	if s.current != nil {
		s.history = append(s.history, e)
		if len(s.history) > keepLines {
			s.history = s.history[1:]
		}
	}
	s.current, s.typed = nil, ""
	s.mu.Unlock()
	s.render()
	sleep(ctx, pause)
}
```

## Menyambungkan live chat ke radio

`Config` di `radio/stream.go` mendapat satu field baru, dan `Run` menjalankan pembaca chat di goroutine-nya sendiri kalau field itu diisi. Kedua bagian itu adalah:

```go
	Chat   *Chat  // live chat yang disapa; nil kalau tidak ada
```

```go
	if c.Chat != nil {
		wg.Go(func() { c.Chat.Run(ctx, screen) })
	}
```

Di `cmd/radioku/main.go`, ada dua flag baru, yaitu `-channel` untuk ID channel dan `-blokir` untuk file kata terlarang. API key dibaca dari variabel lingkungan `YOUTUBE_API_KEY`, supaya tidak tertulis di perintah. Bagian yang berubah adalah:

```go
	channel := flag.String("channel", "", "ID channel YouTube yang live chat-nya disapa, diawali UC")
	blocked := flag.String("blokir", "", "file berisi kata terlarang, satu kata per baris")
```

```go
	if *channel != "" {
		key := os.Getenv("YOUTUBE_API_KEY")
		if key == "" {
			logger.Fatal("isi variabel lingkungan YOUTUBE_API_KEY dengan API key YouTube Data API")
		}
		c.Chat = &radio.Chat{Key: key, Channel: *channel, Log: logger}
		if *blocked != "" {
			words, err := os.ReadFile(*blocked)
			if err != nil {
				logger.Fatal(err)
			}
			c.Chat.Blocked = strings.Fields(strings.ToLower(string(words)))
		}
	}
```

## Mencoba sapaan di layar

Test untuk pembaca chat ada di `radio/chat_test.go`. Test yang memeriksa anggaran kuota mensimulasikan satu hari penuh, dengan jam palsu, untuk chat yang sepi dan chat yang terus ramai. Test itu adalah:

```go
func TestAnggaranCukupSehari(t *testing.T) {
	for _, active := range []bool{false, true} {
		b := newBudget(0)
		now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
		end := now.Add(24 * time.Hour)
		calls := 0
		for now.Before(end) {
			calls++
			// YouTube meminta jeda 2 detik; saat ramai, setiap panggilan
			// membawa komentar baru.
			now = now.Add(b.next(now, active, 2*time.Second))
		}
		// Chat yang sepi memakai jatah dasar saja, 1.600 panggilan. Chat yang
		// ramai juga memakai jatah tambahan, paling banyak 330, yaitu 300
		// yang terkumpul sehari dan 30 yang mungkin sudah ada.
		if calls > 1930 || active && calls <= 1600 || !active && calls != 1600 {
			t.Errorf("chat ramai=%v: %d panggilan sehari, %d unit", active, calls, 5*calls)
		}
		t.Logf("chat ramai=%v: %d panggilan sehari", active, calls)
	}
}
```

Test terbesar, `TestChatMenyapaDanMenghapus`, menjalankan pembaca chat terhadap YouTube Data API palsu, yaitu server HTTP kecil di dalam test yang menjawab dengan format dari dokumentasi YouTube. Server itu menjawab empat halaman live chat. Isi halaman-halaman itu adalah:

1. Satu komentar lama, yang tidak boleh disapa.
2. Komentar dari `@budi` dengan emoji dan link, Super Chat dari Sari, dan komentar dari `@toko` yang berisi kata terlarang "beli".
3. Moderator menghapus komentar Budi dan memblokir Sari.
4. Komentar dari `@ani`.

Setelah keempat halaman itu, antrean terminal harus berisi satu perintah saja, yaitu `greet @ani "lagunya enak"`. Test itu juga memeriksa bahwa `pageToken` diteruskan dengan benar, dan bahwa API key tidak pernah muncul di log. Hasil `go test -v` untuk test pembaca chat adalah:

```text
=== RUN   TestKomentarDibersihkan
--- PASS: TestKomentarDibersihkan (0.00s)
=== RUN   TestAnggaranCukupSehari
    chat_test.go:50: chat ramai=false: 1600 panggilan sehari
    chat_test.go:50: chat ramai=true: 1898 panggilan sehari
--- PASS: TestAnggaranCukupSehari (0.00s)
=== RUN   TestChatMenyapaDanMenghapus
--- PASS: TestChatMenyapaDanMenghapus (3.03s)
=== RUN   TestChatSaatTidakLive
--- PASS: TestChatSaatTidakLive (0.00s)
=== RUN   TestAPIKeyTidakMasukError
--- PASS: TestAPIKeyTidakMasukError (0.00s)
PASS
ok  	radioku/radio	3.873s
```

Chat yang sepi memakai tepat 1.600 panggilan sehari, dan chat yang terus ramai memakai 1.898 panggilan, yaitu 9.490 unit. Keduanya masih di bawah kuota 10.000 unit.

### Menyiapkan API key dan ID channel

Untuk mencobanya dengan YouTube sungguhan, kamu butuh dua hal. Keduanya adalah:

- **API key** dari Google Cloud Console. Buat sebuah project, aktifkan YouTube Data API v3 di project itu, lalu buat API key. Batasi API key itu supaya hanya bisa dipakai untuk YouTube Data API v3, sehingga kalau bocor, kerugiannya kecil.
- **ID channel,** yang diawali `UC`. Di YouTube, buka Settings dari foto profilmu, lalu Advanced settings. ID channel hanya terlihat oleh pemilik utama channel.

Lalu jalankan radionya dengan flag `-channel`, dan buat siaran live di YouTube Studio dengan stream key yang sama. Perintahnya adalah:

```sh
export STREAM_KEY=xxxx-xxxx-xxxx-xxxx-xxxx
export YOUTUBE_API_KEY=AIza...
printf 'beli\npromo\n' > blokir.txt
go run ./cmd/radioku -latar latar/latar.mp4 -musik musik -channel UCxxxxxxxxxxxxxxxxxxxxxx -blokir blokir.txt
```

Ada satu hal yang perlu kamu tahu sebelum memakainya. Kode di bagian ini belum saya jalankan terhadap YouTube sungguhan, karena butuh API key dan siaran yang sedang live. Test-nya memakai server palsu dengan format dari dokumentasi YouTube Data API.

Dokumentasi resmi juga tidak menyebut dengan jelas apakah API key saja cukup untuk membaca live chat siaran publik, meskipun panduan dari komunitas menyebut cukup.

Gambar di awal artikel ini dibuat dengan cara yang sama seperti di Bagian 3. Satu frame digambar dengan filter yang sama, dari file-file teks yang isinya ditentukan.

## Kelemahan pembaca live chat versi keempat

Pembaca live chat ini sudah menyapa penonton, menghormati moderator, dan menjaga kuota harian. Tetapi masih ada lima kelemahan yang perlu kamu tahu. Kelimanya adalah:

- **Komentar pertama bisa menunggu sampai 54 detik.** Setelah chat sepi, komentar baru terbaca di panggilan jatah dasar berikutnya.
- **Belum diuji dengan YouTube sungguhan.** Format jawaban diambil dari dokumentasi, dan kode ini perlu dicoba pada siaran nyata sebelum diandalkan.
- **Jatah tambahan mulai dari 10 setiap kali program dijalankan.** Kalau program sering dijalankan ulang, kuota harian bisa terpakai sedikit lebih banyak dari perhitungan.
- **Antrean bisa membuang komentar.** Saat lebih dari 10 komentar menunggu, komentar tertua tidak pernah diketik.
- **Semuanya masih berjalan di PC.** Radio yang sesungguhnya harus menyala sendiri dan berjalan berbulan-bulan, di komputer kecil yang tidak perlu diawasi.

## Berikutnya dalam seri ini

Di Bagian 5, kita memindahkan radio ke Raspberry Pi. Kita memilih encoder hardware dengan mencobanya lebih dulu, mengukur apakah Raspberry Pi tertinggal dari waktu siaran, dan membuat radio menyala sendiri setiap kali Raspberry Pi dinyalakan. Bagian itu juga menutup seri ini, dengan rangkuman kelemahan desainnya dan kapan sebaiknya memilih alat lain.
