package plaintext

import "testing"

func TestFrom(t *testing.T) {
	for _, c := range []struct{ name, md, want string }{
		{"plain text stays", "Toko online-mu makin ramai.\n\nDi sinilah message broker berguna.", "Toko online-mu makin ramai.\n\nDi sinilah message broker berguna."},
		{"line breaks stay", "baris satu\nbaris dua\n#blender #gamedev", "baris satu\nbaris dua\n#blender #gamedev"},
		{"hashtags are not headings", "#blender #blender3d Batu low poly", "#blender #blender3d Batu low poly"},
		{"bold and italic", "**Jakarta** dan _Surabaya_, *kota* besar", "Jakarta dan Surabaya, kota besar"},
		{"headings", "# Judul\n\nIsi.\n\n## Bagian\nLagi.", "Judul\n\nIsi.\n\nBagian\n\nLagi."},
		{"lists", "Empat masalah:\n\n- Satu.\n- Dua.\n\n1. a\n2. b", "Empat masalah:\n\n- Satu.\n- Dua.\n\n1. a\n2. b"},
		{"nested list", "- a\n  - b\n- c", "- a\n  - b\n- c"},
		{"loose list", "- a\n\n- b", "- a\n\n- b"},
		{"ordered start", "3. tiga\n4. empat", "3. tiga\n4. empat"},
		{"links", "Baca [docs](https://example.com/d) atau <https://a.id> dan [https://b.id](https://b.id)", "Baca docs (https://example.com/d) atau https://a.id dan https://b.id"},
		{"code", "Jalankan `go test`:\n\n```sh\ngo test ./...\n```", "Jalankan go test:\n\ngo test ./..."},
		{"not markup", "snake_case_name, 5 * 3 = 15, harga 100%", "snake_case_name, 5 * 3 = 15, harga 100%"},
		{"escapes and entities", `\*bukan tebal\* &amp; &#65;`, "*bukan tebal* & A"},
		{"images dropped", "Lihat:\n\n![gambar](a.png)\n\nSelesai.", "Lihat:\n\nSelesai."},
		{"blockquote", "> kutipan\n> dua", "kutipan\ndua"},
		{"crlf", "a\r\nb\r\n\r\nc", "a\nb\n\nc"},
		{"empty", "  \n ", ""},
	} {
		if got := From(c.md); got != c.want {
			t.Errorf("%s: From(%q)\n got %q\nwant %q", c.name, c.md, got, c.want)
		}
	}
}
