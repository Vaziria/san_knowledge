# Mengotomasi posting media sosial lewat browser, dan kapan sebaiknya tidak

*Banyak platform tidak lagi memberi API posting yang mudah dipakai perorangan. Artikel ini membahas dua cara mengendalikan browser sungguhan, satu post yang dijadwalkan langkah demi langkah, dan risikonya, dari syarat layanan sampai akun yang diblokir.*

Kamu menulis satu artikel, lalu ingin membagikannya ke Threads, X, LinkedIn, dan Facebook setiap minggu pada jam yang sama. Langkah pertama seorang programmer biasanya menulis program kecil yang memanggil API setiap platform. Artikel ini membahas kenapa jalan itu makin sempit, bagaimana browser mengisi celahnya, dan kapan jalan ini justru tidak layak diambil.

## Kenapa API posting makin sulit dipakai perorangan?

API adalah daftar fungsi yang ditawarkan sebuah layanan untuk dipanggil oleh program lain, seperti fungsi "buat post baru". Untuk posting, jalan resmi ini makin sempit bagi perorangan. Saya membuka dokumentasi resmi empat platform untuk melihat apa yang masih tersisa, dan hasilnya adalah:

- **Medium** menulis di pusat bantuannya bahwa mereka tidak lagi menerbitkan integration token baru untuk API-nya dan tidak menerima integrasi baru. Token lama masih berjalan, tetapi pengguna baru tidak bisa mendapatkannya.
- **X** sekarang memasang harga pay-per-use di dokumentasi API-nya. Kamu membeli kredit lebih dulu, lalu setiap post lewat API dipotong US$0,015, atau US$0,20 jika post itu berisi URL, menurut changelog X yang berlaku sejak 20 April 2026.
- **Facebook** menghentikan izin `publish_actions` mulai 24 April 2018, dan aplikasi lama hanya boleh memakainya sampai 1 Agustus 2018. Izin itu dulu membuat aplikasi bisa memposting sebagai pengguna yang login, dan Meta menyarankan Share dialog sebagai gantinya, yaitu jendela tempat orangnya sendiri yang menekan tombol. Memposting ke Page Facebook masih bisa, tetapi lewat aplikasi yang memakai Facebook Login dan token khusus untuk Page itu.
- **LinkedIn** masih membuka satu pintu. Izin `w_member_social` tersedia untuk semua developer tanpa persetujuan khusus, untuk membuat post atas nama dirimu sendiri. Tetapi "article" di Posts API adalah post berisi tautan dengan judul, deskripsi, dan gambar kecil, dan saya tidak menemukan cara menulis artikel panjang LinkedIn di dokumentasinya.

Platform jarang menjelaskan alasannya, dan halaman Medium tadi tidak menyebutnya sama sekali. X menyebut alasan untuk beberapa pembatasannya, yaitu melawan penyalahgunaan, manipulasi platform, dan interaksi palsu, termasuk spam balasan yang ditulis dengan AI. Jalan tulis lewat API memang jalan yang paling mudah dipakai spammer, sehingga platform menutupnya, membatasinya, atau menjualnya.

Bukankah X dan LinkedIn masih punya API? Itu benar, dan jika API yang ada sudah mencakup kebutuhanmu, pakailah API itu, seperti yang dibahas di bagian akhir. Masalahnya, profil pribadi Facebook, Medium, dan artikel LinkedIn tidak punya jalan API untukmu, padahal semuanya bisa kamu kerjakan dengan tangan di browser.

## Browser adalah "pintu yang selalu terbuka"

Apa pun yang boleh kamu lakukan dengan tangan di sebuah platform, bisa kamu lakukan di browser. Otomasi browser berarti sebuah program menjalankan langkah yang biasa kamu kerjakan dengan mouse dan keyboard. Seperti membuka halaman, mengetik, memilih file, dan menekan tombol.

Bedanya dengan API, program tidak memanggil server platform secara langsung. Kode halaman itu sendiri yang mengirim request, persis seperti saat kamu menekan tombolnya. Karena itu, cara ini tidak butuh token, aplikasi terdaftar, atau kredit.

Tapi pintu ini tidak dibuat untuk program. Platform tidak berjanji halamannya akan tetap sama, dan beberapa platform melarang otomasi seperti ini di syarat layanannya. Jadi, otomasi browser cocok sebagai pengganti API untuk pekerjaan yang memang kamu lakukan sendiri di akunmu sendiri, bukan untuk mengirim ratusan post sehari.

## Dua cara mengendalikan browser sungguhan

Ada dua cara yang umum untuk mengendalikan browser dari program. Keduanya berbeda pada satu hal, yaitu browser mana yang dikendalikan.

### Cara pertama adalah "browser terpisah lewat Chrome DevTools Protocol"

Chrome DevTools Protocol, atau CDP, adalah protokol yang dipakai DevTools untuk memeriksa dan mengendalikan Chrome. Program kamu terhubung lewat debugging port, yaitu port yang dibuka browser khusus untuk dikendalikan, lalu mengirim perintah seperti "buka halaman ini" atau "ketik teks ini". Library seperti Puppeteer dan Playwright untuk JavaScript, atau chromedp untuk Go, membungkus protokol ini menjadi fungsi yang mudah dipanggil.

Browser seperti ini memakai profilnya sendiri, yaitu folder berisi login, cookie, dan pengaturan. Sejak Chrome 136, flag `--remote-debugging-port` tidak berlaku untuk profil bawaan Chrome, dan harus disertai `--user-data-dir` yang menunjuk folder lain. Google mengubahnya karena penyerang memakai debugging port untuk mencuri cookie dari profil pengguna.

Kelebihan cara ini adalah kendali penuh, karena program bisa menjalankan browser headless, yaitu browser tanpa jendela, di server, dengan akun uji yang terpisah. Kelemahannya, kamu harus login lagi di profil itu, dan halaman bisa tahu bahwa browser-nya dikendalikan. Menurut MDN, `navigator.webdriver` bernilai `true` di Chrome yang dijalankan dengan flag `--enable-automation`, `--headless`, atau `--remote-debugging-port` bernilai 0.

### Cara kedua adalah "extension di browser yang kamu pakai setiap hari"

Extension adalah program kecil yang dipasang di dalam browser dan boleh memakai API milik browser itu. Extension Chrome sekarang ditulis dengan Manifest V3, aturan terbaru untuk extension Chrome. Berbeda dengan cara pertama, extension berjalan di browser yang sudah kamu pakai, dengan semua login yang sudah ada.

Untuk menjalankan kode yang dikirim dari luar, extension memakai `chrome.userScripts.execute`, yang tersedia sejak Chrome 135. Extension butuh izin `userScripts`, dan sejak Chrome 138 pengguna harus menyalakan sakelar **Allow User Scripts** di halaman detail extension itu. Kodenya berjalan di lingkungan khusus yang tidak terlihat oleh halaman dan tidak terkena CSP halaman, yaitu aturan halaman tentang kode mana yang boleh berjalan.

Pemanggilnya adalah service worker, yaitu bagian extension yang berjalan di latar belakang tanpa halaman sendiri. Potongan ilustrasinya adalah:

```js
// Ilustrasi saja, bukan kode siap pakai.
// Di service worker extension: jalankan kode di sebuah tab, lalu baca hasilnya.
const [hasil] = await chrome.userScripts.execute({
  target: { tabId },           // nomor tab tujuan
  js: [{ code: "document.title" }],
  world: "USER_SCRIPT",
});
console.log(hasil.error ?? hasil.result);
```

Kelebihan cara ini adalah tidak ada debugging port dan tidak ada login kedua. Kelemahannya, event yang dibuat script ditandai buatan. Menurut MDN, `isTrusted` bernilai `false` untuk event yang dikirim dengan `dispatchEvent()`, juga untuk klik dari `click()`, dan sebagian halaman mengabaikan event seperti itu.

Jadi, CDP cocok jika program berjalan di server tanpa layar, atau memakai akun uji yang memang terpisah. Extension cocok jika yang diotomasi adalah akunmu sendiri di komputer yang kamu pakai, karena login-nya sudah ada. Kelemahannya, extension hanya bekerja selama browser-mu menyala.

![Dua panel berdampingan. Kiri, cara 1: program kamu dengan Puppeteer, Playwright, atau chromedp terhubung lewat CDP dan debugging port ke browser terpisah dengan profil sendiri, yang butuh login lagi dan bisa menunjukkan navigator.webdriver true. Kanan, cara 2: program kamu mengirim perintah ke server lokal di 127.0.0.1, extension di browser harianmu menghubungi server itu, lalu menjalankan perintah di tab yang sudah login.](assets/dua-cara-mengendalikan-browser.png)

*Cara pertama membawa browser sendiri. Cara kedua menumpang di browser yang sudah kamu pakai.*

## Anatomi otomasi lewat extension

Otomasi lewat extension terdiri atas empat bagian, dan semuanya berada di satu komputer. Sisa artikel ini memakai cara kedua, karena yang ingin kita otomasi adalah akunmu sendiri, yang sudah login di browser harianmu. Keempat bagian itu adalah:

- **Program pengirim** adalah program yang tahu langkah-langkah untuk satu situs, seperti script yang kamu jalankan di terminal.
- **Server lokal** adalah program kecil yang menerima perintah dari program pengirim dan meneruskannya ke extension. Ia hanya mendengarkan di alamat loopback `127.0.0.1`, yaitu alamat yang hanya bisa dijangkau dari komputer itu sendiri.
- **Extension** menghubungi server lokal, lalu menunggu perintah. Setiap perintah ia jalankan dengan API browser, dan hasilnya ia kirim kembali.
- **Tab** adalah halaman situs yang sedang dikerjakan, lengkap dengan sesi login-mu.

Perintahnya sedikit dan umum. Seperti membuka tab dan menunggu halamannya dimuat, menjalankan script dan mengembalikan hasilnya sebagai JSON, membawa tab ke depan, mengambil screenshot, dan menutup tab.

Pengetahuan tentang setiap situs sengaja disimpan di program pengirim, bukan di extension. Alasannya, situs sering berubah, dan memperbaiki program jauh lebih mudah daripada memasang ulang extension. Kelemahannya, setiap langkah menempuh perjalanan bolak-balik lewat server, sehingga satu post bisa butuh puluhan perintah.

![Empat kotak dari kiri ke kanan di dalam satu komputer. Program pengirim, yang tahu langkah setiap situs, mengirim perintah buka tab, jalankan script, bawa ke depan, dan tutup tab ke server lokal di 127.0.0.1. Extension di browser harianmu menghubungi server itu, menerima perintah, lalu menjalankannya di tab lewat chrome.tabs dan chrome.userScripts.execute. Hasilnya kembali sebagai JSON lewat jalan yang sama.](assets/anatomi-otomasi-extension.png)

*Program tahu langkahnya, extension hanya menjalankan perintah.*

## Cara kerja satu post adalah "isi, baca ulang, baru tekan"

Satu post terjadwal melewati bagian-bagian tadi dalam tujuh langkah, dan dua langkah terpentingnya hanya membaca. Kita ikuti satu post berisi teks dan satu gambar, dan langkah-langkahnya adalah:

1. **Buka composer.** Composer adalah kotak tempat kamu menulis post. Program meminta tab baru di halaman utama situs dan membawanya ke depan, lalu script mencari tombol post baru dari namanya, seperti `aria-label`, bukan dari nama class CSS yang dibuat otomatis dan berubah setiap rilis.
2. **Tempel teks.** Editor teks yang kaya biasanya menerima event paste, yaitu event yang sama dengan saat kamu menekan Ctrl+V, dan baris barunya tetap terjaga. Kotak input biasa yang dikendalikan React butuh cara lain, yang dibahas di bagian berikutnya.
3. **Lampirkan file.** Script tidak bisa menjawab jendela pemilih file. Jadi program mengirim isi file ke tab, script membungkusnya sebagai `File` di dalam `DataTransfer`, lalu mengisi `files` milik input file di halaman itu. Menurut spesifikasi HTML, mengisi `files` hanya mengganti daftar file, jadi script harus mengirim event `change` atau `input` sendiri supaya halaman tahu.
4. **Atur jadwal.** Script membuka pemilih tanggal, memilih hari dan jam, lalu menutupnya.
5. **Baca ulang sebelum menekan tombol.** Script membaca kembali apa yang ditampilkan halaman, yaitu teks di composer, jumlah lampiran, dan kalimat jadwal seperti "akan dikirim Minggu pukul 19.00". Jika satu saja berbeda dari yang diminta, program berhenti tanpa menekan tombol dan menyimpan screenshot.
6. **Tekan tombol Jadwalkan,** hanya jika langkah 5 lolos.
7. **Pastikan post muncul di daftar terjadwal.** Program membuka daftar post terjadwal dan mencari tepat satu baris dengan teks dan waktu yang sama. Jika tidak ada, atau ada dua, program melaporkan gagal.

Langkah 5 dan 7 adalah inti dari cara ini. Halaman bisa menelan sebuah klik tanpa pesan apa pun, dan hanya membaca ulang yang membuktikan bahwa halaman menerima apa yang kamu maksud.

![Alur tujuh langkah dalam dua baris. Baris atas: buka composer, tempel teks, lampirkan file, atur jadwal. Lalu gerbang baca ulang yang memeriksa teks, lampiran, dan waktu. Jika semuanya cocok, alur lanjut ke tekan Jadwalkan, lalu cek daftar terjadwal. Jika ada yang berbeda, alur berhenti tanpa menekan tombol dan menyimpan screenshot. Gerbang baca ulang diberi warna.](assets/alur-satu-post.png)

*Tombol baru ditekan setelah halaman menunjukkan hal yang sama dengan yang diminta.*

## Pelajaran dari halaman yang sungguhan

Halaman sungguhan punya kebiasaan yang tidak terlihat di contoh, dan empat di antaranya paling sering membuat otomasi gagal.

### Halaman yang tersembunyi diperlambat oleh browser

Tab yang tidak terlihat tidak bekerja seperti tab di depan. Menurut MDN, `requestAnimationFrame()` dihentikan di tab latar belakang pada sebagian besar browser. Sejak Chrome 88, timer berantai di halaman yang tersembunyi lebih dari lima menit dan tidak bersuara hanya diperiksa sekali per menit.

Akibatnya, menu yang menunggu frame animasi tidak pernah terbuka. Bukankah lebih nyaman jika semuanya berjalan di latar belakang?

Untuk membaca isi halaman memang bisa, karena script tidak menunggu animasi. Tetapi langkah yang mengklik antarmuka butuh tab di depan, sehingga selama post diisi, jendela browser itu bukan milikmu.

### Input React butuh setter-nya sendiri

React, library tampilan yang dipakai banyak situs, menyimpan isi kotak input di state, yaitu data milik komponen yang menentukan apa yang digambar. React juga mencatat nilai terakhir yang ia lihat di kotak itu. Jika script menulis `el.value = "..."`, catatan itu ikut berubah, sehingga event input berikutnya terlihat seperti tidak ada perubahan.

Saya mencobanya dengan React 19 di jsdom, yaitu tiruan browser yang berjalan di Node.js. Setelah `el.value` diisi dan event input dikirim, kotak menampilkan teksnya, tetapi state React tetap kosong.

Fungsi di bawah ini memakai setter asli milik browser, dan dengannya state ikut berubah. Fungsinya adalah:

```js
// Ilustrasi: mengisi input atau textarea yang dikendalikan React.
function isiInput(el, teks) {
  const proto = Object.getPrototypeOf(el);
  const setter = Object.getOwnPropertyDescriptor(proto, "value").set;
  setter.call(el, teks);
  el.dispatchEvent(new Event("input", { bubbles: true }));
}
```

Editor yang kaya, seperti Lexical atau ProseMirror, punya model datanya sendiri, jadi fungsi ini hanya untuk input dan textarea.

### Verifikasi dengan membaca ulang dari aplikasi

Percobaan React tadi menunjukkan bahayanya. Kotak terlihat terisi, tetapi aplikasi menganggapnya kosong, dan post kosong itulah yang akan terkirim. Karena itu, yang dibaca ulang adalah apa yang ditampilkan aplikasi setelah memprosesnya, seperti penghitung karakter, preview lampiran, dan kalimat jadwal, lalu daftar terjadwal yang berasal dari server.

### Simpan satu langkah persetujuan manusia

Program sebaiknya mencetak setiap post dan panjangnya lebih dulu, tanpa membuka browser. Lalu kamu menyetujui teks dan waktunya, baru program menjadwalkannya. Pilih menjadwalkan daripada memposting langsung, karena post terjadwal masih bisa dihapus sebelum terbit.

## Kelemahan otomasi browser, dan kapan sebaiknya tidak memakainya

Otomasi browser terdengar seperti jalan keluar untuk semua platform, tetapi ada empat kelemahan yang perlu kamu timbang. Keempatnya adalah:

- **Syarat layanan dan akun yang diblokir.** Aturan otomasi X melarang otomasi yang tidak memakai API, seperti menjalankan script di situs X, dan menyebut akibatnya bisa berupa penangguhan akun permanen. Perjanjian pengguna LinkedIn melarang bot atau cara otomatis tanpa izin untuk membuat, mengomentari, menyukai, atau membagikan post. Syarat layanan Meta melarang mengakses atau mengumpulkan data dari produknya dengan cara otomatis tanpa izin.
- **CAPTCHA dan deteksi bot.** Halaman bisa memeriksa `navigator.webdriver`, event yang tidak `isTrusted`, atau klik yang terlalu cepat. Saat halaman meminta CAPTCHA, program harus berhenti dan menyerahkannya kepadamu, bukan mencoba menjawabnya.
- **Rapuh saat situs berubah.** Setiap perubahan tampilan bisa mematahkan satu langkah, dan platform tidak memberi tahu lebih dulu. Program yang baik berhenti dengan screenshot di langkah itu, tetapi tetap ada orang yang harus memperbaikinya, untuk setiap platform.
- **Keamanan.** Apa pun yang bisa mengirim perintah ke server lokal bisa bertindak sebagai dirimu di setiap situs tempat kamu login. Jadi server hanya mendengarkan di loopback, hanya menerima extension milikmu, dan menolak request yang membawa header `Origin`, yang menurut MDN ditambahkan browser pada request dari sebuah halaman ke alamat lain, seperti ke server lokalmu. Jangan membuka server ini ke jaringan tanpa token dan TLS.

Ada jalan yang lebih sederhana, dan sering lebih tepat. Ketiganya adalah:

- **API resmi**, jika ada dan cukup. Post LinkedIn atas nama dirimu bisa lewat `w_member_social`, dan post X lewat API jika US$0,015 per post masih sepadan. API stabil, didokumentasikan, dan diizinkan oleh platformnya.
- **Layanan penjadwal**, seperti Buffer atau Meta Business Suite. Buffer bisa menjadwalkan thread di X dan Threads, tetapi tidak bisa terhubung ke profil pribadi Facebook. Meta Business Suite, milik Meta sendiri, menjadwalkan post untuk Page Facebook dan akun Instagram.
- **Menjadwalkan dengan tangan.** Sejak Januari 2025, Threads punya menu Schedule di composer-nya sendiri, dan post terjadwal disimpan di draf. Jika kamu hanya memposting beberapa kali seminggu, menjadwalkannya sendiri lebih murah daripada merawat program.

Jadi, otomasi browser cocok jika platformnya tidak memberi API untuk akunmu, post-nya sedikit dan ditulis sendiri, ada manusia yang menyetujui setiap jadwal, dan kamu menerima risiko syarat layanan di akunmu sendiri. Otomasi browser kurang cocok jika API resmi sudah cukup, jika platformnya terang-terangan melarang otomasi seperti X, atau jika kamu ingin mengirim post dalam jumlah besar. Untuk kasus terakhir, yang kamu butuhkan bukan browser, tetapi izin resmi dari platformnya.
