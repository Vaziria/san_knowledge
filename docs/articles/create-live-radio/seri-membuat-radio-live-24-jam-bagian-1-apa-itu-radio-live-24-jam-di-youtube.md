# [Seri Membuat Radio Live 24 Jam Bagian 1] Apa itu radio live 24 jam di YouTube?

*Sebelum menulis kode, kita bedah dulu bagian-bagian sebuah radio live 24 jam, bagaimana gambar dan suaranya sampai ke penonton, dan kenapa semuanya bisa berjalan di sebuah Raspberry Pi.*

Dalam seri ini, kita membangun radio live 24 jam dari nol. Radio ini adalah live stream di YouTube yang memutar musik tanpa henti, dengan layar bergaya terminal yang mengetik judul setiap lagu dan menyapa penonton yang menulis komentar. Live stream adalah siaran langsung, yang ditonton orang pada saat siaran itu dibuat.

Semuanya berjalan di sebuah Raspberry Pi, komputer kecil seukuran kartu nama. Bagian ini menjelaskan bagian-bagian radio itu dan cara kerjanya, lalu ditutup dengan siaran pertama dari satu perintah.

## Apa itu radio live 24 jam?

Radio live 24 jam adalah siaran yang tidak pernah selesai. Isinya musik yang terus berganti, dengan gambar yang hampir diam. Penonton bisa datang kapan saja, mendengarkan sebentar atau berjam-jam, lalu pergi, dan siarannya tetap berjalan.

Di YouTube, siaran seperti ini banyak dipakai untuk musik santai, musik untuk belajar, atau suara hujan. Penonton memutarnya sebagai teman bekerja, bukan untuk ditatap.

### Kenapa harus live, dan bukan video biasa?

Video musik berdurasi satu jam juga bisa diputar kapan saja, dan jauh lebih sederhana. Video itu cukup di-upload sekali, lalu YouTube yang mengurus sisanya. Tetapi siaran live punya tiga hal yang tidak dimiliki video biasa. Ketiganya adalah:

- **Live chat.** Live chat adalah kolom komentar di samping siaran. Penonton bisa saling menyapa, dan radio kita akan membalas setiap komentar di layarnya.
- **Isi yang berubah tanpa upload ulang.** Lagu baru bisa ditambahkan kapan saja, dan langsung ikut diputar.
- **Radio yang selalu hidup.** Penonton yang kembali besok menemukan radio yang sama, masih berjalan.

Ada juga yang dikorbankan. Hal-hal itu adalah:

- **Sebuah komputer harus menyala terus.** Kalau komputer itu mati, atau internet di rumah putus, radionya ikut berhenti.
- **Upload berjalan terus.** Siaran mengirim data ke YouTube setiap detik, siang dan malam. Berapa banyak datanya dibahas di bagian tentang anjuran YouTube di bawah.
- **Siarannya tidak tersimpan sebagai video.** YouTube hanya bisa menyimpan siaran yang lebih pendek dari 12 jam secara otomatis. Siaran yang lebih panjang mungkin tidak tersimpan sama sekali.

Jadi, radio live 24 jam cocok jika kamu ingin tempat yang selalu hidup bagi penonton yang sudah datang. Radio ini tidak mencari penonton sendiri. Orang tetap harus menemukannya lebih dulu, misalnya lewat video biasa atau Shorts di channel yang sama.

## Anatomi radio live 24 jam

Sebelum melihat cara kerjanya, kita lihat dulu bagian-bagiannya. Sebuah radio live 24 jam terdiri dari enam bagian, dan empat di antaranya berjalan di satu komputer. Keenam bagian itu adalah:

- **Musik** adalah folder berisi file lagu. Lagu-lagu itu diputar satu per satu menurut urutan nama, lalu kembali ke awal.
- **Video latar** adalah gambar yang tidak berubah. Seperti jendela terminal, pemandangan, atau ilustrasi. Video ini diulang terus selama siaran berjalan.
- **Teks live** adalah tulisan yang berubah di atas video latar. Seperti jam, judul lagu yang sedang diputar, dan sapaan untuk penonton.
- **Encoder** adalah program yang menggabungkan gambar dan suara, lalu memadatkannya supaya cukup kecil untuk dikirim lewat internet. Seri ini memakai ffmpeg sebagai encoder.
- **Server YouTube** menerima siaran itu, mengubahnya ke beberapa ukuran gambar, lalu mengirimnya ke setiap penonton.
- **Live chat** berisi komentar penonton. Komentar itu dibaca kembali oleh komputer kita, lalu disapa di layar.

Bagian-bagian di komputer kita diatur oleh satu program yang kita tulis sendiri. Program ini memutar musik, menulis teks live, membaca live chat, dan menjaga encoder tetap berjalan.

![Diagram anatomi radio live 24 jam. Di kiri, kotak besar Raspberry Pi berisi folder musik, video latar, teks live, dan program pengendali, yang semuanya masuk ke ffmpeg sebagai encoder. Panah biru dari ffmpeg menuju server YouTube lewat RTMP dengan stream key. Server YouTube mengirim siaran ke penonton. Komentar penonton masuk ke live chat, dan panah abu-abu membawa komentar itu kembali ke program pengendali.](assets/radio-anatomy.png)

*Semua yang dilihat penonton dibuat di satu komputer kecil, lalu dikirim sebagai satu siaran.*

## Cara kerja siaran adalah "gambar dan suara yang dipadatkan sambil berjalan"

Sekarang kita ikuti satu detik siaran melewati anatomi tadi. Satu detik siaran berisi 30 gambar diam, yang disebut frame, dan 44.100 sampel suara untuk setiap telinga. Karena itu, siaran ini disebut 30 fps, yaitu 30 frame per detik.

Perjalanan satu detik itu adalah:

1. Program kita membuka lagu dan mengubahnya menjadi suara mentah, yaitu deretan angka yang menggambarkan getaran suara. Suara mentah ini disebut PCM.
2. Encoder mengambil satu frame dari video latar, lalu menggambar teks live di atasnya.
3. Encoder memadatkan frame itu dengan codec H.264, dan memadatkan suaranya dengan codec AAC. Codec adalah cara memadatkan gambar atau suara, beserta cara membukanya kembali.
4. Hasilnya dibungkus dalam format FLV, lalu dikirim ke server YouTube lewat RTMP, yaitu protokol untuk mengirim siaran ke sebuah server.
5. YouTube membuka bungkus itu, mengubahnya ke beberapa ukuran gambar, lalu mengirimnya ke setiap penonton.
6. Penonton melihat detik itu beberapa saat kemudian. Jeda ini disebut latency. Dengan pengaturan latency paling rendah di YouTube, kebanyakan penonton melihatnya kurang dari 5 detik kemudian.

### Keyframe menentukan kapan penonton baru bisa mulai menonton

H.264 tidak menyimpan setiap frame secara utuh. Kebanyakan frame hanya berisi bagian yang berubah dari frame sebelumnya. Untuk gambar yang hampir diam, seperti radio kita, perubahannya kecil sekali, jadi datanya juga kecil.

Frame yang disimpan utuh disebut keyframe. Penonton yang baru bergabung harus menunggu keyframe berikutnya sebelum gambarnya muncul, karena frame lain hanya berisi perubahan dari frame yang belum ia terima. Karena itu, YouTube menganjurkan satu keyframe setiap 2 detik, dan tidak lebih jarang dari 4 detik.

![Garis waktu 4 detik siaran pada 30 fps. Keyframe, digambar sebagai kotak biru tinggi, muncul di detik 0, 2, dan 4. Di antara dua keyframe ada 59 frame kecil yang hanya berisi perubahan. Seorang penonton bergabung di detik 1,3, menunggu 0,7 detik, dan gambarnya baru muncul di keyframe detik 2.](assets/keyframe-every-two-seconds.png)

*Penonton yang bergabung di tengah harus menunggu keyframe berikutnya.*

## Anjuran YouTube untuk encoder siaran

YouTube punya anjuran untuk encoder, dan siaran yang mengikutinya diterima dengan paling mulus. Seri ini bersiaran dalam 720p 30 fps, yaitu gambar 1280 × 720 piksel dengan 30 frame per detik. Untuk ukuran itu, dengan H.264, anjurannya adalah:

- Bitrate video minimal 3 Mbit/s, dan 8 Mbit/s yang dianjurkan. Bitrate adalah jumlah data per detik.
- Bitrate yang tetap, yang disebut CBR (constant bitrate).
- Satu keyframe setiap 2 detik.
- Suara AAC, 128 kbit/s, 44,1 kHz, stereo.
- Dikirim lewat RTMP atau RTMPS. RTMPS adalah RTMP di dalam koneksi terenkripsi, dan YouTube menganjurkannya.

### Kenapa bitrate radio ini tidak dibuat tetap?

Bitrate yang tetap masuk akal untuk siaran yang gambarnya banyak bergerak. Encoder punya jatah data yang sama setiap detik, dan penonton mendapat gambar yang stabil. Tetapi radio kita gambarnya hampir diam, dan H.264 hanya butuh sedikit data untuk itu.

Di percobaan saya, video latar yang diam dengan batas 3000 kbit/s hanya menghasilkan sekitar 150 kbit/s, sudah termasuk suara 128 kbit/s. Untuk mencapai 3000 kbit/s yang tetap, encoder harus mengisi sisanya dengan data kosong. Saat dipaksa begitu, siaran yang sama menjadi sekitar 3060 kbit/s.

Selama sebulan penuh, bedanya besar. Pada 3060 kbit/s, radio meng-upload sekitar 990 GB sebulan. Pada 150 kbit/s, hanya sekitar 49 GB.

Jadi, seri ini memakai 3000 kbit/s sebagai batas atas, bukan sebagai angka tetap. Kelemahannya, YouTube Studio bisa memperingatkan bahwa bitrate siaranmu terlalu rendah. Di siaran saya, penonton tidak melihat bedanya, karena gambarnya memang hampir diam.

Kalau video latarmu banyak bergerak, misalnya animasi, bitrate-nya naik sendiri sampai batas itu. Kalau kamu tetap ingin bitrate yang benar-benar tetap, caranya ada di bagian percobaan di bawah.

## Di mana radio live 24 jam dijalankan?

Encoder harus berjalan di sebuah komputer, setiap jam, setiap hari. Ada empat pilihan yang masuk akal, dan masing-masing punya kelemahan. Keempatnya adalah:

- **PC di rumah.** PC biasa cukup kuat untuk siaran apa pun. Kelemahannya, PC itu harus menyala terus, lengkap dengan listrik dan suara kipasnya, dan setiap restart untuk update memutus siaran.
- **VPS.** VPS adalah server virtual yang disewa di data center, dengan internet yang cepat dan selalu menyala. Kelemahannya, sewanya dibayar setiap bulan, dan VPS biasa tidak punya encoder hardware, jadi semua pemadatan gambar dikerjakan oleh CPU yang ikut disewa.
- **Layanan siaran 24 jam.** Ada layanan berbayar yang menerima video dan musikmu, lalu menyiarkannya ke YouTube. Kelemahannya, kamu membayar setiap bulan, dan hanya bisa memakai fitur yang mereka sediakan.
- **Raspberry Pi.** Raspberry Pi 3 dan 4 punya encoder H.264 hardware, yaitu bagian chip yang khusus memadatkan video tanpa membebani CPU. Power supply yang dianjurkan untuk Raspberry Pi 3B+ adalah 5,1 V 2,5 A, jadi dayanya paling banyak sekitar 13 watt.

Seri ini memilih Raspberry Pi, karena kecil, hemat listrik, dan punya encoder hardware. Kelemahannya, CPU-nya lemah. Setiap teks yang digambar di atas video menambah beban, dan di Bagian 5 kita akan mengukur seberapa jauh Raspberry Pi 3B+ sanggup.

Raspberry Pi 5 justru tidak punya encoder H.264 hardware. Pi 5 memadatkan H.264 dengan CPU-nya, yang memang jauh lebih cepat daripada CPU Pi 3 dan Pi 4. Kode di seri ini mencoba encoder hardware lebih dulu, lalu memakai encoder software kalau encoder hardware tidak ada.

## Kenapa encoder-nya ffmpeg, dan bukan OBS?

OBS adalah jawaban yang biasa untuk siaran live. OBS punya tampilan untuk menyusun scene, yaitu susunan gambar, teks, dan suara, dan bisa langsung terhubung ke YouTube. Untuk siaran dari PC dengan layar, OBS hampir selalu pilihan yang tepat.

Masalahnya, OBS adalah aplikasi desktop. OBS butuh layar dan tampilan grafis, sedangkan Raspberry Pi kita berjalan tanpa layar di pojok ruangan. Susunan scene juga diatur dengan mouse, bukan oleh program lain.

ffmpeg adalah program baris perintah yang bisa membaca, mengubah, dan mengirim hampir semua format gambar dan suara. ffmpeg bisa dijalankan dan diatur oleh program lain, tanpa layar. Kelemahannya, semua pengaturan ditulis sebagai opsi di baris perintah, dan susunan layar ditulis sebagai filter, yaitu langkah pengolahan gambar yang dijalankan di setiap frame.

Jadi, OBS cocok jika kamu bersiaran dari PC sambil duduk di depannya. ffmpeg cocok jika siaran harus berjalan sendiri berbulan-bulan, di komputer tanpa layar.

## Bahasa pemrograman untuk radio ini adalah "Go"

ffmpeg yang memadatkan dan mengirim siaran, tetapi harus ada yang mengatur ffmpeg. Program pengendali ini memutar lagu satu per satu, menulis teks live, membaca live chat, dan menjalankan ffmpeg lagi saat ia berhenti. Semua itu terjadi pada saat yang sama.

Seri ini menulis program itu dengan Go, bahasa pemrograman buatan Google. Alasannya adalah:

- **Banyak pekerjaan bersamaan.** Go punya goroutine, yaitu cara ringan untuk menjalankan banyak pekerjaan sekaligus. Pemutar musik, pengetik teks, dan pembaca chat masing-masing berjalan di goroutine-nya sendiri.
- **Satu file untuk Raspberry Pi.** Go bisa membuat program untuk Raspberry Pi dari PC biasa. Hasilnya satu file executable, tanpa runtime yang harus dipasang lebih dulu. Runtime adalah program pendukung yang harus terpasang supaya sebuah bahasa bisa berjalan.
- **Library bawaan yang cukup.** Library adalah kode siap pakai yang dipanggil dari program kita. Menjalankan program lain, HTTP, dan JSON sudah ada di library bawaan Go, jadi program ini tidak butuh library tambahan.

Tapi kenapa tidak script shell saja? Script shell memang cukup untuk menjalankan satu perintah berulang-ulang, dan Bagian 5 memakainya untuk itu. Tetapi menulis suara ke ffmpeg tepat waktu, sambil mengetik teks dan membaca chat, jauh lebih sulit ditulis dengan shell.

Python juga bisa mengerjakan semua itu. Seri ini memilih Go karena satu file executable lebih mudah dipindahkan ke Raspberry Pi, tanpa memasang apa pun selain ffmpeg.

## Mencoba siaran pertama dengan satu perintah ffmpeg

Sebelum menulis program, kita coba dulu siaran paling sederhana. Satu perintah ffmpeg sudah bisa mengulang sebuah video dan sebuah lagu, lalu mengirimnya sebagai siaran.

Kalau belum punya video dan lagu, ffmpeg bisa membuatnya. Perintah pertama membuat video latar berwarna gelap sepanjang 10 detik, dan perintah kedua membuat nada 30 detik sebagai pengganti lagu. Perintahnya adalah:

```sh
ffmpeg -f lavfi -i "color=c=0x1a1b26:s=1280x720:r=30" -t 10 -pix_fmt yuv420p latar.mp4
ffmpeg -f lavfi -i "sine=frequency=440:duration=30" lagu.wav
```

Sekarang kita siarkan keduanya ke sebuah file selama 20 detik. File ini berisi persis apa yang akan dikirim ke YouTube, jadi bisa diperiksa dulu. Perintahnya adalah:

```sh
ffmpeg -re -stream_loop -1 -i latar.mp4 -re -stream_loop -1 -i lagu.wav \
  -map 0:v -map 1:a \
  -c:v libx264 -preset veryfast -r 30 -b:v 3000k -maxrate 3000k -bufsize 6000k \
  -g 60 -keyint_min 60 -sc_threshold 0 -pix_fmt yuv420p \
  -c:a aac -b:a 128k -ar 44100 -ac 2 \
  -t 20 -f flv coba.flv
```

Setiap opsi di perintah itu punya tugasnya sendiri. Tugas-tugas itu adalah:

- `-re` membaca input secepat waktu aslinya. Tanpa opsi ini, ffmpeg membaca secepat mungkin, dan 20 detik siaran selesai dalam sekejap.
- `-stream_loop -1` mengulang input tanpa akhir.
- `-map 0:v -map 1:a` mengambil gambar dari input pertama dan suara dari input kedua.
- `-c:v libx264 -preset veryfast` memadatkan gambar dengan H.264. Preset menentukan seberapa keras encoder bekerja, dan `veryfast` memakai sedikit CPU.
- `-r 30` membuat siaran 30 fps.
- `-b:v 3000k -maxrate 3000k -bufsize 6000k` membatasi bitrate video sampai 3000 kbit/s.
- `-g 60 -keyint_min 60 -sc_threshold 0` membuat keyframe setiap 60 frame, yaitu setiap 2 detik, dan tidak di tempat lain.
- `-pix_fmt yuv420p` memakai format warna yang bisa diputar oleh semua pemutar video.
- `-c:a aac -b:a 128k -ar 44100 -ac 2` memadatkan suara menjadi AAC 128 kbit/s, 44,1 kHz, stereo.
- `-t 20 -f flv coba.flv` berhenti setelah 20 detik, dan menulis hasilnya dalam format FLV.

Isi file itu bisa diperiksa dengan ffprobe, yang ikut terpasang bersama ffmpeg. Perintahnya adalah:

```sh
ffprobe -v error -show_entries stream=codec_name,width,height,sample_rate,channels:format=bit_rate -of compact coba.flv
```

Di komputer saya, hasilnya adalah:

```text
stream|codec_name=h264|width=1280|height=720
stream|codec_name=aac|sample_rate=44100|channels=2
format|bit_rate=149057
```

Hasil itu sesuai dengan anjuran YouTube, yaitu H.264 1280 × 720 dan AAC 44,1 kHz stereo. Bitrate seluruh siaran hanya sekitar 149 kbit/s, karena gambarnya diam.

### Mengirim siaran pertama ke YouTube

Untuk bersiaran ke YouTube, buka YouTube Studio, buat siaran live, lalu salin stream key-nya. Stream key adalah kode rahasia yang menentukan ke channel mana sebuah siaran masuk. Siapa pun yang memegangnya bisa bersiaran di channel-mu, jadi jangan dibagikan dan jangan pernah tampil di layar.

Simpan stream key itu di variabel lingkungan `STREAM_KEY`, supaya tidak tertulis di perintah. Lalu ganti tujuan `-t 20 -f flv coba.flv` dengan alamat server YouTube. Perintahnya menjadi:

```sh
ffmpeg -re -stream_loop -1 -i latar.mp4 -re -stream_loop -1 -i lagu.wav \
  -map 0:v -map 1:a \
  -c:v libx264 -preset veryfast -r 30 -b:v 3000k -maxrate 3000k -bufsize 6000k \
  -g 60 -keyint_min 60 -sc_threshold 0 -pix_fmt yuv420p \
  -c:a aac -b:a 128k -ar 44100 -ac 2 \
  -f flv "rtmp://a.rtmp.youtube.com/live2/$STREAM_KEY"
```

Dalam beberapa detik, YouTube Studio menampilkan siaranmu. Siaran itu berjalan sampai kamu menekan Ctrl+C.

Kalau kamu ingin bitrate yang benar-benar tetap, tambahkan `-x264-params nal-hrd=cbr` setelah `-bufsize 6000k`. Encoder lalu mengisi kekurangan data dengan data kosong, seperti yang dibahas di bagian tentang anjuran YouTube.

## Kenapa satu perintah ffmpeg belum cukup?

Perintah tadi sudah menghasilkan siaran yang sah. Tetapi untuk radio yang berjalan berbulan-bulan, ada tiga masalah. Ketiganya adalah:

- **Hanya satu lagu.** ffmpeg bisa membaca daftar lagu dari sebuah file, tetapi daftar itu hanya dibaca sekali, saat ffmpeg mulai. Menambah lagu berarti menghentikan ffmpeg dan menjalankannya lagi, dan penonton melihat siarannya terputus.
- **Tidak ada yang menyalakannya lagi.** Saat internet putus sebentar, ffmpeg berhenti dengan error, dan siaran mati sampai ada orang yang menjalankannya lagi.
- **Layarnya diam.** Penonton tidak tahu lagu apa yang sedang diputar, dan tidak ada yang membalas komentar mereka.

Ketiga masalah ini butuh program yang mengatur ffmpeg dari luar. Program itulah yang kita tulis di bagian-bagian berikutnya. Rencananya adalah:

- **Bagian 2** memutar playlist tanpa henti ke satu ffmpeg, dan menjalankan ffmpeg lagi saat ia berhenti, tanpa memotong lagu.
- **Bagian 3** menulis teks live di atas video latar, yaitu jam, judul lagu, dan terminal yang mengetik `play <lagu>`.
- **Bagian 4** membaca live chat dan menyapa setiap penonton yang berkomentar.
- **Bagian 5** memindahkan semuanya ke Raspberry Pi, supaya menyala sendiri dan berjalan 24 jam.

### Yang perlu kamu siapkan untuk mengikuti seri ini

Untuk mengikuti seri ini dan menjalankan radionya sendiri, ada empat hal yang perlu kamu siapkan. Keempatnya adalah:

- ffmpeg 7 atau yang lebih baru. Kode di seri ini diuji dengan ffmpeg 7.1.
- Go 1.25 atau yang lebih baru, untuk menulis dan membangun programnya.
- Channel YouTube yang sudah bisa melakukan live stream.
- Raspberry Pi 3 atau 4 dengan Raspberry Pi OS, atau komputer Linux lain, untuk Bagian 5.

## Berikutnya dalam seri ini

Di Bagian 2, kita mulai menulis program pengendalinya dengan Go. Kita mulai dari musik, yaitu cara memutar playlist tanpa henti ke satu ffmpeg yang terus berjalan. Kita juga membuat ffmpeg menyala lagi sendiri saat internet putus, tanpa memotong lagu yang sedang diputar.
