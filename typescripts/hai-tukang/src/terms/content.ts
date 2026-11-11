import type { Language } from '../i18n'

// A section is a heading and its blocks: a string is a paragraph, a list of
// strings is a bulleted list.
export type TermsSection = { heading: string; blocks: (string | string[])[] }
export type TermsText = { title: string; updated: string; draft: string; intro: string; sections: TermsSection[] }

// The Terms and Policies (acceptance HT-014), from the app's actual rules:
// the numbers are internal/penalties' and internal/visits'. A draft for the
// pilot: to be reviewed, legally too (UU PDP No. 27/2022), with the
// operator's contact filled in, before launch. Changing it means a new
// TERMS_VERSION (version.ts).
export const terms: Record<Language, TermsText> = {
  id: {
    title: 'Syarat dan Kebijakan',
    updated: 'Versi 7 November 2026',
    draft: 'Draf untuk masa uji coba. Teks ini akan ditinjau, termasuk secara hukum, sebelum Hai Tukang diluncurkan.',
    intro:
      'Halaman ini menjelaskan aturan memakai Hai Tukang dan bagaimana data Anda digunakan. Bacalah sebelum mendaftar: akun hanya dibuat jika Anda menyetujuinya.',
    sections: [
      {
        heading: 'Apa itu Hai Tukang',
        blocks: [
          'Hai Tukang mempertemukan orang yang butuh perbaikan rumah dengan tukang. Pekerjaan, harga, dan pembayaran disepakati langsung antara pelanggan dan tukang.',
          [
            'Hai Tukang bukan pihak dalam kesepakatan kerja itu dan tidak mempekerjakan tukang.',
            'Hai Tukang tidak memeriksa keahlian, identitas, atau hasil kerja tukang. Yang diperiksa hanya bahwa nomor HP-nya aktif di WhatsApp ("Nomor HP terverifikasi").',
            'Hai Tukang tidak menerima atau menyimpan pembayaran. Berhati-hatilah membayar di muka.',
          ],
        ],
      },
      {
        heading: 'Akun',
        blocks: [
          [
            'Satu nomor HP Indonesia untuk satu akun. Nomor itu diverifikasi dengan kode WhatsApp saat mendaftar.',
            'Anda bisa mendaftar dengan kata sandi atau dengan akun Google. Satu akun Google juga hanya untuk satu akun Hai Tukang.',
            'Isi data yang benar: nama Anda sendiri dan nomor HP milik Anda.',
            'Jaga kata sandi dan kode WhatsApp Anda. Hai Tukang tidak pernah meminta kode Anda.',
          ],
        ],
      },
      {
        heading: 'Sikap yang diharapkan',
        blocks: [
          [
            'Bersikap sopan dan jujur kepada sesama pengguna.',
            'Dilarang menipu, mengancam, melecehkan, atau membuat laporan palsu.',
            'Hai Tukang dapat menangguhkan atau menutup akun yang melanggar aturan ini.',
          ],
        ],
      },
      {
        heading: 'Kunjungan dan kode kedatangan',
        blocks: [
          'Pelanggan mengusulkan waktu kunjungan kepada tukang yang menanggapi permintaannya. Pihak lain menerima atau mengusulkan waktu lain.',
          'Setiap kunjungan punya kode kedatangan yang hanya dilihat pelanggan. Berikan kode itu kepada tukang setelah ia tiba, bukan sebelumnya. Tukang memasukkannya di aplikasi sebagai tanda ia sudah datang.',
        ],
      },
      {
        heading: 'Poin pelanggaran',
        blocks: [
          'Agar janji kunjungan ditepati, pelanggaran memberi poin:',
          [
            'Tukang tidak datang: 2 poin. Pelanggan dapat melaporkannya mulai 30 menit setelah waktu kunjungan berakhir.',
            'Pelanggan tidak ada di rumah: 2 poin. Tukang dapat melaporkannya selama waktu kunjungan sampai 30 menit sesudahnya.',
            'Membatalkan kunjungan yang sudah disepakati kurang dari 12 jam sebelum dimulai: 1 poin bagi yang membatalkan.',
            'Tukang tiba lebih dari 30 menit setelah waktu kunjungan berakhir: 1 poin.',
          ],
          'Poin berlaku 90 hari. Dengan 4 poin, Anda mendapat peringatan. Dengan 6 poin, akun ditangguhkan: pertama kali 7 hari, kedua kali 30 hari, dan ketiga kalinya akun ditutup. Penangguhan menghabiskan poin yang dihitung.',
          'Selama ditangguhkan, Anda tidak dapat mengatur kunjungan baru, dan tukang tidak tampil di pencarian.',
        ],
      },
      {
        heading: 'Bantahan dan peninjauan',
        blocks: [
          'Jika dilaporkan, Anda punya 24 jam untuk membantah dengan penjelasan. Poin dari laporan baru dihitung setelah 24 jam itu; poin yang dibantah ditahan sampai admin memutuskan.',
          'Admin Hai Tukang memutuskan bantahan dan meninjau akun yang ditutup. Setiap keputusan disertai alasan dan dicatat. Admin juga dapat menangguhkan akun 1 sampai 365 hari karena pelanggaran aturan ini.',
        ],
      },
      {
        heading: 'Data dan privasi',
        blocks: [
          'Yang kami simpan:',
          [
            'Data akun: nama, nomor HP, peran (pelanggan atau tukang), dan kata sandi dalam bentuk tersandi.',
            'Jika mendaftar dengan Google: ID akun Google, alamat email, dan nama dari Google.',
            'Profil tukang: jenis pekerjaan, wilayah, dan keterangan tentang dirinya.',
            'Permintaan perbaikan, tanggapan, kunjungan, laporan, bantahan, poin, dan keputusan admin.',
            'Untuk notifikasi, alamat langganan push dari browser Anda; untuk tetap masuk, cookie sesi.',
          ],
          'Siapa melihat apa:',
          [
            'Nama, nomor HP, dan profil tukang dapat dilihat pengguna yang sudah masuk.',
            'Permintaan pelanggan (jenis pekerjaan, wilayah, dan keterangan) dapat dilihat tukang. Nomor HP pelanggan hanya dilihat tukang yang sudah menanggapi permintaannya.',
            'Alamat email Google tidak ditampilkan kepada pengguna lain.',
            'Admin Hai Tukang dapat melihat data akun untuk meninjau laporan, bantahan, dan penangguhan.',
          ],
          'Kode verifikasi dikirim lewat penyedia layanan WhatsApp. Notifikasi dikirim lewat layanan push browser Anda (misalnya milik Google, Mozilla, atau Apple). Kami tidak menjual data Anda.',
          'Untuk meminta salinan atau penghapusan data akun Anda, hubungi pengelola Hai Tukang. Kontak pengelola akan dicantumkan di sini sebelum peluncuran.',
        ],
      },
      {
        heading: 'Perubahan',
        blocks: [
          'Jika halaman ini berubah, tanggal versinya ikut berubah, dan Anda diminta membaca serta menyetujuinya lagi sebelum memakai Hai Tukang.',
        ],
      },
    ],
  },
  en: {
    title: 'Terms and Policies',
    updated: 'Version of 7 November 2026',
    draft: 'Draft for the pilot. This text will be reviewed, legally too, before Hai Tukang launches.',
    intro:
      'This page sets out the rules for using Hai Tukang and how your data is used. Read it before you sign up: an account is only made if you agree to it.',
    sections: [
      {
        heading: 'What Hai Tukang is',
        blocks: [
          'Hai Tukang connects people who need home repairs with repairmen. The work, the price and the payment are agreed directly between the customer and the repairman.',
          [
            'Hai Tukang is not a party to that agreement and does not employ repairmen.',
            'Hai Tukang does not check repairmen’s skills, identity or work. It only checks that their phone number is active on WhatsApp ("Phone number verified").',
            'Hai Tukang does not take or hold payments. Be careful about paying in advance.',
          ],
        ],
      },
      {
        heading: 'Accounts',
        blocks: [
          [
            'One Indonesian phone number per account. The number is verified with a WhatsApp code when you sign up.',
            'You can sign up with a password or with a Google account. A Google account, too, is for one Hai Tukang account only.',
            'Give true details: your own name and a phone number that is yours.',
            'Keep your password and WhatsApp codes to yourself. Hai Tukang never asks for your code.',
          ],
        ],
      },
      {
        heading: 'Expected conduct',
        blocks: [
          [
            'Be polite and honest with other users.',
            'Fraud, threats, harassment and false reports are not allowed.',
            'Hai Tukang may suspend or close accounts that break these rules.',
          ],
        ],
      },
      {
        heading: 'Visits and arrival codes',
        blocks: [
          'The customer proposes a visit time to a repairman who responded to their request. The other side accepts it or proposes another time.',
          'Every visit has an arrival code that only the customer sees. Give it to the repairman once he has arrived, not before. He enters it in the app to show he came.',
        ],
      },
      {
        heading: 'Penalty points',
        blocks: [
          'So that visits happen as agreed, breaking an agreement gives points:',
          [
            'Repairman does not come: 2 points. The customer can report it from 30 minutes after the visit window ends.',
            'Customer not at home: 2 points. The repairman can report it during the visit window and up to 30 minutes after.',
            'Cancelling an agreed visit less than 12 hours before it starts: 1 point to whoever cancels.',
            'Repairman arrives more than 30 minutes after the visit window ends: 1 point.',
          ],
          'Points last 90 days. At 4 points you get a warning. At 6 points the account is suspended: 7 days the first time, 30 days the second, and the third time the account is closed. A suspension uses up the points counted.',
          'While suspended, you cannot arrange new visits, and a repairman is not shown in search.',
        ],
      },
      {
        heading: 'Disputes and review',
        blocks: [
          'If you are reported, you have 24 hours to dispute it with an explanation. A report’s points count only after those 24 hours; disputed points are held until an admin decides.',
          'Hai Tukang’s admins decide disputes and review closed accounts. Every decision comes with a reason and is recorded. Admins can also suspend an account for 1 to 365 days for breaking these rules.',
        ],
      },
      {
        heading: 'Data and privacy',
        blocks: [
          'What we keep:',
          [
            'Account details: name, phone number, role (customer or repairman), and your password in hashed form.',
            'If you sign up with Google: your Google account ID, email address, and the name Google gives.',
            'A repairman’s profile: kinds of work, area, and what he writes about himself.',
            'Repair requests, responses, visits, reports, disputes, points, and admin decisions.',
            'For notifications, your browser’s push subscription address; to keep you signed in, a session cookie.',
          ],
          'Who sees what:',
          [
            'A repairman’s name, phone number and profile can be seen by signed-in users.',
            'A customer’s request (kind of work, area and description) can be seen by repairmen. The customer’s phone number is only seen by repairmen who responded to the request.',
            'Your Google email address is not shown to other users.',
            'Hai Tukang’s admins can see account details to review reports, disputes and suspensions.',
          ],
          'Verification codes are sent through a WhatsApp service provider. Notifications are sent through your browser’s push service (for example Google’s, Mozilla’s or Apple’s). We do not sell your data.',
          'To ask for a copy of your account’s data, or for its deletion, contact Hai Tukang’s operator. The operator’s contact will be given here before launch.',
        ],
      },
      {
        heading: 'Changes',
        blocks: [
          'When this page changes, its version date changes too, and you are asked to read and agree to it again before using Hai Tukang.',
        ],
      },
    ],
  },
}
